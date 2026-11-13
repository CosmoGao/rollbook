package search

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
)

// ImpactNode 记录故障对象到受影响对象的最短已知路径。
type ImpactNode struct {
	ID     string   `json:"id"`
	Path   []string `json:"path"`
	Source string   `json:"source"`
}

type Impact struct {
	Hosts     []ImpactNode `json:"hosts"`
	Services  []ImpactNode `json:"services"`
	Endpoints []ImpactNode `json:"endpoints"`
}

type impactEdge struct{ to, source string }

// Impact 沿宿主、服务反向依赖和入口经由链计算影响面。
// 每张关系表只读一次；已访问 ID 限制遍历，导入数据有环也能终止。
func (s Store) Impact(ctx context.Context, kind, id string, recursive bool) (Impact, error) {
	return ImpactFrom(ctx, s.DB, kind, id, recursive)
}

// ImpactFrom 让停用预览与事务读取共用同一计算逻辑。
func ImpactFrom(ctx context.Context, db interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, kind, id string, recursive bool) (Impact, error) {
	result := Impact{Hosts: []ImpactNode{}, Services: []ImpactNode{}, Endpoints: []ImpactNode{}}
	edges := map[string][]impactEdge{}
	add := func(from, to, source string) {
		if from != "" && to != "" {
			edges[from] = append(edges[from], impactEdge{to, source})
		}
	}
	read := func(query string, scan func(*sql.Rows) error) error {
		rows, err := db.QueryContext(ctx, query)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			if err = scan(rows); err != nil {
				return err
			}
		}
		return rows.Err()
	}
	if err := read(`SELECT id,parent_host_id FROM hosts ORDER BY id`, func(r *sql.Rows) error {
		var child string
		var parent sql.NullString
		if e := r.Scan(&child, &parent); e != nil {
			return e
		}
		add(parent.String, child, "parent_host_id")
		return nil
	}); err != nil {
		return result, err
	}
	if err := read(`SELECT id,host_id FROM services ORDER BY id`, func(r *sql.Rows) error {
		var child, parent string
		if e := r.Scan(&child, &parent); e != nil {
			return e
		}
		add(parent, child, "host_id")
		return nil
	}); err != nil {
		return result, err
	}
	if err := read(`SELECT from_id,to_id FROM dependencies JOIN objects o ON o.id=from_id WHERE o.type='service' AND dependencies.generated=0 ORDER BY from_id,to_id`, func(r *sql.Rows) error {
		var from, to string
		if e := r.Scan(&from, &to); e != nil {
			return e
		}
		add(to, from, "dependency")
		return nil
	}); err != nil {
		return result, err
	}
	if err := read(`SELECT id,service_id,via_json FROM endpoints ORDER BY id`, func(r *sql.Rows) error {
		var endpoint, service, raw string
		if e := r.Scan(&endpoint, &service, &raw); e != nil {
			return e
		}
		add(service, endpoint, "service_id")
		var via []string
		if e := json.Unmarshal([]byte(raw), &via); e != nil {
			return e
		}
		for _, v := range via {
			add(v, endpoint, "via_json")
		}
		return nil
	}); err != nil {
		return result, err
	}
	for from := range edges {
		sort.Slice(edges[from], func(i, j int) bool {
			a, b := edges[from][i], edges[from][j]
			if a.to == b.to {
				return a.source < b.source
			}
			return a.to < b.to
		})
	}
	type state struct {
		id     string
		path   []string
		depth  int
		source string
	}
	queue := []state{{id: id, path: []string{id}}}
	seen := map[string]bool{id: true}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if !recursive && current.depth > 0 {
			continue
		}
		for _, edge := range edges[current.id] {
			if seen[edge.to] {
				continue
			}
			seen[edge.to] = true
			path := append(append([]string{}, current.path...), edge.to)
			item := ImpactNode{ID: edge.to, Path: path, Source: edge.source}
			switch {
			case containsType(edge.to, "host") && kind == "host":
				result.Hosts = append(result.Hosts, item)
			case containsType(edge.to, "service"):
				result.Services = append(result.Services, item)
			case containsType(edge.to, "endpoint"):
				result.Endpoints = append(result.Endpoints, item)
			}
			queue = append(queue, state{id: edge.to, path: path, depth: current.depth + 1, source: edge.source})
		}
	}
	sort.Slice(result.Hosts, func(i, j int) bool { return result.Hosts[i].ID < result.Hosts[j].ID })
	sort.Slice(result.Services, func(i, j int) bool { return result.Services[i].ID < result.Services[j].ID })
	sort.Slice(result.Endpoints, func(i, j int) bool { return result.Endpoints[i].ID < result.Endpoints[j].ID })
	return result, nil
}

func containsType(id, kind string) bool { return len(id) > len(kind) && id[:len(kind)+1] == kind+":" }
