# LinuxBot MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the LinuxBot MVP as a Go single binary with interactive CLI, SQLite sessions/runs/steps, BYOK OpenAI-compatible provider, policy-gated shell execution, Tavily search, and a local-only embedded Web UI.

**Architecture:** The CLI and Web UI both call the same service layer. The agent talks to an LLM provider, routes executable actions through a small tool router, records each run/step in SQLite, and applies policy before shell execution. The Web UI is embedded with `go:embed` and reads the same SQLite state as the CLI.

**Tech Stack:** Go 1.22+, `database/sql`, `modernc.org/sqlite`, standard `net/http`, standard `embed`, no heavy frontend framework.

---

## File Structure

- Create: `go.mod` - module definition and dependencies.
- Create: `cmd/linuxbot/main.go` - executable entrypoint and top-level command routing.
- Create: `internal/app/app.go` - dependency wiring for storage, provider, tools, agent, CLI, and Web server.
- Create: `internal/storage/db.go` - SQLite open, pragmas, and migrations.
- Create: `internal/storage/models.go` - persisted model structs.
- Create: `internal/storage/store.go` - session, message, run, step, approval, config, summary, and deletion queries.
- Create: `internal/session/service.go` - session create/list/switch/delete and working-directory updates.
- Create: `internal/provider/provider.go` - provider interface and chat request/response types.
- Create: `internal/provider/openai_compatible.go` - OpenAI-compatible HTTP implementation.
- Create: `internal/provider/fake.go` - scripted provider for tests.
- Create: `internal/contextmgr/context.go` - model context assembly and session summary update policy.
- Create: `internal/policy/normalize.go` - command normalization and shell wrapper extraction.
- Create: `internal/policy/policy.go` - safe allowlist, critical denylist, mode decisions, approval matching.
- Create: `internal/tool/tool.go` - tool interface, request/result types, and router.
- Create: `internal/tool/shell.go` - shell tool with normalization, policy, approval, runner, and output truncation.
- Create: `internal/tool/search.go` - Tavily search tool.
- Create: `internal/agent/agent.go` - run loop that asks provider, parses actions, routes tools, and persists steps.
- Create: `internal/agent/protocol.go` - JSON protocol between provider output and agent actions.
- Create: `internal/cli/cli.go` - interactive CLI loop, session commands, and approval prompts.
- Create: `internal/web/server.go` - local HTTP server.
- Create: `internal/web/handlers.go` - JSON handlers for sessions, messages, runs, approvals, settings, and deletion.
- Create: `internal/web/static/index.html` - embedded Web shell.
- Create: `internal/web/static/app.js` - small frontend behavior.
- Create: `internal/web/static/style.css` - restrained operational UI styling.
- Create tests next to packages as `*_test.go`.

## Execution Notes

- This workspace is currently not a git repository. During implementation, run `git init` before the first commit.
- Run `go test ./...` after each task.
- Keep commits task-scoped.
- Use fake providers in tests. Do not call real LLM or Tavily APIs in automated tests.

---

### Task 1: Go Module And App Skeleton

**Files:**
- Create: `go.mod`
- Create: `cmd/linuxbot/main.go`
- Create: `internal/app/app.go`

- [ ] **Step 1: Initialize git and Go module**

Run:

```bash
git init
go mod init linuxbot
```

Expected: `.git/` exists and `go.mod` contains `module linuxbot`.

- [ ] **Step 2: Add SQLite dependency**

Run:

```bash
go get modernc.org/sqlite
```

Expected: `go.mod` includes `modernc.org/sqlite`.

- [ ] **Step 3: Create the entrypoint**

Create `cmd/linuxbot/main.go`:

```go
package main

import (
	"context"
	"fmt"
	"os"

	"linuxbot/internal/app"
)

func main() {
	if err := app.Run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 4: Create minimal app command routing**

Create `internal/app/app.go`:

```go
package app

import (
	"context"
	"fmt"
	"io"
)

func Run(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		_, err := fmt.Fprintln(stdout, "linuxbot bootstrap interactive CLI")
		return err
	}
	switch args[0] {
	case "sessions", "switch", "config", "web":
		_, err := fmt.Fprintf(stdout, "linuxbot %s bootstrap command\n", args[0])
		return err
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}
```

- [ ] **Step 5: Run skeleton build**

Run:

```bash
go test ./...
go run ./cmd/linuxbot sessions
```

Expected: tests pass and command prints `linuxbot sessions bootstrap command`.

- [ ] **Step 6: Commit**

Run:

```bash
git add go.mod go.sum cmd internal
git commit -m "chore: scaffold linuxbot module"
```

---

### Task 2: SQLite Schema And Store

**Files:**
- Create: `internal/storage/db.go`
- Create: `internal/storage/models.go`
- Create: `internal/storage/store.go`
- Create: `internal/storage/store_test.go`

- [ ] **Step 1: Write failing storage tests**

Create `internal/storage/store_test.go`:

```go
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
		RunID: run.ID,
		Kind:  "answer",
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
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/storage
```

Expected: fails because storage types are missing.

- [ ] **Step 3: Implement models**

Create `internal/storage/models.go`:

```go
package storage

import "time"

type Session struct {
	ID               int64
	Name             string
	Description      string
	Mode             string
	WorkingDirectory string
	LastUsedAt       time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type Run struct {
	ID        int64
	SessionID int64
	Prompt    string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Step struct {
	ID                 int64
	RunID              int64
	Kind               string
	Status             string
	Input              string
	Output             string
	ErrorText          string
	ExitCode           int
	DurationMillis     int64
	StdoutBytesObserved int64
	StderrBytesObserved int64
	CreatedAt          time.Time
}
```

- [ ] **Step 4: Implement SQLite open and migrations**

Create `internal/storage/db.go`:

```go
package storage

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	statements := []string{
		`PRAGMA journal_mode=WAL`,
		`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			description TEXT NOT NULL DEFAULT '',
			mode TEXT NOT NULL DEFAULT 'safe',
			working_directory TEXT NOT NULL,
			last_used_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS session_summaries (
			session_id INTEGER PRIMARY KEY,
			summary TEXT NOT NULL,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(session_id) REFERENCES sessions(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id INTEGER NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(session_id) REFERENCES sessions(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id INTEGER NOT NULL,
			prompt TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(session_id) REFERENCES sessions(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS steps (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id INTEGER NOT NULL,
			kind TEXT NOT NULL,
			status TEXT NOT NULL,
			input TEXT NOT NULL DEFAULT '',
			output TEXT NOT NULL DEFAULT '',
			error_text TEXT NOT NULL DEFAULT '',
			exit_code INTEGER NOT NULL DEFAULT 0,
			duration_millis INTEGER NOT NULL DEFAULT 0,
			stdout_bytes_observed INTEGER NOT NULL DEFAULT 0,
			stderr_bytes_observed INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(run_id) REFERENCES runs(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS approvals (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id INTEGER NOT NULL,
			run_id INTEGER NOT NULL,
			command TEXT NOT NULL,
			decision TEXT NOT NULL,
			source TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS always_approve_rules (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id INTEGER NOT NULL,
			match_type TEXT NOT NULL DEFAULT 'exact',
			pattern TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS provider_config (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			base_url TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '',
			api_key TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS search_config (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			enabled INTEGER NOT NULL DEFAULT 0,
			tavily_api_key TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS deletion_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id INTEGER NOT NULL,
			deleted_ref TEXT NOT NULL,
			source TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}
```

- [ ] **Step 5: Implement store queries**

Create `internal/storage/store.go`:

```go
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
	parsed, err := time.Parse("2006-01-02 15:04:05", value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}
```

- [ ] **Step 6: Run storage tests**

Run:

```bash
go test ./internal/storage
```

Expected: PASS.

- [ ] **Step 7: Commit**

Run:

```bash
git add internal/storage go.mod go.sum
git commit -m "feat: add sqlite storage schema"
```

---

### Task 3: Session Service And CLI Session Commands

**Files:**
- Create: `internal/session/service.go`
- Create: `internal/session/service_test.go`
- Modify: `internal/app/app.go`

- [ ] **Step 1: Write failing session service tests**

Create `internal/session/service_test.go`:

```go
package session

import (
	"context"
	"testing"

	"linuxbot/internal/storage"
)

func TestServiceSwitchesDefaultSession(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(t.TempDir() + "/linuxbot.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	service := NewService(store)
	created, err := service.Create(ctx, "prod", "/opt/app")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Name != "prod" {
		t.Fatalf("name = %q", created.Name)
	}
	if err := service.Switch(ctx, "prod"); err != nil {
		t.Fatalf("Switch: %v", err)
	}
	current, err := service.Current(ctx, "/tmp")
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if current.Name != "prod" {
		t.Fatalf("current = %q", current.Name)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/session
```

Expected: fails because session service is missing.

- [ ] **Step 3: Extend storage for settings and session list**

Append to `internal/storage/store.go`:

```go
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
	_, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS app_settings (key TEXT PRIMARY KEY, value TEXT NOT NULL)`)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO app_settings(key, value) VALUES('current_session', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, name)
	return err
}

func (s *Store) CurrentSessionName(ctx context.Context) (string, error) {
	_, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS app_settings (key TEXT PRIMARY KEY, value TEXT NOT NULL)`)
	if err != nil {
		return "", err
	}
	var name string
	err = s.db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = 'current_session'`).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "default", nil
	}
	return name, err
}
```

- [ ] **Step 4: Implement session service**

Create `internal/session/service.go`:

```go
package session

import (
	"context"

	"linuxbot/internal/storage"
)

type Service struct {
	store *storage.Store
}

func NewService(store *storage.Store) *Service {
	return &Service{store: store}
}

func (s *Service) Current(ctx context.Context, fallbackWorkingDir string) (storage.Session, error) {
	name, err := s.store.CurrentSessionName(ctx)
	if err != nil {
		return storage.Session{}, err
	}
	if name == "default" {
		return s.store.EnsureDefaultSession(ctx, fallbackWorkingDir)
	}
	return s.store.GetSessionByName(ctx, name)
}

func (s *Service) Create(ctx context.Context, name string, workingDir string) (storage.Session, error) {
	return s.store.CreateSession(ctx, name, workingDir)
}

func (s *Service) Switch(ctx context.Context, name string) error {
	if _, err := s.store.GetSessionByName(ctx, name); err != nil {
		return err
	}
	return s.store.SetCurrentSessionName(ctx, name)
}

func (s *Service) List(ctx context.Context) ([]storage.Session, error) {
	return s.store.ListSessions(ctx)
}
```

- [ ] **Step 5: Wire CLI session commands**

Replace `internal/app/app.go` with:

```go
package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"linuxbot/internal/session"
	"linuxbot/internal/storage"
)

func Run(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	dbPath, err := defaultDBPath()
	if err != nil {
		return err
	}
	store, err := storage.Open(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	sessionService := session.NewService(store)

	if len(args) == 0 {
		current, err := sessionService.Current(ctx, cwd)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(stdout, "linuxbot interactive CLI for session %s\n", current.Name)
		return err
	}

	switch args[0] {
	case "sessions":
		sessions, err := sessionService.List(ctx)
		if err != nil {
			return err
		}
		for _, item := range sessions {
			if _, err := fmt.Fprintf(stdout, "%s\t%s\t%s\n", item.Name, item.Mode, item.WorkingDirectory); err != nil {
				return err
			}
		}
		return nil
	case "switch":
		if len(args) != 2 {
			return fmt.Errorf("usage: linuxbot switch <name>")
		}
		return sessionService.Switch(ctx, args[1])
	case "web", "config":
		_, err := fmt.Fprintf(stdout, "linuxbot %s bootstrap command\n", args[0])
		return err
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func defaultDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".linuxbot")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "linuxbot.db"), nil
}
```

- [ ] **Step 6: Run tests and manual session check**

Run:

```bash
go test ./...
go run ./cmd/linuxbot
go run ./cmd/linuxbot sessions
```

Expected: tests pass, default session is created, and sessions command lists it.

- [ ] **Step 7: Commit**

Run:

```bash
git add internal cmd
git commit -m "feat: add session service"
```

---

### Task 4: Provider Abstraction And OpenAI-Compatible Client

**Files:**
- Create: `internal/provider/provider.go`
- Create: `internal/provider/openai_compatible.go`
- Create: `internal/provider/fake.go`
- Create: `internal/provider/openai_compatible_test.go`

- [ ] **Step 1: Write provider interface**

Create `internal/provider/provider.go`:

```go
package provider

import "context"

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Message struct {
	Role    Role
	Content string
}

type ChatRequest struct {
	Messages []Message
	Model    string
}

type ChatResponse struct {
	Content string
}

type Provider interface {
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
}
```

- [ ] **Step 2: Write failing OpenAI-compatible test**

Create `internal/provider/openai_compatible_test.go`:

```go
package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAICompatibleProviderSendsChatCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body["model"] != "test-model" {
			t.Fatalf("model = %#v", body["model"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"hello"}}]}`))
	}))
	defer server.Close()

	client := NewOpenAICompatible(server.URL, "test-model", "test-key", server.Client())
	response, err := client.Chat(context.Background(), ChatRequest{Messages: []Message{{Role: RoleUser, Content: "hi"}}})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if response.Content != "hello" {
		t.Fatalf("content = %q", response.Content)
	}
}
```

- [ ] **Step 3: Implement provider client**

Create `internal/provider/openai_compatible.go`:

```go
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type OpenAICompatible struct {
	baseURL string
	model   string
	apiKey  string
	client  *http.Client
}

func NewOpenAICompatible(baseURL string, model string, apiKey string, client *http.Client) *OpenAICompatible {
	if client == nil {
		client = http.DefaultClient
	}
	return &OpenAICompatible{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		apiKey:  apiKey,
		client:  client,
	}
}

func (p *OpenAICompatible) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}
	payload := map[string]any{
		"model":       model,
		"messages":    toOpenAIMessages(req.Messages),
		"temperature": 0,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return ChatResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return ChatResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return ChatResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ChatResponse{}, fmt.Errorf("provider status %d", resp.StatusCode)
	}
	var decoded struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return ChatResponse{}, err
	}
	if len(decoded.Choices) == 0 {
		return ChatResponse{}, fmt.Errorf("provider returned no choices")
	}
	return ChatResponse{Content: decoded.Choices[0].Message.Content}, nil
}

func toOpenAIMessages(messages []Message) []map[string]string {
	out := make([]map[string]string, 0, len(messages))
	for _, message := range messages {
		out = append(out, map[string]string{"role": string(message.Role), "content": message.Content})
	}
	return out
}
```

- [ ] **Step 4: Implement fake provider**

Create `internal/provider/fake.go`:

```go
package provider

import (
	"context"
	"fmt"
)

type Fake struct {
	Responses []ChatResponse
	Requests  []ChatRequest
}

func (f *Fake) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	f.Requests = append(f.Requests, req)
	if len(f.Responses) == 0 {
		return ChatResponse{}, fmt.Errorf("fake provider has no responses")
	}
	response := f.Responses[0]
	f.Responses = f.Responses[1:]
	return response, nil
}
```

- [ ] **Step 5: Run provider tests**

Run:

```bash
go test ./internal/provider
```

Expected: PASS.

- [ ] **Step 6: Commit**

Run:

```bash
git add internal/provider
git commit -m "feat: add provider abstraction"
```

---

### Task 5: Command Normalization And Policy

**Files:**
- Create: `internal/policy/normalize.go`
- Create: `internal/policy/policy.go`
- Create: `internal/policy/policy_test.go`

- [ ] **Step 1: Write policy tests**

Create `internal/policy/policy_test.go`:

```go
package policy

import "testing"

func TestCriticalDenylistCatchesDirectAndWrappedCommands(t *testing.T) {
	cases := []string{
		"rm -rf /",
		"bash -c \"rm -rf /\"",
		"sh -c 'shutdown now'",
		"eval \"poweroff\"",
		"dd if=/dev/zero of=/dev/sda",
	}
	for _, command := range cases {
		normalized := Normalize(command)
		decision := Evaluate(EvaluationRequest{Mode: ModeOpen, Command: normalized})
		if decision.Action != ActionDeny {
			t.Fatalf("%q action = %s", command, decision.Action)
		}
	}
}

func TestSafeAllowlistAllowsSimpleReadCommand(t *testing.T) {
	decision := Evaluate(EvaluationRequest{Mode: ModeSafe, Command: Normalize("df -h")})
	if decision.Action != ActionAllow {
		t.Fatalf("action = %s reason = %s", decision.Action, decision.Reason)
	}
}

func TestSafeAllowlistRequiresApprovalForDocker(t *testing.T) {
	decision := Evaluate(EvaluationRequest{Mode: ModeSafe, Command: Normalize("docker ps")})
	if decision.Action != ActionApproval {
		t.Fatalf("action = %s", decision.Action)
	}
}

func TestReviewExactAlwaysApprove(t *testing.T) {
	decision := Evaluate(EvaluationRequest{
		Mode: ModeReview,
		Command: Normalize("systemctl restart nginx"),
		AlwaysApproveExact: []string{"systemctl restart nginx"},
	})
	if decision.Action != ActionAllow {
		t.Fatalf("action = %s", decision.Action)
	}
}
```

- [ ] **Step 2: Implement command normalization**

Create `internal/policy/normalize.go`:

```go
package policy

import (
	"strings"
)

type NormalizedCommand struct {
	Raw              string
	Trimmed          string
	Root             string
	Wrapped          string
	HasShellFeatures bool
}

func Normalize(raw string) NormalizedCommand {
	trimmed := strings.TrimSpace(raw)
	root := firstField(trimmed)
	wrapped := extractWrapped(trimmed)
	return NormalizedCommand{
		Raw:              raw,
		Trimmed:          trimmed,
		Root:             root,
		Wrapped:          wrapped,
		HasShellFeatures: hasShellFeatures(trimmed),
	}
}

func firstField(command string) string {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func extractWrapped(command string) string {
	fields := strings.Fields(command)
	if len(fields) < 3 {
		return ""
	}
	if (fields[0] == "bash" || fields[0] == "sh") && fields[1] == "-c" {
		return strings.Trim(strings.Join(fields[2:], " "), `"'`)
	}
	if fields[0] == "eval" && len(fields) >= 2 {
		return strings.Trim(strings.Join(fields[1:], " "), `"'`)
	}
	return ""
}

func hasShellFeatures(command string) bool {
	markers := []string{"&&", "||", ";", "|", ">", "<", "$(", "`"}
	for _, marker := range markers {
		if strings.Contains(command, marker) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 3: Implement policy evaluator**

Create `internal/policy/policy.go`:

```go
package policy

import "strings"

type Mode string

const (
	ModeSafe   Mode = "safe"
	ModeReview Mode = "review"
	ModeOpen   Mode = "open"
)

type Action string

const (
	ActionAllow    Action = "allow"
	ActionApproval Action = "approval"
	ActionDeny     Action = "deny"
)

type EvaluationRequest struct {
	Mode               Mode
	Command            NormalizedCommand
	AlwaysApproveExact []string
}

type Decision struct {
	Action Action
	Reason string
}

func Evaluate(req EvaluationRequest) Decision {
	if isCritical(req.Command.Trimmed) || isCritical(req.Command.Wrapped) {
		return Decision{Action: ActionDeny, Reason: "critical command denylist"}
	}
	switch req.Mode {
	case ModeSafe:
		if safeAllowed(req.Command) {
			return Decision{Action: ActionAllow, Reason: "safe allowlist"}
		}
		return Decision{Action: ActionApproval, Reason: "outside safe allowlist"}
	case ModeReview:
		if exactApproved(req.Command.Trimmed, req.AlwaysApproveExact) {
			return Decision{Action: ActionAllow, Reason: "always approved"}
		}
		return Decision{Action: ActionApproval, Reason: "review mode"}
	case ModeOpen:
		return Decision{Action: ActionAllow, Reason: "open mode"}
	default:
		return Decision{Action: ActionApproval, Reason: "unknown mode"}
	}
}

func isCritical(command string) bool {
	command = strings.TrimSpace(command)
	if command == "" {
		return false
	}
	checks := []string{
		"rm -rf /",
		"rm -rf /*",
		"reboot",
		"shutdown now",
		"poweroff",
		"userdel",
		"mkfs",
		"dd if=/dev/zero",
		"dd if=/dev/random",
		"dd if=/dev/urandom",
	}
	for _, check := range checks {
		if command == check || strings.HasPrefix(command, check+" ") {
			return true
		}
	}
	return false
}

func safeAllowed(command NormalizedCommand) bool {
	if command.HasShellFeatures || command.Wrapped != "" {
		return false
	}
	if strings.Contains(command.Trimmed, "/etc/shadow") || strings.Contains(command.Trimmed, ".env") || strings.Contains(command.Trimmed, "id_rsa") {
		return false
	}
	roots := map[string]bool{
		"ls": true, "pwd": true, "whoami": true, "id": true, "hostname": true, "date": true,
		"uptime": true, "uname": true, "ps": true, "df": true, "free": true, "cat": true,
		"head": true, "tail": true, "grep": true, "find": true, "du": true, "journalctl": true,
	}
	if roots[command.Root] {
		return true
	}
	return strings.HasPrefix(command.Trimmed, "systemctl status ")
}

func exactApproved(command string, rules []string) bool {
	for _, rule := range rules {
		if strings.TrimSpace(rule) == command {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run policy tests**

Run:

```bash
go test ./internal/policy
```

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
git add internal/policy
git commit -m "feat: add command policy"
```

---

### Task 6: Tool Router And Shell Tool

**Files:**
- Create: `internal/tool/tool.go`
- Create: `internal/tool/shell.go`
- Create: `internal/tool/shell_test.go`

- [ ] **Step 1: Write shell tool tests**

Create `internal/tool/shell_test.go`:

```go
package tool

import (
	"context"
	"strings"
	"testing"

	"linuxbot/internal/policy"
)

func TestShellToolRunsAllowedCommand(t *testing.T) {
	shell := NewShellTool(ShellOptions{Mode: policy.ModeSafe, WorkingDirectory: t.TempDir(), OutputLimitBytes: 1024})
	result, err := shell.Execute(context.Background(), ToolRequest{Name: "shell", Input: map[string]string{"command": "pwd"}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Status != "done" {
		t.Fatalf("status = %s", result.Status)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit = %d stderr = %s", result.ExitCode, result.Stderr)
	}
}

func TestShellToolDeniesCriticalCommand(t *testing.T) {
	shell := NewShellTool(ShellOptions{Mode: policy.ModeOpen, WorkingDirectory: t.TempDir(), OutputLimitBytes: 1024})
	result, err := shell.Execute(context.Background(), ToolRequest{Name: "shell", Input: map[string]string{"command": "rm -rf /"}})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result.Status != "denied" {
		t.Fatalf("status = %s", result.Status)
	}
}

func TestTruncatesOutput(t *testing.T) {
	shell := NewShellTool(ShellOptions{Mode: policy.ModeOpen, WorkingDirectory: t.TempDir(), OutputLimitBytes: 16})
	result, err := shell.Execute(context.Background(), ToolRequest{Name: "shell", Input: map[string]string{"command": "echo 12345678901234567890"}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Stdout, "[output truncated]") {
		t.Fatalf("stdout = %q", result.Stdout)
	}
	if result.StdoutBytesObserved <= 16 {
		t.Fatalf("observed = %d", result.StdoutBytesObserved)
	}
}
```

- [ ] **Step 2: Implement tool interface and router**

Create `internal/tool/tool.go`:

```go
package tool

import (
	"context"
	"fmt"
)

type Tool interface {
	Name() string
	Execute(ctx context.Context, req ToolRequest) (ToolResult, error)
}

type ToolRequest struct {
	Name  string
	Input map[string]string
}

type ToolResult struct {
	Status              string
	Output              string
	Stdout              string
	Stderr              string
	ErrorText           string
	ExitCode            int
	DurationMillis      int64
	StdoutBytesObserved int64
	StderrBytesObserved int64
}

type Router struct {
	tools map[string]Tool
}

func NewRouter(tools ...Tool) *Router {
	router := &Router{tools: map[string]Tool{}}
	for _, item := range tools {
		router.tools[item.Name()] = item
	}
	return router
}

func (r *Router) Execute(ctx context.Context, req ToolRequest) (ToolResult, error) {
	item, ok := r.tools[req.Name]
	if !ok {
		return ToolResult{}, fmt.Errorf("unknown tool %q", req.Name)
	}
	return item.Execute(ctx, req)
}
```

- [ ] **Step 3: Implement shell tool**

Create `internal/tool/shell.go`:

```go
package tool

import (
	"bytes"
	"context"
	"os/exec"
	"runtime"
	"time"

	"linuxbot/internal/policy"
)

type ShellOptions struct {
	Mode               policy.Mode
	WorkingDirectory   string
	AlwaysApproveExact []string
	OutputLimitBytes   int64
}

type ShellTool struct {
	options ShellOptions
}

func NewShellTool(options ShellOptions) *ShellTool {
	if options.OutputLimitBytes <= 0 {
		options.OutputLimitBytes = 4 * 1024 * 1024
	}
	return &ShellTool{options: options}
}

func (s *ShellTool) Name() string {
	return "shell"
}

func (s *ShellTool) Execute(ctx context.Context, req ToolRequest) (ToolResult, error) {
	command := req.Input["command"]
	normalized := policy.Normalize(command)
	decision := policy.Evaluate(policy.EvaluationRequest{
		Mode:               s.options.Mode,
		Command:            normalized,
		AlwaysApproveExact: s.options.AlwaysApproveExact,
	})
	if decision.Action == policy.ActionDeny {
		return ToolResult{Status: "denied", ErrorText: decision.Reason}, nil
	}
	if decision.Action == policy.ActionApproval {
		return ToolResult{Status: "approval_required", ErrorText: decision.Reason}, nil
	}

	start := time.Now()
	cmd := shellCommand(ctx, normalized.Trimmed)
	cmd.Dir = s.options.WorkingDirectory
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := ToolResult{
		Status:              "done",
		Stdout:              truncate(stdout.String(), s.options.OutputLimitBytes),
		Stderr:              truncate(stderr.String(), s.options.OutputLimitBytes),
		StdoutBytesObserved: int64(stdout.Len()),
		StderrBytesObserved: int64(stderr.Len()),
		DurationMillis:      time.Since(start).Milliseconds(),
	}
	if err != nil {
		result.Status = "failed"
		result.ErrorText = err.Error()
		if exit, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exit.ExitCode()
		} else {
			result.ExitCode = 1
		}
	}
	return result, nil
}

func shellCommand(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/C", command)
	}
	return exec.CommandContext(ctx, "sh", "-c", command)
}

func truncate(value string, limit int64) string {
	if int64(len(value)) <= limit {
		return value
	}
	if limit < 32 {
		return value[:limit] + "\n[output truncated]\n"
	}
	headSize := int(limit / 2)
	tailSize := int(limit) - headSize
	return value[:headSize] + "\n[output truncated]\n" + value[len(value)-tailSize:]
}
```

- [ ] **Step 4: Run tool tests**

Run:

```bash
go test ./internal/tool
```

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
git add internal/tool
git commit -m "feat: add shell tool router"
```

---

### Task 7: Agent Protocol, Context Manager, And Run Loop

**Files:**
- Create: `internal/agent/protocol.go`
- Create: `internal/agent/agent.go`
- Create: `internal/agent/agent_test.go`
- Create: `internal/contextmgr/context.go`

- [ ] **Step 1: Write agent test**

Create `internal/agent/agent_test.go`:

```go
package agent

import (
	"context"
	"strings"
	"testing"

	"linuxbot/internal/provider"
	"linuxbot/internal/storage"
	"linuxbot/internal/tool"
)

type staticTool struct{}

func (staticTool) Name() string { return "shell" }

func (staticTool) Execute(ctx context.Context, req tool.ToolRequest) (tool.ToolResult, error) {
	return tool.ToolResult{Status: "done", Stdout: "ok\n"}, nil
}

func TestAgentRecordsPlanCommandAndAnswer(t *testing.T) {
	store, err := storage.Open(t.TempDir() + "/linuxbot.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	session, err := store.EnsureDefaultSession(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("EnsureDefaultSession: %v", err)
	}
	fake := &provider.Fake{Responses: []provider.ChatResponse{
		{Content: `{"plan":"check pwd","actions":[{"tool":"shell","input":{"command":"pwd"}}],"final_answer":""}`},
		{Content: `{"plan":"","actions":[],"final_answer":"done"}`},
	}}
	agent := New(store, fake, tool.NewRouter(staticTool{}))
	answer, err := agent.Run(context.Background(), session, "where am I")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if answer != "done" {
		t.Fatalf("answer = %q", answer)
	}
	steps, err := store.ListRunSteps(context.Background(), 1)
	if err != nil {
		t.Fatalf("ListRunSteps: %v", err)
	}
	var kinds []string
	for _, step := range steps {
		kinds = append(kinds, step.Kind)
	}
	if !strings.Contains(strings.Join(kinds, ","), "command") {
		t.Fatalf("kinds = %#v", kinds)
	}
}
```

- [ ] **Step 2: Implement protocol types**

Create `internal/agent/protocol.go`:

```go
package agent

type ModelAction struct {
	Tool  string            `json:"tool"`
	Input map[string]string `json:"input"`
}

type ModelResponse struct {
	Plan        string        `json:"plan"`
	Actions     []ModelAction `json:"actions"`
	FinalAnswer string        `json:"final_answer"`
}
```

- [ ] **Step 3: Implement context manager**

Create `internal/contextmgr/context.go`:

```go
package contextmgr

import (
	"fmt"

	"linuxbot/internal/provider"
	"linuxbot/internal/storage"
)

func Build(session storage.Session, prompt string) []provider.Message {
	system := fmt.Sprintf("You are LinuxBot. Session=%s Mode=%s WorkingDirectory=%s. Respond only as JSON with fields plan, actions, final_answer.", session.Name, session.Mode, session.WorkingDirectory)
	return []provider.Message{
		{Role: provider.RoleSystem, Content: system},
		{Role: provider.RoleUser, Content: prompt},
	}
}
```

- [ ] **Step 4: Implement agent run loop**

Create `internal/agent/agent.go`:

```go
package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"linuxbot/internal/contextmgr"
	"linuxbot/internal/provider"
	"linuxbot/internal/storage"
	"linuxbot/internal/tool"
)

type Agent struct {
	store    *storage.Store
	provider provider.Provider
	router   *tool.Router
}

func New(store *storage.Store, provider provider.Provider, router *tool.Router) *Agent {
	return &Agent{store: store, provider: provider, router: router}
}

func (a *Agent) Run(ctx context.Context, session storage.Session, prompt string) (string, error) {
	run, err := a.store.CreateRun(ctx, session.ID, prompt)
	if err != nil {
		return "", err
	}
	messages := contextmgr.Build(session, prompt)
	for i := 0; i < 4; i++ {
		response, err := a.provider.Chat(ctx, provider.ChatRequest{Messages: messages})
		if err != nil {
			return "", err
		}
		var model ModelResponse
		if err := json.Unmarshal([]byte(response.Content), &model); err != nil {
			return "", fmt.Errorf("parse model response: %w", err)
		}
		if model.Plan != "" {
			if err := a.store.AddStep(ctx, storage.Step{RunID: run.ID, Kind: "plan", Status: "done", Output: model.Plan}); err != nil {
				return "", err
			}
		}
		if len(model.Actions) == 0 {
			if err := a.store.AddStep(ctx, storage.Step{RunID: run.ID, Kind: "answer", Status: "done", Output: model.FinalAnswer}); err != nil {
				return "", err
			}
			return model.FinalAnswer, nil
		}
		var observation string
		for _, action := range model.Actions {
			result, err := a.router.Execute(ctx, tool.ToolRequest{Name: action.Tool, Input: action.Input})
			if err != nil {
				return "", err
			}
			if err := a.store.AddStep(ctx, storage.Step{
				RunID: run.ID, Kind: action.Tool, Status: result.Status, Input: action.Input["command"],
				Output: result.Stdout, ErrorText: result.ErrorText, ExitCode: result.ExitCode,
				DurationMillis: result.DurationMillis, StdoutBytesObserved: result.StdoutBytesObserved, StderrBytesObserved: result.StderrBytesObserved,
			}); err != nil {
				return "", err
			}
			observation += fmt.Sprintf("tool=%s status=%s stdout=%s stderr=%s\n", action.Tool, result.Status, result.Stdout, result.Stderr)
		}
		messages = append(messages, provider.Message{Role: provider.RoleAssistant, Content: response.Content})
		messages = append(messages, provider.Message{Role: provider.RoleUser, Content: "Tool observations:\n" + observation})
	}
	return "", fmt.Errorf("agent exceeded action loop limit")
}
```

- [ ] **Step 5: Run agent tests**

Run:

```bash
go test ./internal/agent ./internal/contextmgr
```

Expected: PASS.

- [ ] **Step 6: Commit**

Run:

```bash
git add internal/agent internal/contextmgr
git commit -m "feat: add agent run loop"
```

---

### Task 8: Interactive CLI With Approval-Oriented Output

**Files:**
- Create: `internal/cli/cli.go`
- Modify: `internal/app/app.go`
- Create: `internal/cli/cli_test.go`

- [ ] **Step 1: Write CLI smoke test**

Create `internal/cli/cli_test.go`:

```go
package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestLoopExitsOnQuit(t *testing.T) {
	in := strings.NewReader("/quit\n")
	var out bytes.Buffer
	err := Run(context.Background(), Options{SessionName: "default", Mode: "safe"}, in, &out)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out.String(), "linuxbot [default/safe]>") {
		t.Fatalf("output = %q", out.String())
	}
}
```

- [ ] **Step 2: Implement CLI loop**

Create `internal/cli/cli.go`:

```go
package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
)

type Options struct {
	SessionName string
	Mode        string
	Ask         func(ctx context.Context, prompt string) (string, error)
}

func Run(ctx context.Context, options Options, stdin io.Reader, stdout io.Writer) error {
	scanner := bufio.NewScanner(stdin)
	for {
		if _, err := fmt.Fprintf(stdout, "linuxbot [%s/%s]> ", options.SessionName, options.Mode); err != nil {
			return err
		}
		if !scanner.Scan() {
			return scanner.Err()
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "/quit" || line == "/exit" {
			return nil
		}
		if line == "" {
			continue
		}
		if options.Ask == nil {
			if _, err := fmt.Fprintln(stdout, "agent is not configured"); err != nil {
				return err
			}
			continue
		}
		answer, err := options.Ask(ctx, line)
		if err != nil {
			if _, writeErr := fmt.Fprintf(stdout, "error: %v\n", err); writeErr != nil {
				return writeErr
			}
			continue
		}
		if _, err := fmt.Fprintln(stdout, answer); err != nil {
			return err
		}
	}
}
```

- [ ] **Step 3: Wire default CLI path**

Modify the no-args branch in `internal/app/app.go`:

```go
if len(args) == 0 {
	current, err := sessionService.Current(ctx, cwd)
	if err != nil {
		return err
	}
	return cli.Run(ctx, cli.Options{SessionName: current.Name, Mode: current.Mode}, stdin, stdout)
}
```

Add import:

```go
"linuxbot/internal/cli"
```

- [ ] **Step 4: Run CLI tests**

Run:

```bash
go test ./internal/cli ./...
printf "/quit\n" | go run ./cmd/linuxbot
```

Expected: tests pass and CLI exits cleanly.

- [ ] **Step 5: Commit**

Run:

```bash
git add internal/cli internal/app
git commit -m "feat: add interactive cli loop"
```

---

### Task 9: Tavily Search Tool

**Files:**
- Create: `internal/tool/search.go`
- Create: `internal/tool/search_test.go`

- [ ] **Step 1: Write Tavily test with local server**

Create `internal/tool/search_test.go`:

```go
package tool

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchToolCallsTavily(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"title":"Doc","url":"https://example.com","content":"Result text"}]}`))
	}))
	defer server.Close()

	search := NewSearchTool(SearchOptions{Enabled: true, APIKey: "key", BaseURL: server.URL, Client: server.Client()})
	result, err := search.Execute(context.Background(), ToolRequest{Name: "search", Input: map[string]string{"query": "nginx logs"}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Output, "Result text") {
		t.Fatalf("output = %q", result.Output)
	}
}
```

- [ ] **Step 2: Implement Tavily tool**

Create `internal/tool/search.go`:

```go
package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type SearchOptions struct {
	Enabled bool
	APIKey  string
	BaseURL string
	Client  *http.Client
}

type SearchTool struct {
	options SearchOptions
}

func NewSearchTool(options SearchOptions) *SearchTool {
	if options.BaseURL == "" {
		options.BaseURL = "https://api.tavily.com"
	}
	if options.Client == nil {
		options.Client = http.DefaultClient
	}
	return &SearchTool{options: options}
}

func (s *SearchTool) Name() string {
	return "search"
}

func (s *SearchTool) Execute(ctx context.Context, req ToolRequest) (ToolResult, error) {
	if !s.options.Enabled || s.options.APIKey == "" {
		return ToolResult{Status: "skipped", ErrorText: "tavily is not configured"}, nil
	}
	body, err := json.Marshal(map[string]string{
		"api_key": s.options.APIKey,
		"query":   req.Input["query"],
	})
	if err != nil {
		return ToolResult{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(s.options.BaseURL, "/")+"/search", bytes.NewReader(body))
	if err != nil {
		return ToolResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := s.options.Client.Do(httpReq)
	if err != nil {
		return ToolResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ToolResult{Status: "failed", ErrorText: fmt.Sprintf("tavily status %d", resp.StatusCode)}, nil
	}
	var decoded struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return ToolResult{}, err
	}
	var builder strings.Builder
	for _, item := range decoded.Results {
		builder.WriteString(item.Title)
		builder.WriteString("\n")
		builder.WriteString(item.URL)
		builder.WriteString("\n")
		builder.WriteString(item.Content)
		builder.WriteString("\n\n")
	}
	return ToolResult{Status: "done", Output: builder.String()}, nil
}
```

- [ ] **Step 3: Run search tests**

Run:

```bash
go test ./internal/tool
```

Expected: PASS.

- [ ] **Step 4: Commit**

Run:

```bash
git add internal/tool/search.go internal/tool/search_test.go
git commit -m "feat: add tavily search tool"
```

---

### Task 10: Local Web Server And Embedded UI

**Files:**
- Create: `internal/web/server.go`
- Create: `internal/web/handlers.go`
- Create: `internal/web/server_test.go`
- Create: `internal/web/static/index.html`
- Create: `internal/web/static/app.js`
- Create: `internal/web/static/style.css`
- Modify: `internal/app/app.go`

- [ ] **Step 1: Write Web server test**

Create `internal/web/server_test.go`:

```go
package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	server := NewServer(Options{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if recorder.Body.String() != `{"ok":true}`+"\n" {
		t.Fatalf("body = %q", recorder.Body.String())
	}
}
```

- [ ] **Step 2: Implement Web server**

Create `internal/web/server.go`:

```go
package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static/*
var staticFiles embed.FS

type Options struct{}

type Server struct {
	mux *http.ServeMux
}

func NewServer(options Options) *Server {
	mux := http.NewServeMux()
	server := &Server{mux: mux}
	server.routes()
	return server
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func staticFS() http.FileSystem {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	return http.FS(sub)
}
```

Create `internal/web/handlers.go`:

```go
package web

import (
	"encoding/json"
	"net/http"
)

func (s *Server) routes() {
	s.mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})
	s.mux.Handle("/", http.FileServer(staticFS()))
}
```

- [ ] **Step 3: Add embedded UI files**

Create `internal/web/static/index.html`:

```html
<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>LinuxBot</title>
  <link rel="stylesheet" href="/style.css">
</head>
<body>
  <main class="shell">
    <header>
      <h1>LinuxBot</h1>
      <select id="session"></select>
      <select id="mode">
        <option>safe</option>
        <option>review</option>
        <option>open</option>
      </select>
    </header>
    <section id="messages" class="messages"></section>
    <form id="ask">
      <textarea id="prompt" rows="3" aria-label="输入服务器管理请求"></textarea>
      <button type="submit">发送</button>
    </form>
  </main>
  <script src="/app.js"></script>
</body>
</html>
```

Create `internal/web/static/app.js`:

```javascript
async function health() {
  const response = await fetch('/api/health');
  const data = await response.json();
  document.querySelector('#messages').textContent = data.ok ? '已连接本机 LinuxBot' : '连接失败';
}

document.querySelector('#ask').addEventListener('submit', event => {
  event.preventDefault();
  const prompt = document.querySelector('#prompt').value.trim();
  if (prompt) {
    document.querySelector('#messages').textContent = '请求已输入: ' + prompt;
  }
});

health();
```

Create `internal/web/static/style.css`:

```css
body {
  margin: 0;
  font-family: system-ui, sans-serif;
  background: #f7f7f5;
  color: #171717;
}

.shell {
  max-width: 960px;
  margin: 0 auto;
  padding: 24px;
}

header {
  display: flex;
  align-items: center;
  gap: 12px;
}

h1 {
  font-size: 20px;
  margin-right: auto;
}

.messages {
  min-height: 360px;
  border: 1px solid #d8d8d3;
  background: #fff;
  padding: 16px;
  margin: 16px 0;
}

textarea {
  width: 100%;
  box-sizing: border-box;
  resize: vertical;
}

button {
  margin-top: 8px;
}
```

- [ ] **Step 4: Wire `linuxbot web`**

Modify the `web` case in `internal/app/app.go`:

```go
case "web":
	server := web.NewServer(web.Options{})
	_, _ = fmt.Fprintln(stdout, "LinuxBot Web listening on http://127.0.0.1:8787")
	return http.ListenAndServe("127.0.0.1:8787", server.Handler())
```

Add imports:

```go
"net/http"
"linuxbot/internal/web"
```

- [ ] **Step 5: Run Web tests**

Run:

```bash
go test ./internal/web ./...
```

Expected: PASS.

- [ ] **Step 6: Commit**

Run:

```bash
git add internal/web internal/app
git commit -m "feat: add local web ui"
```

---

### Task 11: Web Chat API And Collapsed Run Details

**Files:**
- Modify: `internal/web/server.go`
- Modify: `internal/web/handlers.go`
- Modify: `internal/web/static/app.js`
- Modify: `internal/web/static/index.html`
- Create: `internal/web/handlers_test.go`

- [ ] **Step 1: Write Web handlers test**

Create `internal/web/handlers_test.go`:

```go
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
```

- [ ] **Step 2: Extend Web server options**

Modify `internal/web/server.go`:

```go
package web

import (
	"context"
	"embed"
	"io/fs"
	"net/http"

	"linuxbot/internal/storage"
)

//go:embed static/*
var staticFiles embed.FS

type AskFunc func(ctx context.Context, session storage.Session, prompt string) (Answer, error)

type Answer struct {
	Text           string
	RunID          int64
	StepCount      int
	DurationMillis int64
}

type Options struct {
	Store *storage.Store
	Ask   AskFunc
}

type Server struct {
	mux     *http.ServeMux
	options Options
}

func NewServer(options Options) *Server {
	mux := http.NewServeMux()
	server := &Server{mux: mux, options: options}
	server.routes()
	return server
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func staticFS() http.FileSystem {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	return http.FS(sub)
}
```

- [ ] **Step 3: Implement sessions, ask, run detail handlers**

Replace `internal/web/handlers.go` with:

```go
package web

import (
	"encoding/json"
	"net/http"
	"strconv"

	"linuxbot/internal/storage"
)

func (s *Server) routes() {
	s.mux.HandleFunc("/api/health", s.handleHealth)
	s.mux.HandleFunc("/api/sessions", s.handleSessions)
	s.mux.HandleFunc("/api/ask", s.handleAsk)
	s.mux.HandleFunc("/api/runs/", s.handleRun)
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

func (s *Server) handleAsk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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
	writeJSON(w, map[string]any{
		"answer": answer.Text,
		"run_id": answer.RunID,
		"summary": "已处理 " + strconv.Itoa(answer.StepCount) + " 个步骤 · " + strconv.FormatInt(answer.DurationMillis, 10) + "ms",
	})
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	idText := r.URL.Path[len("/api/runs/"):]
	runID, err := strconv.ParseInt(idText, 10, 64)
	if err != nil {
		http.Error(w, "invalid run id", http.StatusBadRequest)
		return
	}
	steps, err := s.options.Store.ListRunSteps(r.Context(), runID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, steps)
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
```

- [ ] **Step 4: Update Web UI to show collapsed run details**

Replace `internal/web/static/app.js` with:

```javascript
const state = { sessionId: null };

async function loadSessions() {
  const response = await fetch('/api/sessions');
  const sessions = await response.json();
  const select = document.querySelector('#session');
  select.innerHTML = '';
  for (const session of sessions) {
    const option = document.createElement('option');
    option.value = session.ID;
    option.textContent = `${session.Name} / ${session.Mode}`;
    select.appendChild(option);
  }
  if (sessions.length > 0) {
    state.sessionId = sessions[0].ID;
  }
}

function appendMessage(answer, summary, runId) {
  const container = document.querySelector('#messages');
  const article = document.createElement('article');
  const text = document.createElement('p');
  text.textContent = answer;
  const details = document.createElement('details');
  const summaryNode = document.createElement('summary');
  summaryNode.textContent = summary;
  const pre = document.createElement('pre');
  details.appendChild(summaryNode);
  details.appendChild(pre);
  details.addEventListener('toggle', async () => {
    if (details.open && pre.textContent === '') {
      const response = await fetch(`/api/runs/${runId}`);
      pre.textContent = JSON.stringify(await response.json(), null, 2);
    }
  });
  article.appendChild(text);
  article.appendChild(details);
  container.appendChild(article);
}

document.querySelector('#session').addEventListener('change', event => {
  state.sessionId = Number(event.target.value);
});

document.querySelector('#ask').addEventListener('submit', async event => {
  event.preventDefault();
  const promptNode = document.querySelector('#prompt');
  const prompt = promptNode.value.trim();
  if (!prompt || !state.sessionId) {
    return;
  }
  const response = await fetch('/api/ask', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({session_id: state.sessionId, prompt})
  });
  const data = await response.json();
  appendMessage(data.answer, data.summary, data.run_id);
  promptNode.value = '';
});

loadSessions();
```

- [ ] **Step 5: Run Web API tests**

Run:

```bash
go test ./internal/web ./...
```

Expected: PASS.

- [ ] **Step 6: Commit**

Run:

```bash
git add internal/web
git commit -m "feat: add web chat api"
```

---

### Task 12: Deletion, Summary Invalidation, And Final Integration

**Files:**
- Modify: `internal/storage/store.go`
- Modify: `internal/app/app.go`
- Create: `internal/storage/delete_test.go`
- Create: `internal/app/app_test.go`

- [ ] **Step 1: Write deletion test**

Create `internal/storage/delete_test.go`:

```go
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
}
```

- [ ] **Step 2: Implement summary and deletion queries**

Append to `internal/storage/store.go`:

```go
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
	if _, err := tx.ExecContext(ctx, `DELETE FROM steps WHERE run_id = ?`, runID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM approvals WHERE run_id = ?`, runID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM runs WHERE id = ? AND session_id = ?`, runID, sessionID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM session_summaries WHERE session_id = ?`, sessionID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO deletion_events(session_id, deleted_ref, source) VALUES(?, ?, ?)`, sessionID, "run", source); err != nil {
		return err
	}
	return tx.Commit()
}
```

- [ ] **Step 3: Add app smoke test**

Create `internal/app/app_test.go`:

```go
package app

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestUnknownCommandReturnsError(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	err := Run(context.Background(), []string{"unknown"}, strings.NewReader(""), &out, &errOut)
	if err == nil {
		t.Fatalf("expected error")
	}
}
```

- [ ] **Step 4: Run full test suite**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 5: Manual acceptance check**

Run:

```bash
go run ./cmd/linuxbot sessions
printf "/quit\n" | go run ./cmd/linuxbot
```

Expected: sessions list prints at least `default`, and interactive CLI exits on `/quit`.

- [ ] **Step 6: Commit**

Run:

```bash
git add internal
git commit -m "feat: add deletion and final integration"
```

---

### Task 13: Provider Config And Application Wiring

**Files:**
- Modify: `internal/storage/models.go`
- Modify: `internal/storage/store.go`
- Modify: `internal/app/app.go`
- Create: `internal/app/config_test.go`

- [ ] **Step 1: Write config command test**

Create `internal/app/config_test.go`:

```go
package app

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestConfigProviderCommandRequiresArguments(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	err := Run(context.Background(), []string{"config", "provider"}, strings.NewReader(""), &out, &errOut)
	if err == nil {
		t.Fatalf("expected usage error")
	}
}
```

- [ ] **Step 2: Add config models**

Append to `internal/storage/models.go`:

```go
type ProviderConfig struct {
	BaseURL string
	Model   string
	APIKey  string
}

type SearchConfig struct {
	Enabled      bool
	TavilyAPIKey string
}
```

- [ ] **Step 3: Add config store methods**

Append to `internal/storage/store.go`:

```go
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
```

- [ ] **Step 4: Wire config CLI commands**

Replace the `config` branch in `internal/app/app.go`:

```go
case "config":
	if len(args) < 2 {
		return fmt.Errorf("usage: linuxbot config provider|search")
	}
	switch args[1] {
	case "provider":
		if len(args) != 5 {
			return fmt.Errorf("usage: linuxbot config provider <base-url> <model> <api-key>")
		}
		return store.SetProviderConfig(ctx, storage.ProviderConfig{BaseURL: args[2], Model: args[3], APIKey: args[4]})
	case "search":
		if len(args) != 4 {
			return fmt.Errorf("usage: linuxbot config search <on|off> <tavily-api-key>")
		}
		return store.SetSearchConfig(ctx, storage.SearchConfig{Enabled: args[2] == "on", TavilyAPIKey: args[3]})
	default:
		return fmt.Errorf("unknown config section %q", args[1])
	}
```

- [ ] **Step 5: Run config tests**

Run:

```bash
go test ./internal/app ./...
```

Expected: PASS.

- [ ] **Step 6: Commit**

Run:

```bash
git add internal/app internal/storage
git commit -m "feat: add provider config"
```

---

### Task 14: Approval Persistence And CLI Agent Wiring

**Files:**
- Modify: `internal/tool/tool.go`
- Modify: `internal/tool/shell.go`
- Modify: `internal/storage/store.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/app/app.go`
- Create: `internal/tool/approval_test.go`

- [ ] **Step 1: Write approval callback test**

Create `internal/tool/approval_test.go`:

```go
package tool

import (
	"context"
	"testing"

	"linuxbot/internal/policy"
)

func TestShellToolUsesApprovalCallback(t *testing.T) {
	called := false
	shell := NewShellTool(ShellOptions{
		Mode: policy.ModeReview,
		WorkingDirectory: t.TempDir(),
		OutputLimitBytes: 1024,
		Approve: func(ctx context.Context, req ApprovalRequest) (ApprovalDecision, error) {
			called = true
			return ApprovalApprove, nil
		},
	})
	result, err := shell.Execute(context.Background(), ToolRequest{Name: "shell", Input: map[string]string{"command": "echo ok"}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !called {
		t.Fatalf("approval callback was not called")
	}
	if result.Status != "done" {
		t.Fatalf("status = %s", result.Status)
	}
}
```

- [ ] **Step 2: Add approval types**

Append to `internal/tool/tool.go`:

```go
type ApprovalDecision string

const (
	ApprovalApprove ApprovalDecision = "approve"
	ApprovalReject  ApprovalDecision = "reject"
	ApprovalAlways  ApprovalDecision = "always"
)

type ApprovalRequest struct {
	Command string
	Reason  string
}

type ApprovalFunc func(ctx context.Context, req ApprovalRequest) (ApprovalDecision, error)
```

Also extend `ToolResult` in `internal/tool/tool.go`:

```go
type ToolResult struct {
	Status              string
	Output              string
	Stdout              string
	Stderr              string
	ErrorText           string
	ExitCode            int
	DurationMillis      int64
	StdoutBytesObserved int64
	StderrBytesObserved int64
	Command             string
	ApprovalDecision    ApprovalDecision
}
```

- [ ] **Step 3: Update shell tool approval branch**

Modify `ShellOptions` in `internal/tool/shell.go`:

```go
type ShellOptions struct {
	Mode               policy.Mode
	WorkingDirectory   string
	AlwaysApproveExact []string
	OutputLimitBytes   int64
	Approve            ApprovalFunc
}
```

Replace the approval branch in `Execute`:

```go
if decision.Action == policy.ActionApproval {
	if s.options.Approve == nil {
		return ToolResult{Status: "approval_required", ErrorText: decision.Reason}, nil
	}
	approval, err := s.options.Approve(ctx, ApprovalRequest{Command: normalized.Trimmed, Reason: decision.Reason})
	if err != nil {
		return ToolResult{}, err
	}
	if approval == ApprovalReject {
		return ToolResult{Status: "rejected", ErrorText: "command rejected"}, nil
	}
}
```

In the final `ToolResult` construction inside `Execute`, set the normalized command and approval decision:

```go
result := ToolResult{
	Status:              "done",
	Command:             normalized.Trimmed,
	ApprovalDecision:    approval,
	Stdout:              truncate(stdout.String(), s.options.OutputLimitBytes),
	Stderr:              truncate(stderr.String(), s.options.OutputLimitBytes),
	StdoutBytesObserved: int64(stdout.Len()),
	StderrBytesObserved: int64(stderr.Len()),
	DurationMillis:      time.Since(start).Milliseconds(),
}
```

Declare the approval variable before the approval branch:

```go
approval := ApprovalDecision("")
```

- [ ] **Step 4: Add approval storage methods**

Append to `internal/storage/store.go`:

```go
func (s *Store) AddApproval(ctx context.Context, sessionID int64, runID int64, command string, decision string, source string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO approvals(session_id, run_id, command, decision, source) VALUES(?, ?, ?, ?, ?)`, sessionID, runID, command, decision, source)
	return err
}

func (s *Store) AddAlwaysApproveRule(ctx context.Context, sessionID int64, command string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO always_approve_rules(session_id, match_type, pattern) VALUES(?, 'exact', ?)`, sessionID, strings.TrimSpace(command))
	return err
}

func (s *Store) AlwaysApproveRules(ctx context.Context, sessionID int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT pattern FROM always_approve_rules WHERE session_id = ? AND match_type = 'exact'`, sessionID)
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
```

Add import if missing:

```go
"strings"
```

- [ ] **Step 5: Add CLI approval prompt**

Append to `internal/cli/cli.go`:

```go
func PromptApproval(stdin io.Reader, stdout io.Writer) tool.ApprovalFunc {
	reader := bufio.NewReader(stdin)
	return func(ctx context.Context, req tool.ApprovalRequest) (tool.ApprovalDecision, error) {
		if _, err := fmt.Fprintf(stdout, "Command requires approval: %s\nReason: %s\nApprove? yes/no/always: ", req.Command, req.Reason); err != nil {
			return tool.ApprovalReject, err
		}
		line, err := reader.ReadString('\n')
		if err != nil {
			return tool.ApprovalReject, err
		}
		switch strings.TrimSpace(line) {
		case "yes", "y":
			return tool.ApprovalApprove, nil
		case "always":
			return tool.ApprovalAlways, nil
		default:
			return tool.ApprovalReject, nil
		}
	}
}
```

Add import:

```go
"linuxbot/internal/tool"
```

- [ ] **Step 6: Wire app agent path**

Before app wiring, update `internal/agent/agent.go` after each tool execution so approval decisions are persisted:

```go
if result.ApprovalDecision != "" {
	if err := a.store.AddApproval(ctx, session.ID, run.ID, result.Command, string(result.ApprovalDecision), "cli"); err != nil {
		return "", err
	}
	if result.ApprovalDecision == tool.ApprovalAlways {
		if err := a.store.AddAlwaysApproveRule(ctx, session.ID, result.Command); err != nil {
			return "", err
		}
	}
}
```

- [ ] **Step 6: Wire app agent path**

Add this helper to `internal/app/app.go`:

```go
func buildAgent(ctx context.Context, store *storage.Store, current storage.Session, approve tool.ApprovalFunc) (*agent.Agent, error) {
	providerConfig, err := store.ProviderConfig(ctx)
	if err != nil {
		return nil, err
	}
	if providerConfig.BaseURL == "" || providerConfig.Model == "" || providerConfig.APIKey == "" {
		return nil, fmt.Errorf("provider is not configured; run linuxbot config provider <base-url> <model> <api-key>")
	}
	searchConfig, err := store.SearchConfig(ctx)
	if err != nil {
		return nil, err
	}
	rules, err := store.AlwaysApproveRules(ctx, current.ID)
	if err != nil {
		return nil, err
	}
	llm := provider.NewOpenAICompatible(providerConfig.BaseURL, providerConfig.Model, providerConfig.APIKey, nil)
	shell := tool.NewShellTool(tool.ShellOptions{
		Mode: policy.Mode(current.Mode),
		WorkingDirectory: current.WorkingDirectory,
		AlwaysApproveExact: rules,
		OutputLimitBytes: 4 * 1024 * 1024,
		Approve: approve,
	})
	search := tool.NewSearchTool(tool.SearchOptions{Enabled: searchConfig.Enabled, APIKey: searchConfig.TavilyAPIKey})
	return agent.New(store, llm, tool.NewRouter(shell, search)), nil
}
```

Add imports:

```go
"linuxbot/internal/agent"
"linuxbot/internal/policy"
"linuxbot/internal/provider"
"linuxbot/internal/tool"
```

Replace the no-args CLI branch:

```go
if len(args) == 0 {
	current, err := sessionService.Current(ctx, cwd)
	if err != nil {
		return err
	}
	bot, err := buildAgent(ctx, store, current, cli.PromptApproval(stdin, stdout))
	if err != nil {
		return err
	}
	return cli.Run(ctx, cli.Options{
		SessionName: current.Name,
		Mode: current.Mode,
		Ask: func(ctx context.Context, prompt string) (string, error) {
			return bot.Run(ctx, current, prompt)
		},
	}, stdin, stdout)
}
```

- [ ] **Step 7: Run approval and app tests**

Run:

```bash
go test ./internal/tool ./internal/app ./...
```

Expected: PASS.

- [ ] **Step 8: Commit**

Run:

```bash
git add internal
git commit -m "feat: wire cli agent approvals"
```

---

### Task 15: Wire Web To Agent Services

**Files:**
- Modify: `internal/app/app.go`
- Modify: `internal/storage/store.go`
- Modify: `internal/web/handlers.go`
- Modify: `internal/web/handlers_test.go`

- [ ] **Step 1: Add latest run helper**

Append to `internal/storage/store.go`:

```go
func (s *Store) LatestRun(ctx context.Context, sessionID int64) (Run, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, session_id, prompt, status, created_at, updated_at FROM runs WHERE session_id = ? ORDER BY id DESC LIMIT 1`, sessionID)
	var run Run
	var created, updated string
	if err := row.Scan(&run.ID, &run.SessionID, &run.Prompt, &run.Status, &created, &updated); err != nil {
		return Run{}, err
	}
	run.CreatedAt = parseTime(created)
	run.UpdatedAt = parseTime(updated)
	return run, nil
}
```

- [ ] **Step 2: Wire `linuxbot web` with store and agent callback**

Replace the `web` branch in `internal/app/app.go`:

```go
case "web":
	current, err := sessionService.Current(ctx, cwd)
	if err != nil {
		return err
	}
	bot, err := buildAgent(ctx, store, current, nil)
	if err != nil {
		return err
	}
	server := web.NewServer(web.Options{
		Store: store,
		Ask: func(ctx context.Context, session storage.Session, prompt string) (web.Answer, error) {
			start := time.Now()
			text, err := bot.Run(ctx, session, prompt)
			if err != nil {
				return web.Answer{}, err
			}
			run, err := store.LatestRun(ctx, session.ID)
			if err != nil {
				return web.Answer{}, err
			}
			steps, err := store.ListRunSteps(ctx, run.ID)
			if err != nil {
				return web.Answer{}, err
			}
			return web.Answer{Text: text, RunID: run.ID, StepCount: len(steps), DurationMillis: time.Since(start).Milliseconds()}, nil
		},
	})
	_, _ = fmt.Fprintln(stdout, "LinuxBot Web listening on http://127.0.0.1:8787")
	return http.ListenAndServe("127.0.0.1:8787", server.Handler())
```

Add import:

```go
"time"
```

- [ ] **Step 3: Run Web wiring tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 4: Commit**

Run:

```bash
git add internal
git commit -m "feat: wire web agent"
```

---

### Task 16: Web Pending Approval Execution

**Files:**
- Modify: `internal/storage/store.go`
- Modify: `internal/web/handlers.go`
- Modify: `internal/web/static/app.js`
- Modify: `internal/app/app.go`
- Create: `internal/web/approval_test.go`

- [ ] **Step 1: Write Web approval test**

Create `internal/web/approval_test.go`:

```go
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
	if err := store.AddStep(context.Background(), storage.Step{RunID: run.ID, Kind: "shell", Status: "approval_required", Input: "echo ok"}); err != nil {
		t.Fatalf("AddStep: %v", err)
	}
	server := NewServer(Options{Store: store, ApproveCommand: func(ctx context.Context, session storage.Session, step storage.Step) error {
		return store.AddStep(ctx, storage.Step{RunID: step.RunID, Kind: "shell", Status: "done", Input: step.Input, Output: "ok\n"})
	}})
	body := strings.NewReader(`{"session_id":` + strconv.FormatInt(session.ID, 10) + `,"step_id":1}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/approve", body)
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
}
```

- [ ] **Step 2: Add step lookup**

Append to `internal/storage/store.go`:

```go
func (s *Store) GetStep(ctx context.Context, stepID int64) (Step, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, run_id, kind, status, input, output, error_text, exit_code, duration_millis, stdout_bytes_observed, stderr_bytes_observed, created_at FROM steps WHERE id = ?`, stepID)
	var step Step
	var created string
	if err := row.Scan(&step.ID, &step.RunID, &step.Kind, &step.Status, &step.Input, &step.Output, &step.ErrorText, &step.ExitCode, &step.DurationMillis, &step.StdoutBytesObserved, &step.StderrBytesObserved, &created); err != nil {
		return Step{}, err
	}
	step.CreatedAt = parseTime(created)
	return step, nil
}
```

- [ ] **Step 3: Extend Web options and route**

Modify `internal/web/server.go`:

```go
type ApproveCommandFunc func(ctx context.Context, session storage.Session, step storage.Step) error

type Options struct {
	Store          *storage.Store
	Ask            AskFunc
	ApproveCommand ApproveCommandFunc
}
```

Add route in `internal/web/handlers.go`:

```go
s.mux.HandleFunc("/api/approve", s.handleApprove)
```

Add handler:

```go
func (s *Server) handleApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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
	if step.Status != "approval_required" {
		http.Error(w, "step is not pending approval", http.StatusConflict)
		return
	}
	if err := s.options.ApproveCommand(r.Context(), session, step); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}
```

- [ ] **Step 4: Wire Web approval execution in app**

In the `web.NewServer` call in `internal/app/app.go`, add:

```go
ApproveCommand: func(ctx context.Context, session storage.Session, step storage.Step) error {
	shell := tool.NewShellTool(tool.ShellOptions{
		Mode: policy.ModeReview,
		WorkingDirectory: session.WorkingDirectory,
		OutputLimitBytes: 4 * 1024 * 1024,
		Approve: func(ctx context.Context, req tool.ApprovalRequest) (tool.ApprovalDecision, error) {
			return tool.ApprovalApprove, nil
		},
	})
	result, err := shell.Execute(ctx, tool.ToolRequest{Name: "shell", Input: map[string]string{"command": step.Input}})
	if err != nil {
		return err
	}
	return store.AddStep(ctx, storage.Step{
		RunID: step.RunID, Kind: "shell", Status: result.Status, Input: step.Input,
		Output: result.Stdout, ErrorText: result.ErrorText, ExitCode: result.ExitCode,
		DurationMillis: result.DurationMillis, StdoutBytesObserved: result.StdoutBytesObserved, StderrBytesObserved: result.StderrBytesObserved,
	})
},
```

Add import:

```go
"linuxbot/internal/policy"
```

- [ ] **Step 5: Update Web UI to show approval controls**

In `internal/web/static/app.js`, inside the details toggle after loading steps, replace the `pre.textContent` assignment:

```javascript
const steps = await response.json();
pre.textContent = JSON.stringify(steps, null, 2);
for (const step of steps) {
  if (step.Status === 'approval_required') {
    const button = document.createElement('button');
    button.textContent = '批准执行';
    button.addEventListener('click', async () => {
      await fetch('/api/approve', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({session_id: state.sessionId, step_id: step.ID})
      });
      pre.textContent = '';
      details.open = false;
    });
    details.appendChild(button);
  }
}
```

- [ ] **Step 6: Run Web approval tests**

Run:

```bash
go test ./internal/web ./...
```

Expected: PASS.

- [ ] **Step 7: Commit**

Run:

```bash
git add internal
git commit -m "feat: add web approval execution"
```

---

## Final Verification

- [ ] Run full tests:

```bash
go test ./...
```

Expected: PASS.

- [ ] Build single binary:

```bash
go build -o linuxbot ./cmd/linuxbot
```

Expected: `linuxbot` binary exists.

- [ ] Verify default CLI:

```bash
printf "/quit\n" | ./linuxbot
```

Expected: prompt appears and exits.

- [ ] Verify local Web binding:

```bash
./linuxbot web
```

Expected: server prints `http://127.0.0.1:8787`. Stop it after checking `/api/health`.

## Self-Review Checklist

- Spec coverage: sessions, SQLite, provider, context manager, tool router, normalization, safe allowlist, denylist, shell runner, truncation, Tavily, Web, deletion, and CLI are covered by tasks.
- Unresolved marker scan: no task should contain unresolved implementation markers.
- Type consistency: use `provider.Provider`, `tool.Tool`, `storage.Store`, `policy.NormalizedCommand`, and `agent.ModelResponse` consistently.
- Web chat execution is covered by Task 11 and Task 15; Web pending approval execution is covered by Task 16.
