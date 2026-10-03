package domain

import "slices"

// TaskState is the lifecycle state of a Task. Only the values declared below
// are valid; any other value is rejected rather than interpreted.
type TaskState string

const (
	StateDraft            TaskState = "DRAFT"
	StateInspecting       TaskState = "INSPECTING"
	StateContractReady    TaskState = "CONTRACT_READY"
	StateAwaitingApproval TaskState = "AWAITING_APPROVAL"
	StateApproved         TaskState = "APPROVED"
	StateImplementing     TaskState = "IMPLEMENTING"
	StateVerifying        TaskState = "VERIFYING"
	StateCorrecting       TaskState = "CORRECTING"
	StateReviewing        TaskState = "REVIEWING"
	StateAwaitingMerge    TaskState = "AWAITING_MERGE"
	StateHumanApproval    TaskState = "HUMAN_APPROVAL"
	StateMerged           TaskState = "MERGED"
	StateHalted           TaskState = "HALTED"
	StateHumanReview      TaskState = "HUMAN_REVIEW"
	StateContractRevised  TaskState = "CONTRACT_REVISED"
)

// IsValid reports whether s is one of the declared states.
func (s TaskState) IsValid() bool {
	switch s {
	case StateDraft,
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
		StateContractRevised:
		return true
	default:
		return false
	}
}

// IsTerminal reports whether no transition may leave s.
func (s TaskState) IsTerminal() bool {
	return s == StateMerged
}

// CanTransition reports whether the lifecycle permits moving directly from
// one state to another. It is deny-by-default: unknown states, unknown
// targets, and any pair not listed in allowedFrom all return false.
func CanTransition(from, to TaskState) bool {
	return slices.Contains(allowedFrom(from), to)
}

// allowedFrom is the complete transition table. A fresh slice is returned on
// every call so the table cannot be altered at run time. States without a
// case (including unknown values) have no outgoing transitions.
func allowedFrom(s TaskState) []TaskState {
	switch s {
	case StateDraft:
		return []TaskState{StateInspecting}
	case StateInspecting:
		return []TaskState{StateContractReady}
	case StateContractReady:
		return []TaskState{StateAwaitingApproval}
	case StateAwaitingApproval:
		return []TaskState{StateApproved}
	case StateApproved:
		return []TaskState{StateImplementing}
	case StateImplementing:
		return []TaskState{StateVerifying, StateHalted}
	case StateVerifying:
		return []TaskState{StateCorrecting, StateReviewing, StateHalted}
	case StateCorrecting:
		return []TaskState{StateVerifying, StateHalted}
	case StateReviewing:
		return []TaskState{StateCorrecting, StateAwaitingMerge, StateHalted}
	case StateAwaitingMerge:
		return []TaskState{StateHumanApproval, StateHalted}
	case StateHumanApproval:
		return []TaskState{StateMerged, StateHalted}
	case StateHalted:
		return []TaskState{StateHumanReview}
	case StateHumanReview:
		return []TaskState{StateContractRevised}
	case StateContractRevised:
		return []TaskState{StateAwaitingApproval}
	case StateMerged:
		return nil
	default:
		return nil
	}
}
