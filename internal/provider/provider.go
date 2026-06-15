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
