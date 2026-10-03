package domain

import (
	"errors"
	"fmt"
	"slices"
	"testing"
)

// The tables below are an independent copy of the approved lifecycle. They are
// deliberately not derived from the implementation, so weakening or extending
// the state machine makes these tests fail.

var specStates = []TaskState{
	StateDraft,
	StateInspecting,
	StateContractReady,
	StateAwaitingApproval,
	StateApproved,
	StateImplementing,
	StateVerifying,
	StateCorrecting,
	StateReviewing,
	StateAwaitingMerge,
	StateHumanApproval,
	StateMerged,
	StateHalted,
	StateHumanReview,
	StateContractRevised,
}

var specEdges = []struct {
	from TaskState
	to   []TaskState
}{
	{StateDraft, []TaskState{StateInspecting}},
	{StateInspecting, []TaskState{StateContractReady}},
	{StateContractReady, []TaskState{StateAwaitingApproval}},
	{StateAwaitingApproval, []TaskState{StateApproved}},
	{StateApproved, []TaskState{StateImplementing}},
	{StateImplementing, []TaskState{StateVerifying, StateHalted}},
	{StateVerifying, []TaskState{StateCorrecting, StateReviewing, StateHalted}},
	{StateCorrecting, []TaskState{StateVerifying, StateHalted}},
	{StateReviewing, []TaskState{StateCorrecting, StateAwaitingMerge, StateHalted}},
	{StateAwaitingMerge, []TaskState{StateHumanApproval, StateHalted}},
	{StateHumanApproval, []TaskState{StateMerged, StateHalted}},
	{StateMerged, nil},
	{StateHalted, []TaskState{StateHumanReview}},
	{StateHumanReview, []TaskState{StateContractRevised}},
	{StateContractRevised, []TaskState{StateAwaitingApproval}},
}

func specAllows(from, to TaskState) bool {
	for _, e := range specEdges {
		if e.from == from {
			return slices.Contains(e.to, to)
		}
	}
	return false
}

// pathTo returns a sequence of Transition calls that takes a new task from
// StateDraft to target using only approved transitions.
func pathTo(target TaskState) ([]TaskState, bool) {
	approved := []TaskState{StateInspecting, StateContractReady, StateAwaitingApproval, StateApproved}
	implementing := append(slices.Clone(approved), StateImplementing)
	verifying := append(slices.Clone(implementing), StateVerifying)
	reviewing := append(slices.Clone(verifying), StateReviewing)
	awaitingMerge := append(slices.Clone(reviewing), StateAwaitingMerge)
	humanApproval := append(slices.Clone(awaitingMerge), StateHumanApproval)
	halted := append(slices.Clone(implementing), StateHalted)
	humanReview := append(slices.Clone(halted), StateHumanReview)

	switch target {
	case StateDraft:
		return nil, true
	case StateInspecting:
		return approved[:1], true
	case StateContractReady:
		return approved[:2], true
	case StateAwaitingApproval:
		return approved[:3], true
	case StateApproved:
		return approved, true
	case StateImplementing:
		return implementing, true
	case StateVerifying:
		return verifying, true
	case StateCorrecting:
		return append(slices.Clone(verifying), StateCorrecting), true
	case StateReviewing:
		return reviewing, true
	case StateAwaitingMerge:
		return awaitingMerge, true
	case StateHumanApproval:
		return humanApproval, true
	case StateMerged:
		return append(slices.Clone(humanApproval), StateMerged), true
	case StateHalted:
		return halted, true
	case StateHumanReview:
		return humanReview, true
	case StateContractRevised:
		return append(slices.Clone(humanReview), StateContractRevised), true
	default:
		return nil, false
	}
}

// newTaskAt builds a task and walks it to target through the public API.
func newTaskAt(t *testing.T, target TaskState) *Task {
	t.Helper()
	path, ok := pathTo(target)
	if !ok {
		t.Fatalf("test bug: no setup path to %q", target)
	}
	task, err := NewTask("task-1")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	for _, step := range path {
		if err := task.Transition(step); err != nil {
			t.Fatalf("setup for %s: Transition(%s): %v", target, step, err)
		}
	}
	if got := task.State(); got != target {
		t.Fatalf("setup produced %s, want %s", got, target)
	}
	return task
}

// reachable returns every state reachable from the given state without
// entering any blocked state, using the implementation's CanTransition.
func reachable(from TaskState, blocked ...TaskState) map[TaskState]bool {
	seen := map[TaskState]bool{from: true}
	queue := []TaskState{from}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range specStates {
			if seen[next] || slices.Contains(blocked, next) || !CanTransition(cur, next) {
				continue
			}
			seen[next] = true
			queue = append(queue, next)
		}
	}
	return seen
}

func TestSpecTablesAreComplete(t *testing.T) {
	if len(specStates) != 15 {
		t.Fatalf("test bug: expected 15 states, have %d", len(specStates))
	}
	if len(specEdges) != len(specStates) {
		t.Fatalf("test bug: %d edge rows for %d states", len(specEdges), len(specStates))
	}
	for _, s := range specStates {
		if _, ok := pathTo(s); !ok {
			t.Errorf("test bug: no setup path to %s", s)
		}
	}
}

func TestNewTaskStartsInDraft(t *testing.T) {
	task, err := NewTask("task-1")
	if err != nil {
		t.Fatalf("NewTask returned error: %v", err)
	}
	if got := task.State(); got != StateDraft {
		t.Errorf("State() = %s, want %s", got, StateDraft)
	}
	if got := task.ID(); got != "task-1" {
		t.Errorf("ID() = %q, want %q", got, "task-1")
	}
}

func TestNewTaskRejectsBlankID(t *testing.T) {
	for _, id := range []TaskID{"", " ", "\t\n"} {
		task, err := NewTask(id)
		if !errors.Is(err, ErrInvalidTaskID) {
			t.Errorf("NewTask(%q) error = %v, want ErrInvalidTaskID", id, err)
		}
		if task != nil {
			t.Errorf("NewTask(%q) returned a task for an invalid id", id)
		}
	}
}

func TestApprovedTransitionsSucceed(t *testing.T) {
	for _, e := range specEdges {
		for _, to := range e.to {
			t.Run(fmt.Sprintf("%s_to_%s", e.from, to), func(t *testing.T) {
				task := newTaskAt(t, e.from)
				if err := task.Transition(to); err != nil {
					t.Fatalf("Transition(%s) from %s: %v", to, e.from, err)
				}
				if got := task.State(); got != to {
					t.Fatalf("State() = %s, want %s", got, to)
				}
			})
		}
	}
}

// Every (from, to) pair in the lifecycle is checked: pairs in the approved
// table must succeed, every other pair must be rejected and must leave the
// task completely unchanged.
func TestTransitionMatrix(t *testing.T) {
	for _, from := range specStates {
		for _, to := range specStates {
			t.Run(fmt.Sprintf("%s_to_%s", from, to), func(t *testing.T) {
				task := newTaskAt(t, from)
				before := *task
				err := task.Transition(to)

				if specAllows(from, to) {
					if err != nil {
						t.Fatalf("expected transition to succeed: %v", err)
					}
					if got := task.State(); got != to {
						t.Fatalf("State() = %s, want %s", got, to)
					}
					return
				}

				if !errors.Is(err, ErrInvalidTransition) {
					t.Fatalf("error = %v, want ErrInvalidTransition", err)
				}
				if *task != before {
					t.Fatalf("rejected transition mutated the task: before %+v, after %+v", before, *task)
				}
			})
		}
	}
}

func TestInvalidTargetStatesRejected(t *testing.T) {
	bad := []TaskState{"", "NOT_A_STATE", "draft", " DRAFT", "DRAFT ", "merged"}
	for _, from := range specStates {
		for _, to := range bad {
			task := newTaskAt(t, from)
			before := *task
			err := task.Transition(to)
			if !errors.Is(err, ErrInvalidState) {
				t.Errorf("%s -> %q: error = %v, want ErrInvalidState", from, to, err)
			}
			if *task != before {
				t.Errorf("%s -> %q mutated the task: before %+v, after %+v", from, to, before, *task)
			}
		}
	}
}

func TestZeroValueTaskFailsClosed(t *testing.T) {
	for _, task := range []*Task{new(Task), {}} {
		for _, to := range specStates {
			err := task.Transition(to)
			if !errors.Is(err, ErrInvalidState) {
				t.Errorf("zero Task -> %s: error = %v, want ErrInvalidState", to, err)
			}
			if task.State().IsValid() {
				t.Errorf("zero Task acquired valid state %q after Transition(%s)", task.State(), to)
			}
		}
		if task.ID() != "" {
			t.Errorf("zero Task has id %q", task.ID())
		}
	}
}

func TestMergedIsTerminal(t *testing.T) {
	for _, to := range specStates {
		task := newTaskAt(t, StateMerged)
		err := task.Transition(to)
		if !errors.Is(err, ErrTerminalState) || !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("MERGED -> %s: error = %v, want ErrTerminalState and ErrInvalidTransition", to, err)
		}
		if got := task.State(); got != StateMerged {
			t.Errorf("MERGED -> %s changed state to %s", to, got)
		}
	}

	task := newTaskAt(t, StateMerged)
	if err := task.Transition("NOT_A_STATE"); err == nil {
		t.Error("MERGED -> invalid state succeeded")
	}
	if got := task.State(); got != StateMerged {
		t.Errorf("state after rejected move out of MERGED = %s", got)
	}
}

func TestFullLifecycleReachesMerged(t *testing.T) {
	task, err := NewTask("task-1")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	steps := []TaskState{
		StateInspecting,
		StateContractReady,
		StateAwaitingApproval,
		StateApproved,
		StateImplementing,
		StateVerifying,
		StateReviewing,
		StateAwaitingMerge,
		StateHumanApproval,
		StateMerged,
	}
	for _, s := range steps {
		if err := task.Transition(s); err != nil {
			t.Fatalf("Transition(%s): %v", s, err)
		}
	}
	if got := task.State(); got != StateMerged {
		t.Errorf("final state = %s, want %s", got, StateMerged)
	}
}

// No state may be entered without first passing through its gate. In
// particular MERGED is only reachable through HUMAN_APPROVAL, which is only
// reachable through AWAITING_MERGE, so nothing jumps straight to a merge.
func TestStatesCannotBeReachedWithoutTheirGate(t *testing.T) {
	gates := []struct{ target, gate TaskState }{
		{StateMerged, StateHumanApproval},
		{StateHumanApproval, StateAwaitingMerge},
		{StateAwaitingMerge, StateReviewing},
		{StateReviewing, StateVerifying},
		{StateImplementing, StateApproved},
		{StateApproved, StateAwaitingApproval},
	}
	for _, g := range gates {
		if !reachable(StateDraft)[g.target] {
			t.Errorf("%s should be reachable from %s", g.target, StateDraft)
		}
		if reachable(StateDraft, g.gate)[g.target] {
			t.Errorf("%s is reachable from %s without passing through %s", g.target, StateDraft, g.gate)
		}
	}
}

func TestOnlyHumanApprovalCanMerge(t *testing.T) {
	for _, from := range specStates {
		if from == StateHumanApproval {
			continue
		}
		task := newTaskAt(t, from)
		if err := task.Transition(StateMerged); err == nil {
			t.Errorf("%s -> MERGED succeeded; only HUMAN_APPROVAL may precede MERGED", from)
		}
		if got := task.State(); got != from {
			t.Errorf("rejected %s -> MERGED changed state to %s", from, got)
		}
	}
}

func TestFailureAndViolationPathsAreDistinct(t *testing.T) {
	if StateCorrecting == StateHalted {
		t.Fatal("CORRECTING and HALTED must be different states")
	}

	allowed := []struct{ from, to TaskState }{
		{StateVerifying, StateCorrecting},
		{StateCorrecting, StateVerifying},
		{StateVerifying, StateHalted},
		{StateImplementing, StateHalted},
	}
	for _, c := range allowed {
		task := newTaskAt(t, c.from)
		if err := task.Transition(c.to); err != nil {
			t.Errorf("%s -> %s should be allowed: %v", c.from, c.to, err)
		}
	}

	denied := []struct{ from, to TaskState }{
		{StateImplementing, StateCorrecting},
		{StateHalted, StateCorrecting},
		{StateHalted, StateVerifying},
		{StateHalted, StateImplementing},
		{StateCorrecting, StateReviewing},
		{StateCorrecting, StateAwaitingMerge},
	}
	for _, c := range denied {
		task := newTaskAt(t, c.from)
		if err := task.Transition(c.to); !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("%s -> %s error = %v, want ErrInvalidTransition", c.from, c.to, err)
		}
		if got := task.State(); got != c.from {
			t.Errorf("rejected %s -> %s changed state to %s", c.from, c.to, got)
		}
	}
}

func TestRepeatedCorrectionCyclesThenReview(t *testing.T) {
	task := newTaskAt(t, StateVerifying)
	for i := 0; i < 3; i++ {
		for _, s := range []TaskState{StateCorrecting, StateVerifying} {
			if err := task.Transition(s); err != nil {
				t.Fatalf("cycle %d: Transition(%s): %v", i, s, err)
			}
		}
	}
	if err := task.Transition(StateReviewing); err != nil {
		t.Fatalf("Transition(REVIEWING) after corrections: %v", err)
	}
}

func TestHaltedOnlyExitsToHumanReview(t *testing.T) {
	for _, to := range specStates {
		task := newTaskAt(t, StateHalted)
		err := task.Transition(to)
		if to == StateHumanReview {
			if err != nil {
				t.Errorf("HALTED -> HUMAN_REVIEW should be allowed: %v", err)
			}
			continue
		}
		if !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("HALTED -> %s error = %v, want ErrInvalidTransition", to, err)
		}
		if got := task.State(); got != StateHalted {
			t.Errorf("rejected HALTED -> %s changed state to %s", to, got)
		}
	}
}

// Recovery from HALTED must run the whole human path: HUMAN_REVIEW, then
// CONTRACT_REVISED, then AWAITING_APPROVAL, then APPROVED, before any
// execution or merge state can be entered again.
func TestHaltedRecoveryRequiresFullHumanPath(t *testing.T) {
	if !reachable(StateHalted)[StateImplementing] {
		t.Fatal("IMPLEMENTING should be reachable from HALTED via the human recovery path")
	}

	recovery := []TaskState{
		StateHumanReview,
		StateContractRevised,
		StateAwaitingApproval,
		StateApproved,
	}
	beyond := []TaskState{
		StateImplementing,
		StateVerifying,
		StateCorrecting,
		StateReviewing,
		StateAwaitingMerge,
		StateHumanApproval,
		StateMerged,
	}
	for _, step := range recovery {
		got := reachable(StateHalted, step)
		for _, s := range beyond {
			if got[s] {
				t.Errorf("%s is reachable from HALTED without passing through %s", s, step)
			}
		}
	}
}
