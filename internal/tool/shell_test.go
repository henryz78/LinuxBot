package tool

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"linuxbot/internal/policy"
)

func TestShellToolRunsAllowedCommand(t *testing.T) {
	shell := NewShellTool(ShellOptions{Mode: policy.ModeSafe, WorkingDirectory: t.TempDir(), OutputLimitBytes: 1024})
	result, err := shell.Execute(context.Background(), ToolRequest{Name: "shell", Input: map[string]string{"command": "whoami"}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Status != "done" {
		t.Fatalf("status = %s error = %s", result.Status, result.ErrorText)
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

func TestShellToolUsesApprovalCallback(t *testing.T) {
	called := false
	shell := NewShellTool(ShellOptions{
		Mode:             policy.ModeReview,
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

func TestShellToolTimesOut(t *testing.T) {
	shell := NewShellTool(ShellOptions{
		Mode:             policy.ModeOpen,
		WorkingDirectory: t.TempDir(),
		OutputLimitBytes: 1024,
		Timeout:          10 * time.Millisecond,
	})
	result, err := shell.Execute(context.Background(), ToolRequest{Name: "shell", Input: map[string]string{"command": sleepCommand()}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Status != "timeout" {
		t.Fatalf("status = %s error = %s", result.Status, result.ErrorText)
	}
}

func sleepCommand() string {
	if runtime.GOOS != "windows" {
		return "sleep 1"
	}
	return "ping -n 3 127.0.0.1 >NUL"
}
