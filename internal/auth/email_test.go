package auth

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/CosmoGao/rollbook/internal/db"
)

func TestFeishuEmailAndBinding(t *testing.T) {
	ctx := context.Background()
	d, err := db.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	s := Service{DB: d, Pepper: []byte("12345678901234567890123456789012")}
	owner, err := s.BootstrapUser(ctx, "owner", "initial password 123")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CreateExternalUser(ctx, "other"); err != nil {
		t.Fatal(err)
	}
	var other string
	if err = d.QueryRow("SELECT id FROM users WHERE login='other'").Scan(&other); err != nil {
		t.Fatal(err)
	}
	if err = s.SetEmail(ctx, owner, " Owner@Example.COM "); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.UserEmail(ctx, owner); got != "owner@example.com" {
		t.Fatal(got)
	}
	if err = s.SetEmail(ctx, other, "owner@example.com"); !errors.Is(err, ErrEmailTaken) {
		t.Fatal(err)
	}
	if err = s.SetEmail(ctx, other, "invalid address"); !errors.Is(err, ErrEmailInvalid) {
		t.Fatal(err)
	}
	if _, _, _, err = s.FeishuSession(ctx, "app", "subject", "none@example.com"); !errors.Is(err, ErrEmailUnmatched) {
		t.Fatal(err)
	}
	token, _, p, err := s.FeishuSession(ctx, "app", "subject", " OWNER@example.com ")
	if err != nil || p.UserID != owner || token == "" {
		t.Fatalf("auto bind: %v", err)
	}
	if _, _, p, err = s.FeishuSession(ctx, "app", "subject", "owner@example.com"); err != nil || p.UserID != owner {
		t.Fatalf("existing binding: %v", err)
	}
	if err = s.SetEmail(ctx, other, "other@example.com"); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = s.FeishuSession(ctx, "app", "subject", "other@example.com"); !errors.Is(err, ErrBindingConflict) {
		t.Fatal(err)
	}
	if err = s.BindFeishu(ctx, other, "app", "subject"); !errors.Is(err, ErrBindingConflict) {
		t.Fatal(err)
	}
	if err = s.BindFeishu(ctx, owner, "app", "different"); !errors.Is(err, ErrBindingConflict) {
		t.Fatal(err)
	}
	if _, err = d.Exec("UPDATE identity_bindings SET allowed=0 WHERE subject='subject'"); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = s.FeishuSession(ctx, "app", "subject", "owner@example.com"); !errors.Is(err, ErrBindingDisabled) {
		t.Fatal(err)
	}
	if err = s.BindFeishu(ctx, owner, "app", "subject"); !errors.Is(err, ErrBindingDisabled) {
		t.Fatal(err)
	}
	if _, err = d.Exec("UPDATE users SET enabled=0 WHERE id=?", owner); err != nil {
		t.Fatal(err)
	}
	if err = s.BindFeishu(ctx, owner, "app", "new"); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
}
