package domain

import (
	"errors"
	"testing"
)

// The table below is an independent copy of the architect's authoritative
// transition authority matrix. Agents hold no lifecycle authority, so only the
// Human and System columns are listed.
type transitionRow struct {
	from, to      TaskState
	human, system bool
}

var transitionSpec = []transitionRow{
	{StateDraft, StateInspecting, false, true},                 //  1
	{StateInspecting, StateContractReady, false, true},         //  2
	{StateContractReady, StateAwaitingApproval, false, true},   //  3
	{StateAwaitingApproval, StateApproved, true, false},        //  4
	{StateApproved, StateImplementing, false, true},            //  5
	{StateImplementing, StateVerifying, false, true},           //  6
	{StateImplementing, StateHalted, true, true},               //  7
	{StateVerifying, StateCorrecting, false, true},             //  8
	{StateVerifying, StateReviewing, false, false},             //  9
	{StateVerifying, StateHalted, true, true},                  // 10
	{StateCorrecting, StateVerifying, false, true},             // 11
	{StateCorrecting, StateHalted, true, true},                 // 12
	{StateReviewing, StateCorrecting, false, false},            // 13
	{StateReviewing, StateAwaitingMerge, false, false},         // 14
	{StateReviewing, StateHalted, true, true},                  // 15
	{StateAwaitingMerge, StateHumanApproval, false, true},      // 16
	{StateAwaitingMerge, StateHalted, true, true},              // 17
	{StateHumanApproval, StateMerged, false, false},            // 18
	{StateHumanApproval, StateHalted, true, true},              // 19
	{StateHalted, StateHumanReview, false, true},               // 20
	{StateHumanReview, StateContractRevised, true, false},      // 21
	{StateContractRevised, StateAwaitingApproval, false, true}, // 22
}

type actorCategory struct {
	name string
	kind ActorKind
	role Role
}

var actorCategories = []actorCategory{
	{"human", ActorKindHuman, ""},
	{"architect", ActorKindAgent, RoleArchitect},
	{"engineer", ActorKindAgent, RoleEngineer},
	{"verifier", ActorKindAgent, RoleVerifier},
	{"system", ActorKindSystem, ""},
}

func (r transitionRow) allows(c actorCategory) bool {
	switch c.kind {
	case ActorKindHuman:
		return r.human
	case ActorKindSystem:
		return r.system
	default:
		return false
	}
}

func specRow(from, to TaskState) (transitionRow, bool) {
	for _, r := range transitionSpec {
		if r.from == from && r.to == to {
			return r, true
		}
	}
	return transitionRow{}, false
}

func categoryActor(t *testing.T, c actorCategory) TrustedActor {
	t.Helper()
	return mustTrusted(t, ActorID(c.name+"-1"), c.kind, c.role)
}

// foreignTask implements Task from inside the package to prove that
// AuthorizeTransition accepts only the real implementation.
type foreignTask struct{}

func (foreignTask) ID() TaskID                                     { return "foreign" }
func (foreignTask) State() TaskState                               { return StateDraft }
func (foreignTask) Apply(TrustedActor, AuthorizedTransition) error { return ErrNotAuthorized }
func (foreignTask) sealed()                                        {}

func TestTransitionAuthorityMatrix(t *testing.T) {
	if len(transitionSpec) != 22 {
		t.Fatalf("test bug: expected 22 transitions, have %d", len(transitionSpec))
	}
	for _, row := range transitionSpec {
		if !specAllows(row.from, row.to) {
			t.Fatalf("test bug: %s -> %s is not a Gate 1 lifecycle edge", row.from, row.to)
		}
		for _, c := range actorCategories {
			actor := categoryActor(t, c)
			task := newTaskAt(t, row.from)
			proof, d := AuthorizeTransition(TransitionRequest{Actor: actor, Task: task, To: row.to})

			if row.allows(c) {
				if !d.IsAllowed() || d.Reason != ReasonAllowed {
					t.Errorf("%s %s->%s: %s/%s, want ALLOW", c.name, row.from, row.to, d.Outcome, d.Reason)
					continue
				}
				if err := task.Apply(actor, proof); err != nil {
					t.Errorf("%s %s->%s: Apply: %v", c.name, row.from, row.to, err)
				}
				if got := task.State(); got != row.to {
					t.Errorf("%s %s->%s: state = %s", c.name, row.from, row.to, got)
				}
				continue
			}

			if d.Outcome != OutcomeDeny || d.Reason != ReasonDeniedInsufficientAuthority {
				t.Errorf("%s %s->%s: %s/%s, want DENY/DeniedInsufficientAuthority", c.name, row.from, row.to, d.Outcome, d.Reason)
			}
			if err := task.Apply(actor, proof); !errors.Is(err, ErrNotAuthorized) {
				t.Errorf("%s %s->%s: Apply = %v, want ErrNotAuthorized", c.name, row.from, row.to, err)
			}
			if got := task.State(); got != row.from {
				t.Errorf("%s %s->%s: denied move changed state to %s", c.name, row.from, row.to, got)
			}
		}
	}
}

// Anything not listed in the authority table is denied to every actor, whether
// or not the lifecycle would permit it.
func TestUnlistedTransitionsAreDenied(t *testing.T) {
	for _, from := range specStates {
		for _, to := range specStates {
			if _, listed := specRow(from, to); listed {
				continue
			}
			for _, c := range actorCategories {
				actor := categoryActor(t, c)
				task := newTaskAt(t, from)
				proof, d := AuthorizeTransition(TransitionRequest{Actor: actor, Task: task, To: to})
				if d.Outcome != OutcomeDeny {
					t.Errorf("%s %s->%s: %s, want DENY", c.name, from, to, d.Outcome)
				}
				if err := task.Apply(actor, proof); !errors.Is(err, ErrNotAuthorized) {
					t.Errorf("%s %s->%s: Apply = %v, want ErrNotAuthorized", c.name, from, to, err)
				}
				if got := task.State(); got != from {
					t.Errorf("%s %s->%s: state changed to %s", c.name, from, to, got)
				}
			}
		}
	}
}

func TestAgentsNeverAdvanceTheLifecycle(t *testing.T) {
	for _, row := range transitionSpec {
		for _, c := range actorCategories {
			if c.kind != ActorKindAgent {
				continue
			}
			task := newTaskAt(t, row.from)
			_, d := AuthorizeTransition(TransitionRequest{Actor: categoryActor(t, c), Task: task, To: row.to})
			if d.Outcome != OutcomeDeny {
				t.Errorf("agent %s %s->%s: %s, want DENY", c.name, row.from, row.to, d.Outcome)
			}
		}
	}
}

func TestEngineerCannotSelfCertify(t *testing.T) {
	eng := mustTrusted(t, "eng-1", ActorKindAgent, RoleEngineer)
	for _, e := range []struct{ from, to TaskState }{
		{StateImplementing, StateVerifying},
		{StateVerifying, StateReviewing},
		{StateReviewing, StateAwaitingMerge},
		{StateVerifying, StateCorrecting},
		{StateAwaitingMerge, StateHumanApproval},
	} {
		task := newTaskAt(t, e.from)
		if _, d := AuthorizeTransition(TransitionRequest{Actor: eng, Task: task, To: e.to}); d.Outcome != OutcomeDeny {
			t.Errorf("engineer %s->%s: %s, want DENY", e.from, e.to, d.Outcome)
		}
	}
}

func TestPendingGateTransitionsAreDeniedToEveryone(t *testing.T) {
	pending := []struct{ from, to TaskState }{
		{StateVerifying, StateReviewing},
		{StateReviewing, StateCorrecting},
		{StateReviewing, StateAwaitingMerge},
		{StateHumanApproval, StateMerged},
	}
	for _, e := range pending {
		for _, c := range actorCategories {
			task := newTaskAt(t, e.from)
			proof, d := AuthorizeTransition(TransitionRequest{Actor: categoryActor(t, c), Task: task, To: e.to})
			if d.Outcome != OutcomeDeny {
				t.Errorf("%s %s->%s: %s, want DENY", c.name, e.from, e.to, d.Outcome)
			}
			if err := task.Apply(categoryActor(t, c), proof); !errors.Is(err, ErrNotAuthorized) {
				t.Errorf("%s %s->%s: Apply = %v", c.name, e.from, e.to, err)
			}
		}
	}
}

func TestSystemHoldsOnlyListedTransitions(t *testing.T) {
	system := mustTrusted(t, "forge-1", ActorKindSystem, "")
	for _, from := range specStates {
		for _, to := range specStates {
			row, listed := specRow(from, to)
			want := listed && row.system
			task := newTaskAt(t, from)
			_, d := AuthorizeTransition(TransitionRequest{Actor: system, Task: task, To: to})
			if d.IsAllowed() != want {
				t.Errorf("system %s->%s: allowed = %v, want %v", from, to, d.IsAllowed(), want)
			}
		}
	}
}

// A Human Merge approval lets an agent perform the Merge action in policy, but
// it never authorizes the MERGED transition. No actor may reach MERGED.
func TestMergedIsDeniedToEveryActor(t *testing.T) {
	agent := mustTrusted(t, "agent-1", ActorKindAgent, RoleEngineer)
	ap := mustApproval(t, "ap-1", "task-1", ActionMerge, agent)
	if d := Evaluate(Request{Actor: agent, Action: ActionMerge, TaskID: "task-1", Approval: ap}); !d.IsAllowed() {
		t.Fatalf("test premise: policy should allow the Merge action with approval, got %s", d.Outcome)
	}

	for _, from := range specStates {
		for _, c := range actorCategories {
			task := newTaskAt(t, from)
			_, d := AuthorizeTransition(TransitionRequest{Actor: categoryActor(t, c), Task: task, To: StateMerged})
			if d.Outcome != OutcomeDeny {
				t.Errorf("%s %s->MERGED: %s, want DENY", c.name, from, d.Outcome)
			}
		}
	}
}

// Lifecycle validation is independent of authorization: even a proof that
// authorization would never issue is rejected by the lifecycle.
func TestLifecycleValidationIsIndependentOfAuthorization(t *testing.T) {
	system := mustTrusted(t, "forge-1", ActorKindSystem, "")
	for _, from := range specStates {
		for _, to := range specStates {
			if specAllows(from, to) {
				continue
			}
			task := newTaskAt(t, from)
			before := snapOf(task)
			grant := &authGrant{task: task, from: from, to: to, actor: system}
			err := task.Apply(system, AuthorizedTransition{g: grant})
			if !errors.Is(err, ErrInvalidTransition) {
				t.Errorf("%s->%s: Apply = %v, want ErrInvalidTransition", from, to, err)
			}
			if snapOf(task) != before {
				t.Errorf("%s->%s: rejected move mutated the task", from, to)
			}
			if grant.consumed.Load() {
				t.Errorf("%s->%s: a rejected move consumed the proof", from, to)
			}
		}
	}
}

func TestZeroProofIsRejected(t *testing.T) {
	system := mustTrusted(t, "forge-1", ActorKindSystem, "")
	task := newTaskAt(t, StateDraft)
	if err := task.Apply(system, AuthorizedTransition{}); !errors.Is(err, ErrNotAuthorized) {
		t.Errorf("zero proof: Apply = %v, want ErrNotAuthorized", err)
	}
	if task.State() != StateDraft {
		t.Errorf("zero proof changed state to %s", task.State())
	}
}

// Two Task instances with the same textual id must not share proofs.
func TestProofIsBoundToItsTaskInstance(t *testing.T) {
	system := mustTrusted(t, "forge-1", ActorKindSystem, "")
	one := newTaskAt(t, StateDraft)
	two := newTaskAt(t, StateDraft)
	if one.ID() != two.ID() {
		t.Fatal("test bug: both tasks should share an id")
	}
	proof, d := AuthorizeTransition(TransitionRequest{Actor: system, Task: one, To: StateInspecting})
	if !d.IsAllowed() {
		t.Fatalf("not authorized: %+v", d)
	}
	if err := two.Apply(system, proof); !errors.Is(err, ErrNotAuthorized) {
		t.Errorf("proof for one instance applied to another: %v", err)
	}
	if two.State() != StateDraft {
		t.Errorf("the other task changed state to %s", two.State())
	}
	if err := one.Apply(system, proof); err != nil {
		t.Errorf("the proof should still work on its own task: %v", err)
	}
}

func TestProofIsBoundToItsActor(t *testing.T) {
	issued := mustTrusted(t, "forge-1", ActorKindSystem, "")
	otherSystem := mustTrusted(t, "forge-2", ActorKindSystem, "")
	sameIDOtherKind := mustTrusted(t, "forge-1", ActorKindAgent, RoleEngineer)
	var zero TrustedActor

	task := newTaskAt(t, StateDraft)
	proof, d := AuthorizeTransition(TransitionRequest{Actor: issued, Task: task, To: StateInspecting})
	if !d.IsAllowed() {
		t.Fatalf("not authorized: %+v", d)
	}
	for name, actor := range map[string]TrustedActor{
		"other system":        otherSystem,
		"same id, other kind": sameIDOtherKind,
		"zero actor":          zero,
	} {
		if err := task.Apply(actor, proof); !errors.Is(err, ErrNotAuthorized) {
			t.Errorf("%s: Apply = %v, want ErrNotAuthorized", name, err)
		}
	}
	if task.State() != StateDraft {
		t.Fatalf("a wrongly-bound apply changed state to %s", task.State())
	}
	if err := task.Apply(issued, proof); err != nil {
		t.Errorf("the issued actor should be able to apply: %v", err)
	}
}

func TestStaleProofIsRejected(t *testing.T) {
	system := mustTrusted(t, "forge-1", ActorKindSystem, "")
	task := newTaskAt(t, StateDraft)
	proof, _ := AuthorizeTransition(TransitionRequest{Actor: system, Task: task, To: StateInspecting})
	if err := task.transition(StateInspecting); err != nil {
		t.Fatal(err)
	}
	if err := task.Apply(system, proof); !errors.Is(err, ErrNotAuthorized) {
		t.Errorf("stale proof: Apply = %v, want ErrNotAuthorized", err)
	}
}

func TestProofAppliesOnlyItsOwnTarget(t *testing.T) {
	system := mustTrusted(t, "forge-1", ActorKindSystem, "")
	task := newTaskAt(t, StateVerifying)
	proof, d := AuthorizeTransition(TransitionRequest{Actor: system, Task: task, To: StateCorrecting})
	if !d.IsAllowed() {
		t.Fatalf("not authorized: %+v", d)
	}
	if err := task.Apply(system, proof); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if task.State() != StateCorrecting {
		t.Errorf("state = %s, want CORRECTING", task.State())
	}
}

// A human's approval of a contract must not be replayable after the task has
// looped back to AWAITING_APPROVAL.
func TestProofIsSingleUse(t *testing.T) {
	human := mustTrusted(t, "human-1", ActorKindHuman, "")
	task := newTaskAt(t, StateAwaitingApproval)
	proof, d := AuthorizeTransition(TransitionRequest{Actor: human, Task: task, To: StateApproved})
	if !d.IsAllowed() {
		t.Fatalf("not authorized: %+v", d)
	}
	replay := proof // a copy shares the same use
	if err := task.Apply(human, proof); err != nil {
		t.Fatalf("first Apply: %v", err)
	}
	for _, s := range []TaskState{StateImplementing, StateHalted, StateHumanReview, StateContractRevised, StateAwaitingApproval} {
		if err := task.transition(s); err != nil {
			t.Fatalf("setup %s: %v", s, err)
		}
	}
	if err := task.Apply(human, replay); !errors.Is(err, ErrNotAuthorized) {
		t.Errorf("replayed proof: Apply = %v, want ErrNotAuthorized", err)
	}
	if task.State() != StateAwaitingApproval {
		t.Errorf("replay changed state to %s", task.State())
	}
}

func TestAuthorizeTransitionFailsClosed(t *testing.T) {
	system := mustTrusted(t, "forge-1", ActorKindSystem, "")
	task := newTaskAt(t, StateDraft)
	var zeroActor TrustedActor

	cases := []struct {
		name string
		req  TransitionRequest
		want Reason
	}{
		{"zero actor", TransitionRequest{Actor: zeroActor, Task: task, To: StateInspecting}, ReasonDeniedUnknownActor},
		{"nil task", TransitionRequest{Actor: system, Task: nil, To: StateInspecting}, ReasonDeniedInvalidContext},
		{"zero task", TransitionRequest{Actor: system, Task: new(taskImpl), To: StateInspecting}, ReasonDeniedInvalidContext},
		{"foreign task", TransitionRequest{Actor: system, Task: foreignTask{}, To: StateInspecting}, ReasonDeniedInvalidContext},
		{"invalid target", TransitionRequest{Actor: system, Task: task, To: "NOT_A_STATE"}, ReasonDeniedInvalidContext},
	}
	for _, c := range cases {
		proof, d := AuthorizeTransition(c.req)
		if d.Outcome != OutcomeDeny || d.Reason != c.want {
			t.Errorf("%s: %s/%s, want DENY/%s", c.name, d.Outcome, d.Reason, c.want)
		}
		if err := task.Apply(system, proof); !errors.Is(err, ErrNotAuthorized) {
			t.Errorf("%s: Apply with the returned proof = %v, want ErrNotAuthorized", c.name, err)
		}
	}
	if task.State() != StateDraft {
		t.Errorf("failed-closed requests changed state to %s", task.State())
	}
}

func TestApplyOnNilTask(t *testing.T) {
	var task *taskImpl
	system := mustTrusted(t, "forge-1", ActorKindSystem, "")
	if err := task.Apply(system, AuthorizedTransition{}); !errors.Is(err, ErrNotAuthorized) {
		t.Errorf("nil task Apply = %v, want ErrNotAuthorized", err)
	}
}
