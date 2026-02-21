package domain

import (
	"strings"
	"testing"
	"time"
)

func TestCollisionError_Error_SingleCollision(t *testing.T) {
	err := &CollisionError{
		Collisions: []RouteCollision{
			{
				Method:        "GET",
				Path:          "/api/v1/users",
				CollisionType: ExactCollision,
				RegisteredBy:  "user-service",
				RegisteredAt:  time.Now(),
			},
		},
	}

	msg := err.Error()
	if !strings.Contains(msg, "GET /api/v1/users") {
		t.Errorf("Error() should contain route info, got %q", msg)
	}
	if !strings.Contains(msg, "user-service") {
		t.Errorf("Error() should contain service name, got %q", msg)
	}
	if !strings.Contains(msg, "exact") {
		t.Errorf("Error() should contain collision type, got %q", msg)
	}
}

func TestCollisionError_Error_MultipleCollisions(t *testing.T) {
	err := &CollisionError{
		Collisions: []RouteCollision{
			{
				Method:        "GET",
				Path:          "/api/v1/users",
				CollisionType: ExactCollision,
				RegisteredBy:  "user-service",
			},
			{
				Method:        "POST",
				Path:          "/api/v1/items",
				CollisionType: PatternCollision,
				RegisteredBy:  "item-service",
			},
		},
	}

	msg := err.Error()
	if !strings.Contains(msg, "GET /api/v1/users") {
		t.Errorf("Error() should contain first route, got %q", msg)
	}
	if !strings.Contains(msg, "POST /api/v1/items") {
		t.Errorf("Error() should contain second route, got %q", msg)
	}
	if !strings.Contains(msg, "user-service") {
		t.Errorf("Error() should contain first service, got %q", msg)
	}
	if !strings.Contains(msg, "item-service") {
		t.Errorf("Error() should contain second service, got %q", msg)
	}
}

func TestCollisionError_ImplementsError(t *testing.T) {
	var err error = &CollisionError{
		Collisions: []RouteCollision{},
	}

	if err == nil {
		t.Error("CollisionError should implement error interface")
	}
}
