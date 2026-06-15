package app

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
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

func setTestHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
}
