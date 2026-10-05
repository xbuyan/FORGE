package domain_test

import (
	"errors"
	"go/ast"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/xbuyan/FORGE/internal/domain"
)

// These tests use only the exported API, as any other package would.

var allStates = []domain.TaskState{
	domain.StateDraft, domain.StateInspecting, domain.StateContractReady,
	domain.StateAwaitingApproval, domain.StateApproved, domain.StateImplementing,
	domain.StateVerifying, domain.StateCorrecting, domain.StateReviewing,
	domain.StateAwaitingMerge, domain.StateHumanApproval, domain.StateMerged,
	domain.StateHalted, domain.StateHumanReview, domain.StateContractRevised,
}

// trusted is the test-only issuer. Production code must not call
// IssueTrustedActor; TestTrustedActorIssuerBoundary enforces that.
func trusted(t *testing.T, id domain.ActorID, kind domain.ActorKind, role domain.Role) domain.TrustedActor {
	t.Helper()
	a, err := domain.NewActorAssertion(id, kind, role)
	if err != nil {
		t.Fatalf("NewActorAssertion: %v", err)
	}
	ta, err := domain.IssueTrustedActor(a)
	if err != nil {
		t.Fatalf("IssueTrustedActor: %v", err)
	}
	return ta
}

func step(t *testing.T, task domain.Task, actor domain.TrustedActor, to domain.TaskState) {
	t.Helper()
	proof, d := domain.AuthorizeTransition(domain.TransitionRequest{Actor: actor, Task: task, To: to})
	if !d.IsAllowed() {
		t.Fatalf("-> %s not authorized: %s/%s", to, d.Outcome, d.Reason)
	}
	if err := task.Apply(actor, proof); err != nil {
		t.Fatalf("-> %s: Apply: %v", to, err)
	}
}

// driveToVerifying builds a task and walks it to VERIFYING through the public
// API only, returning the task, a System actor and a Human actor.
func driveToVerifying(t *testing.T) (domain.Task, domain.TrustedActor, domain.TrustedActor) {
	t.Helper()
	task, err := domain.NewTask("task-1")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	system := trusted(t, "forge-1", domain.ActorKindSystem, "")
	human := trusted(t, "human-1", domain.ActorKindHuman, "")
	step(t, task, system, domain.StateInspecting)
	step(t, task, system, domain.StateContractReady)
	step(t, task, system, domain.StateAwaitingApproval)
	step(t, task, human, domain.StateApproved)
	step(t, task, system, domain.StateImplementing)
	step(t, task, system, domain.StateVerifying)
	return task, system, human
}

func TestTaskInterfaceIsSealedAndOpaque(t *testing.T) {
	tt := reflect.TypeOf((*domain.Task)(nil)).Elem()
	if tt.Kind() != reflect.Interface {
		t.Fatalf("Task must be an interface, got %s", tt.Kind())
	}
	var exported, unexported []string
	for i := 0; i < tt.NumMethod(); i++ {
		m := tt.Method(i)
		if m.PkgPath == "" {
			exported = append(exported, m.Name)
		} else {
			unexported = append(unexported, m.Name)
		}
	}
	slices.Sort(exported)
	if want := []string{"Apply", "ID", "State"}; !slices.Equal(exported, want) {
		t.Errorf("Task exported methods = %v, want %v", exported, want)
	}
	if len(unexported) != 1 {
		t.Errorf("Task must have exactly one unexported (sealing) method, has %v", unexported)
	}

	task, err := domain.NewTask("task-1")
	if err != nil {
		t.Fatal(err)
	}
	dt := reflect.TypeOf(task)
	if dt.Kind() != reflect.Ptr || ast.IsExported(dt.Elem().Name()) {
		t.Errorf("the concrete Task type must be an unexported pointer type, got %s", dt)
	}
}

// Copying a Task copies a reference, not a snapshot of its state, so a saved
// copy cannot be used to restore an earlier state.
func TestCopyingATaskDoesNotSnapshotIt(t *testing.T) {
	system := trusted(t, "forge-1", domain.ActorKindSystem, "")
	task, err := domain.NewTask("task-1")
	if err != nil {
		t.Fatal(err)
	}
	saved := task
	step(t, task, system, domain.StateInspecting)
	if saved.State() != domain.StateInspecting {
		t.Errorf("a copied Task kept an old state: %s", saved.State())
	}
}

func TestAgentsCannotAdvanceATask(t *testing.T) {
	for _, role := range []domain.Role{domain.RoleArchitect, domain.RoleEngineer, domain.RoleVerifier} {
		agent := trusted(t, "agent-1", domain.ActorKindAgent, role)
		task, err := domain.NewTask("task-1")
		if err != nil {
			t.Fatal(err)
		}
		proof, d := domain.AuthorizeTransition(domain.TransitionRequest{Actor: agent, Task: task, To: domain.StateInspecting})
		if d.Outcome != domain.OutcomeDeny {
			t.Errorf("agent %s: %s, want DENY", role, d.Outcome)
		}
		if err := task.Apply(agent, proof); !errors.Is(err, domain.ErrNotAuthorized) {
			t.Errorf("agent %s: Apply = %v, want ErrNotAuthorized", role, err)
		}
		if err := task.Apply(agent, domain.AuthorizedTransition{}); !errors.Is(err, domain.ErrNotAuthorized) {
			t.Errorf("agent %s: Apply(zero proof) = %v, want ErrNotAuthorized", role, err)
		}
		if task.State() != domain.StateDraft {
			t.Errorf("agent %s changed state to %s", role, task.State())
		}
	}
}

type move struct {
	actor domain.TrustedActor
	to    domain.TaskState
}

func replay(t *testing.T, path []move) domain.Task {
	t.Helper()
	task, err := domain.NewTask("task-1")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range path {
		step(t, task, m.actor, m.to)
	}
	return task
}

// Using every actor category and only the public API, MERGED and the states
// that wait for the future review and merge gates are unreachable.
func TestReachableStatesViaPublicAPI(t *testing.T) {
	actors := []domain.TrustedActor{
		trusted(t, "human-1", domain.ActorKindHuman, ""),
		trusted(t, "forge-1", domain.ActorKindSystem, ""),
		trusted(t, "arch-1", domain.ActorKindAgent, domain.RoleArchitect),
		trusted(t, "eng-1", domain.ActorKindAgent, domain.RoleEngineer),
		trusted(t, "ver-1", domain.ActorKindAgent, domain.RoleVerifier),
	}
	paths := map[domain.TaskState][]move{domain.StateDraft: nil}
	queue := []domain.TaskState{domain.StateDraft}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, to := range allStates {
			if _, seen := paths[to]; seen {
				continue
			}
			for _, actor := range actors {
				task := replay(t, paths[cur])
				proof, d := domain.AuthorizeTransition(domain.TransitionRequest{Actor: actor, Task: task, To: to})
				if !d.IsAllowed() {
					continue
				}
				if err := task.Apply(actor, proof); err != nil {
					t.Fatalf("%s -> %s: Apply: %v", cur, to, err)
				}
				paths[to] = append(append([]move{}, paths[cur]...), move{actor: actor, to: to})
				queue = append(queue, to)
				break
			}
		}
	}

	for _, s := range []domain.TaskState{
		domain.StateReviewing, domain.StateAwaitingMerge, domain.StateHumanApproval, domain.StateMerged,
	} {
		if _, reachable := paths[s]; reachable {
			t.Errorf("%s must be unreachable through the public API in Gate 2", s)
		}
	}
	for _, s := range []domain.TaskState{
		domain.StateDraft, domain.StateInspecting, domain.StateContractReady,
		domain.StateAwaitingApproval, domain.StateApproved, domain.StateImplementing,
		domain.StateVerifying, domain.StateCorrecting, domain.StateHalted,
		domain.StateHumanReview, domain.StateContractRevised,
	} {
		if _, reachable := paths[s]; !reachable {
			t.Errorf("%s should be reachable through the public API", s)
		}
	}
}

func TestZeroProofIsRejectedExternally(t *testing.T) {
	task, system, _ := driveToVerifying(t)
	if err := task.Apply(system, domain.AuthorizedTransition{}); !errors.Is(err, domain.ErrNotAuthorized) {
		t.Errorf("Apply(zero) = %v, want ErrNotAuthorized", err)
	}
	if task.State() != domain.StateVerifying {
		t.Errorf("state = %s, want VERIFYING", task.State())
	}
}

// Many goroutines using one proof: exactly one success, all replays fail.
func TestConcurrentReuseOfOneProof(t *testing.T) {
	const goroutines = 64
	task, system, _ := driveToVerifying(t)
	proof, d := domain.AuthorizeTransition(domain.TransitionRequest{Actor: system, Task: task, To: domain.StateCorrecting})
	if !d.IsAllowed() {
		t.Fatalf("not authorized: %+v", d)
	}

	var wg sync.WaitGroup
	var successes atomic.Int32
	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			switch err := task.Apply(system, proof); {
			case err == nil:
				successes.Add(1)
			case !errors.Is(err, domain.ErrNotAuthorized):
				t.Errorf("replay failed with %v, want ErrNotAuthorized", err)
			}
		}()
	}
	close(start)
	wg.Wait()

	if got := successes.Load(); got != 1 {
		t.Fatalf("%d concurrent applications succeeded, want exactly 1", got)
	}
	if task.State() != domain.StateCorrecting {
		t.Errorf("state = %s, want CORRECTING", task.State())
	}
}

// Two valid proofs for different moves from the same state race: exactly one
// wins and the other fails.
func TestCompetingProofsOnlyOneWins(t *testing.T) {
	for i := 0; i < 200; i++ {
		task, system, human := driveToVerifying(t)
		toCorrecting, dA := domain.AuthorizeTransition(domain.TransitionRequest{Actor: system, Task: task, To: domain.StateCorrecting})
		toHalted, dB := domain.AuthorizeTransition(domain.TransitionRequest{Actor: human, Task: task, To: domain.StateHalted})
		if !dA.IsAllowed() || !dB.IsAllowed() {
			t.Fatalf("setup: not authorized: %+v / %+v", dA, dB)
		}

		var wg sync.WaitGroup
		var wins atomic.Int32
		start := make(chan struct{})
		attempt := func(actor domain.TrustedActor, proof domain.AuthorizedTransition) {
			defer wg.Done()
			<-start
			switch err := task.Apply(actor, proof); {
			case err == nil:
				wins.Add(1)
			case !errors.Is(err, domain.ErrNotAuthorized):
				t.Errorf("loser failed with %v, want ErrNotAuthorized", err)
			}
		}
		for j := 0; j < 4; j++ {
			wg.Add(2)
			go attempt(system, toCorrecting)
			go attempt(human, toHalted)
		}
		close(start)
		wg.Wait()

		if got := wins.Load(); got != 1 {
			t.Fatalf("iteration %d: %d applications succeeded, want exactly 1", i, got)
		}
		if st := task.State(); st != domain.StateCorrecting && st != domain.StateHalted {
			t.Fatalf("iteration %d: unexpected final state %s", i, st)
		}
	}
}
