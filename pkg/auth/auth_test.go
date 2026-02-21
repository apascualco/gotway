package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func generateTestKeys(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key pair: %v", err)
	}
	return privateKey, &privateKey.PublicKey
}

func createTestToken(t *testing.T, privateKey *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return tokenString
}

func TestNewValidator(t *testing.T) {
	privateKey, publicKey := generateTestKeys(t)
	_ = privateKey

	tests := []struct {
		name    string
		opts    []ValidatorOption
		wantErr error
	}{
		{
			name:    "no public key",
			opts:    []ValidatorOption{},
			wantErr: ErrPublicKeyNotSet,
		},
		{
			name: "with public key",
			opts: []ValidatorOption{
				WithPublicKeyRSA(publicKey),
			},
			wantErr: nil,
		},
		{
			name: "with allowed issuers",
			opts: []ValidatorOption{
				WithPublicKeyRSA(publicKey),
				WithAllowedIssuers([]string{"api-api", "auth-service"}),
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewValidator(tt.opts...)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("NewValidator() error = %v, wantErr %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("NewValidator() unexpected error = %v", err)
			}
		})
	}
}

func TestValidator_ValidateInternalToken(t *testing.T) {
	privateKey, publicKey := generateTestKeys(t)
	now := time.Now()

	validator, err := NewValidator(
		WithPublicKeyRSA(publicKey),
		WithAllowedIssuers([]string{"api-api", "auth-service"}),
	)
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	tests := []struct {
		name             string
		claims           jwt.MapClaims
		expectedAudience string
		wantErr          error
		checkClaims      func(*testing.T, *Claims)
	}{
		{
			name: "valid token",
			claims: jwt.MapClaims{
				"sub":          "user123",
				"email":        "user@example.com",
				"scopes":       []interface{}{"read", "write"},
				"iss":          "api-api",
				"aud":          "user-service",
				"original_iss": "auth-service",
				"trace":        []interface{}{"api-api"},
				"iat":          now.Unix(),
				"exp":          now.Add(5 * time.Minute).Unix(),
			},
			expectedAudience: "user-service",
			wantErr:          nil,
			checkClaims: func(t *testing.T, c *Claims) {
				if c.Subject != "user123" {
					t.Errorf("Subject = %s, want user123", c.Subject)
				}
				if c.Email != "user@example.com" {
					t.Errorf("Email = %s, want user@example.com", c.Email)
				}
				if !c.HasScope("read") || !c.HasScope("write") {
					t.Error("missing expected scopes")
				}
			},
		},
		{
			name: "expired token",
			claims: jwt.MapClaims{
				"sub": "user123",
				"iss": "api-api",
				"aud": "user-service",
				"iat": now.Add(-10 * time.Minute).Unix(),
				"exp": now.Add(-5 * time.Minute).Unix(),
			},
			expectedAudience: "user-service",
			wantErr:          ErrTokenExpired,
		},
		{
			name: "wrong audience",
			claims: jwt.MapClaims{
				"sub": "user123",
				"iss": "api-api",
				"aud": "wrong-service",
				"iat": now.Unix(),
				"exp": now.Add(5 * time.Minute).Unix(),
			},
			expectedAudience: "user-service",
			wantErr:          ErrTokenAudienceMismatch,
		},
		{
			name: "issuer not allowed",
			claims: jwt.MapClaims{
				"sub": "user123",
				"iss": "unknown-service",
				"aud": "user-service",
				"iat": now.Unix(),
				"exp": now.Add(5 * time.Minute).Unix(),
			},
			expectedAudience: "user-service",
			wantErr:          ErrTokenIssuerNotAllowed,
		},
		{
			name: "missing subject",
			claims: jwt.MapClaims{
				"iss": "api-api",
				"aud": "user-service",
				"iat": now.Unix(),
				"exp": now.Add(5 * time.Minute).Unix(),
			},
			expectedAudience: "user-service",
			wantErr:          ErrTokenInvalidSubject,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenString := createTestToken(t, privateKey, tt.claims)
			claims, err := validator.ValidateInternalToken(tokenString, tt.expectedAudience)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("ValidateInternalToken() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("ValidateInternalToken() unexpected error = %v", err)
				return
			}

			if tt.checkClaims != nil {
				tt.checkClaims(t, claims)
			}
		})
	}
}

func TestValidator_InvalidSignature(t *testing.T) {
	privateKey1, _ := generateTestKeys(t)
	_, publicKey2 := generateTestKeys(t)

	validator, err := NewValidator(WithPublicKeyRSA(publicKey2))
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	now := time.Now()
	tokenString := createTestToken(t, privateKey1, jwt.MapClaims{
		"sub": "user123",
		"iss": "api-api",
		"aud": "user-service",
		"iat": now.Unix(),
		"exp": now.Add(5 * time.Minute).Unix(),
	})

	_, err = validator.ValidateInternalToken(tokenString, "user-service")
	if !errors.Is(err, ErrTokenInvalidSignature) {
		t.Errorf("ValidateInternalToken() error = %v, want ErrTokenInvalidSignature", err)
	}
}

func TestNewGenerator(t *testing.T) {
	privateKey, _ := generateTestKeys(t)

	tests := []struct {
		name    string
		opts    []GeneratorOption
		wantErr bool
	}{
		{
			name:    "no private key",
			opts:    []GeneratorOption{WithIssuer("test-service")},
			wantErr: true,
		},
		{
			name:    "no issuer",
			opts:    []GeneratorOption{WithPrivateKeyRSA(privateKey)},
			wantErr: true,
		},
		{
			name: "valid configuration",
			opts: []GeneratorOption{
				WithPrivateKeyRSA(privateKey),
				WithIssuer("test-service"),
			},
			wantErr: false,
		},
		{
			name: "with custom TTL",
			opts: []GeneratorOption{
				WithPrivateKeyRSA(privateKey),
				WithIssuer("test-service"),
				WithDefaultTTL(10 * time.Minute),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewGenerator(tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewGenerator() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGenerator_GenerateServiceToken(t *testing.T) {
	privateKey, publicKey := generateTestKeys(t)

	generator, err := NewGenerator(
		WithPrivateKeyRSA(privateKey),
		WithIssuer("user-service"),
		WithDefaultTTL(5*time.Minute),
	)
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	validator, err := NewValidator(
		WithPublicKeyRSA(publicKey),
		WithAllowedIssuers([]string{"user-service"}),
	)
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	incomingClaims := &Claims{
		Subject:        "user123",
		Email:          "user@example.com",
		Scopes:         []string{"read", "write"},
		Issuer:         "api-api",
		Audience:       "user-service",
		OriginalIssuer: "auth-service",
		Trace:          []string{"api-api"},
		IssuedAt:       time.Now().Unix(),
		ExpiresAt:      time.Now().Add(5 * time.Minute).Unix(),
	}

	tokenString, err := generator.GenerateServiceToken(incomingClaims, "order-service")
	if err != nil {
		t.Fatalf("GenerateServiceToken() error = %v", err)
	}

	claims, err := validator.ValidateInternalToken(tokenString, "order-service")
	if err != nil {
		t.Fatalf("ValidateInternalToken() error = %v", err)
	}

	if claims.Subject != "user123" {
		t.Errorf("Subject = %s, want user123", claims.Subject)
	}

	if claims.Issuer != "user-service" {
		t.Errorf("Issuer = %s, want user-service", claims.Issuer)
	}

	if claims.Audience != "order-service" {
		t.Errorf("Audience = %s, want order-service", claims.Audience)
	}

	if len(claims.Trace) != 2 || claims.Trace[0] != "api-api" || claims.Trace[1] != "user-service" {
		t.Errorf("Trace = %v, want [api-api, user-service]", claims.Trace)
	}
}

func TestClaims_HasScope(t *testing.T) {
	claims := &Claims{
		Scopes: []string{"read", "write", "admin"},
	}

	tests := []struct {
		scope string
		want  bool
	}{
		{"read", true},
		{"write", true},
		{"admin", true},
		{"delete", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.scope, func(t *testing.T) {
			if got := claims.HasScope(tt.scope); got != tt.want {
				t.Errorf("HasScope(%s) = %v, want %v", tt.scope, got, tt.want)
			}
		})
	}
}

func TestClaims_HasAllScopes(t *testing.T) {
	claims := &Claims{
		Scopes: []string{"read", "write", "admin"},
	}

	tests := []struct {
		name   string
		scopes []string
		want   bool
	}{
		{"all present", []string{"read", "write"}, true},
		{"one present", []string{"read"}, true},
		{"none present", []string{"delete"}, false},
		{"some missing", []string{"read", "delete"}, false},
		{"empty", []string{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := claims.HasAllScopes(tt.scopes); got != tt.want {
				t.Errorf("HasAllScopes(%v) = %v, want %v", tt.scopes, got, tt.want)
			}
		})
	}
}

func TestClient(t *testing.T) {
	privateKey, publicKey := generateTestKeys(t)

	client, err := NewClient(
		[]ValidatorOption{
			WithPublicKeyRSA(publicKey),
			WithAllowedIssuers([]string{"api-api", "user-service"}),
		},
		[]GeneratorOption{
			WithPrivateKeyRSA(privateKey),
			WithIssuer("user-service"),
		},
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	incomingClaims := &Claims{
		Subject:        "user123",
		Email:          "user@example.com",
		Scopes:         []string{"read"},
		Issuer:         "api-api",
		Audience:       "user-service",
		OriginalIssuer: "auth-service",
		Trace:          []string{"api-api"},
		IssuedAt:       time.Now().Unix(),
		ExpiresAt:      time.Now().Add(5 * time.Minute).Unix(),
	}

	tokenString, err := client.GenerateServiceToken(incomingClaims, "order-service")
	if err != nil {
		t.Fatalf("GenerateServiceToken() error = %v", err)
	}

	claims, err := client.ValidateInternalToken(tokenString, "order-service")
	if err != nil {
		t.Fatalf("ValidateInternalToken() error = %v", err)
	}

	if claims.Subject != "user123" {
		t.Errorf("Subject = %s, want user123", claims.Subject)
	}
}
