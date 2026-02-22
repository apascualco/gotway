package tracing

import (
	"bytes"
	"context"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

const (
	defaultBufferSize    = 1024
	defaultBatchSize     = 64
	defaultFlushInterval = 5 * time.Second
)

type OTLPExporter struct {
	endpoint    string
	serviceName string
	client      *http.Client
	spans       chan SpanData
	done        chan struct{}
	wg          sync.WaitGroup
}

func NewOTLPExporter(endpoint, serviceName string) *OTLPExporter {
	e := &OTLPExporter{
		endpoint:    endpoint,
		serviceName: serviceName,
		client:      &http.Client{Timeout: 10 * time.Second},
		spans:       make(chan SpanData, defaultBufferSize),
		done:        make(chan struct{}),
	}
	e.wg.Add(1)
	go e.batchLoop()
	return e
}

func (e *OTLPExporter) Export(_ context.Context, span SpanData) {
	select {
	case e.spans <- span:
	default:
		slog.Warn("otlp exporter: span dropped, buffer full")
	}
}

func (e *OTLPExporter) Shutdown(ctx context.Context) error {
	close(e.done)

	finished := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(finished)
	}()

	select {
	case <-finished:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (e *OTLPExporter) batchLoop() {
	defer e.wg.Done()

	ticker := time.NewTicker(defaultFlushInterval)
	defer ticker.Stop()

	batch := make([]SpanData, 0, defaultBatchSize)

	for {
		select {
		case span := <-e.spans:
			batch = append(batch, span)
			if len(batch) >= defaultBatchSize {
				e.flush(batch)
				batch = make([]SpanData, 0, defaultBatchSize)
			}
		case <-ticker.C:
			if len(batch) > 0 {
				e.flush(batch)
				batch = make([]SpanData, 0, defaultBatchSize)
			}
		case <-e.done:
			// Drain remaining spans from channel
			for {
				select {
				case span := <-e.spans:
					batch = append(batch, span)
					if len(batch) >= defaultBatchSize {
						e.flush(batch)
						batch = make([]SpanData, 0, defaultBatchSize)
					}
				default:
					if len(batch) > 0 {
						e.flush(batch)
					}
					return
				}
			}
		}
	}
}

func (e *OTLPExporter) flush(batch []SpanData) {
	data := e.buildProto(batch)
	body, err := proto.Marshal(data)
	if err != nil {
		slog.Error("otlp exporter: failed to marshal protobuf", slog.String("error", err.Error()))
		return
	}

	url := e.endpoint + "/v1/traces"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		slog.Error("otlp exporter: failed to create request", slog.String("error", err.Error()))
		return
	}
	req.Header.Set("Content-Type", "application/x-protobuf")

	resp, err := e.client.Do(req)
	if err != nil {
		slog.Error("otlp exporter: failed to send spans",
			slog.String("error", err.Error()),
			slog.Int("count", len(batch)),
		)
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(resp.Body)

	if resp.StatusCode >= 300 {
		slog.Warn("otlp exporter: unexpected status",
			slog.Int("status", resp.StatusCode),
			slog.Int("count", len(batch)),
		)
	}
}

func (e *OTLPExporter) buildProto(batch []SpanData) *tracepb.TracesData {
	spans := make([]*tracepb.Span, 0, len(batch))
	for _, s := range batch {
		spans = append(spans, spanDataToProto(s))
	}

	return &tracepb.TracesData{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{
							Key:   "service.name",
							Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: e.serviceName}},
						},
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Scope: &commonpb.InstrumentationScope{
							Name:    "gotway",
							Version: "1.0.0",
						},
						Spans: spans,
					},
				},
			},
		},
	}
}

func spanDataToProto(s SpanData) *tracepb.Span {
	traceID, _ := hex.DecodeString(s.TraceID)
	spanID, _ := hex.DecodeString(s.SpanID)

	span := &tracepb.Span{
		TraceId:           traceID,
		SpanId:            spanID,
		Name:              s.Name,
		Kind:              toProtoSpanKind(s.Kind),
		StartTimeUnixNano: uint64(s.StartTime.UnixNano()),
		EndTimeUnixNano:   uint64(s.EndTime.UnixNano()),
		Status:            toProtoStatus(s.StatusCode),
		Attributes:        toProtoAttributes(s.Attributes),
	}

	if s.ParentSpanID != "" {
		parentID, _ := hex.DecodeString(s.ParentSpanID)
		span.ParentSpanId = parentID
	}

	return span
}

func toProtoSpanKind(k SpanKind) tracepb.Span_SpanKind {
	switch k {
	case SpanKindServer:
		return tracepb.Span_SPAN_KIND_SERVER
	case SpanKindClient:
		return tracepb.Span_SPAN_KIND_CLIENT
	default:
		return tracepb.Span_SPAN_KIND_UNSPECIFIED
	}
}

func toProtoStatus(httpStatus int) *tracepb.Status {
	if httpStatus >= 500 {
		return &tracepb.Status{Code: tracepb.Status_STATUS_CODE_ERROR}
	}
	return &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK}
}

func toProtoAttributes(attrs map[string]string) []*commonpb.KeyValue {
	if len(attrs) == 0 {
		return nil
	}
	kvs := make([]*commonpb.KeyValue, 0, len(attrs))
	for k, v := range attrs {
		kvs = append(kvs, &commonpb.KeyValue{
			Key:   k,
			Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: v}},
		})
	}
	return kvs
}
