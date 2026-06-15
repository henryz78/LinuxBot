package session

import (
	"context"
	"testing"

	"linuxbot/internal/storage"
)

func TestServiceSwitchesDefaultSession(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(t.TempDir() + "/linuxbot.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	service := NewService(store)
	created, err := service.Create(ctx, "prod", "/opt/app")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Name != "prod" {
		t.Fatalf("name = %q", created.Name)
	}
	if err := service.Switch(ctx, "prod"); err != nil {
		t.Fatalf("Switch: %v", err)
	}
	current, err := service.Current(ctx, "/tmp")
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if current.Name != "prod" {
		t.Fatalf("current = %q", current.Name)
	}
}
