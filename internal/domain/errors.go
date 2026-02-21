package domain

import (
	"fmt"
	"strings"
	"time"
)

type CollisionType string

const (
	ExactCollision   CollisionType = "exact"
	PatternCollision CollisionType = "pattern"
)

type RouteCollision struct {
	Method        string        `json:"method"`
	Path          string        `json:"path"`
	CollisionType CollisionType `json:"collision_type"`
	RegisteredBy  string        `json:"registered_by"`
	RegisteredAt  time.Time     `json:"registered_at"`
}

type CollisionError struct {
	Collisions []RouteCollision `json:"collisions"`
}

func (e *CollisionError) Error() string {
	var msgs []string
	for _, c := range e.Collisions {
		msgs = append(msgs, fmt.Sprintf("%s %s conflicts with %s (%s)",
			c.Method, c.Path, c.RegisteredBy, c.CollisionType))
	}
	return fmt.Sprintf("route collisions detected: %s", strings.Join(msgs, "; "))
}

var (
	ErrServiceNotFound  = fmt.Errorf("service not found")
	ErrInstanceNotFound = fmt.Errorf("instance not found")
	ErrRouteNotFound    = fmt.Errorf("route not found")
	ErrInvalidRequest   = fmt.Errorf("invalid request")
)
