package backup

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/CosmoGao/rollbook/internal/db"
	"gopkg.in/yaml.v3"
)

func TestRoundTripAndRollback(t *testing.T) {
	ctx := context.Background()
	d, err := db.Open(ctx, filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	for _, q := range []string{
		`INSERT INTO users(id,login,password_hash,created_at) VALUES('u','admin','bcrypt-hash','now')`,
		`INSERT INTO api_keys(id,name,token_hash,level,issuer_user_id,created_at) VALUES('k','agent',x'001122','read','u','now')`,
		`INSERT INTO sessions(id_hash,user_id,expires_at,csrf_hash) VALUES(x'01','u','later',x'02')`,
		`INSERT INTO objects(id,type,name,created_at,created_by,updated_at,updated_by) VALUES('location:a','location','北京','now','u','now','u')`,
		`INSERT INTO locations(id,kind) VALUES('location:a','站点')`,
		`INSERT INTO objects(id,type,name,created_at,created_by,updated_at,updated_by) VALUES('host:a','host','机器','now','u','now','u')`,
		`INSERT INTO hosts(id,kind,location_id) VALUES('host:a','物理机','location:a')`,
		`INSERT INTO search_documents(object_id,name) VALUES('host:a','机器')`,
	} {
		if _, err = d.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	s := Service{DB: d}
	data, err := s.Export(ctx, "full")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("sessions")) || bytes.Contains(data, []byte("001122")) {
		t.Fatal("session or raw hash leaked")
	}
	f, p, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if p.Counts["objects"] != 2 || p.Counts["api_keys"] != 1 {
		t.Fatal(p)
	}
	if err = s.Restore(ctx, f); err != nil {
		t.Fatal(err)
	}
	var n int
	for _, tc := range []struct {
		q    string
		want int
	}{{"SELECT count(*) FROM objects", 2}, {"SELECT count(*) FROM hosts", 1}, {"SELECT count(*) FROM sessions", 0}, {"SELECT count(*) FROM search_documents WHERE search_documents MATCH '机器'", 1}} {
		if err = d.QueryRowContext(ctx, tc.q).Scan(&n); err != nil || n != tc.want {
			t.Fatalf("%s: %d %v", tc.q, n, err)
		}
	}
	broken := f
	broken.Tables = map[string][]map[string]any{}
	for k, v := range f.Tables {
		broken.Tables[k] = v
	}
	broken.Tables["hosts"] = []map[string]any{{"id": "host:a", "kind": "物理机", "location_id": "location:missing"}}
	if err = s.Restore(ctx, broken); err == nil {
		t.Fatal("broken reference accepted")
	}
	if err = d.QueryRowContext(ctx, "SELECT count(*) FROM hosts").Scan(&n); err != nil || n != 1 {
		t.Fatal("old database changed", n, err)
	}
	f.SchemaVersion = 99
	if _, _, err := Parse(mustYAML(t, f)); err == nil {
		t.Fatal("future version accepted")
	}
}
func mustYAML(t *testing.T, f File) []byte {
	t.Helper()
	b, e := yaml.Marshal(f)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestAtomicFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", "backup.yaml")
	if err := WriteAtomic(path, []byte("schema_version: 1\n")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode: %v", info.Mode())
	}
}
