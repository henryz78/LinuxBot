# LinuxBot MVP Design

Date: 2026-06-14

## Summary

LinuxBot is a minimal BYOK server management assistant for Linux. It ships as a Go single binary with SQLite local storage and an embedded local Web UI. The default entry is an interactive CLI. The Web panel is started manually and binds to `127.0.0.1` by default.

The product is intentionally not a communication-platform bot. It does not target Telegram, Discord, Enterprise WeChat, OpenClaw-style orchestration, or multi-host fleet management in the first version.

## Goals

- Provide a lightweight natural-language assistant for managing one Linux server.
- Support BYOK LLM configuration.
- Support Tavily as the only initial search provider.
- Let the AI decide whether a task needs web search, while recording every search step.
- Allow arbitrary shell commands, controlled by session mode and approval policy.
- Support multiple stored sessions and switching between them.
- Store conversations, runs, command history, approvals, search steps, and settings in SQLite.
- Keep idle resource usage at zero by avoiding a persistent daemon.
- Keep runtime memory low by using Go, SQLite, and embedded static assets.

## Non-Goals

- No chat-platform integrations in the MVP.
- No always-on daemon by default.
- No remote Web exposure by default.
- No browser automation or desktop control.
- No multi-user role model in the MVP.
- No cluster, fleet, Kubernetes, or cloud-account management in the MVP.
- No heavy frontend framework requirement.

## Runtime Shape

`linuxbot` is a single executable.

Primary commands:

- `linuxbot`: enter the interactive CLI using the last selected session.
- `linuxbot --session <name>`: enter the interactive CLI using a specific session.
- `linuxbot sessions`: list stored sessions.
- `linuxbot switch <name>`: set the default session.
- `linuxbot web`: start the local Web panel on `127.0.0.1`.
- `linuxbot config`: inspect or update provider, search, and session settings.

The CLI and Web panel use the same SQLite database. Starting the Web panel does not create a privileged background agent. Shell commands run with the OS permissions of the process that launched `linuxbot`.

## Sessions

A session stores:

- Name and optional description.
- Current working directory.
- Current mode: `safe`, `review`, or `open`.
- Conversation history.
- Runs and execution steps.
- Always-approve command rules.
- LLM provider settings reference.
- Tavily search settings reference.
- Last-used timestamp.

Switching sessions changes context, history, mode, and approval rules. It does not change the Linux user or privilege boundary.

The session working directory defaults to the process working directory when the session is created. Users can change it from CLI or Web. Relative shell commands run from the session working directory unless a command explicitly changes directory for that invocation.

## Modes

### Safe

Safe mode is the conservative default.

- Safe mode uses an explicit allowlist.
- Commands inside the allowlist can execute automatically when they do not use `sudo`, shell control operators, redirection, command substitution, or denied paths.
- Commands outside the allowlist require one-time approval.
- Always-approve rules are ignored in safe mode.
- Commands matching the global critical command denylist are refused in safe mode, as they are in every mode.
- The UI must make it clear that arbitrary shell is still subject to policy evaluation.

MVP safe allowlist examples:

- `ls`
- `pwd`
- `whoami`
- `id`
- `hostname`
- `date`
- `uptime`
- `uname`
- `ps`
- `df`
- `free`
- `cat`
- `head`
- `tail`
- `grep`
- `find`
- `du`
- `systemctl status`
- `journalctl` read-only log inspection

Commands such as `docker ps`, `kubectl get pods`, database clients, cloud CLIs, package managers, and service mutation commands are outside the MVP safe allowlist and require approval.

Safe mode should also block obviously sensitive read paths from automatic execution, including private keys, `.env` files, credential files, and `/etc/shadow`. Those commands can move to approval unless they match the global critical denylist.

### Review

Review mode is the primary operational mode.

- Every shell command proposed by the AI requires approval before execution.
- The user can approve once, reject, edit and run, or mark a command as always approved.
- Always-approve rules are stored per session.
- MVP always-approve matching uses exact command text after trimming surrounding whitespace. Pattern, glob, or regex matching is out of scope for the MVP.

### Open

Open mode is a persistent session-level dangerous mode.

- All AI-proposed shell commands execute automatically after planning.
- The mode remains active for that session until the user changes it.
- The global critical command denylist still applies in open mode.
- CLI and Web must show clear danger labeling when a session is in open mode.
- All commands, outputs, exit codes, and timings are recorded.

## Critical Command Denylist

A small critical denylist applies in every mode, including `open`.

The MVP must refuse commands that clearly match destructive system patterns, including:

- `mkfs` and filesystem formatting variants.
- `dd` patterns that write zeroes or random data to block devices.
- `rm -rf /`, `rm -rf /*`, and equivalent recursive deletion of root.
- `reboot`.
- `shutdown now`.
- `poweroff`.
- destructive user deletion commands such as `userdel`.

The denylist is not a complete sandbox. It is a final guardrail for obvious catastrophic commands. Matching should happen before execution and should fail closed when the command parser cannot safely classify a critical pattern.

## AI Execution Flow

Each user prompt creates a `run`.

A run can contain these step types:

- `message`: user input or assistant output.
- `plan`: AI explanation of intended actions.
- `search`: Tavily search request and result summary.
- `command`: shell command, status, output, exit code, and duration.
- `approval`: approval decision and rule match.
- `answer`: final user-facing answer.
- `error`: tool, provider, command, or policy failure.

The AI may request a Tavily search before deciding on commands. The policy engine evaluates search availability, command execution, and approval state before actions run.

## Tool Router

The agent should not call shell execution directly.

All executable capabilities go through a small tool interface:

```go
type Tool interface {
    Name() string
    Execute(ctx context.Context, req ToolRequest) (ToolResult, error)
}
```

MVP registered tools:

- `shell`: command normalization, policy evaluation, approval, execution, and output capture.
- `search`: Tavily search execution and result capture.

The agent chooses tools, the tool router records steps, and individual tools execute. This keeps the MVP small while avoiding a future rewrite when adding tools such as file read/write, HTTP, Docker, or Git.

## Command Normalization

Raw AI-proposed shell must be normalized before policy evaluation.

Flow:

```text
LLM
 -> raw command
 -> command parser
 -> normalized command
 -> policy engine
 -> approval or runner
```

Normalization responsibilities:

- Trim surrounding whitespace.
- Extract the command root where possible.
- Preserve the original raw command for display and audit.
- Detect shell wrappers such as `sh -c`, `bash -c`, and `eval`.
- Run denylist checks against both the raw command and any extracted wrapped command string.
- Identify unsafe shell features for safe-mode allowlist decisions, including control operators, redirection, command substitution, and pipelines.

If normalization cannot confidently classify a command, policy must fail closed: deny if it might match the critical denylist, otherwise require approval.

This is not a full shell sandbox. It is a deterministic preprocessing layer so policy and audit work against consistent command metadata.

## Context Manager

The assistant must not send the full session history to the LLM forever.

Each request builds model context from:

- Recent messages.
- Recent runs.
- A persisted session summary.
- Current session settings and mode.
- Relevant pending approvals or command results.

MVP context defaults:

- Include the latest 20 messages.
- Include the latest 10 runs with compact step summaries.
- Maintain a `session_summary` that the AI updates when history grows beyond the active context window.

The session summary should preserve durable operational facts, user preferences, server assumptions, unresolved tasks, and previous important outcomes. It should not copy large command output verbatim.

## Provider Abstraction

LLM access goes through a provider interface.

Conceptual Go shape:

```go
type Provider interface {
    Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
}
```

MVP provider:

- OpenAI-compatible chat completion API.

Future provider targets:

- OpenAI native.
- Anthropic.
- Gemini.
- Ollama.

The rest of the system should depend on `Provider`, not directly on a specific HTTP API shape.

## Shell Execution

The MVP allows arbitrary shell commands, but execution must go through the command runner.

Command runner responsibilities:

- Receive the exact command text or argument vector chosen for execution.
- Apply the current session mode and approval rules.
- Run the command with timeout handling.
- Capture stdout, stderr, exit code, start time, end time, and duration.
- Stream output to CLI.
- Persist complete command metadata and output for Web inspection.

The command runner does not elevate privileges by itself. If users want privileged operations, they must run `linuxbot` under a user with suitable sudo permissions or explicitly include sudo in the approved command.

Command output must be bounded.

MVP output limits:

- Capture at most 4 MiB per stream for stdout and stderr.
- Store the head and tail of truncated streams, with a truncation marker in the middle.
- Preserve total observed byte counts when truncation occurs.
- Stream CLI output live but clearly mark truncation when the persisted limit is exceeded.
- Web should show truncated output with an explicit `[output truncated]` marker.

This prevents commands such as unbounded `journalctl` or `find /` from rapidly growing SQLite.

## CLI Experience

The CLI is interactive and transparent.

Example shape:

```text
linuxbot [default/review]> 重启 nginx
AI plan:
1. 检查 nginx 当前状态
2. 执行 systemctl restart nginx
3. 查看重启后的状态

Command:
systemctl status nginx --no-pager

Command:
systemctl restart nginx
Approve? yes/no/always/edit
```

CLI behavior:

- Show plans, searches, commands, outputs, and approval prompts directly.
- Do not collapse execution steps.
- Keep the final answer visible after execution.
- Allow session switching through commands or flags.

## Web Experience

The Web panel is a local operational view.

Default binding:

- Host: `127.0.0.1`
- Remote access should be done through SSH tunneling if needed.

Primary Web views:

- Chat/session view.
- Session switcher.
- Mode selector.
- Pending approval prompts.
- Run detail drawer or expandable section.
- Settings view for BYOK LLM and Tavily.

After each question, Web shows the final answer first and collapses the execution trace into a compact summary such as:

```text
已处理 4 个步骤 · 18s
```

Expanding the trace shows:

- AI plan.
- Tavily search steps.
- Shell commands.
- Approval decisions.
- stdout and stderr.
- Exit codes.
- Durations.
- Errors.

This collapse behavior is Web-only. CLI remains fully expanded.

## Search

Tavily is the only MVP search provider.

Search behavior:

- Disabled unless configured.
- The AI can decide when search is useful.
- Search requests are represented as explicit `search` steps.
- Search results are stored with the run for later inspection.
- If Tavily is unavailable or not configured, the assistant continues without search and explains the limitation when relevant.

## Configuration And Secrets

Configuration is stored locally.

Expected config areas:

- LLM provider base URL, model, and API key.
- Tavily API key and search enablement.
- Default session.
- Web bind address and port.
- Default mode for new sessions.

Secrets should not be printed in CLI, Web, logs, or exported records. API keys should be stored with restrictive file permissions. If OS keyring support is added later, it should be optional and not required for MVP operation.

## SQLite Data Model

Minimum tables:

- `sessions`: session metadata and current mode.
- `sessions.working_directory`: current working directory for relative commands.
- `session_summaries`: compact rolling summary for context-window management.
- `messages`: user and assistant conversation messages.
- `runs`: one user request and its execution lifecycle.
- `steps`: ordered plan, search, command, approval, answer, and error steps.
- `approvals`: approval decisions for commands.
- `always_approve_rules`: per-session command approval rules, with `match_type` and `pattern` fields reserved for future templates. MVP only enables exact command matching.
- `provider_config`: LLM provider configuration references.
- `search_config`: Tavily configuration references.
- `deletion_events`: deletion audit events without sensitive content.

The schema should support future migrations with a simple `schema_migrations` table.

## Deletion

All conversations support deletion.

Deleting a conversation hard-deletes:

- Messages.
- Runs.
- Steps.
- Shell command text and output.
- Search results.
- Final answers.
- Approval records attached to the deleted runs.

After deletion, the session summary must be regenerated from remaining history or invalidated and rebuilt on the next request. Deleted content must not survive inside `session_summaries`.

Deletion leaves only a minimal `deletion_events` record:

- Session ID.
- Deleted conversation or run ID.
- Timestamp.
- Source: CLI or Web.

The deletion event must not include user prompts, command text, command output, search text, or model output.

## Error Handling

The assistant should fail visibly and locally.

Required error cases:

- Missing LLM provider config.
- LLM provider request failure.
- Tavily not configured.
- Tavily request failure.
- Command timeout.
- Command non-zero exit.
- Approval rejected.
- Open mode warning.
- SQLite write failure.

The final answer should summarize what succeeded, what failed, and any relevant command output.

## Security Boundaries

The MVP is intentionally powerful because it allows arbitrary shell. The design relies on explicit local control rather than a claim of strong sandboxing.

Security rules:

- Web binds only to `127.0.0.1` by default.
- Safe mode uses an explicit allowlist.
- The global critical command denylist applies in every mode.
- Open mode is persistent but visibly dangerous.
- Shell commands always run as the launching OS user.
- Approval rules are session-scoped.
- Every command execution is recorded unless its conversation is later deleted.
- Deleted conversations leave only non-sensitive deletion events.
- Secrets are redacted from UI and logs.

## Resource Constraints

Implementation choices should preserve low resource use:

- Go single binary.
- SQLite local file.
- Embedded static Web assets.
- No Electron.
- No heavy frontend SPA requirement.
- No daemon unless the user explicitly starts `linuxbot web`.
- Avoid background polling when no Web page is active.

Target behavior:

- Zero process memory when not running.
- Low double-digit MB idle memory while Web is running.
- No dependency on containers for normal use.

## Testing Strategy

Core tests should cover:

- Session creation, switching, and persistence.
- Mode behavior for `safe`, `review`, and `open`.
- Safe mode allowlist behavior.
- Critical command denylist behavior across all modes.
- Command normalization for direct commands, `bash -c`, `sh -c`, and `eval`.
- Approval flow and always-approve matching.
- Provider abstraction with an OpenAI-compatible implementation.
- Context manager prompt assembly and session summary updates.
- Tool router dispatch for shell and search tools.
- Command runner success, failure, timeout, stdout, and stderr capture.
- Command output truncation and persisted byte counts.
- Session working directory behavior for relative commands.
- Run and step ordering.
- Web trace collapse data shape.
- Conversation hard deletion and minimal deletion event retention.
- Session summary regeneration or invalidation after deletion.
- Secret redaction.
- Tavily disabled and failure paths.

Manual verification should cover:

- `linuxbot` interactive CLI.
- `linuxbot --session <name>`.
- `linuxbot web` binding only to `127.0.0.1`.
- Web session switching.
- Web approval flow.
- Web expanded execution trace.

## Acceptance Criteria

The MVP is complete when:

- A user can configure BYOK LLM settings.
- A user can optionally configure Tavily.
- `linuxbot` opens an interactive CLI.
- `linuxbot web` opens a local-only Web panel.
- Multiple sessions can be created, stored, switched, and deleted.
- Each session can persistently use `safe`, `review`, or `open`.
- The AI can propose arbitrary shell commands.
- Commands are executed according to the active mode.
- Commands are normalized before policy evaluation.
- Safe mode uses an explicit allowlist and requires approval outside it.
- A global critical denylist blocks catastrophic commands in all modes.
- Review mode requires approval and supports always-approve rules.
- Open mode executes automatically with visible warning.
- Shell and Tavily execution go through a tool router.
- Sessions store and use a current working directory.
- stdout and stderr are truncated to bounded persisted sizes.
- Model requests use recent context plus a session summary rather than unbounded full history.
- LLM calls go through an OpenAI-compatible provider implementation behind a provider interface.
- CLI shows execution details directly.
- Web collapses execution details after each run and supports expansion.
- Conversation deletion removes sensitive content and leaves only a minimal deletion event.

## Roadmap Notes

Always-approve matching should stay exact in the MVP. A later v1 can add command templates such as:

- `systemctl restart nginx`
- `systemctl restart *`

The schema reserves `match_type` and `pattern` so this does not require a disruptive migration later.
