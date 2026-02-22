package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"
)

type TraceContext struct {
	TraceID  string
	SpanID   string
	ParentID string
	Flags    string
	State    string
}

type TraceProvider interface {
	Extract(c *gin.Context) *TraceContext
	Inject(c *gin.Context, tc *TraceContext)
}

func TraceMiddleware(provider TraceProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		tc := provider.Extract(c)

		if tc.TraceID == "" {
			tc.TraceID = generateTraceID()
		}

		tc.ParentID = tc.SpanID
		tc.SpanID = generateSpanID()

		if tc.Flags == "" {
			tc.Flags = "01"
		}

		c.Set("trace_id", tc.TraceID)
		c.Set("span_id", tc.SpanID)
		c.Set("trace_flags", tc.Flags)
		if tc.State != "" {
			c.Set("trace_state", tc.State)
		}

		provider.Inject(c, tc)

		c.Next()
	}
}

func generateTraceID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func generateSpanID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
