package tool

import (
	"context"
	"errors"
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
	Timeout            time.Duration
	Approve            ApprovalFunc
}

type ShellTool struct {
	options ShellOptions
}

func NewShellTool(options ShellOptions) *ShellTool {
	if options.OutputLimitBytes <= 0 {
		options.OutputLimitBytes = 4 * 1024 * 1024
	}
	if options.Timeout <= 0 {
		options.Timeout = 5 * time.Minute
	}
	return &ShellTool{options: options}
}

func (s *ShellTool) Name() string {
	return "shell"
}

func (s *ShellTool) Execute(ctx context.Context, req ToolRequest) (ToolResult, error) {
	normalized := policy.Normalize(req.Input["command"])
	decision := policy.Evaluate(policy.EvaluationRequest{
		Mode:               s.options.Mode,
		Command:            normalized,
		AlwaysApproveExact: s.options.AlwaysApproveExact,
	})
	if decision.Action == policy.ActionDeny {
		return ToolResult{Status: "denied", ErrorText: decision.Reason, Command: normalized.Trimmed}, nil
	}

	approval := ApprovalDecision("")
	if decision.Action == policy.ActionApproval {
		if s.options.Approve == nil {
			return ToolResult{Status: "approval_required", ErrorText: decision.Reason, Command: normalized.Trimmed}, nil
		}
		var err error
		approval, err = s.options.Approve(ctx, ApprovalRequest{Command: normalized.Trimmed, Reason: decision.Reason})
		if err != nil {
			return ToolResult{}, err
		}
		if approval == ApprovalReject {
			return ToolResult{Status: "rejected", ErrorText: "command rejected", Command: normalized.Trimmed, ApprovalDecision: approval}, nil
		}
	}

	start := time.Now()
	stdout := newLimitedOutput(s.options.OutputLimitBytes)
	stderr := newLimitedOutput(s.options.OutputLimitBytes)
	execCtx, cancel := context.WithTimeout(ctx, s.options.Timeout)
	defer cancel()
	cmd := shellCommand(execCtx, normalized.Trimmed)
	cmd.Dir = s.options.WorkingDirectory
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	result := ToolResult{
		Status:              "done",
		Command:             normalized.Trimmed,
		ApprovalDecision:    approval,
		Stdout:              stdout.String(),
		Stderr:              stderr.String(),
		StdoutBytesObserved: stdout.Observed(),
		StderrBytesObserved: stderr.Observed(),
		DurationMillis:      time.Since(start).Milliseconds(),
	}
	if err != nil {
		result.Status = "failed"
		result.ErrorText = err.Error()
		if errors.Is(execCtx.Err(), context.DeadlineExceeded) {
			result.Status = "timeout"
			result.ErrorText = "command timed out"
		}
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

type limitedOutput struct {
	limit     int64
	observed  int64
	full      []byte
	head      []byte
	tail      []byte
	truncated bool
}

func newLimitedOutput(limit int64) *limitedOutput {
	if limit <= 0 {
		limit = 4 * 1024 * 1024
	}
	return &limitedOutput{limit: limit}
}

func (o *limitedOutput) Write(p []byte) (int, error) {
	o.observed += int64(len(p))
	if !o.truncated {
		o.full = append(o.full, p...)
		if int64(len(o.full)) <= o.limit {
			return len(p), nil
		}
		o.truncated = true
		headLimit, tailLimit := splitLimit(o.limit)
		o.head = append([]byte(nil), o.full[:headLimit]...)
		o.tail = append([]byte(nil), o.full[len(o.full)-tailLimit:]...)
		o.full = nil
		return len(p), nil
	}
	_, tailLimit := splitLimit(o.limit)
	o.tail = append(o.tail, p...)
	if len(o.tail) > tailLimit {
		o.tail = append([]byte(nil), o.tail[len(o.tail)-tailLimit:]...)
	}
	return len(p), nil
}

func (o *limitedOutput) String() string {
	if !o.truncated {
		return string(o.full)
	}
	return string(o.head) + "\n[output truncated]\n" + string(o.tail)
}

func (o *limitedOutput) Observed() int64 {
	return o.observed
}

func splitLimit(limit int64) (int, int) {
	if limit < 2 {
		return int(limit), 0
	}
	head := int(limit / 2)
	tail := int(limit) - head
	return head, tail
}
