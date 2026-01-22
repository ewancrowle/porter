package strategy

import (
	"context"
	"testing"
)

func TestSimpleStrategy(t *testing.T) {
	s := NewSimpleStrategy()
	s.UpdateRoute("test.com", "1.2.3.4:5000")

	target, err := s.Resolve(context.Background(), "test.com")
	if err != nil {
		t.Fatalf("Failed to resolve: %v", err)
	}

	if target != "1.2.3.4:5000" {
		t.Errorf("Expected 1.2.3.4:5000, got %s", target)
	}

	_, err = s.Resolve(context.Background(), "unknown.com")
	if err == nil {
		t.Error("Expected error for unknown FQDN")
	}
}
