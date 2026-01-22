package strategy

import (
	"context"
	"errors"
	"sync"
)

type SimpleStrategy struct {
	mu     sync.RWMutex
	routes map[string]string // FQDN -> target
}

func NewSimpleStrategy() *SimpleStrategy {
	return &SimpleStrategy{
		routes: make(map[string]string),
	}
}

func (s *SimpleStrategy) Resolve(ctx context.Context, fqdn string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	target, ok := s.routes[fqdn]
	if !ok {
		return "", errors.New("route not found")
	}
	return target, nil
}

func (s *SimpleStrategy) UpdateRoute(fqdn, target string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.routes[fqdn] = target
}
