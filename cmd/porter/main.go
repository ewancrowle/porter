package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ewancrowle/porter/internal/api"
	"github.com/ewancrowle/porter/internal/config"
	"github.com/ewancrowle/porter/internal/relay"
	"github.com/ewancrowle/porter/internal/strategy"
	"github.com/ewancrowle/porter/internal/sync"
)

func main() {
	// 1. Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// 2. Initialize strategies
	manager := strategy.NewStrategyManager()

	simple := strategy.NewSimpleStrategy()
	manager.Register(strategy.StrategySimple, simple)

	agones := strategy.NewAgonesStrategy()
	if cfg.Agones.Enabled {
		if err := agones.Setup(cfg.Agones.Enabled, cfg.Agones.Namespace, cfg.Agones.AllocatorHost, cfg.Agones.AllocatorClientCert, cfg.Agones.AllocatorClientKey); err != nil {
			log.Fatalf("Failed to setup Agones strategy: %v", err)
		}
		manager.Register(strategy.StrategyAgones, agones)
	}

	// 3. Load initial routes from config
	for _, r := range cfg.Routes {
		switch strategy.StrategyType(r.Type) {
		case strategy.StrategySimple:
			simple.UpdateRoute(r.FQDN, r.Target)
			log.Printf("Loaded route from config: %s -> %s (simple)", r.FQDN, r.Target)
		case strategy.StrategyAgones:
			agones.UpdateRoute(r.FQDN, r.Target)
			log.Printf("Loaded route from config: %s -> %s (agones)", r.FQDN, r.Target)
		default:
			log.Printf("Warning: unknown strategy type %s for FQDN %s", r.Type, r.FQDN)
		}
	}

	// 4. Initialize Redis sync
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	redisSync := sync.NewRedisSync(cfg, simple, agones)
	if redisSync != nil {
		if err := redisSync.LoadInitialRoutes(ctx); err != nil {
			log.Printf("Warning: Failed to load initial routes from Redis: %v", err)
		}
		go redisSync.Subscribe(ctx)
	}

	// 4. Initialize and start UDP Relay
	engine, err := relay.NewRelay(cfg, manager)
	if err != nil {
		log.Fatalf("Failed to initialize UDP relay: %v", err)
	}

	go func() {
		if err := engine.Start(ctx); err != nil {
			log.Fatalf("UDP relay error: %v", err)
		}
	}()

	// 5. Initialize and start API Server
	server := api.NewServer(cfg, simple, agones, redisSync)
	go func() {
		log.Printf("API Server listening on :%d", cfg.API.Port)
		if err := server.Start(); err != nil {
			log.Fatalf("API server error: %v", err)
		}
	}()

	// Wait for interruption
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down Porter...")
	cancel()
}
