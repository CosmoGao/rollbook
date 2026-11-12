package backup

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/CosmoGao/rollbook/internal/catalog"
	"github.com/CosmoGao/rollbook/internal/db"
	"github.com/CosmoGao/rollbook/internal/secrets"
	"gopkg.in/yaml.v3"
)

const MaxSize = 16 << 20
const SchemaVersion = 1

var ErrInvalid = errors.New("invalid backup")
var ErrConflict = errors.New("merge conflict")

// 表名固定，避免导入文件控制 SQL；顺序只用于稳定输出。
var catalogTables = []string{"objects", "locations", "uplinks", "hosts", "networks", "ips", "services", "ports", "endpoints", "dependencies", "versions", "credentials", "notes", "contexts", "changes", "version_checks", "host_offline_operations"}
var fullTables = append(append([]string{}, catalogTables...), "users", "api_keys")

type File struct {
	SchemaVersion int                         `yaml:"schema_version" json:"schema_version"`
	Kind          string                      `yaml:"kind" json:"kind"`
	ExportedAt    string                      `yaml:"exported_at" json:"exported_at"`
	Tables        map[string][]map[string]any `yaml:"tables" json:"tables"`
	Objects       []map[string]any            `yaml:"objects,omitempty" json:"objects,omitempty"`
}
type Preview struct {
	Kind              string                     `json:"kind"`
	Counts            map[string]int             `json:"counts"`
	Objects           []ObjectPreview            `json:"objects,omitempty"`
	Differences       map[string]DifferenceCount `json:"differences,omitempty"`
	ObjectDifferences map[string]DifferenceCount `json:"object_differences,omitempty"`
	LostChanges       []ChangePreview            `json:"lost_changes,omitempty"`
}
type Service struct{ DB *sql.DB }

func tables(kind string) ([]string, error) {
	switch kind {
	case "catalog":
		return catalogTables, nil
	case "full":
		return fullTables, nil
	}
	return nil, ErrInvalid
}
func (s Service) Export(ctx context.Context, kind string) ([]byte, error) {
	names, err := tables(kind)
	if err != nil {
		return nil, err
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	f := File{SchemaVersion: SchemaVersion, Kind: kind, ExportedAt: time.Now().UTC().Format(time.RFC3339Nano), Tables: map[string][]map[string]any{}}
	for _, name := range names {
		rows, err := tx.QueryContext(ctx, "SELECT * FROM "+name+" ORDER BY 1")
		if err != nil {
			return nil, err
		}
		cols, _ := rows.Columns()
		list := []map[string]any{}
		for rows.Next() {
			vals := make([]any, len(cols))
			ptr := make([]any, len(cols))
			for i := range vals {
				ptr[i] = &vals[i]
			}
			if err = rows.Scan(ptr...); err != nil {
				rows.Close()
				return nil, err
			}
			row := map[string]any{}
			for i, c := range cols {
				v := vals[i]
				if b, ok := v.([]byte); ok {
					if name == "api_keys" && c == "token_hash" {
						v = base64.StdEncoding.EncodeToString(b)
					} else {
						v = string(b)
					}
				}
				row[c] = v
			}
			if name != "users" && name != "api_keys" {
				if err := secrets.Check(row); err != nil {
					rows.Close()
					return nil, err
				}
			}
			list = append(list, row)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		f.Tables[name] = list
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	b, err := yaml.Marshal(f)
	if err != nil {
		return nil, err
	}
	if len(b) > MaxSize {
		return nil, ErrInvalid
	}
	return b, nil
}
func Parse(data []byte) (File, Preview, error) {
	var f File
	var p Preview
	if len(data) == 0 || len(data) > MaxSize {
		return f, p, ErrInvalid
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil {
		return f, p, ErrInvalid
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return f, p, ErrInvalid
	}
	if f.SchemaVersion != SchemaVersion {
		return f, p, fmt.Errorf("unsupported schema_version: %d", f.SchemaVersion)
	}
	names, err := tables(f.Kind)
	if err != nil {
		if f.Kind != "merge" {
			return f, p, err
		}
		return parseMerge(f)
	}
	// 旧版备份没有停用快照表。允许缺省并在恢复时清空当前快照，避免误用旧操作。
	if _, ok := f.Tables["host_offline_operations"]; !ok && len(f.Tables) == len(names)-1 {
		f.Tables["host_offline_operations"] = []map[string]any{}
	}
	if len(f.Tables) != len(names) {
		return f, p, ErrInvalid
	}
	p = Preview{Kind: f.Kind, Counts: map[string]int{}}
	for _, name := range names {
		rows, ok := f.Tables[name]
		if !ok {
			return f, p, ErrInvalid
		}
		p.Counts[name] = len(rows)
		for _, row := range rows {
			if len(row) == 0 {
				return f, p, ErrInvalid
			}
			for _, field := range []string{"mgmt_url", "docs_url", "url", "latest_url"} {
				if value, ok := row[field]; ok && value != nil && fmt.Sprint(value) != "" && !catalog.ValidHTTPURL(fmt.Sprint(value)) {
					return f, p, &catalog.ValidationError{Fields: []catalog.FieldError{{Field: "fields." + field, Reason: fmt.Sprintf("%s 中对象 %v 仅接受绝对 HTTP 或 HTTPS 地址", name, row["id"]), Expected: "http:// 或 https:// 地址"}}}
				}
			}
			if name != "users" && name != "api_keys" {
				if err := secrets.Check(row); err != nil {
					return f, p, err
				}
			}
		}
	}
	if _, err := time.Parse(time.RFC3339Nano, f.ExportedAt); err != nil {
		return f, p, ErrInvalid
	}
	return f, p, nil
}
func columns(ctx context.Context, q *sql.Tx, name string) ([]string, error) {
	rows, err := q.QueryContext(ctx, "SELECT * FROM "+name+" LIMIT 0")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return rows.Columns()
}
func apply(ctx context.Context, conn *sql.Conn, f File) error {
	names, _ := tables(f.Kind)
	// 一个专用连接持有事务；其他请求在事务和外键开关期间无法使用连接。
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); err != nil {
		return err
	}
	defer conn.ExecContext(context.Background(), "PRAGMA foreign_keys=ON")
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i := len(names) - 1; i >= 0; i-- {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+names[i]); err != nil {
			return err
		}
	}
	// FTS 是派生索引；会话和待导入申请不会进入备份。
	if _, err := tx.ExecContext(ctx, "DELETE FROM search_documents"); err != nil {
		return err
	}
	for _, name := range names {
		cols, err := columns(ctx, tx, name)
		if err != nil {
			return err
		}
		sort.Strings(cols)
		for _, row := range f.Tables[name] {
			// 旧备份没有提议提交时快照；恢复后沿用审计记录做保守冲突判断。
			if name == "changes" && len(row) == len(cols)-1 {
				if _, ok := row["base_object_json"]; !ok {
					row["base_object_json"] = nil
				}
			}
			if len(row) != len(cols) {
				return ErrInvalid
			}
			values := make([]any, len(cols))
			for i, c := range cols {
				v, ok := row[c]
				if !ok {
					return ErrInvalid
				}
				if name == "api_keys" && c == "token_hash" {
					str, ok := v.(string)
					if !ok {
						return ErrInvalid
					}
					v, err = base64.StdEncoding.DecodeString(str)
					if err != nil {
						return ErrInvalid
					}
				}
				values[i] = v
			}
			marks := strings.TrimSuffix(strings.Repeat("?,", len(cols)), ",")
			if _, err := tx.ExecContext(ctx, "INSERT INTO "+name+"("+strings.Join(cols, ",")+") VALUES ("+marks+")", values...); err != nil {
				return err
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO search_documents(object_id,name,aliases,tags,description,url)
 SELECT o.id,o.name,coalesce((SELECT group_concat(value,' ') FROM json_each(o.aliases_json)),''),
 coalesce((SELECT group_concat(value,' ') FROM json_each(o.tags_json)),''),coalesce(l.description,''),coalesce(e.url,'')
 FROM objects o LEFT JOIN locations l ON l.id=o.id LEFT JOIN endpoints e ON e.id=o.id`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM sessions"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM version_check_lease"); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return err
	}
	bad := rows.Next()
	rows.Close()
	if bad {
		return ErrInvalid
	}
	return tx.Commit()
}

// Validate 在独立临时库中执行完整恢复，再检查引用和索引。
func Validate(ctx context.Context, f File) error {
	dir, err := os.MkdirTemp("", "rollbook-validate-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "staging.db")
	d, err := db.Open(ctx, path)
	if err != nil {
		return err
	}
	defer d.Close()
	conn, err := d.Conn(ctx)
	if err != nil {
		return err
	}
	if err = apply(ctx, conn, f); err != nil {
		conn.Close()
		return err
	}
	if err = conn.Close(); err != nil {
		return err
	}
	var n, m int
	if err = d.QueryRowContext(ctx, "SELECT count(*) FROM objects").Scan(&n); err != nil {
		return err
	}
	if err = d.QueryRowContext(ctx, "SELECT count(*) FROM search_documents").Scan(&m); err != nil {
		return err
	}
	if n != m {
		return ErrInvalid
	}
	return validateRelations(ctx, d)
}
func (s Service) Restore(ctx context.Context, f File) error {
	if err := Validate(ctx, f); err != nil {
		return err
	}
	conn, err := s.DB.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	return apply(ctx, conn, f)
}

// WriteAtomic writes one private backup in the data directory and fsyncs it before rename.
func WriteAtomic(path string, data []byte) error {
	if len(data) == 0 || len(data) > MaxSize {
		return ErrInvalid
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".rollbook-backup-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err = f.Chmod(0600); err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

// validateRelations checks references stored in JSON and network addresses, which SQLite foreign keys cannot cover.
type relationQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func validateRelations(ctx context.Context, d relationQuerier) error {
	// 释放后的地址占位符必须对应已保存的操作快照，且原地址仍在网段内。
	released := map[string]string{}
	snapshots, err := d.QueryContext(ctx, "SELECT snapshot_json FROM host_offline_operations ORDER BY created_at,id")
	if err != nil {
		return err
	}
	for snapshots.Next() {
		var raw string
		if err = snapshots.Scan(&raw); err != nil {
			snapshots.Close()
			return err
		}
		var items []struct {
			ID            string `json:"id"`
			BeforeAddress string `json:"before_address"`
		}
		if json.Unmarshal([]byte(raw), &items) != nil {
			snapshots.Close()
			return ErrInvalid
		}
		for _, item := range items {
			if item.BeforeAddress != "" {
				released[item.ID] = item.BeforeAddress
			}
		}
	}
	err = snapshots.Err()
	snapshots.Close()
	if err != nil {
		return err
	}
	rows, err := d.QueryContext(ctx, "SELECT cidr FROM networks")
	if err != nil {
		return err
	}
	for rows.Next() {
		var cidr string
		if err = rows.Scan(&cidr); err != nil {
			rows.Close()
			return err
		}
		if _, err = netip.ParsePrefix(cidr); err != nil {
			rows.Close()
			return ErrInvalid
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	rows, err = d.QueryContext(ctx, "SELECT ips.id, ips.address, objects.status, networks.cidr FROM ips JOIN networks ON networks.id=ips.network_id JOIN objects ON objects.id=ips.id")
	if err != nil {
		return err
	}
	for rows.Next() {
		var id, address, status, cidr string
		if err = rows.Scan(&id, &address, &status, &cidr); err != nil {
			rows.Close()
			return err
		}
		if address == "released:"+id {
			if status != "停用" || released[id] == "" {
				rows.Close()
				return ErrInvalid
			}
			address = released[id]
		}
		addr, e := netip.ParseAddr(address)
		if e != nil {
			rows.Close()
			return ErrInvalid
		}
		prefix, e := netip.ParsePrefix(cidr)
		if e != nil || !prefix.Contains(addr) {
			rows.Close()
			return ErrInvalid
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	ids := map[string]bool{}
	rows, err = d.QueryContext(ctx, "SELECT id FROM objects")
	if err != nil {
		return err
	}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids[id] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, query := range []string{"SELECT via_json FROM endpoints", "SELECT related_ids_json FROM contexts"} {
		rows, err = d.QueryContext(ctx, query)
		if err != nil {
			return err
		}
		for rows.Next() {
			var raw string
			if err = rows.Scan(&raw); err != nil {
				rows.Close()
				return err
			}
			var refs []string
			if json.Unmarshal([]byte(raw), &refs) != nil {
				rows.Close()
				return ErrInvalid
			}
			for _, id := range refs {
				if !ids[id] {
					rows.Close()
					return ErrInvalid
				}
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	edges := map[string][]string{}
	rows, err = d.QueryContext(ctx, "SELECT from_id,to_id FROM dependencies")
	if err != nil {
		return err
	}
	for rows.Next() {
		var from, to string
		if err = rows.Scan(&from, &to); err != nil {
			rows.Close()
			return err
		}
		edges[from] = append(edges[from], to)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	state := map[string]uint8{}
	var visit func(string) bool
	visit = func(id string) bool {
		if state[id] == 1 {
			return false
		}
		if state[id] == 2 {
			return true
		}
		state[id] = 1
		for _, to := range edges[id] {
			if !visit(to) {
				return false
			}
		}
		state[id] = 2
		return true
	}
	for id := range edges {
		if !visit(id) {
			return ErrInvalid
		}
	}
	return nil
}
