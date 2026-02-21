package domain

import (
	"testing"
	"time"
)

func TestExternalClaims_Valid(t *testing.T) {
	tests := []struct {
		name    string
		claims  ExternalClaims
		wantErr error
	}{
		{
			name: "valid claims",
			claims: ExternalClaims{
				Subject:   "user-123",
				Email:     "user@example.com",
				Scopes:    []string{"read", "write"},
				Issuer:    IssuerAuthService,
				ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
				IssuedAt:  time.Now().Unix(),
			},
			wantErr: nil,
		},
		{
			name: "expired token",
			claims: ExternalClaims{
				Subject:   "user-123",
				ExpiresAt: time.Now().Add(-1 * time.Hour).Unix(),
			},
			wantErr: ErrTokenExpired,
		},
		{
			name: "token not yet valid",
			claims: ExternalClaims{
				Subject:   "user-123",
				NotBefore: time.Now().Add(1 * time.Hour).Unix(),
				ExpiresAt: time.Now().Add(2 * time.Hour).Unix(),
			},
			wantErr: ErrTokenNotYetValid,
		},
		{
			name: "missing subject",
			claims: ExternalClaims{
				Email:     "user@example.com",
				ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
			},
			wantErr: ErrTokenInvalidSubject,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.claims.Valid()
			if err != tt.wantErr {
				t.Errorf("Valid() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInternalClaims_Valid(t *testing.T) {
	tests := []struct {
		name    string
		claims  InternalClaims
		wantErr error
	}{
		{
			name: "valid claims",
			claims: InternalClaims{
				Subject:   "user-123",
				Issuer:    IssuerAPIGateway,
				Audience:  "user-service",
				ExpiresAt: time.Now().Add(5 * time.Minute).Unix(),
				IssuedAt:  time.Now().Unix(),
			},
			wantErr: nil,
		},
		{
			name: "expired token",
			claims: InternalClaims{
				Subject:   "user-123",
				Issuer:    IssuerAPIGateway,
				Audience:  "user-service",
				ExpiresAt: time.Now().Add(-1 * time.Minute).Unix(),
			},
			wantErr: ErrTokenExpired,
		},
		{
			name: "missing subject",
			claims: InternalClaims{
				Issuer:    IssuerAPIGateway,
				Audience:  "user-service",
				ExpiresAt: time.Now().Add(5 * time.Minute).Unix(),
			},
			wantErr: ErrTokenInvalidSubject,
		},
		{
			name: "missing issuer",
			claims: InternalClaims{
				Subject:   "user-123",
				Audience:  "user-service",
				ExpiresAt: time.Now().Add(5 * time.Minute).Unix(),
			},
			wantErr: ErrTokenInvalidIssuer,
		},
		{
			name: "missing audience",
			claims: InternalClaims{
				Subject:   "user-123",
				Issuer:    IssuerAPIGateway,
				ExpiresAt: time.Now().Add(5 * time.Minute).Unix(),
			},
			wantErr: ErrTokenInvalidAudience,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.claims.Valid()
			if err != tt.wantErr {
				t.Errorf("Valid() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInternalClaims_ValidateAudience(t *testing.T) {
	claims := InternalClaims{
		Subject:  "user-123",
		Issuer:   IssuerAPIGateway,
		Audience: "user-service",
	}

	if err := claims.ValidateAudience("user-service"); err != nil {
		t.Errorf("ValidateAudience() should pass for matching audience, got %v", err)
	}

	if err := claims.ValidateAudience("billing-service"); err == nil {
		t.Error("ValidateAudience() should fail for mismatched audience")
	}
}

func TestInternalClaims_ValidateIssuer(t *testing.T) {
	claims := InternalClaims{
		Subject:  "user-123",
		Issuer:   IssuerAPIGateway,
		Audience: "user-service",
	}

	allowedIssuers := []string{IssuerAPIGateway, "user-service", "task-service"}

	if err := claims.ValidateIssuer(allowedIssuers); err != nil {
		t.Errorf("ValidateIssuer() should pass for allowed issuer, got %v", err)
	}

	if err := claims.ValidateIssuer([]string{"other-service"}); err == nil {
		t.Error("ValidateIssuer() should fail for not allowed issuer")
	}
}

func TestInternalClaims_AddToTrace(t *testing.T) {
	claims := InternalClaims{
		Subject:  "user-123",
		Issuer:   IssuerAPIGateway,
		Audience: "user-service",
		Trace:    []string{IssuerAPIGateway},
	}

	claims.AddToTrace("user-service")

	if len(claims.Trace) != 2 {
		t.Errorf("AddToTrace() should add to trace, got len=%d", len(claims.Trace))
	}

	if claims.Trace[1] != "user-service" {
		t.Errorf("AddToTrace() should append service, got %v", claims.Trace)
	}
}

func TestInternalClaims_HasScope(t *testing.T) {
	claims := InternalClaims{
		Subject:  "user-123",
		Issuer:   IssuerAPIGateway,
		Audience: "user-service",
		Scopes:   []string{"users:read", "users:write", "admin"},
	}

	if !claims.HasScope("users:read") {
		t.Error("HasScope() should return true for existing scope")
	}

	if claims.HasScope("billing:read") {
		t.Error("HasScope() should return false for non-existing scope")
	}
}

func TestInternalClaims_HasAllScopes(t *testing.T) {
	claims := InternalClaims{
		Subject:  "user-123",
		Issuer:   IssuerAPIGateway,
		Audience: "user-service",
		Scopes:   []string{"users:read", "users:write", "admin"},
	}

	if !claims.HasAllScopes([]string{"users:read", "admin"}) {
		t.Error("HasAllScopes() should return true when all scopes present")
	}

	if claims.HasAllScopes([]string{"users:read", "billing:read"}) {
		t.Error("HasAllScopes() should return false when some scopes missing")
	}

	if !claims.HasAllScopes([]string{}) {
		t.Error("HasAllScopes() should return true for empty required scopes")
	}
}
