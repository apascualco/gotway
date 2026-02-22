package client

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

func generateTestKeyPEM() (string, *rsa.PublicKey) {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	privBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privBytes,
	})

	return string(privPEM), &privateKey.PublicKey
}

func validateServiceJWT(t *testing.T, tokenString string, publicKey *rsa.PublicKey) string {
	t.Helper()
	token, err := jwtlib.Parse(tokenString, func(token *jwtlib.Token) (interface{}, error) {
		return publicKey, nil
	})
	if err != nil {
		t.Fatalf("failed to parse JWT: %v", err)
	}
	claims := token.Claims.(jwtlib.MapClaims)
	sub, _ := claims["sub"].(string)
	return sub
}

func TestNewRegistryClient(t *testing.T) {
	privPEM, _ := generateTestKeyPEM()

	client, err := NewRegistryClient("http://localhost:8080", privPEM, "test-service")
	if err != nil {
		t.Fatalf("NewRegistryClient() error = %v", err)
	}

	if client.gatewayURL != "http://localhost:8080" {
		t.Errorf("gatewayURL = %s, want http://localhost:8080", client.gatewayURL)
	}

	if client.serviceName != "test-service" {
		t.Errorf("serviceName = %s, want test-service", client.serviceName)
	}

	if client.privateKey == nil {
		t.Error("privateKey should not be nil")
	}

	if client.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
}

func TestNewRegistryClient_InvalidKey(t *testing.T) {
	_, err := NewRegistryClient("http://localhost:8080", "invalid-key", "test-service")
	if err == nil {
		t.Error("expected error for invalid key")
	}
}

func TestRegister_Success(t *testing.T) {
	privPEM, publicKey := generateTestKeyPEM()
	expectedInstanceID := "instance-123"
	expectedHeartbeatInterval := 10

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/registry/register":
			if r.Method != http.MethodPost {
				t.Errorf("unexpected method: %s", r.Method)
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			token := r.Header.Get("X-Service-Token")
			sub := validateServiceJWT(t, token, publicKey)
			if sub != "test-service" {
				t.Errorf("unexpected sub in JWT: %s", sub)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			var req RegisterRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("failed to decode request: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			if req.ServiceName != "test-service" {
				t.Errorf("unexpected service_name: %s", req.ServiceName)
			}

			resp := RegisterResponse{
				InstanceID:        expectedInstanceID,
				HeartbeatInterval: expectedHeartbeatInterval,
				HeartbeatURL:      "/internal/registry/heartbeat",
				RegisteredRoutes:  []string{"GET:/api/v1/test"},
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(resp)

		case "/internal/registry/deregister":
			w.WriteHeader(http.StatusOK)

		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	client, err := NewRegistryClient(server.URL, privPEM, "test-service")
	if err != nil {
		t.Fatalf("NewRegistryClient() error = %v", err)
	}
	defer func() { _ = client.Shutdown(context.Background()) }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Register(ctx, RegisterRequest{
		ServiceName: "test-service",
		Host:        "localhost",
		Port:        8081,
		BasePath:    "/api/v1",
		Routes: []Route{
			{Method: "GET", Path: "/test", Public: true},
		},
	})

	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if resp.InstanceID != expectedInstanceID {
		t.Errorf("InstanceID = %s, want %s", resp.InstanceID, expectedInstanceID)
	}

	if resp.HeartbeatInterval != expectedHeartbeatInterval {
		t.Errorf("HeartbeatInterval = %d, want %d", resp.HeartbeatInterval, expectedHeartbeatInterval)
	}

	if client.InstanceID() != expectedInstanceID {
		t.Errorf("client.InstanceID() = %s, want %s", client.InstanceID(), expectedInstanceID)
	}
}

func TestRegister_Collision(t *testing.T) {
	privPEM, _ := generateTestKeyPEM()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"error":   "route_collision",
			"message": "one or more routes are already registered",
			"collisions": []RouteCollision{
				{
					Method:        "GET",
					Path:          "/api/v1/test",
					CollisionType: "exact",
					RegisteredBy:  "other-service",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewRegistryClient(server.URL, privPEM, "test-service")
	if err != nil {
		t.Fatalf("NewRegistryClient() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.Register(ctx, RegisterRequest{
		ServiceName: "test-service",
		Host:        "localhost",
		Port:        8081,
		BasePath:    "/api/v1",
		Routes: []Route{
			{Method: "GET", Path: "/test", Public: true},
		},
	})

	if err == nil {
		t.Fatal("expected collision error, got nil")
	}

	collisionErr, ok := err.(*CollisionError)
	if !ok {
		t.Fatalf("expected *CollisionError, got %T", err)
	}

	if len(collisionErr.Collisions) != 1 {
		t.Errorf("expected 1 collision, got %d", len(collisionErr.Collisions))
	}

	if collisionErr.Collisions[0].RegisteredBy != "other-service" {
		t.Errorf("RegisteredBy = %s, want other-service", collisionErr.Collisions[0].RegisteredBy)
	}
}

func TestHeartbeat_Sends(t *testing.T) {
	privPEM, _ := generateTestKeyPEM()
	var heartbeatCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/registry/register":
			resp := RegisterResponse{
				InstanceID:        "instance-123",
				HeartbeatInterval: 1,
				HeartbeatURL:      "/internal/registry/heartbeat",
				RegisteredRoutes:  []string{"GET:/api/v1/test"},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(resp)

		case "/internal/registry/heartbeat":
			atomic.AddInt32(&heartbeatCount, 1)

			var req map[string]string
			_ = json.NewDecoder(r.Body).Decode(&req)

			if req["instance_id"] != "instance-123" {
				t.Errorf("unexpected instance_id: %s", req["instance_id"])
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

		case "/internal/registry/deregister":
			w.WriteHeader(http.StatusOK)

		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewRegistryClient(server.URL, privPEM, "test-service")
	if err != nil {
		t.Fatalf("NewRegistryClient() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.Register(ctx, RegisterRequest{
		ServiceName: "test-service",
		Host:        "localhost",
		Port:        8081,
		BasePath:    "/api/v1",
		Routes: []Route{
			{Method: "GET", Path: "/test", Public: true},
		},
	})

	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	time.Sleep(2500 * time.Millisecond)

	count := atomic.LoadInt32(&heartbeatCount)
	if count < 2 {
		t.Errorf("expected at least 2 heartbeats, got %d", count)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := client.Shutdown(shutdownCtx); err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
}

func TestShutdown_Deregisters(t *testing.T) {
	privPEM, _ := generateTestKeyPEM()
	var deregisterCalled bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/registry/register":
			resp := RegisterResponse{
				InstanceID:        "instance-123",
				HeartbeatInterval: 60,
				HeartbeatURL:      "/internal/registry/heartbeat",
				RegisteredRoutes:  []string{"GET:/api/v1/test"},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(resp)

		case "/internal/registry/deregister":
			deregisterCalled = true

			var req map[string]string
			_ = json.NewDecoder(r.Body).Decode(&req)

			if req["instance_id"] != "instance-123" {
				t.Errorf("unexpected instance_id: %s", req["instance_id"])
			}

			w.WriteHeader(http.StatusOK)

		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	client, err := NewRegistryClient(server.URL, privPEM, "test-service")
	if err != nil {
		t.Fatalf("NewRegistryClient() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.Register(ctx, RegisterRequest{
		ServiceName: "test-service",
		Host:        "localhost",
		Port:        8081,
		BasePath:    "/api/v1",
		Routes: []Route{
			{Method: "GET", Path: "/test", Public: true},
		},
	})

	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := client.Shutdown(shutdownCtx); err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}

	if !deregisterCalled {
		t.Error("expected deregister to be called")
	}
}

func TestRetry_EventuallySucceeds(t *testing.T) {
	privPEM, _ := generateTestKeyPEM()
	var attemptCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/registry/register":
			count := atomic.AddInt32(&attemptCount, 1)

			if count < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte("service unavailable"))
				return
			}

			resp := RegisterResponse{
				InstanceID:        "instance-123",
				HeartbeatInterval: 60,
				HeartbeatURL:      "/internal/registry/heartbeat",
				RegisteredRoutes:  []string{"GET:/api/v1/test"},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(resp)

		case "/internal/registry/deregister":
			w.WriteHeader(http.StatusOK)

		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	client, err := NewRegistryClient(server.URL, privPEM, "test-service")
	if err != nil {
		t.Fatalf("NewRegistryClient() error = %v", err)
	}
	defer func() { _ = client.Shutdown(context.Background()) }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Register(ctx, RegisterRequest{
		ServiceName: "test-service",
		Host:        "localhost",
		Port:        8081,
		BasePath:    "/api/v1",
		Routes: []Route{
			{Method: "GET", Path: "/test", Public: true},
		},
	})

	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if resp.InstanceID != "instance-123" {
		t.Errorf("InstanceID = %s, want instance-123", resp.InstanceID)
	}

	finalCount := atomic.LoadInt32(&attemptCount)
	if finalCount != 3 {
		t.Errorf("expected 3 attempts, got %d", finalCount)
	}
}

func TestRegister_InvalidToken(t *testing.T) {
	privPEM, _ := generateTestKeyPEM()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_token"})
	}))
	defer server.Close()

	client, err := NewRegistryClient(server.URL, privPEM, "test-service")
	if err != nil {
		t.Fatalf("NewRegistryClient() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = client.Register(ctx, RegisterRequest{
		ServiceName: "test-service",
		Host:        "localhost",
		Port:        8081,
		BasePath:    "/api/v1",
		Routes: []Route{
			{Method: "GET", Path: "/test", Public: true},
		},
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCollisionError_Error(t *testing.T) {
	err := &CollisionError{
		Collisions: []RouteCollision{
			{Method: "GET", Path: "/test", CollisionType: "exact", RegisteredBy: "other"},
			{Method: "POST", Path: "/test", CollisionType: "exact", RegisteredBy: "another"},
		},
	}

	expected := "route collision: 2 route(s) already registered"
	if err.Error() != expected {
		t.Errorf("Error() = %s, want %s", err.Error(), expected)
	}
}

func TestSendHeartbeat_ReturnsErrInstanceNotFound(t *testing.T) {
	privPEM, _ := generateTestKeyPEM()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := NewRegistryClient(server.URL, privPEM, "test-service")
	if err != nil {
		t.Fatalf("NewRegistryClient() error = %v", err)
	}
	client.instanceID = "stale-instance"

	err = client.sendHeartbeat(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, ErrInstanceNotFound) {
		t.Errorf("expected ErrInstanceNotFound, got %v", err)
	}
}

func TestHeartbeat_ReregistersOnNotFound(t *testing.T) {
	privPEM, _ := generateTestKeyPEM()

	var (
		registerCount int32
		newInstanceID = "new-instance-456"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/registry/register":
			count := atomic.AddInt32(&registerCount, 1)
			instanceID := "instance-123"
			if count > 1 {
				instanceID = newInstanceID
			}
			resp := RegisterResponse{
				InstanceID:        instanceID,
				HeartbeatInterval: 1,
				HeartbeatURL:      "/internal/registry/heartbeat",
				RegisteredRoutes:  []string{"GET:/api/v1/test"},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(resp)

		case "/internal/registry/heartbeat":
			var req map[string]string
			_ = json.NewDecoder(r.Body).Decode(&req)

			if req["instance_id"] == "instance-123" {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

		case "/internal/registry/deregister":
			w.WriteHeader(http.StatusOK)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewRegistryClient(server.URL, privPEM, "test-service")
	if err != nil {
		t.Fatalf("NewRegistryClient() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = client.Register(ctx, RegisterRequest{
		ServiceName: "test-service",
		Host:        "localhost",
		Port:        8081,
		BasePath:    "/api/v1",
		Routes: []Route{
			{Method: "GET", Path: "/test", Public: true},
		},
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if client.InstanceID() != "instance-123" {
		t.Fatalf("initial InstanceID = %s, want instance-123", client.InstanceID())
	}

	// Wait for heartbeat to fire, get 404, and re-register
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if client.InstanceID() == newInstanceID {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if client.InstanceID() != newInstanceID {
		t.Errorf("InstanceID after re-registration = %s, want %s", client.InstanceID(), newInstanceID)
	}

	regCount := atomic.LoadInt32(&registerCount)
	if regCount < 2 {
		t.Errorf("expected at least 2 registrations, got %d", regCount)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := client.Shutdown(shutdownCtx); err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
}
