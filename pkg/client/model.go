package client

import (
	"errors"
	"fmt"
)

var ErrInstanceNotFound = errors.New("instance not found")

type Route struct {
	Method    string   `json:"method"`
	Path      string   `json:"path"`
	Public    bool     `json:"public"`
	RateLimit int      `json:"rate_limit,omitempty"`
	Scopes    []string `json:"scopes,omitempty"`
}

type RegisterRequest struct {
	ServiceName string            `json:"service_name"`
	Host        string            `json:"host"`
	Port        int               `json:"port"`
	HealthURL   string            `json:"health_url,omitempty"`
	Version     string            `json:"version,omitempty"`
	BasePath    string            `json:"base_path"`
	Routes      []Route           `json:"routes"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type RegisterResponse struct {
	InstanceID        string   `json:"instance_id"`
	HeartbeatInterval int      `json:"heartbeat_interval"`
	HeartbeatURL      string   `json:"heartbeat_url"`
	RegisteredRoutes  []string `json:"registered_routes"`
}

type RouteCollision struct {
	Method        string `json:"method"`
	Path          string `json:"path"`
	CollisionType string `json:"collision_type"`
	RegisteredBy  string `json:"registered_by"`
}

type CollisionError struct {
	Collisions []RouteCollision
}

func (e *CollisionError) Error() string {
	return fmt.Sprintf("route collision: %d route(s) already registered", len(e.Collisions))
}
