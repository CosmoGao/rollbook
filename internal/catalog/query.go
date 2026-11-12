package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/search"
	"github.com/CosmoGao/rollbook/internal/versioncheck"
)

// Query 将所有传输入口的读取请求收敛到同一个授权边界。
type Query struct{ Service Service }

func (q Query) Search(ctx context.Context, p auth.Principal, term string) ([]search.Result, error) {
	if err := auth.Authorize(p, auth.View, ""); err != nil {
		return nil, err
	}
	return (search.Store{DB: q.Service.Store.DB}).Search(ctx, term)
}
func (q Query) Site(ctx context.Context, p auth.Principal, id string) (search.Site, error) {
	if err := auth.Authorize(p, auth.View, ""); err != nil {
		return search.Site{}, err
	}
	return (search.Store{DB: q.Service.Store.DB}).GetSite(ctx, id)
}
func (q Query) Inventory(ctx context.Context, p auth.Principal, typ, id string) (search.Inventory, error) {
	if err := auth.Authorize(p, auth.View, ""); err != nil {
		return search.Inventory{}, err
	}
	s := search.Store{DB: q.Service.Store.DB}
	if typ == "host" {
		return s.ByHost(ctx, id)
	}
	return s.ByNetwork(ctx, id)
}
func (q Query) Topology(ctx context.Context, p auth.Principal, id string, depth int) (search.Topology, error) {
	if err := auth.Authorize(p, auth.View, ""); err != nil {
		return search.Topology{}, err
	}
	return (search.Store{DB: q.Service.Store.DB}).Topology(ctx, id, depth)
}

type ServiceDetail struct {
	Object       Object   `json:"service"`
	Host         Object   `json:"host"`
	IPs          []Object `json:"ips"`
	Ports        []Object `json:"ports"`
	Endpoints    []Object `json:"endpoints"`
	Versions     []Object `json:"versions"`
	Dependencies []Object `json:"dependencies"`
	Credentials  []Object `json:"credentials"`
	Notes        []Object `json:"notes"`
	Changes      []Object `json:"changes"`
}

func (q Query) ServiceDetail(ctx context.Context, p auth.Principal, idOrName string) (ServiceDetail, error) {
	var d ServiceDetail
	if err := auth.Authorize(p, auth.View, ""); err != nil {
		return d, err
	}
	db := q.Service.Store.DB
	var id string
	err := db.QueryRowContext(ctx, `SELECT s.id FROM services s JOIN objects o ON o.id=s.id WHERE s.id=? OR o.name=? OR EXISTS (SELECT 1 FROM json_each(o.aliases_json) WHERE value=?) ORDER BY CASE WHEN s.id=? THEN 0 WHEN o.name=? THEN 1 ELSE 2 END,s.id LIMIT 1`, idOrName, idOrName, idOrName, idOrName, idOrName).Scan(&id)
	if err != nil {
		return d, err
	}
	d.Object, err = q.Service.Get(ctx, p, id)
	if err != nil {
		return d, err
	}
	hostID, _ := d.Object.Fields["host_id"].(string)
	d.Host, err = q.Service.Get(ctx, p, hostID)
	if err != nil {
		return d, err
	}
	for _, x := range []struct {
		dst  *[]Object
		sql  string
		args []any
	}{
		{&d.IPs, `SELECT id FROM ips WHERE host_id=? ORDER BY id`, []any{hostID}},
		{&d.Ports, `SELECT id FROM ports WHERE service_id=? ORDER BY id`, []any{id}},
		{&d.Endpoints, `SELECT id FROM endpoints WHERE service_id=? ORDER BY id`, []any{id}},
		{&d.Versions, `SELECT id FROM versions WHERE service_id=? ORDER BY id`, []any{id}},
		{&d.Dependencies, `SELECT id FROM dependencies WHERE from_id=? OR to_id=? ORDER BY id`, []any{id, id}},
		{&d.Credentials, `SELECT id FROM credentials WHERE target_id=? ORDER BY id`, []any{id}},
		{&d.Notes, `SELECT id FROM notes WHERE target_id=? ORDER BY id`, []any{id}},
		{&d.Changes, `SELECT id FROM changes WHERE object_id=? ORDER BY id DESC LIMIT 50`, []any{id}},
	} {
		if err = q.collect(ctx, p, x.sql, x.args, x.dst); err != nil {
			return d, err
		}
	}
	for i := range d.Versions {
		if err = q.versionState(ctx, &d.Versions[i]); err != nil {
			return d, err
		}
	}
	return d, nil
}

func (q Query) versionState(ctx context.Context, o *Object) error {
	var checkError, attempted sql.NullString
	err := q.Service.Store.DB.QueryRowContext(ctx, `SELECT last_error,last_attempt_at FROM version_checks WHERE service_id=?`, o.Fields["service_id"]).Scan(&checkError, &attempted)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if checkError.Valid {
		o.Fields["check_error"] = checkError.String
	}
	if attempted.Valid {
		o.Fields["last_attempt_at"] = attempted.String
	}
	current, _ := o.Fields["current_version"].(string)
	latest, _ := o.Fields["latest_version"].(string)
	state := versioncheck.State(current, latest, o.Fields["pinned"], o.Fields["eol_date"], time.Now().UTC())
	o.Fields["version_state"] = state
	var raw sql.NullString
	if err := q.Service.Store.DB.QueryRowContext(ctx, `SELECT raw_tag FROM versions WHERE id=?`, o.ID).Scan(&raw); err != nil {
		return err
	}
	if raw.Valid {
		o.Fields["upstream_tag"] = raw.String
	} else if latest != "" {
		o.Fields["upstream_tag"] = latest
	}
	if source, ok := o.Fields["latest_source"].(string); ok && source != "" {
		o.Fields["upstream_source"] = source
	}
	return nil
}

func (q Query) collect(ctx context.Context, p auth.Principal, statement string, args []any, dst *[]Object) error {
	rows, err := q.Service.Store.DB.QueryContext(ctx, statement, args...)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			break
		}
		ids = append(ids, id)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return err
	}
	*dst = []Object{}
	for _, id := range ids {
		o, e := q.Service.Get(ctx, p, id)
		if e != nil {
			return e
		}
		*dst = append(*dst, o)
	}
	return nil
}

func (q Query) Outdated(ctx context.Context, p auth.Principal) ([]Object, error) {
	if err := auth.Authorize(p, auth.View, ""); err != nil {
		return nil, err
	}
	out := []Object{}
	all := []Object{}
	err := q.collect(ctx, p, `SELECT v.id FROM versions v JOIN services s ON s.id=v.service_id WHERE s.upstream_json IS NOT NULL AND s.upstream_json<>'' ORDER BY v.id`, nil, &all)
	if err != nil {
		return nil, err
	}
	for i := range all {
		if err = q.versionState(ctx, &all[i]); err != nil {
			return nil, err
		}
		if all[i].Fields["version_state"] != "已锁定" && (all[i].Fields["version_state"] != "已是最新" || all[i].Fields["check_error"] != nil) {
			out = append(out, all[i])
		}
	}
	return out, nil
}
func (q Query) Unverified(ctx context.Context, p auth.Principal) ([]Object, error) {
	if err := auth.Authorize(p, auth.View, ""); err != nil {
		return nil, err
	}
	out := []Object{}
	cutoff := time.Now().UTC().AddDate(0, 0, -30).Format(time.RFC3339Nano)
	err := q.collect(ctx, p, `SELECT id FROM objects WHERE type NOT IN ('change','note','dependency') AND (verified_at IS NULL OR verified_at < ?) ORDER BY updated_at DESC LIMIT 500`, []any{cutoff}, &out)
	return out, err
}
func (q Query) Browse(ctx context.Context, p auth.Principal, typ string) ([]Object, error) {
	if err := auth.Authorize(p, auth.View, ""); err != nil {
		return nil, err
	}
	if _, ok := tables[typ]; !ok || typ == "change" || typ == "context" {
		return nil, ErrInvalid
	}
	out := []Object{}
	err := q.collect(ctx, p, `SELECT id FROM objects WHERE type=? ORDER BY name LIMIT 500`, []any{typ}, &out)
	return out, err
}
func (q Query) Changes(ctx context.Context, p auth.Principal, objectID, state string) ([]Object, error) {
	if err := auth.Authorize(p, auth.View, ""); err != nil {
		return nil, err
	}
	out := []Object{}
	err := q.collect(ctx, p, `SELECT id FROM changes WHERE (?='' OR object_id=?) AND (?='' OR state=?) ORDER BY id DESC LIMIT 200`, []any{objectID, objectID, state, state}, &out)
	if err != nil {
		return nil, err
	}
	for i := range out {
		if out[i].Fields["state"] != "待确认" {
			continue
		}
		var baseJSON sql.NullString
		if err = q.Service.Store.DB.QueryRowContext(ctx, "SELECT base_object_json FROM changes WHERE id=?", out[i].ID).Scan(&baseJSON); err != nil {
			return nil, err
		}
		id, _ := out[i].Fields["object_id"].(string)
		current, getErr := q.Service.Store.Get(ctx, id)
		if out[i].Fields["action"] == "create" {
			if getErr == nil {
				out[i].Fields["conflicts"] = []string{"id"}
			} else if getErr != sql.ErrNoRows {
				return nil, getErr
			}
			continue
		}
		if getErr == nil {
			out[i].Fields["current_rev"] = current.Rev
		}
		if getErr != nil && getErr != sql.ErrNoRows {
			return nil, getErr
		}
		if !baseJSON.Valid || getErr != nil {
			if getErr != nil {
				out[i].Fields["conflicts"] = []string{"对象已不存在"}
			} else if baseRev, ok := out[i].Fields["base_rev"].(int64); ok && current.Rev != baseRev {
				var patch Patch
				if out[i].Fields["action"] == "update" {
					_ = json.Unmarshal([]byte(str(out[i].Fields["diff_json"])), &patch)
				}
				tx, txErr := q.Service.Store.DB.BeginTx(ctx, nil)
				if txErr != nil {
					return nil, txErr
				}
				fields, checkErr := legacyConflicts(ctx, tx, id, baseRev, patch, out[i].Fields["action"] == "delete")
				_ = tx.Rollback()
				if checkErr != nil {
					return nil, checkErr
				}
				out[i].Fields["conflicts"] = fields
			}
			continue
		}
		var base Object
		if err = json.Unmarshal([]byte(baseJSON.String), &base); err != nil {
			return nil, err
		}
		out[i].Fields["base_object"] = base
		var patch Patch
		if out[i].Fields["action"] == "update" {
			_ = json.Unmarshal([]byte(str(out[i].Fields["diff_json"])), &patch)
		}
		out[i].Fields["conflicts"] = changedFields(base, current, patch, out[i].Fields["action"] == "delete")
	}
	return out, err
}
func (q Query) Contexts(ctx context.Context, p auth.Principal, archived bool) ([]Object, error) {
	if err := auth.Authorize(p, auth.View, ""); err != nil {
		return nil, err
	}
	return q.Service.Store.Contexts(ctx, archived, time.Now().UTC())
}
func (q Query) CheckVersions(ctx context.Context, p auth.Principal) (versioncheck.Summary, error) {
	if err := q.Service.CheckVersionsAllowed(p); err != nil {
		return versioncheck.Summary{}, err
	}
	return (versioncheck.Checker{DB: q.Service.Store.DB}).Run(ctx, false)
}
