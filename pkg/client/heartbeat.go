package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

func (c *RegistryClient) startHeartbeat() {
	c.mu.RLock()
	interval := c.heartbeatInterval
	c.mu.RUnlock()

	if interval == 0 {
		interval = 10 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	c.mu.Lock()
	c.stopCancel = cancel
	c.mu.Unlock()

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		defer cancel()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := c.sendHeartbeat(ctx); err != nil {
					if errors.Is(err, ErrInstanceNotFound) {
						c.logger.Warn("instance not found, attempting re-registration")
						if reErr := c.reregister(ctx); reErr != nil {
							c.logger.Error("re-registration failed", "error", reErr)
						}
					} else {
						c.logger.Warn("heartbeat failed", "error", err)
					}
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

	token, err := c.serviceToken()
	if err != nil {
		return fmt.Errorf("failed to generate service token: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Token", token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send heartbeat: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
		}
	}(resp.Body)

	if resp.StatusCode == http.StatusNotFound {
		return ErrInstanceNotFound
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("heartbeat failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
