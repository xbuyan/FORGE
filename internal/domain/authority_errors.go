package domain

import "errors"

// Sentinel errors for the authority model. Callers distinguish them with
// errors.Is.
var (
	ErrInvalidActorID   = errors.New("invalid actor id")
	ErrInvalidActorKind = errors.New("invalid actor kind")
	ErrInvalidActor     = errors.New("invalid actor")
	ErrInvalidApproval  = errors.New("invalid approval")
	ErrInvalidApprover  = errors.New("approver must be a valid human actor")
	ErrNotAuthorized    = errors.New("operation not authorized")
)
