package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestMigrationAndSQLiteFeatures(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollbook.db")
	ctx := context.Background()
	for attempt := 0; attempt < 2; attempt++ {
		database, err := Open(ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		var count int
		if err := database.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 5 {
			t.Fatalf("migration count = %d", count)
		}
		var mode string
		if err := database.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
			t.Fatal(err)
		}
		if mode != "wal" {
			t.Fatalf("journal mode = %q", mode)
		}
		var foreignKeys int
		if err := database.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
			t.Fatal(err)
		}
		if foreignKeys != 1 {
			t.Fatal("foreign keys disabled")
		}
		if _, err := database.Exec("INSERT INTO search_documents(object_id,name) VALUES ('host:a','测试主机')"); err != nil {
			t.Fatal(err)
		}
		var name string
		err = database.QueryRow("SELECT name FROM search_documents WHERE search_documents MATCH '测试主机'").Scan(&name)
		if err != nil && err != sql.ErrNoRows {
			t.Fatal(err)
		}
		if err == sql.ErrNoRows {
			t.Fatal("FTS5 failed to index document")
		}
		if err := database.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
