package search

import (
	"context"
	"database/sql"
	"fmt"
	"net/netip"
	"sort"
	"strings"
)

// AddressUse 保留每条登记记录，避免重复地址被集合去重后隐藏冲突。
type AddressUse struct {
	Address  string `json:"address"`
	IPID     string `json:"ip_id"`
	HostID   string `json:"host_id"`
	Gateway  bool   `json:"gateway,omitempty"`
	DHCP     bool   `json:"dhcp,omitempty"`
	Conflict bool   `json:"conflict,omitempty"`
}

type AddressRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type NetworkOccupancy struct {
	CIDR          string         `json:"cidr"`
	Gateway       string         `json:"gateway,omitempty"`
	DHCPRange     string         `json:"dhcp_range,omitempty"`
	Used          []AddressUse   `json:"used"`
	Free          []AddressRange `json:"free"`
	NextAvailable string         `json:"next_available,omitempty"`
}

// 可分配范围：IPv4 /30 及更大网段排除网络地址和广播地址；/31、/32
// 按点对点及单地址网段处理。IPv6 没有广播地址。
func usableBounds(p netip.Prefix) (netip.Addr, netip.Addr) {
	first := p.Masked().Addr()
	bytes := first.AsSlice()
	for i := p.Bits(); i < first.BitLen(); i++ {
		bytes[i/8] |= 1 << (7 - uint(i%8))
	}
	last, _ := netip.AddrFromSlice(bytes)
	if first.Is4() && p.Bits() <= 30 {
		first, last = first.Next(), last.Prev()
	}
	return first, last
}

func parseDHCP(value string, p netip.Prefix) (netip.Addr, netip.Addr, error) {
	if strings.TrimSpace(value) == "" {
		return netip.Addr{}, netip.Addr{}, nil
	}
	parts := strings.Split(value, "-")
	if len(parts) != 2 {
		return netip.Addr{}, netip.Addr{}, fmt.Errorf("invalid DHCP range %q", value)
	}
	start, e1 := netip.ParseAddr(strings.TrimSpace(parts[0]))
	end, e2 := netip.ParseAddr(strings.TrimSpace(parts[1]))
	if e1 != nil || e2 != nil || !p.Contains(start) || !p.Contains(end) || start.Compare(end) > 0 {
		return netip.Addr{}, netip.Addr{}, fmt.Errorf("invalid DHCP range %q", value)
	}
	return start, end, nil
}

// calculateOccupancy 用范围运算处理大网段，运行时间取决于登记数，而非地址空间大小。
func calculateOccupancy(cidr, gateway, dhcp string, used []AddressUse) (NetworkOccupancy, error) {
	o := NetworkOccupancy{CIDR: cidr, Gateway: gateway, DHCPRange: dhcp, Used: used, Free: []AddressRange{}}
	p, err := netip.ParsePrefix(cidr)
	if err != nil {
		return o, err
	}
	first, last := usableBounds(p)
	var gw netip.Addr
	if gateway != "" {
		gw, err = netip.ParseAddr(gateway)
		if err != nil || !p.Contains(gw) {
			return o, fmt.Errorf("invalid gateway %q", gateway)
		}
	}
	dStart, dEnd, err := parseDHCP(dhcp, p)
	if err != nil {
		return o, err
	}
	counts := map[string]int{}
	blocked := []AddressRange{}
	for i := range o.Used {
		a, e := netip.ParseAddr(o.Used[i].Address)
		if e != nil {
			return o, e
		}
		counts[o.Used[i].Address]++
		o.Used[i].Gateway = a == gw
		o.Used[i].DHCP = dStart.IsValid() && a.Compare(dStart) >= 0 && a.Compare(dEnd) <= 0
		blocked = append(blocked, AddressRange{a.String(), a.String()})
	}
	for i := range o.Used {
		o.Used[i].Conflict = counts[o.Used[i].Address] > 1
	}
	if gw.IsValid() {
		blocked = append(blocked, AddressRange{gw.String(), gw.String()})
	}
	if dStart.IsValid() {
		blocked = append(blocked, AddressRange{dStart.String(), dEnd.String()})
	}
	sort.Slice(blocked, func(i, j int) bool {
		a, _ := netip.ParseAddr(blocked[i].Start)
		b, _ := netip.ParseAddr(blocked[j].Start)
		return a.Compare(b) < 0
	})
	cursor := first
	addFree := func(end netip.Addr) {
		if cursor.IsValid() && cursor.Compare(end) <= 0 {
			o.Free = append(o.Free, AddressRange{cursor.String(), end.String()})
			if o.NextAvailable == "" {
				o.NextAvailable = cursor.String()
			}
		}
	}
	for _, r := range blocked {
		start, _ := netip.ParseAddr(r.Start)
		end, _ := netip.ParseAddr(r.End)
		if end.Compare(first) < 0 || start.Compare(last) > 0 || !cursor.IsValid() || cursor.Compare(last) > 0 {
			continue
		}
		if start.Compare(cursor) > 0 {
			addFree(start.Prev())
		}
		if end.Compare(cursor) >= 0 {
			cursor = end.Next()
		}
	}
	if cursor.IsValid() && cursor.Compare(last) <= 0 {
		addFree(last)
	}
	return o, nil
}

func (s Store) NetworkOccupancy(ctx context.Context, id string) (NetworkOccupancy, error) {
	var cidr string
	var gateway, dhcp sql.NullString
	err := s.DB.QueryRowContext(ctx, `SELECT cidr,gateway,dhcp_range FROM networks WHERE id=?`, id).Scan(&cidr, &gateway, &dhcp)
	if err != nil {
		return NetworkOccupancy{}, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT id,address,host_id FROM ips WHERE network_id=? ORDER BY address,id`, id)
	if err != nil {
		return NetworkOccupancy{}, err
	}
	used := []AddressUse{}
	for rows.Next() {
		var u AddressUse
		if err = rows.Scan(&u.IPID, &u.Address, &u.HostID); err != nil {
			break
		}
		used = append(used, u)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return NetworkOccupancy{}, err
	}
	return calculateOccupancy(cidr, gateway.String, dhcp.String, used)
}
