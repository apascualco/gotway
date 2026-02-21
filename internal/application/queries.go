package application

import "github.com/apascualco/gotway/internal/domain"

func (r *Registry) GetInstance(instanceID string) *domain.ServiceInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.instances[instanceID]
}

func (r *Registry) GetInstances(serviceName string) []*domain.ServiceInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()

	instanceIDs := r.services[serviceName]
	if len(instanceIDs) == 0 {
		return nil
	}

	instances := make([]*domain.ServiceInstance, 0, len(instanceIDs))
	for _, id := range instanceIDs {
		if instance := r.instances[id]; instance != nil {
			instances = append(instances, instance)
		}
	}
	return instances
}

func (r *Registry) GetHealthyInstances(serviceName string) []*domain.ServiceInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()

	instanceIDs := r.services[serviceName]
	if len(instanceIDs) == 0 {
		return nil
	}

	var healthy []*domain.ServiceInstance
	for _, id := range instanceIDs {
		if instance := r.instances[id]; instance != nil && instance.Status == domain.StatusHealthy {
			healthy = append(healthy, instance)
		}
	}
	return healthy
}

func (r *Registry) GetRoute(method, path string) *domain.RouteEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := method + ":" + path
	return r.routes[key]
}

func (r *Registry) GetAllServices() map[string][]*domain.ServiceInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string][]*domain.ServiceInstance)
	for serviceName, instanceIDs := range r.services {
		instances := make([]*domain.ServiceInstance, 0, len(instanceIDs))
		for _, id := range instanceIDs {
			if instance := r.instances[id]; instance != nil {
				instances = append(instances, instance)
			}
		}
		if len(instances) > 0 {
			result[serviceName] = instances
		}
	}
	return result
}

func (r *Registry) GetAllRoutes() map[string]*domain.RouteEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*domain.RouteEntry, len(r.routes))
	for k, v := range r.routes {
		result[k] = v
	}
	return result
}
