package test

import (
	"testing"

	server "apascualco.com/gotway/internal/server"
)

type MiddlewareTestType struct {
	Middleware server.Middleware
	Path string
	Roles []string
}

func setup() MiddlewareTestType {
	return MiddlewareTestType {
		Middleware: server.Middleware{},
		Path: "/v1/path",
		Roles: []string{"admin", "op"},
	}
}

func TestAfterAddRolesTheRolesShouldNotBeNull(t *testing.T) {
	middlewareTestType := setup()
	middleware := middlewareTestType.Middleware
	path := middlewareTestType.Path
	roles := middlewareTestType.Roles

	middleware.AddRole(path, roles)
	if middleware.Roles == nil {
		t.Errorf("expected: %v, got: %v", roles, nil)
	}
}

func TestAfterAddRolesThePathShouldBeExist(t *testing.T) {
	middlewareTestType := setup()
	middleware := middlewareTestType.Middleware
	path := middlewareTestType.Path
	roles := middlewareTestType.Roles

	middleware.AddRole(path, roles)

	if middleware.Roles[path] == nil {
		t.Errorf("the path: %v  doesn't exist into definitions", path)
	}
}

func TestTheRolesSizeShouldBeTwo(t *testing.T) {
	middlewareTestType := setup()
	middleware := middlewareTestType.Middleware
	path := middlewareTestType.Path
	roles := middlewareTestType.Roles

	middleware.AddRole(path, roles)

	if len(middleware.Roles[path]) != 2 {
		t.Errorf("expected: %v, got: %v", 2, len(middleware.Roles[path]))
	}
}

func TestTheRolesShouldBeInTheSamePosition(t *testing.T) {
	middlewareTestType := setup()
	middleware := middlewareTestType.Middleware
	path := middlewareTestType.Path
	roles := middlewareTestType.Roles

	middleware.AddRole(path, roles)

	if middleware.Roles[path][0] != roles[0] {
		t.Errorf("expected: %v, got: %v", roles[0], middleware.Roles[path][0])
	}
	if middleware.Roles[path][1] != roles[1] {
		t.Errorf("expected: %v, got: %v", roles[0], middleware.Roles[path][0])
	}
}
