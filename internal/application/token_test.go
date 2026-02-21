package application

import "testing"

func TestValidateToken_Valid(t *testing.T) {
	registry := NewRegistry(RegistryConfig{
		ServiceToken: "test-secret-token",
	})

	if !registry.ValidateToken("test-secret-token") {
		t.Error("expected valid token to return true")
	}
}

func TestValidateToken_Invalid(t *testing.T) {
	registry := NewRegistry(RegistryConfig{
		ServiceToken: "test-secret-token",
	})

	tests := []struct {
		name  string
		token string
	}{
		{"wrong token", "wrong-token"},
		{"empty token", ""},
		{"similar token", "test-secret-toke"},
		{"extra char", "test-secret-token1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if registry.ValidateToken(tt.token) {
				t.Errorf("expected token %q to return false", tt.token)
			}
		})
	}
}

func TestValidateToken_EmptyConfigToken(t *testing.T) {
	registry := NewRegistry(RegistryConfig{
		ServiceToken: "",
	})

	if registry.ValidateToken("any-token") {
		t.Error("expected false when config token is empty")
	}
}
