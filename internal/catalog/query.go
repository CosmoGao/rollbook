package catalog

import (
	"context"
	"time"

	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/search"
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
	return d, nil
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
	err := q.collect(ctx, p, `SELECT id FROM versions WHERE latest_version IS NOT NULL AND current_version IS NOT NULL AND latest_version<>current_version ORDER BY id`, nil, &out)
	return out, err
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
	switch typ {
	case "location", "host", "network":
	default:
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
	return out, err
}
func (q Query) Contexts(ctx context.Context, p auth.Principal, archived bool) ([]Object, error) {
	if err := auth.Authorize(p, auth.View, ""); err != nil {
		return nil, err
	}
	return q.Service.Store.Contexts(ctx, archived, time.Now().UTC())
}
func (q Query) CheckVersions(ctx context.Context, p auth.Principal) (map[string]any, error) {
	if err := q.Service.CheckVersionsAllowed(p); err != nil {
		return nil, err
	}
	// 版本上游任务在后续阶段接入。此处仅返回明确状态，不伪称检查成功。
	return map[string]any{"status": "not_configured"}, nil
}
