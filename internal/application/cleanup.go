package application

import (
	"log/slog"
	"time"

	"github.com/apascualco/gotway/internal/domain"
)

func (r *Registry) Start() {
	go r.cleanupLoop()
}

func (r *Registry) Stop() {
	close(r.stopCh)
}

func (r *Registry) cleanupLoop() {
	interval := r.config.HeartbeatTTL / 2
	if interval < time.Second {
		interval = time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.cleanup()
		case <-r.stopCh:
			return
		}
	}
}

func (r *Registry) cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	var toRemove []string

	for id, instance := range r.instances {
		elapsed := now.Sub(instance.LastHeartbeat)

		if elapsed > r.config.HeartbeatTTL*2 {
			toRemove = append(toRemove, id)
			slog.Info("removing expired instance",
				"instance_id", id,
				"service", instance.ServiceName,
				"last_heartbeat", instance.LastHeartbeat,
			)
		} else if elapsed > r.config.HeartbeatTTL && instance.Status == domain.StatusHealthy {
			instance.Status = domain.StatusUnhealthy
			slog.Warn("marking instance unhealthy",
				"instance_id", id,
				"service", instance.ServiceName,
				"elapsed", elapsed,
			)
		}
	}

	for _, id := range toRemove {
		r.removeInstanceLocked(id)
	}
}

func (r *Registry) removeInstanceLocked(instanceID string) {
	instance, exists := r.instances[instanceID]
	if !exists {
		return
	}

	s := instance.ServiceName

	instanceIDs := r.services[s]
	for i, id := range instanceIDs {
		if id == instanceID {
			r.services[s] = append(instanceIDs[:i], instanceIDs[i+1:]...)
			break
		}
	}

	if len(r.services[s]) == 0 {
		for key, entry := range r.routes {
			if entry.ServiceName == s {
				delete(r.routes, key)
			}
		}
		delete(r.services, s)
	}

	delete(r.instances, instanceID)
}
