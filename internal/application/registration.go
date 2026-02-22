package application

import (
	"fmt"
	"time"

	"github.com/apascualco/gotway/internal/domain"
	"github.com/google/uuid"
)

func (r *Registry) Register(req *domain.RegisterRequest) (*domain.RegisterResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	collisions, err := r.ValidateRoutes(req.ServiceName, req.BasePath, req.Routes)
	if err != nil {
		return nil, err
	}
	if len(collisions) > 0 {
		return nil, &domain.CollisionError{Collisions: collisions}
	}

	instanceID := uuid.New().String()
	now := time.Now()

	instance := &domain.ServiceInstance{
		ID:            instanceID,
		ServiceName:   req.ServiceName,
		Host:          req.Host,
		Port:          req.Port,
		HealthURL:     req.HealthURL,
		Version:       req.Version,
		Status:        domain.StatusHealthy,
		Weight:        1,
		Metadata:      req.Metadata,
		RegisteredAt:  now,
		LastHeartbeat: now,
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.instances[instanceID] = instance
	r.services[req.ServiceName] = append(r.services[req.ServiceName], instanceID)

	var registeredRoutes []string
	for _, route := range req.Routes {
		fullPath := route.FullPath(req.BasePath)
		key := route.Method + ":" + fullPath

		r.routes[key] = &domain.RouteEntry{
			ServiceName:  req.ServiceName,
			BasePath:     req.BasePath,
			Route:        route,
			RegisteredAt: now,
		}
		registeredRoutes = append(registeredRoutes, key)
	}

	return &domain.RegisterResponse{
		InstanceID:        instanceID,
		HeartbeatInterval: int(r.config.HeartbeatTTL.Seconds()),
		HeartbeatURL:      fmt.Sprintf("/internal/registry/heartbeat"),
		RegisteredRoutes:  registeredRoutes,
	}, nil
}

func (r *Registry) Deregister(instanceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	instance, exists := r.instances[instanceID]
	if !exists {
		return domain.ErrInstanceNotFound
	}

	serviceName := instance.ServiceName

	instanceIDs := r.services[serviceName]
	for i, id := range instanceIDs {
		if id == instanceID {
			r.services[serviceName] = append(instanceIDs[:i], instanceIDs[i+1:]...)
			break
		}
	}

	if len(r.services[serviceName]) == 0 {
		for key, entry := range r.routes {
			if entry.ServiceName == serviceName {
				delete(r.routes, key)
			}
		}
		delete(r.services, serviceName)
	}

	delete(r.instances, instanceID)

	return nil
}
