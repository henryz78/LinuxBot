package policy

import (
	"path"
	"strings"
)

type NormalizedCommand struct {
	Raw              string
	Trimmed          string
	Root             string
	Wrapped          string
	HasShellFeatures bool
}

func Normalize(raw string) NormalizedCommand {
	trimmed := strings.TrimSpace(raw)
	wrapped := extractWrapped(trimmed)
	return NormalizedCommand{
		Raw:              raw,
		Trimmed:          trimmed,
		Root:             commandRoot(trimmed),
		Wrapped:          wrapped,
		HasShellFeatures: hasShellFeatures(trimmed),
	}
}

func firstField(command string) string {
	fields := shellFields(command)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func extractWrapped(command string) string {
	fields := shellFields(command)
	if len(fields) < 2 {
		return ""
	}
	fields = stripPrefixTokens(fields)
	if len(fields) < 2 {
		return ""
	}
	root := commandBase(fields[0])
	if root == "eval" {
		return trimShellQuotes(strings.Join(fields[1:], " "))
	}
	if len(fields) >= 3 && (root == "bash" || root == "sh") && (fields[1] == "-c" || fields[1] == "-lc") {
		return trimShellQuotes(strings.Join(fields[2:], " "))
	}
	return ""
}

func trimShellQuotes(value string) string {
	return strings.Trim(strings.TrimSpace(value), `"'`)
}

func hasShellFeatures(command string) bool {
	for _, marker := range []string{"&&", "||", ";", "|", ">", "<", "$(", "`"} {
		if strings.Contains(command, marker) {
			return true
		}
	}
	return false
}

func shellFields(command string) []string {
	var fields []string
	var builder strings.Builder
	var quote rune
	escaped := false
	for _, r := range command {
		switch {
		case escaped:
			builder.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				builder.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ' ' || r == '\t' || r == '\n':
			if builder.Len() > 0 {
				fields = append(fields, builder.String())
				builder.Reset()
			}
		default:
			builder.WriteRune(r)
		}
	}
	if escaped {
		builder.WriteRune('\\')
	}
	if builder.Len() > 0 {
		fields = append(fields, builder.String())
	}
	return fields
}

func commandRoot(command string) string {
	fields := stripPrefixTokens(shellFields(command))
	if len(fields) == 0 {
		return ""
	}
	return commandBase(fields[0])
}

func stripPrefixTokens(fields []string) []string {
	for len(fields) > 0 {
		root := commandBase(fields[0])
		switch {
		case root == "sudo":
			fields = fields[1:]
		case root == "env":
			fields = fields[1:]
			for len(fields) > 0 && strings.Contains(fields[0], "=") {
				fields = fields[1:]
			}
		case strings.Contains(fields[0], "=") && !strings.HasPrefix(fields[0], "-"):
			fields = fields[1:]
		default:
			return fields
		}
	}
	return fields
}

func commandBase(command string) string {
	command = strings.ReplaceAll(command, "\\", "/")
	return path.Base(command)
}
