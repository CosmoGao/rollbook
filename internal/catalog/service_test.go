package catalog_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/catalog"
	"github.com/CosmoGao/rollbook/internal/db"
)

func serviceSetup(t *testing.T) (context.Context, *sql.DB, catalog.Service, auth.Service, auth.Principal) {
	t.Helper()
	ctx := context.Background()
	d, e := db.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { d.Close() })
	a := auth.Service{DB: d, Pepper: []byte("0123456789abcdef0123456789abcdef")}
	id, e := a.BootstrapUser(ctx, "owner", "strong password 12345")
	if e != nil {
		t.Fatal(e)
	}
	return ctx, d, catalog.Service{Store: catalog.Store{DB: d}}, a, auth.Principal{UserID: id, Login: "owner"}
}
func TestKeyLifecycleAndPermissionMatrix(t *testing.T) {
	ctx, d, _, a, user := serviceSetup(t)
	levels := []auth.Level{auth.Read, auth.Propose, auth.Write}
	keys := make([]string, 3)
	principals := make([]auth.Principal, 3)
	for i, l := range levels {
		k, raw, e := a.IssueKey(ctx, user, string(l), l, nil, "")
		if e != nil {
			t.Fatal(e)
		}
		keys[i] = raw
		p, e := a.Authenticate(ctx, raw)
		if e != nil {
			t.Fatal(e)
		}
		principals[i] = p
		if p.KeyID != k.ID || p.IssuerName != "owner" {
			t.Fatal(p)
		}
	}
	var stored string
	if e := d.QueryRow("SELECT hex(token_hash) FROM api_keys WHERE id=?", principals[0].KeyID).Scan(&stored); e != nil {
		t.Fatal(e)
	}
	if strings.Contains(stored, keys[0]) {
		t.Fatal("plaintext key stored")
	}
	ops := []auth.Operation{auth.View, auth.PutContext, auth.RecordNote, auth.ProposeChange, auth.Update, auth.MarkVerified, auth.CheckVersions, auth.Approve, auth.Reject, auth.Delete, auth.ManageKeys, auth.Restore}
	want := [][]bool{{true, true, false, false, false, false, false, false, false, false, false, false}, {true, true, true, true, false, false, false, false, false, false, false, false}, {true, true, true, true, true, true, true, false, false, false, false, false}}
	for i, p := range principals {
		for j, op := range ops {
			e := auth.Authorize(p, op, p.KeyID)
			if (e == nil) != want[i][j] {
				t.Fatalf("level %s op %s: %v", p.Level, op, e)
			}
		}
		if e := auth.Authorize(p, auth.PutContext, "another-key"); !errors.Is(e, auth.ErrForbidden) {
			t.Fatal(e)
		}
		if _, _, e := a.IssueKey(ctx, p, "bad", auth.Read, nil, ""); !errors.Is(e, auth.ErrForbidden) {
			t.Fatal(e)
		}
	}
	if e := a.SetLevel(ctx, user, principals[0].KeyID, auth.Write); e != nil {
		t.Fatal(e)
	}
	promoted, e := a.Authenticate(ctx, keys[0])
	if e != nil || promoted.Level != auth.Write {
		t.Fatal(promoted, e)
	}
	if e := a.Revoke(ctx, user, principals[1].KeyID); e != nil {
		t.Fatal(e)
	}
	if _, e := a.Authenticate(ctx, keys[1]); !errors.Is(e, auth.ErrUnauthorized) {
		t.Fatal(e)
	}
	if _, e := a.Authenticate(ctx, keys[2]); e != nil {
		t.Fatal(e)
	}
}
func TestSessionCSRFAndCookie(t *testing.T) {
	ctx, _, _, a, _ := serviceSetup(t)
	sid, csrf, p, e := a.Login(ctx, "owner", "strong password 12345")
	if e != nil || !p.IsUser() {
		t.Fatal(e, p)
	}
	if _, e = a.Session(ctx, sid, "", true); !errors.Is(e, auth.ErrForbidden) {
		t.Fatal(e)
	}
	if _, e = a.Session(ctx, sid, csrf, true); e != nil {
		t.Fatal(e)
	}
	c := auth.SessionCookie(sid)
	if !c.Secure || !c.HttpOnly || c.SameSite == 0 {
		t.Fatal(c)
	}
	if e = a.Logout(ctx, sid); e != nil {
		t.Fatal(e)
	}
	if _, e = a.Session(ctx, sid, csrf, false); !errors.Is(e, auth.ErrUnauthorized) {
		t.Fatal(e)
	}
}
func TestUpdateProposalAuditAndSecrets(t *testing.T) {
	ctx, d, s, a, user := serviceSetup(t)
	o, e := s.Store.Create(ctx, catalog.Object{ID: "location:one", Type: "location", Name: "One", Fields: map[string]any{"kind": "站点"}}, "owner")
	if e != nil {
		t.Fatal(e)
	}
	_, readRaw, e := a.IssueKey(ctx, user, "reader", auth.Read, nil, "")
	if e != nil {
		t.Fatal(e)
	}
	read, _ := a.Authenticate(ctx, readRaw)
	_, writeRaw, e := a.IssueKey(ctx, user, "writer", auth.Write, nil, "")
	if e != nil {
		t.Fatal(e)
	}
	writer, _ := a.Authenticate(ctx, writeRaw)
	name := "Two"
	patch := catalog.Patch{Name: &name}
	if _, e = s.Update(ctx, read, o.ID, 1, patch, "because", "test"); !errors.Is(e, auth.ErrForbidden) {
		t.Fatal(e)
	}
	updated, e := s.Update(ctx, writer, o.ID, 1, patch, "because", "test")
	if e != nil {
		t.Fatal(e)
	}
	if updated.Rev != 2 || updated.Name != "Two" {
		t.Fatal(updated)
	}
	if _, e = s.Update(ctx, writer, o.ID, 1, patch, "because", "test"); !errors.Is(e, catalog.ErrConflict) {
		t.Fatal(e)
	}
	actual, e := s.Store.Get(ctx, o.ID)
	if e != nil || actual.Rev != 2 || actual.Name != "Two" {
		t.Fatal(actual, e)
	}
	var keyName, issuer, reason string
	if e = d.QueryRow("SELECT actor_key_name,actor_issuer_name,reason FROM changes WHERE object_id=? AND action='update'", o.ID).Scan(&keyName, &issuer, &reason); e != nil {
		t.Fatal(e)
	}
	if keyName != "writer" || issuer != "owner" || reason != "because" {
		t.Fatal(keyName, issuer, reason)
	}
	bad := "password=topsecret"
	if _, e = s.Update(ctx, writer, o.ID, 2, catalog.Patch{Name: &bad}, "because", "test"); !errors.Is(e, catalog.ErrSecret) || strings.Contains(e.Error(), "topsecret") {
		t.Fatal(e)
	}
	if _, e = s.Update(ctx, writer, o.ID, 2, catalog.Patch{Fields: map[string]any{"description": "-----BEGIN PRIVATE KEY-----"}}, "because", "test"); !errors.Is(e, catalog.ErrSecret) {
		t.Fatal(e)
	}
	change, e := s.Propose(ctx, writer, o.ID, 2, catalog.Patch{Name: &name}, "proposed reason", "test")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Review(ctx, writer, change, true); !errors.Is(e, auth.ErrForbidden) {
		t.Fatal(e)
	}
	reviewed, e := s.Review(ctx, user, change, true)
	if e != nil || reviewed.Rev != 3 {
		t.Fatal(reviewed, e)
	}
	var reviewer, state string
	if e = d.QueryRow("SELECT reviewed_by,state FROM changes WHERE id=?", change).Scan(&reviewer, &state); e != nil || reviewer != "owner" || state != "已生效" {
		t.Fatal(reviewer, state, e)
	}
}

func TestContextOwnerAndReviewConflict(t *testing.T) {
	ctx, d, s, a, user := serviceSetup(t)
	_, firstRaw, e := a.IssueKey(ctx, user, "first", auth.Read, nil, "")
	if e != nil {
		t.Fatal(e)
	}
	first, _ := a.Authenticate(ctx, firstRaw)
	_, secondRaw, e := a.IssueKey(ctx, user, "second", auth.Read, nil, "")
	if e != nil {
		t.Fatal(e)
	}
	second, _ := a.Authenticate(ctx, secondRaw)
	created, e := s.CreateContext(ctx, first, catalog.Object{ID: "context:one", Type: "context", Name: "One", Fields: map[string]any{"scope": "会话", "key": "one", "body_md": "safe"}}, "share", "test")
	if e != nil {
		t.Fatal(e)
	}
	if created.Fields["owner"] != first.KeyID {
		t.Fatal(created.Fields)
	}
	if e = s.PutContext(ctx, second, created.ID, 1, "other", created.CreatedAt.Add(24*time.Hour), "renew", "test"); !errors.Is(e, auth.ErrForbidden) {
		t.Fatal(e)
	}
	var count int
	if e = d.QueryRow("SELECT count(*) FROM changes WHERE object_id=?", created.ID).Scan(&count); e != nil || count != 1 {
		t.Fatal(count, e)
	}
	location, e := s.Store.Create(ctx, catalog.Object{ID: "location:site", Type: "location", Name: "Site", Fields: map[string]any{"kind": "站点"}}, "owner")
	if e != nil {
		t.Fatal(e)
	}
	_, proposeRaw, e := a.IssueKey(ctx, user, "proposer", auth.Propose, nil, "")
	if e != nil {
		t.Fatal(e)
	}
	proposer, _ := a.Authenticate(ctx, proposeRaw)
	name := "Changed"
	change, e := s.Propose(ctx, proposer, location.ID, 1, catalog.Patch{Name: &name}, "proposal", "test")
	if e != nil {
		t.Fatal(e)
	}
	_, e = s.Update(ctx, user, location.ID, 1, catalog.Patch{Name: &name}, "direct", "test")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Review(ctx, user, change, true); !errors.Is(e, catalog.ErrConflict) {
		t.Fatal(e)
	}
	var state string
	if e = d.QueryRow("SELECT state FROM changes WHERE id=?", change).Scan(&state); e != nil || state != "待确认" {
		t.Fatal(state, e)
	}
	if _, e = s.ProposeDelete(ctx, first, location.ID, 2, "delete", "test"); !errors.Is(e, auth.ErrForbidden) {
		t.Fatal(e)
	}
	deleteChange, e := s.ProposeDelete(ctx, proposer, location.ID, 2, "delete", "test")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Review(ctx, proposer, deleteChange, true); !errors.Is(e, auth.ErrForbidden) {
		t.Fatal(e)
	}
	if _, e = s.Review(ctx, user, deleteChange, true); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Store.Get(ctx, location.ID); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal(e)
	}
}

func TestRejectStaleProposal(t *testing.T) {
	ctx, d, s, a, user := serviceSetup(t)
	_, raw, e := a.IssueKey(ctx, user, "proposer", auth.Propose, nil, "")
	if e != nil {
		t.Fatal(e)
	}
	proposer, e := a.Authenticate(ctx, raw)
	if e != nil {
		t.Fatal(e)
	}
	for _, scenario := range []string{"updated", "deleted"} {
		t.Run(scenario, func(t *testing.T) {
			id := "location:" + scenario
			o, e := s.Store.Create(ctx, catalog.Object{ID: id, Type: "location", Name: "Probe", Fields: map[string]any{"kind": "站点"}}, "owner")
			if e != nil {
				t.Fatal(e)
			}
			name := "Proposal"
			change, e := s.Propose(ctx, proposer, o.ID, 1, catalog.Patch{Name: &name}, "proposal", "test")
			if e != nil {
				t.Fatal(e)
			}
			if scenario == "updated" {
				newer := "Direct update"
				if _, e = s.Update(ctx, user, o.ID, 1, catalog.Patch{Name: &newer}, "direct", "test"); e != nil {
					t.Fatal(e)
				}
			} else {
				deleteChange, e := s.ProposeDelete(ctx, proposer, o.ID, 1, "remove", "test")
				if e != nil {
					t.Fatal(e)
				}
				if _, e = s.Review(ctx, user, deleteChange, true); e != nil {
					t.Fatal(e)
				}
			}
			if _, e = s.Review(ctx, user, change, false); e != nil {
				t.Fatal(e)
			}
			var state, reviewer string
			if e = d.QueryRow("SELECT state,reviewed_by FROM changes WHERE id=?", change).Scan(&state, &reviewer); e != nil || state != "已驳回" || reviewer != "owner" {
				t.Fatal(state, reviewer, e)
			}
			if scenario == "updated" {
				got, e := s.Store.Get(ctx, o.ID)
				if e != nil || got.Name != "Direct update" || got.Rev != 2 {
					t.Fatal(got, e)
				}
			} else if _, e = s.Store.Get(ctx, o.ID); !errors.Is(e, sql.ErrNoRows) {
				t.Fatal(e)
			}
			if _, e = s.Review(ctx, user, change, false); !errors.Is(e, catalog.ErrConflict) {
				t.Fatal(e)
			}
		})
	}
}
