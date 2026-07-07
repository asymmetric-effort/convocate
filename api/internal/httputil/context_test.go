package httputil

import (
	"context"
	"testing"
)

func TestContextWithPrincipal_RoundTrip(t *testing.T) {
	p := &Principal{
		ID:                "usr-001",
		Username:          "alice",
		Name:              "Alice Smith",
		Email:             "alice@example.com",
		Groups:            []string{"dev", "ops"},
		Roles:             []string{"admin"},
		IDP:               "openbao",
		AuthorizedApplets: []string{"nmgr", "amgr"},
	}

	ctx := ContextWithPrincipal(context.Background(), p)
	got, ok := PrincipalFromContext(ctx)
	if !ok {
		t.Fatal("PrincipalFromContext returned ok=false")
	}
	if got.ID != p.ID {
		t.Errorf("ID = %q, want %q", got.ID, p.ID)
	}
	if got.Username != p.Username {
		t.Errorf("Username = %q, want %q", got.Username, p.Username)
	}
	if got.Name != p.Name {
		t.Errorf("Name = %q, want %q", got.Name, p.Name)
	}
	if got.Email != p.Email {
		t.Errorf("Email = %q, want %q", got.Email, p.Email)
	}
	if got.IDP != p.IDP {
		t.Errorf("IDP = %q, want %q", got.IDP, p.IDP)
	}
	if len(got.Groups) != 2 {
		t.Errorf("Groups len = %d, want 2", len(got.Groups))
	}
	if len(got.Roles) != 1 || got.Roles[0] != "admin" {
		t.Errorf("Roles = %v, want [admin]", got.Roles)
	}
	if len(got.AuthorizedApplets) != 2 {
		t.Errorf("AuthorizedApplets len = %d, want 2", len(got.AuthorizedApplets))
	}
}

func TestPrincipalFromContext_EmptyContext(t *testing.T) {
	_, ok := PrincipalFromContext(context.Background())
	if ok {
		t.Error("PrincipalFromContext on empty context returned ok=true, want false")
	}
}

func TestPrincipalFromContext_NilPrincipal(t *testing.T) {
	ctx := ContextWithPrincipal(context.Background(), nil)
	p, ok := PrincipalFromContext(ctx)
	// A nil *Principal stored via context.WithValue will type-assert successfully
	// (ok=true) but the value will be nil. This is standard Go behavior.
	if !ok {
		t.Error("expected ok=true for nil *Principal (typed nil)")
	}
	if p != nil {
		t.Error("expected nil principal value")
	}
}

func TestContextWithPrincipal_Overwrite(t *testing.T) {
	p1 := &Principal{ID: "usr-001", Username: "alice"}
	p2 := &Principal{ID: "usr-002", Username: "bob"}

	ctx := ContextWithPrincipal(context.Background(), p1)
	ctx = ContextWithPrincipal(ctx, p2)

	got, ok := PrincipalFromContext(ctx)
	if !ok {
		t.Fatal("PrincipalFromContext returned ok=false")
	}
	if got.ID != "usr-002" {
		t.Errorf("ID = %q, want %q", got.ID, "usr-002")
	}
	if got.Username != "bob" {
		t.Errorf("Username = %q, want %q", got.Username, "bob")
	}
}
