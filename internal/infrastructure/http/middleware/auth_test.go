package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/apascualco/gotway/internal/domain"
	"github.com/apascualco/gotway/internal/infrastructure/jwt"
	"github.com/gin-gonic/gin"
	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestKeys(t *testing.T) *rsa.PrivateKey {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return privateKey
}

func createTestJWTService(t *testing.T, privateKey *rsa.PrivateKey) *jwt.Service {
	return jwt.NewServiceWithKeys(
		privateKey,
		&privateKey.PublicKey,
		"api-api",
		5*time.Minute,
		[]string{"auth-service", "api-api"},
	)
}

func generateExternalToken(t *testing.T, privateKey *rsa.PrivateKey, claims *domain.ExternalClaims) string {
	now := time.Now()
	jwtClaims := gojwt.MapClaims{
		"sub":    claims.Subject,
		"email":  claims.Email,
		"scopes": claims.Scopes,
		"iss":    claims.Issuer,
		"iat":    now.Unix(),
		"exp":    now.Add(1 * time.Hour).Unix(),
	}
	if claims.Audience != "" {
		jwtClaims["aud"] = claims.Audience
	}

	token := gojwt.NewWithClaims(gojwt.SigningMethodRS256, jwtClaims)
	tokenString, err := token.SignedString(privateKey)
	require.NoError(t, err)
	return tokenString
}

func createPublicRoute() *domain.RouteEntry {
	return &domain.RouteEntry{
		ServiceName: "test-service",
		Route: domain.Route{
			Path:   "/public",
			Method: "GET",
			Public: true,
		},
	}
}

func createProtectedRoute(scopes ...string) *domain.RouteEntry {
	return &domain.RouteEntry{
		ServiceName: "test-service",
		Route: domain.Route{
			Path:   "/protected",
			Method: "GET",
			Public: false,
			Scopes: scopes,
		},
	}
}

func TestAuth_PublicRoute(t *testing.T) {
	privateKey := setupTestKeys(t)
	jwtService := createTestJWTService(t, privateKey)
	authMiddleware := NewAuthMiddleware(jwtService)

	route := createPublicRoute()
	handler := authMiddleware.Authenticate(route, "test-service")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/public", nil)

	handler(c)

	assert.False(t, c.IsAborted())
}

func TestAuth_MissingToken(t *testing.T) {
	privateKey := setupTestKeys(t)
	jwtService := createTestJWTService(t, privateKey)
	authMiddleware := NewAuthMiddleware(jwtService)

	route := createProtectedRoute()
	handler := authMiddleware.Authenticate(route, "test-service")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/protected", nil)

	handler(c)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_InvalidToken(t *testing.T) {
	privateKey := setupTestKeys(t)
	jwtService := createTestJWTService(t, privateKey)
	authMiddleware := NewAuthMiddleware(jwtService)

	route := createProtectedRoute()
	handler := authMiddleware.Authenticate(route, "test-service")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/protected", nil)
	c.Request.Header.Set("Authorization", "Bearer invalid-token")

	handler(c)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_ValidToken_GeneratesInternalToken(t *testing.T) {
	privateKey := setupTestKeys(t)
	jwtService := createTestJWTService(t, privateKey)
	authMiddleware := NewAuthMiddleware(jwtService)

	route := createProtectedRoute()

	externalClaims := &domain.ExternalClaims{
		Subject: "user-123",
		Email:   "user@example.com",
		Scopes:  []string{"read", "write"},
		Issuer:  "auth-service",
	}
	externalToken := generateExternalToken(t, privateKey, externalClaims)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/protected", nil)
	c.Request.Header.Set("Authorization", "Bearer "+externalToken)

	handler := authMiddleware.Authenticate(route, "test-service")
	handler(c)

	assert.False(t, c.IsAborted())

	userID, exists := c.Get(ContextKeyUserID)
	assert.True(t, exists)
	assert.Equal(t, "user-123", userID)

	email, exists := c.Get(ContextKeyEmail)
	assert.True(t, exists)
	assert.Equal(t, "user@example.com", email)

	newAuthHeader := c.Request.Header.Get(HeaderAuthorization)
	assert.NotEmpty(t, newAuthHeader)
	assert.True(t, len(newAuthHeader) > len("Bearer "))
	assert.NotEqual(t, "Bearer "+externalToken, newAuthHeader)

	originalIssuer := c.Request.Header.Get(HeaderOriginalIssuer)
	assert.Equal(t, "auth-service", originalIssuer)
}

func TestAuth_InsufficientScopes(t *testing.T) {
	privateKey := setupTestKeys(t)
	jwtService := createTestJWTService(t, privateKey)
	authMiddleware := NewAuthMiddleware(jwtService)

	route := createProtectedRoute("admin", "superuser")

	externalClaims := &domain.ExternalClaims{
		Subject: "user-123",
		Email:   "user@example.com",
		Scopes:  []string{"read", "write"},
		Issuer:  "auth-service",
	}
	externalToken := generateExternalToken(t, privateKey, externalClaims)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/protected", nil)
	c.Request.Header.Set("Authorization", "Bearer "+externalToken)

	handler := authMiddleware.Authenticate(route, "test-service")
	handler(c)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAuth_SufficientScopes(t *testing.T) {
	privateKey := setupTestKeys(t)
	jwtService := createTestJWTService(t, privateKey)
	authMiddleware := NewAuthMiddleware(jwtService)

	route := createProtectedRoute("read", "write")

	externalClaims := &domain.ExternalClaims{
		Subject: "user-123",
		Email:   "user@example.com",
		Scopes:  []string{"read", "write", "admin"},
		Issuer:  "auth-service",
	}
	externalToken := generateExternalToken(t, privateKey, externalClaims)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/protected", nil)
	c.Request.Header.Set("Authorization", "Bearer "+externalToken)

	handler := authMiddleware.Authenticate(route, "test-service")
	handler(c)

	assert.False(t, c.IsAborted())
}

func TestAuth_InternalTokenHasCorrectAudience(t *testing.T) {
	privateKey := setupTestKeys(t)
	jwtService := createTestJWTService(t, privateKey)
	authMiddleware := NewAuthMiddleware(jwtService)

	route := createProtectedRoute()
	serviceName := "user-service"

	externalClaims := &domain.ExternalClaims{
		Subject: "user-123",
		Email:   "user@example.com",
		Scopes:  []string{"read"},
		Issuer:  "auth-service",
	}
	externalToken := generateExternalToken(t, privateKey, externalClaims)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/protected", nil)
	c.Request.Header.Set("Authorization", "Bearer "+externalToken)

	handler := authMiddleware.Authenticate(route, serviceName)
	handler(c)

	assert.False(t, c.IsAborted())

	newAuthHeader := c.Request.Header.Get(HeaderAuthorization)
	internalTokenStr := newAuthHeader[len("Bearer "):]

	internalClaims, err := jwtService.ValidateInternalToken(internalTokenStr, serviceName)
	require.NoError(t, err)
	assert.Equal(t, serviceName, internalClaims.Audience)
	assert.Equal(t, "api-api", internalClaims.Issuer)
	assert.Equal(t, "auth-service", internalClaims.OriginalIssuer)
}

func TestAuthenticateRequest_PublicRoute(t *testing.T) {
	privateKey := setupTestKeys(t)
	jwtService := createTestJWTService(t, privateKey)
	authMiddleware := NewAuthMiddleware(jwtService)

	route := createPublicRoute()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/public", nil)

	result := authMiddleware.AuthenticateRequest(c, route, "test-service")

	assert.True(t, result)
}

func TestAuthenticateRequest_MissingToken(t *testing.T) {
	privateKey := setupTestKeys(t)
	jwtService := createTestJWTService(t, privateKey)
	authMiddleware := NewAuthMiddleware(jwtService)

	route := createProtectedRoute()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/protected", nil)

	result := authMiddleware.AuthenticateRequest(c, route, "test-service")

	assert.False(t, result)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthenticateRequest_ValidToken(t *testing.T) {
	privateKey := setupTestKeys(t)
	jwtService := createTestJWTService(t, privateKey)
	authMiddleware := NewAuthMiddleware(jwtService)

	route := createProtectedRoute()

	externalClaims := &domain.ExternalClaims{
		Subject: "user-456",
		Email:   "test@example.com",
		Scopes:  []string{"read"},
		Issuer:  "auth-service",
	}
	externalToken := generateExternalToken(t, privateKey, externalClaims)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/protected", nil)
	c.Request.Header.Set("Authorization", "Bearer "+externalToken)

	result := authMiddleware.AuthenticateRequest(c, route, "test-service")

	assert.True(t, result)

	userID, exists := c.Get(ContextKeyUserID)
	assert.True(t, exists)
	assert.Equal(t, "user-456", userID)
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected string
	}{
		{
			name:     "valid bearer token",
			header:   "Bearer abc123",
			expected: "abc123",
		},
		{
			name:     "empty header",
			header:   "",
			expected: "",
		},
		{
			name:     "no bearer prefix",
			header:   "abc123",
			expected: "",
		},
		{
			name:     "basic auth",
			header:   "Basic abc123",
			expected: "",
		},
		{
			name:     "lowercase bearer",
			header:   "bearer abc123",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/test", nil)
			if tt.header != "" {
				c.Request.Header.Set("Authorization", tt.header)
			}

			result := extractBearerToken(c)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasAllScopes(t *testing.T) {
	tests := []struct {
		name     string
		provided []string
		required []string
		expected bool
	}{
		{
			name:     "has all required scopes",
			provided: []string{"read", "write", "admin"},
			required: []string{"read", "write"},
			expected: true,
		},
		{
			name:     "missing one scope",
			provided: []string{"read"},
			required: []string{"read", "write"},
			expected: false,
		},
		{
			name:     "empty required",
			provided: []string{"read", "write"},
			required: []string{},
			expected: true,
		},
		{
			name:     "empty provided",
			provided: []string{},
			required: []string{"read"},
			expected: false,
		},
		{
			name:     "exact match",
			provided: []string{"read", "write"},
			required: []string{"read", "write"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasAllScopes(tt.provided, tt.required)
			assert.Equal(t, tt.expected, result)
		})
	}
}
