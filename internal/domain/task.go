// Package domain holds FORGE's provider-neutral domain model: tasks, task
// lifecycle states, engineering roles, and the authority boundary that decides
// who may request which operation. It is pure in-memory logic with no
// persistence, network, Git, shell, or model-provider dependencies.
package domain

import (
	"fmt"
	"strings"
	"sync"
)

// TaskID is the textual label of a Task. It is not an identity proof: two
// Tasks may carry the same TaskID, and authorization is bound to the Task
// instance, never to this string.
type TaskID string

// Task is the central unit of engineering work. It is a sealed interface: the
// only implementation is unexported, so code outside this package cannot
// obtain, copy, or assign the concrete value that holds the lifecycle state.
// Copying a Task copies a reference to the same task, not a snapshot of its
// state. State changes only through Apply, which requires an
// AuthorizedTransition and still validates the move against the lifecycle.
// All methods are safe for concurrent use.
//
// Limitation: this protects against ordinary Go code. It does not protect
// against reflection or unsafe code in the same process.
type Task interface {
	ID() TaskID
	State() TaskState
	Apply(actor TrustedActor, proof AuthorizedTransition) error
	sealed()
}

// taskImpl is the only Task implementation. Its mutex serializes every state
// change and read.
type taskImpl struct {
	mu    sync.Mutex
	id    TaskID
	state TaskState
}

func newTaskImpl(id TaskID) (*taskImpl, error) {
	if strings.TrimSpace(string(id)) == "" {
		return nil, fmt.Errorf("%w: must not be blank", ErrInvalidTaskID)
	}
	return &taskImpl{id: id, state: StateDraft}, nil
}

// NewTask returns a Task in StateDraft. A blank id is rejected.
func NewTask(id TaskID) (Task, error) {
	t, err := newTaskImpl(id)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (t *taskImpl) sealed() {}

// ID returns the task's label.
func (t *taskImpl) ID() TaskID {
	return t.id
}

// State returns the task's current lifecycle state.
func (t *taskImpl) State() TaskState {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state
}

// snapshot returns the task's label and state atomically.
func (t *taskImpl) snapshot() (TaskID, TaskState) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.id, t.state
}

// transition moves the task to next if the lifecycle permits it. On any error
// the task is left unchanged. Errors wrap ErrInvalidState (current or
// requested state is not a declared state) or ErrInvalidTransition (the move
// is not in the lifecycle); a move out of a terminal state also wraps
// ErrTerminalState.
//
// transition validates the lifecycle only. It does not identify its caller and
// does not hold the task lock: the caller must. The only production caller is
// Apply, which requires an AuthorizedTransition.
func (t *taskImpl) transition(next TaskState) error {
	if !t.state.IsValid() {
		return fmt.Errorf("%w: current state %q", ErrInvalidState, t.state)
	}
	if !next.IsValid() {
		return fmt.Errorf("%w: requested state %q", ErrInvalidState, next)
	}
	if t.state.IsTerminal() {
		return fmt.Errorf("%w: %w: %s is terminal", ErrInvalidTransition, ErrTerminalState, t.state)
	}
	if !CanTransition(t.state, next) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, t.state, next)
	}
	t.state = next
	return nil
}

// Apply performs one authorized lifecycle move. It fails with ErrNotAuthorized
// if the proof is the zero value, was issued for a different Task instance or
// a different actor, was already used, or was issued for a state the task is
// no longer in. Otherwise the move still passes through lifecycle validation,
// so an illegal move is rejected and the task is left unchanged. Check, move
// and consumption happen under one lock, so a proof succeeds at most once even
// under concurrent use, and competing proofs cannot both move the same state.
func (t *taskImpl) Apply(actor TrustedActor, proof AuthorizedTransition) error {
	if t == nil || proof.g == nil || proof.g.task != t || !actor.IsValid() {
		return ErrNotAuthorized
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	g := proof.g
	if g.consumed.Load() || g.actor != actor || g.from != t.state {
		return ErrNotAuthorized
	}
	if err := t.transition(g.to); err != nil {
		return err
	}
	g.consumed.Store(true)
	return nil
}
