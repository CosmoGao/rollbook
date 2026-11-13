package search

import (
	"context"
)

// LayerNode 是只读画布节点，父链和站点归属在浏览器端用于展开及筛选。
type LayerNode struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Kind       string `json:"kind,omitempty"`
	ParentID   string `json:"parent_id,omitempty"`
	LocationID string `json:"location_id,omitempty"`
	HostID     string `json:"host_id,omitempty"`
	URL        string `json:"url,omitempty"`
	IP         string `json:"ip,omitempty"`
}
type LayerEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}
type Layers struct {
	Nodes []LayerNode `json:"nodes"`
	Edges []LayerEdge `json:"edges"`
}

// TopologyLayers 固定数量的查询返回分层资产、入口及原始依赖，规模与服务数量无关。
func (s Store) TopologyLayers(ctx context.Context) (Layers, error) {
	out := Layers{Nodes: []LayerNode{}, Edges: []LayerEdge{}}
	queries := []struct{ sql, typ string }{
		{`SELECT o.id,o.name,l.kind,COALESCE(l.parent_id,''),'','','','' FROM locations l JOIN objects o ON o.id=l.id ORDER BY o.id`, "location"},
		{`SELECT o.id,o.name,h.kind,COALESCE(h.parent_host_id,''),h.location_id,'','',COALESCE((SELECT i.address FROM ips i WHERE i.host_id=h.id ORDER BY i.id LIMIT 1),'') FROM hosts h JOIN objects o ON o.id=h.id ORDER BY o.id`, "host"},
		{`SELECT o.id,o.name,COALESCE(s.kind,''),'','',s.host_id,'','' FROM services s JOIN objects o ON o.id=s.id ORDER BY o.id`, "service"},
		{`SELECT o.id,o.name,e.kind,'','',e.service_id,e.url,'' FROM endpoints e JOIN objects o ON o.id=e.id ORDER BY o.id`, "endpoint"},
	}
	for _, query := range queries {
		rows, err := s.DB.QueryContext(ctx, query.sql)
		if err != nil {
			return Layers{}, err
		}
		for rows.Next() {
			n := LayerNode{Type: query.typ}
			if err = rows.Scan(&n.ID, &n.Name, &n.Kind, &n.ParentID, &n.LocationID, &n.HostID, &n.URL, &n.IP); err != nil {
				rows.Close()
				return Layers{}, err
			}
			out.Nodes = append(out.Nodes, n)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return Layers{}, err
		}
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT from_id,to_id,kind FROM dependencies ORDER BY from_id,to_id,kind`)
	if err != nil {
		return Layers{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var e LayerEdge
		if err = rows.Scan(&e.From, &e.To, &e.Kind); err != nil {
			return Layers{}, err
		}
		out.Edges = append(out.Edges, e)
	}
	if err = rows.Err(); err != nil {
		return Layers{}, err
	}
	return out, nil
}
