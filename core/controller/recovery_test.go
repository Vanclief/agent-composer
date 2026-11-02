package controller

import "testing"

func TestResolveRecovery(t *testing.T) {
	t.Setenv("AGC_RECOVERY_RESTARTS", "")

	got := ResolveRecovery(RecoverySettings{})
	if got.Restarts != 3 {
		t.Fatalf("built-in restarts = %d, want 3", got.Restarts)
	}

	zero := 0
	got = ResolveRecovery(RecoverySettings{Restarts: &zero})
	if got.Restarts != 0 {
		t.Fatalf("configured restarts = %d, want 0", got.Restarts)
	}

	t.Setenv("AGC_RECOVERY_RESTARTS", "5")
	got = ResolveRecovery(RecoverySettings{Restarts: &zero})
	if got.Restarts != 5 {
		t.Fatalf("env restarts = %d, want 5", got.Restarts)
	}

	t.Setenv("AGC_RECOVERY_RESTARTS", "many")
	got = ResolveRecovery(RecoverySettings{})
	if got.Restarts != 3 {
		t.Fatalf("unreadable env restarts = %d, want the built-in 3", got.Restarts)
	}
}
