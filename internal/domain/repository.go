package domain

import "context"

// Repository defines the interface for service registry persistence (output port)
type Repository interface {
	// Instance operations
	SaveInstance(ctx context.Context, instance *ServiceInstance) error
	GetInstance(ctx context.Context, instanceID string) (*ServiceInstance, error)
	GetInstancesByService(ctx context.Context, serviceName string) ([]*ServiceInstance, error)
	DeleteInstance(ctx context.Context, instanceID string) error
	UpdateInstanceStatus(ctx context.Context, instanceID string, status ServiceStatus) error
	UpdateHeartbeat(ctx context.Context, instanceID string) error

	// Route operations
	SaveRoutes(ctx context.Context, serviceName string, basePath string, routes []Route) error
	GetRoute(ctx context.Context, method, path string) (*RouteEntry, error)
	GetAllRoutes(ctx context.Context) ([]RouteEntry, error)
	DeleteRoutesByService(ctx context.Context, serviceName string) error
	CheckRouteCollisions(ctx context.Context, basePath string, routes []Route) ([]RouteCollision, error)

	// Service operations
	GetAllServices(ctx context.Context) ([]string, error)
	GetHealthyInstances(ctx context.Context, serviceName string) ([]*ServiceInstance, error)
}
