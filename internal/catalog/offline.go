package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/CosmoGao/rollbook/internal/audit"
	"github.com/CosmoGao/rollbook/internal/auth"
)

type OfflineOptions struct {
	Services  bool `json:"services"`
	IPs       bool `json:"ips"`
	Endpoints bool `json:"endpoints"`
}

type OfflinePreview struct {
	Host      Object   `json:"host"`
	Services  []Object `json:"services"`
	IPs       []Object `json:"ips"`
	Endpoints []Object `json:"endpoints"`
}

type offlineItem struct {
	ID            string `json:"id"`
	Type          string `json:"type"`
	Rev           int64  `json:"rev"`
	BeforeStatus  string `json:"before_status"`
	BeforeAddress string `json:"before_address,omitempty"`
	BeforeName    string `json:"before_name,omitempty"`
	AfterRev      int64  `json:"after_rev"`
}

func (s Service) OfflinePreview(ctx context.Context, p auth.Principal, hostID string) (OfflinePreview, error) {
	if err := auth.Authorize(p, auth.View, ""); err != nil {
		return OfflinePreview{}, err
	}
	return offlinePreview(ctx, s.Store.DB, hostID)
}

func offlinePreview(ctx context.Context, q rowQuerier, hostID string) (OfflinePreview, error) {
	host, err := get(ctx, q, hostID)
	if err != nil {
		return OfflinePreview{}, err
	}
	if host.Type != "host" {
		return OfflinePreview{}, invalidField("id", "对象不是主机", "主机 ID")
	}
	result := OfflinePreview{Host: host, Services: []Object{}, IPs: []Object{}, Endpoints: []Object{}}
	for _, spec := range []struct {
		query string
		dst   *[]Object
	}{
		{`SELECT id FROM services WHERE host_id=?`, &result.Services},
		{`SELECT id FROM ips WHERE host_id=?`, &result.IPs},
		{`SELECT e.id FROM endpoints e JOIN services s ON s.id=e.service_id WHERE s.host_id=?`, &result.Endpoints},
	} {
		rows, e := sQuery(ctx, q, spec.query, hostID)
		if e != nil {
			return result, e
		}
		ids := []string{}
		for rows.Next() {
			var id string
			if e = rows.Scan(&id); e != nil {
				break
			}
			ids = append(ids, id)
		}
		if e == nil {
			e = rows.Err()
		}
		rows.Close()
		if e != nil {
			return result, e
		}
		for _, id := range ids {
			o, e := get(ctx, q, id)
			if e != nil {
				return result, e
			}
			*spec.dst = append(*spec.dst, o)
		}
	}
	return result, nil
}

type offlineQuerier interface {
	rowQuerier
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func sQuery(ctx context.Context, q rowQuerier, query string, args ...any) (*sql.Rows, error) {
	return q.(offlineQuerier).QueryContext(ctx, query, args...)
}

// OfflineHost changes selected objects atomically and keeps their prior state for a guarded restore.
func (s Service) OfflineHost(ctx context.Context, p auth.Principal, id string, rev int64, opts OfflineOptions, reason string) (string, error) {
	if !p.IsUser() {
		return "", auth.ErrForbidden
	}
	if rev < 1 || !reasonOK(reason) {
		return "", invalidField("reason", "操作原因和主机版本必填", "非空原因及当前 rev")
	}
	tx, e := s.Store.DB.BeginTx(ctx, nil)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	preview, e := offlinePreview(ctx, tx, id)
	if e != nil {
		return "", e
	}
	if preview.Host.Rev != rev {
		return "", ErrConflict
	}
	if preview.Host.Status == "停用" {
		return "", invalidField("status", "主机已停用", "在用主机")
	}
	opID, e := newID("offline:")
	if e != nil {
		return "", e
	}
	items := []offlineItem{}
	selected := []Object{preview.Host}
	if opts.Services {
		selected = append(selected, preview.Services...)
	}
	if opts.IPs {
		selected = append(selected, preview.IPs...)
	}
	if opts.Endpoints {
		selected = append(selected, preview.Endpoints...)
	}
	for _, o := range selected {
		if o.Status == "停用" {
			continue
		}
		item := offlineItem{ID: o.ID, Type: o.Type, Rev: o.Rev, BeforeStatus: o.Status, AfterRev: o.Rev + 1}
		if o.Type == "ip" {
			item.BeforeAddress = str(o.Fields["address"])
			item.BeforeName = o.Name
		}
		if e = offlineSet(ctx, tx, o, item, true, p, reason, opID); e != nil {
			return "", e
		}
		items = append(items, item)
	}
	b, _ := json.Marshal(items)
	_, e = tx.ExecContext(ctx, `INSERT INTO host_offline_operations(id,host_id,snapshot_json,created_at) VALUES(?,?,?,?)`, opID, id, string(b), time.Now().UTC().Format(time.RFC3339Nano))
	if e != nil {
		return "", e
	}
	return opID, tx.Commit()
}

func offlineSet(ctx context.Context, tx *sql.Tx, o Object, item offlineItem, down bool, p auth.Principal, reason, opID string) error {
	status := item.BeforeStatus
	name := o.Name
	if down {
		status = "停用"
	}
	if o.Type == "ip" && item.BeforeAddress != "" {
		address := item.BeforeAddress
		if down {
			address = "released:" + o.ID
			name = "已释放 " + item.BeforeAddress
		} else {
			name = item.BeforeName
		}
		if _, e := tx.ExecContext(ctx, `UPDATE ips SET address=? WHERE id=?`, address, o.ID); e != nil {
			return storageError(e, o)
		}
	}
	result, e := tx.ExecContext(ctx, `UPDATE objects SET status=?,name=?,rev=rev+1,updated_at=?,updated_by=? WHERE id=? AND rev=?`, status, name, time.Now().UTC().Format(time.RFC3339Nano), actorName(p), o.ID, o.Rev)
	if e != nil {
		return e
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return ErrConflict
	}
	if _, e = tx.ExecContext(ctx, `DELETE FROM search_documents WHERE object_id=?`, o.ID); e != nil {
		return e
	}
	o.Status = status
	o.Name = name
	if o.Type == "ip" && item.BeforeAddress != "" {
		if down {
			o.Fields["address"] = "released:" + o.ID
		} else {
			o.Fields["address"] = item.BeforeAddress
		}
	}
	if e = index(ctx, tx, o); e != nil {
		return e
	}
	action := "host_offline"
	if !down {
		action = "host_restore"
	}
	_, e = audit.AppendTx(ctx, tx, audit.Entry{ObjectID: o.ID, ObjectType: o.Type, Action: action, Diff: map[string]any{"operation_id": opID, "before_status": item.BeforeStatus, "after_status": status, "address": item.BeforeAddress}, Actor: p, Via: "web", Reason: reason, State: "已生效", BaseRev: o.Rev})
	return e
}

type RestoreResult struct {
	Restored []string `json:"restored"`
	Skipped  []string `json:"skipped"`
}

func (s Service) RestoreHost(ctx context.Context, p auth.Principal, hostID, opID, reason string) (RestoreResult, error) {
	result := RestoreResult{Restored: []string{}, Skipped: []string{}}
	if !p.IsUser() {
		return result, auth.ErrForbidden
	}
	if !reasonOK(reason) {
		return result, invalidField("reason", "操作原因必填", "非空字符串")
	}
	tx, e := s.Store.DB.BeginTx(ctx, nil)
	if e != nil {
		return result, e
	}
	defer tx.Rollback()
	var raw string
	var restored sql.NullString
	e = tx.QueryRowContext(ctx, `SELECT snapshot_json,restored_at FROM host_offline_operations WHERE id=? AND host_id=?`, opID, hostID).Scan(&raw, &restored)
	if e == sql.ErrNoRows {
		return result, invalidField("operation_id", "未找到本次停用记录", "有效操作 ID")
	}
	if e != nil {
		return result, e
	}
	if restored.Valid {
		return result, ErrConflict
	}
	var items []offlineItem
	if e = json.Unmarshal([]byte(raw), &items); e != nil {
		return result, e
	}
	ready := []struct {
		o    Object
		item offlineItem
	}{}
	conflicts := []FieldError{}
	for _, item := range items {
		o, err := get(ctx, tx, item.ID)
		if err == sql.ErrNoRows {
			result.Skipped = append(result.Skipped, item.ID)
			continue
		}
		if err != nil {
			return result, err
		}
		if o.Rev != item.AfterRev || o.Status != "停用" || (item.BeforeAddress != "" && str(o.Fields["address"]) != "released:"+o.ID) {
			result.Skipped = append(result.Skipped, item.ID)
			continue
		}
		if item.BeforeAddress != "" {
			var owner string
			err = tx.QueryRowContext(ctx, `SELECT id FROM ips WHERE address=? AND id<>?`, item.BeforeAddress, item.ID).Scan(&owner)
			if err == nil {
				conflicts = append(conflicts, FieldError{Field: "address", Reason: fmt.Sprintf("地址 %s 已被占用", item.BeforeAddress), References: []string{owner}})
			} else if err != sql.ErrNoRows {
				return result, err
			}
		}
		ready = append(ready, struct {
			o    Object
			item offlineItem
		}{o, item})
	}
	if len(conflicts) > 0 {
		return result, &ValidationError{Fields: conflicts}
	}
	for _, entry := range ready {
		if e = offlineSet(ctx, tx, entry.o, entry.item, false, p, reason, opID); e != nil {
			return result, e
		}
		result.Restored = append(result.Restored, entry.o.ID)
	}
	_, e = tx.ExecContext(ctx, `UPDATE host_offline_operations SET restored_at=? WHERE id=?`, time.Now().UTC().Format(time.RFC3339Nano), opID)
	if e != nil {
		return result, e
	}
	return result, tx.Commit()
}

// DeleteAsset is reserved for a signed-in web user. Referenced objects are reported before deletion.
func (s Service) DeleteAsset(ctx context.Context, p auth.Principal, id string, rev int64, reason string) error {
	if !p.IsUser() {
		return auth.ErrForbidden
	}
	if rev < 1 || !reasonOK(reason) {
		return ErrInvalid
	}
	tx, e := s.Store.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	o, e := get(ctx, tx, id)
	if e != nil {
		return e
	}
	if o.Rev != rev {
		return ErrConflict
	}
	if o.Type == "change" || o.Type == "context" || o.Type == "dependency" && str(o.Fields["generated"]) == "1" {
		return auth.ErrForbidden
	}
	rows, e := tx.QueryContext(ctx, `SELECT id FROM dependencies WHERE generated=1 AND (from_id=? OR to_id=? OR id LIKE ?)`, id, id, "dependency:auto:"+id+":%")
	if e != nil {
		return e
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
	if e != nil {
		return e
	}
	for _, dep := range deps {
		if _, e = tx.ExecContext(ctx, `DELETE FROM dependencies WHERE id=?`, dep); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, `DELETE FROM objects WHERE id=?`, dep); e != nil {
			return e
		}
	}
	if e = referencedBy(ctx, tx, id); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, `DELETE FROM search_documents WHERE object_id=?`, id); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, `DELETE FROM `+tables[o.Type]+` WHERE id=?`, id); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, `DELETE FROM objects WHERE id=? AND rev=?`, id, rev); e != nil {
		return e
	}
	_, e = audit.AppendTx(ctx, tx, audit.Entry{ObjectID: id, ObjectType: o.Type, Action: "delete", Diff: map[string]any{"name": o.Name}, Actor: p, Via: "web", Reason: reason, State: "已生效", BaseRev: rev})
	if e != nil {
		return e
	}
	return tx.Commit()
}

func (s Service) LatestOfflineOperation(ctx context.Context, p auth.Principal, hostID string) (string, error) {
	if !p.IsUser() {
		return "", auth.ErrForbidden
	}
	var id string
	e := s.Store.DB.QueryRowContext(ctx, `SELECT id FROM host_offline_operations WHERE host_id=? AND restored_at IS NULL ORDER BY created_at DESC LIMIT 1`, hostID).Scan(&id)
	if e == sql.ErrNoRows {
		return "", nil
	}
	return id, e
}
