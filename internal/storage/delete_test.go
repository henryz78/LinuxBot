package storage

import (
	"context"
	"testing"
)

func TestDeleteRunRemovesStepsAndInvalidatesSummary(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	session, err := store.EnsureDefaultSession(ctx, "/tmp")
	if err != nil {
		t.Fatalf("EnsureDefaultSession: %v", err)
	}
	run, err := store.CreateRun(ctx, session.ID, "secret command")
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := store.AddStep(ctx, Step{RunID: run.ID, Kind: "command", Status: "done", Input: "cat .env", Output: "TOKEN=secret"}); err != nil {
		t.Fatalf("AddStep: %v", err)
	}
	if err := store.AddMessage(ctx, session.ID, run.ID, "user", "cat .env"); err != nil {
		t.Fatalf("AddMessage user: %v", err)
	}
	if err := store.AddMessage(ctx, session.ID, run.ID, "assistant", "TOKEN=secret"); err != nil {
		t.Fatalf("AddMessage assistant: %v", err)
	}
	if err := store.SetSessionSummary(ctx, session.ID, "TOKEN=secret"); err != nil {
		t.Fatalf("SetSessionSummary: %v", err)
	}
	if err := store.DeleteRun(ctx, session.ID, run.ID, "cli"); err != nil {
		t.Fatalf("DeleteRun: %v", err)
	}
	steps, err := store.ListRunSteps(ctx, run.ID)
	if err != nil {
		t.Fatalf("ListRunSteps: %v", err)
	}
	if len(steps) != 0 {
		t.Fatalf("steps remain = %#v", steps)
	}
	summary, err := store.SessionSummary(ctx, session.ID)
	if err != nil {
		t.Fatalf("SessionSummary: %v", err)
	}
	if summary != "" {
		t.Fatalf("summary = %q", summary)
	}
	messages, err := store.ListRecentMessages(ctx, session.ID, 10)
	if err != nil {
		t.Fatalf("ListRecentMessages: %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("messages remain = %#v", messages)
	}
}

func TestDeleteRunDoesNotDeleteOtherSessionRun(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	first, err := store.CreateSession(ctx, "first", "/tmp/first")
	if err != nil {
		t.Fatalf("CreateSession first: %v", err)
	}
	second, err := store.CreateSession(ctx, "second", "/tmp/second")
	if err != nil {
		t.Fatalf("CreateSession second: %v", err)
	}
	run, err := store.CreateRun(ctx, second.ID, "belongs to second")
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := store.AddStep(ctx, Step{RunID: run.ID, Kind: "command", Status: "done", Input: "whoami"}); err != nil {
		t.Fatalf("AddStep: %v", err)
	}

	err = store.DeleteRun(ctx, first.ID, run.ID, "cli")

	if err == nil {
		t.Fatalf("expected cross-session delete to fail")
	}
	steps, err := store.ListRunSteps(ctx, run.ID)
	if err != nil {
		t.Fatalf("ListRunSteps: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("steps after failed delete = %#v", steps)
	}
}
