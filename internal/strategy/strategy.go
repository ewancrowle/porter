package strategy

import (
	"context"
)

type StrategyType string

const (
	StrategySimple StrategyType = "simple"
	StrategyAgones StrategyType = "agones"
)

type Route struct {
	FQDN   string       `json:"fqdn"`
	Type   StrategyType `json:"type"`
	Target string       `json:"target"` // For simple: ip:port. For agones: fleet name.
}

type RoutingStrategy interface {
	Resolve(ctx context.Context, fqdn string) (string, error)
}

type StrategyManager struct {
	strategies map[StrategyType]RoutingStrategy
}

func NewStrategyManager() *StrategyManager {
	return &StrategyManager{
		strategies: make(map[StrategyType]RoutingStrategy),
	}
}

func (m *StrategyManager) Register(t StrategyType, s RoutingStrategy) {
	m.strategies[t] = s
}

func (m *StrategyManager) Get(t StrategyType) RoutingStrategy {
	return m.strategies[t]
}
