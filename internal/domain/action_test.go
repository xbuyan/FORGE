package domain

import (
	"strings"
	"testing"
)

var ordinaryActions = []Action{
	ActionReadFile, ActionWriteFile, ActionDeleteFile, ActionListFiles,
	ActionRunTest, ActionGitStatus, ActionGitDiff,
}

var consequentialActions = []Action{
	ActionCreateWorktree, ActionCreateCommit, ActionPush, ActionMerge,
	ActionExternalRequest, ActionAccessSecret,
}

func TestActionClassification(t *testing.T) {
	if len(ordinaryActions) != 7 || len(consequentialActions) != 6 {
		t.Fatalf("test bug: expected 7 ordinary and 6 consequential actions")
	}
	seen := map[Action]bool{}
	for _, a := range ordinaryActions {
		if !a.IsValid() || !a.IsOrdinary() || a.IsConsequential() {
			t.Errorf("%q should be a valid ordinary action", a)
		}
		seen[a] = true
	}
	for _, a := range consequentialActions {
		if !a.IsValid() || a.IsOrdinary() || !a.IsConsequential() {
			t.Errorf("%q should be a valid consequential action", a)
		}
		if seen[a] {
			t.Errorf("duplicate action value %q", a)
		}
		seen[a] = true
	}
}

func TestUnknownActionsAreInvalid(t *testing.T) {
	for _, s := range []string{
		"", " ", "RUN_COMMAND", "EXEC", "SHELL", "EXECUTE_ANYTHING",
		"RunCommand", "read_file", "READ_FILE ", "PUSH\n",
	} {
		if Action(s).IsValid() {
			t.Errorf("Action(%q) must be invalid", s)
		}
	}
}

func TestNoGenericCommandAuthority(t *testing.T) {
	for _, a := range append(append([]Action{}, ordinaryActions...), consequentialActions...) {
		u := strings.ToUpper(string(a))
		for _, banned := range []string{"COMMAND", "EXEC", "SHELL", "ANYTHING"} {
			if strings.Contains(u, banned) {
				t.Errorf("action %q looks like unrestricted command authority", a)
			}
		}
	}
}
