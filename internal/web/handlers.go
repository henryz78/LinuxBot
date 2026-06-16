package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"linuxbot/internal/storage"
)

func (s *Server) routes() {
	s.mux.HandleFunc("/api/health", s.handleHealth)
	s.mux.HandleFunc("/api/sessions", s.handleSessions)
	s.mux.HandleFunc("/api/history", s.handleHistory)
	s.mux.HandleFunc("/api/mode", s.handleMode)
	s.mux.HandleFunc("/api/ask", s.handleAsk)
	s.mux.HandleFunc("/api/runs/", s.handleRun)
	s.mux.HandleFunc("/api/delete-run", s.handleDeleteRun)
	s.mux.HandleFunc("/api/approve", s.handleApprove)
	s.mux.Handle("/", http.FileServer(staticFS()))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	if s.options.Store == nil {
		writeJSON(w, []storage.Session{})
		return
	}
	sessions, err := s.options.Store.ListSessions(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, sessions)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.options.Store == nil {
		http.Error(w, "store is not configured", http.StatusServiceUnavailable)
		return
	}
	sessionID, err := strconv.ParseInt(r.URL.Query().Get("session_id"), 10, 64)
	if err != nil || sessionID <= 0 {
		http.Error(w, "invalid session_id", http.StatusBadRequest)
		return
	}
	runs, err := s.options.Store.ListRecentRuns(r.Context(), sessionID, 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	views := make([]RunView, 0, len(runs))
	for i := len(runs) - 1; i >= 0; i-- {
		view, err := s.runView(r.Context(), runs[i])
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		views = append(views, view)
	}
	writeJSON(w, views)
}

func (s *Server) handleMode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.options.Store == nil {
		http.Error(w, "store is not configured", http.StatusServiceUnavailable)
		return
	}
	var request struct {
		SessionID int64  `json:"session_id"`
		Mode      string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if request.Mode != "safe" && request.Mode != "review" && request.Mode != "open" {
		http.Error(w, "mode must be safe, review, or open", http.StatusBadRequest)
		return
	}
	if err := s.options.Store.SetSessionMode(r.Context(), request.SessionID, request.Mode); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleAsk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.options.Store == nil || s.options.Ask == nil {
		http.Error(w, "agent is not configured", http.StatusServiceUnavailable)
		return
	}
	var request struct {
		SessionID int64  `json:"session_id"`
		Prompt    string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	session, err := s.options.Store.GetSession(r.Context(), request.SessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	answer, err := s.options.Ask(r.Context(), session, request.Prompt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	run, err := s.options.Store.GetRun(r.Context(), answer.RunID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	view, err := s.runView(r.Context(), run)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"answer": answer.Text,
		"run_id": answer.RunID,
		"run":    view,
		"summary": "已处理 " + strconv.Itoa(answer.StepCount) + " 个步骤 · " +
			strconv.FormatInt(answer.DurationMillis, 10) + "ms",
	})
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	if s.options.Store == nil {
		http.Error(w, "store is not configured", http.StatusServiceUnavailable)
		return
	}
	idText := r.URL.Path[len("/api/runs/"):]
	runID, err := strconv.ParseInt(idText, 10, 64)
	if err != nil {
		http.Error(w, "invalid run id", http.StatusBadRequest)
		return
	}
	sessionID, err := strconv.ParseInt(r.URL.Query().Get("session_id"), 10, 64)
	if err != nil || sessionID <= 0 {
		http.Error(w, "invalid session_id", http.StatusBadRequest)
		return
	}
	run, err := s.options.Store.GetRun(r.Context(), runID)
	if err != nil || run.SessionID != sessionID {
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}
	steps, err := s.options.Store.ListRunSteps(r.Context(), runID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, steps)
}

func (s *Server) handleDeleteRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request struct {
		SessionID int64 `json:"session_id"`
		RunID     int64 `json:"run_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.options.Store.DeleteRun(r.Context(), request.SessionID, request.RunID, "web"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.options.Store == nil || s.options.ApproveCommand == nil {
		http.Error(w, "approval is not configured", http.StatusServiceUnavailable)
		return
	}
	var request struct {
		SessionID int64 `json:"session_id"`
		StepID    int64 `json:"step_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	session, err := s.options.Store.GetSession(r.Context(), request.SessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	step, err := s.options.Store.GetStep(r.Context(), request.StepID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	run, err := s.options.Store.GetRun(r.Context(), step.RunID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if run.SessionID != session.ID {
		http.Error(w, "step does not belong to session", http.StatusNotFound)
		return
	}
	if step.Status != "approval_required" {
		http.Error(w, "step is not pending approval", http.StatusConflict)
		return
	}
	if err := s.options.ApproveCommand(r.Context(), session, step); err != nil {
		if errors.Is(err, storage.ErrStepNotPendingApproval) {
			http.Error(w, "step is not pending approval", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	updated, err := s.options.Store.GetStep(r.Context(), step.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if updated.Status == "approval_required" {
		http.Error(w, "approval did not execute the command", http.StatusInternalServerError)
		return
	}
	updatedRun, err := s.options.Store.GetRun(r.Context(), step.RunID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	view, err := s.runView(r.Context(), updatedRun)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, view)
}

func (s *Server) runView(ctx context.Context, run storage.Run) (RunView, error) {
	steps, err := s.options.Store.ListRunSteps(ctx, run.ID)
	if err != nil {
		return RunView{}, err
	}
	answer := ""
	var duration int64
	for _, step := range steps {
		duration += step.DurationMillis
		if step.Kind == "answer" {
			answer = step.Output
		}
	}
	if answer == "" {
		switch run.Status {
		case "waiting_approval":
			answer = "有命令需要批准后才能继续执行。"
		case "failed":
			answer = "执行失败。"
		default:
			answer = "处理中。"
		}
	}
	return RunView{
		ID:             run.ID,
		SessionID:      run.SessionID,
		Prompt:         run.Prompt,
		Status:         run.Status,
		Answer:         answer,
		Summary:        fmt.Sprintf("已处理 %d 个步骤 · %dms", len(steps), duration),
		StepCount:      len(steps),
		DurationMillis: duration,
		CreatedAt:      run.CreatedAt,
		UpdatedAt:      run.UpdatedAt,
	}, nil
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
