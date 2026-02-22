package tracing

import (
	"context"
	"log/slog"
	"time"

	"github.com/apascualco/gotway/internal/infrastructure/config"
)

type SpanKind int

const (
	SpanKindServer SpanKind = iota
	SpanKindClient
)

func (k SpanKind) String() string {
	switch k {
	case SpanKindServer:
		return "SERVER"
	case SpanKindClient:
		return "CLIENT"
	default:
		return "UNSPECIFIED"
	}
}

type SpanData struct {
	TraceID      string
	SpanID       string
	ParentSpanID string
	Name         string
	ServiceName  string
	Kind         SpanKind
	StartTime    time.Time
	EndTime      time.Time
	StatusCode   int
	Attributes   map[string]string
}

type SpanExporter interface {
	Export(ctx context.Context, span SpanData)
	Shutdown(ctx context.Context) error
}

func NewExporter(cfg *config.Config) SpanExporter {
	switch cfg.TraceExporter {
	case "otlp":
		if cfg.TraceOTLPEndpoint == "" {
			slog.Warn("TRACE_EXPORTER=otlp but TRACE_OTLP_ENDPOINT is empty, falling back to noop")
			return &NoopExporter{}
		}
		slog.Info("trace exporter enabled",
			slog.String("exporter", "otlp"),
			slog.String("endpoint", cfg.TraceOTLPEndpoint),
			slog.String("service_name", cfg.TraceServiceName),
		)
		return NewOTLPExporter(cfg.TraceOTLPEndpoint, cfg.TraceServiceName)
	default:
		slog.Debug("trace exporter disabled (noop)")
		return &NoopExporter{}
	}
}
