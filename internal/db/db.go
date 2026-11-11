package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Open 配置单连接，保证每次访问都启用连接级外键约束。
func Open(ctx context.Context, path string) (*sql.DB, error) {
	if path == "" {
		return nil, fmt.Errorf("database path is empty")
	}
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return nil, err
		}
	}
	database, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	for _, pragma := range []string{
		"PRAGMA busy_timeout = 5000",
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
	} {
		if _, err := database.ExecContext(ctx, pragma); err != nil {
			database.Close()
			return nil, fmt.Errorf("%s: %w", pragma, err)
		}
	}
	if err := Migrate(ctx, database); err != nil {
		database.Close()
		return nil, err
	}
	return database, nil
}

// Migrate 按文件名中的版本号顺序执行迁移，成功后在同一事务中记账。
func Migrate(ctx context.Context, database *sql.DB) error {
	entries, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return fmt.Errorf("no migrations embedded")
	}
	for _, name := range entries {
		base := filepath.Base(name)
		prefix, _, ok := strings.Cut(base, "_")
		if !ok {
			return fmt.Errorf("invalid migration name %q", name)
		}
		version, err := strconv.Atoi(prefix)
		if err != nil {
			return fmt.Errorf("invalid migration version %q: %w", name, err)
		}
		var applied int
		err = database.QueryRowContext(ctx, "SELECT 1 FROM sqlite_master WHERE type='table' AND name='schema_migrations'").Scan(&applied)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if err == nil {
			err = database.QueryRowContext(ctx, "SELECT 1 FROM schema_migrations WHERE version=?", version).Scan(&applied)
			if err == nil {
				continue
			}
			if err != sql.ErrNoRows {
				return err
			}
		}
		source, err := migrations.ReadFile(name)
		if err != nil {
			return err
		}
		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, string(source)); err == nil {
			_, err = tx.ExecContext(ctx, "INSERT INTO schema_migrations(version, applied_at) VALUES (?, ?)", version, time.Now().UTC().Format(time.RFC3339))
		}
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
	}
	return nil
}
