package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestLoopExitsOnQuit(t *testing.T) {
	in := strings.NewReader("/quit\n")
	var out bytes.Buffer
	err := Run(context.Background(), Options{SessionName: "default", Mode: "safe"}, in, &out)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out.String(), "linuxbot [default/safe]>") {
		t.Fatalf("output = %q", out.String())
	}
}
