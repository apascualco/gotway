package domain

import "testing"

func TestRoute_FullPath(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
		path     string
		expected string
	}{
		{"with base and path", "/api/v1", "/users", "/api/v1/users"},
		{"empty base", "", "/users", "/users"},
		{"empty path", "/api/v1", "", "/api/v1"},
		{"both empty", "", "", ""},
		{"nested path", "/api/v1/auth", "/login", "/api/v1/auth/login"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := &Route{Path: tt.path}
			if got := route.FullPath(tt.basePath); got != tt.expected {
				t.Errorf("FullPath(%q) = %q, want %q", tt.basePath, got, tt.expected)
			}
		})
	}
}

func TestRoute_Key(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		basePath string
		path     string
		expected string
	}{
		{"GET users", "GET", "/api/v1", "/users", "GET:/api/v1/users"},
		{"POST login", "POST", "/api/v1/auth", "/login", "POST:/api/v1/auth/login"},
		{"DELETE item", "DELETE", "/api", "/items/:id", "DELETE:/api/items/:id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := &Route{Method: tt.method, Path: tt.path}
			if got := route.Key(tt.basePath); got != tt.expected {
				t.Errorf("Key(%q) = %q, want %q", tt.basePath, got, tt.expected)
			}
		})
	}
}
