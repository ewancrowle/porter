package sync

import (
	"context"
	"encoding/json"
	"log"

	"github.com/ewancrowle/porter/internal/config"
	"github.com/ewancrowle/porter/internal/strategy"
	"github.com/redis/go-redis/v9"
)

type RedisSync struct {
	client  *redis.Client
	channel string
	simple  *strategy.SimpleStrategy
	agones  *strategy.AgonesStrategy
}

func NewRedisSync(cfg *config.Config, simple *strategy.SimpleStrategy, agones *strategy.AgonesStrategy) *RedisSync {
	if !cfg.Redis.Enabled {
		return nil
	}

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Address,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	return &RedisSync{
		client:  client,
		channel: cfg.Redis.Channel,
		simple:  simple,
		agones:  agones,
	}
}

func (s *RedisSync) LoadInitialRoutes(ctx context.Context) error {
	if s == nil {
		return nil
	}

	// Load Simple routes from a Redis Hash "porter:routes:simple"
	simpleRoutes, err := s.client.HGetAll(ctx, "porter:routes:simple").Result()
	if err != nil {
		return err
	}
	for fqdn, target := range simpleRoutes {
		s.simple.UpdateRoute(fqdn, target)
		log.Printf("Loaded route from Redis: %s -> %s (simple)", fqdn, target)
	}

	// Load Agones routes from a Redis Hash "porter:routes:agones"
	agonesRoutes, err := s.client.HGetAll(ctx, "porter:routes:agones").Result()
	if err != nil {
		return err
	}
	for fqdn, fleet := range agonesRoutes {
		s.agones.UpdateRoute(fqdn, fleet)
		log.Printf("Loaded route from Redis: %s -> %s (agones)", fqdn, fleet)
	}

	return nil
}

func (s *RedisSync) PublishUpdate(ctx context.Context, route strategy.Route) error {
	if s == nil {
		return nil
	}

	data, err := json.Marshal(route)
	if err != nil {
		return err
	}

	// Persist in Hash
	key := "porter:routes:" + string(route.Type)
	if err := s.client.HSet(ctx, key, route.FQDN, route.Target).Err(); err != nil {
		return err
	}

	// Publish message
	return s.client.Publish(ctx, s.channel, data).Err()
}

func (s *RedisSync) Subscribe(ctx context.Context) {
	if s == nil {
		return
	}

	pubsub := s.client.Subscribe(ctx, s.channel)
	defer pubsub.Close()

	ch := pubsub.Channel()
	for msg := range ch {
		var route strategy.Route
		if err := json.Unmarshal([]byte(msg.Payload), &route); err != nil {
			log.Printf("Error unmarshaling sync message: %v", err)
			continue
		}

		log.Printf("Syncing route update from Redis: %s -> %s (%s)", route.FQDN, route.Target, route.Type)
		if route.Type == strategy.StrategySimple {
			s.simple.UpdateRoute(route.FQDN, route.Target)
		} else if route.Type == strategy.StrategyAgones {
			s.agones.UpdateRoute(route.FQDN, route.Target)
		}
	}
}
