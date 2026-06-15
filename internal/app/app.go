package app

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"linuxbot/internal/agent"
	"linuxbot/internal/cli"
	"linuxbot/internal/policy"
	"linuxbot/internal/provider"
	"linuxbot/internal/session"
	"linuxbot/internal/storage"
	"linuxbot/internal/tool"
	"linuxbot/internal/web"
)

func Run(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dataDir := filepath.Join(home, ".linuxbot")
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return err
	}
	if err := os.Chmod(dataDir, 0700); err != nil {
		return err
	}
	dbPath := filepath.Join(dataDir, "linuxbot.db")
	store, err := storage.Open(dbPath)
	if err != nil {
		return err
	}
	if err := os.Chmod(dbPath, 0600); err != nil {
		return err
	}
	defer store.Close()

	workingDir, err := os.Getwd()
	if err != nil {
		return err
	}
	service := session.NewService(store)
	args, explicitSession, err := parseSessionArg(args)
	if err != nil {
		return err
	}

	if len(args) == 0 {
		current, err := currentSession(ctx, service, explicitSession, workingDir)
		if err != nil {
			return err
		}
		reader := bufio.NewReader(stdin)
		return cli.Run(ctx, cli.Options{
			SessionName: current.Name,
			Mode:        current.Mode,
			Ask: func(ctx context.Context, prompt string) (string, error) {
				fresh, err := store.GetSession(ctx, current.ID)
				if err != nil {
					return "", err
				}
				bot, err := buildAgent(ctx, store, fresh, cli.PromptApproval(reader, stdout), "cli", stdout)
				if err != nil {
					return "", err
				}
				return bot.Run(ctx, fresh, prompt)
			},
		}, reader, stdout)
	}
	switch args[0] {
	case "sessions":
		if _, err := service.Current(ctx, workingDir); err != nil {
			return err
		}
		if len(args) >= 2 && args[1] == "create" {
			if len(args) < 3 || len(args) > 4 {
				return fmt.Errorf("usage: linuxbot sessions create <name> [working-directory]")
			}
			dir := workingDir
			if len(args) == 4 {
				dir = args[3]
			}
			created, err := service.Create(ctx, args[2], dir)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(stdout, "created session %s\n", created.Name)
			return err
		}
		sessions, err := service.List(ctx)
		if err != nil {
			return err
		}
		for _, item := range sessions {
			if _, err := fmt.Fprintf(stdout, "%s\t%s\t%s\n", item.Name, item.Mode, item.WorkingDirectory); err != nil {
				return err
			}
		}
		return nil
	case "switch":
		if len(args) != 2 {
			return fmt.Errorf("usage: linuxbot switch <name>")
		}
		return service.Switch(ctx, args[1])
	case "config":
		return runConfig(ctx, store, service, args[1:], workingDir, stdout)
	case "delete":
		return runDelete(ctx, store, service, args[1:], workingDir)
	case "web":
		if _, err := service.Current(ctx, workingDir); err != nil {
			return err
		}
		server := web.NewServer(web.Options{
			Store: store,
			Ask: func(ctx context.Context, session storage.Session, prompt string) (web.Answer, error) {
				start := time.Now()
				bot, err := buildAgent(ctx, store, session, nil, "web", nil)
				if err != nil {
					return web.Answer{}, err
				}
				text, err := bot.Run(ctx, session, prompt)
				if err != nil {
					return web.Answer{}, err
				}
				run, err := store.LatestRun(ctx, session.ID)
				if err != nil {
					return web.Answer{}, err
				}
				steps, err := store.ListRunSteps(ctx, run.ID)
				if err != nil {
					return web.Answer{}, err
				}
				return web.Answer{Text: text, RunID: run.ID, StepCount: len(steps), DurationMillis: time.Since(start).Milliseconds()}, nil
			},
			ApproveCommand: func(ctx context.Context, session storage.Session, step storage.Step) error {
				shell := tool.NewShellTool(tool.ShellOptions{
					Mode:             policy.ModeReview,
					WorkingDirectory: session.WorkingDirectory,
					OutputLimitBytes: 4 * 1024 * 1024,
					Approve: func(ctx context.Context, req tool.ApprovalRequest) (tool.ApprovalDecision, error) {
						return tool.ApprovalApprove, nil
					},
				})
				result, err := shell.Execute(ctx, tool.ToolRequest{Name: "shell", Input: map[string]string{"command": step.Input}})
				if err != nil {
					return err
				}
				if err := store.AddApproval(ctx, session.ID, step.RunID, result.Command, string(tool.ApprovalApprove), "web"); err != nil {
					return err
				}
				if err := store.UpdateStepResult(ctx, storage.Step{
					ID:                  step.ID,
					Status:              "approved",
					Output:              result.Stdout,
					ErrorText:           result.ErrorText,
					ExitCode:            result.ExitCode,
					DurationMillis:      result.DurationMillis,
					StdoutBytesObserved: result.StdoutBytesObserved,
					StderrBytesObserved: result.StderrBytesObserved,
				}); err != nil {
					return err
				}
				if err := store.AddStep(ctx, storage.Step{
					RunID:               step.RunID,
					Kind:                "command",
					Status:              result.Status,
					Input:               step.Input,
					Output:              result.Stdout,
					ErrorText:           result.ErrorText,
					ExitCode:            result.ExitCode,
					DurationMillis:      result.DurationMillis,
					StdoutBytesObserved: result.StdoutBytesObserved,
					StderrBytesObserved: result.StderrBytesObserved,
				}); err != nil {
					return err
				}
				if err := store.AddStep(ctx, storage.Step{RunID: step.RunID, Kind: "answer", Status: "done", Output: "命令已批准并执行。"}); err != nil {
					return err
				}
				return store.UpdateRunStatus(ctx, step.RunID, "done")
			},
		})
		_, _ = fmt.Fprintln(stdout, "LinuxBot Web listening on http://127.0.0.1:8787")
		return http.ListenAndServe("127.0.0.1:8787", server.Handler())
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func parseSessionArg(args []string) ([]string, string, error) {
	if len(args) >= 2 && args[0] == "--session" {
		return args[2:], args[1], nil
	}
	if len(args) == 1 && args[0] == "--session" {
		return nil, "", fmt.Errorf("usage: linuxbot --session <name>")
	}
	return args, "", nil
}

func currentSession(ctx context.Context, service *session.Service, explicitName string, fallbackWorkingDir string) (storage.Session, error) {
	if explicitName != "" {
		return service.Get(ctx, explicitName)
	}
	return service.Current(ctx, fallbackWorkingDir)
}

func runConfig(ctx context.Context, store *storage.Store, service *session.Service, args []string, workingDir string, stdout io.Writer) error {
	if len(args) == 0 {
		current, err := service.Current(ctx, workingDir)
		if err != nil {
			return err
		}
		providerConfig, err := store.ProviderConfig(ctx)
		if err != nil {
			return err
		}
		searchConfig, err := store.SearchConfig(ctx)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(stdout, "session=%s mode=%s cwd=%s\nprovider base_url=%s model=%s api_key=%s\nsearch enabled=%t tavily_api_key=%s\n",
			current.Name,
			current.Mode,
			current.WorkingDirectory,
			emptyLabel(providerConfig.BaseURL),
			emptyLabel(providerConfig.Model),
			secretLabel(providerConfig.APIKey),
			searchConfig.Enabled,
			secretLabel(searchConfig.TavilyAPIKey),
		)
		return err
	}
	switch args[0] {
	case "provider":
		if len(args) != 4 {
			return fmt.Errorf("usage: linuxbot config provider <base-url> <model> <api-key>")
		}
		return store.SetProviderConfig(ctx, storage.ProviderConfig{BaseURL: args[1], Model: args[2], APIKey: args[3]})
	case "search":
		if len(args) != 3 {
			return fmt.Errorf("usage: linuxbot config search <on|off> <tavily-api-key>")
		}
		if args[1] != "on" && args[1] != "off" {
			return fmt.Errorf("search must be on or off")
		}
		return store.SetSearchConfig(ctx, storage.SearchConfig{Enabled: args[1] == "on", TavilyAPIKey: args[2]})
	case "mode":
		if len(args) != 2 {
			return fmt.Errorf("usage: linuxbot config mode <safe|review|open>")
		}
		if args[1] != string(policy.ModeSafe) && args[1] != string(policy.ModeReview) && args[1] != string(policy.ModeOpen) {
			return fmt.Errorf("mode must be safe, review, or open")
		}
		current, err := service.Current(ctx, workingDir)
		if err != nil {
			return err
		}
		return store.SetSessionMode(ctx, current.ID, args[1])
	case "cwd":
		if len(args) != 2 {
			return fmt.Errorf("usage: linuxbot config cwd <working-directory>")
		}
		current, err := service.Current(ctx, workingDir)
		if err != nil {
			return err
		}
		return store.SetWorkingDirectory(ctx, current.ID, args[1])
	default:
		return fmt.Errorf("unknown config section %q", args[0])
	}
}

func emptyLabel(value string) string {
	if value == "" {
		return "<unset>"
	}
	return value
}

func secretLabel(value string) string {
	if value == "" {
		return "<unset>"
	}
	return "***"
}

func runDelete(ctx context.Context, store *storage.Store, service *session.Service, args []string, workingDir string) error {
	if len(args) != 2 || args[0] != "run" {
		return fmt.Errorf("usage: linuxbot delete run <run-id>")
	}
	runID, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid run id: %w", err)
	}
	current, err := service.Current(ctx, workingDir)
	if err != nil {
		return err
	}
	return store.DeleteRun(ctx, current.ID, runID, "cli")
}

func buildAgent(ctx context.Context, store *storage.Store, current storage.Session, approve tool.ApprovalFunc, approvalSource string, eventWriter io.Writer) (*agent.Agent, error) {
	providerConfig, err := store.ProviderConfig(ctx)
	if err != nil {
		return nil, err
	}
	if providerConfig.BaseURL == "" || providerConfig.Model == "" || providerConfig.APIKey == "" {
		return nil, fmt.Errorf("provider is not configured; run linuxbot config provider <base-url> <model> <api-key>")
	}
	searchConfig, err := store.SearchConfig(ctx)
	if err != nil {
		return nil, err
	}
	rules, err := store.AlwaysApproveRules(ctx, current.ID)
	if err != nil {
		return nil, err
	}
	llm := provider.NewOpenAICompatible(providerConfig.BaseURL, providerConfig.Model, providerConfig.APIKey, nil)
	shell := tool.NewShellTool(tool.ShellOptions{
		Mode:               policy.Mode(current.Mode),
		WorkingDirectory:   current.WorkingDirectory,
		AlwaysApproveExact: rules,
		OutputLimitBytes:   4 * 1024 * 1024,
		Approve:            approve,
	})
	search := tool.NewSearchTool(tool.SearchOptions{Enabled: searchConfig.Enabled, APIKey: searchConfig.TavilyAPIKey})
	return agent.New(store, llm, tool.NewRouter(shell, search), agent.Options{ApprovalSource: approvalSource, EventWriter: eventWriter}), nil
}
