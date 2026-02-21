package domain

import (
	"fmt"
	"time"
)

type ServiceStatus string

const (
	StatusHealthy   ServiceStatus = "healthy"
	StatusUnhealthy ServiceStatus = "unhealthy"
	StatusUnknown   ServiceStatus = "unknown"
)

type ServiceInstance struct {
	ID            string            `json:"id"`
	ServiceName   string            `json:"service_name"`
	Host          string            `json:"host"`
	Port          int               `json:"port"`
	HealthURL     string            `json:"health_url"`
	Version       string            `json:"version"`
	Status        ServiceStatus     `json:"status"`
	Weight        int               `json:"weight"`
	Metadata      map[string]string `json:"metadata"`
	RegisteredAt  time.Time         `json:"registered_at"`
	LastHeartbeat time.Time         `json:"last_heartbeat"`
}

func (i *ServiceInstance) Address() string {
	return fmt.Sprintf("%s:%d", i.Host, i.Port)
}

func (i *ServiceInstance) IsHealthy() bool {
	return i.Status == StatusHealthy
}
