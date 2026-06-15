package storage

import (
	"context"
	"testing"
)

func TestStoreCreatesDefaultSession(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	session, err := store.EnsureDefaultSession(ctx, "/tmp")
	if err != nil {
		t.Fatalf("EnsureDefaultSession: %v", err)
	}
	if session.Name != "default" {
		t.Fatalf("session name = %q", session.Name)
	}
	if session.Mode != "safe" {
		t.Fatalf("mode = %q", session.Mode)
	}
	if session.WorkingDirectory != "/tmp" {
		t.Fatalf("working dir = %q", session.WorkingDirectory)
	}
}

func TestRunStepRoundTrip(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	session, err := store.EnsureDefaultSession(ctx, "/srv/app")
	if err != nil {
		t.Fatalf("EnsureDefaultSession: %v", err)
	}
	run, err := store.CreateRun(ctx, session.ID, "hello")
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := store.AddStep(ctx, Step{
		RunID:  run.ID,
		Kind:   "answer",
		Status: "done",
		Input:  "hello",
		Output: "world",
	}); err != nil {
		t.Fatalf("AddStep: %v", err)
	}
	steps, err := store.ListRunSteps(ctx, run.ID)
	if err != nil {
		t.Fatalf("ListRunSteps: %v", err)
	}
	if len(steps) != 1 || steps[0].Output != "world" {
		t.Fatalf("steps = %#v", steps)
	}
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(t.TempDir() + "/linuxbot.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})
	return store
}
