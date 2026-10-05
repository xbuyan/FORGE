package domain

import "strings"

// Request is the context of one authorization question: which trusted actor
// wants to do what on which task, and which human approval (if any) FORGE
// holds for it. Evaluate only decides; it never performs the action and never
// changes any state.
type Request struct {
	Actor    TrustedActor
	Action   Action
	TaskID   TaskID
	Approval Approval
}

// Evaluate applies the Gate 2 action policy. It is a pure, deny-by-default
// evaluation: an invalid actor, unknown action or invalid context is DENY, and
// so is anything the policy does not explicitly grant. Role alone never grants
// authority; it only selects a row of the policy. The returned Decision is a
// record, not a credential.
func Evaluate(req Request) Decision {
	d := Decision{
		Outcome: OutcomeDeny,
		ActorID: req.Actor.ID(),
		Action:  req.Action,
		TaskID:  req.TaskID,
	}
	switch {
	case !req.Actor.IsValid():
		d.Reason = ReasonDeniedUnknownActor
		return d
	case !req.Action.IsValid():
		d.Reason = ReasonDeniedUnknownAction
		return d
	case strings.TrimSpace(string(req.TaskID)) == "":
		d.Reason = ReasonDeniedInvalidContext
		return d
	}

	switch req.Actor.Kind() {
	case ActorKindHuman:
		d.Outcome, d.Reason = OutcomeAllow, ReasonAllowed
	case ActorKindAgent:
		evaluateAgent(req, &d)
	default:
		// System, and anything unforeseen, has no granted action authority.
		d.Reason = ReasonDeniedInsufficientAuthority
	}
	return d
}

func evaluateAgent(req Request, d *Decision) {
	role := req.Actor.Role()
	if !role.IsValid() {
		d.Reason = ReasonDeniedInsufficientAuthority
		return
	}

	if req.Action.IsConsequential() {
		if req.Approval.covers(req.TaskID, req.Action, req.Actor) {
			d.Outcome, d.Reason, d.ApprovalID = OutcomeAllow, ReasonAllowed, req.Approval.ID()
			return
		}
		d.Outcome, d.Reason = OutcomeRequiresApproval, ReasonRequiresHumanApproval
		return
	}

	if agentMayUseOrdinary(role, req.Action) {
		d.Outcome, d.Reason = OutcomeAllow, ReasonAllowed
		return
	}
	d.Reason = ReasonDeniedInsufficientAuthority
}

// agentMayUseOrdinary is the ordinary-capability policy for agents.
func agentMayUseOrdinary(role Role, action Action) bool {
	switch role {
	case RoleEngineer:
		switch action {
		case ActionReadFile, ActionWriteFile, ActionDeleteFile, ActionListFiles,
			ActionRunTest, ActionGitStatus, ActionGitDiff:
			return true
		}
	case RoleArchitect, RoleVerifier:
		switch action {
		case ActionReadFile, ActionListFiles, ActionRunTest, ActionGitStatus, ActionGitDiff:
			return true
		}
	}
	return false
}
