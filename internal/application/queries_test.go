package application

import (
	"testing"

	"github.com/apascualco/gotway/internal/domain"
)

func TestGetInstance_Found(t *testing.T) {
	registry := NewRegistry(RegistryConfig{})

	req := &domain.RegisterRequest{
		ServiceName: "test-service",
		Host:        "localhost",
		Port:        8081,
		BasePath:    "/api/v1",
		Routes:      []domain.Route{{Method: "GET", Path: "/test"}},
	}
	resp, _ := registry.Register(req)

	instance := registry.GetInstance(resp.InstanceID)
	if instance == nil {
		t.Fatal("expected instance, got nil")
	}
	if instance.ServiceName != "test-service" {
		t.Errorf("expected service name test-service, got %s", instance.ServiceName)
	}
}

func TestGetInstance_NotFound(t *testing.T) {
	registry := NewRegistry(RegistryConfig{})

	instance := registry.GetInstance("non-existent-id")
	if instance != nil {
		t.Error("expected nil for non-existent instance")
	}
}

func TestGetInstances(t *testing.T) {
	registry := NewRegistry(RegistryConfig{})

	req1 := &domain.RegisterRequest{
		ServiceName: "test-service",
		Host:        "localhost",
		Port:        8081,
		BasePath:    "/api/v1",
		Routes:      []domain.Route{{Method: "GET", Path: "/test1"}},
	}
	registry.Register(req1)

	req2 := &domain.RegisterRequest{
		ServiceName: "test-service",
		Host:        "localhost",
		Port:        8082,
		BasePath:    "/api/v2",
		Routes:      []domain.Route{{Method: "GET", Path: "/test2"}},
	}
	registry.Register(req2)

	instances := registry.GetInstances("test-service")
	if len(instances) != 2 {
		t.Errorf("expected 2 instances, got %d", len(instances))
	}

	instances = registry.GetInstances("non-existent")
	if instances != nil {
		t.Errorf("expected nil for non-existent service, got %d instances", len(instances))
	}
}

func TestGetHealthyInstances(t *testing.T) {
	registry := NewRegistry(RegistryConfig{})

	req := &domain.RegisterRequest{
		ServiceName: "test-service",
		Host:        "localhost",
		Port:        8081,
		BasePath:    "/api/v1",
		Routes:      []domain.Route{{Method: "GET", Path: "/test"}},
	}
	resp, _ := registry.Register(req)

	healthy := registry.GetHealthyInstances("test-service")
	if len(healthy) != 1 {
		t.Errorf("expected 1 healthy instance, got %d", len(healthy))
	}

	registry.instances[resp.InstanceID].Status = domain.StatusUnhealthy

	healthy = registry.GetHealthyInstances("test-service")
	if len(healthy) != 0 {
		t.Errorf("expected 0 healthy instances, got %d", len(healthy))
	}
}

func TestGetRoute(t *testing.T) {
	registry := NewRegistry(RegistryConfig{})

	req := &domain.RegisterRequest{
		ServiceName: "test-service",
		Host:        "localhost",
		Port:        8081,
		BasePath:    "/api/v1",
		Routes:      []domain.Route{{Method: "GET", Path: "/users"}},
	}
	registry.Register(req)

	route := registry.GetRoute("GET", "/api/v1/users")
	if route == nil {
		t.Fatal("expected route, got nil")
	}
	if route.ServiceName != "test-service" {
		t.Errorf("expected service name test-service, got %s", route.ServiceName)
	}

	route = registry.GetRoute("POST", "/api/v1/users")
	if route != nil {
		t.Error("expected nil for non-existent route")
	}
}

func TestGetAllServices(t *testing.T) {
	registry := NewRegistry(RegistryConfig{})

	req1 := &domain.RegisterRequest{
		ServiceName: "service-a",
		Host:        "localhost",
		Port:        8081,
		BasePath:    "/api/v1",
		Routes:      []domain.Route{{Method: "GET", Path: "/a"}},
	}
	registry.Register(req1)

	req2 := &domain.RegisterRequest{
		ServiceName: "service-b",
		Host:        "localhost",
		Port:        8082,
		BasePath:    "/api/v2",
		Routes:      []domain.Route{{Method: "GET", Path: "/b"}},
	}
	registry.Register(req2)

	services := registry.GetAllServices()
	if len(services) != 2 {
		t.Errorf("expected 2 services, got %d", len(services))
	}
	if _, exists := services["service-a"]; !exists {
		t.Error("expected service-a to exist")
	}
	if _, exists := services["service-b"]; !exists {
		t.Error("expected service-b to exist")
	}
}

func TestGetAllRoutes(t *testing.T) {
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
	registry.Register(req)

	routes := registry.GetAllRoutes()
	if len(routes) != 2 {
		t.Errorf("expected 2 routes, got %d", len(routes))
	}
	if _, exists := routes["GET:/api/v1/users"]; !exists {
		t.Error("expected GET:/api/v1/users route to exist")
	}
	if _, exists := routes["POST:/api/v1/users"]; !exists {
		t.Error("expected POST:/api/v1/users route to exist")
	}
}
