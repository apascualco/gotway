package application

import (
	"time"

	"github.com/apascualco/gotway/internal/domain"
)

func (r *Registry) Heartbeat(instanceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	instance, exists := r.instances[instanceID]
	if !exists {
		return domain.ErrInstanceNotFound
	}

	instance.LastHeartbeat = time.Now()
	return nil
}
