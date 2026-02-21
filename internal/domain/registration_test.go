package domain

import "testing"

func TestRegisterRequest_Validate_Success(t *testing.T) {
	req := &RegisterRequest{
		ServiceName: "test-service",
		Host:        "localhost",
		Port:        8080,
		BasePath:    "/api/v1",
		Routes:      []Route{{Method: "GET", Path: "/users"}},
	}

	if err := req.Validate(); err != nil {
		t.Errorf("Validate() returned error for valid request: %v", err)
	}

	if req.HealthURL != "/health" {
		t.Errorf("Validate() should set default HealthURL, got %q", req.HealthURL)
	}
}

func TestRegisterRequest_Validate_MissingServiceName(t *testing.T) {
	req := &RegisterRequest{
		Host:     "localhost",
		Port:     8080,
		BasePath: "/api/v1",
		Routes:   []Route{{Method: "GET", Path: "/users"}},
	}

	if err := req.Validate(); err == nil {
		t.Error("Validate() should return error for missing service_name")
	}
}

func TestRegisterRequest_Validate_MissingHost(t *testing.T) {
	req := &RegisterRequest{
		ServiceName: "test-service",
		Port:        8080,
		BasePath:    "/api/v1",
		Routes:      []Route{{Method: "GET", Path: "/users"}},
	}

	if err := req.Validate(); err == nil {
		t.Error("Validate() should return error for missing host")
	}
}

func TestRegisterRequest_Validate_InvalidPort(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"zero", 0},
		{"negative", -1},
		{"too high", 65536},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &RegisterRequest{
				ServiceName: "test-service",
				Host:        "localhost",
				Port:        tt.port,
				BasePath:    "/api/v1",
				Routes:      []Route{{Method: "GET", Path: "/users"}},
			}

			if err := req.Validate(); err == nil {
				t.Errorf("Validate() should return error for port %d", tt.port)
			}
		})
	}
}

func TestRegisterRequest_Validate_MissingBasePath(t *testing.T) {
	req := &RegisterRequest{
		ServiceName: "test-service",
		Host:        "localhost",
		Port:        8080,
		Routes:      []Route{{Method: "GET", Path: "/users"}},
	}

	if err := req.Validate(); err == nil {
		t.Error("Validate() should return error for missing base_path")
	}
}

func TestRegisterRequest_Validate_EmptyRoutes(t *testing.T) {
	req := &RegisterRequest{
		ServiceName: "test-service",
		Host:        "localhost",
		Port:        8080,
		BasePath:    "/api/v1",
		Routes:      []Route{},
	}

	if err := req.Validate(); err == nil {
		t.Error("Validate() should return error for empty routes")
	}
}

func TestRegisterRequest_Validate_PreservesHealthURL(t *testing.T) {
	req := &RegisterRequest{
		ServiceName: "test-service",
		Host:        "localhost",
		Port:        8080,
		BasePath:    "/api/v1",
		HealthURL:   "/custom-health",
		Routes:      []Route{{Method: "GET", Path: "/users"}},
	}

	if err := req.Validate(); err != nil {
		t.Errorf("Validate() returned error: %v", err)
	}

	if req.HealthURL != "/custom-health" {
		t.Errorf("Validate() should preserve custom HealthURL, got %q", req.HealthURL)
	}
}
