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
	server := NewServer(Options{
		Store: store,
		Ask: func(ctx context.Context, session storage.Session, prompt string) (Answer, error) {
			return Answer{Text: "done", RunID: 1, StepCount: 2, DurationMillis: 15}, nil
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
