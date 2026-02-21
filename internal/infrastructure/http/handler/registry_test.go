package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/apascualco/gotway/internal/application"
	"github.com/apascualco/gotway/internal/domain"
	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestRouter(registry *application.Registry) *gin.Engine {
	router := gin.New()
	handler := NewRegistryHandler(registry)
	router.POST("/internal/registry/register", handler.Register)
	router.POST("/internal/registry/heartbeat", handler.Heartbeat)
	router.POST("/internal/registry/deregister", handler.Deregister)
	router.GET("/internal/registry/services", handler.ListServices)
	return router
}

func TestRegister_Success(t *testing.T) {
	registry := application.NewRegistry(application.RegistryConfig{
		ServiceToken: "test-token",
		HeartbeatTTL: 30 * time.Second,
	})
	router := setupTestRouter(registry)

	body := domain.RegisterRequest{
		ServiceName: "test-service",
		Host:        "localhost",
		Port:        8081,
		BasePath:    "/api/v1",
		Routes: []domain.Route{
			{Method: "GET", Path: "/users"},
		},
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/internal/registry/register", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderServiceToken, "test-token")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", resp.Code, resp.Body.String())
	}

	var response domain.RegisterResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.InstanceID == "" {
		t.Error("expected instance_id in response")
	}
	if response.HeartbeatInterval != 30 {
		t.Errorf("expected heartbeat_interval 30, got %d", response.HeartbeatInterval)
	}
}

func TestRegister_Unauthorized(t *testing.T) {
	registry := application.NewRegistry(application.RegistryConfig{
		ServiceToken: "test-token",
	})
	router := setupTestRouter(registry)

	body := domain.RegisterRequest{
		ServiceName: "test-service",
		Host:        "localhost",
		Port:        8081,
		BasePath:    "/api/v1",
		Routes: []domain.Route{
			{Method: "GET", Path: "/users"},
		},
	}
	jsonBody, _ := json.Marshal(body)

	tests := []struct {
		name  string
		token string
	}{
		{"no token", ""},
		{"wrong token", "wrong-token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("POST", "/internal/registry/register", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			if tt.token != "" {
				req.Header.Set(HeaderServiceToken, tt.token)
			}

			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != http.StatusUnauthorized {
				t.Errorf("expected status 401, got %d", resp.Code)
			}
		})
	}
}

func TestRegister_Collision(t *testing.T) {
	registry := application.NewRegistry(application.RegistryConfig{
		ServiceToken: "test-token",
	})
	router := setupTestRouter(registry)

	body1 := domain.RegisterRequest{
		ServiceName: "service-a",
		Host:        "localhost",
		Port:        8081,
		BasePath:    "/api/v1",
		Routes: []domain.Route{
			{Method: "GET", Path: "/users"},
		},
	}
	jsonBody1, _ := json.Marshal(body1)

	req1, _ := http.NewRequest("POST", "/internal/registry/register", bytes.NewBuffer(jsonBody1))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set(HeaderServiceToken, "test-token")

	resp1 := httptest.NewRecorder()
	router.ServeHTTP(resp1, req1)

	if resp1.Code != http.StatusCreated {
		t.Fatalf("first register failed: %d", resp1.Code)
	}

	body2 := domain.RegisterRequest{
		ServiceName: "service-b",
		Host:        "localhost",
		Port:        8082,
		BasePath:    "/api/v1",
		Routes: []domain.Route{
			{Method: "GET", Path: "/users"},
		},
	}
	jsonBody2, _ := json.Marshal(body2)

	req2, _ := http.NewRequest("POST", "/internal/registry/register", bytes.NewBuffer(jsonBody2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set(HeaderServiceToken, "test-token")

	resp2 := httptest.NewRecorder()
	router.ServeHTTP(resp2, req2)

	if resp2.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d: %s", resp2.Code, resp2.Body.String())
	}

	var errorResp map[string]interface{}
	if err := json.Unmarshal(resp2.Body.Bytes(), &errorResp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}

	if errorResp["error"] != "route_collision" {
		t.Errorf("expected error route_collision, got %v", errorResp["error"])
	}
	if errorResp["collisions"] == nil {
		t.Error("expected collisions in response")
	}
}

func TestRegister_InvalidRequest(t *testing.T) {
	registry := application.NewRegistry(application.RegistryConfig{
		ServiceToken: "test-token",
	})
	router := setupTestRouter(registry)

	body := domain.RegisterRequest{
		Host:     "localhost",
		Port:     8081,
		BasePath: "/api/v1",
		Routes: []domain.Route{
			{Method: "GET", Path: "/users"},
		},
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/internal/registry/register", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderServiceToken, "test-token")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestRegister_InvalidJSON(t *testing.T) {
	registry := application.NewRegistry(application.RegistryConfig{
		ServiceToken: "test-token",
	})
	router := setupTestRouter(registry)

	req, _ := http.NewRequest("POST", "/internal/registry/register", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderServiceToken, "test-token")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.Code)
	}
}

func TestHeartbeat_Success(t *testing.T) {
	registry := application.NewRegistry(application.RegistryConfig{
		ServiceToken: "test-token",
		HeartbeatTTL: 30 * time.Second,
	})
	router := setupTestRouter(registry)

	registerBody := domain.RegisterRequest{
		ServiceName: "test-service",
		Host:        "localhost",
		Port:        8081,
		BasePath:    "/api/v1",
		Routes: []domain.Route{
			{Method: "GET", Path: "/users"},
		},
	}
	jsonBody, _ := json.Marshal(registerBody)

	req, _ := http.NewRequest("POST", "/internal/registry/register", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderServiceToken, "test-token")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	var registerResp domain.RegisterResponse
	json.Unmarshal(resp.Body.Bytes(), &registerResp)

	heartbeatBody := domain.HeartbeatRequest{
		InstanceID: registerResp.InstanceID,
	}
	jsonBody, _ = json.Marshal(heartbeatBody)

	req, _ = http.NewRequest("POST", "/internal/registry/heartbeat", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderServiceToken, "test-token")

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var heartbeatResp domain.HeartbeatResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &heartbeatResp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if heartbeatResp.Status != "ok" {
		t.Errorf("expected status 'ok', got %s", heartbeatResp.Status)
	}
}

func TestHeartbeat_Unauthorized(t *testing.T) {
	registry := application.NewRegistry(application.RegistryConfig{
		ServiceToken: "test-token",
	})
	router := setupTestRouter(registry)

	body := domain.HeartbeatRequest{
		InstanceID: "some-instance-id",
	}
	jsonBody, _ := json.Marshal(body)

	tests := []struct {
		name  string
		token string
	}{
		{"no token", ""},
		{"wrong token", "wrong-token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("POST", "/internal/registry/heartbeat", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			if tt.token != "" {
				req.Header.Set(HeaderServiceToken, tt.token)
			}

			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != http.StatusUnauthorized {
				t.Errorf("expected status 401, got %d", resp.Code)
			}
		})
	}
}

func TestHeartbeat_InstanceNotFound(t *testing.T) {
	registry := application.NewRegistry(application.RegistryConfig{
		ServiceToken: "test-token",
	})
	router := setupTestRouter(registry)

	body := domain.HeartbeatRequest{
		InstanceID: "non-existent-instance",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/internal/registry/heartbeat", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderServiceToken, "test-token")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", resp.Code, resp.Body.String())
	}

	var errorResp map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &errorResp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}

	if errorResp["error"] != "instance_not_found" {
		t.Errorf("expected error instance_not_found, got %v", errorResp["error"])
	}
}

func TestHeartbeat_InvalidJSON(t *testing.T) {
	registry := application.NewRegistry(application.RegistryConfig{
		ServiceToken: "test-token",
	})
	router := setupTestRouter(registry)

	req, _ := http.NewRequest("POST", "/internal/registry/heartbeat", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderServiceToken, "test-token")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.Code)
	}
}

func TestDeregister_Success(t *testing.T) {
	registry := application.NewRegistry(application.RegistryConfig{
		ServiceToken: "test-token",
		HeartbeatTTL: 30 * time.Second,
	})
	router := setupTestRouter(registry)

	registerBody := domain.RegisterRequest{
		ServiceName: "test-service",
		Host:        "localhost",
		Port:        8081,
		BasePath:    "/api/v1",
		Routes: []domain.Route{
			{Method: "GET", Path: "/users"},
		},
	}
	jsonBody, _ := json.Marshal(registerBody)

	req, _ := http.NewRequest("POST", "/internal/registry/register", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderServiceToken, "test-token")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	var registerResp domain.RegisterResponse
	json.Unmarshal(resp.Body.Bytes(), &registerResp)

	deregisterBody := domain.DeregisterRequest{
		InstanceID: registerResp.InstanceID,
	}
	jsonBody, _ = json.Marshal(deregisterBody)

	req, _ = http.NewRequest("POST", "/internal/registry/deregister", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderServiceToken, "test-token")

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var deregisterResp map[string]string
	if err := json.Unmarshal(resp.Body.Bytes(), &deregisterResp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if deregisterResp["status"] != "deregistered" {
		t.Errorf("expected status 'deregistered', got %s", deregisterResp["status"])
	}
}

func TestDeregister_Unauthorized(t *testing.T) {
	registry := application.NewRegistry(application.RegistryConfig{
		ServiceToken: "test-token",
	})
	router := setupTestRouter(registry)

	body := domain.DeregisterRequest{
		InstanceID: "some-instance-id",
	}
	jsonBody, _ := json.Marshal(body)

	tests := []struct {
		name  string
		token string
	}{
		{"no token", ""},
		{"wrong token", "wrong-token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("POST", "/internal/registry/deregister", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			if tt.token != "" {
				req.Header.Set(HeaderServiceToken, tt.token)
			}

			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != http.StatusUnauthorized {
				t.Errorf("expected status 401, got %d", resp.Code)
			}
		})
	}
}

func TestDeregister_InstanceNotFound(t *testing.T) {
	registry := application.NewRegistry(application.RegistryConfig{
		ServiceToken: "test-token",
	})
	router := setupTestRouter(registry)

	body := domain.DeregisterRequest{
		InstanceID: "non-existent-instance",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/internal/registry/deregister", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderServiceToken, "test-token")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", resp.Code, resp.Body.String())
	}

	var errorResp map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &errorResp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}

	if errorResp["error"] != "instance_not_found" {
		t.Errorf("expected error instance_not_found, got %v", errorResp["error"])
	}
}

func TestListServices_Success(t *testing.T) {
	registry := application.NewRegistry(application.RegistryConfig{
		ServiceToken: "test-token",
		HeartbeatTTL: 30 * time.Second,
	})
	router := setupTestRouter(registry)

	registerBody := domain.RegisterRequest{
		ServiceName: "test-service",
		Host:        "localhost",
		Port:        8081,
		BasePath:    "/api/v1",
		Routes: []domain.Route{
			{Method: "GET", Path: "/users"},
		},
	}
	jsonBody, _ := json.Marshal(registerBody)

	req, _ := http.NewRequest("POST", "/internal/registry/register", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderServiceToken, "test-token")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	req, _ = http.NewRequest("GET", "/internal/registry/services", nil)
	req.Header.Set(HeaderServiceToken, "test-token")

	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var listResp map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	services, ok := listResp["services"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected services map in response")
	}

	if _, exists := services["test-service"]; !exists {
		t.Error("expected test-service in services list")
	}
}

func TestListServices_Empty(t *testing.T) {
	registry := application.NewRegistry(application.RegistryConfig{
		ServiceToken: "test-token",
	})
	router := setupTestRouter(registry)

	req, _ := http.NewRequest("GET", "/internal/registry/services", nil)
	req.Header.Set(HeaderServiceToken, "test-token")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.Code)
	}
}

func TestListServices_Unauthorized(t *testing.T) {
	registry := application.NewRegistry(application.RegistryConfig{
		ServiceToken: "test-token",
	})
	router := setupTestRouter(registry)

	tests := []struct {
		name  string
		token string
	}{
		{"no token", ""},
		{"wrong token", "wrong-token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/internal/registry/services", nil)
			if tt.token != "" {
				req.Header.Set(HeaderServiceToken, tt.token)
			}

			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != http.StatusUnauthorized {
				t.Errorf("expected status 401, got %d", resp.Code)
			}
		})
	}
}
