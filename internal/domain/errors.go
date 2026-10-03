package domain

import "errors"

// Sentinel errors for the domain package. Callers distinguish them with
// errors.Is.
var (
	ErrInvalidTaskID     = errors.New("invalid task id")
	ErrInvalidState      = errors.New("invalid task state")
	ErrInvalidRole       = errors.New("invalid role")
	ErrInvalidTransition = errors.New("invalid state transition")
	ErrTerminalState     = errors.New("task is in a terminal state")
)
