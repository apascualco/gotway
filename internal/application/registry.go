package application

import (
	"sync"
	"time"

	"github.com/apascualco/gotway/internal/domain"
)

type RegistryConfig struct {
	ServiceToken              string
	HeartbeatTTL              time.Duration
	HealthCheckInterval       time.Duration
	AllowSameServiceOverwrite bool
	StrictPatternMatching     bool
}

type Registry struct {
	config    RegistryConfig
	mu        sync.RWMutex
	instances map[string]*domain.ServiceInstance
	services  map[string][]string
	routes    map[string]*domain.RouteEntry
	stopCh    chan struct{}
}

func NewRegistry(cfg RegistryConfig) *Registry {
	return &Registry{
		config:    cfg,
		instances: make(map[string]*domain.ServiceInstance),
		services:  make(map[string][]string),
		routes:    make(map[string]*domain.RouteEntry),
		stopCh:    make(chan struct{}),
	}
}
