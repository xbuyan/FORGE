// Package domain holds FORGE's provider-neutral domain model: tasks, task
// lifecycle states, and engineering roles. It is pure in-memory logic with no
// persistence, network, Git, shell, or model-provider dependencies.
package domain

import (
	"fmt"
	"strings"
)

// TaskID is the stable identity of a Task.
type TaskID string

// Task is the central unit of engineering work. Its state is unexported so it
// can only change through Transition, which validates every move. Task is not
// safe for concurrent use.
type Task struct {
	id    TaskID
	state TaskState
}

// NewTask returns a Task in StateDraft. A blank id is rejected.
func NewTask(id TaskID) (*Task, error) {
	if strings.TrimSpace(string(id)) == "" {
		return nil, fmt.Errorf("%w: must not be blank", ErrInvalidTaskID)
	}
	return &Task{id: id, state: StateDraft}, nil
}

// ID returns the task's identity.
func (t *Task) ID() TaskID {
	return t.id
}

// State returns the task's current lifecycle state. A zero-value Task has the
// invalid empty state and cannot be transitioned.
func (t *Task) State() TaskState {
	return t.state
}

// Transition moves the task to next if the lifecycle permits it. On any error
// the task is left unchanged. Errors wrap ErrInvalidState (current or
// requested state is not a declared state) or ErrInvalidTransition (the move
// is not in the lifecycle); a move out of a terminal state also wraps
// ErrTerminalState.
//
// Transition does not identify its caller. Which actor may request a given
// transition is decided outside this package.
func (t *Task) Transition(next TaskState) error {
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
