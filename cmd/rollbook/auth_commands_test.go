package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/db"
)

func TestLocalRecoveryCommandRequiresVerifiedExternalLogin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.db")
	t.Setenv("ROLLBOOK_DB_PATH", path)
	d, err := db.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	s := auth.Service{DB: d, Pepper: []byte("12345678901234567890123456789012")}
	if _, err = s.BootstrapUser(context.Background(), "owner", "initial password 123"); err != nil {
		t.Fatal(err)
	}
	if err = authCommand([]string{"local-login", "disable"}); err == nil {
		t.Fatal("disabled without provider")
	}
	t.Setenv("ROLLBOOK_OIDC_ISSUER", "https://issuer.example")
	t.Setenv("ROLLBOOK_OIDC_CLIENT_ID", "client")
	t.Setenv("ROLLBOOK_OIDC_CLIENT_SECRET", "test-secret")
	t.Setenv("ROLLBOOK_OIDC_REDIRECT_URL", "https://rollbook.example/api/auth/oidc/callback")
	if err = authCommand([]string{"bind", "oidc", "https://issuer.example", "sub", "owner"}); err != nil {
		t.Fatal(err)
	}
	if err = authCommand([]string{"local-login", "disable"}); err == nil {
		t.Fatal("disabled without trial")
	}
	if _, _, _, err = s.ExternalSession(context.Background(), "oidc", "https://issuer.example", "sub"); err != nil {
		t.Fatal(err)
	}
	if err = authCommand([]string{"local-login", "disable"}); err != nil {
		t.Fatal(err)
	}
	enabled, err := s.LocalEnabled(context.Background())
	if err != nil || enabled {
		t.Fatal(enabled, err)
	}
	if err = authCommand([]string{"local-login", "enable"}); err != nil {
		t.Fatal(err)
	}
	enabled, err = s.LocalEnabled(context.Background())
	if err != nil || !enabled {
		t.Fatal(enabled, err)
	}
}
