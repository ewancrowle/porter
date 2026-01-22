package strategy

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type AgonesStrategy struct {
	mu     sync.RWMutex
	fleets map[string]string // FQDN -> Fleet Name
	// agonesClient *agones.Client // To be implemented/integrated
}

func NewAgonesStrategy() *AgonesStrategy {
	return &AgonesStrategy{
		fleets: make(map[string]string),
	}
}

func (s *AgonesStrategy) Resolve(ctx context.Context, fqdn string) (string, error) {
	s.mu.RLock()
	fleetName, ok := s.fleets[fqdn]
	s.mu.RUnlock()

	if !ok {
		return "", errors.New("agones fleet not mapped for FQDN")
	}

	// This is where Porter calls Agones Allocator Service
	// For now, returning a placeholder or error if not fully implemented
	return s.allocate(ctx, fleetName)
}

func (s *AgonesStrategy) UpdateRoute(fqdn, fleetName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fleets[fqdn] = fleetName
}

func (s *AgonesStrategy) allocate(ctx context.Context, fleetName string) (string, error) {
	// TODO: Implement actual Agones Allocation call
	return "", fmt.Errorf("agones allocation for fleet %s not yet implemented", fleetName)
}
