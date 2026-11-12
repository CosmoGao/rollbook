package backup

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/catalog"
	"github.com/CosmoGao/rollbook/internal/db"
)

func TestCSVImportApproval(t *testing.T) {
	ctx := context.Background()
	d, err := db.Open(ctx, filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	for _, q := range []string{
		`INSERT INTO objects(id,type,name,created_at,created_by,updated_at,updated_by) VALUES('location:a','location','站点','now','owner','now','owner')`,
		`INSERT INTO locations(id,kind) VALUES('location:a','站点')`,
	} {
		if _, err = d.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	s := Service{DB: d}
	actor := auth.Principal{KeyID: "key", KeyName: "agent", IssuerID: "owner", IssuerName: "owner", Level: auth.Propose}
	for _, tc := range []struct{ kind, row, id string }{
		{"host", "host:a,主机,物理机,location:a", "host:a"},
		{"service", "service:a,服务,host:a", "service:a"},
	} {
		header := "id,name,kind,location_id\n"
		if tc.kind == "service" {
			header = "id,name,host_id\n"
		}
		data, err := CSVToMerge(tc.kind, []byte(header+tc.row+"\n"))
		if err != nil {
			t.Fatal(err)
		}
		item, err := s.Submit(ctx, data, actor)
		if err != nil {
			t.Fatal(err)
		}
		if item.Preview.Counts["create"] != 1 {
			t.Fatal(item.Preview)
		}
		var count int
		d.QueryRowContext(ctx, "SELECT count(*) FROM objects WHERE id=?", tc.id).Scan(&count)
		if count != 0 {
			t.Fatal("propose key bypassed approval")
		}
		if err := s.Review(ctx, item.ID, true, "owner"); err != nil {
			t.Fatal(err)
		}
		d.QueryRowContext(ctx, "SELECT count(*) FROM search_documents WHERE object_id=?", tc.id).Scan(&count)
		if count != 1 {
			t.Fatal("missing search index", tc.id)
		}
	}
}

func TestCSVValidation(t *testing.T) {
	for _, tc := range []struct{ data, want string }{
		{"id,name,kind,location_id,unknown\nhost:a,A,物理机,location:a,x\n", "第 1 行 unknown 列"},
		{"id,name,kind,location_id\nhost:a,A,物理机,location:a\nhost:a,B,物理机,location:a\n", "第 3 行 id 列"},
		{"id,name,kind,location_id,mgmt_url\nhost:a,A,物理机,location:a,javascript:alert(1)\n", "第 2 行 mgmt_url 列"},
	} {
		_, err := CSVToMerge("host", []byte(tc.data))
		var validation *catalog.ValidationError
		if !errors.As(err, &validation) || len(validation.Fields) != 1 || !strings.Contains(validation.Fields[0].Field, tc.want) {
			t.Fatalf("%q: %v", tc.want, err)
		}
	}
	for _, kind := range []string{"host", "network", "ip", "service"} {
		if _, err := CSVTemplate(kind); err != nil {
			t.Fatal(err)
		}
	}
}
