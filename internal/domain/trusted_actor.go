package domain

import "fmt"

// TrustedActor is an authority-bearing actor context. Authorization and
// approval functions accept only a TrustedActor, never an ActorAssertion. The
// zero TrustedActor is invalid and has no authority. Its fields are
// unexported, so the only way to obtain a valid one is IssueTrustedActor.
//
// Limitation, stated plainly: Gate 2 establishes a trusted-context boundary
// but does not authenticate a real external human. Nothing in this package
// proves that a Human TrustedActor is a person. Go cannot stop code in the
// same process from calling IssueTrustedActor, so the boundary is held by two
// things: the type system (assertions cannot be used as authority) and an
// architecture test that restricts which production packages may reference
// IssueTrustedActor. The intended issuer is a future authentication
// subsystem.
type TrustedActor struct {
	id     ActorID
	kind   ActorKind
	role   Role
	issued bool
}

// IssueTrustedActor is the issuer seam. It converts a valid assertion into a
// TrustedActor and performs no authentication. It must be called only by the
// future authentication subsystem and by test helpers; the architecture test
// in this package fails if any other production code references it. Agent
// adapters and any code handling agent output must never call it.
func IssueTrustedActor(a ActorAssertion) (TrustedActor, error) {
	if !a.IsValid() {
		return TrustedActor{}, fmt.Errorf("%w: assertion is not valid", ErrInvalidActor)
	}
	return TrustedActor{id: a.id, kind: a.kind, role: a.role, issued: true}, nil
}

// ID returns the actor's identity.
func (a TrustedActor) ID() ActorID { return a.id }

// Kind returns the actor's kind.
func (a TrustedActor) Kind() ActorKind { return a.kind }

// Role returns the actor's engineering role, or the empty Role for none.
func (a TrustedActor) Role() Role { return a.role }

// IsValid reports whether the actor was issued and its fields are valid. The
// zero TrustedActor is not valid.
func (a TrustedActor) IsValid() bool {
	return a.issued && validActorFields(a.id, a.kind, a.role)
}
