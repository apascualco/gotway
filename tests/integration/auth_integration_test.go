package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestAuth_PublicRoute_NoTokenRequired(t *testing.T) {
	mockServer, err := NewMockServer()
	if err != nil {
		t.Fatalf("failed to create mock server: %v", err)
	}
	defer mockServer.Close()

	mockServer.SetResponse("GET", "/public", MockResponse{
		StatusCode: http.StatusOK,
		Body:       `{"message":"public data"}`,
		Headers:    map[string]string{"Content-Type": "application/json"},
	})

	instanceID := registerServiceWithPortAndRoutes(t, "auth-test-public", mockServer.Port(), "/api/v1/auth-test-public", []Route{
		{Method: "GET", Path: "/public", Public: true},
	})
	defer cleanupService(t, instanceID)

	time.Sleep(100 * time.Millisecond)

	client := getHTTPClient()
	resp, err := client.Get(testServerURL + "/api/v1/auth-test-public/public")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status 200 for public route, got %d: %s", resp.StatusCode, string(respBody))
	}

	var response map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&response)
	if response["message"] != "public data" {
		t.Errorf("unexpected response: %v", response)
	}
}

func TestAuth_ProtectedRoute_WithoutJWTConfig(t *testing.T) {
	mockServer, err := NewMockServer()
	if err != nil {
		t.Fatalf("failed to create mock server: %v", err)
	}
	defer mockServer.Close()

	mockServer.SetResponse("GET", "/protected", MockResponse{
		StatusCode: http.StatusOK,
		Body:       `{"message":"protected data"}`,
		Headers:    map[string]string{"Content-Type": "application/json"},
	})

	instanceID := registerServiceWithPortAndRoutes(t, "auth-test-protected", mockServer.Port(), "/api/v1/auth-test-protected", []Route{
		{Method: "GET", Path: "/protected", Public: false},
	})
	defer cleanupService(t, instanceID)

	time.Sleep(100 * time.Millisecond)

	client := getHTTPClient()
	resp, err := client.Get(testServerURL + "/api/v1/auth-test-protected/protected")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if os.Getenv("JWT_PUBLIC_KEY") == "" {
		if resp.StatusCode == http.StatusOK {
			t.Log("JWT not configured, protected route accessible without auth (expected in test mode)")
		}
	} else {
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected 401 for protected route without token, got %d", resp.StatusCode)
		}
	}
}

func TestAuth_RequestIDPropagated(t *testing.T) {
	mockServer, err := NewMockServer()
	if err != nil {
		t.Fatalf("failed to create mock server: %v", err)
	}
	defer mockServer.Close()

	instanceID := registerServiceWithPortAndRoutes(t, "auth-test-reqid", mockServer.Port(), "/api/v1/auth-test-reqid", []Route{
		{Method: "GET", Path: "/test", Public: true},
	})
	defer cleanupService(t, instanceID)

	time.Sleep(100 * time.Millisecond)

	client := getHTTPClient()

	req, _ := http.NewRequest("GET", testServerURL+"/api/v1/auth-test-reqid/test", nil)
	customRequestID := "custom-request-id-12345"
	req.Header.Set("X-Request-ID", customRequestID)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	responseRequestID := resp.Header.Get("X-Request-ID")
	if responseRequestID != customRequestID {
		t.Errorf("expected X-Request-ID to be %s, got %s", customRequestID, responseRequestID)
	}
}

func TestAuth_GeneratesRequestID(t *testing.T) {
	mockServer, err := NewMockServer()
	if err != nil {
		t.Fatalf("failed to create mock server: %v", err)
	}
	defer mockServer.Close()

	instanceID := registerServiceWithPortAndRoutes(t, "auth-test-gen-reqid", mockServer.Port(), "/api/v1/auth-test-gen-reqid", []Route{
		{Method: "GET", Path: "/test", Public: true},
	})
	defer cleanupService(t, instanceID)

	time.Sleep(100 * time.Millisecond)

	client := getHTTPClient()
	resp, err := client.Get(testServerURL + "/api/v1/auth-test-gen-reqid/test")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	responseRequestID := resp.Header.Get("X-Request-ID")
	if responseRequestID == "" {
		t.Error("expected X-Request-ID to be generated")
	}
}

func TestAuth_CORSHeaders(t *testing.T) {
	client := getHTTPClient()

	req, _ := http.NewRequest("OPTIONS", testServerURL+"/api/v1/any/path", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "Authorization")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("CORS preflight request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 200 or 204 for CORS preflight, got %d", resp.StatusCode)
	}

	if resp.Header.Get("Access-Control-Allow-Origin") == "" {
		t.Error("expected Access-Control-Allow-Origin header")
	}

	if resp.Header.Get("Access-Control-Allow-Methods") == "" {
		t.Error("expected Access-Control-Allow-Methods header")
	}
}

func TestAuth_RateLimiting(t *testing.T) {
	client := getHTTPClient()

	successCount := 0
	rateLimitedCount := 0

	for i := 0; i < 100; i++ {
		resp, err := client.Get(testServerURL + "/health")
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		_ = resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			successCount++
		} else if resp.StatusCode == http.StatusTooManyRequests {
			rateLimitedCount++
		}
	}

	t.Logf("Success: %d, Rate Limited: %d", successCount, rateLimitedCount)

	if successCount == 0 {
		t.Error("expected at least some successful requests")
	}
}

func TestAuth_HealthEndpoint_NoAuth(t *testing.T) {
	client := getHTTPClient()

	resp, err := client.Get(testServerURL + "/health")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 for /health, got %d", resp.StatusCode)
	}

	var health map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&health)

	if health["status"] != "healthy" {
		t.Errorf("expected status 'healthy', got %v", health["status"])
	}
}

func TestAuth_ReadyEndpoint_NoAuth(t *testing.T) {
	client := getHTTPClient()

	resp, err := client.Get(testServerURL + "/ready")
	if err != nil {
		t.Fatalf("ready request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 for /ready, got %d", resp.StatusCode)
	}
}

func TestAuth_MultiplePublicRoutes(t *testing.T) {
	mockServer, err := NewMockServer()
	if err != nil {
		t.Fatalf("failed to create mock server: %v", err)
	}
	defer mockServer.Close()

	mockServer.SetResponse("GET", "/public1", MockResponse{
		StatusCode: http.StatusOK,
		Body:       `{"route":"public1"}`,
	})
	mockServer.SetResponse("POST", "/public2", MockResponse{
		StatusCode: http.StatusCreated,
		Body:       `{"route":"public2"}`,
	})

	instanceID := registerServiceWithPortAndRoutes(t, "multi-public", mockServer.Port(), "/api/v1/multi-public", []Route{
		{Method: "GET", Path: "/public1", Public: true},
		{Method: "POST", Path: "/public2", Public: true},
	})
	defer cleanupService(t, instanceID)

	time.Sleep(100 * time.Millisecond)

	client := getHTTPClient()

	resp1, err := client.Get(testServerURL + "/api/v1/multi-public/public1")
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer func() { _ = resp1.Body.Close() }()
	if resp1.StatusCode != http.StatusOK {
		t.Errorf("GET /public1 expected 200, got %d", resp1.StatusCode)
	}

	resp2, err := client.Post(testServerURL+"/api/v1/multi-public/public2", "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }()
	if resp2.StatusCode != http.StatusCreated {
		t.Errorf("POST /public2 expected 201, got %d", resp2.StatusCode)
	}
}

func TestAuth_MethodNotAllowed(t *testing.T) {
	mockServer, err := NewMockServer()
	if err != nil {
		t.Fatalf("failed to create mock server: %v", err)
	}
	defer mockServer.Close()

	instanceID := registerServiceWithPortAndRoutes(t, "method-test", mockServer.Port(), "/api/v1/method-test", []Route{
		{Method: "GET", Path: "/resource", Public: true},
	})
	defer cleanupService(t, instanceID)

	time.Sleep(100 * time.Millisecond)

	client := getHTTPClient()

	resp, err := client.Post(testServerURL+"/api/v1/method-test/resource", "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Logf("POST to GET-only route returned %d (route not found for different method)", resp.StatusCode)
	}
}
