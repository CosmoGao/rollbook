// Package search provides FTS and relation-aware catalog browsing.
package search

import (
	"context"
	"database/sql"
	"encoding/json"
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
	aliases    []string
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
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if (a.Status == "停用") != (b.Status == "停用") {
			return b.Status == "停用"
		}
		if ar, br := matchRank(a, q), matchRank(b, q); ar != br {
			return ar < br
		}
		if an, bn := strings.ToLower(a.Name), strings.ToLower(b.Name); an != bn {
			return an < bn
		}
		return a.ID < b.ID
	})
	return out, nil
}

// 名称或别名全词命中优先，其次名称包含、别名包含，其他字段命中最后。
func matchRank(r Result, q string) int {
	q = strings.ToLower(strings.TrimSpace(q))
	if strings.ToLower(r.ID) == q || strings.ToLower(r.Name) == q {
		return 0
	}
	for _, alias := range r.aliases {
		if strings.ToLower(alias) == q {
			return 0
		}
	}
	if strings.Contains(strings.ToLower(r.Name), q) {
		return 1
	}
	for _, alias := range r.aliases {
		if strings.Contains(strings.ToLower(alias), q) {
			return 2
		}
	}
	return 3
}

func (s Store) enrich(ctx context.Context, id string) (Result, error) {
	r := Result{ID: id}
	var aliasesJSON string
	e := s.DB.QueryRowContext(ctx, `SELECT type,name,status,aliases_json FROM objects WHERE id=?`, id).Scan(&r.Type, &r.Name, &r.Status, &aliasesJSON)
	if e != nil {
		return r, e
	}
	if e = json.Unmarshal([]byte(aliasesJSON), &r.aliases); e != nil {
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
	ID        string            `json:"id"`
	Hosts     []string          `json:"hosts,omitempty"`
	Services  []string          `json:"services,omitempty"`
	IPs       []string          `json:"ips,omitempty"`
	RouterID  string            `json:"router_id,omitempty"`
	Occupancy *NetworkOccupancy `json:"occupancy,omitempty"`
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
	occupancy, e := s.NetworkOccupancy(ctx, id)
	if e != nil {
		return inv, e
	}
	inv.Occupancy = &occupancy
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
	Center  string         `json:"center"`
	Nodes   []string       `json:"nodes"`
	Edges   []Edge         `json:"edges"`
	Details []TopologyNode `json:"details"`
}

type TopologyNode struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Kind   string `json:"kind,omitempty"`
}

func (s Store) Topology(ctx context.Context, center string, depth int) (Topology, error) {
	return s.TopologyView(ctx, center, depth, "graph")
}

// TopologyView 保留旧拓扑的 nodes/edges 字段，并提供图形展示所需的节点详情。
func (s Store) TopologyView(ctx context.Context, center string, depth int, view string) (Topology, error) {
	if depth < 0 || depth > 2 {
		return Topology{}, fmt.Errorf("depth must be 0..2")
	}
	if view != "graph" && view != "access" {
		return Topology{}, fmt.Errorf("unknown topology view")
	}
	var typ string
	if e := s.DB.QueryRowContext(ctx, `SELECT type FROM objects WHERE id=?`, center).Scan(&typ); e != nil {
		return Topology{}, e
	}
	if typ != "service" && typ != "host" {
		return Topology{}, fmt.Errorf("center must be service or host")
	}
	g := topologyGraph{store: s, ctx: ctx, seen: map[string]bool{center: true}, edges: map[Edge]bool{}}
	if view == "access" {
		if typ == "service" {
			g.accessService(center)
		} else {
			g.host(center, true)
		}
	} else if typ == "host" {
		g.host(center, true)
	} else {
		g.service(center, depth)
	}
	if g.err != nil {
		return Topology{}, g.err
	}
	t := Topology{Center: center, Nodes: []string{}, Edges: []Edge{}, Details: []TopologyNode{}}
	for id := range g.seen {
		t.Nodes = append(t.Nodes, id)
	}
	sort.Strings(t.Nodes)
	for edge := range g.edges {
		t.Edges = append(t.Edges, edge)
	}
	sort.Slice(t.Edges, func(i, j int) bool {
		a, b := t.Edges[i], t.Edges[j]
		if a.From != b.From {
			return a.From < b.From
		}
		if a.To != b.To {
			return a.To < b.To
		}
		return a.Kind < b.Kind
	})
	for _, id := range t.Nodes {
		var n TopologyNode
		n.ID = id
		g.err = s.DB.QueryRowContext(ctx, `SELECT name,type,status FROM objects WHERE id=?`, id).Scan(&n.Name, &n.Type, &n.Status)
		if g.err != nil {
			return Topology{}, g.err
		}
		if n.Type == "host" {
			g.err = s.DB.QueryRowContext(ctx, `SELECT kind FROM hosts WHERE id=?`, id).Scan(&n.Kind)
			if g.err != nil {
				return Topology{}, g.err
			}
		}
		t.Details = append(t.Details, n)
	}
	return t, nil
}

type topologyGraph struct {
	store Store
	ctx   context.Context
	seen  map[string]bool
	edges map[Edge]bool
	err   error
}

func (g *topologyGraph) add(from, to, kind string) {
	if from == "" || to == "" || g.err != nil {
		return
	}
	g.seen[from] = true
	g.seen[to] = true
	g.edges[Edge{from, to, kind}] = true
}
func (g *topologyGraph) rows(query string, args ...any) *sql.Rows {
	if g.err != nil {
		return nil
	}
	var r *sql.Rows
	r, g.err = g.store.DB.QueryContext(g.ctx, query, args...)
	return r
}
func (g *topologyGraph) service(id string, depth int) {
	front := []string{id}
	visited := map[string]bool{id: true}
	for level := 0; level < depth; level++ {
		next := []string{}
		for _, current := range front {
			r := g.rows(`SELECT from_id,to_id,kind FROM dependencies WHERE from_id=? OR to_id=? ORDER BY from_id,to_id,kind`, current, current)
			if r == nil {
				return
			}
			for r.Next() {
				var e Edge
				if g.err = r.Scan(&e.From, &e.To, &e.Kind); g.err != nil {
					break
				}
				g.add(e.From, e.To, e.Kind)
				for _, x := range []string{e.From, e.To} {
					if !visited[x] {
						visited[x] = true
						next = append(next, x)
					}
				}
			}
			if g.err == nil {
				g.err = r.Err()
			}
			r.Close()
		}
		front = next
	}
	// 服务中心同时显示宿主、入口与反代，以便从节点进入详情。
	for id := range visited {
		var host string
		err := g.store.DB.QueryRowContext(g.ctx, `SELECT host_id FROM services WHERE id=?`, id).Scan(&host)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			g.err = err
			return
		}
		g.add(id, host, "宿主")
		g.endpoints(id)
	}
}
func (g *topologyGraph) endpoints(service string) {
	r := g.rows(`SELECT id FROM endpoints WHERE service_id=? ORDER BY id`, service)
	if r == nil {
		return
	}
	for r.Next() {
		var id string
		if g.err = r.Scan(&id); g.err != nil {
			break
		}
		g.add(id, service, "入口")
	}
	if g.err == nil {
		g.err = r.Err()
	}
	r.Close()
}
func (g *topologyGraph) host(center string, recursive bool) {
	front := []string{center}
	visited := map[string]bool{center: true}
	affected := []string{}
	serviceSeen := map[string]bool{}
	for len(front) > 0 {
		id := front[0]
		front = front[1:]
		r := g.rows(`SELECT id FROM hosts WHERE parent_host_id=? ORDER BY id`, id)
		if r == nil {
			return
		}
		for r.Next() {
			var child string
			if g.err = r.Scan(&child); g.err != nil {
				break
			}
			g.add(child, id, "父主机")
			if !visited[child] {
				visited[child] = true
				front = append(front, child)
			}
		}
		if g.err == nil {
			g.err = r.Err()
		}
		r.Close()
		if g.err != nil {
			return
		}
		if !recursive {
			break
		}
	}
	for host := range visited {
		r := g.rows(`SELECT id FROM services WHERE host_id=? ORDER BY id`, host)
		if r == nil {
			return
		}
		services := []string{}
		for r.Next() {
			var id string
			if g.err = r.Scan(&id); g.err != nil {
				break
			}
			services = append(services, id)
		}
		if g.err == nil {
			g.err = r.Err()
		}
		r.Close()
		if g.err != nil {
			return
		}
		for _, id := range services {
			g.add(id, host, "宿主")
			g.endpoints(id)
			serviceSeen[id] = true
			affected = append(affected, id)
		}
	}
	// 依赖受损服务的其他服务也会受到影响，沿反向依赖找出它们的入口。
	for len(affected) > 0 {
		id := affected[0]
		affected = affected[1:]
		r := g.rows(`SELECT d.from_id,d.kind FROM dependencies d JOIN services s ON s.id=d.from_id WHERE d.to_id=? ORDER BY d.from_id,d.kind`, id)
		if r == nil {
			return
		}
		type relation struct{ from, kind string }
		relations := []relation{}
		for r.Next() {
			var x relation
			if g.err = r.Scan(&x.from, &x.kind); g.err != nil {
				break
			}
			relations = append(relations, x)
		}
		if g.err == nil {
			g.err = r.Err()
		}
		r.Close()
		if g.err != nil {
			return
		}
		for _, x := range relations {
			g.add(x.from, id, x.kind)
			if serviceSeen[x.from] {
				continue
			}
			serviceSeen[x.from] = true
			affected = append(affected, x.from)
			var host string
			if g.err = g.store.DB.QueryRowContext(g.ctx, `SELECT host_id FROM services WHERE id=?`, x.from).Scan(&host); g.err != nil {
				return
			}
			g.add(x.from, host, "宿主")
			g.endpoints(x.from)
		}
	}
}
func (g *topologyGraph) accessService(service string) {
	var host string
	if g.err = g.store.DB.QueryRowContext(g.ctx, `SELECT host_id FROM services WHERE id=?`, service).Scan(&host); g.err != nil {
		return
	}
	// 链路从入口指向服务，再顺着宿主关系追溯到最上层主机。
	r := g.rows(`SELECT id,via_json FROM endpoints WHERE service_id=? ORDER BY id`, service)
	if r == nil {
		return
	}
	for r.Next() {
		var id, raw string
		if g.err = r.Scan(&id, &raw); g.err != nil {
			break
		}
		var via []string
		if g.err = json.Unmarshal([]byte(raw), &via); g.err != nil {
			break
		}
		prev := id
		for _, proxy := range via {
			g.add(prev, proxy, "访问")
			prev = proxy
		}
		g.add(prev, service, "访问")
	}
	if g.err == nil {
		g.err = r.Err()
	}
	r.Close()
	if g.err != nil {
		return
	}
	g.add(service, host, "宿主")
	visited := map[string]bool{}
	for host != "" && !visited[host] {
		visited[host] = true
		var parent sql.NullString
		g.err = g.store.DB.QueryRowContext(g.ctx, `SELECT parent_host_id FROM hosts WHERE id=?`, host).Scan(&parent)
		if g.err != nil {
			return
		}
		if !parent.Valid {
			break
		}
		g.add(host, parent.String, "父主机")
		host = parent.String
	}
}
