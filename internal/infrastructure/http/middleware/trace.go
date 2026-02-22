package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/apascualco/gotway/internal/infrastructure/tracing"
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

func TraceMiddleware(provider TraceProvider, exporter tracing.SpanExporter) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

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

		exporter.Export(context.Background(), tracing.SpanData{
			TraceID:      tc.TraceID,
			SpanID:       tc.SpanID,
			ParentSpanID: tc.ParentID,
			Name:         fmt.Sprintf("%s %s", c.Request.Method, c.FullPath()),
			Kind:         tracing.SpanKindServer,
			StartTime:    start,
			EndTime:      time.Now(),
			StatusCode:   c.Writer.Status(),
			Attributes: map[string]string{
				"http.method":      c.Request.Method,
				"http.url":         c.Request.URL.String(),
				"http.status_code": fmt.Sprintf("%d", c.Writer.Status()),
				"http.route":       c.FullPath(),
				"net.peer.ip":      c.ClientIP(),
			},
		})
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
