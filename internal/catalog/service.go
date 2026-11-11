package catalog

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/CosmoGao/rollbook/internal/audit"
	"github.com/CosmoGao/rollbook/internal/auth"
)

var ErrConflict = errors.New("revision conflict")
var ErrInvalid = errors.New("invalid change")

type Service struct{ Store Store }
type Patch struct {
	Name       *string        `json:"name,omitempty"`
	Aliases    *[]string      `json:"aliases,omitempty"`
	Tags       *[]string      `json:"tags,omitempty"`
	Status     *string        `json:"status,omitempty"`
	Source     *string        `json:"source,omitempty"`
	Confidence *string        `json:"confidence,omitempty"`
	Fields     map[string]any `json:"fields,omitempty"`
}

// CreateAsset accepts a new object at base revision zero and audits its creation.
func (s Service) CreateAsset(ctx context.Context, p auth.Principal, o Object, baseRev int64, reason, via string) (Object, error) {
	if e := auth.Authorize(p, auth.Update, ""); e != nil {
		return o, e
	}
	if e := prepareCreate(&o, baseRev, reason, via, p); e != nil {
		return o, e
	}
	return s.Store.create(ctx, o, actorName(p), func(tx *sql.Tx, created Object) error {
		_, err := audit.AppendTx(ctx, tx, audit.Entry{ObjectID: created.ID, ObjectType: created.Type, Action: "create", Diff: created, Actor: p, Via: via, Reason: reason, State: "已生效", BaseRev: 0})
		return err
	})
}

func prepareCreate(o *Object, baseRev int64, reason, via string, p auth.Principal) error {
	if CheckSecrets(reason) != nil {
		return secretField("reason")
	}
	if strings.TrimSpace(reason) == "" {
		return invalidField("reason", "操作原因必填", "非空字符串")
	}
	if baseRev != 0 {
		return invalidField("base_rev", "创建时版本必须为 0", "0")
	}
	if via == "" || o.Type == "context" || o.Type == "change" {
		return ErrInvalid
	}
	if o.CreatedBy != "" || o.UpdatedBy != "" || o.VerifiedBy != "" || o.VerifiedAt != nil || o.Rev != 0 || !o.CreatedAt.IsZero() || !o.UpdatedAt.IsZero() {
		return ErrInvalid
	}
	if o.Fields == nil {
		o.Fields = map[string]any{}
	}
	if o.Type == "dependency" {
		if _, supplied := o.Fields["generated"]; supplied {
			return invalidField("fields.generated", "手工依赖不能设置自动生成标志", "省略此字段")
		}
	}
	if o.Type == "note" {
		o.Fields["author"] = actorName(p)
	}
	return nil
}

// ProposeCreate 先校验候选对象；审批时在落库事务中再次校验。
func (s Service) ProposeCreate(ctx context.Context, p auth.Principal, o Object, baseRev int64, reason, via string) (string, error) {
	if e := auth.Authorize(p, auth.ProposeChange, ""); e != nil {
		return "", e
	}
	if e := prepareCreate(&o, baseRev, reason, via, p); e != nil {
		return "", e
	}
	if e := checkCreateSecrets(o); e != nil {
		return "", e
	}
	if _, ok := columns[o.Type]; !ok {
		return "", ErrInvalid
	}
	if o.ID == "" || o.Name == "" || !strings.HasPrefix(o.ID, o.Type+":") {
		return "", ErrInvalid
	}
	tx, e := s.Store.DB.BeginTx(ctx, nil)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	var n int
	if e = tx.QueryRowContext(ctx, "SELECT count(*) FROM objects WHERE id=?", o.ID).Scan(&n); e != nil {
		return "", e
	}
	if n != 0 {
		return "", ErrConflict
	}
	if _, e = tx.ExecContext(ctx, "SAVEPOINT proposal_check"); e != nil {
		return "", e
	}
	_, e = s.Store.createTx(ctx, tx, o, actorName(p), nil)
	if _, rollbackErr := tx.ExecContext(ctx, "ROLLBACK TO proposal_check"); rollbackErr != nil {
		return "", rollbackErr
	}
	if _, releaseErr := tx.ExecContext(ctx, "RELEASE proposal_check"); releaseErr != nil {
		return "", releaseErr
	}
	if e != nil {
		return "", e
	}
	change, e := audit.AppendTx(ctx, tx, audit.Entry{ObjectID: o.ID, ObjectType: o.Type, Action: "create", Diff: o, Actor: p, Via: via, Reason: reason, State: "待确认", BaseRev: 0})
	if e != nil {
		return "", e
	}
	return change, tx.Commit()
}

func reasonOK(reason string) bool {
	return strings.TrimSpace(reason) != "" && CheckSecrets(reason) == nil
}
func actorName(p auth.Principal) string {
	if p.IsUser() {
		return p.Login
	}
	return p.KeyName + " (" + p.IssuerName + ")"
}
func actorID(p auth.Principal) string {
	if p.IsUser() {
		return p.UserID
	}
	return p.KeyID
}
func newID(prefix string) (string, error) {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return prefix + hex.EncodeToString(b), nil
}

// Update applies a typed patch with a mandatory revision and audit row in one transaction.
func (s Service) Update(ctx context.Context, p auth.Principal, id string, rev int64, patch Patch, reason, via string) (Object, error) {
	if e := auth.Authorize(p, auth.Update, ""); e != nil {
		return Object{}, e
	}
	if rev < 1 || !reasonOK(reason) || via == "" {
		return Object{}, ErrInvalid
	}
	if e := checkPatchSecrets(patch); e != nil {
		return Object{}, e
	}
	tx, e := s.Store.DB.BeginTx(ctx, nil)
	if e != nil {
		return Object{}, e
	}
	defer tx.Rollback()
	o, e := s.applyUpdate(ctx, tx, id, rev, patch, actorName(p))
	if e != nil {
		return Object{}, e
	}
	_, e = audit.AppendTx(ctx, tx, audit.Entry{ObjectID: id, ObjectType: o.Type, Action: "update", Diff: patch, Actor: p, Via: via, Reason: reason, State: "已生效", BaseRev: rev})
	if e != nil {
		return Object{}, e
	}
	if e = tx.Commit(); e != nil {
		return Object{}, e
	}
	return o, nil
}
func checkPatchSecrets(patch Patch) error {
	for field, value := range map[string]any{"name": patch.Name, "aliases": patch.Aliases, "tags": patch.Tags, "status": patch.Status, "source": patch.Source, "confidence": patch.Confidence} {
		switch v := value.(type) {
		case *string:
			if v != nil {
				value = *v
			}
		case *[]string:
			if v != nil {
				value = *v
			}
		}
		if CheckSecrets(value) != nil {
			return secretField(field)
		}
	}
	for field, value := range patch.Fields {
		if CheckSecrets(map[string]any{field: value}) != nil {
			return secretField("fields." + field)
		}
	}
	return nil
}

func (s Service) applyUpdate(ctx context.Context, tx *sql.Tx, id string, rev int64, patch Patch, actor string) (Object, error) {
	o, e := get(ctx, tx, id)
	if e != nil {
		return o, e
	}
	if o.Rev != rev {
		return o, ErrConflict
	}
	if o.Type == "change" || o.Type == "context" {
		return o, auth.ErrForbidden
	}
	if o.Type == "note" {
		if _, ok := patch.Fields["author"]; ok {
			return o, invalidField("fields.author", "作者由服务端维护", "省略此字段")
		}
	}
	if o.Type == "dependency" {
		if _, ok := patch.Fields["generated"]; ok {
			return o, invalidField("fields.generated", "不能更改自动生成标志", "省略此字段")
		}
		if str(o.Fields["generated"]) == "1" {
			return o, auth.ErrForbidden
		}
	}
	oldService := str(o.Fields["service_id"])
	oldAddress := str(o.Fields["address"])
	if patch.Name != nil {
		o.Name = *patch.Name
	}
	if patch.Aliases != nil {
		o.Aliases = *patch.Aliases
	}
	if patch.Tags != nil {
		o.Tags = *patch.Tags
	}
	if patch.Status != nil {
		o.Status = *patch.Status
	}
	if patch.Source != nil {
		o.Source = *patch.Source
	}
	if patch.Confidence != nil {
		o.Confidence = *patch.Confidence
	}
	allowed := map[string]bool{}
	for _, c := range columns[o.Type] {
		allowed[c] = true
	}
	for k, v := range patch.Fields {
		if !allowed[k] {
			return o, ErrInvalid
		}
		o.Fields[k] = v
	}
	if o.Type == "ip" && str(o.Fields["address"]) != oldAddress {
		// 旧网段来自已存对象，不应当被当成用户指定的新网段。
		if _, supplied := patch.Fields["network_id"]; !supplied {
			delete(o.Fields, "network_id")
		}
		o.Name = str(o.Fields["address"])
	}
	if o.Name == "" {
		return o, ErrInvalid
	}
	if e = validate(ctx, tx, &o); e != nil {
		return o, e
	}
	if o.Type == "host" {
		if e = validateHostIPs(ctx, tx, o.ID, str(o.Fields["location_id"])); e != nil {
			return o, e
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	aliases, _ := json.Marshal(nonNil(o.Aliases))
	tags, _ := json.Marshal(nonNil(o.Tags))
	r, e := tx.ExecContext(ctx, `UPDATE objects SET name=?,aliases_json=?,tags_json=?,status=?,source=?,confidence=?,rev=rev+1,updated_at=?,updated_by=? WHERE id=? AND rev=?`, o.Name, string(aliases), string(tags), o.Status, o.Source, o.Confidence, now, actor, id, rev)
	if e != nil {
		return o, e
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return o, ErrConflict
	}
	for k, v := range patch.Fields {
		if jsonFields[k] && v != nil {
			switch x := v.(type) {
			case string:
				if !json.Valid([]byte(x)) {
					return o, ErrInvalid
				}
			default:
				b, e := json.Marshal(v)
				if e != nil {
					return o, e
				}
				v = string(b)
			}
		}
		if _, e = tx.ExecContext(ctx, "UPDATE "+tables[o.Type]+" SET "+k+"=? WHERE id=?", v, id); e != nil {
			return o, e
		}
	}
	if o.Type == "ip" {
		if _, e = tx.ExecContext(ctx, "UPDATE ips SET network_id=? WHERE id=?", o.Fields["network_id"], id); e != nil {
			return o, e
		}
	}
	if o.Type == "host" || o.Type == "service" || o.Type == "endpoint" {
		rows, e := tx.QueryContext(ctx, `SELECT id FROM dependencies WHERE generated=1 AND id LIKE ?`, "dependency:auto:"+id+":%")
		if e != nil {
			return o, e
		}
		ids := []string{}
		for rows.Next() {
			var dep string
			if e = rows.Scan(&dep); e != nil {
				rows.Close()
				return o, e
			}
			ids = append(ids, dep)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return o, e
		}
		for _, dep := range ids {
			if _, e = tx.ExecContext(ctx, "DELETE FROM dependencies WHERE id=?", dep); e != nil {
				return o, e
			}
			if _, e = tx.ExecContext(ctx, "DELETE FROM objects WHERE id=?", dep); e != nil {
				return o, e
			}
		}
		if e = generate(ctx, tx, o, actor); e != nil {
			return o, e
		}
		if o.Type == "endpoint" {
			for _, serviceID := range []string{oldService, str(o.Fields["service_id"])} {
				if e = regenerateProxyEdges(ctx, tx, serviceID, actor); e != nil {
					return o, e
				}
			}
		}
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM search_documents WHERE object_id=?", id); e != nil {
		return o, e
	}
	if e = index(ctx, tx, o); e != nil {
		return o, e
	}
	o.Rev++
	o.UpdatedAt, _ = time.Parse(time.RFC3339Nano, now)
	o.UpdatedBy = actor
	return o, nil
}

func validateHostIPs(ctx context.Context, tx *sql.Tx, hostID, locationID string) error {
	var site string
	if err := tx.QueryRowContext(ctx, `WITH RECURSIVE parents(id,parent_id,kind) AS (
 SELECT id,parent_id,kind FROM locations WHERE id=?
 UNION ALL SELECT l.id,l.parent_id,l.kind FROM locations l JOIN parents p ON p.parent_id=l.id
 ) SELECT id FROM parents WHERE kind='站点' LIMIT 1`, locationID).Scan(&site); err != nil {
		return fmt.Errorf("location_id: %w", err)
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,address FROM ips WHERE host_id=?`, hostID)
	if err != nil {
		return err
	}
	type hostIP struct{ id, address string }
	var ips []hostIP
	for rows.Next() {
		var ip hostIP
		if err = rows.Scan(&ip.id, &ip.address); err != nil {
			rows.Close()
			return err
		}
		ips = append(ips, ip)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, ip := range ips {
		addr, err := netip.ParseAddr(ip.address)
		if err != nil {
			return err
		}
		matches, best := []string{}, -1
		networks, err := tx.QueryContext(ctx, `WITH RECURSIVE places(id) AS (
 SELECT ? UNION ALL SELECT l.id FROM locations l JOIN places p ON l.parent_id=p.id
 ) SELECT n.id,n.cidr FROM networks n JOIN places p ON n.location_id=p.id`, site)
		if err != nil {
			return err
		}
		for networks.Next() {
			var id, cidr string
			if err = networks.Scan(&id, &cidr); err != nil {
				networks.Close()
				return err
			}
			prefix, parseErr := netip.ParsePrefix(cidr)
			if parseErr != nil {
				networks.Close()
				return parseErr
			}
			if prefix.Contains(addr) {
				if prefix.Bits() > best {
					best, matches = prefix.Bits(), nil
				}
				if prefix.Bits() == best {
					matches = append(matches, id)
				}
			}
		}
		err = networks.Err()
		networks.Close()
		if err != nil {
			return err
		}
		if len(matches) != 1 {
			return fmt.Errorf("location_id: IP %s (%s) does not belong to exactly one network in site %s", ip.address, ip.id, site)
		}
		if _, err = tx.ExecContext(ctx, `UPDATE ips SET network_id=? WHERE id=?`, matches[0], ip.id); err != nil {
			return err
		}
	}
	return nil
}

func regenerateProxyEdges(ctx context.Context, tx *sql.Tx, serviceID, actor string) error {
	if serviceID == "" {
		return nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT e.id,e.via_json FROM endpoints e WHERE e.service_id=? ORDER BY e.id`, serviceID)
	if err != nil {
		return err
	}
	type endpoint struct {
		id  string
		via []string
	}
	var endpoints []endpoint
	for rows.Next() {
		var id, raw string
		if err = rows.Scan(&id, &raw); err != nil {
			rows.Close()
			return err
		}
		via, parseErr := stringList(raw)
		if parseErr != nil {
			rows.Close()
			return parseErr
		}
		endpoints = append(endpoints, endpoint{id, via})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, ep := range endpoints {
		if err = generate(ctx, tx, Object{ID: ep.id, Type: "endpoint", Fields: map[string]any{"service_id": serviceID, "via_json": ep.via}}, actor); err != nil {
			return err
		}
	}
	return nil
}

// Propose records an immutable review request; approval rechecks its base revision.
func (s Service) Propose(ctx context.Context, p auth.Principal, id string, rev int64, patch Patch, reason, via string) (string, error) {
	if e := auth.Authorize(p, auth.ProposeChange, ""); e != nil {
		return "", e
	}
	if rev < 1 || !reasonOK(reason) || via == "" {
		return "", ErrInvalid
	}
	if e := checkPatchSecrets(patch); e != nil {
		return "", e
	}
	o, e := s.Store.Get(ctx, id)
	if e != nil {
		return "", e
	}
	if o.Rev != rev {
		return "", ErrConflict
	}
	if o.Type == "note" || o.Type == "change" {
		return "", ErrInvalid
	}
	tx, e := s.Store.DB.BeginTx(ctx, nil)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	change, e := audit.AppendTx(ctx, tx, audit.Entry{ObjectID: id, ObjectType: o.Type, Action: "update", Diff: patch, Actor: p, Via: via, Reason: reason, State: "待确认", BaseRev: rev})
	if e != nil {
		return "", e
	}
	return change, tx.Commit()
}

func (s Service) ProposeDelete(ctx context.Context, p auth.Principal, id string, rev int64, reason, via string) (string, error) {
	if e := auth.Authorize(p, auth.ProposeChange, ""); e != nil {
		return "", e
	}
	if rev < 1 || !reasonOK(reason) || via == "" {
		return "", ErrInvalid
	}
	o, e := s.Store.Get(ctx, id)
	if e != nil {
		return "", e
	}
	if o.Rev != rev {
		return "", ErrConflict
	}
	if o.Type == "note" || o.Type == "change" || o.Type == "dependency" {
		return "", ErrInvalid
	}
	tx, e := s.Store.DB.BeginTx(ctx, nil)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	change, e := audit.AppendTx(ctx, tx, audit.Entry{ObjectID: id, ObjectType: o.Type, Action: "delete", Diff: map[string]any{}, Actor: p, Via: via, Reason: reason, State: "待确认", BaseRev: rev})
	if e != nil {
		return "", e
	}
	return change, tx.Commit()
}
func (s Service) Review(ctx context.Context, p auth.Principal, changeID string, approve bool) (Object, error) {
	if e := auth.Authorize(p, auth.Approve, ""); e != nil {
		return Object{}, e
	}
	tx, e := s.Store.DB.BeginTx(ctx, nil)
	if e != nil {
		return Object{}, e
	}
	defer tx.Rollback()
	var id, diff, state, action string
	var rev int64
	e = tx.QueryRowContext(ctx, "SELECT object_id,diff_json,state,action,base_rev FROM changes WHERE id=?", changeID).Scan(&id, &diff, &state, &action, &rev)
	if e != nil {
		return Object{}, e
	}
	if state != "待确认" {
		return Object{}, ErrConflict
	}
	var o Object
	now := time.Now().UTC().Format(time.RFC3339Nano)
	next := "已驳回"
	if approve {
		if action != "create" {
			o, e = get(ctx, tx, id)
			if e != nil {
				return Object{}, e
			}
			if o.Rev != rev {
				return Object{}, ErrConflict
			}
		}
		switch action {
		case "create":
			if rev != 0 || json.Unmarshal([]byte(diff), &o) != nil || o.ID != id {
				return Object{}, ErrInvalid
			}
			if e = checkCreateSecrets(o); e != nil {
				return Object{}, e
			}
			var exists int
			if e = tx.QueryRowContext(ctx, "SELECT count(*) FROM objects WHERE id=?", id).Scan(&exists); e != nil {
				return Object{}, e
			}
			if exists != 0 {
				return Object{}, ErrConflict
			}
			o, e = s.Store.createTx(ctx, tx, o, p.Login, nil)
			if e != nil {
				return Object{}, e
			}
		case "update":
			var patch Patch
			if e = json.Unmarshal([]byte(diff), &patch); e != nil {
				return Object{}, ErrInvalid
			}
			if e = checkPatchSecrets(patch); e != nil {
				return Object{}, e
			}
			o, e = s.applyUpdate(ctx, tx, id, rev, patch, p.Login)
			if e != nil {
				return Object{}, e
			}
		case "delete":
			// Foreign keys reject deletion while another user-owned object still refers to this one.
			rows, queryErr := tx.QueryContext(ctx, "SELECT id FROM dependencies WHERE generated=1 AND (from_id=? OR to_id=?)", id, id)
			if queryErr != nil {
				return Object{}, queryErr
			}
			deps := []string{}
			for rows.Next() {
				var dep string
				if e = rows.Scan(&dep); e != nil {
					rows.Close()
					return Object{}, e
				}
				deps = append(deps, dep)
			}
			e = rows.Err()
			rows.Close()
			if e != nil {
				return Object{}, e
			}
			for _, dep := range deps {
				if _, e = tx.ExecContext(ctx, "DELETE FROM dependencies WHERE id=?", dep); e != nil {
					return Object{}, e
				}
				if _, e = tx.ExecContext(ctx, "DELETE FROM objects WHERE id=?", dep); e != nil {
					return Object{}, e
				}
			}
			if e = referencedBy(ctx, tx, id); e != nil {
				return Object{}, e
			}
			if _, e = tx.ExecContext(ctx, "DELETE FROM search_documents WHERE object_id=?", id); e != nil {
				return Object{}, e
			}
			if _, e = tx.ExecContext(ctx, "DELETE FROM "+tables[o.Type]+" WHERE id=?", id); e != nil {
				return Object{}, e
			}
			if _, e = tx.ExecContext(ctx, "DELETE FROM objects WHERE id=? AND rev=?", id, rev); e != nil {
				return Object{}, e
			}
		default:
			return Object{}, ErrInvalid
		}
		next = "已生效"
	}
	r, e := tx.ExecContext(ctx, "UPDATE changes SET state=?,reviewed_by=?,reviewed_at=? WHERE id=? AND state='待确认'", next, p.Login, now, changeID)
	if e != nil {
		return Object{}, e
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return Object{}, ErrConflict
	}
	if e = tx.Commit(); e != nil {
		return Object{}, e
	}
	return o, nil
}

func referencedBy(ctx context.Context, tx *sql.Tx, id string) error {
	refs := []string{}
	for typ, table := range tables {
		if typ == "change" || typ == "context" {
			continue
		}
		for _, field := range columns[typ] {
			if !strings.HasSuffix(field, "_id") && field != "parent_id" && field != "account_ref" {
				continue
			}
			rows, err := tx.QueryContext(ctx, "SELECT id FROM "+table+" WHERE "+field+"=?", id)
			if err != nil {
				return err
			}
			for rows.Next() {
				var ref string
				if err = rows.Scan(&ref); err != nil {
					break
				}
				if ref != id {
					refs = append(refs, ref)
				}
			}
			if err == nil {
				err = rows.Err()
			}
			rows.Close()
			if err != nil {
				return err
			}
		}
	}
	if len(refs) > 0 {
		return &ValidationError{Fields: []FieldError{{Field: "id", Reason: "对象仍被引用", References: refs}}}
	}
	return nil
}

func (s Service) MarkVerified(ctx context.Context, p auth.Principal, id string, rev int64, reason, via string) (Object, error) {
	if e := auth.Authorize(p, auth.MarkVerified, ""); e != nil {
		return Object{}, e
	}
	if rev < 1 || !reasonOK(reason) || via == "" {
		return Object{}, ErrInvalid
	}
	tx, e := s.Store.DB.BeginTx(ctx, nil)
	if e != nil {
		return Object{}, e
	}
	defer tx.Rollback()
	o, e := get(ctx, tx, id)
	if e != nil {
		return o, e
	}
	if o.Rev != rev {
		return o, ErrConflict
	}
	now := time.Now().UTC()
	r, e := tx.ExecContext(ctx, "UPDATE objects SET verified_at=?,verified_by=?,rev=rev+1,updated_at=?,updated_by=? WHERE id=? AND rev=?", now.Format(time.RFC3339Nano), actorName(p), now.Format(time.RFC3339Nano), actorName(p), id, rev)
	if e != nil {
		return o, e
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return o, ErrConflict
	}
	_, e = audit.AppendTx(ctx, tx, audit.Entry{ObjectID: id, ObjectType: o.Type, Action: "mark_verified", Diff: map[string]any{"verified_at": now.Format(time.RFC3339Nano)}, Actor: p, Via: via, Reason: reason, State: "已生效", BaseRev: rev})
	if e != nil {
		return o, e
	}
	if e = tx.Commit(); e != nil {
		return o, e
	}
	o.Rev++
	o.VerifiedAt = &now
	o.VerifiedBy = actorName(p)
	return o, nil
}

func (s Service) RecordNote(ctx context.Context, p auth.Principal, target string, rev int64, kind, body, reason, via string) (string, error) {
	if e := auth.Authorize(p, auth.RecordNote, ""); e != nil {
		return "", e
	}
	if rev < 1 || !reasonOK(reason) || via == "" || kind == "" || body == "" {
		return "", ErrInvalid
	}
	if e := CheckSecrets(body); e != nil {
		return "", e
	}
	tx, e := s.Store.DB.BeginTx(ctx, nil)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	var typ string
	var current int64
	e = tx.QueryRowContext(ctx, "SELECT type,rev FROM objects WHERE id=?", target).Scan(&typ, &current)
	if e != nil {
		return "", e
	}
	if current != rev {
		return "", ErrConflict
	}
	id, e := newID("note:")
	if e != nil {
		return "", e
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	actor := actorName(p)
	r, e := tx.ExecContext(ctx, "UPDATE objects SET rev=rev+1,updated_at=?,updated_by=? WHERE id=? AND rev=?", now, actor, target, rev)
	if e != nil {
		return "", e
	}
	affected, _ := r.RowsAffected()
	if affected != 1 {
		return "", ErrConflict
	}
	_, e = tx.ExecContext(ctx, "INSERT INTO objects(id,type,name,created_at,created_by,updated_at,updated_by) VALUES(?,'note',?,?,?,?,?)", id, kind, now, actor, now, actor)
	if e != nil {
		return "", e
	}
	_, e = tx.ExecContext(ctx, "INSERT INTO notes(id,target_id,kind,body_md,author) VALUES(?,?,?,?,?)", id, target, kind, body, actor)
	if e != nil {
		return "", e
	}
	_, e = audit.AppendTx(ctx, tx, audit.Entry{ObjectID: target, ObjectType: typ, Action: "record_note", Diff: map[string]any{"note_id": id}, Actor: p, Via: via, Reason: reason, State: "已生效", BaseRev: rev})
	if e != nil {
		return "", e
	}
	return id, tx.Commit()
}

// PutContext only permits keys to create or renew their own records.
func (s Service) PutContext(ctx context.Context, p auth.Principal, id string, rev int64, body string, expires time.Time, reason, via string) error {
	if rev < 1 || !reasonOK(reason) || via == "" {
		return ErrInvalid
	}
	if e := CheckSecrets(body); e != nil {
		return e
	}
	tx, e := s.Store.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var owner string
	var current int64
	e = tx.QueryRowContext(ctx, "SELECT c.owner,o.rev FROM contexts c JOIN objects o ON o.id=c.id WHERE c.id=?", id).Scan(&owner, &current)
	if e != nil {
		return e
	}
	if e = auth.Authorize(p, auth.PutContext, owner); e != nil {
		return e
	}
	if current != rev {
		return ErrConflict
	}
	if expires.Before(time.Now()) {
		return ErrInvalid
	}
	r, e := tx.ExecContext(ctx, "UPDATE objects SET rev=rev+1,updated_at=?,updated_by=? WHERE id=? AND rev=?", time.Now().UTC().Format(time.RFC3339Nano), actorName(p), id, rev)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return ErrConflict
	}
	_, e = tx.ExecContext(ctx, "UPDATE contexts SET body_md=?,expires_at=?,archived_at=NULL WHERE id=?", body, expires.UTC().Format(time.RFC3339Nano), id)
	if e != nil {
		return e
	}
	_, e = audit.AppendTx(ctx, tx, audit.Entry{ObjectID: id, ObjectType: "context", Action: "put_context", Diff: map[string]any{"expires_at": expires.UTC().Format(time.RFC3339Nano)}, Actor: p, Via: via, Reason: reason, State: "已生效", BaseRev: rev})
	if e != nil {
		return e
	}
	return tx.Commit()
}

func (s Service) CreateContext(ctx context.Context, p auth.Principal, o Object, reason, via string) (Object, error) {
	if e := auth.Authorize(p, auth.PutContext, ""); e != nil {
		return o, e
	}
	if o.Type != "context" || !reasonOK(reason) || via == "" {
		return o, ErrInvalid
	}
	if e := CheckSecrets(o.Fields); e != nil {
		return o, e
	}
	if o.Fields == nil {
		o.Fields = map[string]any{}
	}
	o.Fields["owner"] = actorID(p)
	return s.Store.create(ctx, o, actorName(p), func(tx *sql.Tx, created Object) error {
		_, err := audit.AppendTx(ctx, tx, audit.Entry{ObjectID: created.ID, ObjectType: "context", Action: "put_context", Diff: map[string]any{"scope": created.Fields["scope"]}, Actor: p, Via: via, Reason: reason, State: "已生效", BaseRev: 0})
		return err
	})
}
func (s Service) CheckVersionsAllowed(p auth.Principal) error {
	return auth.Authorize(p, auth.CheckVersions, "")
}
func (s Service) DeleteAllowed(p auth.Principal) error { return auth.Authorize(p, auth.Delete, "") }
func (s Service) AssertNoSecret(v any) error {
	if e := CheckSecrets(v); e != nil {
		return e
	}
	return nil
}
func (s Service) Get(ctx context.Context, p auth.Principal, id string) (Object, error) {
	if e := auth.Authorize(p, auth.View, ""); e != nil {
		return Object{}, e
	}
	return s.Store.Get(ctx, id)
}
