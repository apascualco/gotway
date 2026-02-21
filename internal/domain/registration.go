package domain

import "errors"

type RegisterRequest struct {
	ServiceName string            `json:"service_name" binding:"required"`
	Host        string            `json:"host" binding:"required"`
	Port        int               `json:"port" binding:"required"`
	HealthURL   string            `json:"health_url"`
	Version     string            `json:"version"`
	BasePath    string            `json:"base_path" binding:"required"`
	Routes      []Route           `json:"routes" binding:"required"`
	Metadata    map[string]string `json:"metadata"`
}

func (r *RegisterRequest) Validate() error {
	if r.ServiceName == "" {
		return errors.New("service_name is required")
	}
	if r.Host == "" {
		return errors.New("host is required")
	}
	if r.Port <= 0 || r.Port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	if r.BasePath == "" {
		return errors.New("base_path is required")
	}
	if len(r.Routes) == 0 {
		return errors.New("at least one route is required")
	}
	if r.HealthURL == "" {
		r.HealthURL = "/health"
	}
	return nil
}

type RegisterResponse struct {
	InstanceID        string   `json:"instance_id"`
	HeartbeatInterval int      `json:"heartbeat_interval"`
	HeartbeatURL      string   `json:"heartbeat_url"`
	RegisteredRoutes  []string `json:"registered_routes"`
}

type HeartbeatRequest struct {
	InstanceID string `json:"instance_id" binding:"required"`
}

type HeartbeatResponse struct {
	Status string `json:"status"`
}

type DeregisterRequest struct {
	InstanceID string `json:"instance_id" binding:"required"`
}
