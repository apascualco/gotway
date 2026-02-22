package middleware

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/apascualco/gotway/internal/infrastructure/tracing"
	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

var validTraceparent = regexp.MustCompile(`^00-[0-9a-f]{32}-[0-9a-f]{16}-[0-9a-f]{2}$`)

func TestTraceContext_NoTraceparent_GeneratesNew(t *testing.T) {
	router := gin.New()
	router.Use(TraceMiddleware(NewW3CTraceProvider(), &tracing.NoopExporter{}))
	router.GET("/test", func(c *gin.Context) {
		traceID, _ := c.Get("trace_id")
		spanID, _ := c.Get("span_id")

		if traceID == "" {
			t.Error("expected trace_id to be set")
		}
		if spanID == "" {
			t.Error("expected span_id to be set")
		}

		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	traceparent := w.Header().Get("Traceparent")
	if traceparent == "" {
		t.Fatal("expected Traceparent response header")
	}
	if !validTraceparent.MatchString(traceparent) {
		t.Errorf("invalid traceparent format: %s", traceparent)
	}
}

func TestTraceContext_ValidTraceparent_PreservesTraceID(t *testing.T) {
	originalTraceID := "abcdef1234567890abcdef1234567890"
	originalSpanID := "1234567890abcdef"
	incoming := "00-" + originalTraceID + "-" + originalSpanID + "-01"

	router := gin.New()
	router.Use(TraceMiddleware(NewW3CTraceProvider(), &tracing.NoopExporter{}))
	router.GET("/test", func(c *gin.Context) {
		traceID, _ := c.Get("trace_id")
		spanID, _ := c.Get("span_id")

		if traceID != originalTraceID {
			t.Errorf("expected trace_id %s, got %s", originalTraceID, traceID)
		}
		if spanID == originalSpanID {
			t.Error("expected a new span_id, got the original")
		}
		if spanID == "" {
			t.Error("expected span_id to be set")
		}

		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Traceparent", incoming)
	router.ServeHTTP(w, req)

	traceparent := w.Header().Get("Traceparent")
	parts := strings.Split(traceparent, "-")
	if len(parts) != 4 {
		t.Fatalf("expected 4 parts in traceparent, got %d: %s", len(parts), traceparent)
	}
	if parts[1] != originalTraceID {
		t.Errorf("response traceparent should preserve trace_id %s, got %s", originalTraceID, parts[1])
	}
	if parts[2] == originalSpanID {
		t.Errorf("response traceparent should have new span_id, got original: %s", parts[2])
	}
}

func TestTraceContext_InvalidTraceparent_GeneratesNew(t *testing.T) {
	cases := []struct {
		name        string
		traceparent string
	}{
		{"wrong version", "01-abcdef1234567890abcdef1234567890-1234567890abcdef-01"},
		{"short trace_id", "00-abcdef-1234567890abcdef-01"},
		{"not hex", "00-zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz-1234567890abcdef-01"},
		{"empty", ""},
		{"garbage", "not-a-traceparent"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.Use(TraceMiddleware(NewW3CTraceProvider(), &tracing.NoopExporter{}))
			router.GET("/test", func(c *gin.Context) {
				traceID, _ := c.Get("trace_id")
				if traceID == "" {
					t.Error("expected trace_id to be generated")
				}
				c.Status(http.StatusOK)
			})

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			if tc.traceparent != "" {
				req.Header.Set("Traceparent", tc.traceparent)
			}
			router.ServeHTTP(w, req)

			traceparent := w.Header().Get("Traceparent")
			if !validTraceparent.MatchString(traceparent) {
				t.Errorf("expected valid traceparent in response, got: %s", traceparent)
			}
		})
	}
}

func TestTraceContext_AllZerosTraceID_GeneratesNew(t *testing.T) {
	zeroTrace := "00-00000000000000000000000000000000-1234567890abcdef-01"

	router := gin.New()
	router.Use(TraceMiddleware(NewW3CTraceProvider(), &tracing.NoopExporter{}))
	router.GET("/test", func(c *gin.Context) {
		traceID, _ := c.Get("trace_id")
		if traceID == strings.Repeat("0", 32) {
			t.Error("trace_id should not be all zeros")
		}
		if traceID == "" {
			t.Error("expected trace_id to be generated")
		}
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Traceparent", zeroTrace)
	router.ServeHTTP(w, req)

	traceparent := w.Header().Get("Traceparent")
	if strings.Contains(traceparent, strings.Repeat("0", 32)) {
		t.Errorf("response traceparent should not contain all-zero trace_id, got: %s", traceparent)
	}
}

func TestTraceContext_TracestateIsPropagated(t *testing.T) {
	incoming := "00-abcdef1234567890abcdef1234567890-1234567890abcdef-01"
	tracestate := "vendor1=value1,vendor2=value2"

	router := gin.New()
	router.Use(TraceMiddleware(NewW3CTraceProvider(), &tracing.NoopExporter{}))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Traceparent", incoming)
	req.Header.Set("Tracestate", tracestate)
	router.ServeHTTP(w, req)

	responseTracestate := w.Header().Get("Tracestate")
	if responseTracestate != tracestate {
		t.Errorf("expected tracestate %q, got %q", tracestate, responseTracestate)
	}
}

func TestTraceContext_AllZerosSpanID_GeneratesNew(t *testing.T) {
	zeroSpan := "00-abcdef1234567890abcdef1234567890-0000000000000000-01"

	router := gin.New()
	router.Use(TraceMiddleware(NewW3CTraceProvider(), &tracing.NoopExporter{}))
	router.GET("/test", func(c *gin.Context) {
		traceID, _ := c.Get("trace_id")
		if traceID == "" {
			t.Error("expected trace_id to be generated")
		}
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Traceparent", zeroSpan)
	router.ServeHTTP(w, req)

	traceparent := w.Header().Get("Traceparent")
	if !validTraceparent.MatchString(traceparent) {
		t.Errorf("expected valid traceparent, got: %s", traceparent)
	}
}
