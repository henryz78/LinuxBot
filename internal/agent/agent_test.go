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
	return tool.ToolResult{Status: "done", Stdout: "ok\n", Command: req.Input["command"]}, nil
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
	agent := New(store, fake, tool.NewRouter(staticTool{}), Options{})
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
