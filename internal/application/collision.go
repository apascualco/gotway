package application

import (
	"regexp"
	"strings"

	"github.com/apascualco/gotway/internal/domain"
)

var paramPattern = regexp.MustCompile(`:[^/]+|\{[^}]+\}`)

func normalizePath(path string) string {
	return paramPattern.ReplaceAllString(path, "*")
}

func (r *Registry) checkExactCollision(serviceName, method, path string) *domain.RouteCollision {
	key := method + ":" + path
	if entry, exists := r.routes[key]; exists {
		if entry.ServiceName == serviceName {
			return nil
		}
		return &domain.RouteCollision{
			Method:        method,
			Path:          path,
			CollisionType: domain.ExactCollision,
			RegisteredBy:  entry.ServiceName,
			RegisteredAt:  entry.RegisteredAt,
		}
	}
	return nil
}

func (r *Registry) checkPatternCollision(serviceName, method, path string) []domain.RouteCollision {
	var collisions []domain.RouteCollision
	normalizedNew := normalizePath(path)

	for key, entry := range r.routes {
		if entry.ServiceName == serviceName {
			continue
		}

		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 {
			continue
		}
		existingMethod, existingPath := parts[0], parts[1]

		if existingMethod != method {
			continue
		}

		normalizedExisting := normalizePath(existingPath)
		if pathsOverlap(normalizedNew, normalizedExisting) {
			collisions = append(collisions, domain.RouteCollision{
				Method:        method,
				Path:          path,
				CollisionType: domain.PatternCollision,
				RegisteredBy:  entry.ServiceName,
				RegisteredAt:  entry.RegisteredAt,
			})
		}
	}

	return collisions
}

func pathsOverlap(path1, path2 string) bool {
	if path1 == path2 {
		return true
	}

	segments1 := strings.Split(strings.Trim(path1, "/"), "/")
	segments2 := strings.Split(strings.Trim(path2, "/"), "/")

	if len(segments1) != len(segments2) {
		return false
	}

	for i := range segments1 {
		s1, s2 := segments1[i], segments2[i]
		if s1 == "*" || s2 == "*" {
			continue
		}
		if s1 != s2 {
			return false
		}
	}

	return true
}

func (r *Registry) ValidateRoutes(serviceName, basePath string, routes []domain.Route) ([]domain.RouteCollision, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var collisions []domain.RouteCollision

	for _, route := range routes {
		fullPath := route.FullPath(basePath)

		if collision := r.checkExactCollision(serviceName, route.Method, fullPath); collision != nil {
			collisions = append(collisions, *collision)
			continue
		}

		if r.config.StrictPatternMatching {
			patternCollisions := r.checkPatternCollision(serviceName, route.Method, fullPath)
			collisions = append(collisions, patternCollisions...)
		}
	}

	return collisions, nil
}
