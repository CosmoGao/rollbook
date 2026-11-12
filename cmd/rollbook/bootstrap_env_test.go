package main

import (
	"bytes"
	"context"
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/db"
)

func TestBootstrapFromEnvironment(t *testing.T) {
	const pepper = "0123456789abcdef0123456789abcdef"
	for _, source := range []string{"variable", "file"} {
		t.Run(source, func(t *testing.T) {
			clearBootstrapEnv(t)
			database, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "rollbook.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			password := "long password 123"
			t.Setenv("ROLLBOOK_BOOTSTRAP_USER", "owner")
			if source == "file" {
				path := filepath.Join(t.TempDir(), "password")
				if err := os.WriteFile(path, []byte(password+"\n"), 0600); err != nil {
					t.Fatal(err)
				}
				t.Setenv("ROLLBOOK_BOOTSTRAP_PASSWORD_FILE", path)
			} else {
				t.Setenv("ROLLBOOK_BOOTSTRAP_PASSWORD", password)
			}
			if err := bootstrapFromEnvironment(context.Background(), database, []byte(pepper)); err != nil {
				t.Fatal(err)
			}
			checkLogin(t, database, []byte(pepper), password)
			// 重启时的配置不应改变现有口令，文件即使已被删除也不应读取。
			if source == "file" {
				if err := os.Remove(os.Getenv("ROLLBOOK_BOOTSTRAP_PASSWORD_FILE")); err != nil {
					t.Fatal(err)
				}
			} else {
				t.Setenv("ROLLBOOK_BOOTSTRAP_PASSWORD", "different password 456")
			}
			var logs bytes.Buffer
			originalOutput := log.Writer()
			log.SetOutput(&logs)
			if err := bootstrapFromEnvironment(context.Background(), database, []byte(pepper)); err != nil {
				log.SetOutput(originalOutput)
				t.Fatal(err)
			}
			log.SetOutput(originalOutput)
			if strings.Count(logs.String(), "忽略首次管理员自动引导") != 1 || strings.Contains(logs.String(), password) {
				t.Fatalf("unexpected bootstrap log: %q", logs.String())
			}
			checkLogin(t, database, []byte(pepper), password)
		})
	}
}

func checkLogin(t *testing.T, database *sql.DB, pepper []byte, password string) {
	t.Helper()
	_, _, p, err := (auth.Service{DB: database, Pepper: pepper}).Login(context.Background(), "owner", password)
	if err != nil || p.Login != "owner" {
		t.Fatalf("owner login: %v, %+v", err, p)
	}
}

func TestBootstrapEnvironmentErrors(t *testing.T) {
	const pepper = "0123456789abcdef0123456789abcdef"
	for _, tc := range []struct {
		name, user, password, file, want string
	}{
		{"missing user", "", "long password 123", "", "ROLLBOOK_BOOTSTRAP_USER"},
		{"missing password", "owner", "", "", "ROLLBOOK_BOOTSTRAP_PASSWORD"},
		{"both", "owner", "long password 123", "/missing", "cannot both be set"},
		{"short", "owner", "short", "", "at least 12"},
		{"missing file", "owner", "", "/missing", "read ROLLBOOK_BOOTSTRAP_PASSWORD_FILE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clearBootstrapEnv(t)
			database, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "rollbook.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			if tc.user != "" {
				t.Setenv("ROLLBOOK_BOOTSTRAP_USER", tc.user)
			}
			if tc.password != "" {
				t.Setenv("ROLLBOOK_BOOTSTRAP_PASSWORD", tc.password)
			}
			if tc.file != "" {
				t.Setenv("ROLLBOOK_BOOTSTRAP_PASSWORD_FILE", tc.file)
			}
			err = bootstrapFromEnvironment(context.Background(), database, []byte(pepper))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want %q", err, tc.want)
			}
			if strings.Contains(err.Error(), tc.password) && tc.password != "" {
				t.Fatalf("password leaked: %v", err)
			}
			var count int
			if err := database.QueryRow("SELECT count(*) FROM users").Scan(&count); err != nil || count != 0 {
				t.Fatalf("users: %d, %v", count, err)
			}
		})
	}
	clearBootstrapEnv(t)
	database, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "rollbook.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := bootstrapFromEnvironment(context.Background(), database, []byte(pepper)); err != nil {
		t.Fatal(err)
	}
}

func clearBootstrapEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{"ROLLBOOK_BOOTSTRAP_USER", "ROLLBOOK_BOOTSTRAP_PASSWORD", "ROLLBOOK_BOOTSTRAP_PASSWORD_FILE"} {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
}
