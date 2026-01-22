package api

import (
	"fmt"

	"github.com/ewancrowle/porter/internal/config"
	"github.com/ewancrowle/porter/internal/strategy"
	"github.com/ewancrowle/porter/internal/sync"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

type Server struct {
	app    *fiber.App
	cfg    *config.Config
	simple *strategy.SimpleStrategy
	agones *strategy.AgonesStrategy
	sync   *sync.RedisSync
}

func NewServer(cfg *config.Config, simple *strategy.SimpleStrategy, agones *strategy.AgonesStrategy, redisSync *sync.RedisSync) *Server {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	if cfg.API.LogRequests {
		app.Use(logger.New())
	}

	s := &Server{
		app:    app,
		cfg:    cfg,
		simple: simple,
		agones: agones,
		sync:   redisSync,
	}

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	s.app.Post("/routes", s.handleUpdateRoute)
	s.app.Post("/allocate", s.handleAgonesAllocation)
}

func (s *Server) Start() error {
	return s.app.Listen(fmt.Sprintf(":%d", s.cfg.API.Port))
}

func (s *Server) handleUpdateRoute(c *fiber.Ctx) error {
	var route strategy.Route
	if err := c.BodyParser(&route); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if route.Type == strategy.StrategySimple {
		s.simple.UpdateRoute(route.FQDN, route.Target)
	} else if route.Type == strategy.StrategyAgones {
		if !s.cfg.Agones.Enabled {
			return c.Status(400).JSON(fiber.Map{"error": "Agones is disabled"})
		}
		s.agones.UpdateRoute(route.FQDN, route.Target)
	} else {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid strategy type"})
	}

	// Publish to Redis for sync
	if s.sync != nil {
		if err := s.sync.PublishUpdate(c.Context(), route); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to sync route"})
		}
	}

	return c.JSON(fiber.Map{"status": "ok"})
}

func (s *Server) handleAgonesAllocation(c *fiber.Ctx) error {
	if !s.cfg.Agones.Enabled {
		return c.Status(400).JSON(fiber.Map{"error": "Agones is disabled"})
	}

	type allocationRequest struct {
		FQDN string `json:"fqdn"`
	}
	var req allocationRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	target, err := s.agones.Resolve(c.Context(), req.FQDN)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"fqdn":   req.FQDN,
		"target": target,
	})
}
