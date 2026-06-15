package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"linuxbot/internal/tool"
)

type Options struct {
	SessionName string
	Mode        string
	Ask         func(ctx context.Context, prompt string) (string, error)
}

func Run(ctx context.Context, options Options, stdin io.Reader, stdout io.Writer) error {
	reader, ok := stdin.(*bufio.Reader)
	if !ok {
		reader = bufio.NewReader(stdin)
	}
	for {
		if _, err := fmt.Fprintf(stdout, "linuxbot [%s/%s]> ", options.SessionName, options.Mode); err != nil {
			return err
		}
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		line = strings.TrimSpace(line)
		if line == "/quit" || line == "/exit" {
			return nil
		}
		if line == "" {
			continue
		}
		if options.Ask == nil {
			if _, err := fmt.Fprintln(stdout, "agent is not configured"); err != nil {
				return err
			}
			continue
		}
		answer, err := options.Ask(ctx, line)
		if err != nil {
			if _, writeErr := fmt.Fprintf(stdout, "error: %v\n", err); writeErr != nil {
				return writeErr
			}
			continue
		}
		if _, err := fmt.Fprintln(stdout, answer); err != nil {
			return err
		}
	}
}

func PromptApproval(stdin io.Reader, stdout io.Writer) tool.ApprovalFunc {
	reader, ok := stdin.(*bufio.Reader)
	if !ok {
		reader = bufio.NewReader(stdin)
	}
	return func(ctx context.Context, req tool.ApprovalRequest) (tool.ApprovalDecision, error) {
		if _, err := fmt.Fprintf(stdout, "Command requires approval: %s\nReason: %s\nApprove? yes/no/always: ", req.Command, req.Reason); err != nil {
			return tool.ApprovalReject, err
		}
		line, err := reader.ReadString('\n')
		if err != nil {
			return tool.ApprovalReject, err
		}
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "yes", "y":
			return tool.ApprovalApprove, nil
		case "always":
			return tool.ApprovalAlways, nil
		default:
			return tool.ApprovalReject, nil
		}
	}
}
