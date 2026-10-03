package domain

import (
	"errors"
	"testing"
)

func TestValidRoles(t *testing.T) {
	roles := []Role{RoleArchitect, RoleEngineer, RoleVerifier}
	seen := make(map[Role]bool)
	for _, r := range roles {
		if !r.IsValid() {
			t.Errorf("%q should be a valid role", r)
		}
		if seen[r] {
			t.Errorf("duplicate role value %q", r)
		}
		seen[r] = true

		parsed, err := ParseRole(string(r))
		if err != nil {
			t.Errorf("ParseRole(%q) returned error: %v", r, err)
		}
		if parsed != r {
			t.Errorf("ParseRole(%q) = %q", r, parsed)
		}
	}
}

func TestInvalidRolesRejected(t *testing.T) {
	invalid := []string{
		"", " ", "architect", "Architect", " ARCHITECT", "ENGINEER ",
		"HUMAN", "ADMIN", "GPT", "Claude", "OpenAI", "Anthropic",
	}
	for _, s := range invalid {
		if Role(s).IsValid() {
			t.Errorf("Role(%q) should be invalid", s)
		}
		got, err := ParseRole(s)
		if !errors.Is(err, ErrInvalidRole) {
			t.Errorf("ParseRole(%q) error = %v, want ErrInvalidRole", s, err)
		}
		if got != "" {
			t.Errorf("ParseRole(%q) = %q, want the zero Role", s, got)
		}
	}
}

func TestZeroRoleIsInvalid(t *testing.T) {
	var r Role
	if r.IsValid() {
		t.Error("the zero Role must not be valid")
	}
}
