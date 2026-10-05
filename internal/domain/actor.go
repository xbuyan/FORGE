package domain

import (
	"fmt"
	"strings"
)

// ActorID is the identity an actor asserts for itself. It is a label, not an
// authenticated identity.
type ActorID string

// ActorKind says what kind of actor something claims to be. It is
// provider-neutral. Claiming a kind proves nothing.
type ActorKind string

const (
	ActorKindHuman  ActorKind = "HUMAN"
	ActorKindAgent  ActorKind = "AGENT"
	ActorKindSystem ActorKind = "SYSTEM"
)

// IsValid reports whether k is a declared kind. The zero value and any
// unrecognised string are invalid.
func (k ActorKind) IsValid() bool {
	switch k {
	case ActorKindHuman, ActorKindAgent, ActorKindSystem:
		return true
	default:
		return false
	}
}

func validActorFields(id ActorID, kind ActorKind, role Role) bool {
	if strings.TrimSpace(string(id)) == "" || !kind.IsValid() {
		return false
	}
	if role != "" && !role.IsValid() {
		return false
	}
	return !(kind == ActorKindSystem && role != "")
}

// ActorAssertion is what a caller claims about an actor: an id, a kind and a
// role. It carries no authority. An asserted ActorKindHuman does not prove that
// the caller is human or authenticated, and no authorization or approval
// function accepts an ActorAssertion: they require a TrustedActor.
type ActorAssertion struct {
	id   ActorID
	kind ActorKind
	role Role
}

// NewActorAssertion validates and returns an ActorAssertion. A blank id, an
// unknown kind, or a non-empty unknown role is rejected. An empty role means
// "no engineering role". A System actor must not have a role.
func NewActorAssertion(id ActorID, kind ActorKind, role Role) (ActorAssertion, error) {
	if strings.TrimSpace(string(id)) == "" {
		return ActorAssertion{}, fmt.Errorf("%w: must not be blank", ErrInvalidActorID)
	}
	if !kind.IsValid() {
		return ActorAssertion{}, fmt.Errorf("%w: %q", ErrInvalidActorKind, kind)
	}
	if role != "" && !role.IsValid() {
		return ActorAssertion{}, fmt.Errorf("%w: %q", ErrInvalidRole, role)
	}
	if kind == ActorKindSystem && role != "" {
		return ActorAssertion{}, fmt.Errorf("%w: a system actor has no role", ErrInvalidActor)
	}
	return ActorAssertion{id: id, kind: kind, role: role}, nil
}

// ID returns the asserted identity.
func (a ActorAssertion) ID() ActorID { return a.id }

// Kind returns the asserted kind.
func (a ActorAssertion) Kind() ActorKind { return a.kind }

// Role returns the asserted role, or the empty Role for none.
func (a ActorAssertion) Role() Role { return a.role }

// IsValid reports whether the assertion satisfies every NewActorAssertion
// rule. The zero ActorAssertion is not valid.
func (a ActorAssertion) IsValid() bool {
	return validActorFields(a.id, a.kind, a.role)
}
