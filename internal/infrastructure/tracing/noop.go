package tracing

import "context"

type NoopExporter struct{}

func (n *NoopExporter) Export(_ context.Context, _ SpanData) {}

func (n *NoopExporter) Shutdown(_ context.Context) error { return nil }
