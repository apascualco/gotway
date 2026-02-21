package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apascualco/gotway/internal/infrastructure/config"
	"github.com/apascualco/gotway/internal/infrastructure/ratelimit"
	"github.com/gin-gonic/gin"
)

func TestRateLimitMiddleware_AllowsUnderLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	limiter := ratelimit.NewInMemoryLimiter()
	cfg := &config.Config{
		RateLimitIPRPM:   10,
		RateLimitUserRPM: 100,
	}

	router := gin.New()
	router.Use(RateLimitMiddleware(limiter, cfg))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected status 200, got %d", i, w.Code)
		}

		if w.Header().Get("X-RateLimit-Limit") == "" {
			t.Error("missing X-RateLimit-Limit header")
		}

		if w.Header().Get("X-RateLimit-Remaining") == "" {
			t.Error("missing X-RateLimit-Remaining header")
		}

		if w.Header().Get("X-RateLimit-Reset") == "" {
			t.Error("missing X-RateLimit-Reset header")
		}
	}
}

func TestRateLimitMiddleware_BlocksOverLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	limiter := ratelimit.NewInMemoryLimiter()
	cfg := &config.Config{
		RateLimitIPRPM:   3,
		RateLimitUserRPM: 100,
	}

	router := gin.New()
	router.Use(RateLimitMiddleware(limiter, cfg))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected status 200, got %d", i, w.Code)
		}
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	router.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", w.Code)
	}

	if w.Header().Get("X-RateLimit-Remaining") != "0" {
		t.Errorf("X-RateLimit-Remaining should be 0, got %s", w.Header().Get("X-RateLimit-Remaining"))
	}
}

func TestRateLimitMiddleware_DifferentIPsHaveDifferentLimits(t *testing.T) {
	gin.SetMode(gin.TestMode)

	limiter := ratelimit.NewInMemoryLimiter()
	cfg := &config.Config{
		RateLimitIPRPM:   2,
		RateLimitUserRPM: 100,
	}

	router := gin.New()
	router.Use(RateLimitMiddleware(limiter, cfg))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		router.ServeHTTP(w, req)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	router.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("IP 1 should be rate limited, got status %d", w.Code)
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.2:12345"
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("IP 2 should be allowed, got status %d", w.Code)
	}
}

func TestRateLimitMiddleware_UsesUserIDWhenAuthenticated(t *testing.T) {
	gin.SetMode(gin.TestMode)

	limiter := ratelimit.NewInMemoryLimiter()
	cfg := &config.Config{
		RateLimitIPRPM:   2,
		RateLimitUserRPM: 5,
	}

	router := gin.New()

	router.Use(func(c *gin.Context) {
		c.Set("user_id", "user123")
		c.Next()
	})

	router.Use(RateLimitMiddleware(limiter, cfg))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("request %d should be allowed with user rate limit, got %d", i, w.Code)
		}
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	router.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("6th request should be rate limited, got %d", w.Code)
	}
}

func TestRouteRateLimitMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	limiter := ratelimit.NewInMemoryLimiter()

	router := gin.New()
	router.GET("/limited", RouteRateLimitMiddleware(limiter, 2), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/unlimited", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/limited", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		router.ServeHTTP(w, req)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/limited", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	router.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("/limited should be rate limited, got %d", w.Code)
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/unlimited", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("/unlimited should not be affected, got %d", w.Code)
	}
}
