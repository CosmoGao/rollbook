// Package search provides FTS and relation-aware catalog browsing.
package search

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

type Result struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	Name       string   `json:"name"`
	Status     string   `json:"status"`
	SiteID     string   `json:"site_id,omitempty"`
	NetworkID  string   `json:"network_id,omitempty"`
	RouterID   string   `json:"router_id,omitempty"`
	HostID     string   `json:"host_id,omitempty"`
	RelatedIDs []string `json:"related_ids,omitempty"`
}
type Store struct{ DB *sql.DB }

// Search combines tokenized FTS with bounded IP and exact port matching.
func (s Store) Search(ctx context.Context, q string) ([]Result, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, nil
	}
	ids := map[string]bool{}
	tokens := strings.FieldsFunc(q, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	if len(tokens) > 0 {
		for i, t := range tokens {
			tokens[i] = `"` + strings.ReplaceAll(t, `"`, ``) + `"`
			// unicode61 将连续汉字作为一个词；前缀匹配允许「香河」找到「香河站点」。
			if strings.IndexFunc(t, func(r rune) bool { return unicode.Is(unicode.Han, r) }) >= 0 {
				tokens[i] += "*"
			}
		}
		rows, e := s.DB.QueryContext(ctx, `SELECT object_id FROM search_documents WHERE search_documents MATCH ? LIMIT 100`, strings.Join(tokens, " AND "))
		if e != nil {
			return nil, e
		}
		for rows.Next() {
			var id string
			if e = rows.Scan(&id); e != nil {
				rows.Close()
				return nil, e
			}
			ids[id] = true
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return nil, e
		}
	}
	for _, query := range []struct {
		sql string
		arg any
	}{{`SELECT id FROM ips WHERE address=? OR address LIKE ? LIMIT 100`, []any{q, "%." + q}}, {`SELECT id FROM ports WHERE port=? LIMIT 100`, q}, {`SELECT id FROM networks WHERE cidr LIKE ? LIMIT 100`, q + "%"}} {
		var rows *sql.Rows
		var e error
		if a, ok := query.arg.([]any); ok {
			rows, e = s.DB.QueryContext(ctx, query.sql, a...)
		} else {
			rows, e = s.DB.QueryContext(ctx, query.sql, query.arg)
		}
		if e != nil {
			return nil, e
		}
		for rows.Next() {
			var id string
			if e = rows.Scan(&id); e != nil {
				rows.Close()
				return nil, e
			}
			ids[id] = true
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return nil, e
		}
	}
	out := []Result{}
	for id := range ids {
		r, e := s.enrich(ctx, id)
		if e != nil {
			return nil, e
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s Store) enrich(ctx context.Context, id string) (Result, error) {
	r := Result{ID: id}
	e := s.DB.QueryRowContext(ctx, `SELECT type,name,status FROM objects WHERE id=?`, id).Scan(&r.Type, &r.Name, &r.Status)
	if e != nil {
		return r, e
	}
	var host, network, site sql.NullString
	switch r.Type {
	case "ip":
		e = s.DB.QueryRowContext(ctx, `SELECT host_id,network_id FROM ips WHERE id=?`, id).Scan(&host, &network)
	case "port":
		e = s.DB.QueryRowContext(ctx, `SELECT s.host_id FROM ports p JOIN services s ON s.id=p.service_id WHERE p.id=?`, id).Scan(&host)
	case "service":
		e = s.DB.QueryRowContext(ctx, `SELECT host_id FROM services WHERE id=?`, id).Scan(&host)
	case "endpoint":
		e = s.DB.QueryRowContext(ctx, `SELECT s.host_id FROM endpoints e JOIN services s ON s.id=e.service_id WHERE e.id=?`, id).Scan(&host)
	case "host":
		host.String = id
		host.Valid = true
	case "network":
		network.String = id
		network.Valid = true
	case "uplink":
		e = s.DB.QueryRowContext(ctx, `SELECT location_id FROM uplinks WHERE id=?`, id).Scan(&site)
	case "location":
		site.String = id
		site.Valid = true
	}
	if e != nil {
		return r, e
	}
	if host.Valid {
		r.HostID = host.String
		e = s.DB.QueryRowContext(ctx, `SELECT location_id FROM hosts WHERE id=?`, host.String).Scan(&site)
		if e != nil {
			return r, e
		}
	}
	if network.Valid {
		r.NetworkID = network.String
		var router sql.NullString
		e = s.DB.QueryRowContext(ctx, `SELECT location_id,router_id FROM networks WHERE id=?`, network.String).Scan(&site, &router)
		if e != nil {
			return r, e
		}
		r.RouterID = router.String
	}
	if site.Valid {
		var root string
		e = s.DB.QueryRowContext(ctx, `WITH RECURSIVE parents(id,parent_id,kind) AS (
 SELECT id,parent_id,kind FROM locations WHERE id=?
 UNION ALL SELECT l.id,l.parent_id,l.kind FROM locations l JOIN parents p ON p.parent_id=l.id
 ) SELECT id FROM parents WHERE kind='站点' LIMIT 1`, site.String).Scan(&root)
		if e != nil {
			return r, e
		}
		site.String = root
		r.SiteID = site.String
	}
	related := map[string]bool{}
	if r.HostID != "" {
		related[r.HostID] = true
		rows, e := s.DB.QueryContext(ctx, `SELECT id FROM services WHERE host_id=? UNION SELECT e.id FROM endpoints e JOIN services s ON s.id=e.service_id WHERE s.host_id=?`, r.HostID, r.HostID)
		if e != nil {
			return r, e
		}
		for rows.Next() {
			var x string
			if e = rows.Scan(&x); e != nil {
				rows.Close()
				return r, e
			}
			related[x] = true
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return r, e
		}
	}
	if r.SiteID != "" {
		rows, e := s.DB.QueryContext(ctx, `WITH RECURSIVE places(id) AS (SELECT ? UNION ALL SELECT l.id FROM locations l JOIN places p ON l.parent_id=p.id)
 SELECT n.id FROM networks n JOIN places p ON n.location_id=p.id
 UNION SELECT u.id FROM uplinks u JOIN places p ON u.location_id=p.id
 UNION SELECT h.id FROM hosts h JOIN places p ON h.location_id=p.id`, r.SiteID)
		if e != nil {
			return r, e
		}
		for rows.Next() {
			var x string
			if e = rows.Scan(&x); e != nil {
				rows.Close()
				return r, e
			}
			related[x] = true
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return r, e
		}
	}
	delete(related, id)
	for x := range related {
		r.RelatedIDs = append(r.RelatedIDs, x)
	}
	sort.Strings(r.RelatedIDs)
	return r, nil
}

type Site struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Locations []string `json:"locations"`
	Uplinks   []string `json:"uplinks"`
	Networks  []string `json:"networks"`
	Hosts     []string `json:"hosts"`
	Routers   []string `json:"routers"`
}

func (s Store) GetSite(ctx context.Context, idOrName string) (Site, error) {
	var site Site
	e := s.DB.QueryRowContext(ctx, `SELECT l.id,o.name FROM locations l JOIN objects o ON o.id=l.id WHERE l.kind='站点' AND (l.id=? OR o.name=? OR l.city=?)`, idOrName, idOrName, idOrName).Scan(&site.ID, &site.Name)
	if e != nil {
		return site, e
	}
	rows, e := s.DB.QueryContext(ctx, `WITH RECURSIVE tree(id) AS (SELECT ? UNION SELECT l.id FROM locations l JOIN tree t ON l.parent_id=t.id) SELECT id FROM tree`, site.ID)
	if e != nil {
		return site, e
	}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return site, e
		}
		site.Locations = append(site.Locations, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return site, e
	}
	for _, loc := range site.Locations {
		for _, x := range []struct {
			query string
			dest  *[]string
		}{{`SELECT id FROM uplinks WHERE location_id=?`, &site.Uplinks}, {`SELECT id FROM networks WHERE location_id=?`, &site.Networks}, {`SELECT id FROM hosts WHERE location_id=?`, &site.Hosts}, {`SELECT router_id FROM networks WHERE location_id=? AND router_id IS NOT NULL`, &site.Routers}} {
			rows, e = s.DB.QueryContext(ctx, x.query, loc)
			if e != nil {
				return site, e
			}
			for rows.Next() {
				var id string
				if e = rows.Scan(&id); e != nil {
					rows.Close()
					return site, e
				}
				*x.dest = append(*x.dest, id)
			}
			e = rows.Err()
			rows.Close()
			if e != nil {
				return site, e
			}
		}
	}
	sort.Strings(site.Locations)
	sort.Strings(site.Uplinks)
	sort.Strings(site.Networks)
	sort.Strings(site.Hosts)
	sort.Strings(site.Routers)
	return site, nil
}

type Inventory struct {
	ID       string   `json:"id"`
	Hosts    []string `json:"hosts,omitempty"`
	Services []string `json:"services,omitempty"`
	IPs      []string `json:"ips,omitempty"`
	RouterID string   `json:"router_id,omitempty"`
}

func (s Store) ByHost(ctx context.Context, id string) (Inventory, error) {
	inv := Inventory{ID: id}
	if e := s.collect(ctx, `SELECT id FROM services WHERE host_id=?`, id, &inv.Services); e != nil {
		return inv, e
	}
	if e := s.collect(ctx, `SELECT id FROM ips WHERE host_id=?`, id, &inv.IPs); e != nil {
		return inv, e
	}
	return inv, nil
}
func (s Store) ByNetwork(ctx context.Context, id string) (Inventory, error) {
	inv := Inventory{ID: id}
	if e := s.DB.QueryRowContext(ctx, `SELECT coalesce(router_id,'') FROM networks WHERE id=?`, id).Scan(&inv.RouterID); e != nil {
		return inv, e
	}
	if e := s.collect(ctx, `SELECT id FROM ips WHERE network_id=?`, id, &inv.IPs); e != nil {
		return inv, e
	}
	if e := s.collect(ctx, `SELECT DISTINCT host_id FROM ips WHERE network_id=?`, id, &inv.Hosts); e != nil {
		return inv, e
	}
	return inv, nil
}
func (s Store) collect(ctx context.Context, q, id string, dst *[]string) error {
	rows, e := s.DB.QueryContext(ctx, q, id)
	if e != nil {
		return e
	}
	defer rows.Close()
	for rows.Next() {
		var x string
		if e = rows.Scan(&x); e != nil {
			return e
		}
		*dst = append(*dst, x)
	}
	return rows.Err()
}

type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}
type Topology struct {
	Center string   `json:"center"`
	Nodes  []string `json:"nodes"`
	Edges  []Edge   `json:"edges"`
}

func (s Store) Topology(ctx context.Context, center string, depth int) (Topology, error) {
	if depth < 0 || depth > 2 {
		return Topology{}, fmt.Errorf("depth must be 0..2")
	}
	var typ string
	if e := s.DB.QueryRowContext(ctx, `SELECT type FROM objects WHERE id=?`, center).Scan(&typ); e != nil {
		return Topology{}, e
	}
	if typ != "service" {
		return Topology{}, fmt.Errorf("center must be service")
	}
	t := Topology{Center: center}
	seen := map[string]bool{center: true}
	front := []string{center}
	for level := 0; level < depth; level++ {
		next := []string{}
		for _, id := range front {
			rows, e := s.DB.QueryContext(ctx, `SELECT from_id,to_id,kind FROM dependencies WHERE from_id=? OR to_id=?`, id, id)
			if e != nil {
				return t, e
			}
			for rows.Next() {
				var edge Edge
				if e = rows.Scan(&edge.From, &edge.To, &edge.Kind); e != nil {
					rows.Close()
					return t, e
				}
				t.Edges = append(t.Edges, edge)
				for _, x := range []string{edge.From, edge.To} {
					if !seen[x] {
						seen[x] = true
						next = append(next, x)
					}
				}
			}
			e = rows.Err()
			rows.Close()
			if e != nil {
				return t, e
			}
		}
		front = next
	}
	for x := range seen {
		t.Nodes = append(t.Nodes, x)
	}
	sort.Strings(t.Nodes)
	sort.Slice(t.Edges, func(i, j int) bool { return t.Edges[i].From+t.Edges[i].To < t.Edges[j].From+t.Edges[j].To })
	return t, nil
}
