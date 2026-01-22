package config

import (
	"os"
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.UDP.Port != 443 {
		t.Errorf("Expected default UDP port 443, got %d", cfg.UDP.Port)
	}

	if cfg.API.Port != 8080 {
		t.Errorf("Expected default API port 8080, got %d", cfg.API.Port)
	}
}

func TestLoadConfigFile(t *testing.T) {
	content := `
udp:
  port: 1234
api:
  port: 9090
redis:
  enabled: true
  address: "localhost:6379"
`
	err := os.WriteFile("config.yaml", []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}
	defer os.Remove("config.yaml")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config from file: %v", err)
	}

	if cfg.UDP.Port != 1234 {
		t.Errorf("Expected 1234, got %d", cfg.UDP.Port)
	}
	if cfg.API.Port != 9090 {
		t.Errorf("Expected 9090, got %d", cfg.API.Port)
	}
	if !cfg.Redis.Enabled {
		t.Error("Expected Redis enabled")
	}
}

func TestLoadConfigWithRoutes(t *testing.T) {
	content := `
routes:
  - fqdn: "test.example.com"
    type: "simple"
    target: "1.2.3.4:5678"
  - fqdn: "agones.example.com"
    type: "agones"
    target: "my-fleet"
`
	err := os.WriteFile("config.yaml", []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}
	defer os.Remove("config.yaml")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config from file: %v", err)
	}

	if len(cfg.Routes) != 2 {
		t.Fatalf("Expected 2 routes, got %d", len(cfg.Routes))
	}

	if cfg.Routes[0].FQDN != "test.example.com" || cfg.Routes[0].Target != "1.2.3.4:5678" || cfg.Routes[0].Type != "simple" {
		t.Errorf("Unexpected route 0: %+v", cfg.Routes[0])
	}
	if cfg.Routes[1].FQDN != "agones.example.com" || cfg.Routes[1].Target != "my-fleet" || cfg.Routes[1].Type != "agones" {
		t.Errorf("Unexpected route 1: %+v", cfg.Routes[1])
	}
}
