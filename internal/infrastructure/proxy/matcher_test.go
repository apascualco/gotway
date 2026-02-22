package proxy

import (
	"testing"
	"time"

	"github.com/apascualco/gotway/internal/application"
	"github.com/apascualco/gotway/internal/domain"
)

func setupTestRegistry() *application.Registry {
	registry := application.NewRegistry(application.RegistryConfig{
		HeartbeatTTL: 30 * time.Second,
	})

	_, _ = registry.Register(&domain.RegisterRequest{
		ServiceName: "user-service",
		Host:        "localhost",
		Port:        8081,
		BasePath:    "/api/v1",
		Routes: []domain.Route{
			{Method: "GET", Path: "/users"},
			{Method: "GET", Path: "/users/:id"},
			{Method: "POST", Path: "/users"},
			{Method: "DELETE", Path: "/users/:id"},
		},
	})

	_, _ = registry.Register(&domain.RegisterRequest{
		ServiceName: "auth-service",
		Host:        "localhost",
		Port:        8082,
		BasePath:    "/api/v1/auth",
		Routes: []domain.Route{
			{Method: "POST", Path: "/login"},
			{Method: "POST", Path: "/logout"},
		},
	})

	return registry
}

func TestMatchRoute_Exact(t *testing.T) {
	registry := setupTestRegistry()

	tests := []struct {
		name        string
		method      string
		path        string
		wantService string
	}{
		{"GET users list", "GET", "/api/v1/users", "user-service"},
		{"POST users", "POST", "/api/v1/users", "user-service"},
		{"POST login", "POST", "/api/v1/auth/login", "auth-service"},
		{"POST logout", "POST", "/api/v1/auth/logout", "auth-service"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchRoute(registry, tt.method, tt.path)
			if result == nil {
				t.Fatalf("expected match for %s %s, got nil", tt.method, tt.path)
			}
			if result.Entry.ServiceName != tt.wantService {
				t.Errorf("expected service %s, got %s", tt.wantService, result.Entry.ServiceName)
			}
		})
	}
}

func TestMatchRoute_WithParams(t *testing.T) {
	registry := setupTestRegistry()

	tests := []struct {
		name        string
		method      string
		path        string
		wantService string
		wantParams  map[string]string
	}{
		{
			name:        "GET user by id",
			method:      "GET",
			path:        "/api/v1/users/123",
			wantService: "user-service",
			wantParams:  map[string]string{"id": "123"},
		},
		{
			name:        "DELETE user by id",
			method:      "DELETE",
			path:        "/api/v1/users/456",
			wantService: "user-service",
			wantParams:  map[string]string{"id": "456"},
		},
		{
			name:        "GET user by uuid",
			method:      "GET",
			path:        "/api/v1/users/abc-123-def",
			wantService: "user-service",
			wantParams:  map[string]string{"id": "abc-123-def"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchRoute(registry, tt.method, tt.path)
			if result == nil {
				t.Fatalf("expected match for %s %s, got nil", tt.method, tt.path)
			}
			if result.Entry.ServiceName != tt.wantService {
				t.Errorf("expected service %s, got %s", tt.wantService, result.Entry.ServiceName)
			}
			for key, want := range tt.wantParams {
				if got := result.Params[key]; got != want {
					t.Errorf("param %s: expected %s, got %s", key, want, got)
				}
			}
		})
	}
}

func TestMatchRoute_NotFound(t *testing.T) {
	registry := setupTestRegistry()

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"wrong method", "PUT", "/api/v1/users"},
		{"wrong path", "GET", "/api/v2/users"},
		{"non-existent path", "GET", "/api/v1/products"},
		{"partial match", "GET", "/api/v1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchRoute(registry, tt.method, tt.path)
			if result != nil {
				t.Errorf("expected no match for %s %s, got %v", tt.method, tt.path, result.Entry.ServiceName)
			}
		})
	}
}

func TestMatchPathWithParams(t *testing.T) {
	tests := []struct {
		name       string
		pattern    string
		path       string
		wantMatch  bool
		wantParams map[string]string
	}{
		{
			name:       "exact match",
			pattern:    "/users",
			path:       "/users",
			wantMatch:  true,
			wantParams: map[string]string{},
		},
		{
			name:       "single param",
			pattern:    "/users/:id",
			path:       "/users/123",
			wantMatch:  true,
			wantParams: map[string]string{"id": "123"},
		},
		{
			name:       "multiple params",
			pattern:    "/users/:userId/posts/:postId",
			path:       "/users/1/posts/42",
			wantMatch:  true,
			wantParams: map[string]string{"userId": "1", "postId": "42"},
		},
		{
			name:       "wildcard",
			pattern:    "/static/*",
			path:       "/static/css/style.css",
			wantMatch:  true,
			wantParams: map[string]string{"*": "css/style.css"},
		},
		{
			name:      "no match different length",
			pattern:   "/users/:id/posts",
			path:      "/users/1",
			wantMatch: false,
		},
		{
			name:      "no match different segment",
			pattern:   "/users/:id",
			path:      "/posts/1",
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, ok := matchPathWithParams(tt.pattern, tt.path)
			if ok != tt.wantMatch {
				t.Errorf("match: expected %v, got %v", tt.wantMatch, ok)
			}
			if tt.wantMatch {
				for key, want := range tt.wantParams {
					if got := params[key]; got != want {
						t.Errorf("param %s: expected %s, got %s", key, want, got)
					}
				}
			}
		})
	}
}
