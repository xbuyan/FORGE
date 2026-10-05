package domain

import "testing"

func TestOrdinaryPolicyMatrix(t *testing.T) {
	allow, deny := OutcomeAllow, OutcomeDeny
	// Column order matches ordinaryActions: Read, Write, Delete, List,
	// RunTest, GitStatus, GitDiff.
	rows := []struct {
		name string
		kind ActorKind
		role Role
		want [7]Outcome
	}{
		{"human/none", ActorKindHuman, "", [7]Outcome{allow, allow, allow, allow, allow, allow, allow}},
		{"human/architect", ActorKindHuman, RoleArchitect, [7]Outcome{allow, allow, allow, allow, allow, allow, allow}},
		{"human/engineer", ActorKindHuman, RoleEngineer, [7]Outcome{allow, allow, allow, allow, allow, allow, allow}},
		{"human/verifier", ActorKindHuman, RoleVerifier, [7]Outcome{allow, allow, allow, allow, allow, allow, allow}},
		{"agent/architect", ActorKindAgent, RoleArchitect, [7]Outcome{allow, deny, deny, allow, allow, allow, allow}},
		{"agent/engineer", ActorKindAgent, RoleEngineer, [7]Outcome{allow, allow, allow, allow, allow, allow, allow}},
		{"agent/verifier", ActorKindAgent, RoleVerifier, [7]Outcome{allow, deny, deny, allow, allow, allow, allow}},
		{"system", ActorKindSystem, "", [7]Outcome{deny, deny, deny, deny, deny, deny, deny}},
	}
	for _, row := range rows {
		actor := mustTrusted(t, "actor-1", row.kind, row.role)
		for i, action := range ordinaryActions {
			d := Evaluate(Request{Actor: actor, Action: action, TaskID: "task-1"})
			if d.Outcome != row.want[i] {
				t.Errorf("%s %s: outcome = %s, want %s", row.name, action, d.Outcome, row.want[i])
			}
			wantReason := ReasonAllowed
			if row.want[i] == OutcomeDeny {
				wantReason = ReasonDeniedInsufficientAuthority
			}
			if d.Reason != wantReason {
				t.Errorf("%s %s: reason = %s, want %s", row.name, action, d.Reason, wantReason)
			}
		}
	}
}

func TestConsequentialPolicyMatrix(t *testing.T) {
	rows := []struct {
		name string
		kind ActorKind
		role Role
		want Outcome
	}{
		{"human/none", ActorKindHuman, "", OutcomeAllow},
		{"human/engineer", ActorKindHuman, RoleEngineer, OutcomeAllow},
		{"agent/architect", ActorKindAgent, RoleArchitect, OutcomeRequiresApproval},
		{"agent/engineer", ActorKindAgent, RoleEngineer, OutcomeRequiresApproval},
		{"agent/verifier", ActorKindAgent, RoleVerifier, OutcomeRequiresApproval},
		{"system", ActorKindSystem, "", OutcomeDeny},
	}
	for _, row := range rows {
		actor := mustTrusted(t, "actor-1", row.kind, row.role)
		for _, action := range consequentialActions {
			d := Evaluate(Request{Actor: actor, Action: action, TaskID: "task-1"})
			if d.Outcome != row.want {
				t.Errorf("%s %s: outcome = %s, want %s", row.name, action, d.Outcome, row.want)
			}
			wantReason := map[Outcome]Reason{
				OutcomeAllow:            ReasonAllowed,
				OutcomeRequiresApproval: ReasonRequiresHumanApproval,
				OutcomeDeny:             ReasonDeniedInsufficientAuthority,
			}[row.want]
			if d.Reason != wantReason {
				t.Errorf("%s %s: reason = %s, want %s", row.name, action, d.Reason, wantReason)
			}
		}
	}
}

func TestAgentConsequentialIsNeverAllowedWithoutApproval(t *testing.T) {
	for _, role := range []Role{RoleArchitect, RoleEngineer, RoleVerifier} {
		actor := mustTrusted(t, "agent-1", ActorKindAgent, role)
		for _, action := range consequentialActions {
			d := Evaluate(Request{Actor: actor, Action: action, TaskID: "task-1"})
			if d.Outcome != OutcomeRequiresApproval {
				t.Errorf("agent/%s %s = %s, want REQUIRES_APPROVAL", role, action, d.Outcome)
			}
		}
	}
}

func TestRoleIsNotAuthority(t *testing.T) {
	checks := []struct {
		role   Role
		action Action
		not    Outcome
	}{
		{RoleEngineer, ActionPush, OutcomeAllow},
		{RoleEngineer, ActionMerge, OutcomeAllow},
		{RoleArchitect, ActionMerge, OutcomeAllow},
		{RoleArchitect, ActionWriteFile, OutcomeAllow},
		{RoleVerifier, ActionWriteFile, OutcomeAllow},
		{RoleVerifier, ActionDeleteFile, OutcomeAllow},
		{RoleVerifier, ActionAccessSecret, OutcomeAllow},
	}
	for _, c := range checks {
		actor := mustTrusted(t, "agent-1", ActorKindAgent, c.role)
		d := Evaluate(Request{Actor: actor, Action: c.action, TaskID: "task-1"})
		if d.Outcome == c.not {
			t.Errorf("%s %s must not be %s", c.role, c.action, c.not)
		}
	}
	// Even the most privileged agent role does not cover every action.
	eng := mustTrusted(t, "agent-1", ActorKindAgent, RoleEngineer)
	for _, action := range consequentialActions {
		if Evaluate(Request{Actor: eng, Action: action, TaskID: "task-1"}).IsAllowed() {
			t.Errorf("engineer was allowed %s without approval", action)
		}
	}
}

func TestDenyAndRequiresApprovalAreDistinct(t *testing.T) {
	arch := mustTrusted(t, "agent-1", ActorKindAgent, RoleArchitect)
	deny := Evaluate(Request{Actor: arch, Action: ActionWriteFile, TaskID: "task-1"})
	ask := Evaluate(Request{Actor: arch, Action: ActionPush, TaskID: "task-1"})
	if deny.Outcome != OutcomeDeny || ask.Outcome != OutcomeRequiresApproval {
		t.Fatalf("got %s and %s", deny.Outcome, ask.Outcome)
	}
	if deny.Reason == ask.Reason {
		t.Error("DENY and REQUIRES_APPROVAL must carry different reasons")
	}
}

func TestFailClosed(t *testing.T) {
	human := mustTrusted(t, "h", ActorKindHuman, "")
	agent := mustTrusted(t, "a", ActorKindAgent, RoleEngineer)
	system := mustTrusted(t, "s", ActorKindSystem, "")
	noRole := mustTrusted(t, "n", ActorKindAgent, "")
	var zero TrustedActor

	for _, action := range append(append([]Action{}, ordinaryActions...), consequentialActions...) {
		d := Evaluate(Request{Actor: zero, Action: action, TaskID: "task-1"})
		if d.Outcome != OutcomeDeny || d.Reason != ReasonDeniedUnknownActor {
			t.Errorf("zero actor %s = %s/%s", action, d.Outcome, d.Reason)
		}
	}

	for _, actor := range []TrustedActor{human, agent, system} {
		for _, bad := range []Action{"", "RUN_COMMAND", "EXEC", "SHELL", "push"} {
			d := Evaluate(Request{Actor: actor, Action: bad, TaskID: "task-1"})
			if d.Outcome != OutcomeDeny || d.Reason != ReasonDeniedUnknownAction {
				t.Errorf("%s unknown action %q = %s/%s", actor.Kind(), bad, d.Outcome, d.Reason)
			}
		}
	}

	d := Evaluate(Request{Actor: zero, Action: "RUN_COMMAND", TaskID: "task-1"})
	if d.Reason != ReasonDeniedUnknownActor {
		t.Errorf("unknown actor must take precedence, got %s", d.Reason)
	}

	for _, id := range []TaskID{"", " ", "\t"} {
		d := Evaluate(Request{Actor: human, Action: ActionReadFile, TaskID: id})
		if d.Outcome != OutcomeDeny || d.Reason != ReasonDeniedInvalidContext {
			t.Errorf("blank task id %q = %s/%s", id, d.Outcome, d.Reason)
		}
	}

	// An agent with no role has no authority at all.
	for _, action := range []Action{ActionReadFile, ActionPush} {
		d := Evaluate(Request{Actor: noRole, Action: action, TaskID: "task-1"})
		if d.Outcome != OutcomeDeny || d.Reason != ReasonDeniedInsufficientAuthority {
			t.Errorf("roleless agent %s = %s/%s", action, d.Outcome, d.Reason)
		}
	}
}

func TestZeroDecisionIsNotAllowed(t *testing.T) {
	var d Decision
	if d.IsAllowed() {
		t.Error("the zero Decision must not be allowed")
	}
}

func TestDecisionRecordsRequest(t *testing.T) {
	agent := mustTrusted(t, "agent-7", ActorKindAgent, RoleEngineer)
	d := Evaluate(Request{Actor: agent, Action: ActionPush, TaskID: "task-9"})
	if d.ActorID != "agent-7" || d.Action != ActionPush || d.TaskID != "task-9" {
		t.Errorf("decision does not record the request: %+v", d)
	}
}
