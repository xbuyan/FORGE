package domain

import "slices"

// transitionEdge is one lifecycle move.
type transitionEdge struct {
	from, to TaskState
}

// transitionPermitted is the explicit transition authority table. A move is
// permitted only if it is listed for the actor's kind; everything else is
// denied. Agents, whatever their role, hold no lifecycle authority in Gate 2,
// and unknown kinds hold none either. Four moves are listed for nobody:
// VERIFYING->REVIEWING, REVIEWING->CORRECTING and REVIEWING->AWAITING_MERGE
// (they wait for an independent verification/review gate) and
// HUMAN_APPROVAL->MERGED (MERGED must reflect a merge FORGE verified, which is
// a future gate).
func transitionPermitted(kind ActorKind, from, to TaskState) bool {
	e := transitionEdge{from: from, to: to}
	switch kind {
	case ActorKindHuman:
		return slices.Contains(humanTransitions(), e)
	case ActorKindSystem:
		return slices.Contains(systemTransitions(), e)
	default:
		return false
	}
}

// humanTransitions lists the moves a Human actor may request. A fresh slice is
// returned on every call so the table cannot be altered at run time.
func humanTransitions() []transitionEdge {
	return []transitionEdge{
		{StateAwaitingApproval, StateApproved},
		{StateImplementing, StateHalted},
		{StateVerifying, StateHalted},
		{StateCorrecting, StateHalted},
		{StateReviewing, StateHalted},
		{StateAwaitingMerge, StateHalted},
		{StateHumanApproval, StateHalted},
		{StateHumanReview, StateContractRevised},
	}
}

// systemTransitions lists the FORGE-internal moves a System actor may request.
func systemTransitions() []transitionEdge {
	return []transitionEdge{
		{StateDraft, StateInspecting},
		{StateInspecting, StateContractReady},
		{StateContractReady, StateAwaitingApproval},
		{StateApproved, StateImplementing},
		{StateImplementing, StateVerifying},
		{StateImplementing, StateHalted},
		{StateVerifying, StateCorrecting},
		{StateVerifying, StateHalted},
		{StateCorrecting, StateVerifying},
		{StateCorrecting, StateHalted},
		{StateReviewing, StateHalted},
		{StateAwaitingMerge, StateHumanApproval},
		{StateAwaitingMerge, StateHalted},
		{StateHumanApproval, StateHalted},
		{StateHalted, StateHumanReview},
		{StateContractRevised, StateAwaitingApproval},
	}
}
