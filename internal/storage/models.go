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
	ID                  int64
	RunID               int64
	Kind                string
	Status              string
	Input               string
	Output              string
	ErrorText           string
	ExitCode            int
	DurationMillis      int64
	StdoutBytesObserved int64
	StderrBytesObserved int64
	CreatedAt           time.Time
}

type Message struct {
	ID        int64
	SessionID int64
	RunID     int64
	Role      string
	Content   string
	CreatedAt time.Time
}

type ProviderConfig struct {
	BaseURL string
	Model   string
	APIKey  string
}

type SearchConfig struct {
	Enabled      bool
	TavilyAPIKey string
}
