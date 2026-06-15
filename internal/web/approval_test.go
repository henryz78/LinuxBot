package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"linuxbot/internal/storage"
)

func TestApproveEndpointExecutesPendingCommand(t *testing.T) {
	store, err := storage.Open(t.TempDir() + "/linuxbot.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	session, err := store.EnsureDefaultSession(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("EnsureDefaultSession: %v", err)
	}
	run, err := store.CreateRun(context.Background(), session.ID, "run command")
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := store.AddStep(context.Background(), storage.Step{RunID: run.ID, Kind: "command", Status: "approval_required", Input: "echo ok"}); err != nil {
		t.Fatalf("AddStep: %v", err)
	}
	server := NewServer(Options{Store: store, ApproveCommand: func(ctx context.Context, session storage.Session, step storage.Step) error {
		return store.AddStep(ctx, storage.Step{RunID: step.RunID, Kind: "command", Status: "done", Input: step.Input, Output: "ok\n"})
	}})
	body := strings.NewReader(`{"session_id":` + strconv.FormatInt(session.ID, 10) + `,"step_id":1}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/approve", body)
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	step, err := store.GetStep(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetStep: %v", err)
	}
	if step.Status != "approved" {
		t.Fatalf("approved step status = %s", step.Status)
	}
	storedRun, err := store.GetRun(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if storedRun.Status != "done" {
		t.Fatalf("run status = %s", storedRun.Status)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/approve", strings.NewReader(`{"session_id":`+strconv.FormatInt(session.ID, 10)+`,"step_id":1}`))
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("repeat approve status = %d", recorder.Code)
	}
}
