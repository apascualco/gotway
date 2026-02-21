package domain

import (
	"fmt"
	"time"
)

type Route struct {
	Method    string   `json:"method"`
	Path      string   `json:"path"`
	Public    bool     `json:"public"`
	RateLimit int      `json:"rate_limit"`
	Scopes    []string `json:"scopes"`
}

func (r *Route) FullPath(basePath string) string {
	if basePath == "" {
		return r.Path
	}
	if r.Path == "" {
		return basePath
	}
	return fmt.Sprintf("%s%s", basePath, r.Path)
}

func (r *Route) Key(basePath string) string {
	return fmt.Sprintf("%s:%s", r.Method, r.FullPath(basePath))
}

type RouteEntry struct {
	ServiceName  string    `json:"service_name"`
	BasePath     string    `json:"base_path"`
	Route        Route     `json:"route"`
	RegisteredAt time.Time `json:"registered_at"`
}
