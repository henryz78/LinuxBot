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
	if err := store.AddStep(context.Background(), storage.Step{RunID: run.ID, Kind: "answer", Status: "waiting_approval", Output: "waiting"}); err != nil {
		t.Fatalf("AddStep waiting answer: %v", err)
	}
	server := NewServer(Options{Store: store, ApproveCommand: func(ctx context.Context, session storage.Session, step storage.Step) error {
		if err := store.UpdateStepResult(ctx, storage.Step{ID: step.ID, Status: "done", Output: "ok\n"}); err != nil {
			return err
		}
		if err := store.UpdateWaitingApprovalAnswer(ctx, step.RunID, "approved output"); err != nil {
			return err
		}
		return store.UpdateRunStatus(ctx, step.RunID, "done")
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
	if step.Status != "done" {
		t.Fatalf("approved step status = %s", step.Status)
	}
	steps, err := store.ListRunSteps(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("ListRunSteps: %v", err)
	}
	var commandSteps int
	var finalAnswer string
	for _, item := range steps {
		if item.Kind == "command" {
			commandSteps++
		}
		if item.Kind == "answer" {
			finalAnswer = item.Output
		}
	}
	if commandSteps != 1 {
		t.Fatalf("command steps = %d", commandSteps)
	}
	if finalAnswer != "approved output" {
		t.Fatalf("final answer = %q", finalAnswer)
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
