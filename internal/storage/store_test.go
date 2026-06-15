package storage

import (
	"context"
	"sync"
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

func TestEnsureDefaultSessionPreservesOriginalWorkingDirectory(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	first, err := store.EnsureDefaultSession(ctx, "/workspace/first")
	if err != nil {
		t.Fatalf("first EnsureDefaultSession: %v", err)
	}
	second, err := store.EnsureDefaultSession(ctx, "/workspace/second")
	if err != nil {
		t.Fatalf("second EnsureDefaultSession: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("session IDs differ: first=%d second=%d", first.ID, second.ID)
	}
	if second.WorkingDirectory != first.WorkingDirectory {
		t.Fatalf("working directory changed from %q to %q", first.WorkingDirectory, second.WorkingDirectory)
	}
}

func TestEnsureDefaultSessionConcurrentCallersReturnSameSession(t *testing.T) {
	ctx := context.Background()
	dbPath := t.TempDir() + "/linuxbot.db"

	const callers = 2
	stores := make([]*Store, 0, callers)
	for range callers {
		store, err := Open(dbPath)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		stores = append(stores, store)
		t.Cleanup(func() {
			if err := store.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}
		})
	}

	type result struct {
		session Session
		err     error
	}
	start := make(chan struct{})
	results := make(chan result, callers)
	var wg sync.WaitGroup
	for _, store := range stores {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			session, err := store.EnsureDefaultSession(ctx, "/workspace/default")
			results <- result{session: session, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	var sessionID int64
	for result := range results {
		if result.err != nil {
			t.Fatalf("EnsureDefaultSession: %v", result.err)
		}
		if sessionID == 0 {
			sessionID = result.session.ID
		}
		if result.session.ID != sessionID {
			t.Fatalf("session ID = %d, want %d", result.session.ID, sessionID)
		}
		if result.session.WorkingDirectory != "/workspace/default" {
			t.Fatalf("working directory = %q", result.session.WorkingDirectory)
		}
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

func TestCreateRunReturnsPersistedTimestamps(t *testing.T) {
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
	if run.CreatedAt.IsZero() {
		t.Fatal("CreateRun returned zero CreatedAt")
	}
	if run.UpdatedAt.IsZero() {
		t.Fatal("CreateRun returned zero UpdatedAt")
	}

	stored, err := store.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if stored.CreatedAt.IsZero() {
		t.Fatal("GetRun returned zero CreatedAt")
	}
	if stored.UpdatedAt.IsZero() {
		t.Fatal("GetRun returned zero UpdatedAt")
	}
}

func TestStoreEnforcesStepForeignKeys(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	err := store.AddStep(ctx, Step{
		RunID:  999,
		Kind:   "answer",
		Status: "done",
	})
	if err == nil {
		t.Fatal("AddStep with missing run succeeded")
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
