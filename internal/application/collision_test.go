package application

import (
	"testing"
	"time"

	"github.com/apascualco/gotway/internal/domain"
)

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/users/:id", "/users/*"},
		{"/users/{id}", "/users/*"},
		{"/users/:id/posts/:postId", "/users/*/posts/*"},
		{"/users/{userId}/posts/{postId}", "/users/*/posts/*"},
		{"/static/path", "/static/path"},
		{"/mix/:id/fixed/{name}", "/mix/*/fixed/*"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizePath(tt.input)
			if result != tt.expected {
				t.Errorf("normalizePath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestValidateRoutes_NoCollision(t *testing.T) {
	registry := NewRegistry(RegistryConfig{})

	routes := []domain.Route{
		{Method: "GET", Path: "/users"},
		{Method: "POST", Path: "/users"},
	}

	collisions, err := registry.ValidateRoutes("service-a", "/api/v1", routes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(collisions) != 0 {
		t.Errorf("expected no collisions, got %d", len(collisions))
	}
}

func TestValidateRoutes_ExactCollision(t *testing.T) {
	registry := NewRegistry(RegistryConfig{})
	registry.routes["GET:/api/v1/users"] = &domain.RouteEntry{
		ServiceName:  "service-a",
		BasePath:     "/api/v1",
		Route:        domain.Route{Method: "GET", Path: "/users"},
		RegisteredAt: time.Now(),
	}

	routes := []domain.Route{
		{Method: "GET", Path: "/users"},
	}

	collisions, err := registry.ValidateRoutes("service-b", "/api/v1", routes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(collisions) != 1 {
		t.Fatalf("expected 1 collision, got %d", len(collisions))
	}
	if collisions[0].CollisionType != domain.ExactCollision {
		t.Errorf("expected ExactCollision, got %s", collisions[0].CollisionType)
	}
	if collisions[0].RegisteredBy != "service-a" {
		t.Errorf("expected RegisteredBy service-a, got %s", collisions[0].RegisteredBy)
	}
}

func TestValidateRoutes_PatternCollision(t *testing.T) {
	registry := NewRegistry(RegistryConfig{
		StrictPatternMatching: true,
	})
	registry.routes["GET:/api/v1/users/:id"] = &domain.RouteEntry{
		ServiceName:  "service-a",
		BasePath:     "/api/v1",
		Route:        domain.Route{Method: "GET", Path: "/users/:id"},
		RegisteredAt: time.Now(),
	}

	routes := []domain.Route{
		{Method: "GET", Path: "/users/{userId}"},
	}

	collisions, err := registry.ValidateRoutes("service-b", "/api/v1", routes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(collisions) != 1 {
		t.Fatalf("expected 1 collision, got %d", len(collisions))
	}
	if collisions[0].CollisionType != domain.PatternCollision {
		t.Errorf("expected PatternCollision, got %s", collisions[0].CollisionType)
	}
}

func TestValidateRoutes_SameServiceAllowed(t *testing.T) {
	registry := NewRegistry(RegistryConfig{})
	registry.routes["GET:/api/v1/users"] = &domain.RouteEntry{
		ServiceName:  "service-a",
		BasePath:     "/api/v1",
		Route:        domain.Route{Method: "GET", Path: "/users"},
		RegisteredAt: time.Now(),
	}

	routes := []domain.Route{
		{Method: "GET", Path: "/users"},
	}

	collisions, err := registry.ValidateRoutes("service-a", "/api/v1", routes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(collisions) != 0 {
		t.Errorf("expected no collisions for same service, got %d", len(collisions))
	}
}

func TestPathsOverlap(t *testing.T) {
	tests := []struct {
		path1    string
		path2    string
		expected bool
	}{
		{"/users/*", "/users/*", true},
		{"/users/*", "/users/123", true},
		{"/users/*/posts", "/users/*/posts", true},
		{"/users/*", "/posts/*", false},
		{"/users/*/posts", "/users/*/comments", false},
		{"/a/b/c", "/a/b/d", false},
		{"/a/b", "/a/b/c", false},
	}

	for _, tt := range tests {
		name := tt.path1 + " vs " + tt.path2
		t.Run(name, func(t *testing.T) {
			result := pathsOverlap(tt.path1, tt.path2)
			if result != tt.expected {
				t.Errorf("pathsOverlap(%q, %q) = %v, want %v", tt.path1, tt.path2, result, tt.expected)
			}
		})
	}
}
