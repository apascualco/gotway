package application

import (
	"testing"
	"time"

	"github.com/apascualco/gotway/internal/domain"
)

func TestHeartbeat_Success(t *testing.T) {
	registry := NewRegistry(RegistryConfig{})

	req := &domain.RegisterRequest{
		ServiceName: "test-service",
		Host:        "localhost",
		Port:        8081,
		BasePath:    "/api/v1",
		Routes:      []domain.Route{{Method: "GET", Path: "/test"}},
	}
	resp, _ := registry.Register(req)

	oldHeartbeat := registry.instances[resp.InstanceID].LastHeartbeat
	time.Sleep(10 * time.Millisecond)

	err := registry.Heartbeat(resp.InstanceID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	newHeartbeat := registry.instances[resp.InstanceID].LastHeartbeat
	if !newHeartbeat.After(oldHeartbeat) {
		t.Error("LastHeartbeat was not updated")
	}
}

func TestHeartbeat_NotFound(t *testing.T) {
	registry := NewRegistry(RegistryConfig{})

	err := registry.Heartbeat("non-existent-id")
	if err != domain.ErrInstanceNotFound {
		t.Errorf("expected ErrInstanceNotFound, got %v", err)
	}
}
