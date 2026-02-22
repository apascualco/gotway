package client

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type RegistryClient struct {
	gatewayURL  string
	privateKey  *rsa.PrivateKey
	serviceName string
	instanceID  string

	lastRegisterReq RegisterRequest

	httpClient        *http.Client
	heartbeatInterval time.Duration
	stopCh            chan struct{}
	stopCancel        context.CancelFunc
	wg                sync.WaitGroup
	mu                sync.RWMutex
	registered        bool
	stopped           bool

	logger *slog.Logger
}

type Option func(*RegistryClient)

func WithLogger(logger *slog.Logger) Option {
	return func(c *RegistryClient) {
		c.logger = logger
	}
}

func NewRegistryClient(gatewayURL, privateKeyPEM, serviceName string, opts ...Option) (*RegistryClient, error) {
	privKey, err := parseRSAPrivateKey(privateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	c := &RegistryClient{
		gatewayURL:  gatewayURL,
		privateKey:  privKey,
		serviceName: serviceName,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		stopCh: make(chan struct{}),
		logger: slog.Default(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

func (c *RegistryClient) Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, error) {
	c.mu.Lock()
	if c.registered {
		c.mu.Unlock()
		return nil, fmt.Errorf("already registered, call Shutdown first")
	}
	c.lastRegisterReq = req
	c.mu.Unlock()

	var resp *RegisterResponse
	var err error

	err = c.retryWithBackoff(ctx, 5, func() error {
		resp, err = c.doRegister(ctx, req)
		return err
	})

	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.instanceID = resp.InstanceID
	c.heartbeatInterval = time.Duration(resp.HeartbeatInterval) * time.Second
	c.registered = true
	c.mu.Unlock()

	c.startHeartbeat()

	c.logger.Info("service registered",
		"instance_id", resp.InstanceID,
		"heartbeat_interval", resp.HeartbeatInterval,
		"routes", len(resp.RegisteredRoutes),
	)

	return resp, nil
}

func (c *RegistryClient) doRegister(ctx context.Context, req RegisterRequest) (*RegisterResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.gatewayURL+"/internal/registry/register", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	token, err := c.serviceToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate service token: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Service-Token", token)

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
		}
	}(httpResp.Body)

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if httpResp.StatusCode == http.StatusConflict {
		var collisionResp struct {
			Error      string           `json:"error"`
			Message    string           `json:"message"`
			Collisions []RouteCollision `json:"collisions"`
		}
		if err := json.Unmarshal(respBody, &collisionResp); err == nil {
			return nil, &CollisionError{Collisions: collisionResp.Collisions}
		}
		return nil, fmt.Errorf("route collision: %s", string(respBody))
	}

	if httpResp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("registration failed with status %d: %s",
			httpResp.StatusCode, string(respBody))
	}

	var resp RegisterResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &resp, nil
}

func (c *RegistryClient) reregister(ctx context.Context) error {
	c.mu.RLock()
	req := c.lastRegisterReq
	c.mu.RUnlock()

	var resp *RegisterResponse
	var err error

	err = c.retryWithBackoff(ctx, 5, func() error {
		resp, err = c.doRegister(ctx, req)
		return err
	})
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.instanceID = resp.InstanceID
	c.heartbeatInterval = time.Duration(resp.HeartbeatInterval) * time.Second
	c.mu.Unlock()

	c.logger.Info("service re-registered",
		"instance_id", resp.InstanceID,
		"heartbeat_interval", resp.HeartbeatInterval,
	)

	return nil
}

func (c *RegistryClient) Deregister(ctx context.Context) error {
	c.mu.RLock()
	instanceID := c.instanceID
	c.mu.RUnlock()

	if instanceID == "" {
		return nil
	}

	body, err := json.Marshal(map[string]string{
		"instance_id": instanceID,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.gatewayURL+"/internal/registry/deregister", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	token, err := c.serviceToken()
	if err != nil {
		return fmt.Errorf("failed to generate service token: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Token", token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send deregister: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("deregister failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	c.logger.Info("service deregistered", "instance_id", instanceID)
	return nil
}

func (c *RegistryClient) Shutdown(ctx context.Context) error {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return nil
	}
	c.stopped = true
	c.mu.Unlock()

	close(c.stopCh)
	if c.stopCancel != nil {
		c.stopCancel()
	}

	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		return fmt.Errorf("shutdown timed out waiting for heartbeat to stop")
	}

	return c.Deregister(ctx)
}

func (c *RegistryClient) InstanceID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.instanceID
}

func (c *RegistryClient) serviceToken() (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub": c.serviceName,
		"aud": "api-gateway",
		"iss": c.serviceName,
		"iat": now.Unix(),
		"exp": now.Add(5 * time.Minute).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(c.privateKey)
}

func (c *RegistryClient) retryWithBackoff(ctx context.Context, maxRetries int, fn func() error) error {
	var lastErr error
	backoff := time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err := fn(); err != nil {
			lastErr = err

			var collisionError *CollisionError
			if errors.As(err, &collisionError) {
				return err
			}

			if attempt < maxRetries {
				c.logger.Warn("operation failed, retrying",
					"attempt", attempt+1,
					"max_retries", maxRetries,
					"backoff", backoff,
					"error", err,
				)

				select {
				case <-time.After(backoff):
					backoff *= 2
					if backoff > 30*time.Second {
						backoff = 30 * time.Second
					}
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		} else {
			return nil
		}
	}

	return fmt.Errorf("operation failed after %d retries: %w", maxRetries, lastErr)
}

func parseRSAPrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("not an RSA private key")
		}
		return rsaKey, nil
	}

	return x509.ParsePKCS1PrivateKey(block.Bytes)
}
