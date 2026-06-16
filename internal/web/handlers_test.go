package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"linuxbot/internal/storage"
)

func TestAskEndpointReturnsCollapsedRun(t *testing.T) {
	store, err := storage.Open(t.TempDir() + "/linuxbot.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	session, err := store.EnsureDefaultSession(context.Background(), "/tmp")
	if err != nil {
		t.Fatalf("EnsureDefaultSession: %v", err)
	}
	run, err := store.CreateRun(context.Background(), session.ID, "status")
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := store.AddStep(context.Background(), storage.Step{RunID: run.ID, Kind: "answer", Status: "done", Output: "done"}); err != nil {
		t.Fatalf("AddStep answer: %v", err)
	}
	server := NewServer(Options{
		Store: store,
		Ask: func(ctx context.Context, session storage.Session, prompt string) (Answer, error) {
			return Answer{Text: "done", RunID: run.ID, StepCount: 2, DurationMillis: 15}, nil
		},
	})
	body := strings.NewReader(`{"session_id":` + strconv.FormatInt(session.ID, 10) + `,"prompt":"status"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/ask", body)
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"answer":"done"`) {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

func TestAskEndpointReturnsRunJSONWhenAgentFailsAfterCreatingRun(t *testing.T) {
	store, err := storage.Open(t.TempDir() + "/linuxbot.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	session, err := store.EnsureDefaultSession(context.Background(), "/tmp")
	if err != nil {
		t.Fatalf("EnsureDefaultSession: %v", err)
	}
	run, err := store.CreateRun(context.Background(), session.ID, "status")
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := store.AddStep(context.Background(), storage.Step{RunID: run.ID, Kind: "answer", Status: "failed", Output: "执行失败：provider status 500"}); err != nil {
		t.Fatalf("AddStep answer: %v", err)
	}
	if err := store.UpdateRunStatus(context.Background(), run.ID, "failed"); err != nil {
		t.Fatalf("UpdateRunStatus: %v", err)
	}
	server := NewServer(Options{
		Store: store,
		Ask: func(ctx context.Context, session storage.Session, prompt string) (Answer, error) {
			return Answer{RunID: run.ID}, fmt.Errorf("provider status 500")
		},
	})
	body := strings.NewReader(`{"session_id":` + strconv.FormatInt(session.ID, 10) + `,"prompt":"status"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/ask", body)

	server.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	response := recorder.Body.String()
	if !strings.Contains(response, `"error":"provider status 500"`) {
		t.Fatalf("body = %s", response)
	}
	if !strings.Contains(response, `"run"`) || !strings.Contains(response, `"status":"failed"`) {
		t.Fatalf("body = %s", response)
	}
}

func TestModeEndpointPersistsSessionMode(t *testing.T) {
	store, err := storage.Open(t.TempDir() + "/linuxbot.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	session, err := store.EnsureDefaultSession(context.Background(), "/tmp")
	if err != nil {
		t.Fatalf("EnsureDefaultSession: %v", err)
	}
	server := NewServer(Options{Store: store})
	body := strings.NewReader(`{"session_id":` + strconv.FormatInt(session.ID, 10) + `,"mode":"review"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/mode", body)

	server.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	updated, err := store.GetSession(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if updated.Mode != "review" {
		t.Fatalf("mode = %s", updated.Mode)
	}
}

func TestHistoryEndpointReturnsStoredConversationRuns(t *testing.T) {
	store, err := storage.Open(t.TempDir() + "/linuxbot.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	session, err := store.EnsureDefaultSession(context.Background(), "/tmp")
	if err != nil {
		t.Fatalf("EnsureDefaultSession: %v", err)
	}
	run, err := store.CreateRun(context.Background(), session.ID, "check disk")
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := store.AddMessage(context.Background(), session.ID, run.ID, "user", "check disk"); err != nil {
		t.Fatalf("AddMessage user: %v", err)
	}
	if err := store.AddStep(context.Background(), storage.Step{RunID: run.ID, Kind: "command", Status: "done", Input: "df -h", Output: "ok\n"}); err != nil {
		t.Fatalf("AddStep command: %v", err)
	}
	if err := store.AddStep(context.Background(), storage.Step{RunID: run.ID, Kind: "answer", Status: "done", Output: "disk is ok"}); err != nil {
		t.Fatalf("AddStep answer: %v", err)
	}
	if err := store.AddMessage(context.Background(), session.ID, run.ID, "assistant", "disk is ok"); err != nil {
		t.Fatalf("AddMessage assistant: %v", err)
	}
	if err := store.UpdateRunStatus(context.Background(), run.ID, "done"); err != nil {
		t.Fatalf("UpdateRunStatus: %v", err)
	}
	server := NewServer(Options{Store: store})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/history?session_id="+strconv.FormatInt(session.ID, 10), nil)

	server.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"prompt":"check disk"`) {
		t.Fatalf("body = %s", body)
	}
	if !strings.Contains(body, `"answer":"disk is ok"`) {
		t.Fatalf("body = %s", body)
	}
	if !strings.Contains(body, `"summary":"已处理 2 个步骤`) {
		t.Fatalf("body = %s", body)
	}
}

func TestRunEndpointRequiresMatchingSession(t *testing.T) {
	store, err := storage.Open(t.TempDir() + "/linuxbot.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	first, err := store.CreateSession(context.Background(), "first", "/tmp")
	if err != nil {
		t.Fatalf("CreateSession first: %v", err)
	}
	second, err := store.CreateSession(context.Background(), "second", "/tmp")
	if err != nil {
		t.Fatalf("CreateSession second: %v", err)
	}
	run, err := store.CreateRun(context.Background(), second.ID, "secret")
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := store.AddStep(context.Background(), storage.Step{RunID: run.ID, Kind: "command", Status: "done", Input: "whoami", Output: "root"}); err != nil {
		t.Fatalf("AddStep: %v", err)
	}
	server := NewServer(Options{Store: store})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/runs/"+strconv.FormatInt(run.ID, 10)+"?session_id="+strconv.FormatInt(first.ID, 10), nil)

	server.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
}
