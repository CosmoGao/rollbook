package backup

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/CosmoGao/rollbook/internal/audit"
	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/catalog"
	"github.com/CosmoGao/rollbook/internal/secrets"
)

type ObjectPreview struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Action string `json:"action"`
	Reason string `json:"reason,omitempty"`
}

var mergeTypes = map[string]string{
	"location": "locations", "uplink": "uplinks", "host": "hosts", "network": "networks", "ip": "ips", "service": "services", "port": "ports", "endpoint": "endpoints", "dependency": "dependencies", "version": "versions", "credential": "credentials", "note": "notes", "context": "contexts",
}
var objectBusiness = map[string]bool{"id": true, "type": true, "name": true, "aliases_json": true, "tags_json": true, "status": true, "source": true, "confidence": true, "verified_at": true, "verified_by": true, "rev": true}

func parseMerge(f File) (File, Preview, error) {
	p := Preview{Kind: "merge", Counts: map[string]int{}}
	if len(f.Objects) > 0 {
		if len(f.Tables) > 0 {
			return f, p, ErrInvalid
		}
		f.Tables = map[string][]map[string]any{}
		for _, entry := range f.Objects {
			obj := map[string]any{}
			for k, v := range entry {
				if k != "fields" {
					if k == "aliases" || k == "tags" {
						raw, e := json.Marshal(v)
						if e != nil {
							return f, p, ErrInvalid
						}
						obj[k+"_json"] = string(raw)
					} else {
						obj[k] = v
					}
				}
			}
			typ, _ := obj["type"].(string)
			table := mergeTypes[typ]
			if table == "" {
				return f, p, ErrInvalid
			}
			fields, ok := entry["fields"].(map[string]any)
			if !ok {
				return f, p, ErrInvalid
			}
			detail := map[string]any{"id": obj["id"]}
			for k, v := range fields {
				if strings.HasSuffix(k, "_json") && v != nil {
					if _, ok := v.(string); !ok {
						raw, e := json.Marshal(v)
						if e != nil {
							return f, p, ErrInvalid
						}
						v = string(raw)
					}
				}
				detail[k] = v
			}
			f.Tables["objects"] = append(f.Tables["objects"], obj)
			f.Tables[table] = append(f.Tables[table], detail)
		}
		f.Objects = nil
	}
	if len(f.Tables) == 0 {
		return f, p, ErrInvalid
	}
	ids := map[string]bool{}
	for table, rows := range f.Tables {
		if table != "objects" {
			valid := false
			for _, t := range mergeTypes {
				if t == table {
					valid = true
				}
			}
			if !valid {
				return f, p, ErrInvalid
			}
		}
		p.Counts[table] = len(rows)
		for _, row := range rows {
			for _, field := range []string{"mgmt_url", "docs_url", "url", "latest_url"} {
				if value, ok := row[field]; ok && value != nil && fmt.Sprint(value) != "" && !catalog.ValidHTTPURL(fmt.Sprint(value)) {
					return f, p, &catalog.ValidationError{Fields: []catalog.FieldError{{Field: "fields." + field, Reason: fmt.Sprintf("%s 中对象 %v 仅接受绝对 HTTP 或 HTTPS 地址", table, row["id"]), Expected: "http:// 或 https:// 地址"}}}
				}
			}
			id, _ := row["id"].(string)
			if id == "" {
				return f, p, ErrInvalid
			}
			if secrets.Check(row) != nil {
				return f, p, ErrInvalid
			}
			key := table + ":" + id
			if ids[key] {
				return f, p, fmt.Errorf("%w: duplicate %s", ErrInvalid, key)
			}
			ids[key] = true
			if table == "objects" {
				typ, _ := row["type"].(string)
				if mergeTypes[typ] == "" || !strings.HasPrefix(id, typ+":") {
					return f, p, ErrInvalid
				}
				for col := range row {
					if !objectBusiness[col] {
						return f, p, fmt.Errorf("%w: objects.%s", ErrInvalid, col)
					}
				}
			} else if _, ok := row["rev"]; ok {
				return f, p, ErrInvalid
			}
		}
	}
	if len(f.Tables["objects"]) == 0 {
		return f, p, ErrInvalid
	}
	for _, row := range f.Tables["objects"] {
		id := row["id"].(string)
		typ := row["type"].(string)
		if !ids[mergeTypes[typ]+":"+id] {
			return f, p, fmt.Errorf("%w: missing %s fields", ErrInvalid, id)
		}
	}
	for table, rows := range f.Tables {
		if table == "objects" {
			continue
		}
		for _, row := range rows {
			if !ids["objects:"+row["id"].(string)] {
				return f, p, ErrInvalid
			}
		}
	}
	return f, p, nil
}

func orderedMerge(f File) []map[string]any {
	rows := append([]map[string]any{}, f.Tables["objects"]...)
	// 先处理父对象；关系校验在整批写入后进行。
	sort.Slice(rows, func(i, j int) bool { return fmt.Sprint(rows[i]["id"]) < fmt.Sprint(rows[j]["id"]) })
	return rows
}

func (s Service) MergePreview(ctx context.Context, f File) (Preview, error) {
	f, p, err := parseMerge(f)
	if err != nil {
		return p, err
	}
	for _, row := range orderedMerge(f) {
		id := row["id"].(string)
		typ := row["type"].(string)
		item := ObjectPreview{ID: id, Type: typ, Action: "create"}
		var currentType string
		var rev int64
		err = s.DB.QueryRowContext(ctx, "SELECT type,rev FROM objects WHERE id=?", id).Scan(&currentType, &rev)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return p, err
		}
		if err == nil {
			item.Action = "update"
			expected, ok := asRev(row["rev"])
			if currentType != typ {
				item.Action = "conflict"
				item.Reason = "对象类型不一致"
			} else if !ok {
				item.Action = "conflict"
				item.Reason = "更新必须提供 rev"
			} else if expected != rev {
				item.Action = "conflict"
				item.Reason = fmt.Sprintf("rev 不一致：当前 %d，文件 %d", rev, expected)
			}
		} else if _, ok := row["rev"]; ok {
			item.Action = "conflict"
			item.Reason = "新对象不应提供 rev"
		}
		if item.Action != "conflict" {
			for _, detail := range f.Tables[mergeTypes[typ]] {
				if detail["id"] != id {
					continue
				}
				for field, value := range detail {
					if !strings.HasSuffix(field, "_id") || field == "id" || field == "vlan_id" || value == nil || value == "" {
						continue
					}
					ref, ok := value.(string)
					if !ok {
						item.Action = "conflict"
						item.Reason = field + " 必须为对象 ID"
						break
					}
					found := false
					for _, candidate := range f.Tables["objects"] {
						if candidate["id"] == ref {
							found = true
							break
						}
					}
					if !found {
						var n int
						err = s.DB.QueryRowContext(ctx, "SELECT count(*) FROM objects WHERE id=?", ref).Scan(&n)
						if err != nil {
							return p, err
						}
						found = n > 0
					}
					if !found {
						item.Action = "conflict"
						item.Reason = field + " 引用的对象不存在"
						break
					}
				}
				break
			}
		}
		p.Objects = append(p.Objects, item)
		p.Counts[item.Action]++
	}
	return p, nil
}

func asRev(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int64:
		return n, true
	case float64:
		if n == float64(int64(n)) {
			return int64(n), true
		}
	}
	return 0, false
}

func columnsFor(ctx context.Context, tx *sql.Tx, table string) (map[string]bool, error) {
	cols, err := columns(ctx, tx, table)
	m := map[string]bool{}
	for _, c := range cols {
		m[c] = true
	}
	return m, err
}
func insertPartial(ctx context.Context, tx *sql.Tx, table string, row map[string]any) error {
	allowed, err := columnsFor(ctx, tx, table)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(row))
	for col := range row {
		if !allowed[col] {
			return fmt.Errorf("%w: %s.%s", ErrInvalid, table, col)
		}
		names = append(names, col)
	}
	sort.Strings(names)
	vals := make([]any, len(names))
	for i, c := range names {
		vals[i] = row[c]
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO "+table+"("+strings.Join(names, ",")+") VALUES("+strings.TrimSuffix(strings.Repeat("?,", len(names)), ",")+")", vals...)
	return err
}
func updatePartial(ctx context.Context, tx *sql.Tx, table string, row map[string]any) error {
	allowed, err := columnsFor(ctx, tx, table)
	if err != nil {
		return err
	}
	names := []string{}
	for c := range row {
		if c == "id" || c == "type" || c == "rev" {
			continue
		}
		if !allowed[c] {
			return fmt.Errorf("%w: %s.%s", ErrInvalid, table, c)
		}
		names = append(names, c)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return nil
	}
	sets := []string{}
	vals := []any{}
	for _, c := range names {
		sets = append(sets, c+"=?")
		vals = append(vals, row[c])
	}
	vals = append(vals, row["id"])
	_, err = tx.ExecContext(ctx, "UPDATE "+table+" SET "+strings.Join(sets, ",")+" WHERE id=?", vals...)
	return err
}

func (s Service) merge(ctx context.Context, id string, f File, actor auth.Principal, reviewer string) error {
	var parseErr error
	f, _, parseErr = parseMerge(f)
	if parseErr != nil {
		return parseErr
	}
	p, err := s.MergePreview(ctx, f)
	if err != nil {
		return err
	}
	for _, item := range p.Objects {
		if item.Action == "conflict" {
			return fmt.Errorf("%w: %s: %s", ErrConflict, item.ID, item.Reason)
		}
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "PRAGMA defer_foreign_keys=ON"); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	proposer := actor.Login
	if actor.KeyID != "" {
		proposer = actor.KeyName + " (" + actor.IssuerName + ")"
	}
	for _, item := range p.Objects {
		var obj map[string]any
		for _, r := range f.Tables["objects"] {
			if r["id"] == item.ID {
				obj = r
				break
			}
		}
		var detail map[string]any
		for _, r := range f.Tables[mergeTypes[item.Type]] {
			if r["id"] == item.ID {
				detail = r
				break
			}
		}
		if obj == nil || detail == nil {
			return ErrInvalid
		}
		row := map[string]any{}
		for k, v := range obj {
			if k != "rev" {
				row[k] = v
			}
		}
		if item.Action == "create" {
			if _, ok := row["name"]; !ok {
				return fmt.Errorf("%w: %s name required", ErrInvalid, item.ID)
			}
			row["created_at"] = now
			row["updated_at"] = now
			row["created_by"] = proposer
			row["updated_by"] = proposer
			if err = insertPartial(ctx, tx, "objects", row); err != nil {
				return err
			}
			if err = insertPartial(ctx, tx, mergeTypes[item.Type], detail); err != nil {
				return err
			}
		} else {
			var rev int64
			var typ string
			err = tx.QueryRowContext(ctx, "SELECT type,rev FROM objects WHERE id=?", item.ID).Scan(&typ, &rev)
			if err != nil {
				return err
			}
			expected, _ := asRev(obj["rev"])
			if typ != item.Type || rev != expected {
				return fmt.Errorf("%w: %s rev changed", ErrConflict, item.ID)
			}
			row["updated_at"] = now
			row["updated_by"] = proposer
			if err = updatePartial(ctx, tx, "objects", row); err != nil {
				return err
			}
			if err = updatePartial(ctx, tx, mergeTypes[item.Type], detail); err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, "UPDATE objects SET rev=rev+1 WHERE id=?", item.ID); err != nil {
				return err
			}
		}
	}
	// 延后校验外键，可接受文件中对象彼此引用。
	rows, err := tx.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return err
	}
	bad := rows.Next()
	rows.Close()
	if bad {
		return fmt.Errorf("%w: foreign key", ErrInvalid)
	}
	if err = validateRelations(ctx, tx); err != nil {
		return err
	}
	for _, item := range p.Objects {
		if _, err = tx.ExecContext(ctx, "DELETE FROM search_documents WHERE object_id=?", item.ID); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO search_documents(object_id,name,aliases,tags,description,url)
 SELECT o.id,o.name,coalesce((SELECT group_concat(value,' ') FROM json_each(o.aliases_json)),''),coalesce((SELECT group_concat(value,' ') FROM json_each(o.tags_json)),''),coalesce(l.description,''),coalesce(e.url,'') FROM objects o LEFT JOIN locations l ON l.id=o.id LEFT JOIN endpoints e ON e.id=o.id WHERE o.id=?`, item.ID); err != nil {
			return err
		}
		var obj map[string]any
		for _, r := range f.Tables["objects"] {
			if r["id"] == item.ID {
				obj = r
				break
			}
		}
		rev, _ := asRev(obj["rev"])
		_, err = audit.AppendTx(ctx, tx, audit.Entry{ObjectID: item.ID, ObjectType: item.Type, Action: item.Action, Diff: map[string]any{"object": obj, "fields": findDetail(f, item)}, Actor: actor, Via: "import", Reason: "批准合并导入", State: "已生效", BaseRev: rev, ReviewedBy: reviewer, ReviewedAt: now})
		if err != nil {
			return err
		}
	}
	result, err := tx.ExecContext(ctx, "UPDATE pending_imports SET state='approved',yaml=x'',reviewed_by=?,reviewed_at=? WHERE id=? AND state='pending'", reviewer, now, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return sql.ErrNoRows
	}
	return tx.Commit()
}
func findDetail(f File, item ObjectPreview) map[string]any {
	for _, r := range f.Tables[mergeTypes[item.Type]] {
		if r["id"] == item.ID {
			return r
		}
	}
	return nil
}
