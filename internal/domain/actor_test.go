package domain

import (
	"errors"
	"reflect"
	"testing"
)

func TestNewActorAssertionValid(t *testing.T) {
	cases := []struct {
		name string
		id   ActorID
		kind ActorKind
		role Role
	}{
		{"human without role", "alice", ActorKindHuman, ""},
		{"human with role", "alice", ActorKindHuman, RoleArchitect},
		{"agent engineer", "agent-1", ActorKindAgent, RoleEngineer},
		{"agent without role", "agent-2", ActorKindAgent, ""},
		{"system", "forge", ActorKindSystem, ""},
	}
	for _, c := range cases {
		a, err := NewActorAssertion(c.id, c.kind, c.role)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if !a.IsValid() || a.ID() != c.id || a.Kind() != c.kind || a.Role() != c.role {
			t.Errorf("%s: assertion fields wrong: %+v", c.name, a)
		}
	}
}

func TestNewActorAssertionRejectsInvalid(t *testing.T) {
	for _, id := range []ActorID{"", " ", "\t\n"} {
		if _, err := NewActorAssertion(id, ActorKindHuman, ""); !errors.Is(err, ErrInvalidActorID) {
			t.Errorf("blank id %q: error = %v, want ErrInvalidActorID", id, err)
		}
	}
	for _, k := range []ActorKind{"", "human", "GPT", "CLAUDE", "ADMIN", "HUMAN "} {
		if _, err := NewActorAssertion("x", k, ""); !errors.Is(err, ErrInvalidActorKind) {
			t.Errorf("kind %q: error = %v, want ErrInvalidActorKind", k, err)
		}
	}
	for _, r := range []Role{"GPT", "architect", "ADMIN", " ENGINEER"} {
		if _, err := NewActorAssertion("x", ActorKindAgent, r); !errors.Is(err, ErrInvalidRole) {
			t.Errorf("role %q: error = %v, want ErrInvalidRole", r, err)
		}
	}
	if _, err := NewActorAssertion("forge", ActorKindSystem, RoleEngineer); !errors.Is(err, ErrInvalidActor) {
		t.Errorf("system with role: error = %v, want ErrInvalidActor", err)
	}
}

func TestZeroActorValuesAreInvalid(t *testing.T) {
	var a ActorAssertion
	if a.IsValid() {
		t.Error("the zero ActorAssertion must not be valid")
	}
	var ta TrustedActor
	if ta.IsValid() {
		t.Error("the zero TrustedActor must not be valid")
	}
}

func TestIssueTrustedActor(t *testing.T) {
	ta, err := IssueTrustedActor(mustAssertion(t, "alice", ActorKindHuman, RoleArchitect))
	if err != nil {
		t.Fatalf("IssueTrustedActor: %v", err)
	}
	if !ta.IsValid() || ta.ID() != "alice" || ta.Kind() != ActorKindHuman || ta.Role() != RoleArchitect {
		t.Errorf("trusted actor fields wrong: %+v", ta)
	}
	var zero ActorAssertion
	got, err := IssueTrustedActor(zero)
	if !errors.Is(err, ErrInvalidActor) {
		t.Errorf("issuing from the zero assertion: error = %v, want ErrInvalidActor", err)
	}
	if got.IsValid() {
		t.Error("issuing from an invalid assertion produced a valid TrustedActor")
	}
}

// The authorization surface accepts only TrustedActor. An ActorAssertion cannot
// be passed where authority is required.
func TestAssertionIsNotAcceptedAsAuthority(t *testing.T) {
	trustedType := reflect.TypeOf(TrustedActor{})
	if trustedType == reflect.TypeOf(ActorAssertion{}) {
		t.Fatal("TrustedActor and ActorAssertion must be different types")
	}
	if f, ok := reflect.TypeOf(Request{}).FieldByName("Actor"); !ok || f.Type != trustedType {
		t.Error("Request.Actor must be a TrustedActor")
	}
	if f, ok := reflect.TypeOf(TransitionRequest{}).FieldByName("Actor"); !ok || f.Type != trustedType {
		t.Error("TransitionRequest.Actor must be a TrustedActor")
	}
	fn := reflect.TypeOf(NewApproval)
	if fn.NumIn() != 5 || fn.In(3) != trustedType || fn.In(4) != trustedType {
		t.Error("NewApproval approver and requester must be TrustedActor")
	}
	apply, ok := reflect.TypeOf((*Task)(nil)).Elem().MethodByName("Apply")
	if !ok || apply.Type.In(0) != trustedType {
		t.Error("Task.Apply actor must be a TrustedActor")
	}
}

// A caller that can only assert an actor has nothing it can pass as authority.
func TestForgedHumanAssertionConfersNoAuthority(t *testing.T) {
	_ = mustAssertion(t, "mallory", ActorKindHuman, "")
	var untrusted TrustedActor // all an assertion-only caller can hold

	for _, action := range append(append([]Action{}, ordinaryActions...), consequentialActions...) {
		d := Evaluate(Request{Actor: untrusted, Action: action, TaskID: "task-1"})
		if d.Outcome != OutcomeDeny || d.Reason != ReasonDeniedUnknownActor {
			t.Errorf("untrusted %s = %s/%s, want DENY/DeniedUnknownActor", action, d.Outcome, d.Reason)
		}
	}
	agent := mustTrusted(t, "agent-1", ActorKindAgent, RoleEngineer)
	if _, err := NewApproval("ap-1", "task-1", ActionPush, untrusted, agent); !errors.Is(err, ErrInvalidApprover) {
		t.Errorf("approval from an untrusted approver: %v, want ErrInvalidApprover", err)
	}
}

func TestIdentityKindAndRoleAreDistinct(t *testing.T) {
	human := mustTrusted(t, "same", ActorKindHuman, "")
	agent := mustTrusted(t, "same", ActorKindAgent, RoleEngineer)
	other := mustTrusted(t, "same", ActorKindAgent, RoleVerifier)

	if human.ID() != agent.ID() {
		t.Fatal("test bug: ids should match")
	}
	if human == agent || agent == other {
		t.Error("actors differing in kind or role must not be equal")
	}
	for _, k := range []ActorKind{ActorKindHuman, ActorKindAgent, ActorKindSystem} {
		if Role(k).IsValid() {
			t.Errorf("kind %q must not be a valid role", k)
		}
	}
	for _, r := range []Role{RoleArchitect, RoleEngineer, RoleVerifier} {
		if ActorKind(r).IsValid() {
			t.Errorf("role %q must not be a valid kind", r)
		}
	}
}

func TestNoProviderKindsOrRoles(t *testing.T) {
	for _, s := range []string{"GPT", "CLAUDE", "OPENAI", "ANTHROPIC", "gpt", "Claude"} {
		if ActorKind(s).IsValid() {
			t.Errorf("%q must not be an actor kind", s)
		}
		if Role(s).IsValid() {
			t.Errorf("%q must not be a role", s)
		}
	}
}
