package application

import (
	"sync"
	"testing"

	"github.com/apascualco/gotway/internal/domain"
)

func TestRoundRobin_Distributes(t *testing.T) {
	lb := NewRoundRobinBalancer()

	instances := []*domain.ServiceInstance{
		{ID: "instance-1", Host: "host1", Port: 8081},
		{ID: "instance-2", Host: "host2", Port: 8082},
		{ID: "instance-3", Host: "host3", Port: 8083},
	}

	selected1 := lb.Select(instances)
	if selected1.ID != "instance-1" {
		t.Errorf("first select: expected instance-1, got %s", selected1.ID)
	}

	selected2 := lb.Select(instances)
	if selected2.ID != "instance-2" {
		t.Errorf("second select: expected instance-2, got %s", selected2.ID)
	}

	selected3 := lb.Select(instances)
	if selected3.ID != "instance-3" {
		t.Errorf("third select: expected instance-3, got %s", selected3.ID)
	}

	selected4 := lb.Select(instances)
	if selected4.ID != "instance-1" {
		t.Errorf("fourth select: expected instance-1 (wrap around), got %s", selected4.ID)
	}
}

func TestRoundRobin_EmptyList(t *testing.T) {
	lb := NewRoundRobinBalancer()

	selected := lb.Select([]*domain.ServiceInstance{})
	if selected != nil {
		t.Errorf("expected nil for empty list, got %v", selected)
	}

	selected = lb.Select(nil)
	if selected != nil {
		t.Errorf("expected nil for nil list, got %v", selected)
	}
}

func TestRoundRobin_SingleInstance(t *testing.T) {
	lb := NewRoundRobinBalancer()

	instances := []*domain.ServiceInstance{
		{ID: "only-instance", Host: "host1", Port: 8081},
	}

	for i := 0; i < 5; i++ {
		selected := lb.Select(instances)
		if selected.ID != "only-instance" {
			t.Errorf("iteration %d: expected only-instance, got %s", i, selected.ID)
		}
	}
}

func TestRoundRobin_Concurrent(t *testing.T) {
	lb := NewRoundRobinBalancer()

	instances := []*domain.ServiceInstance{
		{ID: "instance-1", Host: "host1", Port: 8081},
		{ID: "instance-2", Host: "host2", Port: 8082},
		{ID: "instance-3", Host: "host3", Port: 8083},
	}

	counts := make(map[string]int)
	var mu sync.Mutex
	var wg sync.WaitGroup

	numGoroutines := 100
	selectionsPerGoroutine := 30

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < selectionsPerGoroutine; j++ {
				selected := lb.Select(instances)
				mu.Lock()
				counts[selected.ID]++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	totalSelections := numGoroutines * selectionsPerGoroutine
	expectedPerInstance := totalSelections / len(instances)

	for id, count := range counts {
		if count != expectedPerInstance {
			t.Errorf("instance %s: expected %d selections, got %d", id, expectedPerInstance, count)
		}
	}
}
