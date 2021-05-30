package test

import (
	"testing"

	"apascualco.com/gotway/internal/server"
)

func TestRolesConfiguration(t *testing.T) {
	middleware := server.Middleware{}
	path := "/v1/path"
	roles := []string{"admin", "op"}
	middleware.AddRole(path, roles)
	if middleware.Roles == nil {
		t.Errorf("expected: %v, got: %v", roles, nil)
	}
	if middleware.Roles[path] == nil {
		t.Errorf("the path: %v  doesn't exist into definitions", path)
	}
	if len(middleware.Roles[path]) != 2 {
		t.Errorf("expected: %v, got: %v", 2, len(middleware.Roles[path]))
	}
	if len(middleware.Roles[path]) != 2 {
		t.Errorf("expected: %v, got: %v", 2, len(middleware.Roles[path]))
	}
	if middleware.Roles[path][0] != roles[0] {
		t.Errorf("expected: %v, got: %v", roles[0], middleware.Roles[path][0])
	}
	if middleware.Roles[path][1] != roles[1] {
		t.Errorf("expected: %v, got: %v", roles[0], middleware.Roles[path][0])
	}
}
