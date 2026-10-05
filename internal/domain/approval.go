package domain

import (
	"fmt"
	"strings"
)

// ApprovalID identifies an Approval.
type ApprovalID string

// Approval is delegated human authority for one action by one requesting actor
// on one task. It is a domain value, never a string, and it is bound to an
// approval id, a task id, an action, the approving Human and the requesting
// actor. An approval for another task, action or requester does not apply. Its
// fields are unexported, so it can only be made by NewApproval. The zero
// Approval is empty and covers nothing.
//
// Deferred, by architecture decision: expiry, revocation, signatures,
// persistence, and consumption of an approval on use. Evaluate is a pure
// policy evaluation and does not consume approvals; consumption belongs to a
// future executable-action authorization capability.
type Approval struct {
	id            ApprovalID
	taskID        TaskID
	action        Action
	approverID    ActorID
	requesterID   ActorID
	requesterKind ActorKind
}

// NewApproval records a human approval. The approver must be a valid Human
// TrustedActor; Agent, System and invalid actors are rejected. The requester
// must be a valid TrustedActor. Agent-generated text is never an input.
func NewApproval(id ApprovalID, taskID TaskID, action Action, approver TrustedActor, requester TrustedActor) (Approval, error) {
	if !approver.IsValid() || approver.Kind() != ActorKindHuman {
		return Approval{}, fmt.Errorf("%w: got kind %q", ErrInvalidApprover, approver.Kind())
	}
	if !requester.IsValid() {
		return Approval{}, fmt.Errorf("%w: requesting actor is not valid", ErrInvalidApproval)
	}
	if strings.TrimSpace(string(id)) == "" {
		return Approval{}, fmt.Errorf("%w: blank approval id", ErrInvalidApproval)
	}
	if strings.TrimSpace(string(taskID)) == "" {
		return Approval{}, fmt.Errorf("%w: blank task id", ErrInvalidApproval)
	}
	if !action.IsValid() {
		return Approval{}, fmt.Errorf("%w: unknown action %q", ErrInvalidApproval, action)
	}
	return Approval{
		id:            id,
		taskID:        taskID,
		action:        action,
		approverID:    approver.ID(),
		requesterID:   requester.ID(),
		requesterKind: requester.Kind(),
	}, nil
}

// ID returns the approval's identity.
func (a Approval) ID() ApprovalID { return a.id }

// TaskID returns the task the approval is bound to.
func (a Approval) TaskID() TaskID { return a.taskID }

// Action returns the action the approval is bound to.
func (a Approval) Action() Action { return a.action }

// ApproverID returns the identity of the human who approved.
func (a Approval) ApproverID() ActorID { return a.approverID }

// ApproverKind is always ActorKindHuman for a real approval.
func (a Approval) ApproverKind() ActorKind { return ActorKindHuman }

// RequesterID returns the identity of the actor the approval was given to.
func (a Approval) RequesterID() ActorID { return a.requesterID }

// RequesterKind returns the kind of the actor the approval was given to.
func (a Approval) RequesterKind() ActorKind { return a.requesterKind }

// IsZero reports whether a is the empty (absent) approval.
func (a Approval) IsZero() bool { return strings.TrimSpace(string(a.id)) == "" }

// covers reports whether a approves exactly this action on this task for this
// requesting actor.
func (a Approval) covers(taskID TaskID, action Action, requester TrustedActor) bool {
	return !a.IsZero() &&
		a.taskID == taskID &&
		a.action == action &&
		a.requesterID == requester.ID() &&
		a.requesterKind == requester.Kind()
}
