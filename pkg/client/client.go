package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// Route represents a route to be registered with the api.
type Route struct {
	Method    string   `json:"method"`
	Path      string   `json:"path"`
	Public    bool     `json:"public"`
	RateLimit int      `json:"rate_limit,omitempty"`
	Scopes    []string `json:"scopes,omitempty"`
}

// RegisterRequest contains the data needed to register a service.
type RegisterRequest struct {
	ServiceName string            `json:"service_name"`
	Host        string            `json:"host"`
	Port        int               `json:"port"`
	HealthURL   string            `json:"health_url,omitempty"`
	Version     string            `json:"version,omitempty"`
	BasePath    string            `json:"base_path"`
	Routes      []Route           `json:"routes"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// RegisterResponse contains the response from a successful registration.
type RegisterResponse struct {
	InstanceID        string   `json:"instance_id"`
	HeartbeatInterval int      `json:"heartbeat_interval"`
	HeartbeatURL      string   `json:"heartbeat_url"`
	RegisteredRoutes  []string `json:"registered_routes"`
}

// RouteCollision represents a conflict with an existing route.
type RouteCollision struct {
	Method        string `json:"method"`
	Path          string `json:"path"`
	CollisionType string `json:"collision_type"`
	RegisteredBy  string `json:"registered_by"`
}

// CollisionError is returned when routes conflict with existing registrations.
type CollisionError struct {
	Collisions []RouteCollision
}

func (e *CollisionError) Error() string {
	return fmt.Sprintf("route collision: %d route(s) already registered", len(e.Collisions))
}

// RegistryClient manages service registration with the API Gateway.
type RegistryClient struct {
	gatewayURL string
	token      string
	instanceID string

	httpClient        *http.Client
	heartbeatInterval time.Duration
	stopCh            chan struct{}
	wg                sync.WaitGroup
	mu                sync.RWMutex

	logger *slog.Logger
}

// Option is a functional option for configuring the RegistryClient.
type Option func(*RegistryClient)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *RegistryClient) {
		c.httpClient = client
	}
}

// WithLogger sets a custom logger.
func WithLogger(logger *slog.Logger) Option {
	return func(c *RegistryClient) {
		c.logger = logger
	}
}

// WithTimeout sets the HTTP client timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *RegistryClient) {
		c.httpClient.Timeout = timeout
	}
}

// NewRegistryClient creates a new registry client.
func NewRegistryClient(gatewayURL, token string, opts ...Option) *RegistryClient {
	c := &RegistryClient{
		gatewayURL: gatewayURL,
		token:      token,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		stopCh: make(chan struct{}),
		logger: slog.Default(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Register registers the service with the api.
func (c *RegistryClient) Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, error) {
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

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Service-Token", c.token)

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer httpResp.Body.Close()

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

func (c *RegistryClient) startHeartbeat() {
	c.mu.RLock()
	interval := c.heartbeatInterval
	c.mu.RUnlock()

	if interval == 0 {
		interval = 10 * time.Second
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := c.sendHeartbeat(context.Background()); err != nil {
					c.logger.Warn("heartbeat failed", "error", err)
				}
			case <-c.stopCh:
				return
			}
		}
	}()
}

func (c *RegistryClient) sendHeartbeat(ctx context.Context) error {
	c.mu.RLock()
	instanceID := c.instanceID
	c.mu.RUnlock()

	if instanceID == "" {
		return fmt.Errorf("not registered")
	}

	body, err := json.Marshal(map[string]string{
		"instance_id": instanceID,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.gatewayURL+"/internal/registry/heartbeat", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Token", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send heartbeat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("heartbeat failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// Deregister removes the service from the api registry.
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

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Token", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send deregister: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("deregister failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	c.logger.Info("service deregistered", "instance_id", instanceID)
	return nil
}

// Shutdown gracefully shuts down the client, stopping heartbeat and deregistering.
func (c *RegistryClient) Shutdown(ctx context.Context) error {
	close(c.stopCh)

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

// InstanceID returns the current instance ID.
func (c *RegistryClient) InstanceID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.instanceID
}

func (c *RegistryClient) retryWithBackoff(ctx context.Context, maxRetries int, fn func() error) error {
	var lastErr error
	backoff := time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err := fn(); err != nil {
			lastErr = err

			if _, ok := err.(*CollisionError); ok {
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
