package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"linuxbot/internal/contextmgr"
	"linuxbot/internal/provider"
	"linuxbot/internal/storage"
	"linuxbot/internal/tool"
)

type Options struct {
	ApprovalSource string
	EventWriter    io.Writer
}

type Agent struct {
	store    *storage.Store
	provider provider.Provider
	router   *tool.Router
	options  Options
}

type RunResult struct {
	Text  string
	RunID int64
}

func New(store *storage.Store, provider provider.Provider, router *tool.Router, options Options) *Agent {
	if options.ApprovalSource == "" {
		options.ApprovalSource = "cli"
	}
	return &Agent{store: store, provider: provider, router: router, options: options}
}

func (a *Agent) Run(ctx context.Context, session storage.Session, prompt string) (string, error) {
	result, err := a.RunResult(ctx, session, prompt)
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

func (a *Agent) RunResult(ctx context.Context, session storage.Session, prompt string) (RunResult, error) {
	messages, err := contextmgr.Build(ctx, a.store, session, prompt)
	if err != nil {
		return RunResult{}, err
	}
	run, err := a.store.CreateRun(ctx, session.ID, prompt)
	if err != nil {
		return RunResult{}, err
	}
	if err := a.store.AddMessage(ctx, session.ID, run.ID, "user", prompt); err != nil {
		return RunResult{RunID: run.ID}, err
	}
	for i := 0; i < 4; i++ {
		response, err := a.provider.Chat(ctx, provider.ChatRequest{Messages: messages})
		if err != nil {
			_ = a.store.UpdateRunStatus(ctx, run.ID, "failed")
			return RunResult{RunID: run.ID}, err
		}
		var model ModelResponse
		if err := json.Unmarshal([]byte(response.Content), &model); err != nil {
			_ = a.store.UpdateRunStatus(ctx, run.ID, "failed")
			return RunResult{RunID: run.ID}, fmt.Errorf("parse model response: %w", err)
		}
		if model.Plan != "" {
			if err := a.store.AddStep(ctx, storage.Step{RunID: run.ID, Kind: "plan", Status: "done", Output: model.Plan}); err != nil {
				return RunResult{RunID: run.ID}, err
			}
			a.emit("AI plan:\n%s\n", model.Plan)
		}
		if len(model.Actions) == 0 {
			answer := strings.TrimSpace(model.FinalAnswer)
			if answer == "" {
				answer = "完成。"
			}
			if err := a.store.AddStep(ctx, storage.Step{RunID: run.ID, Kind: "answer", Status: "done", Output: answer}); err != nil {
				return RunResult{RunID: run.ID}, err
			}
			if err := a.store.AddMessage(ctx, session.ID, run.ID, "assistant", answer); err != nil {
				return RunResult{RunID: run.ID}, err
			}
			if err := a.updateSummary(ctx, session, prompt, answer); err != nil {
				return RunResult{RunID: run.ID}, err
			}
			_ = a.store.UpdateRunStatus(ctx, run.ID, "done")
			return RunResult{Text: answer, RunID: run.ID}, nil
		}

		var observation strings.Builder
		waitingApproval := false
		for _, action := range model.Actions {
			a.emitAction(action)
			result, err := a.router.Execute(ctx, tool.ToolRequest{Name: action.Tool, Input: action.Input})
			if err != nil {
				_ = a.store.UpdateRunStatus(ctx, run.ID, "failed")
				return RunResult{RunID: run.ID}, err
			}
			step := storage.Step{
				RunID:               run.ID,
				Kind:                stepKind(action.Tool),
				Status:              result.Status,
				Input:               stepInput(action),
				Output:              resultOutput(result),
				ErrorText:           result.ErrorText,
				ExitCode:            result.ExitCode,
				DurationMillis:      result.DurationMillis,
				StdoutBytesObserved: result.StdoutBytesObserved,
				StderrBytesObserved: result.StderrBytesObserved,
			}
			if err := a.store.AddStep(ctx, step); err != nil {
				return RunResult{RunID: run.ID}, err
			}
			if result.ApprovalDecision != "" {
				if err := a.store.AddApproval(ctx, session.ID, run.ID, result.Command, string(result.ApprovalDecision), a.options.ApprovalSource); err != nil {
					return RunResult{RunID: run.ID}, err
				}
				if result.ApprovalDecision == tool.ApprovalAlways {
					if err := a.store.AddAlwaysApproveRule(ctx, session.ID, result.Command); err != nil {
						return RunResult{RunID: run.ID}, err
					}
				}
			}
			if result.Status == "approval_required" {
				waitingApproval = true
			}
			a.emitResult(action, result)
			observation.WriteString(fmt.Sprintf("tool=%s status=%s output=%s stdout=%s stderr=%s error=%s\n", action.Tool, result.Status, result.Output, result.Stdout, result.Stderr, result.ErrorText))
		}
		if waitingApproval {
			answer := "有命令需要批准后才能继续执行。"
			if err := a.store.AddStep(ctx, storage.Step{RunID: run.ID, Kind: "answer", Status: "waiting_approval", Output: answer}); err != nil {
				return RunResult{RunID: run.ID}, err
			}
			if err := a.store.AddMessage(ctx, session.ID, run.ID, "assistant", answer); err != nil {
				return RunResult{RunID: run.ID}, err
			}
			if err := a.updateSummary(ctx, session, prompt, answer); err != nil {
				return RunResult{RunID: run.ID}, err
			}
			_ = a.store.UpdateRunStatus(ctx, run.ID, "waiting_approval")
			return RunResult{Text: answer, RunID: run.ID}, nil
		}
		messages = append(messages, provider.Message{Role: provider.RoleAssistant, Content: response.Content})
		messages = append(messages, provider.Message{Role: provider.RoleUser, Content: "Tool observations:\n" + observation.String()})
	}
	_ = a.store.UpdateRunStatus(ctx, run.ID, "failed")
	return RunResult{RunID: run.ID}, fmt.Errorf("agent exceeded action loop limit")
}

func (a *Agent) emit(format string, args ...any) {
	if a.options.EventWriter == nil {
		return
	}
	_, _ = fmt.Fprintf(a.options.EventWriter, format, args...)
}

func (a *Agent) emitAction(action ModelAction) {
	switch action.Tool {
	case "shell":
		a.emit("\nCommand:\n%s\n", action.Input["command"])
	case "search":
		a.emit("\nSearch:\n%s\n", action.Input["query"])
	default:
		a.emit("\nTool %s:\n%v\n", action.Tool, action.Input)
	}
}

func (a *Agent) emitResult(action ModelAction, result tool.ToolResult) {
	switch action.Tool {
	case "shell":
		if result.Stdout != "" {
			a.emit("%s", result.Stdout)
			if !strings.HasSuffix(result.Stdout, "\n") {
				a.emit("\n")
			}
		}
		if result.Stderr != "" {
			a.emit("[stderr]\n%s", result.Stderr)
			if !strings.HasSuffix(result.Stderr, "\n") {
				a.emit("\n")
			}
		}
		if result.Status != "done" {
			a.emit("[%s] %s\n", result.Status, result.ErrorText)
		}
	case "search":
		if result.Output != "" {
			a.emit("%s", result.Output)
		}
		if result.Status != "done" {
			a.emit("[%s] %s\n", result.Status, result.ErrorText)
		}
	default:
		a.emit("[%s] %s\n", result.Status, resultOutput(result))
	}
}

func stepKind(toolName string) string {
	if toolName == "shell" {
		return "command"
	}
	return toolName
}

func stepInput(action ModelAction) string {
	if action.Tool == "search" {
		return action.Input["query"]
	}
	return action.Input["command"]
}

func resultOutput(result tool.ToolResult) string {
	if result.Output != "" {
		return result.Output
	}
	if result.Stdout != "" && result.Stderr != "" {
		return result.Stdout + "\n[stderr]\n" + result.Stderr
	}
	if result.Stdout != "" {
		return result.Stdout
	}
	return result.Stderr
}

func (a *Agent) updateSummary(ctx context.Context, session storage.Session, prompt string, answer string) error {
	current, err := a.store.SessionSummary(ctx, session.ID)
	if err != nil {
		return err
	}
	entry := fmt.Sprintf("User: %s\nAssistant: %s\n", compact(prompt, 300), compact(answer, 300))
	combined := strings.TrimSpace(current + "\n" + entry)
	if len(combined) > 4000 {
		combined = combined[len(combined)-4000:]
	}
	return a.store.SetSessionSummary(ctx, session.ID, strings.TrimSpace(combined))
}

func compact(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}
