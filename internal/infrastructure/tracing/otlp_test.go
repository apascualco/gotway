package tracing

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

func TestSpanDataToProto_BasicConversion(t *testing.T) {
	start := time.Now()
	end := start.Add(100 * time.Millisecond)

	span := SpanData{
		TraceID:      "abcdef1234567890abcdef1234567890",
		SpanID:       "1234567890abcdef",
		ParentSpanID: "fedcba0987654321",
		Name:         "GET /test",
		Kind:         SpanKindServer,
		StartTime:    start,
		EndTime:      end,
		StatusCode:   200,
		Attributes: map[string]string{
			"http.method": "GET",
			"http.url":    "/test",
		},
	}

	protoSpan := spanDataToProto(span)

	if protoSpan.Name != "GET /test" {
		t.Errorf("expected name 'GET /test', got %q", protoSpan.Name)
	}

	if len(protoSpan.TraceId) != 16 {
		t.Errorf("expected 16-byte trace_id, got %d bytes", len(protoSpan.TraceId))
	}

	if len(protoSpan.SpanId) != 8 {
		t.Errorf("expected 8-byte span_id, got %d bytes", len(protoSpan.SpanId))
	}

	if len(protoSpan.ParentSpanId) != 8 {
		t.Errorf("expected 8-byte parent_span_id, got %d bytes", len(protoSpan.ParentSpanId))
	}

	if protoSpan.Kind != tracepb.Span_SPAN_KIND_SERVER {
		t.Errorf("expected SPAN_KIND_SERVER, got %v", protoSpan.Kind)
	}

	if protoSpan.StartTimeUnixNano != uint64(start.UnixNano()) {
		t.Errorf("expected start_time %d, got %d", start.UnixNano(), protoSpan.StartTimeUnixNano)
	}

	if protoSpan.EndTimeUnixNano != uint64(end.UnixNano()) {
		t.Errorf("expected end_time %d, got %d", end.UnixNano(), protoSpan.EndTimeUnixNano)
	}

	if protoSpan.Status.Code != tracepb.Status_STATUS_CODE_OK {
		t.Errorf("expected STATUS_CODE_OK, got %v", protoSpan.Status.Code)
	}

	if len(protoSpan.Attributes) != 2 {
		t.Errorf("expected 2 attributes, got %d", len(protoSpan.Attributes))
	}
}

func TestSpanDataToProto_NoParentSpan(t *testing.T) {
	span := SpanData{
		TraceID:   "abcdef1234567890abcdef1234567890",
		SpanID:    "1234567890abcdef",
		Name:      "root span",
		Kind:      SpanKindServer,
		StartTime: time.Now(),
		EndTime:   time.Now(),
	}

	protoSpan := spanDataToProto(span)

	if len(protoSpan.ParentSpanId) != 0 {
		t.Errorf("expected empty parent_span_id for root span, got %x", protoSpan.ParentSpanId)
	}
}

func TestSpanDataToProto_ErrorStatus(t *testing.T) {
	span := SpanData{
		TraceID:    "abcdef1234567890abcdef1234567890",
		SpanID:     "1234567890abcdef",
		Name:       "failing request",
		StartTime:  time.Now(),
		EndTime:    time.Now(),
		StatusCode: 500,
	}

	protoSpan := spanDataToProto(span)

	if protoSpan.Status.Code != tracepb.Status_STATUS_CODE_ERROR {
		t.Errorf("expected STATUS_CODE_ERROR for 500, got %v", protoSpan.Status.Code)
	}
}

func TestOTLPExporter_BuildProto(t *testing.T) {
	e := &OTLPExporter{
		endpoint:    "http://localhost:4318",
		serviceName: "test-service",
	}

	batch := []SpanData{
		{
			TraceID:   "abcdef1234567890abcdef1234567890",
			SpanID:    "1234567890abcdef",
			Name:      "span-1",
			Kind:      SpanKindServer,
			StartTime: time.Now(),
			EndTime:   time.Now(),
		},
		{
			TraceID:   "abcdef1234567890abcdef1234567890",
			SpanID:    "abcdef1234567890",
			Name:      "span-2",
			Kind:      SpanKindClient,
			StartTime: time.Now(),
			EndTime:   time.Now(),
		},
	}

	data := e.buildProto(batch)

	if len(data.ResourceSpans) != 1 {
		t.Fatalf("expected 1 ResourceSpans, got %d", len(data.ResourceSpans))
	}

	rs := data.ResourceSpans[0]

	// Verify service.name attribute
	found := false
	for _, kv := range rs.Resource.Attributes {
		if kv.Key == "service.name" && kv.Value.GetStringValue() == "test-service" {
			found = true
		}
	}
	if !found {
		t.Error("expected service.name=test-service in resource attributes")
	}

	if len(rs.ScopeSpans) != 1 {
		t.Fatalf("expected 1 ScopeSpans, got %d", len(rs.ScopeSpans))
	}

	if len(rs.ScopeSpans[0].Spans) != 2 {
		t.Errorf("expected 2 spans, got %d", len(rs.ScopeSpans[0].Spans))
	}

	// Verify protobuf marshals without error
	body, err := proto.Marshal(data)
	if err != nil {
		t.Fatalf("failed to marshal protobuf: %v", err)
	}
	if len(body) == 0 {
		t.Error("expected non-empty protobuf body")
	}
}

func TestOTLPExporter_FlushToHTTPServer(t *testing.T) {
	var mu sync.Mutex
	var receivedBody []byte
	var receivedContentType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		receivedContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		receivedBody = body
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	exporter := NewOTLPExporter(server.URL, "test-service")

	for i := 0; i < 5; i++ {
		exporter.Export(context.Background(), SpanData{
			TraceID:    "abcdef1234567890abcdef1234567890",
			SpanID:     "1234567890abcdef",
			Name:       "test-span",
			Kind:       SpanKindServer,
			StartTime:  time.Now(),
			EndTime:    time.Now(),
			StatusCode: 200,
		})
	}

	// Shutdown triggers drain of remaining spans
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := exporter.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if receivedContentType != "application/x-protobuf" {
		t.Errorf("expected Content-Type 'application/x-protobuf', got %q", receivedContentType)
	}

	if len(receivedBody) == 0 {
		t.Fatal("expected non-empty body")
	}

	// Verify received protobuf can be unmarshaled
	var traces tracepb.TracesData
	if err := proto.Unmarshal(receivedBody, &traces); err != nil {
		t.Fatalf("failed to unmarshal received protobuf: %v", err)
	}

	if len(traces.ResourceSpans) == 0 {
		t.Fatal("expected at least one ResourceSpans")
	}

	spans := traces.ResourceSpans[0].ScopeSpans[0].Spans
	if len(spans) != 5 {
		t.Errorf("expected 5 spans, got %d", len(spans))
	}
}

func TestOTLPExporter_BatchFlush(t *testing.T) {
	var mu sync.Mutex
	var flushCount int
	var totalSpans int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var traces tracepb.TracesData
		if err := proto.Unmarshal(body, &traces); err == nil {
			mu.Lock()
			flushCount++
			for _, rs := range traces.ResourceSpans {
				for _, ss := range rs.ScopeSpans {
					totalSpans += len(ss.Spans)
				}
			}
			mu.Unlock()
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	exporter := NewOTLPExporter(server.URL, "test-service")

	// Send enough spans to trigger at least one batch flush (defaultBatchSize = 64)
	for i := 0; i < 70; i++ {
		exporter.Export(context.Background(), SpanData{
			TraceID:    "abcdef1234567890abcdef1234567890",
			SpanID:     "1234567890abcdef",
			Name:       "batch-span",
			Kind:       SpanKindServer,
			StartTime:  time.Now(),
			EndTime:    time.Now(),
			StatusCode: 200,
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := exporter.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if flushCount < 2 {
		t.Errorf("expected at least 2 flushes (64 batch + remainder), got %d", flushCount)
	}

	if totalSpans != 70 {
		t.Errorf("expected 70 total spans, got %d", totalSpans)
	}
}

func TestOTLPExporter_BufferFull_DropsSpan(t *testing.T) {
	// Create exporter with a server that never responds quickly
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	exporter := &OTLPExporter{
		endpoint:    server.URL,
		serviceName: "test",
		client:      &http.Client{Timeout: 10 * time.Second},
		spans:       make(chan SpanData, 2), // tiny buffer
		done:        make(chan struct{}),
	}
	// Don't start batchLoop so channel fills up

	span := SpanData{
		TraceID:   "abcdef1234567890abcdef1234567890",
		SpanID:    "1234567890abcdef",
		Name:      "test",
		StartTime: time.Now(),
		EndTime:   time.Now(),
	}

	// Fill buffer
	exporter.Export(context.Background(), span)
	exporter.Export(context.Background(), span)

	// This should not block (drops silently)
	done := make(chan struct{})
	go func() {
		exporter.Export(context.Background(), span)
		close(done)
	}()

	select {
	case <-done:
		// OK - export returned without blocking
	case <-time.After(time.Second):
		t.Fatal("Export blocked when buffer was full")
	}
}
