package domain

// Outcome is the result of an authorization evaluation.
type Outcome string

const (
	OutcomeAllow            Outcome = "ALLOW"
	OutcomeDeny             Outcome = "DENY"
	OutcomeRequiresApproval Outcome = "REQUIRES_APPROVAL"
)

// Reason is the machine-readable cause of an Outcome. Security decisions
// depend on Outcome and Reason, never on free-form text.
type Reason string

const (
	ReasonAllowed                     Reason = "ALLOWED"
	ReasonDeniedUnknownActor          Reason = "DENIED_UNKNOWN_ACTOR"
	ReasonDeniedUnknownAction         Reason = "DENIED_UNKNOWN_ACTION"
	ReasonDeniedInsufficientAuthority Reason = "DENIED_INSUFFICIENT_AUTHORITY"
	ReasonDeniedInvalidContext        Reason = "DENIED_INVALID_CONTEXT"
	ReasonRequiresHumanApproval       Reason = "REQUIRES_HUMAN_APPROVAL"
)

// Decision is the evidence of one authorization evaluation. It records who
// asked, for what, on which task, and which approval (if any) was relied on,
// so it can later be stored without reconstructing authority from agent
// output. A Decision is a record, not a credential: nothing in this package
// trusts a Decision value handed back to it. A DENY is not by itself a
// contract violation.
type Decision struct {
	Outcome    Outcome
	Reason     Reason
	ActorID    ActorID
	Action     Action
	TaskID     TaskID
	ApprovalID ApprovalID
}

// IsAllowed reports whether the outcome is ALLOW. The zero Decision is not
// allowed.
func (d Decision) IsAllowed() bool { return d.Outcome == OutcomeAllow }
