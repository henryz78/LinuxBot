package web

import (
	"context"
	"embed"
	"io/fs"
	"net/http"
	"time"

	"linuxbot/internal/storage"
)

//go:embed static/*
var staticFiles embed.FS

type AskFunc func(ctx context.Context, session storage.Session, prompt string) (Answer, error)

type ApproveCommandFunc func(ctx context.Context, session storage.Session, step storage.Step) error

type Answer struct {
	Text           string
	RunID          int64
	StepCount      int
	DurationMillis int64
}

type RunView struct {
	ID             int64     `json:"id"`
	SessionID      int64     `json:"session_id"`
	Prompt         string    `json:"prompt"`
	Status         string    `json:"status"`
	Answer         string    `json:"answer"`
	Summary        string    `json:"summary"`
	StepCount      int       `json:"step_count"`
	DurationMillis int64     `json:"duration_millis"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Options struct {
	Store          *storage.Store
	Ask            AskFunc
	ApproveCommand ApproveCommandFunc
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
