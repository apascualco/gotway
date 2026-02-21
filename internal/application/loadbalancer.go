package application

import (
	"sync/atomic"

	"github.com/apascualco/gotway/internal/domain"
)

type LoadBalancer interface {
	Select(instances []*domain.ServiceInstance) *domain.ServiceInstance
}

type RoundRobinBalancer struct {
	counter uint64
}

func NewRoundRobinBalancer() *RoundRobinBalancer {
	return &RoundRobinBalancer{}
}

func (r *RoundRobinBalancer) Select(instances []*domain.ServiceInstance) *domain.ServiceInstance {
	if len(instances) == 0 {
		return nil
	}

	n := atomic.AddUint64(&r.counter, 1)
	idx := (n - 1) % uint64(len(instances))
	return instances[idx]
}
