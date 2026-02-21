package domain

import "testing"

func TestServiceInstance_Address(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     int
		expected string
	}{
		{"localhost", "localhost", 8080, "localhost:8080"},
		{"ip address", "192.168.1.1", 3000, "192.168.1.1:3000"},
		{"hostname", "auth-service", 8081, "auth-service:8081"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instance := &ServiceInstance{
				Host: tt.host,
				Port: tt.port,
			}
			if got := instance.Address(); got != tt.expected {
				t.Errorf("Address() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestServiceInstance_IsHealthy(t *testing.T) {
	tests := []struct {
		name     string
		status   ServiceStatus
		expected bool
	}{
		{"healthy", StatusHealthy, true},
		{"unhealthy", StatusUnhealthy, false},
		{"unknown", StatusUnknown, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instance := &ServiceInstance{Status: tt.status}
			if got := instance.IsHealthy(); got != tt.expected {
				t.Errorf("IsHealthy() = %v, want %v", got, tt.expected)
			}
		})
	}
}
