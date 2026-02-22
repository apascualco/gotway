package proxy

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/apascualco/gotway/internal/application"
	"github.com/apascualco/gotway/internal/domain"
	"github.com/apascualco/gotway/internal/infrastructure/http/middleware"
	"github.com/apascualco/gotway/internal/infrastructure/jwt"
	"github.com/gin-gonic/gin"
	gojwt "github.com/golang-jwt/jwt/v5"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupProxyTestServer() (*application.Registry, *httptest.Server) {
	registry := application.NewRegistry(application.RegistryConfig{
		HeartbeatTTL: 30 * time.Second,
	})

	lb := application.NewRoundRobinBalancer()
	proxyHandler := NewProxyHandler(registry, lb, nil)

	router := gin.New()
	router.NoRoute(proxyHandler.Handle)

	server := httptest.NewServer(router)
	return registry, server
}

func parseHostPort(url string) (string, int) {
	// url format: http://127.0.0.1:port
	hostPort := url[7:] // remove "http://"
	var host string
	var port int
	for i := len(hostPort) - 1; i >= 0; i-- {
		if hostPort[i] == ':' {
			host = hostPort[:i]
			for j := i + 1; j < len(hostPort); j++ {
				port = port*10 + int(hostPort[j]-'0')
			}
			break
		}
	}
	return host, port
}

func TestProxy_ForwardsRequest(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"message":    "hello from backend",
			"method":     r.Method,
			"path":       r.URL.Path,
			"query":      r.URL.RawQuery,
			"user_agent": r.UserAgent(),
		})
	}))
	defer backend.Close()

	registry, gateway := setupProxyTestServer()
	defer gateway.Close()

	host, port := parseHostPort(backend.URL)

	_, _ = registry.Register(&domain.RegisterRequest{
		ServiceName: "test-service",
		Host:        host,
		Port:        port,
		BasePath:    "/api/v1",
		Routes: []domain.Route{
			{Method: "GET", Path: "/users"},
			{Method: "POST", Path: "/users"},
		},
	})

	resp, err := http.Get(gateway.URL + "/api/v1/users?page=1")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if body["message"] != "hello from backend" {
		t.Errorf("expected message from backend, got %v", body["message"])
	}
	if body["method"] != "GET" {
		t.Errorf("expected GET method, got %v", body["method"])
	}
	if body["path"] != "/api/v1/users" {
		t.Errorf("expected path /api/v1/users, got %v", body["path"])
	}
	if body["query"] != "page=1" {
		t.Errorf("expected query page=1, got %v", body["query"])
	}
}

func TestProxy_NoRoute(t *testing.T) {
	_, gateway := setupProxyTestServer()
	defer gateway.Close()

	resp, err := http.Get(gateway.URL + "/api/v1/unknown")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if body["error"] != "route_not_found" {
		t.Errorf("expected error route_not_found, got %v", body["error"])
	}
}

func TestProxy_NoHealthyInstances(t *testing.T) {
	registry, gateway := setupProxyTestServer()
	defer gateway.Close()

	_, _ = registry.Register(&domain.RegisterRequest{
		ServiceName: "unhealthy-service",
		Host:        "localhost",
		Port:        9999,
		BasePath:    "/api/v1",
		Routes: []domain.Route{
			{Method: "GET", Path: "/health"},
		},
	})

	instances := registry.GetInstances("unhealthy-service")
	for _, inst := range instances {
		inst.Status = domain.StatusUnhealthy
	}

	resp, err := http.Get(gateway.URL + "/api/v1/health")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusServiceUnavailable {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected status 503, got %d: %s", resp.StatusCode, string(body))
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if body["error"] != "service_unavailable" {
		t.Errorf("expected error service_unavailable, got %v", body["error"])
	}
}

func TestProxy_UpstreamError(t *testing.T) {
	registry, gateway := setupProxyTestServer()
	defer gateway.Close()

	_, _ = registry.Register(&domain.RegisterRequest{
		ServiceName: "dead-service",
		Host:        "localhost",
		Port:        59999,
		BasePath:    "/api/v1",
		Routes: []domain.Route{
			{Method: "GET", Path: "/dead"},
		},
	})

	resp, err := http.Get(gateway.URL + "/api/v1/dead")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadGateway {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected status 502, got %d: %s", resp.StatusCode, string(body))
	}
}

func TestProxy_PostRequest(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"method":       r.Method,
			"content_type": r.Header.Get("Content-Type"),
			"body":         string(body),
		})
	}))
	defer backend.Close()

	registry, gateway := setupProxyTestServer()
	defer gateway.Close()

	host, port := parseHostPort(backend.URL)

	_, _ = registry.Register(&domain.RegisterRequest{
		ServiceName: "test-service",
		Host:        host,
		Port:        port,
		BasePath:    "/api/v1",
		Routes: []domain.Route{
			{Method: "POST", Path: "/users"},
		},
	})

	resp, err := http.Post(gateway.URL+"/api/v1/users", "application/json", nil)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected status 201, got %d: %s", resp.StatusCode, string(body))
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if body["method"] != "POST" {
		t.Errorf("expected POST method, got %v", body["method"])
	}
}

func TestProxy_ForwardsHeaders(t *testing.T) {
	var receivedHeaders http.Header
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
		})
	}))
	defer backend.Close()

	registry, gateway := setupProxyTestServer()
	defer gateway.Close()

	host, port := parseHostPort(backend.URL)

	_, _ = registry.Register(&domain.RegisterRequest{
		ServiceName: "header-test-service",
		Host:        host,
		Port:        port,
		BasePath:    "/api/v1",
		Routes: []domain.Route{
			{Method: "GET", Path: "/headers"},
		},
	})

	req, _ := http.NewRequest("GET", gateway.URL+"/api/v1/headers", nil)
	req.Header.Set("X-Request-ID", "test-request-123")
	req.Header.Set("X-Custom-Header", "custom-value")
	req.Header.Set("Connection", "keep-alive")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	if receivedHeaders.Get("X-Forwarded-For") == "" {
		t.Error("expected X-Forwarded-For header")
	}

	if receivedHeaders.Get("X-Forwarded-Host") == "" {
		t.Error("expected X-Forwarded-Host header")
	}

	if receivedHeaders.Get("X-Forwarded-Proto") == "" {
		t.Error("expected X-Forwarded-Proto header")
	}

	if receivedHeaders.Get("X-Request-ID") != "test-request-123" {
		t.Errorf("expected X-Request-ID to be test-request-123, got %s", receivedHeaders.Get("X-Request-ID"))
	}

	if receivedHeaders.Get("X-Forwarded-Service") != "header-test-service" {
		t.Errorf("expected X-Forwarded-Service to be header-test-service, got %s", receivedHeaders.Get("X-Forwarded-Service"))
	}

	if receivedHeaders.Get("X-Custom-Header") != "custom-value" {
		t.Errorf("expected X-Custom-Header to be preserved, got %s", receivedHeaders.Get("X-Custom-Header"))
	}
}

func TestProxy_RemovesHopByHopHeaders(t *testing.T) {
	var receivedHeaders http.Header
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	registry, gateway := setupProxyTestServer()
	defer gateway.Close()

	host, port := parseHostPort(backend.URL)

	_, _ = registry.Register(&domain.RegisterRequest{
		ServiceName: "hop-test-service",
		Host:        host,
		Port:        port,
		BasePath:    "/api/v1",
		Routes: []domain.Route{
			{Method: "GET", Path: "/hop"},
		},
	})

	req, _ := http.NewRequest("GET", gateway.URL+"/api/v1/hop", nil)
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Keep-Alive", "timeout=5")
	req.Header.Set("Proxy-Authorization", "Basic xyz")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	hopHeaders := []string{"Connection", "Keep-Alive", "Proxy-Authorization"}
	for _, h := range hopHeaders {
		if receivedHeaders.Get(h) != "" {
			t.Errorf("hop-by-hop header %s should be removed, got %s", h, receivedHeaders.Get(h))
		}
	}
}

func setupProxyTestServerWithAuth(privateKey *rsa.PrivateKey) (*application.Registry, *httptest.Server, *jwt.Service) {
	registry := application.NewRegistry(application.RegistryConfig{
		HeartbeatTTL: 30 * time.Second,
	})

	jwtService := jwt.NewServiceWithKeys(
		privateKey,
		&privateKey.PublicKey,
		"api-api",
		5*time.Minute,
		[]string{"auth-service", "api-api"},
	)
	authMiddleware := middleware.NewAuthMiddleware(jwtService)

	lb := application.NewRoundRobinBalancer()
	proxyHandler := NewProxyHandler(registry, lb, authMiddleware)

	router := gin.New()
	router.NoRoute(proxyHandler.Handle)

	server := httptest.NewServer(router)
	return registry, server, jwtService
}

func generateTestToken(t *testing.T, privateKey *rsa.PrivateKey, subject, email string, scopes []string) string {
	now := time.Now()
	claims := gojwt.MapClaims{
		"sub":    subject,
		"email":  email,
		"scopes": scopes,
		"iss":    "auth-service",
		"iat":    now.Unix(),
		"exp":    now.Add(1 * time.Hour).Unix(),
	}

	token := gojwt.NewWithClaims(gojwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return tokenString
}

func TestProxy_PublicRouteNoAuth(t *testing.T) {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer backend.Close()

	registry, gateway, _ := setupProxyTestServerWithAuth(privateKey)
	defer gateway.Close()

	host, port := parseHostPort(backend.URL)

	_, _ = registry.Register(&domain.RegisterRequest{
		ServiceName: "public-service",
		Host:        host,
		Port:        port,
		BasePath:    "/api/v1",
		Routes: []domain.Route{
			{Method: "GET", Path: "/public", Public: true},
		},
	})

	resp, err := http.Get(gateway.URL + "/api/v1/public")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected status 200, got %d: %s", resp.StatusCode, string(body))
	}
}

func TestProxy_ProtectedRouteNoToken(t *testing.T) {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	registry, gateway, _ := setupProxyTestServerWithAuth(privateKey)
	defer gateway.Close()

	host, port := parseHostPort(backend.URL)

	_, _ = registry.Register(&domain.RegisterRequest{
		ServiceName: "protected-service",
		Host:        host,
		Port:        port,
		BasePath:    "/api/v1",
		Routes: []domain.Route{
			{Method: "GET", Path: "/protected", Public: false},
		},
	})

	resp, err := http.Get(gateway.URL + "/api/v1/protected")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected status 401, got %d: %s", resp.StatusCode, string(body))
	}
}

func TestProxy_ProtectedRouteWithValidToken(t *testing.T) {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	var receivedAuthHeader string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "authenticated"})
	}))
	defer backend.Close()

	registry, gateway, _ := setupProxyTestServerWithAuth(privateKey)
	defer gateway.Close()

	host, port := parseHostPort(backend.URL)

	_, _ = registry.Register(&domain.RegisterRequest{
		ServiceName: "auth-test-service",
		Host:        host,
		Port:        port,
		BasePath:    "/api/v1",
		Routes: []domain.Route{
			{Method: "GET", Path: "/me", Public: false},
		},
	})

	token := generateTestToken(t, privateKey, "user-123", "user@example.com", []string{"read", "write"})

	req, _ := http.NewRequest("GET", gateway.URL+"/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	if receivedAuthHeader == "" {
		t.Error("backend should receive Authorization header with internal token")
	}
	if receivedAuthHeader == "Bearer "+token {
		t.Error("backend should receive a NEW internal token, not the original")
	}
}

func TestProxy_ProtectedRouteInsufficientScopes(t *testing.T) {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	registry, gateway, _ := setupProxyTestServerWithAuth(privateKey)
	defer gateway.Close()

	host, port := parseHostPort(backend.URL)

	_, _ = registry.Register(&domain.RegisterRequest{
		ServiceName: "admin-service",
		Host:        host,
		Port:        port,
		BasePath:    "/api/v1",
		Routes: []domain.Route{
			{Method: "DELETE", Path: "/users/:id", Public: false, Scopes: []string{"admin", "delete"}},
		},
	})

	token := generateTestToken(t, privateKey, "user-123", "user@example.com", []string{"read", "write"})

	req, _ := http.NewRequest("DELETE", gateway.URL+"/api/v1/users/456", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected status 403, got %d: %s", resp.StatusCode, string(body))
	}
}
