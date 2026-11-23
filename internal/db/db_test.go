package db

import (
	"context"
	"database/sql"
	"fmt"
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
		if count != 16 {
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

func TestUpgradePreservesReferencedHostsAndSearch(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "old.db")
	d, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	d.SetMaxOpenConns(1)
	if _, err = d.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 14; i++ {
		files, err := migrations.ReadDir("migrations")
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if len(file.Name()) < 3 || file.Name()[:3] != fmt.Sprintf("%03d", i) {
				continue
			}
			source, err := migrations.ReadFile("migrations/" + file.Name())
			if err != nil {
				t.Fatal(err)
			}
			if _, err = d.Exec(string(source)); err != nil {
				t.Fatalf("%s: %v", file.Name(), err)
			}
			if _, err = d.Exec("INSERT INTO schema_migrations(version,applied_at) VALUES (?,?)", i, "2026-01-01"); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, item := range []struct{ id, typ, name string }{{"location:site", "location", "站点"}, {"host:router", "host", "路由器"}, {"host:vm", "host", "云实例"}, {"service:dns", "service", "DNS"}, {"network:lan", "network", "局域网"}, {"ip:router", "ip", "地址"}, {"uplink:wan", "uplink", "外网"}} {
		if _, err = d.Exec("INSERT INTO objects(id,type,name,created_at,created_by,updated_at,updated_by) VALUES (?,?,?,'2026-01-01','test','2026-01-01','test')", item.id, item.typ, item.name); err != nil {
			t.Fatal(err)
		}
	}
	for _, statement := range []string{
		"INSERT INTO locations(id,kind) VALUES ('location:site','站点')",
		"INSERT INTO hosts(id,kind,location_id,model) VALUES ('host:router','网络设备','location:site','Router X')",
		"INSERT INTO hosts(id,kind,location_id,parent_host_id) VALUES ('host:vm','虚拟机','location:site','host:router')",
		"INSERT INTO uplinks(id,location_id,edge_device_id) VALUES ('uplink:wan','location:site','host:router')",
		"INSERT INTO services(id,host_id) VALUES ('service:dns','host:router')",
		"INSERT INTO networks(id,cidr,location_id,router_id,uplink_id) VALUES ('network:lan','192.0.2.0/24','location:site','host:router','uplink:wan')",
		"INSERT INTO ips(id,address,network_id,host_id) VALUES ('ip:router','192.0.2.1','network:lan','host:router')",
	} {
		if _, err = d.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if err = d.Close(); err != nil {
		t.Fatal(err)
	}
	upgraded, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer upgraded.Close()
	for _, table := range []string{"hosts", "services", "networks", "ips", "uplinks"} {
		var count int
		if err = upgraded.QueryRow("SELECT count(*) FROM " + table).Scan(&count); err != nil || count == 0 {
			t.Fatalf("%s count=%d: %v", table, count, err)
		}
	}
	var id string
	if err = upgraded.QueryRow("SELECT object_id FROM search_documents WHERE search_documents MATCH 'Router'").Scan(&id); err != nil || id != "host:router" {
		t.Fatalf("old model search: %q %v", id, err)
	}
	for _, kind := range []string{"智能电视", "游戏机", "传感器与控制器"} {
		if _, err = upgraded.Exec("INSERT INTO objects(id,type,name,created_at,created_by,updated_at,updated_by) VALUES (?,'host',?,'2026-01-01','test','2026-01-01','test')", "host:"+kind, kind); err != nil {
			t.Fatal(err)
		}
		if _, err = upgraded.Exec("INSERT INTO hosts(id,kind,location_id) VALUES (?,?,?)", "host:"+kind, kind, "location:site"); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := upgraded.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	if rows.Next() {
		t.Fatal("foreign key check failed")
	}
	rows.Close()
}

func TestUpgradeV15PreservesUserKeyAndSession(t *testing.T) {
	ctx := context.Background()
	d, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "old.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	d.SetMaxOpenConns(1)
	for _, q := range []string{
		"PRAGMA foreign_keys=ON",
		"CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,applied_at TEXT)",
		"CREATE TABLE users(id TEXT PRIMARY KEY,login TEXT NOT NULL UNIQUE,password_hash TEXT NOT NULL,enabled INTEGER NOT NULL DEFAULT 1,created_at TEXT NOT NULL,password_reset_at TEXT)",
		"CREATE TABLE api_keys(id TEXT PRIMARY KEY,name TEXT,token_hash BLOB,level TEXT,issuer_user_id TEXT REFERENCES users(id),created_at TEXT)",
		"CREATE TABLE sessions(id_hash BLOB PRIMARY KEY,user_id TEXT REFERENCES users(id),expires_at TEXT,csrf_hash BLOB)",
		"INSERT INTO users(id,login,password_hash,created_at) VALUES('u','owner','old-hash','now')",
		"INSERT INTO api_keys(id,name,token_hash,level,issuer_user_id,created_at) VALUES('k','agent',x'01','read','u','now')",
		"INSERT INTO sessions(id_hash,user_id,expires_at,csrf_hash) VALUES(x'02','u','later',x'03')",
	} {
		if _, err = d.ExecContext(ctx, q); err != nil {
			t.Fatal(q, err)
		}
	}
	for version := 1; version <= 15; version++ {
		if _, err = d.ExecContext(ctx, "INSERT INTO schema_migrations(version,applied_at) VALUES(?,?)", version, "now"); err != nil {
			t.Fatal(err)
		}
	}
	if err = Migrate(ctx, d); err != nil {
		t.Fatal(err)
	}
	var count int
	for _, q := range []string{"SELECT count(*) FROM users WHERE id='u' AND password_hash='old-hash'", "SELECT count(*) FROM api_keys WHERE id='k' AND issuer_user_id='u'", "SELECT count(*) FROM sessions WHERE user_id='u' AND provider='local'"} {
		if err = d.QueryRowContext(ctx, q).Scan(&count); err != nil || count != 1 {
			t.Fatal(q, count, err)
		}
	}
	rows, err := d.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("foreign key broken")
	}
}
