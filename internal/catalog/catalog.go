// Package catalog owns catalog persistence and relation validation.
package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/netip"
	"sort"
	"strings"
	"time"
)

type Object struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Name       string         `json:"name"`
	Aliases    []string       `json:"aliases"`
	Tags       []string       `json:"tags"`
	Status     string         `json:"status"`
	Source     string         `json:"source"`
	Confidence string         `json:"confidence"`
	VerifiedAt *time.Time     `json:"verified_at,omitempty"`
	VerifiedBy string         `json:"verified_by,omitempty"`
	Rev        int64          `json:"rev"`
	CreatedAt  time.Time      `json:"created_at"`
	CreatedBy  string         `json:"created_by"`
	UpdatedAt  time.Time      `json:"updated_at"`
	UpdatedBy  string         `json:"updated_by"`
	Fields     map[string]any `json:"fields"`
}

type Store struct{ DB *sql.DB }

var columns = map[string][]string{
	"location":   {"kind", "parent_id", "city", "region", "address", "timezone", "description"},
	"uplink":     {"location_id", "isp", "type", "bandwidth", "public_ipv4", "public_ipv4_addr", "public_ipv6", "ddns", "inbound_ports_json", "edge_device_id", "account_ref"},
	"host":       {"kind", "net_role", "uplink_id", "parent_host_id", "location_id", "model", "os", "os_version", "resources_json", "mgmt_url"},
	"network":    {"cidr", "location_id", "router_id", "uplink_id", "vlan_id", "gateway", "dhcp_range", "purpose"},
	"ip":         {"address", "network_id", "host_id", "interface", "mac", "assign"},
	"service":    {"host_id", "kind", "deploy_method", "upstream_json", "owner", "config_path", "docs_url"},
	"port":       {"service_id", "bind", "port", "protocol", "purpose"},
	"endpoint":   {"service_id", "kind", "url", "via_json", "auth", "exposure", "nav_card_json"},
	"dependency": {"from_id", "to_id", "kind", "note", "generated"},
	"version":    {"service_id", "current_version", "current_source", "latest_version", "latest_source", "latest_url", "eol_date", "pinned", "pinned_reason", "checked_at"},
	"credential": {"target_id", "vault", "item_ref", "purpose"},
	"note":       {"target_id", "kind", "body_md", "author"},
	"context":    {"scope", "project", "key", "body_md", "owner", "related_ids_json", "expires_at", "archived_at"},
	"change":     {"object_type", "object_id", "action", "diff_json", "actor", "actor_type", "actor_key_id", "actor_key_name", "actor_issuer_id", "actor_issuer_name", "via", "reason", "state", "base_rev", "reviewed_by", "reviewed_at"},
}
var tables = map[string]string{"location": "locations", "uplink": "uplinks", "host": "hosts", "network": "networks", "ip": "ips", "service": "services", "port": "ports", "endpoint": "endpoints", "dependency": "dependencies", "version": "versions", "credential": "credentials", "note": "notes", "context": "contexts", "change": "changes"}
var jsonFields = map[string]bool{"inbound_ports_json": true, "resources_json": true, "upstream_json": true, "via_json": true, "nav_card_json": true, "related_ids_json": true, "diff_json": true}

// EditableFields returns a copy of the business fields accepted for this type.
func EditableFields(typ string) []string { return append([]string(nil), columns[typ]...) }

// Create validates and inserts an object atomically. Actor is supplied by the caller's auth layer.
func (s Store) Create(ctx context.Context, o Object, actor string) (Object, error) {
	return s.create(ctx, o, actor, nil)
}
func (s Store) create(ctx context.Context, o Object, actor string, after func(*sql.Tx, Object) error) (Object, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return o, err
	}
	defer tx.Rollback()
	o, err = s.createTx(ctx, tx, o, actor, after)
	if err != nil {
		return o, err
	}
	return o, tx.Commit()
}
func (s Store) createTx(ctx context.Context, tx *sql.Tx, o Object, actor string, after func(*sql.Tx, Object) error) (Object, error) {
	if err := checkCreateSecrets(o); err != nil {
		return o, err
	}
	if _, ok := columns[o.Type]; !ok {
		return o, invalidField("object.type", "未知对象类型", "现有模型类型")
	}
	if o.ID == "" || !strings.HasPrefix(o.ID, o.Type+":") {
		return o, invalidField("object.id", "ID 缺失或类型前缀不符", o.Type+":...")
	}
	if o.Name == "" {
		return o, invalidField("object.name", "名称必填", "非空字符串")
	}
	if actor == "" {
		return o, ErrInvalid
	}
	if o.Rev != 0 || !o.CreatedAt.IsZero() || !o.UpdatedAt.IsZero() {
		return o, invalidField("object", "审计字段由服务端生成", "省略 rev 和时间字段")
	}
	if o.Fields == nil {
		o.Fields = map[string]any{}
	}
	if err := validate(ctx, tx, &o); err != nil {
		return o, err
	}
	now := time.Now().UTC()
	o.CreatedAt = now
	o.UpdatedAt = now
	o.CreatedBy = actor
	o.UpdatedBy = actor
	o.Rev = 1
	if o.Status == "" {
		o.Status = "在用"
	}
	if o.Source == "" {
		o.Source = "人工"
	}
	if o.Confidence == "" {
		o.Confidence = "低"
	}
	aliases, _ := json.Marshal(nonNil(o.Aliases))
	tags, _ := json.Marshal(nonNil(o.Tags))
	_, err := tx.ExecContext(ctx, `INSERT INTO objects(id,type,name,aliases_json,tags_json,status,source,confidence,verified_at,verified_by,rev,created_at,created_by,updated_at,updated_by) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, o.ID, o.Type, o.Name, string(aliases), string(tags), o.Status, o.Source, o.Confidence, timeText(o.VerifiedAt), null(o.VerifiedBy), o.Rev, now.Format(time.RFC3339Nano), actor, now.Format(time.RFC3339Nano), actor)
	if err != nil {
		return o, storageError(err, o)
	}
	cols := columns[o.Type]
	args := []any{o.ID}
	marks := []string{"?"}
	names := []string{"id"}
	for _, c := range cols {
		v := o.Fields[c]
		if jsonFields[c] && v != nil {
			switch x := v.(type) {
			case string:
				if !json.Valid([]byte(x)) {
					return o, invalidField("fields."+c, "JSON 格式无效", "有效 JSON")
				}
			default:
				b, e := json.Marshal(v)
				if e != nil {
					return o, e
				}
				v = string(b)
			}
		}
		if v == nil {
			if c == "generated" || c == "pinned" {
				v = 0
			}
			if c == "via_json" || c == "related_ids_json" || c == "inbound_ports_json" {
				v = "[]"
			}
			if c == "resources_json" {
				v = "{}"
			}
		}
		names = append(names, c)
		marks = append(marks, "?")
		args = append(args, v)
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO "+tables[o.Type]+"("+strings.Join(names, ",")+") VALUES("+strings.Join(marks, ",")+")", args...)
	if err != nil {
		return o, storageError(err, o)
	}
	if err = generate(ctx, tx, o, actor); err != nil {
		return o, err
	}
	if err = index(ctx, tx, o); err != nil {
		return o, err
	}
	if after != nil {
		if err = after(tx, o); err != nil {
			return o, err
		}
	}
	return o, nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
func null(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func timeText(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339Nano)
}
func str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func validate(ctx context.Context, tx *sql.Tx, o *Object) error {
	allowed := map[string]bool{}
	for _, c := range columns[o.Type] {
		allowed[c] = true
	}
	for k := range o.Fields {
		if !allowed[k] {
			return invalidField("fields."+k, "对象类型不支持该字段", o.Type)
		}
	}
	refTypes := map[string]string{"parent_id": "location", "location_id": "location", "uplink_id": "uplink", "edge_device_id": "host", "parent_host_id": "host", "router_id": "host", "host_id": "host", "network_id": "network", "service_id": "service", "from_id": "*", "to_id": "*", "target_id": "*", "account_ref": "credential"}
	for field, expected := range refTypes {
		if !allowed[field] || str(o.Fields[field]) == "" {
			continue
		}
		var actual string
		err := tx.QueryRowContext(ctx, "SELECT type FROM objects WHERE id=?", str(o.Fields[field])).Scan(&actual)
		if err == sql.ErrNoRows {
			return invalidField("fields."+field, "关联对象不存在", expected)
		}
		if err != nil {
			return err
		}
		if expected != "*" && actual != expected {
			return invalidField("fields."+field, "关联对象类型不符", expected)
		}
	}
	if o.Type == "network" {
		p, e := netip.ParsePrefix(str(o.Fields["cidr"]))
		if e != nil {
			return invalidField("fields.cidr", "CIDR 格式无效", "规范网段地址/前缀")
		}
		if p != p.Masked() {
			return invalidField("fields.cidr", "CIDR 必须使用规范网段地址", "网络地址/前缀")
		}
	}
	if o.Type == "ip" {
		a, e := netip.ParseAddr(str(o.Fields["address"]))
		if e != nil {
			return invalidField("fields.address", "IP 地址格式无效", "IPv4 或 IPv6 地址")
		}
		if a.Is4In6() {
			return invalidField("fields.address", "不支持映射 IPv4", "IPv4 或 IPv6 地址")
		}
		var site string
		e = tx.QueryRowContext(ctx, `WITH RECURSIVE parents(id,parent_id,kind) AS (
 SELECT l.id,l.parent_id,l.kind FROM hosts h JOIN locations l ON l.id=h.location_id WHERE h.id=?
 UNION ALL SELECT l.id,l.parent_id,l.kind FROM locations l JOIN parents p ON p.parent_id=l.id
 ) SELECT id FROM parents WHERE kind='站点' LIMIT 1`, str(o.Fields["host_id"])).Scan(&site)
		if e != nil {
			return invalidField("fields.host_id", "主机不存在或不属于站点", "站点内现有主机")
		}
		rows, e := tx.QueryContext(ctx, `WITH RECURSIVE places(id) AS (SELECT ? UNION ALL SELECT l.id FROM locations l JOIN places p ON l.parent_id=p.id)
 SELECT n.id,n.cidr FROM networks n JOIN places p ON n.location_id=p.id`, site)
		if e != nil {
			return e
		}
		defer rows.Close()
		best := -1
		matches := []string{}
		for rows.Next() {
			var id, cidr string
			if e = rows.Scan(&id, &cidr); e != nil {
				return e
			}
			p, e := netip.ParsePrefix(cidr)
			if e != nil {
				return e
			}
			if p.Contains(a) {
				if p.Bits() > best {
					best = p.Bits()
					matches = nil
				}
				if p.Bits() == best {
					matches = append(matches, id)
				}
			}
		}
		if e = rows.Err(); e != nil {
			return e
		}
		if len(matches) != 1 {
			return invalidField("fields.address", "IP 不在主机站点的唯一最长前缀网段内", "站点网段内地址")
		}
		if supplied := str(o.Fields["network_id"]); supplied != "" && supplied != matches[0] {
			return invalidField("fields.network_id", "所选网段与 IP 地址不符", matches[0])
		}
		o.Fields["network_id"] = matches[0]
	}
	if o.Type == "host" || o.Type == "location" {
		field := "parent_host_id"
		table := "hosts"
		if o.Type == "location" {
			field = "parent_id"
			table = "locations"
		}
		parent := str(o.Fields[field])
		seen := map[string]bool{o.ID: true}
		for parent != "" {
			if seen[parent] {
				return invalidField("fields."+field, "父级关系形成循环", "不形成循环的父级")
			}
			seen[parent] = true
			var next sql.NullString
			e := tx.QueryRowContext(ctx, "SELECT "+field+" FROM "+table+" WHERE id=?", parent).Scan(&next)
			if e == sql.ErrNoRows {
				return invalidField("fields."+field, "父级对象不存在", "现有父级对象")
			}
			if e != nil {
				return e
			}
			parent = next.String
		}
	}
	if o.Type == "endpoint" {
		ids, e := stringList(o.Fields["via_json"])
		if e != nil {
			return invalidField("fields.via_json", "反代列表格式无效", "服务 ID 数组")
		}
		seen := map[string]bool{}
		for _, id := range ids {
			if seen[id] {
				return invalidField("fields.via_json", "反代列表不能重复", "不重复的服务 ID 数组")
			}
			seen[id] = true
			var typ string
			if e = tx.QueryRowContext(ctx, "SELECT type FROM objects WHERE id=?", id).Scan(&typ); e == sql.ErrNoRows {
				return invalidField("fields.via_json", "反代服务不存在", "现有服务 ID")
			} else if e != nil {
				return e
			}
			if typ != "service" {
				return invalidField("fields.via_json", "反代必须引用服务", "服务 ID")
			}
		}
	}
	if o.Type == "context" {
		ids, e := stringList(o.Fields["related_ids_json"])
		if e != nil {
			return invalidField("fields.related_ids_json", "关联列表格式无效", "对象 ID 数组")
		}
		for _, id := range ids {
			var n int
			if e = tx.QueryRowContext(ctx, "SELECT count(*) FROM objects WHERE id=?", id).Scan(&n); e != nil {
				return e
			}
			if n == 0 {
				return invalidField("fields.related_ids_json", "关联对象不存在", "现有对象 ID")
			}
		}
		if o.Fields["expires_at"] == nil {
			switch str(o.Fields["scope"]) {
			case "会话":
				o.Fields["expires_at"] = time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339Nano)
			case "项目":
				o.Fields["expires_at"] = time.Now().UTC().Add(30 * 24 * time.Hour).Format(time.RFC3339Nano)
			}
		}
	}
	if o.Type == "dependency" {
		from, to := str(o.Fields["from_id"]), str(o.Fields["to_id"])
		if from == to {
			return invalidField("fields.to_id", "依赖不能指向自身", "与 from_id 不同的对象 ID")
		}
		if e := checkCycle(ctx, tx, from, to); e != nil {
			return e
		}
	}
	return nil
}

func stringList(v any) ([]string, error) {
	if v == nil {
		return nil, nil
	}
	switch x := v.(type) {
	case []string:
		return x, nil
	case string:
		var a []string
		e := json.Unmarshal([]byte(x), &a)
		return a, e
	default:
		b, e := json.Marshal(v)
		if e != nil {
			return nil, e
		}
		var a []string
		e = json.Unmarshal(b, &a)
		return a, e
	}
}
func checkCycle(ctx context.Context, tx *sql.Tx, from, to string) error {
	if from == to {
		return invalidField("fields.to_id", "依赖形成循环", "不形成循环的对象 ID")
	}
	var n int
	e := tx.QueryRowContext(ctx, `WITH RECURSIVE reach(id) AS (SELECT to_id FROM dependencies WHERE from_id=? UNION SELECT d.to_id FROM dependencies d JOIN reach r ON d.from_id=r.id) SELECT count(*) FROM reach WHERE id=?`, to, from).Scan(&n)
	if e != nil {
		return e
	}
	if n > 0 {
		return invalidField("fields.to_id", "依赖形成循环", "不形成循环的对象 ID")
	}
	return nil
}

func generate(ctx context.Context, tx *sql.Tx, o Object, actor string) error {
	edges := [][3]string{}
	if o.Type == "host" && str(o.Fields["parent_host_id"]) != "" {
		edges = append(edges, [3]string{o.ID, str(o.Fields["parent_host_id"]), "宿主"})
	}
	if o.Type == "service" {
		edges = append(edges, [3]string{o.ID, str(o.Fields["host_id"]), "宿主"})
	}
	if o.Type == "endpoint" {
		ids, _ := stringList(o.Fields["via_json"])
		// 反代链属于入口；全局依赖图只记录服务使用的第一跳。
		if len(ids) > 0 {
			edges = append(edges, [3]string{str(o.Fields["service_id"]), ids[0], "反代"})
		}
	}
	for i, edge := range edges {
		var existing int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM dependencies WHERE from_id=? AND to_id=? AND kind=?`, edge[0], edge[1], edge[2]).Scan(&existing); err != nil {
			return err
		}
		if existing != 0 {
			continue
		}
		if err := checkCycle(ctx, tx, edge[0], edge[1]); err != nil {
			return err
		}
		id := fmt.Sprintf("dependency:auto:%s:%d", o.ID, i)
		now := time.Now().UTC().Format(time.RFC3339Nano)
		_, err := tx.ExecContext(ctx, `INSERT INTO objects(id,type,name,created_at,created_by,updated_at,updated_by) VALUES(?,'dependency',?,?,?,?,?)`, id, edge[2], now, actor, now, actor)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO dependencies(id,from_id,to_id,kind,generated) VALUES(?,?,?,?,1)`, id, edge[0], edge[1], edge[2])
		if err != nil {
			return err
		}
	}
	return nil
}

func index(ctx context.Context, tx *sql.Tx, o Object) error {
	description := str(o.Fields["description"])
	url := str(o.Fields["url"])
	_, err := tx.ExecContext(ctx, `INSERT INTO search_documents(object_id,name,aliases,tags,description,url) VALUES(?,?,?,?,?,?)`, o.ID, o.Name, strings.Join(o.Aliases, " "), strings.Join(o.Tags, " "), description, url)
	return err
}

// Get loads public fields and all type-specific fields.
func (s Store) Get(ctx context.Context, id string) (Object, error) {
	return get(ctx, s.DB, id)
}

type rowQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func get(ctx context.Context, q rowQuerier, id string) (Object, error) {
	var o Object
	var aliases, tags, created, updated string
	var verified sql.NullString
	var verifiedBy sql.NullString
	err := q.QueryRowContext(ctx, `SELECT id,type,name,aliases_json,tags_json,status,source,confidence,verified_at,verified_by,rev,created_at,created_by,updated_at,updated_by FROM objects WHERE id=?`, id).Scan(&o.ID, &o.Type, &o.Name, &aliases, &tags, &o.Status, &o.Source, &o.Confidence, &verified, &verifiedBy, &o.Rev, &created, &o.CreatedBy, &updated, &o.UpdatedBy)
	if err != nil {
		return o, err
	}
	json.Unmarshal([]byte(aliases), &o.Aliases)
	json.Unmarshal([]byte(tags), &o.Tags)
	o.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	o.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	if verified.Valid {
		t, _ := time.Parse(time.RFC3339Nano, verified.String)
		o.VerifiedAt = &t
	}
	o.VerifiedBy = verifiedBy.String
	o.Fields = map[string]any{}
	cols := columns[o.Type]
	raw := make([]any, len(cols))
	dest := make([]any, len(cols))
	for i := range raw {
		dest[i] = &raw[i]
	}
	row := q.QueryRowContext(ctx, "SELECT "+strings.Join(cols, ",")+" FROM "+tables[o.Type]+" WHERE id=?", id)
	if err = row.Scan(dest...); err != nil {
		return o, err
	}
	for i, c := range cols {
		if raw[i] != nil {
			o.Fields[c] = raw[i]
		}
	}
	return o, nil
}

// ArchiveExpired keeps context rows but hides them from active queries.
func (s Store) ArchiveExpired(ctx context.Context, now time.Time) (int64, error) {
	r, e := s.DB.ExecContext(ctx, `UPDATE contexts SET archived_at=? WHERE archived_at IS NULL AND expires_at IS NOT NULL AND expires_at<=?`, now.UTC().Format(time.RFC3339Nano), now.UTC().Format(time.RFC3339Nano))
	if e != nil {
		return 0, e
	}
	return r.RowsAffected()
}
func (s Store) Contexts(ctx context.Context, archived bool, now time.Time) ([]Object, error) {
	if _, e := s.ArchiveExpired(ctx, now); e != nil {
		return nil, e
	}
	op := "IS NULL"
	if archived {
		op = "IS NOT NULL"
	}
	rows, e := s.DB.QueryContext(ctx, "SELECT id FROM contexts WHERE archived_at "+op+" ORDER BY id")
	if e != nil {
		return nil, e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return nil, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	out := []Object{}
	for _, id := range ids {
		o, e := s.Get(ctx, id)
		if e != nil {
			return nil, e
		}
		out = append(out, o)
	}
	return out, nil
}
func (s Store) Unverified(ctx context.Context, now time.Time) ([]string, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT id FROM objects WHERE verified_at IS NULL OR verified_at<? ORDER BY id`, now.UTC().Add(-30*24*time.Hour).Format(time.RFC3339Nano))
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			return nil, e
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
func sorted(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
