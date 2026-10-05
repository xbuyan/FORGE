package domain

import (
	"strings"
	"sync/atomic"
)

// authGrant is the shared, single-use state behind an AuthorizedTransition. It
// is only ever used through a pointer and must never be copied.
type authGrant struct {
	task     *taskImpl
	from, to TaskState
	actor    TrustedActor
	consumed atomic.Bool
}

// AuthorizedTransition is the lifecycle capability: proof that
// AuthorizeTransition allowed one specific move of one specific Task instance
// for one specific TrustedActor. Its state is unexported, so other packages
// cannot construct one, and the zero value is rejected by Apply. It is bound to
// the task instance, the actor, the from-state and the target state, and it is
// single use: copies share the same use, and the use is concurrency-safe.
//
// No Gate 2 transition depends on a human approval, so a proof carries no
// approval binding yet.
type AuthorizedTransition struct {
	g *authGrant
}

// TransitionRequest asks to move Task to To on behalf of Actor.
type TransitionRequest struct {
	Actor TrustedActor
	Task  Task
	To    TaskState
}

// AuthorizeTransition answers "may this trusted actor request this move?" using
// the explicit transition authority table. Anything the table does not list is
// denied, including moves the lifecycle forbids. On anything other than ALLOW
// the returned proof is the unusable zero value. Task.Apply still validates the
// lifecycle independently.
func AuthorizeTransition(req TransitionRequest) (AuthorizedTransition, Decision) {
	d := Decision{Outcome: OutcomeDeny, ActorID: req.Actor.ID()}

	if !req.Actor.IsValid() {
		d.Reason = ReasonDeniedUnknownActor
		return AuthorizedTransition{}, d
	}
	tk, ok := req.Task.(*taskImpl)
	if !ok || tk == nil {
		d.Reason = ReasonDeniedInvalidContext
		return AuthorizedTransition{}, d
	}
	id, from := tk.snapshot()
	if strings.TrimSpace(string(id)) == "" || !from.IsValid() || !req.To.IsValid() {
		d.Reason = ReasonDeniedInvalidContext
		return AuthorizedTransition{}, d
	}
	d.TaskID = id

	if !transitionPermitted(req.Actor.Kind(), from, req.To) {
		d.Reason = ReasonDeniedInsufficientAuthority
		return AuthorizedTransition{}, d
	}

	d.Outcome, d.Reason = OutcomeAllow, ReasonAllowed
	return AuthorizedTransition{g: &authGrant{
		task:  tk,
		from:  from,
		to:    req.To,
		actor: req.Actor,
	}}, d
}
