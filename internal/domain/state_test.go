package domain

import "testing"

func TestTaskStateIsValid(t *testing.T) {
	for _, s := range specStates {
		if !s.IsValid() {
			t.Errorf("%q should be a valid state", s)
		}
	}

	invalid := []TaskState{"", "NOT_A_STATE", "draft", "Draft", " DRAFT", "DRAFT ", "merged", "MERGED\n"}
	for _, s := range invalid {
		if s.IsValid() {
			t.Errorf("%q should not be a valid state", s)
		}
	}
}

func TestOnlyMergedIsTerminal(t *testing.T) {
	for _, s := range specStates {
		if got, want := s.IsTerminal(), s == StateMerged; got != want {
			t.Errorf("%s.IsTerminal() = %v, want %v", s, got, want)
		}
	}
}

func TestCanTransitionFailsClosedOnInvalidInput(t *testing.T) {
	bad := TaskState("NOT_A_STATE")
	cases := []struct{ from, to TaskState }{
		{"", ""},
		{"", StateDraft},
		{StateDraft, ""},
		{bad, bad},
		{bad, StateInspecting},
		{StateDraft, bad},
	}
	for _, c := range cases {
		if CanTransition(c.from, c.to) {
			t.Errorf("CanTransition(%q, %q) = true, want false", c.from, c.to)
		}
	}
}
