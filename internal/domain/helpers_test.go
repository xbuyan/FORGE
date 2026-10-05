package domain

import "testing"

type taskSnap struct {
	id    TaskID
	state TaskState
}

// snapOf captures a task's observable state without copying its lock.
func snapOf(t *taskImpl) taskSnap {
	id, st := t.snapshot()
	return taskSnap{id: id, state: st}
}

func mustAssertion(t *testing.T, id ActorID, kind ActorKind, role Role) ActorAssertion {
	t.Helper()
	a, err := NewActorAssertion(id, kind, role)
	if err != nil {
		t.Fatalf("NewActorAssertion(%q, %q, %q): %v", id, kind, role, err)
	}
	return a
}

// mustTrusted is the test-only issuer. Production code must not call
// IssueTrustedActor; the architecture test enforces that.
func mustTrusted(t *testing.T, id ActorID, kind ActorKind, role Role) TrustedActor {
	t.Helper()
	ta, err := IssueTrustedActor(mustAssertion(t, id, kind, role))
	if err != nil {
		t.Fatalf("IssueTrustedActor(%q, %q, %q): %v", id, kind, role, err)
	}
	return ta
}
