package storage

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"
)

func (s *Store) EnsureDefaultSession(ctx context.Context, workingDir string) (Session, error) {
	if _, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO sessions(name, working_directory) VALUES('default', ?)`, workingDir); err != nil {
		return Session{}, err
	}
	return s.GetSessionByName(ctx, "default")
}

func (s *Store) CreateSession(ctx context.Context, name string, workingDir string) (Session, error) {
	result, err := s.db.ExecContext(ctx, `INSERT INTO sessions(name, working_directory) VALUES(?, ?)`, name, workingDir)
	if err != nil {
		return Session{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Session{}, err
	}
	return s.GetSession(ctx, id)
}

func (s *Store) GetSession(ctx context.Context, id int64) (Session, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, name, description, mode, working_directory, last_used_at, created_at, updated_at FROM sessions WHERE id = ?`, id)
	return scanSession(row)
}

func (s *Store) GetSessionByName(ctx context.Context, name string) (Session, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, name, description, mode, working_directory, last_used_at, created_at, updated_at FROM sessions WHERE name = ?`, name)
	return scanSession(row)
}

func (s *Store) ListSessions(ctx context.Context) ([]Session, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, description, mode, working_directory, last_used_at, created_at, updated_at FROM sessions ORDER BY last_used_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sessions []Session
	for rows.Next() {
		session, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

func (s *Store) SetCurrentSessionName(ctx context.Context, name string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO app_settings(key, value) VALUES('current_session', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, name)
	return err
}

func (s *Store) CurrentSessionName(ctx context.Context) (string, error) {
	var name string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = 'current_session'`).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "default", nil
	}
	return name, err
}

func (s *Store) SetSessionMode(ctx context.Context, sessionID int64, mode string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET mode = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, mode, sessionID)
	return err
}

func (s *Store) SetWorkingDirectory(ctx context.Context, sessionID int64, workingDir string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET working_directory = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, workingDir, sessionID)
	return err
}

func (s *Store) CreateRun(ctx context.Context, sessionID int64, prompt string) (Run, error) {
	result, err := s.db.ExecContext(ctx, `INSERT INTO runs(session_id, prompt, status) VALUES(?, ?, 'running')`, sessionID, prompt)
	if err != nil {
		return Run{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Run{}, err
	}
	return s.GetRun(ctx, id)
}

func (s *Store) GetRun(ctx context.Context, id int64) (Run, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, session_id, prompt, status, created_at, updated_at FROM runs WHERE id = ?`, id)
	return scanRun(row)
}

func (s *Store) LatestRun(ctx context.Context, sessionID int64) (Run, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, session_id, prompt, status, created_at, updated_at FROM runs WHERE session_id = ? ORDER BY id DESC LIMIT 1`, sessionID)
	return scanRun(row)
}

func (s *Store) UpdateRunStatus(ctx context.Context, runID int64, status string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE runs SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, status, runID)
	return err
}

func (s *Store) AddStep(ctx context.Context, step Step) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO steps(run_id, kind, status, input, output, error_text, exit_code, duration_millis, stdout_bytes_observed, stderr_bytes_observed) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		step.RunID, step.Kind, step.Status, step.Input, step.Output, step.ErrorText, step.ExitCode, step.DurationMillis, step.StdoutBytesObserved, step.StderrBytesObserved)
	return err
}

func (s *Store) UpdateStepResult(ctx context.Context, step Step) error {
	_, err := s.db.ExecContext(ctx, `UPDATE steps SET status = ?, output = ?, error_text = ?, exit_code = ?, duration_millis = ?, stdout_bytes_observed = ?, stderr_bytes_observed = ? WHERE id = ?`,
		step.Status, step.Output, step.ErrorText, step.ExitCode, step.DurationMillis, step.StdoutBytesObserved, step.StderrBytesObserved, step.ID)
	return err
}

func (s *Store) ListRunSteps(ctx context.Context, runID int64) ([]Step, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, run_id, kind, status, input, output, error_text, exit_code, duration_millis, stdout_bytes_observed, stderr_bytes_observed, created_at FROM steps WHERE run_id = ? ORDER BY id`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var steps []Step
	for rows.Next() {
		var step Step
		var created string
		if err := rows.Scan(&step.ID, &step.RunID, &step.Kind, &step.Status, &step.Input, &step.Output, &step.ErrorText, &step.ExitCode, &step.DurationMillis, &step.StdoutBytesObserved, &step.StderrBytesObserved, &created); err != nil {
			return nil, err
		}
		step.CreatedAt = parseTime(created)
		steps = append(steps, step)
	}
	return steps, rows.Err()
}

func (s *Store) GetStep(ctx context.Context, stepID int64) (Step, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, run_id, kind, status, input, output, error_text, exit_code, duration_millis, stdout_bytes_observed, stderr_bytes_observed, created_at FROM steps WHERE id = ?`, stepID)
	return scanStep(row)
}

func (s *Store) AddMessage(ctx context.Context, sessionID int64, runID int64, role string, content string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO messages(session_id, run_id, role, content) VALUES(?, ?, ?, ?)`, sessionID, runID, role, content)
	return err
}

func (s *Store) ListRecentMessages(ctx context.Context, sessionID int64, limit int) ([]Message, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, session_id, run_id, role, content, created_at FROM messages WHERE session_id = ? ORDER BY id DESC LIMIT ?`, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reversed []Message
	for rows.Next() {
		message, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		reversed = append(reversed, message)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}
	return reversed, nil
}

func (s *Store) ListRecentRuns(ctx context.Context, sessionID int64, limit int) ([]Run, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, session_id, prompt, status, created_at, updated_at FROM runs WHERE session_id = ? ORDER BY id DESC LIMIT ?`, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var runs []Run
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (s *Store) SetSessionSummary(ctx context.Context, sessionID int64, summary string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO session_summaries(session_id, summary) VALUES(?, ?) ON CONFLICT(session_id) DO UPDATE SET summary = excluded.summary, updated_at = CURRENT_TIMESTAMP`, sessionID, summary)
	return err
}

func (s *Store) SessionSummary(ctx context.Context, sessionID int64) (string, error) {
	var summary string
	err := s.db.QueryRowContext(ctx, `SELECT summary FROM session_summaries WHERE session_id = ?`, sessionID).Scan(&summary)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return summary, err
}

func (s *Store) DeleteRun(ctx context.Context, sessionID int64, runID int64, source string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var existing int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM runs WHERE id = ? AND session_id = ?`, runID, sessionID).Scan(&existing); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM steps WHERE run_id = ?`, runID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM approvals WHERE run_id = ?`, runID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM messages WHERE session_id = ? AND run_id = ?`, sessionID, runID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM runs WHERE id = ? AND session_id = ?`, runID, sessionID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM session_summaries WHERE session_id = ?`, sessionID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO deletion_events(session_id, deleted_ref, source) VALUES(?, ?, ?)`, sessionID, "run:"+strconv.FormatInt(runID, 10), source); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) SetProviderConfig(ctx context.Context, config ProviderConfig) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO provider_config(id, base_url, model, api_key) VALUES(1, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET base_url = excluded.base_url, model = excluded.model, api_key = excluded.api_key, updated_at = CURRENT_TIMESTAMP`, config.BaseURL, config.Model, config.APIKey)
	return err
}

func (s *Store) ProviderConfig(ctx context.Context) (ProviderConfig, error) {
	var config ProviderConfig
	err := s.db.QueryRowContext(ctx, `SELECT base_url, model, api_key FROM provider_config WHERE id = 1`).Scan(&config.BaseURL, &config.Model, &config.APIKey)
	if errors.Is(err, sql.ErrNoRows) {
		return ProviderConfig{}, nil
	}
	return config, err
}

func (s *Store) SetSearchConfig(ctx context.Context, config SearchConfig) error {
	enabled := 0
	if config.Enabled {
		enabled = 1
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO search_config(id, enabled, tavily_api_key) VALUES(1, ?, ?) ON CONFLICT(id) DO UPDATE SET enabled = excluded.enabled, tavily_api_key = excluded.tavily_api_key, updated_at = CURRENT_TIMESTAMP`, enabled, config.TavilyAPIKey)
	return err
}

func (s *Store) SearchConfig(ctx context.Context) (SearchConfig, error) {
	var config SearchConfig
	var enabled int
	err := s.db.QueryRowContext(ctx, `SELECT enabled, tavily_api_key FROM search_config WHERE id = 1`).Scan(&enabled, &config.TavilyAPIKey)
	if errors.Is(err, sql.ErrNoRows) {
		return SearchConfig{}, nil
	}
	config.Enabled = enabled == 1
	return config, err
}

func (s *Store) AddApproval(ctx context.Context, sessionID int64, runID int64, command string, decision string, source string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO approvals(session_id, run_id, command, decision, source) VALUES(?, ?, ?, ?, ?)`, sessionID, runID, command, decision, source)
	return err
}

func (s *Store) AddAlwaysApproveRule(ctx context.Context, sessionID int64, command string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO always_approve_rules(session_id, match_type, pattern) VALUES(?, 'exact', ?)`, sessionID, command)
	return err
}

func (s *Store) AlwaysApproveRules(ctx context.Context, sessionID int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT pattern FROM always_approve_rules WHERE session_id = ? AND match_type = 'exact' ORDER BY id`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rules []string
	for rows.Next() {
		var rule string
		if err := rows.Scan(&rule); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSession(row rowScanner) (Session, error) {
	var session Session
	var lastUsed, created, updated string
	if err := row.Scan(&session.ID, &session.Name, &session.Description, &session.Mode, &session.WorkingDirectory, &lastUsed, &created, &updated); err != nil {
		return Session{}, err
	}
	session.LastUsedAt = parseTime(lastUsed)
	session.CreatedAt = parseTime(created)
	session.UpdatedAt = parseTime(updated)
	return session, nil
}

func scanRun(row rowScanner) (Run, error) {
	var run Run
	var created, updated string
	if err := row.Scan(&run.ID, &run.SessionID, &run.Prompt, &run.Status, &created, &updated); err != nil {
		return Run{}, err
	}
	run.CreatedAt = parseTime(created)
	run.UpdatedAt = parseTime(updated)
	return run, nil
}

func scanStep(row rowScanner) (Step, error) {
	var step Step
	var created string
	if err := row.Scan(&step.ID, &step.RunID, &step.Kind, &step.Status, &step.Input, &step.Output, &step.ErrorText, &step.ExitCode, &step.DurationMillis, &step.StdoutBytesObserved, &step.StderrBytesObserved, &created); err != nil {
		return Step{}, err
	}
	step.CreatedAt = parseTime(created)
	return step, nil
}

func scanMessage(row rowScanner) (Message, error) {
	var message Message
	var created string
	if err := row.Scan(&message.ID, &message.SessionID, &message.RunID, &message.Role, &message.Content, &created); err != nil {
		return Message{}, err
	}
	message.CreatedAt = parseTime(created)
	return message, nil
}

func parseTime(value string) time.Time {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed
		}
	}
	return time.Time{}
}
