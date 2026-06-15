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

func (s *Service) Get(ctx context.Context, name string) (storage.Session, error) {
	return s.store.GetSessionByName(ctx, name)
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
