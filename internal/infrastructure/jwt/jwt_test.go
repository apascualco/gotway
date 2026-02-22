package jwt

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/apascualco/gotway/internal/domain"
	"github.com/apascualco/gotway/internal/infrastructure/config"
	"github.com/golang-jwt/jwt/v5"
)

func generateTestKeys() (string, string) {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	privBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privBytes,
	})

	pubBytes, _ := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})

	return string(privPEM), string(pubPEM)
}

func createTestService(t *testing.T) *Service {
	privPEM, pubPEM := generateTestKeys()

	cfg := &config.Config{
		JWTPrivateKey:     privPEM,
		JWTPublicKey:      pubPEM,
		JWTIssuer:         "api-api",
		JWTInternalTTL:    5 * time.Minute,
		JWTAllowedIssuers: []string{"api-api", "auth-service"},
	}

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	return svc
}

func createExternalToken(privateKey *rsa.PrivateKey, claims jwt.MapClaims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, _ := token.SignedString(privateKey)
	return tokenString
}

func TestNewService(t *testing.T) {
	privPEM, pubPEM := generateTestKeys()

	t.Run("with valid keys", func(t *testing.T) {
		cfg := &config.Config{
			JWTPrivateKey:     privPEM,
			JWTPublicKey:      pubPEM,
			JWTIssuer:         "api-api",
			JWTInternalTTL:    5 * time.Minute,
			JWTAllowedIssuers: []string{"auth-service"},
		}

		svc, err := NewService(cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		if svc.privateKey == nil {
			t.Error("privateKey should not be nil")
		}
		if svc.publicKey == nil {
			t.Error("publicKey should not be nil")
		}
	})

	t.Run("with only private key derives public key", func(t *testing.T) {
		cfg := &config.Config{
			JWTPrivateKey:     privPEM,
			JWTIssuer:         "api-api",
			JWTInternalTTL:    5 * time.Minute,
			JWTAllowedIssuers: []string{"auth-service"},
		}

		svc, err := NewService(cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		if svc.publicKey == nil {
			t.Error("publicKey should be derived from privateKey")
		}
	})

	t.Run("with invalid private key", func(t *testing.T) {
		cfg := &config.Config{
			JWTPrivateKey: "invalid-key",
			JWTIssuer:     "api-api",
		}

		_, err := NewService(cfg)
		if err == nil {
			t.Error("NewService() should return error for invalid private key")
		}
	})
}

func TestValidateExternalToken_Valid(t *testing.T) {
	privPEM, pubPEM := generateTestKeys()

	cfg := &config.Config{
		JWTPublicKey:      pubPEM,
		JWTIssuer:         "api-api",
		JWTInternalTTL:    5 * time.Minute,
		JWTAllowedIssuers: []string{"auth-service"},
	}

	svc, _ := NewService(cfg)

	// Parse the private key to sign a test token
	block, _ := pem.Decode([]byte(privPEM))
	privateKey, _ := x509.ParsePKCS1PrivateKey(block.Bytes)

	tokenString := createExternalToken(privateKey, jwt.MapClaims{
		"sub":    "user-123",
		"email":  "user@example.com",
		"scopes": []interface{}{"read", "write"},
		"iss":    "auth-service",
		"iat":    time.Now().Unix(),
		"exp":    time.Now().Add(1 * time.Hour).Unix(),
	})

	claims, err := svc.ValidateExternalToken(tokenString)
	if err != nil {
		t.Fatalf("ValidateExternalToken() error = %v", err)
	}

	if claims.Subject != "user-123" {
		t.Errorf("Subject = %v, want user-123", claims.Subject)
	}
	if claims.Email != "user@example.com" {
		t.Errorf("Email = %v, want user@example.com", claims.Email)
	}
	if len(claims.Scopes) != 2 {
		t.Errorf("Scopes len = %v, want 2", len(claims.Scopes))
	}
}

func TestValidateExternalToken_Expired(t *testing.T) {
	privPEM, pubPEM := generateTestKeys()

	cfg := &config.Config{
		JWTPublicKey:      pubPEM,
		JWTIssuer:         "api-api",
		JWTAllowedIssuers: []string{"auth-service"},
	}

	svc, _ := NewService(cfg)

	block, _ := pem.Decode([]byte(privPEM))
	privateKey, _ := x509.ParsePKCS1PrivateKey(block.Bytes)

	tokenString := createExternalToken(privateKey, jwt.MapClaims{
		"sub": "user-123",
		"iss": "auth-service",
		"iat": time.Now().Add(-2 * time.Hour).Unix(),
		"exp": time.Now().Add(-1 * time.Hour).Unix(),
	})

	_, err := svc.ValidateExternalToken(tokenString)
	if err != domain.ErrTokenExpired {
		t.Errorf("ValidateExternalToken() error = %v, want ErrTokenExpired", err)
	}
}

func TestValidateExternalToken_InvalidSignature(t *testing.T) {
	_, pubPEM := generateTestKeys()
	differentPrivPEM, _ := generateTestKeys()

	cfg := &config.Config{
		JWTPublicKey:      pubPEM,
		JWTIssuer:         "api-api",
		JWTAllowedIssuers: []string{"auth-service"},
	}

	svc, _ := NewService(cfg)

	// Sign with a different key
	block, _ := pem.Decode([]byte(differentPrivPEM))
	differentPrivateKey, _ := x509.ParsePKCS1PrivateKey(block.Bytes)

	tokenString := createExternalToken(differentPrivateKey, jwt.MapClaims{
		"sub": "user-123",
		"iss": "auth-service",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	})

	_, err := svc.ValidateExternalToken(tokenString)
	if err != domain.ErrTokenInvalidSignature {
		t.Errorf("ValidateExternalToken() error = %v, want ErrTokenInvalidSignature", err)
	}
}

func TestGenerateInternalToken(t *testing.T) {
	svc := createTestService(t)

	extClaims := &domain.ExternalClaims{
		Subject: "user-123",
		Email:   "user@example.com",
		Scopes:  []string{"read", "write"},
		Issuer:  "auth-service",
	}

	tokenString, err := svc.GenerateInternalToken(extClaims, "user-service")
	if err != nil {
		t.Fatalf("GenerateInternalToken() error = %v", err)
	}

	if tokenString == "" {
		t.Error("GenerateInternalToken() returned empty token")
	}

	// Validate the generated token
	intClaims, err := svc.ValidateInternalToken(tokenString, "user-service")
	if err != nil {
		t.Fatalf("ValidateInternalToken() error = %v", err)
	}

	if intClaims.Subject != "user-123" {
		t.Errorf("Subject = %v, want user-123", intClaims.Subject)
	}
	if intClaims.Issuer != "api-api" {
		t.Errorf("Issuer = %v, want api-api", intClaims.Issuer)
	}
	if intClaims.Audience != "user-service" {
		t.Errorf("Audience = %v, want user-service", intClaims.Audience)
	}
	if intClaims.OriginalIssuer != "auth-service" {
		t.Errorf("OriginalIssuer = %v, want auth-service", intClaims.OriginalIssuer)
	}
	if len(intClaims.Trace) != 1 || intClaims.Trace[0] != "api-api" {
		t.Errorf("Trace = %v, want [api-api]", intClaims.Trace)
	}
}

func TestValidateInternalToken_Valid(t *testing.T) {
	svc := createTestService(t)

	extClaims := &domain.ExternalClaims{
		Subject: "user-123",
		Email:   "user@example.com",
		Scopes:  []string{"admin"},
		Issuer:  "auth-service",
	}

	tokenString, _ := svc.GenerateInternalToken(extClaims, "task-service")

	claims, err := svc.ValidateInternalToken(tokenString, "task-service")
	if err != nil {
		t.Fatalf("ValidateInternalToken() error = %v", err)
	}

	if claims.Subject != "user-123" {
		t.Errorf("Subject = %v, want user-123", claims.Subject)
	}
	if !claims.HasScope("admin") {
		t.Error("HasScope(admin) should be true")
	}
}

func TestValidateInternalToken_WrongAudience(t *testing.T) {
	svc := createTestService(t)

	extClaims := &domain.ExternalClaims{
		Subject: "user-123",
		Issuer:  "auth-service",
	}

	tokenString, _ := svc.GenerateInternalToken(extClaims, "user-service")

	_, err := svc.ValidateInternalToken(tokenString, "billing-service")
	if err == nil {
		t.Error("ValidateInternalToken() should fail for wrong audience")
	}
}

func TestValidateInternalToken_UnknownIssuer(t *testing.T) {
	privPEM, pubPEM := generateTestKeys()

	// Create service with restricted allowed issuers
	cfg := &config.Config{
		JWTPrivateKey:     privPEM,
		JWTPublicKey:      pubPEM,
		JWTIssuer:         "unknown-service",
		JWTInternalTTL:    5 * time.Minute,
		JWTAllowedIssuers: []string{"api-api"}, // Only api-api allowed
	}

	svc, _ := NewService(cfg)

	extClaims := &domain.ExternalClaims{
		Subject: "user-123",
		Issuer:  "auth-service",
	}

	// Generate token with "unknown-service" as issuer
	tokenString, _ := svc.GenerateInternalToken(extClaims, "user-service")

	// Try to validate - should fail because "unknown-service" is not in allowed issuers
	_, err := svc.ValidateInternalToken(tokenString, "user-service")
	if err == nil {
		t.Error("ValidateInternalToken() should fail for unknown issuer")
	}
}

func TestGenerateServiceToken(t *testing.T) {
	svc := createTestService(t)

	intClaims := &domain.InternalClaims{
		Subject:        "user-123",
		Email:          "user@example.com",
		Scopes:         []string{"read"},
		Issuer:         "api-api",
		Audience:       "user-service",
		OriginalIssuer: "auth-service",
		Trace:          []string{"api-api"},
	}

	tokenString, err := svc.GenerateServiceToken(intClaims, "billing-service")
	if err != nil {
		t.Fatalf("GenerateServiceToken() error = %v", err)
	}

	// Validate the generated token (need to add api-api to allowed issuers)
	claims, err := svc.ValidateInternalToken(tokenString, "billing-service")
	if err != nil {
		t.Fatalf("ValidateInternalToken() error = %v", err)
	}

	if claims.Audience != "billing-service" {
		t.Errorf("Audience = %v, want billing-service", claims.Audience)
	}

	// Trace should include both api-api and the new issuer
	if len(claims.Trace) != 2 {
		t.Errorf("Trace len = %v, want 2", len(claims.Trace))
	}
}

func TestValidateServiceToken_Valid(t *testing.T) {
	svc := createTestService(t)

	// Sign a service token with the test service's private key
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": "hello-world",
		"aud": "api-gateway",
		"iss": "hello-world",
		"iat": now.Unix(),
		"exp": now.Add(5 * time.Minute).Unix(),
	})
	tokenString, err := token.SignedString(svc.privateKey)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	serviceName, err := svc.ValidateServiceToken(tokenString)
	if err != nil {
		t.Fatalf("ValidateServiceToken() error = %v", err)
	}

	if serviceName != "hello-world" {
		t.Errorf("serviceName = %v, want hello-world", serviceName)
	}
}

func TestValidateServiceToken_Expired(t *testing.T) {
	svc := createTestService(t)

	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": "hello-world",
		"aud": "api-gateway",
		"iss": "hello-world",
		"iat": now.Add(-10 * time.Minute).Unix(),
		"exp": now.Add(-5 * time.Minute).Unix(),
	})
	tokenString, _ := token.SignedString(svc.privateKey)

	_, err := svc.ValidateServiceToken(tokenString)
	if err != domain.ErrTokenExpired {
		t.Errorf("ValidateServiceToken() error = %v, want ErrTokenExpired", err)
	}
}

func TestValidateServiceToken_WrongAudience(t *testing.T) {
	svc := createTestService(t)

	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": "hello-world",
		"aud": "wrong-audience",
		"iss": "hello-world",
		"iat": now.Unix(),
		"exp": now.Add(5 * time.Minute).Unix(),
	})
	tokenString, _ := token.SignedString(svc.privateKey)

	_, err := svc.ValidateServiceToken(tokenString)
	if err == nil {
		t.Error("ValidateServiceToken() should fail for wrong audience")
	}
}

func TestValidateServiceToken_MissingSubject(t *testing.T) {
	svc := createTestService(t)

	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"aud": "api-gateway",
		"iss": "hello-world",
		"iat": now.Unix(),
		"exp": now.Add(5 * time.Minute).Unix(),
	})
	tokenString, _ := token.SignedString(svc.privateKey)

	_, err := svc.ValidateServiceToken(tokenString)
	if err == nil {
		t.Error("ValidateServiceToken() should fail for missing subject")
	}
}

func TestValidateServiceToken_InvalidSignature(t *testing.T) {
	svc := createTestService(t)
	differentKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": "hello-world",
		"aud": "api-gateway",
		"iss": "hello-world",
		"iat": now.Unix(),
		"exp": now.Add(5 * time.Minute).Unix(),
	})
	tokenString, _ := token.SignedString(differentKey)

	_, err := svc.ValidateServiceToken(tokenString)
	if err != domain.ErrTokenInvalidSignature {
		t.Errorf("ValidateServiceToken() error = %v, want ErrTokenInvalidSignature", err)
	}

	_ = svc
}
