package catalog

import (
	"bytes"
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
		return o, invalidField("fields.body_md", "笔记只允许追加，不能编辑", "新增笔记")
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
			return o, invalidField("fields."+k, "对象类型不支持该字段", o.Type)
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
		return o, invalidField("name", "名称必填", "非空字符串")
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
					return o, invalidField("fields."+k, "JSON 格式无效", "有效 JSON")
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
			return invalidField("fields.location_id", fmt.Sprintf("IP %s (%s) 不属于新站点 %s 的唯一最长前缀网段", ip.address, ip.id, site), "新站点下唯一匹配的网段")
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
	if e := validateURLFields(patch.Fields); e != nil {
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
	if _, e = tx.ExecContext(ctx, "SAVEPOINT proposal_check"); e != nil {
		return "", e
	}
	_, e = s.applyUpdate(ctx, tx, id, rev, patch, actorName(p))
	if _, rollbackErr := tx.ExecContext(ctx, "ROLLBACK TO proposal_check"); rollbackErr != nil {
		return "", rollbackErr
	}
	if _, releaseErr := tx.ExecContext(ctx, "RELEASE proposal_check"); releaseErr != nil {
		return "", releaseErr
	}
	if e != nil {
		return "", e
	}
	change, e := audit.AppendTx(ctx, tx, audit.Entry{ObjectID: id, ObjectType: o.Type, Action: "update", Diff: patch, Actor: p, Via: via, Reason: reason, State: "待确认", BaseRev: rev})
	if e != nil {
		return "", e
	}
	if e = saveProposalBase(ctx, tx, change, o); e != nil {
		return "", e
	}
	return change, tx.Commit()
}

func saveProposalBase(ctx context.Context, tx *sql.Tx, change string, o Object) error {
	base, err := json.Marshal(o)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "UPDATE changes SET base_object_json=? WHERE id=?", string(base), change)
	return err
}

func sameValue(a, b any) bool {
	if a == nil || a == "" {
		a = nil
	}
	if b == nil || b == "" {
		b = nil
	}
	left, leftErr := json.Marshal(a)
	right, rightErr := json.Marshal(b)
	return leftErr == nil && rightErr == nil && bytes.Equal(left, right)
}

func changedFields(base, current Object, patch Patch, deleteAll bool) []string {
	fields := []string{}
	compare := func(name string, a, b any) {
		if !sameValue(a, b) {
			fields = append(fields, name)
		}
	}
	if deleteAll || patch.Name != nil {
		compare("name", base.Name, current.Name)
	}
	if deleteAll || patch.Aliases != nil {
		compare("aliases", base.Aliases, current.Aliases)
	}
	if deleteAll || patch.Tags != nil {
		compare("tags", base.Tags, current.Tags)
	}
	if deleteAll || patch.Status != nil {
		compare("status", base.Status, current.Status)
	}
	if deleteAll || patch.Source != nil {
		compare("source", base.Source, current.Source)
	}
	if deleteAll || patch.Confidence != nil {
		compare("confidence", base.Confidence, current.Confidence)
	}
	if deleteAll {
		for field, value := range base.Fields {
			compare("fields."+field, value, current.Fields[field])
		}
		for field, value := range current.Fields {
			if _, ok := base.Fields[field]; !ok {
				compare("fields."+field, nil, value)
			}
		}
	} else {
		for field := range patch.Fields {
			compare("fields."+field, base.Fields[field], current.Fields[field])
		}
	}
	return fields
}

// 迁移前的待审批记录没有原值快照，只能从其 base_rev 后的审计记录保守推断。
func legacyConflicts(ctx context.Context, tx *sql.Tx, id string, baseRev int64, patch Patch, deleteAll bool) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT action,diff_json FROM changes WHERE object_id=? AND state='已生效' AND base_rev>=? ORDER BY id`, id, baseRev)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	conflicts := []string{}
	for rows.Next() {
		var action, diff string
		if err = rows.Scan(&action, &diff); err != nil {
			return nil, err
		}
		if action == "record_note" || action == "mark_verified" {
			continue
		}
		if deleteAll {
			conflicts = append(conflicts, "object")
			continue
		}
		if action != "update" {
			conflicts = append(conflicts, "object")
			continue
		}
		var applied Patch
		if json.Unmarshal([]byte(diff), &applied) != nil {
			conflicts = append(conflicts, "object")
			continue
		}
		if patch.Name != nil && applied.Name != nil {
			conflicts = append(conflicts, "name")
		}
		if patch.Aliases != nil && applied.Aliases != nil {
			conflicts = append(conflicts, "aliases")
		}
		if patch.Tags != nil && applied.Tags != nil {
			conflicts = append(conflicts, "tags")
		}
		if patch.Status != nil && applied.Status != nil {
			conflicts = append(conflicts, "status")
		}
		if patch.Source != nil && applied.Source != nil {
			conflicts = append(conflicts, "source")
		}
		if patch.Confidence != nil && applied.Confidence != nil {
			conflicts = append(conflicts, "confidence")
		}
		for field := range patch.Fields {
			if _, ok := applied.Fields[field]; ok {
				conflicts = append(conflicts, "fields."+field)
			}
		}
	}
	return conflicts, rows.Err()
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
	// 审批会先移除自动依赖；预检也须按同一顺序试跑。
	if _, e = tx.ExecContext(ctx, "SAVEPOINT proposal_check"); e != nil {
		return "", e
	}
	rows, queryErr := tx.QueryContext(ctx, "SELECT id FROM dependencies WHERE generated=1 AND (from_id=? OR to_id=?)", id, id)
	if queryErr != nil {
		return "", queryErr
	}
	deps := []string{}
	for rows.Next() {
		var dep string
		if e = rows.Scan(&dep); e != nil {
			break
		}
		deps = append(deps, dep)
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e == nil {
		for _, dep := range deps {
			if _, e = tx.ExecContext(ctx, "DELETE FROM dependencies WHERE id=?", dep); e != nil {
				break
			}
			if _, e = tx.ExecContext(ctx, "DELETE FROM objects WHERE id=?", dep); e != nil {
				break
			}
		}
	}
	if e == nil {
		e = referencedBy(ctx, tx, id)
	}
	if _, rollbackErr := tx.ExecContext(ctx, "ROLLBACK TO proposal_check"); rollbackErr != nil {
		return "", rollbackErr
	}
	if _, releaseErr := tx.ExecContext(ctx, "RELEASE proposal_check"); releaseErr != nil {
		return "", releaseErr
	}
	if e != nil {
		return "", e
	}
	change, e := audit.AppendTx(ctx, tx, audit.Entry{ObjectID: id, ObjectType: o.Type, Action: "delete", Diff: map[string]any{}, Actor: p, Via: via, Reason: reason, State: "待确认", BaseRev: rev})
	if e != nil {
		return "", e
	}
	if e = saveProposalBase(ctx, tx, change, o); e != nil {
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
	var baseJSON sql.NullString
	var rev int64
	e = tx.QueryRowContext(ctx, "SELECT object_id,diff_json,state,action,base_rev,base_object_json FROM changes WHERE id=?", changeID).Scan(&id, &diff, &state, &action, &rev, &baseJSON)
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
			if baseJSON.Valid {
				var base Object
				if json.Unmarshal([]byte(baseJSON.String), &base) != nil {
					return Object{}, ErrInvalid
				}
				var patch Patch
				if action == "update" && json.Unmarshal([]byte(diff), &patch) != nil {
					return Object{}, ErrInvalid
				}
				if fields := changedFields(base, o, patch, action == "delete"); len(fields) > 0 {
					return Object{}, &ConflictError{Fields: []FieldError{{Field: strings.Join(fields, ", "), Reason: "提交后字段已被修改，请重新提议"}}}
				}
			} else if o.Rev != rev {
				var patch Patch
				if action == "update" && json.Unmarshal([]byte(diff), &patch) != nil {
					return Object{}, ErrInvalid
				}
				fields, checkErr := legacyConflicts(ctx, tx, id, rev, patch, action == "delete")
				if checkErr != nil {
					return Object{}, checkErr
				}
				if len(fields) > 0 {
					return Object{}, &ConflictError{Fields: []FieldError{{Field: strings.Join(fields, ", "), Reason: "提交后字段已被修改，请重新提议"}}}
				}
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
			o, e = s.applyUpdate(ctx, tx, id, o.Rev, patch, p.Login)
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
			if _, e = tx.ExecContext(ctx, "DELETE FROM objects WHERE id=? AND rev=?", id, o.Rev); e != nil {
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

type VerifyResult struct {
	ID    string `json:"id"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// VerifyBatch 保留逐对象审计，且逐条返回结果，避免把部分成功误报为全部成功。
func (s Service) VerifyBatch(ctx context.Context, p auth.Principal, ids []string, withRelated bool, reason, via string) ([]VerifyResult, error) {
	if err := auth.Authorize(p, auth.MarkVerified, ""); err != nil {
		return nil, err
	}
	if len(ids) == 0 || len(ids) > 200 || !reasonOK(reason) || via == "" {
		return nil, ErrInvalid
	}
	all := append([]string{}, ids...)
	if withRelated {
		for _, id := range ids {
			o, err := s.Store.Get(ctx, id)
			if err != nil {
				continue
			}
			if o.Type != "host" {
				continue
			}
			for _, query := range []string{
				"SELECT id FROM ips WHERE host_id=?",
				"SELECT p.id FROM ports p JOIN services s ON s.id=p.service_id WHERE s.host_id=?",
			} {
				rows, err := s.Store.DB.QueryContext(ctx, query, id)
				if err != nil {
					return nil, err
				}
				for rows.Next() {
					var child string
					if err = rows.Scan(&child); err != nil {
						break
					}
					all = append(all, child)
				}
				if err == nil {
					err = rows.Err()
				}
				rows.Close()
				if err != nil {
					return nil, err
				}
			}
		}
	}
	results := make([]VerifyResult, 0, len(all))
	seen := map[string]bool{}
	for _, id := range all {
		if seen[id] {
			continue
		}
		seen[id] = true
		item := VerifyResult{ID: id}
		o, err := s.Store.Get(ctx, id)
		if err == nil {
			_, err = s.MarkVerified(ctx, p, id, o.Rev, reason, via)
		}
		if err != nil {
			item.Error = err.Error()
		} else {
			item.OK = true
		}
		results = append(results, item)
	}
	return results, nil
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
