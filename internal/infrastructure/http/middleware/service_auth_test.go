package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/apascualco/gotway/internal/infrastructure/jwt"
	"github.com/gin-gonic/gin"
	jwtlib "github.com/golang-jwt/jwt/v5"
)

func setupServiceAuthKeys() (*rsa.PrivateKey, *rsa.PublicKey) {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	return privateKey, &privateKey.PublicKey
}

func createServiceAuthJWTService(publicKey *rsa.PublicKey) *jwt.Service {
	return jwt.NewServiceWithKeys(nil, publicKey, "api-gateway", 5*time.Minute, nil)
}

func signServiceToken(privateKey *rsa.PrivateKey, serviceName string) string {
	now := time.Now()
	claims := jwtlib.MapClaims{
		"sub": serviceName,
		"aud": "api-gateway",
		"iss": serviceName,
		"iat": now.Unix(),
		"exp": now.Add(5 * time.Minute).Unix(),
	}
	token := jwtlib.NewWithClaims(jwtlib.SigningMethodRS256, claims)
	tokenString, _ := token.SignedString(privateKey)
	return tokenString
}

func TestServiceAuthMiddleware_ValidToken(t *testing.T) {
	privateKey, publicKey := setupServiceAuthKeys()
	jwtService := createServiceAuthJWTService(publicKey)
	middleware := NewServiceAuthMiddleware(jwtService)

	router := gin.New()
	router.Use(middleware.Authenticate())
	router.GET("/test", func(c *gin.Context) {
		serviceName := c.GetString(ContextKeyServiceName)
		c.JSON(http.StatusOK, gin.H{"service_name": serviceName})
	})

	token := signServiceToken(privateKey, "my-service")

	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set(HeaderServiceToken, token)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err == nil {
		if body["service_name"] != "my-service" {
			t.Errorf("expected service_name 'my-service', got %q", body["service_name"])
		}
	}
}

func TestServiceAuthMiddleware_MissingToken(t *testing.T) {
	_, publicKey := setupServiceAuthKeys()
	jwtService := createServiceAuthJWTService(publicKey)
	middleware := NewServiceAuthMiddleware(jwtService)

	router := gin.New()
	router.Use(middleware.Authenticate())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", resp.Code)
	}
}

func TestServiceAuthMiddleware_InvalidToken(t *testing.T) {
	_, publicKey := setupServiceAuthKeys()
	jwtService := createServiceAuthJWTService(publicKey)
	middleware := NewServiceAuthMiddleware(jwtService)

	router := gin.New()
	router.Use(middleware.Authenticate())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set(HeaderServiceToken, "invalid-token")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", resp.Code)
	}
}

func TestServiceAuthMiddleware_WrongKey(t *testing.T) {
	_, publicKey := setupServiceAuthKeys()
	jwtService := createServiceAuthJWTService(publicKey)
	middleware := NewServiceAuthMiddleware(jwtService)

	differentKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	token := signServiceToken(differentKey, "my-service")

	router := gin.New()
	router.Use(middleware.Authenticate())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set(HeaderServiceToken, token)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", resp.Code)
	}
}

func TestServiceAuthMiddleware_ExpiredToken(t *testing.T) {
	privateKey, publicKey := setupServiceAuthKeys()
	jwtService := createServiceAuthJWTService(publicKey)
	middleware := NewServiceAuthMiddleware(jwtService)

	claims := jwtlib.MapClaims{
		"sub": "my-service",
		"aud": "api-gateway",
		"iss": "my-service",
		"iat": time.Now().Add(-10 * time.Minute).Unix(),
		"exp": time.Now().Add(-5 * time.Minute).Unix(),
	}
	token := jwtlib.NewWithClaims(jwtlib.SigningMethodRS256, claims)
	tokenString, _ := token.SignedString(privateKey)

	router := gin.New()
	router.Use(middleware.Authenticate())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set(HeaderServiceToken, tokenString)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", resp.Code)
	}
}

func TestServiceAuthMiddleware_WrongAudience(t *testing.T) {
	privateKey, publicKey := setupServiceAuthKeys()
	jwtService := createServiceAuthJWTService(publicKey)
	middleware := NewServiceAuthMiddleware(jwtService)

	claims := jwtlib.MapClaims{
		"sub": "my-service",
		"aud": "wrong-audience",
		"iss": "my-service",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(5 * time.Minute).Unix(),
	}
	token := jwtlib.NewWithClaims(jwtlib.SigningMethodRS256, claims)
	tokenString, _ := token.SignedString(privateKey)

	router := gin.New()
	router.Use(middleware.Authenticate())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set(HeaderServiceToken, tokenString)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", resp.Code)
	}
}
