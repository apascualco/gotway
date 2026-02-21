package proxy

import (
	"strings"

	"github.com/apascualco/gotway/internal/application"
	"github.com/apascualco/gotway/internal/domain"
)

type MatchResult struct {
	Entry  *domain.RouteEntry
	Params map[string]string
}

func MatchRoute(registry *application.Registry, method, path string) *MatchResult {
	routes := registry.GetAllRoutes()

	exactKey := method + ":" + path
	if entry, exists := routes[exactKey]; exists {
		return &MatchResult{
			Entry:  entry,
			Params: nil,
		}
	}

	for key, entry := range routes {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 {
			continue
		}
		routeMethod := parts[0]
		routePath := parts[1]

		if routeMethod != method {
			continue
		}

		if params, ok := matchPathWithParams(routePath, path); ok {
			return &MatchResult{
				Entry:  entry,
				Params: params,
			}
		}
	}

	return nil
}

func matchPathWithParams(pattern, path string) (map[string]string, bool) {
	patternSegments := strings.Split(strings.Trim(pattern, "/"), "/")
	pathSegments := strings.Split(strings.Trim(path, "/"), "/")

	if len(patternSegments) != len(pathSegments) {
		if len(patternSegments) > 0 && patternSegments[len(patternSegments)-1] == "*" {
			if len(pathSegments) < len(patternSegments)-1 {
				return nil, false
			}
		} else {
			return nil, false
		}
	}

	params := make(map[string]string)

	for i, patternSeg := range patternSegments {
		if patternSeg == "*" {
			remaining := strings.Join(pathSegments[i:], "/")
			params["*"] = remaining
			return params, true
		}

		if i >= len(pathSegments) {
			return nil, false
		}

		pathSeg := pathSegments[i]

		if strings.HasPrefix(patternSeg, ":") {
			paramName := patternSeg[1:]
			params[paramName] = pathSeg
			continue
		}

		if patternSeg != pathSeg {
			return nil, false
		}
	}

	return params, true
}
