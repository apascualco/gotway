package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"
)

type RegisterRequest struct {
	ServiceName string            `json:"service_name"`
	Host        string            `json:"host"`
	Port        int               `json:"port"`
	HealthURL   string            `json:"health_url"`
	Version     string            `json:"version"`
	BasePath    string            `json:"base_path"`
	Routes      []Route           `json:"routes"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type Route struct {
	Method    string   `json:"method"`
	Path      string   `json:"path"`
	Public    bool     `json:"public"`
	RateLimit int      `json:"rate_limit,omitempty"`
	Scopes    []string `json:"scopes,omitempty"`
}

type RegisterResponse struct {
	InstanceID        string   `json:"instance_id"`
	HeartbeatInterval int      `json:"heartbeat_interval"`
	HeartbeatURL      string   `json:"heartbeat_url"`
	RegisteredRoutes  []string `json:"registered_routes"`
}

type HeartbeatRequest struct {
	InstanceID string `json:"instance_id"`
}

type DeregisterRequest struct {
	InstanceID string `json:"instance_id"`
}

func TestRegistry_RegisterService_Success(t *testing.T) {
	client := getHTTPClient()

	req := RegisterRequest{
		ServiceName: "test-service",
		Host:        "localhost",
		Port:        9001,
		HealthURL:   "/health",
		Version:     "1.0.0",
		BasePath:    "/api/v1/test",
		Routes: []Route{
			{Method: "GET", Path: "/hello", Public: true},
			{Method: "POST", Path: "/echo", Public: false},
		},
	}

	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", testServerURL+"/internal/registry/register", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Service-Token", serviceToken)

	resp, err := client.Do(httpReq)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status 201, got %d: %s", resp.StatusCode, string(respBody))
	}

	var registerResp RegisterResponse
	if err := json.NewDecoder(resp.Body).Decode(&registerResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if registerResp.InstanceID == "" {
		t.Error("expected instance_id to be set")
	}

	if registerResp.HeartbeatInterval <= 0 {
		t.Error("expected heartbeat_interval to be positive")
	}

	if len(registerResp.RegisteredRoutes) != 2 {
		t.Errorf("expected 2 registered routes, got %d", len(registerResp.RegisteredRoutes))
	}

	cleanupService(t, registerResp.InstanceID)
}

func TestRegistry_RegisterService_Unauthorized(t *testing.T) {
	client := getHTTPClient()

	req := RegisterRequest{
		ServiceName: "unauthorized-service",
		Host:        "localhost",
		Port:        9002,
		HealthURL:   "/health",
		BasePath:    "/api/v1/unauth",
		Routes: []Route{
			{Method: "GET", Path: "/test", Public: true},
		},
	}

	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", testServerURL+"/internal/registry/register", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Service-Token", "wrong-token")

	resp, err := client.Do(httpReq)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", resp.StatusCode)
	}
}

func TestRegistry_RegisterService_Collision(t *testing.T) {
	client := getHTTPClient()

	req1 := RegisterRequest{
		ServiceName: "collision-service-1",
		Host:        "localhost",
		Port:        9003,
		HealthURL:   "/health",
		BasePath:    "/api/v1/collision",
		Routes: []Route{
			{Method: "GET", Path: "/data", Public: true},
		},
	}

	body1, _ := json.Marshal(req1)
	httpReq1, _ := http.NewRequest("POST", testServerURL+"/internal/registry/register", bytes.NewReader(body1))
	httpReq1.Header.Set("Content-Type", "application/json")
	httpReq1.Header.Set("X-Service-Token", serviceToken)

	resp1, err := client.Do(httpReq1)
	if err != nil {
		t.Fatalf("request 1 failed: %v", err)
	}
	defer resp1.Body.Close()

	if resp1.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201 for first registration, got %d", resp1.StatusCode)
	}

	var registerResp1 RegisterResponse
	json.NewDecoder(resp1.Body).Decode(&registerResp1)
	defer cleanupService(t, registerResp1.InstanceID)

	req2 := RegisterRequest{
		ServiceName: "collision-service-2",
		Host:        "localhost",
		Port:        9004,
		HealthURL:   "/health",
		BasePath:    "/api/v1/collision",
		Routes: []Route{
			{Method: "GET", Path: "/data", Public: true},
		},
	}

	body2, _ := json.Marshal(req2)
	httpReq2, _ := http.NewRequest("POST", testServerURL+"/internal/registry/register", bytes.NewReader(body2))
	httpReq2.Header.Set("Content-Type", "application/json")
	httpReq2.Header.Set("X-Service-Token", serviceToken)

	resp2, err := client.Do(httpReq2)
	if err != nil {
		t.Fatalf("request 2 failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusConflict {
		t.Errorf("expected status 409 for collision, got %d", resp2.StatusCode)
	}
}

func TestRegistry_Heartbeat_Success(t *testing.T) {
	client := getHTTPClient()

	instanceID := registerTestService(t, "heartbeat-service", 9005, "/api/v1/heartbeat")
	defer cleanupService(t, instanceID)

	heartbeatReq := HeartbeatRequest{InstanceID: instanceID}
	body, _ := json.Marshal(heartbeatReq)
	httpReq, _ := http.NewRequest("POST", testServerURL+"/internal/registry/heartbeat", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Service-Token", serviceToken)

	resp, err := client.Do(httpReq)
	if err != nil {
		t.Fatalf("heartbeat request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestRegistry_Heartbeat_InstanceNotFound(t *testing.T) {
	client := getHTTPClient()

	heartbeatReq := HeartbeatRequest{InstanceID: "non-existent-instance"}
	body, _ := json.Marshal(heartbeatReq)
	httpReq, _ := http.NewRequest("POST", testServerURL+"/internal/registry/heartbeat", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Service-Token", serviceToken)

	resp, err := client.Do(httpReq)
	if err != nil {
		t.Fatalf("heartbeat request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", resp.StatusCode)
	}
}

func TestRegistry_Deregister_Success(t *testing.T) {
	client := getHTTPClient()

	instanceID := registerTestService(t, "deregister-service", 9006, "/api/v1/deregister")

	deregisterReq := DeregisterRequest{InstanceID: instanceID}
	body, _ := json.Marshal(deregisterReq)
	httpReq, _ := http.NewRequest("POST", testServerURL+"/internal/registry/deregister", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Service-Token", serviceToken)

	resp, err := client.Do(httpReq)
	if err != nil {
		t.Fatalf("deregister request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	heartbeatReq := HeartbeatRequest{InstanceID: instanceID}
	body2, _ := json.Marshal(heartbeatReq)
	httpReq2, _ := http.NewRequest("POST", testServerURL+"/internal/registry/heartbeat", bytes.NewReader(body2))
	httpReq2.Header.Set("Content-Type", "application/json")
	httpReq2.Header.Set("X-Service-Token", serviceToken)

	resp2, err := client.Do(httpReq2)
	if err != nil {
		t.Fatalf("heartbeat request failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusNotFound {
		t.Errorf("expected service to be removed, got status %d", resp2.StatusCode)
	}
}

func TestRegistry_ListServices_Success(t *testing.T) {
	client := getHTTPClient()

	instanceID := registerTestService(t, "list-service", 9007, "/api/v1/list")
	defer cleanupService(t, instanceID)

	httpReq, _ := http.NewRequest("GET", testServerURL+"/internal/registry/services", nil)
	httpReq.Header.Set("X-Service-Token", serviceToken)

	resp, err := client.Do(httpReq)
	if err != nil {
		t.Fatalf("list services request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var services map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&services); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if _, ok := services["list-service"]; !ok {
		t.Error("expected list-service to be in the response")
	}
}

func TestRegistry_ListServices_Unauthorized(t *testing.T) {
	client := getHTTPClient()

	httpReq, _ := http.NewRequest("GET", testServerURL+"/internal/registry/services", nil)
	httpReq.Header.Set("X-Service-Token", "invalid-token")

	resp, err := client.Do(httpReq)
	if err != nil {
		t.Fatalf("list services request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", resp.StatusCode)
	}
}

func TestRegistry_MultipleInstances_SameService(t *testing.T) {
	client := getHTTPClient()

	req1 := RegisterRequest{
		ServiceName: "multi-instance-service",
		Host:        "localhost",
		Port:        9010,
		HealthURL:   "/health",
		BasePath:    "/api/v1/multi",
		Routes: []Route{
			{Method: "GET", Path: "/data", Public: true},
		},
	}

	body1, _ := json.Marshal(req1)
	httpReq1, _ := http.NewRequest("POST", testServerURL+"/internal/registry/register", bytes.NewReader(body1))
	httpReq1.Header.Set("Content-Type", "application/json")
	httpReq1.Header.Set("X-Service-Token", serviceToken)

	resp1, _ := client.Do(httpReq1)
	defer resp1.Body.Close()

	var registerResp1 RegisterResponse
	json.NewDecoder(resp1.Body).Decode(&registerResp1)
	defer cleanupService(t, registerResp1.InstanceID)

	req2 := RegisterRequest{
		ServiceName: "multi-instance-service",
		Host:        "localhost",
		Port:        9011,
		HealthURL:   "/health",
		BasePath:    "/api/v1/multi",
		Routes: []Route{
			{Method: "GET", Path: "/data", Public: true},
		},
	}

	body2, _ := json.Marshal(req2)
	httpReq2, _ := http.NewRequest("POST", testServerURL+"/internal/registry/register", bytes.NewReader(body2))
	httpReq2.Header.Set("Content-Type", "application/json")
	httpReq2.Header.Set("X-Service-Token", serviceToken)

	resp2, err := client.Do(httpReq2)
	if err != nil {
		t.Fatalf("request 2 failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusCreated {
		t.Errorf("expected status 201 for second instance, got %d", resp2.StatusCode)
	}

	var registerResp2 RegisterResponse
	json.NewDecoder(resp2.Body).Decode(&registerResp2)
	defer cleanupService(t, registerResp2.InstanceID)

	if registerResp1.InstanceID == registerResp2.InstanceID {
		t.Error("expected different instance IDs for different instances")
	}

	httpReq3, _ := http.NewRequest("GET", testServerURL+"/internal/registry/services", nil)
	httpReq3.Header.Set("X-Service-Token", serviceToken)

	resp3, _ := client.Do(httpReq3)
	defer resp3.Body.Close()

	var services map[string][]interface{}
	json.NewDecoder(resp3.Body).Decode(&services)

	if instances, ok := services["multi-instance-service"]; !ok || len(instances) != 2 {
		t.Errorf("expected 2 instances of multi-instance-service, got %d", len(instances))
	}
}

func TestRegistry_ServiceExpiration(t *testing.T) {
	t.Skip("Skipping TTL test as it requires waiting for heartbeat TTL to expire")
}

func registerTestService(t *testing.T, name string, port int, basePath string) string {
	t.Helper()
	client := getHTTPClient()

	req := RegisterRequest{
		ServiceName: name,
		Host:        "localhost",
		Port:        port,
		HealthURL:   "/health",
		BasePath:    basePath,
		Routes: []Route{
			{Method: "GET", Path: "/test", Public: true},
		},
	}

	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", testServerURL+"/internal/registry/register", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Service-Token", serviceToken)

	resp, err := client.Do(httpReq)
	if err != nil {
		t.Fatalf("failed to register test service: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("failed to register test service, status %d: %s", resp.StatusCode, string(respBody))
	}

	var registerResp RegisterResponse
	json.NewDecoder(resp.Body).Decode(&registerResp)
	return registerResp.InstanceID
}

func cleanupService(t *testing.T, instanceID string) {
	t.Helper()
	client := getHTTPClient()

	deregisterReq := DeregisterRequest{InstanceID: instanceID}
	body, _ := json.Marshal(deregisterReq)
	httpReq, _ := http.NewRequest("POST", testServerURL+"/internal/registry/deregister", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Service-Token", serviceToken)

	resp, err := client.Do(httpReq)
	if err != nil {
		return
	}
	resp.Body.Close()
}

func keepAlive(t *testing.T, instanceID string, duration time.Duration) {
	t.Helper()
	client := getHTTPClient()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	done := time.After(duration)
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			heartbeatReq := HeartbeatRequest{InstanceID: instanceID}
			body, _ := json.Marshal(heartbeatReq)
			httpReq, _ := http.NewRequest("POST", testServerURL+"/internal/registry/heartbeat", bytes.NewReader(body))
			httpReq.Header.Set("Content-Type", "application/json")
			httpReq.Header.Set("X-Service-Token", serviceToken)

			resp, err := client.Do(httpReq)
			if err != nil {
				t.Logf("heartbeat failed: %v", err)
				return
			}
			resp.Body.Close()
		}
	}
}
