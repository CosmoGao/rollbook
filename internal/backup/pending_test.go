package backup

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/catalog"
	"github.com/CosmoGao/rollbook/internal/db"
)

func TestPendingLimitAndExpiry(t *testing.T) {
	ctx := context.Background()
	database, err := db.Open(ctx, filepath.Join(t.TempDir(), "pending.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	s := Service{DB: database}
	data := []byte("schema_version: 1\nkind: merge\nobjects:\n  - id: location:test\n    type: location\n    name: 测试\n    fields: {kind: 站点}\n")
	actor := auth.Principal{UserID: "u", Login: "owner"}
	var first string
	for i := 0; i < 10; i++ {
		item, e := s.Submit(ctx, data, actor)
		if e != nil {
			t.Fatal(i, e)
		}
		if i == 0 {
			first = item.ID
		}
	}
	if _, err = s.Submit(ctx, data, actor); !errors.Is(err, ErrInvalid) {
		t.Fatal("limit", err)
	}
	key := auth.Principal{KeyID: "k", KeyName: "agent", IssuerID: "u", IssuerName: "owner", Level: auth.Propose}
	if _, err = s.Submit(ctx, data, key); !errors.Is(err, ErrInvalid) {
		t.Fatal("issuer limit", err)
	}
	expired := time.Now().UTC().Add(-8 * 24 * time.Hour).Format(time.RFC3339Nano)
	if _, err = database.Exec("UPDATE pending_imports SET created_at=? WHERE id=?", expired, first); err != nil {
		t.Fatal(err)
	}
	if err = s.CleanupExpired(ctx); err != nil {
		t.Fatal(err)
	}
	var state string
	var size int
	if err = database.QueryRow("SELECT state,length(yaml) FROM pending_imports WHERE id=?", first).Scan(&state, &size); err != nil || state != "expired" || size != 0 {
		t.Fatal(state, size, err)
	}
	if _, err = s.Submit(ctx, data, actor); err != nil {
		t.Fatal("slot not freed", err)
	}
}

func TestMergeURLFieldError(t *testing.T) {
	data := []byte("schema_version: 1\nkind: merge\nobjects:\n  - id: endpoint:test\n    type: endpoint\n    name: 入口\n    fields: {url: 'javascript:alert(1)'}\n")
	_, _, err := Parse(data)
	var validation *catalog.ValidationError
	if !errors.As(err, &validation) || len(validation.Fields) == 0 || validation.Fields[0].Field != "fields.url" {
		t.Fatal(err)
	}
}
