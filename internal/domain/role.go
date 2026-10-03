package domain

import "fmt"

// Role is a provider-neutral engineering role. A Role carries no authority;
// what a role may do is decided by a later policy layer, never by this type.
type Role string

const (
	RoleArchitect Role = "ARCHITECT"
	RoleEngineer  Role = "ENGINEER"
	RoleVerifier  Role = "VERIFIER"
)

// IsValid reports whether r is one of the declared roles. The zero value and
// any unrecognised string are invalid.
func (r Role) IsValid() bool {
	switch r {
	case RoleArchitect, RoleEngineer, RoleVerifier:
		return true
	default:
		return false
	}
}

// ParseRole converts s to a Role. Matching is exact; anything that is not a
// declared role returns ErrInvalidRole and the zero Role.
func ParseRole(s string) (Role, error) {
	r := Role(s)
	if !r.IsValid() {
		return "", fmt.Errorf("%w: %q", ErrInvalidRole, s)
	}
	return r, nil
}
