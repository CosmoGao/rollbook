package search

import (
	"context"
	"database/sql"
	"net/url"
	"strings"
)

// Summary 是只读展示数据。site_id 始终指位置父链上的 kind=站点节点，
// 位置自身不是站点时不能把它当作 site_id；孤立位置返回空值。
type Summary struct {
	SiteID    string   `json:"site_id,omitempty"`
	SiteName  string   `json:"site_name,omitempty"`
	Kind      string   `json:"kind,omitempty"`
	PrimaryIP string   `json:"primary_ip,omitempty"`
	HostID    string   `json:"host_id,omitempty"`
	HostName  string   `json:"host_name,omitempty"`
	Services  []string `json:"services,omitempty"`
	Endpoints []string `json:"endpoints,omitempty"`
}

// Contexts 一次读取关联表，避免浏览及搜索逐结果查询站点、IP 和入口。
// 仅返回公开展示字段，不读取凭据表；入口仅保留可安全打开的 URL。
func (s Store) Contexts(ctx context.Context, ids []string) (map[string]Summary, error) {
	out := make(map[string]Summary, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	wanted := map[string]bool{}
	for _, id := range ids {
		wanted[id] = true
	}
	type location struct{ parent, kind, name string }
	locations := map[string]location{}
	rows, err := s.DB.QueryContext(ctx, `SELECT l.id,coalesce(l.parent_id,''),l.kind,o.name FROM locations l JOIN objects o ON o.id=l.id`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id string
		var l location
		if err = rows.Scan(&id, &l.parent, &l.kind, &l.name); err != nil {
			break
		}
		locations[id] = l
	}
	err = closeRows(rows, err)
	if err != nil {
		return nil, err
	}
	site := func(id string) (string, string) {
		seen := map[string]bool{}
		for id != "" && !seen[id] {
			seen[id] = true
			l, ok := locations[id]
			if !ok {
				break
			}
			if l.kind == "站点" {
				return id, l.name
			}
			id = l.parent
		}
		return "", ""
	}
	type host struct{ location, parent, name, kind string }
	hosts := map[string]host{}
	rows, err = s.DB.QueryContext(ctx, `SELECT h.id,h.location_id,coalesce(h.parent_host_id,''),h.kind,o.name FROM hosts h JOIN objects o ON o.id=h.id`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id string
		var h host
		if err = rows.Scan(&id, &h.location, &h.parent, &h.kind, &h.name); err != nil {
			break
		}
		hosts[id] = h
	}
	err = closeRows(rows, err)
	if err != nil {
		return nil, err
	}
	for id := range wanted {
		if l, ok := locations[id]; ok {
			v := Summary{Kind: l.kind}
			v.SiteID, v.SiteName = site(id)
			out[id] = v
		}
		if h, ok := hosts[id]; ok {
			v := Summary{Kind: h.kind, HostID: id, HostName: h.name}
			v.SiteID, v.SiteName = site(h.location)
			out[id] = v
		}
	}
	rows, err = s.DB.QueryContext(ctx, `SELECT id,location_id FROM networks`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id, locationID string
		if err = rows.Scan(&id, &locationID); err != nil {
			break
		}
		if wanted[id] {
			v := out[id]
			v.SiteID, v.SiteName = site(locationID)
			out[id] = v
		}
	}
	if err = closeRows(rows, err); err != nil {
		return nil, err
	}
	rows, err = s.DB.QueryContext(ctx, `SELECT id,location_id FROM uplinks`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id, locationID string
		if err = rows.Scan(&id, &locationID); err != nil {
			break
		}
		if wanted[id] {
			v := out[id]
			v.SiteID, v.SiteName = site(locationID)
			out[id] = v
		}
	}
	if err = closeRows(rows, err); err != nil {
		return nil, err
	}
	ipByHost := map[string][]string{}
	rows, err = s.DB.QueryContext(ctx, `SELECT i.id,i.host_id,i.address FROM ips i ORDER BY i.host_id,i.address,i.id`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id, hid, address string
		if err = rows.Scan(&id, &hid, &address); err != nil {
			break
		}
		ipByHost[hid] = append(ipByHost[hid], address)
		if wanted[id] {
			v := out[id]
			v.HostID = hid
			v.HostName = hosts[hid].name
			v.PrimaryIP = address
			v.SiteID, v.SiteName = site(hosts[hid].location)
			out[id] = v
		}
		if wanted[hid] {
			v := out[hid]
			if v.PrimaryIP == "" {
				v.PrimaryIP = address
			}
			out[hid] = v
		}
	}
	err = closeRows(rows, err)
	if err != nil {
		return nil, err
	}
	serviceHosts := map[string]string{}
	rows, err = s.DB.QueryContext(ctx, `SELECT s.id,s.host_id,o.name,coalesce(s.kind,'') FROM services s JOIN objects o ON o.id=s.id ORDER BY o.name,s.id`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id, hid, name, kind string
		if err = rows.Scan(&id, &hid, &name, &kind); err != nil {
			break
		}
		serviceHosts[id] = hid
		if wanted[id] {
			v := Summary{Kind: kind, HostID: hid, HostName: hosts[hid].name}
			v.SiteID, v.SiteName = site(hosts[hid].location)
			if len(ipByHost[hid]) > 0 {
				v.PrimaryIP = ipByHost[hid][0]
			}
			out[id] = v
		}
		if wanted[hid] {
			v := out[hid]
			v.Services = append(v.Services, name)
			out[hid] = v
		}
		for ipid, v := range out {
			if strings.HasPrefix(ipid, "ip:") && v.HostID == hid && len(v.Services) < 5 {
				v.Services = append(v.Services, name)
				out[ipid] = v
			}
		}
	}
	err = closeRows(rows, err)
	if err != nil {
		return nil, err
	}
	rows, err = s.DB.QueryContext(ctx, `SELECT e.id,e.service_id,e.url,e.kind FROM endpoints e ORDER BY e.id`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id, sid, address, kind string
		if err = rows.Scan(&id, &sid, &address, &kind); err != nil {
			break
		}
		u, e := url.Parse(address)
		if e != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
			continue
		}
		hid := serviceHosts[sid]
		for target := range wanted {
			if target != id && target != sid && target != hid && !(strings.HasPrefix(target, "ip:") && out[target].HostID == hid) {
				continue
			}
			v := out[target]
			if target == id {
				v.Kind = kind
				v.HostID = hid
				v.HostName = hosts[hid].name
				v.SiteID, v.SiteName = site(hosts[hid].location)
			}
			if len(v.Endpoints) < 5 {
				v.Endpoints = append(v.Endpoints, address)
			}
			out[target] = v
		}
	}
	err = closeRows(rows, err)
	return out, err
}

func closeRows(rows *sql.Rows, err error) error {
	if err == nil {
		err = rows.Err()
	}
	if closeErr := rows.Close(); err == nil {
		err = closeErr
	}
	return err
}
