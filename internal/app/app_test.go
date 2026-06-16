package app

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"linuxbot/internal/storage"
)

func TestRunPrintsCurrentSession(t *testing.T) {
	setTestHome(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run(context.Background(), nil, strings.NewReader("/quit\n"), &stdout, &stderr)

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got, want := stdout.String(), "linuxbot [default/safe]> "; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestConfigProviderCommandRequiresArguments(t *testing.T) {
	setTestHome(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run(context.Background(), []string{"config", "provider"}, strings.NewReader(""), &stdout, &stderr)

	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "usage: linuxbot config provider") {
		t.Fatalf("error = %v", err)
	}
}

func TestConfigCommandPrintsRedactedSettings(t *testing.T) {
	setTestHome(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run(context.Background(), []string{"config", "provider", "http://example.test", "model-a", "secret-key"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("config provider: %v", err)
	}
	stdout.Reset()

	err := Run(context.Background(), []string{"config"}, strings.NewReader(""), &stdout, &stderr)

	if err != nil {
		t.Fatalf("config: %v", err)
	}
	output := stdout.String()
	if !strings.Contains(output, "api_key=***") {
		t.Fatalf("output = %q", output)
	}
	if strings.Contains(output, "secret-key") {
		t.Fatalf("secret leaked in output = %q", output)
	}
}

func TestRunListsDefaultSession(t *testing.T) {
	setTestHome(t)
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err = Run(context.Background(), []string{"sessions"}, nil, &stdout, &stderr)

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got, want := stdout.String(), fmt.Sprintf("default\tsafe\t%s\n", workingDir); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestApproveWebCommandUpdatesPendingStepWithoutDuplicateCommand(t *testing.T) {
	store, err := storage.Open(t.TempDir() + "/linuxbot.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	session, err := store.EnsureDefaultSession(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("EnsureDefaultSession: %v", err)
	}
	run, err := store.CreateRun(context.Background(), session.ID, "say ok")
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := store.AddStep(context.Background(), storage.Step{RunID: run.ID, Kind: "command", Status: "approval_required", Input: "echo ok"}); err != nil {
		t.Fatalf("AddStep command: %v", err)
	}
	if err := store.AddStep(context.Background(), storage.Step{RunID: run.ID, Kind: "answer", Status: "waiting_approval", Output: "waiting"}); err != nil {
		t.Fatalf("AddStep answer: %v", err)
	}
	step, err := store.GetStep(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetStep: %v", err)
	}

	if err := approveWebCommand(context.Background(), store, session, step); err != nil {
		t.Fatalf("approveWebCommand: %v", err)
	}

	steps, err := store.ListRunSteps(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("ListRunSteps: %v", err)
	}
	var commandSteps int
	var answer string
	for _, item := range steps {
		if item.Kind == "command" {
			commandSteps++
			if item.Status != "done" {
				t.Fatalf("command status = %s", item.Status)
			}
		}
		if item.Kind == "answer" {
			answer = item.Output
		}
	}
	if commandSteps != 1 {
		t.Fatalf("command steps = %d", commandSteps)
	}
	if !strings.Contains(answer, "执行完成") {
		t.Fatalf("answer = %q", answer)
	}
}

func TestApproveWebCommandKeepsRunWaitingWhenOtherCommandsNeedApproval(t *testing.T) {
	store, err := storage.Open(t.TempDir() + "/linuxbot.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	session, err := store.EnsureDefaultSession(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("EnsureDefaultSession: %v", err)
	}
	run, err := store.CreateRun(context.Background(), session.ID, "two commands")
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := store.AddStep(context.Background(), storage.Step{RunID: run.ID, Kind: "command", Status: "approval_required", Input: "echo one"}); err != nil {
		t.Fatalf("AddStep first command: %v", err)
	}
	if err := store.AddStep(context.Background(), storage.Step{RunID: run.ID, Kind: "command", Status: "approval_required", Input: "echo two"}); err != nil {
		t.Fatalf("AddStep second command: %v", err)
	}
	if err := store.AddStep(context.Background(), storage.Step{RunID: run.ID, Kind: "answer", Status: "waiting_approval", Output: "waiting"}); err != nil {
		t.Fatalf("AddStep answer: %v", err)
	}
	step, err := store.GetStep(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetStep: %v", err)
	}

	if err := approveWebCommand(context.Background(), store, session, step); err != nil {
		t.Fatalf("approveWebCommand: %v", err)
	}

	storedRun, err := store.GetRun(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if storedRun.Status != "waiting_approval" {
		t.Fatalf("run status = %s", storedRun.Status)
	}
	steps, err := store.ListRunSteps(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("ListRunSteps: %v", err)
	}
	if steps[0].Status != "done" {
		t.Fatalf("first command status = %s", steps[0].Status)
	}
	if steps[1].Status != "approval_required" {
		t.Fatalf("second command status = %s", steps[1].Status)
	}
	if steps[2].Status != "waiting_approval" {
		t.Fatalf("answer status = %s", steps[2].Status)
	}
}

func setTestHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
}
