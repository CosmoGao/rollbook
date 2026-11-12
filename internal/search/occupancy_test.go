package search

import "testing"

func TestCalculateOccupancy(t *testing.T) {
	tests := []struct {
		name, cidr, gateway, dhcp string
		used                      []AddressUse
		next                      string
		ranges                    int
		conflict                  bool
	}{
		{"边界与网关", "192.0.2.0/29", "192.0.2.1", "", []AddressUse{{Address: "192.0.2.2", IPID: "ip:a"}}, "192.0.2.3", 1, false},
		{"DHCP 和冲突", "192.0.2.0/29", "192.0.2.1", "192.0.2.2-192.0.2.4", []AddressUse{{Address: "192.0.2.5", IPID: "ip:a"}, {Address: "192.0.2.5", IPID: "ip:b"}}, "192.0.2.6", 1, true},
		{"全部占用", "192.0.2.0/30", "192.0.2.1", "", []AddressUse{{Address: "192.0.2.2"}}, "", 0, false},
		{"点对点", "192.0.2.0/31", "", "", []AddressUse{{Address: "192.0.2.0"}}, "192.0.2.1", 1, false},
		{"单地址", "192.0.2.7/32", "", "", nil, "192.0.2.7", 1, false},
		{"IPv6 大网段", "2001:db8::/64", "2001:db8::1", "", nil, "2001:db8::", 2, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := calculateOccupancy(tt.cidr, tt.gateway, tt.dhcp, tt.used)
			if err != nil {
				t.Fatal(err)
			}
			if got.NextAvailable != tt.next || len(got.Free) != tt.ranges {
				t.Fatalf("next=%s free=%+v", got.NextAvailable, got.Free)
			}
			for _, u := range got.Used {
				if u.Conflict != tt.conflict {
					t.Fatalf("conflict=%v", u.Conflict)
				}
			}
		})
	}
}

func TestInvalidDHCPDoesNotAllocate(t *testing.T) {
	if _, err := calculateOccupancy("192.0.2.0/24", "", "invalid", nil); err == nil {
		t.Fatal("invalid DHCP range accepted")
	}
}
