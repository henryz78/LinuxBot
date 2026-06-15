package policy

import "strings"

type Mode string

const (
	ModeSafe   Mode = "safe"
	ModeReview Mode = "review"
	ModeOpen   Mode = "open"
)

type Action string

const (
	ActionAllow    Action = "allow"
	ActionApproval Action = "approval"
	ActionDeny     Action = "deny"
)

type EvaluationRequest struct {
	Mode               Mode
	Command            NormalizedCommand
	AlwaysApproveExact []string
}

type Decision struct {
	Action Action
	Reason string
}

func Evaluate(req EvaluationRequest) Decision {
	if isCritical(req.Command.Trimmed) || isCritical(req.Command.Wrapped) {
		return Decision{Action: ActionDeny, Reason: "critical command denylist"}
	}
	switch req.Mode {
	case ModeSafe:
		if safeAllowed(req.Command) {
			return Decision{Action: ActionAllow, Reason: "safe allowlist"}
		}
		return Decision{Action: ActionApproval, Reason: "outside safe allowlist"}
	case ModeReview:
		if exactApproved(req.Command.Trimmed, req.AlwaysApproveExact) {
			return Decision{Action: ActionAllow, Reason: "always approved"}
		}
		return Decision{Action: ActionApproval, Reason: "review mode"}
	case ModeOpen:
		return Decision{Action: ActionAllow, Reason: "open mode"}
	default:
		return Decision{Action: ActionApproval, Reason: "unknown mode"}
	}
}

func isCritical(command string) bool {
	command = strings.TrimSpace(command)
	if command == "" {
		return false
	}
	if wrapped := extractWrapped(command); wrapped != "" && isCritical(wrapped) {
		return true
	}
	fields := stripPrefixTokens(shellFields(command))
	if len(fields) == 0 {
		return false
	}
	root := commandBase(fields[0])
	args := fields[1:]
	if root == "reboot" || root == "poweroff" {
		return true
	}
	if root == "shutdown" && len(args) > 0 && args[0] == "now" {
		return true
	}
	if root == "mkfs" || strings.HasPrefix(root, "mkfs.") || root == "userdel" {
		return true
	}
	if root == "rm" && dangerousRM(args) {
		return true
	}
	if root == "dd" && dangerousDD(args) {
		return true
	}
	return false
}

func safeAllowed(command NormalizedCommand) bool {
	if command.Trimmed == "" || command.HasShellFeatures || command.Wrapped != "" {
		return false
	}
	fields := shellFields(command.Trimmed)
	if len(fields) == 0 || usesSudo(fields) || readsSensitivePath(command.Trimmed) {
		return false
	}
	if hasMutatingSafeToolFlags(command.Root, fields[1:]) {
		return false
	}
	roots := map[string]bool{
		"cat":        true,
		"date":       true,
		"df":         true,
		"du":         true,
		"find":       true,
		"free":       true,
		"grep":       true,
		"head":       true,
		"hostname":   true,
		"id":         true,
		"journalctl": true,
		"ls":         true,
		"ps":         true,
		"pwd":        true,
		"tail":       true,
		"uname":      true,
		"uptime":     true,
		"whoami":     true,
	}
	if roots[command.Root] {
		return true
	}
	return strings.HasPrefix(command.Trimmed, "systemctl status ")
}

func usesSudo(fields []string) bool {
	return len(fields) > 0 && commandBase(fields[0]) == "sudo"
}

func readsSensitivePath(command string) bool {
	sensitive := []string{"/etc/shadow", ".env", "id_rsa", "id_ed25519", ".ssh/", "credentials", "credential"}
	for _, item := range sensitive {
		if strings.Contains(command, item) {
			return true
		}
	}
	return false
}

func exactApproved(command string, rules []string) bool {
	for _, rule := range rules {
		if strings.TrimSpace(rule) == command {
			return true
		}
	}
	return false
}

func dangerousRM(args []string) bool {
	recursive := false
	force := false
	dangerousTarget := false
	for _, arg := range args {
		if arg == "/" || arg == "/*" {
			dangerousTarget = true
			continue
		}
		if strings.HasPrefix(arg, "-") {
			if arg == "--recursive" {
				recursive = true
			}
			if arg == "--force" {
				force = true
			}
			if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") {
				if strings.Contains(arg, "r") || strings.Contains(arg, "R") {
					recursive = true
				}
				if strings.Contains(arg, "f") {
					force = true
				}
			}
		}
	}
	return recursive && force && dangerousTarget
}

func dangerousDD(args []string) bool {
	hasDangerousInput := false
	hasBlockOutput := false
	for _, arg := range args {
		if arg == "if=/dev/zero" || arg == "if=/dev/random" || arg == "if=/dev/urandom" {
			hasDangerousInput = true
		}
		if strings.HasPrefix(arg, "of=/dev/") {
			hasBlockOutput = true
		}
	}
	return hasDangerousInput && hasBlockOutput
}

func hasMutatingSafeToolFlags(root string, args []string) bool {
	if root == "find" {
		return containsAnyArg(args, "-delete", "-exec", "-execdir", "-ok", "-okdir")
	}
	if root == "journalctl" {
		for _, arg := range args {
			if strings.HasPrefix(arg, "--vacuum-") || arg == "--rotate" {
				return true
			}
		}
	}
	return false
}

func containsAnyArg(args []string, values ...string) bool {
	for _, arg := range args {
		for _, value := range values {
			if arg == value {
				return true
			}
		}
	}
	return false
}
