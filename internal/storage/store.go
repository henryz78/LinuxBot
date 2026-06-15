package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

func (s *Store) EnsureDefaultSession(ctx context.Context, workingDir string) (Session, error) {
	session, err := s.GetSessionByName(ctx, "default")
	if err == nil {
		return session, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Session{}, err
	}
	return s.CreateSession(ctx, "default", workingDir)
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

func (s *Store) CreateRun(ctx context.Context, sessionID int64, prompt string) (Run, error) {
	result, err := s.db.ExecContext(ctx, `INSERT INTO runs(session_id, prompt, status) VALUES(?, ?, 'running')`, sessionID, prompt)
	if err != nil {
		return Run{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Run{}, err
	}
	return Run{ID: id, SessionID: sessionID, Prompt: prompt, Status: "running"}, nil
}

func (s *Store) AddStep(ctx context.Context, step Step) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO steps(run_id, kind, status, input, output, error_text, exit_code, duration_millis, stdout_bytes_observed, stderr_bytes_observed) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		step.RunID, step.Kind, step.Status, step.Input, step.Output, step.ErrorText, step.ExitCode, step.DurationMillis, step.StdoutBytesObserved, step.StderrBytesObserved)
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

type sessionScanner interface {
	Scan(dest ...any) error
}

func scanSession(row sessionScanner) (Session, error) {
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

func parseTime(value string) time.Time {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed
		}
	}
	return time.Time{}
}
