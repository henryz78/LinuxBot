package app

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRunRoutesBootstrapCommands(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run(context.Background(), []string{"sessions"}, strings.NewReader(""), &stdout, &stderr)

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got, want := stdout.String(), "linuxbot sessions bootstrap command\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}
