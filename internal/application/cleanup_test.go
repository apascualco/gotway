package application

import (
	"testing"
	"time"

	"github.com/apascualco/gotway/internal/domain"
)

func TestCleanup_MarksUnhealthy(t *testing.T) {
	registry := NewRegistry(RegistryConfig{
		HeartbeatTTL: 100 * time.Millisecond,
	})

	req := &domain.RegisterRequest{
		ServiceName: "test-service",
		Host:        "localhost",
		Port:        8081,
		BasePath:    "/api/v1",
		Routes:      []domain.Route{{Method: "GET", Path: "/test"}},
	}
	resp, _ := registry.Register(req)

	registry.instances[resp.InstanceID].LastHeartbeat = time.Now().Add(-150 * time.Millisecond)

	registry.cleanup()

	instance := registry.instances[resp.InstanceID]
	if instance == nil {
		t.Fatal("instance should still exist")
	}
	if instance.Status != domain.StatusUnhealthy {
		t.Errorf("expected status unhealthy, got %s", instance.Status)
	}
}

func TestCleanup_RemovesExpired(t *testing.T) {
	registry := NewRegistry(RegistryConfig{
		HeartbeatTTL: 100 * time.Millisecond,
	})

	req := &domain.RegisterRequest{
		ServiceName: "test-service",
		Host:        "localhost",
		Port:        8081,
		BasePath:    "/api/v1",
		Routes:      []domain.Route{{Method: "GET", Path: "/test"}},
	}
	resp, _ := registry.Register(req)

	registry.instances[resp.InstanceID].LastHeartbeat = time.Now().Add(-250 * time.Millisecond)

	registry.cleanup()

	if _, exists := registry.instances[resp.InstanceID]; exists {
		t.Error("expired instance should be removed")
	}
	if len(registry.routes) != 0 {
		t.Errorf("routes should be removed, got %d", len(registry.routes))
	}
	if len(registry.services["test-service"]) != 0 {
		t.Errorf("service entry should be removed, got %d", len(registry.services["test-service"]))
	}
}

func TestCleanup_KeepsHealthyInstances(t *testing.T) {
	registry := NewRegistry(RegistryConfig{
		HeartbeatTTL: 100 * time.Millisecond,
	})

	req := &domain.RegisterRequest{
		ServiceName: "test-service",
		Host:        "localhost",
		Port:        8081,
		BasePath:    "/api/v1",
		Routes:      []domain.Route{{Method: "GET", Path: "/test"}},
	}
	resp, _ := registry.Register(req)

	registry.cleanup()

	instance := registry.instances[resp.InstanceID]
	if instance == nil {
		t.Fatal("healthy instance should still exist")
	}
	if instance.Status != domain.StatusHealthy {
		t.Errorf("expected status healthy, got %s", instance.Status)
	}
}

func TestCleanup_DoesNotMarkUnhealthyTwice(t *testing.T) {
	registry := NewRegistry(RegistryConfig{
		HeartbeatTTL: 100 * time.Millisecond,
	})

	req := &domain.RegisterRequest{
		ServiceName: "test-service",
		Host:        "localhost",
		Port:        8081,
		BasePath:    "/api/v1",
		Routes:      []domain.Route{{Method: "GET", Path: "/test"}},
	}
	resp, _ := registry.Register(req)

	registry.instances[resp.InstanceID].LastHeartbeat = time.Now().Add(-150 * time.Millisecond)
	registry.instances[resp.InstanceID].Status = domain.StatusUnhealthy

	registry.cleanup()

	instance := registry.instances[resp.InstanceID]
	if instance == nil {
		t.Fatal("instance should still exist (not yet 2x TTL)")
	}
}

func TestStartStop(t *testing.T) {
	registry := NewRegistry(RegistryConfig{
		HeartbeatTTL: 30 * time.Second,
	})

	registry.Start()
	time.Sleep(10 * time.Millisecond)
	registry.Stop()
}
