package app

import (
	"context"
	"fmt"
	"io"
)

func Run(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		_, err := fmt.Fprintln(stdout, "linuxbot bootstrap interactive CLI")
		return err
	}
	switch args[0] {
	case "sessions", "switch", "config", "web":
		_, err := fmt.Fprintf(stdout, "linuxbot %s bootstrap command\n", args[0])
		return err
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}
