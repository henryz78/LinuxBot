package contextmgr

import (
	"context"
	"fmt"
	"strings"

	"linuxbot/internal/provider"
	"linuxbot/internal/storage"
)

const (
	recentMessageLimit = 20
	recentRunLimit     = 10
)

func Build(ctx context.Context, store *storage.Store, session storage.Session, prompt string) ([]provider.Message, error) {
	system := fmt.Sprintf(`You are LinuxBot, a local Linux server management assistant.
Session: %s
Mode: %s
Working directory: %s

Return only JSON:
{"plan":"short plan","actions":[{"tool":"shell|search","input":{"command":"...","query":"..."}}],"final_answer":"..."}

Use shell for commands and search only when web search is useful.`, session.Name, session.Mode, session.WorkingDirectory)

	var contextText strings.Builder
	if store != nil {
		summary, err := store.SessionSummary(ctx, session.ID)
		if err != nil {
			return nil, err
		}
		if summary != "" {
			contextText.WriteString("Session summary:\n")
			contextText.WriteString(summary)
			contextText.WriteString("\n\n")
		}
		messages, err := store.ListRecentMessages(ctx, session.ID, recentMessageLimit)
		if err != nil {
			return nil, err
		}
		if len(messages) > 0 {
			contextText.WriteString("Recent messages:\n")
			for _, message := range messages {
				contextText.WriteString(message.Role)
				contextText.WriteString(": ")
				contextText.WriteString(message.Content)
				contextText.WriteString("\n")
			}
			contextText.WriteString("\n")
		}
		runs, err := store.ListRecentRuns(ctx, session.ID, recentRunLimit)
		if err != nil {
			return nil, err
		}
		if len(runs) > 0 {
			contextText.WriteString("Recent runs:\n")
			for _, run := range runs {
				contextText.WriteString(fmt.Sprintf("#%d [%s] %s\n", run.ID, run.Status, run.Prompt))
			}
			contextText.WriteString("\n")
		}
	}
	user := prompt
	if contextText.Len() > 0 {
		user = contextText.String() + "User request:\n" + prompt
	}
	return []provider.Message{
		{Role: provider.RoleSystem, Content: system},
		{Role: provider.RoleUser, Content: user},
	}, nil
}
