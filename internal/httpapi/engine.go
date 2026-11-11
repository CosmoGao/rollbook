// Package httpapi exposes the catalog through a single authenticated operation dispatcher.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/backup"
	"github.com/CosmoGao/rollbook/internal/catalog"
	"github.com/CosmoGao/rollbook/internal/secrets"
	"github.com/CosmoGao/rollbook/internal/versioncheck"
)

type Engine struct {
	Auth           auth.Service
	Catalog        catalog.Service
	VersionChecker *versioncheck.Checker
}
type Args map[string]json.RawMessage

func (a Args) String(k string) string { var v string; _ = json.Unmarshal(a[k], &v); return v }
func (a Args) Int(k string) int64     { var v int64; _ = json.Unmarshal(a[k], &v); return v }
func (a Args) Bool(k string) bool     { var v bool; _ = json.Unmarshal(a[k], &v); return v }
func (a Args) Patch() (catalog.Patch, error) {
	var p catalog.Patch
	err := json.Unmarshal(a["patch"], &p)
	return p, err
}
func (a Args) Object() (catalog.Object, error) {
	var o catalog.Object
	err := json.Unmarshal(a["object"], &o)
	return o, err
}

var Tools = []string{"search", "get_asset", "get_service", "list_by_host", "list_by_network", "get_site", "topology", "list_outdated", "get_changes", "get_context", "put_context", "record_note", "propose_change", "update", "mark_verified", "check_versions"}

func (e Engine) Call(ctx context.Context, p auth.Principal, op string, a Args, via string) (any, error) {
	q := catalog.Query{Service: e.Catalog}
	id := a.String("id")
	switch op {
	case "search":
		return q.Search(ctx, p, a.String("q"))
	case "get_asset":
		return e.Catalog.Get(ctx, p, id)
	case "get_service":
		return q.ServiceDetail(ctx, p, id)
	case "list_by_host":
		return q.Inventory(ctx, p, "host", id)
	case "list_by_network":
		return q.Inventory(ctx, p, "network", id)
	case "get_site":
		return q.Site(ctx, p, id)
	case "topology":
		depth := int(a.Int("depth"))
		if len(a["depth"]) == 0 {
			depth = 2
		}
		return q.Topology(ctx, p, id, depth)
	case "list_outdated":
		return q.Outdated(ctx, p)
	case "list_unverified":
		return q.Unverified(ctx, p)
	case "browse":
		return q.Browse(ctx, p, a.String("type"))
	case "get_changes":
		return q.Changes(ctx, p, a.String("object_id"), a.String("state"))
	case "get_context":
		return q.Contexts(ctx, p, a.Bool("archived"))
	case "put_context":
		if a.Int("rev") == 0 {
			o, err := a.Object()
			if err != nil {
				return nil, catalog.ErrInvalid
			}
			if o.ID == "" {
				o.ID = id
			}
			return e.Catalog.CreateContext(ctx, p, o, a.String("reason"), via)
		}
		expiry := time.Time{}
		if x := a.String("expires_at"); x != "" {
			var err error
			expiry, err = time.Parse(time.RFC3339, x)
			if err != nil {
				return nil, catalog.ErrInvalid
			}
		}
		if err := e.Catalog.PutContext(ctx, p, id, a.Int("rev"), a.String("body_md"), expiry, a.String("reason"), via); err != nil {
			return nil, err
		}
		return e.Catalog.Get(ctx, p, id)
	case "record_note":
		noteID, err := e.Catalog.RecordNote(ctx, p, id, a.Int("rev"), a.String("kind"), a.String("body_md"), a.String("reason"), via)
		return map[string]string{"id": noteID}, err
	case "propose_change":
		if a.String("action") == "delete" {
			changeID, err := e.Catalog.ProposeDelete(ctx, p, id, a.Int("rev"), a.String("reason"), via)
			return map[string]string{"id": changeID}, err
		}
		patch, err := a.Patch()
		if err != nil {
			return nil, catalog.ErrInvalid
		}
		changeID, err := e.Catalog.Propose(ctx, p, id, a.Int("rev"), patch, a.String("reason"), via)
		return map[string]string{"id": changeID}, err
	case "update":
		patch, err := a.Patch()
		if err != nil {
			return nil, catalog.ErrInvalid
		}
		return e.Catalog.Update(ctx, p, id, a.Int("rev"), patch, a.String("reason"), via)
	case "mark_verified":
		return e.Catalog.MarkVerified(ctx, p, id, a.Int("rev"), a.String("reason"), via)
	case "check_versions":
		if err := e.Catalog.CheckVersionsAllowed(p); err != nil {
			return nil, err
		}
		checker := e.VersionChecker
		if checker == nil {
			checker = &versioncheck.Checker{DB: e.Catalog.Store.DB}
		}
		return checker.Run(ctx, false)
	}
	return nil, catalog.ErrInvalid
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func Error(err error) (int, APIError) {
	switch {
	case errors.Is(err, auth.ErrUnauthorized):
		return 401, APIError{"unauthorized", "Authentication required", nil}
	case errors.Is(err, auth.ErrForbidden):
		return 403, APIError{"forbidden", "Permission denied", nil}
	case errors.Is(err, catalog.ErrConflict):
		return 409, APIError{"conflict", "Revision conflict", nil}
	case errors.Is(err, versioncheck.ErrRunning):
		return 409, APIError{"conflict", "Version check already running", nil}
	case errors.Is(err, catalog.ErrInvalid), errors.Is(err, auth.ErrInvalid), errors.Is(err, backup.ErrInvalid), strings.Contains(err.Error(), "unsupported schema_version"):
		return 422, APIError{"invalid", "Invalid input", nil}
	case errors.Is(err, secrets.ErrSecret):
		return 422, APIError{"invalid", "Content rejected", nil}
	case strings.Contains(err.Error(), "constraint failed"), strings.Contains(err.Error(), "invalid JSON"), strings.Contains(err.Error(), "unknown "), strings.Contains(err.Error(), "required"), strings.Contains(err.Error(), "must "), strings.Contains(err.Error(), "reference"), strings.Contains(err.Error(), "cycle"):
		return 422, APIError{"invalid", "Invalid input", nil}
	case strings.Contains(err.Error(), "depth must"), strings.Contains(err.Error(), "center must"):
		return 422, APIError{"invalid", "Invalid input", nil}
	case strings.Contains(err.Error(), "sql: no rows"):
		return 404, APIError{"not_found", "Not found", nil}
	default:
		return 500, APIError{"internal", "Internal error", nil}
	}
}

func StringArg(v string) json.RawMessage { b, _ := json.Marshal(v); return b }
func IntArg(v string) json.RawMessage {
	if _, err := strconv.ParseInt(v, 10, 64); err != nil {
		return nil
	}
	return json.RawMessage(v)
}
