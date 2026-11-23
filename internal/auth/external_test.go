package auth

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/CosmoGao/rollbook/internal/db"
)

func TestExternalBindingsAndRecovery(t *testing.T) {
	ctx := context.Background()
	d, err := db.Open(ctx, filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	s := Service{DB: d, Pepper: []byte("12345678901234567890123456789012")}
	owner, err := s.BootstrapUser(ctx, "owner", "initial password 123")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CreateExternalUser(ctx, "external"); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = s.ExternalSession(ctx, "oidc", "https://issuer.example", "same-email"); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("unbound login", err)
	}
	if err = s.ManageBinding(ctx, "oidc", "https://issuer.example", "subject-1", "owner", true); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = s.ExternalSession(ctx, "oidc", "https://issuer.example", "same-email"); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("email collision", err)
	}
	local, _, p, err := s.Login(ctx, "owner", "initial password 123")
	if err != nil {
		t.Fatal(err)
	}
	_, key, err := s.IssueKey(ctx, p, "agent", Read, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	external, _, p2, err := s.ExternalSession(ctx, "oidc", "https://issuer.example", "subject-1")
	if err != nil {
		t.Fatal(err)
	}
	if p2.UserID != owner {
		t.Fatal(p2)
	}
	kind, can, err := s.Identity(ctx, p2)
	if err != nil || kind != "mixed" || !can {
		t.Fatal(kind, can, err)
	}
	if err = s.SetLocalEnabled(ctx, false); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Session(ctx, local, "", false); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("local session survived", err)
	}
	if _, err = s.Authenticate(ctx, key); err != nil {
		t.Fatal("local sign-in switch revoked key", err)
	}
	if _, _, _, err = s.Login(ctx, "owner", "initial password 123"); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("local login enabled", err)
	}
	if _, err = s.Session(ctx, external, "", false); err != nil {
		t.Fatal(err)
	}
	kind, can, err = s.Identity(ctx, p2)
	if err != nil || kind != "mixed" || can {
		t.Fatal(kind, can, err)
	}
	if err = s.ChangePassword(ctx, p, "initial password 123", "new password 12345"); !errors.Is(err, ErrForbidden) {
		t.Fatal("change while closed", err)
	}
	if err = s.ManageBinding(ctx, "oidc", "https://issuer.example", "subject-1", "owner", false); !errors.Is(err, ErrForbidden) {
		t.Fatal("last external login removed", err)
	}
	if err = s.SetLocalEnabled(ctx, true); err != nil {
		t.Fatal(err)
	}
	if err = s.ManageBinding(ctx, "oidc", "https://issuer.example", "subject-1", "owner", false); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Session(ctx, external, "", false); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("external session survived", err)
	}
	if _, _, _, err = s.Login(ctx, "owner", "initial password 123"); err != nil {
		t.Fatal("recovery", err)
	}
	if err = s.ManageBinding(ctx, "ldap", "directory-a", "guid-1", "external", true); err != nil {
		t.Fatal(err)
	}
	extOnly, _, externalP, err := s.ExternalSession(ctx, "ldap", "directory-a", "guid-1")
	if err != nil {
		t.Fatal(err)
	}
	kind, can, err = s.Identity(ctx, externalP)
	if err != nil || kind != "external" || can {
		t.Fatal(kind, can, err)
	}
	if err = s.ChangePassword(ctx, externalP, "whatever", "new password 12345"); !errors.Is(err, ErrForbidden) {
		t.Fatal("external change password", err)
	}
	if err = s.SetUserEnabled(ctx, "external", false); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Session(ctx, extOnly, "", false); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("disabled session survived", err)
	}
}
