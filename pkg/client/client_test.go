package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewRegistryClient(t *testing.T) {
	client := NewRegistryClient("http://localhost:8080", "test-token")

	if client.gatewayURL != "http://localhost:8080" {
		t.Errorf("gatewayURL = %s, want http://localhost:8080", client.gatewayURL)
	}

	if client.token != "test-token" {
		t.Errorf("token = %s, want test-token", client.token)
	}

	if client.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
}

func TestRegister_Success(t *testing.T) {
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
			if token != "test-token" {
				t.Errorf("unexpected token: %s", token)
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
			json.NewEncoder(w).Encode(resp)

		case "/internal/registry/deregister":
			w.WriteHeader(http.StatusOK)

		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	client := NewRegistryClient(server.URL, "test-token")
	defer client.Shutdown(context.Background())

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
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewRegistryClient(server.URL, "test-token")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Register(ctx, RegisterRequest{
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
			json.NewEncoder(w).Encode(resp)

		case "/internal/registry/heartbeat":
			atomic.AddInt32(&heartbeatCount, 1)

			var req map[string]string
			json.NewDecoder(r.Body).Decode(&req)

			if req["instance_id"] != "instance-123" {
				t.Errorf("unexpected instance_id: %s", req["instance_id"])
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

		case "/internal/registry/deregister":
			w.WriteHeader(http.StatusOK)

		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewRegistryClient(server.URL, "test-token")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Register(ctx, RegisterRequest{
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
			json.NewEncoder(w).Encode(resp)

		case "/internal/registry/deregister":
			deregisterCalled = true

			var req map[string]string
			json.NewDecoder(r.Body).Decode(&req)

			if req["instance_id"] != "instance-123" {
				t.Errorf("unexpected instance_id: %s", req["instance_id"])
			}

			w.WriteHeader(http.StatusOK)

		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	client := NewRegistryClient(server.URL, "test-token")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Register(ctx, RegisterRequest{
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
	var attemptCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/registry/register":
			count := atomic.AddInt32(&attemptCount, 1)

			if count < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte("service unavailable"))
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
			json.NewEncoder(w).Encode(resp)

		case "/internal/registry/deregister":
			w.WriteHeader(http.StatusOK)

		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	client := NewRegistryClient(server.URL, "test-token")
	defer client.Shutdown(context.Background())

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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid_token"})
	}))
	defer server.Close()

	client := NewRegistryClient(server.URL, "wrong-token")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := client.Register(ctx, RegisterRequest{
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
