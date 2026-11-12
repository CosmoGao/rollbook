package mcp

import "testing"

func TestSchemaAcceptsEnglishAliases(t *testing.T) {
	for _, tc := range []struct{ field, typ, alias string }{{"status", "host", "active"}, {"kind", "host", "physical"}, {"kind", "location", "site"}, {"assign", "ip", "dhcp_reservation"}} {
		schema := fieldSchema(tc.field, tc.typ)
		values, ok := schema["enum"].([]string)
		if !ok {
			t.Fatalf("%s/%s has no enum", tc.field, tc.typ)
		}
		found := false
		for _, value := range values {
			if value == tc.alias {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s/%s lacks %q: %v", tc.field, tc.typ, tc.alias, values)
		}
	}
}
