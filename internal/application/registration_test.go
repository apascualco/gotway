package application

import (
	"testing"
	"time"

	"github.com/apascualco/gotway/internal/domain"
)

func TestRegister_Success(t *testing.T) {
	registry := NewRegistry(RegistryConfig{
		HeartbeatTTL: 30 * time.Second,
	})

	req := &domain.RegisterRequest{
		ServiceName: "user-service",
		Host:        "localhost",
		Port:        8081,
		BasePath:    "/api/v1",
		Routes: []domain.Route{
			{Method: "GET", Path: "/users"},
			{Method: "POST", Path: "/users"},
		},
	}

	resp, err := registry.Register(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.InstanceID == "" {
		t.Error("expected instance_id to be set")
	}
	if resp.HeartbeatInterval != 30 {
		t.Errorf("expected heartbeat_interval 30, got %d", resp.HeartbeatInterval)
	}
	if len(resp.RegisteredRoutes) != 2 {
		t.Errorf("expected 2 registered routes, got %d", len(resp.RegisteredRoutes))
	}

	if _, exists := registry.instances[resp.InstanceID]; !exists {
		t.Error("instance not stored in registry")
	}
	if len(registry.services["user-service"]) != 1 {
		t.Error("service not stored in registry")
	}
	if len(registry.routes) != 2 {
		t.Errorf("expected 2 routes, got %d", len(registry.routes))
	}
}

func TestRegister_Collision(t *testing.T) {
	registry := NewRegistry(RegistryConfig{})

	req1 := &domain.RegisterRequest{
		ServiceName: "service-a",
		Host:        "localhost",
		Port:        8081,
		BasePath:    "/api/v1",
		Routes: []domain.Route{
			{Method: "GET", Path: "/users"},
		},
	}
	_, err := registry.Register(req1)
	if err != nil {
		t.Fatalf("first register failed: %v", err)
	}

	req2 := &domain.RegisterRequest{
		ServiceName: "service-b",
		Host:        "localhost",
		Port:        8082,
		BasePath:    "/api/v1",
		Routes: []domain.Route{
			{Method: "GET", Path: "/users"},
		},
	}
	_, err = registry.Register(req2)
	if err == nil {
		t.Fatal("expected collision error")
	}

	collisionErr, ok := err.(*domain.CollisionError)
	if !ok {
		t.Fatalf("expected CollisionError, got %T", err)
	}
	if len(collisionErr.Collisions) != 1 {
		t.Errorf("expected 1 collision, got %d", len(collisionErr.Collisions))
	}
}

func TestRegister_InvalidRequest(t *testing.T) {
	registry := NewRegistry(RegistryConfig{})

	tests := []struct {
		name string
		req  *domain.RegisterRequest
	}{
		{
			name: "missing service_name",
			req: &domain.RegisterRequest{
				Host:     "localhost",
				Port:     8081,
				BasePath: "/api/v1",
				Routes:   []domain.Route{{Method: "GET", Path: "/users"}},
			},
		},
		{
			name: "missing host",
			req: &domain.RegisterRequest{
				ServiceName: "test",
				Port:        8081,
				BasePath:    "/api/v1",
				Routes:      []domain.Route{{Method: "GET", Path: "/users"}},
			},
		},
		{
			name: "invalid port",
			req: &domain.RegisterRequest{
				ServiceName: "test",
				Host:        "localhost",
				Port:        0,
				BasePath:    "/api/v1",
				Routes:      []domain.Route{{Method: "GET", Path: "/users"}},
			},
		},
		{
			name: "missing routes",
			req: &domain.RegisterRequest{
				ServiceName: "test",
				Host:        "localhost",
				Port:        8081,
				BasePath:    "/api/v1",
				Routes:      []domain.Route{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := registry.Register(tt.req)
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestDeregister_Success(t *testing.T) {
	registry := NewRegistry(RegistryConfig{})

	req := &domain.RegisterRequest{
		ServiceName: "test-service",
		Host:        "localhost",
		Port:        8081,
		BasePath:    "/api/v1",
		Routes: []domain.Route{
			{Method: "GET", Path: "/users"},
			{Method: "POST", Path: "/users"},
		},
	}
	resp, _ := registry.Register(req)

	if len(registry.instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(registry.instances))
	}
	if len(registry.routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(registry.routes))
	}

	err := registry.Deregister(resp.InstanceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(registry.instances) != 0 {
		t.Errorf("expected 0 instances, got %d", len(registry.instances))
	}
	if len(registry.routes) != 0 {
		t.Errorf("expected 0 routes, got %d", len(registry.routes))
	}
	if len(registry.services["test-service"]) != 0 {
		t.Errorf("expected 0 service entries, got %d", len(registry.services["test-service"]))
	}
}

func TestDeregister_NotFound(t *testing.T) {
	registry := NewRegistry(RegistryConfig{})

	err := registry.Deregister("non-existent-id")
	if err != domain.ErrInstanceNotFound {
		t.Errorf("expected ErrInstanceNotFound, got %v", err)
	}
}

func TestDeregister_RoutesRemoved(t *testing.T) {
	registry := NewRegistry(RegistryConfig{})

	req1 := &domain.RegisterRequest{
		ServiceName: "service-a",
		Host:        "localhost",
		Port:        8081,
		BasePath:    "/api/v1",
		Routes:      []domain.Route{{Method: "GET", Path: "/a"}},
	}
	resp1, _ := registry.Register(req1)

	req2 := &domain.RegisterRequest{
		ServiceName: "service-b",
		Host:        "localhost",
		Port:        8082,
		BasePath:    "/api/v2",
		Routes:      []domain.Route{{Method: "GET", Path: "/b"}},
	}
	_, _ = registry.Register(req2)

	if len(registry.routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(registry.routes))
	}

	_ = registry.Deregister(resp1.InstanceID)

	if len(registry.routes) != 1 {
		t.Errorf("expected 1 route after deregister, got %d", len(registry.routes))
	}
	if _, exists := registry.routes["GET:/api/v2/b"]; !exists {
		t.Error("service-b route should still exist")
	}
}
