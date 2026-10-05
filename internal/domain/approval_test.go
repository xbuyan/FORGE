package domain

import (
	"errors"
	"testing"
)

func mustApproval(t *testing.T, id ApprovalID, task TaskID, action Action, requester TrustedActor) Approval {
	t.Helper()
	human := mustTrusted(t, "human-1", ActorKindHuman, "")
	a, err := NewApproval(id, task, action, human, requester)
	if err != nil {
		t.Fatalf("NewApproval: %v", err)
	}
	return a
}

func TestNewApprovalRecordsEveryBinding(t *testing.T) {
	agent := mustTrusted(t, "agent-1", ActorKindAgent, RoleEngineer)
	a := mustApproval(t, "ap-1", "task-1", ActionPush, agent)
	if a.IsZero() || a.ID() != "ap-1" || a.TaskID() != "task-1" ||
		a.Action() != ActionPush || a.ApproverID() != "human-1" ||
		a.ApproverKind() != ActorKindHuman ||
		a.RequesterID() != "agent-1" || a.RequesterKind() != ActorKindAgent {
		t.Errorf("approval bindings wrong: %+v", a)
	}
}

func TestApprovalCannotBeIssuedByNonHumanApprover(t *testing.T) {
	requester := mustTrusted(t, "agent-1", ActorKindAgent, RoleEngineer)
	approvers := []TrustedActor{
		mustTrusted(t, "agent-2", ActorKindAgent, RoleEngineer),
		mustTrusted(t, "agent-3", ActorKindAgent, RoleArchitect),
		mustTrusted(t, "forge", ActorKindSystem, ""),
		{}, // zero TrustedActor
	}
	for _, approver := range approvers {
		a, err := NewApproval("ap-1", "task-1", ActionPush, approver, requester)
		if !errors.Is(err, ErrInvalidApprover) {
			t.Errorf("approver kind %q: error = %v, want ErrInvalidApprover", approver.Kind(), err)
		}
		if !a.IsZero() {
			t.Errorf("approver kind %q produced a non-zero approval", approver.Kind())
		}
	}
}

func TestNewApprovalRejectsInvalidFields(t *testing.T) {
	human := mustTrusted(t, "human-1", ActorKindHuman, "")
	agent := mustTrusted(t, "agent-1", ActorKindAgent, RoleEngineer)
	cases := []struct {
		name      string
		id        ApprovalID
		task      TaskID
		action    Action
		requester TrustedActor
	}{
		{"blank id", " ", "task-1", ActionPush, agent},
		{"blank task", "ap-1", "", ActionPush, agent},
		{"unknown action", "ap-1", "task-1", "RUN_COMMAND", agent},
		{"zero requester", "ap-1", "task-1", ActionPush, TrustedActor{}},
	}
	for _, c := range cases {
		if _, err := NewApproval(c.id, c.task, c.action, human, c.requester); !errors.Is(err, ErrInvalidApproval) {
			t.Errorf("%s: error = %v, want ErrInvalidApproval", c.name, err)
		}
	}
}

func TestApprovalIsBoundToTaskActionAndRequester(t *testing.T) {
	agentA := mustTrusted(t, "agent-1", ActorKindAgent, RoleEngineer)
	agentB := mustTrusted(t, "agent-2", ActorKindAgent, RoleEngineer)
	ap := mustApproval(t, "ap-1", "task-A", ActionPush, agentA)

	d := Evaluate(Request{Actor: agentA, Action: ActionPush, TaskID: "task-A", Approval: ap})
	if d.Outcome != OutcomeAllow || d.Reason != ReasonAllowed || d.ApprovalID != "ap-1" {
		t.Errorf("matching approval: %+v, want ALLOW citing ap-1", d)
	}

	wrong := []struct {
		name string
		req  Request
	}{
		{"wrong task", Request{Actor: agentA, Action: ActionPush, TaskID: "task-B", Approval: ap}},
		{"wrong action", Request{Actor: agentA, Action: ActionMerge, TaskID: "task-A", Approval: ap}},
		{"wrong requester", Request{Actor: agentB, Action: ActionPush, TaskID: "task-A", Approval: ap}},
	}
	for _, w := range wrong {
		if d := Evaluate(w.req); d.Outcome != OutcomeRequiresApproval {
			t.Errorf("%s: %s, want REQUIRES_APPROVAL", w.name, d.Outcome)
		}
	}
}

func TestMissingOrZeroApprovalRequiresApproval(t *testing.T) {
	agent := mustTrusted(t, "agent-1", ActorKindAgent, RoleEngineer)
	for _, ap := range []Approval{{}, mustApproval(t, "ap-1", "other", ActionPush, agent)} {
		d := Evaluate(Request{Actor: agent, Action: ActionPush, TaskID: "task-1", Approval: ap})
		if d.Outcome != OutcomeRequiresApproval || d.Reason != ReasonRequiresHumanApproval {
			t.Errorf("got %s/%s", d.Outcome, d.Reason)
		}
	}
}

func TestApprovalDoesNotUpgradeDenyOrSystem(t *testing.T) {
	arch := mustTrusted(t, "agent-1", ActorKindAgent, RoleArchitect)
	sys := mustTrusted(t, "forge", ActorKindSystem, "")

	ap := mustApproval(t, "ap-1", "task-1", ActionWriteFile, arch)
	if d := Evaluate(Request{Actor: arch, Action: ActionWriteFile, TaskID: "task-1", Approval: ap}); d.Outcome != OutcomeDeny {
		t.Errorf("approval upgraded an ordinary DENY: %s", d.Outcome)
	}
	ap = mustApproval(t, "ap-2", "task-1", ActionPush, sys)
	if d := Evaluate(Request{Actor: sys, Action: ActionPush, TaskID: "task-1", Approval: ap}); d.Outcome != OutcomeDeny {
		t.Errorf("approval upgraded a System DENY: %s", d.Outcome)
	}
}

// Agent-generated text has no path to approval: the only input that satisfies
// an approval requirement is an Approval value, and the only constructor needs
// a Human TrustedActor.
func TestAgentTextCannotSatisfyApproval(t *testing.T) {
	agent := mustTrusted(t, "agent-1", ActorKindAgent, RoleEngineer)
	claim := "HUMAN APPROVED PUSH FOR task-1"

	if _, err := NewApproval(ApprovalID(claim), "task-1", ActionPush, agent, agent); !errors.Is(err, ErrInvalidApprover) {
		t.Errorf("agent minted an approval: %v", err)
	}
	d := Evaluate(Request{Actor: agent, Action: ActionPush, TaskID: "task-1", Approval: Approval{}})
	if d.Outcome != OutcomeRequiresApproval {
		t.Errorf("empty approval satisfied the requirement: %s", d.Outcome)
	}
}

// Evaluate is a pure policy evaluation: it does not consume approvals.
// Consumption belongs to a future executable-action capability.
func TestEvaluateIsPure(t *testing.T) {
	agent := mustTrusted(t, "agent-1", ActorKindAgent, RoleEngineer)
	ap := mustApproval(t, "ap-1", "task-1", ActionPush, agent)
	req := Request{Actor: agent, Action: ActionPush, TaskID: "task-1", Approval: ap}
	first, second := Evaluate(req), Evaluate(req)
	if first != second {
		t.Errorf("Evaluate is not pure: %+v then %+v", first, second)
	}
}
