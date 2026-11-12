package auth

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/CosmoGao/rollbook/internal/db"
)

func TestLoginLimitsAndPasswordSessions(t *testing.T) {
	ctx := context.Background()
	database, err := db.Open(ctx, filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	s := Service{DB: database, Pepper: []byte("12345678901234567890123456789012"), LoginPolicy: LoginPolicy{Threshold: 2, LockDuration: time.Minute, MaxBackoff: time.Minute}}
	id, err := s.BootstrapUser(ctx, "Owner", "initial password 123")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, _, _, err = s.LoginWithIP(ctx, " OWNER ", "bad", "192.0.2.1"); !errors.Is(err, ErrUnauthorized) {
			t.Fatal(err)
		}
	}
	if _, _, _, err = s.LoginWithIP(ctx, "owner", "initial password 123", "192.0.2.2"); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("account limit", err)
	}
	if _, _, _, err = s.LoginWithIP(ctx, "unknown", "bad", "192.0.2.1"); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("ip limit", err)
	}
	if _, err = database.Exec("DELETE FROM login_limits"); err != nil {
		t.Fatal(err)
	}
	first, _, p, err := s.LoginWithIP(ctx, "owner", "initial password 123", "192.0.2.2")
	if err != nil {
		t.Fatal(err)
	}
	second, _, _, err := s.LoginWithIP(ctx, "owner", "initial password 123", "192.0.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ChangePassword(ctx, p, "bad", "replacement password 123"); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
	if err = s.ChangePassword(ctx, p, "initial password 123", "replacement password 123"); err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{first, second} {
		if _, err = s.Session(ctx, token, "", false); !errors.Is(err, ErrUnauthorized) {
			t.Fatal("old session survived", err)
		}
	}
	if _, _, _, err = s.LoginWithIP(ctx, "owner", "initial password 123", "192.0.2.4"); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
	if err = s.ResetPassword(ctx, "owner", "reset password 12345"); err != nil {
		t.Fatal(err)
	}
	token, _, p, err := s.LoginWithIP(ctx, "owner", "reset password 12345", "192.0.2.5")
	if err != nil {
		t.Fatal(err)
	}
	if p.UserID != id {
		t.Fatal(p)
	}
	if err = s.LogoutAll(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Session(ctx, token, "", false); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
}

func TestResetPasswordAllowsLockedIPAndRevokesSessions(t *testing.T) {
	ctx := context.Background()
	database, err := db.Open(ctx, filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	s := Service{DB: database, Pepper: []byte("12345678901234567890123456789012"), LoginPolicy: LoginPolicy{Threshold: 2, LockDuration: time.Minute, MaxBackoff: time.Minute}}
	if _, err := s.BootstrapUser(ctx, "owner", "initial password 123"); err != nil {
		t.Fatal(err)
	}
	oldSession, _, _, err := s.LoginWithIP(ctx, "owner", "initial password 123", "192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, _, _, err := s.LoginWithIP(ctx, "owner", "wrong password", "192.0.2.1"); !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("failed login %d: %v", i, err)
		}
	}
	for _, kind := range []string{"account", "ip"} {
		var count int
		if err := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM login_limits WHERE kind=?", kind).Scan(&count); err != nil || count != 1 {
			t.Fatalf("expected %s lock: count=%d err=%v", kind, count, err)
		}
	}
	if err := s.ResetPassword(ctx, "owner", "replacement password 123"); err != nil {
		t.Fatal(err)
	}
	var ipLimits int
	if err := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM login_limits WHERE kind='ip'").Scan(&ipLimits); err != nil || ipLimits != 1 {
		t.Fatalf("IP limit should remain in force for other accounts: count=%d err=%v", ipLimits, err)
	}
	if _, err := s.Session(ctx, oldSession, "", false); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("old session survived reset: %v", err)
	}
	if _, _, _, err := s.LoginWithIP(ctx, "owner", "replacement password 123", "192.0.2.1"); err != nil {
		t.Fatalf("new password from locked IP: %v", err)
	}
}
