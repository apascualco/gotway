package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if path == "/health" || path == "/ready" {
			c.Next()
			return
		}

		start := time.Now()
		query := c.Request.URL.RawQuery
		method := c.Request.Method

		c.Next()

		duration := time.Since(start)
		status := c.Writer.Status()
		requestID, _ := c.Get("request_id")

		attrs := []any{
			"method", method,
			"path", path,
			"status", status,
			"duration", duration.String(),
			"request_id", requestID,
			"client_ip", c.ClientIP(),
		}

		if query != "" {
			attrs = append(attrs, "query", query)
		}

		if len(c.Errors) > 0 {
			attrs = append(attrs, "errors", c.Errors.String())
		}

		switch {
		case status >= 500:
			slog.Error("request completed", attrs...)
		case status >= 400:
			slog.Warn("request completed", attrs...)
		default:
			slog.Info("request completed", attrs...)
		}
	}
}
