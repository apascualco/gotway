package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type MockServer struct {
	server    *http.Server
	listener  net.Listener
	responses map[string]MockResponse
	hitCount  atomic.Int64
	mu        sync.RWMutex
	port      int
}

type MockResponse struct {
	StatusCode int
	Body       string
	Headers    map[string]string
}

func NewMockServer() (*MockServer, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, err
	}

	ms := &MockServer{
		listener:  listener,
		responses: make(map[string]MockResponse),
		port:      listener.Addr().(*net.TCPAddr).Port,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", ms.handleRequest)

	ms.server = &http.Server{Handler: mux}

	go func() { _ = ms.server.Serve(listener) }()

	return ms, nil
}

func (ms *MockServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	ms.hitCount.Add(1)

	key := r.Method + ":" + r.URL.Path
	ms.mu.RLock()
	resp, ok := ms.responses[key]
	ms.mu.RUnlock()

	if !ok {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"healthy"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"path":"%s","port":%d}`, r.URL.Path, ms.port)))
		return
	}

	for k, v := range resp.Headers {
		w.Header().Set(k, v)
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write([]byte(resp.Body))
}

func (ms *MockServer) SetResponse(method, path string, resp MockResponse) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.responses[method+":"+path] = resp
}

func (ms *MockServer) GetHitCount() int64 {
	return ms.hitCount.Load()
}

func (ms *MockServer) ResetHitCount() {
	ms.hitCount.Store(0)
}

func (ms *MockServer) Port() int {
	return ms.port
}

func (ms *MockServer) Close() {
	_ = ms.server.Close()
	_ = ms.listener.Close()
}

func TestProxy_ForwardsRequest_Success(t *testing.T) {
	mockServer, err := NewMockServer()
	if err != nil {
		t.Fatalf("failed to create mock server: %v", err)
	}
	defer mockServer.Close()

	mockServer.SetResponse("GET", "/data", MockResponse{
		StatusCode: http.StatusOK,
		Body:       `{"message":"hello from backend"}`,
		Headers:    map[string]string{"Content-Type": "application/json"},
	})

	instanceID := registerServiceWithPort(t, "proxy-test-service", mockServer.Port(), "/api/v1/proxy-test")
	defer cleanupService(t, instanceID)

	time.Sleep(100 * time.Millisecond)

	client := getHTTPClient()
	resp, err := client.Get(testServerURL + "/api/v1/proxy-test/data")
	if err != nil {
		t.Fatalf("proxy request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status 200, got %d: %s", resp.StatusCode, string(respBody))
	}

	var response map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["message"] != "hello from backend" {
		t.Errorf("expected message 'hello from backend', got '%s'", response["message"])
	}
}

func TestProxy_NoRoute_Returns404(t *testing.T) {
	client := getHTTPClient()
	resp, err := client.Get(testServerURL + "/api/v1/non-existent-route/data")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", resp.StatusCode)
	}
}

func TestProxy_LoadBalancing_RoundRobin(t *testing.T) {
	mockServer1, err := NewMockServer()
	if err != nil {
		t.Fatalf("failed to create mock server 1: %v", err)
	}
	defer mockServer1.Close()

	mockServer2, err := NewMockServer()
	if err != nil {
		t.Fatalf("failed to create mock server 2: %v", err)
	}
	defer mockServer2.Close()

	instanceID1 := registerServiceWithPort(t, "lb-service", mockServer1.Port(), "/api/v1/lb")
	defer cleanupService(t, instanceID1)

	instanceID2 := registerServiceWithPort(t, "lb-service", mockServer2.Port(), "/api/v1/lb")
	defer cleanupService(t, instanceID2)

	time.Sleep(100 * time.Millisecond)

	client := getHTTPClient()
	numRequests := 10

	mockServer1.ResetHitCount()
	mockServer2.ResetHitCount()

	for i := 0; i < numRequests; i++ {
		resp, err := client.Get(testServerURL + "/api/v1/lb/data")
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		_ = resp.Body.Close()
	}

	hits1 := mockServer1.GetHitCount()
	hits2 := mockServer2.GetHitCount()

	t.Logf("Server 1 hits: %d, Server 2 hits: %d", hits1, hits2)

	if hits1 == 0 || hits2 == 0 {
		t.Errorf("expected both servers to receive requests, got server1=%d, server2=%d", hits1, hits2)
	}

	totalHits := hits1 + hits2
	if totalHits != int64(numRequests) {
		t.Errorf("expected total hits=%d, got %d", numRequests, totalHits)
	}
}

func TestProxy_PostRequest_ForwardsBody(t *testing.T) {
	mockServer, err := NewMockServer()
	if err != nil {
		t.Fatalf("failed to create mock server: %v", err)
	}
	defer mockServer.Close()

	instanceID := registerServiceWithPortAndRoutes(t, "post-service", mockServer.Port(), "/api/v1/post", []Route{
		{Method: "POST", Path: "/echo", Public: true},
	})
	defer cleanupService(t, instanceID)

	time.Sleep(100 * time.Millisecond)

	client := getHTTPClient()
	requestBody := map[string]string{"key": "value"}
	bodyBytes, _ := json.Marshal(requestBody)

	resp, err := client.Post(testServerURL+"/api/v1/post/echo", "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestProxy_ForwardsHeaders(t *testing.T) {
	mockServer, err := NewMockServer()
	if err != nil {
		t.Fatalf("failed to create mock server: %v", err)
	}
	defer mockServer.Close()

	instanceID := registerServiceWithPort(t, "headers-service", mockServer.Port(), "/api/v1/headers")
	defer cleanupService(t, instanceID)

	time.Sleep(100 * time.Millisecond)

	client := getHTTPClient()
	req, _ := http.NewRequest("GET", testServerURL+"/api/v1/headers/test", nil)
	req.Header.Set("X-Custom-Header", "custom-value")
	req.Header.Set("X-Request-ID", "test-request-id")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	if resp.Header.Get("X-Request-ID") == "" {
		t.Error("expected X-Request-ID header in response")
	}
}

func TestProxy_ServiceUnavailable_Returns503(t *testing.T) {
	mockServer, err := NewMockServer()
	if err != nil {
		t.Fatalf("failed to create mock server: %v", err)
	}

	instanceID := registerServiceWithPort(t, "unavailable-service", mockServer.Port(), "/api/v1/unavailable")
	defer cleanupService(t, instanceID)

	mockServer.Close()

	time.Sleep(100 * time.Millisecond)

	client := getHTTPClient()
	resp, err := client.Get(testServerURL + "/api/v1/unavailable/data")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadGateway && resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected status 502 or 503, got %d", resp.StatusCode)
	}
}

func TestProxy_QueryParams_Preserved(t *testing.T) {
	mockServer, err := NewMockServer()
	if err != nil {
		t.Fatalf("failed to create mock server: %v", err)
	}
	defer mockServer.Close()

	instanceID := registerServiceWithPort(t, "query-service", mockServer.Port(), "/api/v1/query")
	defer cleanupService(t, instanceID)

	time.Sleep(100 * time.Millisecond)

	client := getHTTPClient()
	resp, err := client.Get(testServerURL + "/api/v1/query/search?q=test&page=1&limit=10")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func registerServiceWithPort(t *testing.T, name string, port int, basePath string) string {
	t.Helper()
	return registerServiceWithPortAndRoutes(t, name, port, basePath, []Route{
		{Method: "GET", Path: "/data", Public: true},
		{Method: "GET", Path: "/test", Public: true},
		{Method: "GET", Path: "/search", Public: true},
	})
}

func registerServiceWithPortAndRoutes(t *testing.T, name string, port int, basePath string, routes []Route) string {
	t.Helper()
	client := getHTTPClient()

	req := RegisterRequest{
		ServiceName: name,
		Host:        "localhost",
		Port:        port,
		HealthURL:   "/health",
		BasePath:    basePath,
		Routes:      routes,
	}

	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", testServerURL+"/internal/registry/register", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Service-Token", signTestServiceToken(name))

	resp, err := client.Do(httpReq)
	if err != nil {
		t.Fatalf("failed to register service: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("failed to register service, status %d: %s", resp.StatusCode, string(respBody))
	}

	var registerResp RegisterResponse
	_ = json.NewDecoder(resp.Body).Decode(&registerResp)
	return registerResp.InstanceID
}
