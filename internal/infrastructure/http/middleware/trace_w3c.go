package middleware

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	HeaderTraceparent = "Traceparent"
	HeaderTracestate  = "Tracestate"
)

var traceparentRegex = regexp.MustCompile(`^00-([0-9a-f]{32})-([0-9a-f]{16})-([0-9a-f]{2})$`)

var zeroTraceID = strings.Repeat("0", 32)
var zeroSpanID = strings.Repeat("0", 16)

type W3CTraceProvider struct{}

func NewW3CTraceProvider() *W3CTraceProvider {
	return &W3CTraceProvider{}
}

func (w *W3CTraceProvider) Extract(c *gin.Context) *TraceContext {
	tc := &TraceContext{}

	traceparent := c.GetHeader(HeaderTraceparent)
	if traceparent != "" {
		matches := traceparentRegex.FindStringSubmatch(traceparent)
		if len(matches) == 4 {
			traceID := matches[1]
			spanID := matches[2]
			flags := matches[3]

			if traceID != zeroTraceID && spanID != zeroSpanID {
				tc.TraceID = traceID
				tc.SpanID = spanID
				tc.Flags = flags
			}
		}
	}

	if tracestate := c.GetHeader(HeaderTracestate); tracestate != "" {
		tc.State = tracestate
	}

	return tc
}

func (w *W3CTraceProvider) Inject(c *gin.Context, tc *TraceContext) {
	traceparent := fmt.Sprintf("00-%s-%s-%s", tc.TraceID, tc.SpanID, tc.Flags)
	c.Header(HeaderTraceparent, traceparent)

	if tc.State != "" {
		c.Header(HeaderTracestate, tc.State)
	}
}
