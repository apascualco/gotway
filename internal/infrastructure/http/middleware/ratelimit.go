package middleware

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/apascualco/gotway/internal/infrastructure/config"
	"github.com/apascualco/gotway/internal/infrastructure/ratelimit"
	"github.com/gin-gonic/gin"
)

// RateLimitMiddleware creates a rate limiting middleware.
func RateLimitMiddleware(limiter ratelimit.RateLimiter, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		key, limit := determineKeyAndLimit(c, cfg)

		result, err := limiter.Allow(c.Request.Context(), key, limit)
		if err != nil {
			c.Next()
			return
		}

		c.Header("X-RateLimit-Limit", strconv.Itoa(result.Limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))

		if !result.Allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":   "rate_limit_exceeded",
				"message": "too many requests, please try again later",
			})
			return
		}

		c.Next()
	}
}

func determineKeyAndLimit(c *gin.Context, cfg *config.Config) (string, int) {
	if userID, exists := c.Get("user_id"); exists {
		return fmt.Sprintf("ratelimit:user:%v", userID), cfg.RateLimitUserRPM
	}

	clientIP := c.ClientIP()
	return fmt.Sprintf("ratelimit:ip:%s", clientIP), cfg.RateLimitIPRPM
}

// RouteRateLimitMiddleware creates a rate limiting middleware for specific routes.
func RouteRateLimitMiddleware(limiter ratelimit.RateLimiter, routeLimit int) gin.HandlerFunc {
	return func(c *gin.Context) {
		var key string
		if userID, exists := c.Get("user_id"); exists {
			key = fmt.Sprintf("ratelimit:route:%s:user:%v", c.FullPath(), userID)
		} else {
			key = fmt.Sprintf("ratelimit:route:%s:ip:%s", c.FullPath(), c.ClientIP())
		}

		result, err := limiter.Allow(c.Request.Context(), key, routeLimit)
		if err != nil {
			c.Next()
			return
		}

		c.Header("X-RateLimit-Limit", strconv.Itoa(result.Limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))

		if !result.Allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":   "rate_limit_exceeded",
				"message": "too many requests for this endpoint, please try again later",
			})
			return
		}

		c.Next()
	}
}
