package policy

import "testing"

func TestCriticalDenylistCatchesDirectAndWrappedCommands(t *testing.T) {
	cases := []string{
		"rm -rf /",
		"bash -c \"rm -rf /\"",
		"sh -c 'shutdown now'",
		"eval \"poweroff\"",
		"dd if=/dev/zero of=/dev/sda",
	}
	for _, command := range cases {
		decision := Evaluate(EvaluationRequest{Mode: ModeOpen, Command: Normalize(command)})
		if decision.Action != ActionDeny {
			t.Fatalf("%q action = %s reason = %s", command, decision.Action, decision.Reason)
		}
	}
}

func TestSafeAllowlistAllowsSimpleReadCommand(t *testing.T) {
	decision := Evaluate(EvaluationRequest{Mode: ModeSafe, Command: Normalize("df -h")})
	if decision.Action != ActionAllow {
		t.Fatalf("action = %s reason = %s", decision.Action, decision.Reason)
	}
}

func TestSafeAllowlistRequiresApprovalForDocker(t *testing.T) {
	decision := Evaluate(EvaluationRequest{Mode: ModeSafe, Command: Normalize("docker ps")})
	if decision.Action != ActionApproval {
		t.Fatalf("action = %s", decision.Action)
	}
}

func TestReviewExactAlwaysApprove(t *testing.T) {
	decision := Evaluate(EvaluationRequest{
		Mode:               ModeReview,
		Command:            Normalize("systemctl restart nginx"),
		AlwaysApproveExact: []string{"systemctl restart nginx"},
	})
	if decision.Action != ActionAllow {
		t.Fatalf("action = %s", decision.Action)
	}
}

func TestSafeModeRequiresApprovalForSensitiveReads(t *testing.T) {
	decision := Evaluate(EvaluationRequest{Mode: ModeSafe, Command: Normalize("cat /etc/shadow")})
	if decision.Action != ActionApproval {
		t.Fatalf("action = %s", decision.Action)
	}
}

func TestSafeModeRequiresApprovalForMutatingReadTools(t *testing.T) {
	cases := []string{
		"find . -delete",
		"find . -exec rm {} \\;",
		"journalctl --vacuum-size=100M",
	}
	for _, command := range cases {
		decision := Evaluate(EvaluationRequest{Mode: ModeSafe, Command: Normalize(command)})
		if decision.Action != ActionApproval {
			t.Fatalf("%q action = %s", command, decision.Action)
		}
	}
}

func TestCriticalDenylistHandlesSudoPathsAndShellLC(t *testing.T) {
	cases := []string{
		"sudo reboot",
		"/sbin/reboot",
		"bash -lc \"rm -rf /\"",
		"rm -rf --no-preserve-root /",
		"rm --recursive --force /",
	}
	for _, command := range cases {
		decision := Evaluate(EvaluationRequest{Mode: ModeOpen, Command: Normalize(command)})
		if decision.Action != ActionDeny {
			t.Fatalf("%q action = %s reason = %s", command, decision.Action, decision.Reason)
		}
	}
}
