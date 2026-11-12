// Package httpapi exposes the catalog through a single authenticated operation dispatcher.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
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
	Auth              auth.Service
	Catalog           catalog.Service
	VersionChecker    *versioncheck.Checker
	AllowInsecureHTTP bool
	TrustedProxyCIDRs []netip.Prefix
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
func (a Args) CreateRev() (int64, error) {
	var rev int64
	if len(a["base_rev"]) == 0 || json.Unmarshal(a["base_rev"], &rev) != nil {
		return 0, &catalog.ValidationError{Fields: []catalog.FieldError{{Field: "base_rev", Reason: "创建时必须提供整数版本", Expected: "0"}}}
	}
	return rev, nil
}

var Tools = []string{"search", "get_asset", "get_service", "list_by_host", "list_by_network", "get_site", "topology", "list_outdated", "get_changes", "get_context", "put_context", "record_note", "propose_change", "create", "update", "mark_verified", "check_versions"}

func (e Engine) Call(ctx context.Context, p auth.Principal, op string, a Args, via string) (any, error) {
	canonicalArgs(op, a)
	q := catalog.Query{Service: e.Catalog}
	id := a.String("id")
	switch op {
	case "create":
		rev, err := a.CreateRev()
		if err != nil {
			return nil, err
		}
		o, err := a.Object()
		if err != nil || len(a["object"]) == 0 {
			return nil, &catalog.ValidationError{Fields: []catalog.FieldError{{Field: "object", Reason: "对象必填且必须是有效 JSON", Expected: "对象"}}}
		}
		if p.Level == auth.Propose && !p.IsUser() {
			changeID, err := e.Catalog.ProposeCreate(ctx, p, o, rev, a.String("reason"), via)
			return map[string]string{"id": changeID, "action": "create", "state": "待确认"}, err
		}
		return e.Catalog.CreateAsset(ctx, p, o, rev, a.String("reason"), via)
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
		if a.String("action") == "create" {
			rev, err := a.CreateRev()
			if err != nil {
				return nil, err
			}
			o, err := a.Object()
			if err != nil || len(a["object"]) == 0 {
				return nil, &catalog.ValidationError{Fields: []catalog.FieldError{{Field: "object", Reason: "对象必填且必须是有效 JSON", Expected: "对象"}}}
			}
			changeID, err := e.Catalog.ProposeCreate(ctx, p, o, rev, a.String("reason"), via)
			return map[string]string{"id": changeID}, err
		}
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
	case "verify_batch":
		var ids []string
		if json.Unmarshal(a["ids"], &ids) != nil {
			return nil, catalog.ErrInvalid
		}
		results, err := e.Catalog.VerifyBatch(ctx, p, ids, a.Bool("with_related"), a.String("reason"), via)
		return map[string]any{"results": results}, err
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
	Code    string               `json:"code"`
	Message string               `json:"message"`
	Details any                  `json:"details,omitempty"`
	Fields  []catalog.FieldError `json:"fields,omitempty"`
}

func Error(err error) (int, APIError) {
	var conflict *catalog.ConflictError
	if errors.As(err, &conflict) {
		return 409, APIError{Code: "conflict", Message: "字段已变更", Fields: conflict.Fields}
	}
	var validation *catalog.ValidationError
	if errors.As(err, &validation) {
		return 422, APIError{Code: "validation", Message: "Validation failed", Fields: validation.Fields}
	}
	switch {
	case errors.Is(err, auth.ErrUnauthorized):
		return 401, APIError{Code: "unauthorized", Message: "Authentication required"}
	case errors.Is(err, auth.ErrForbidden):
		return 403, APIError{Code: "forbidden", Message: "Permission denied"}
	case errors.Is(err, catalog.ErrConflict):
		return 409, APIError{Code: "conflict", Message: "Revision conflict"}
	case errors.Is(err, backup.ErrConflict):
		return 409, APIError{Code: "conflict", Message: err.Error()}
	case errors.Is(err, versioncheck.ErrRunning):
		return 409, APIError{Code: "conflict", Message: "Version check already running"}
	case errors.Is(err, catalog.ErrInvalid), errors.Is(err, auth.ErrInvalid), errors.Is(err, backup.ErrInvalid), strings.Contains(err.Error(), "unsupported schema_version"):
		return 422, APIError{Code: "invalid", Message: "Invalid input"}
	case errors.Is(err, secrets.ErrSecret):
		return 422, APIError{Code: "invalid", Message: "Content rejected"}
	case strings.Contains(err.Error(), "constraint failed"), strings.Contains(err.Error(), "invalid JSON"), strings.Contains(err.Error(), "unknown "), strings.Contains(err.Error(), "required"), strings.Contains(err.Error(), "must "), strings.Contains(err.Error(), "reference"), strings.Contains(err.Error(), "cycle"):
		return 422, APIError{Code: "invalid", Message: "Invalid input"}
	case strings.Contains(err.Error(), "depth must"), strings.Contains(err.Error(), "center must"):
		return 422, APIError{Code: "invalid", Message: "Invalid input"}
	case strings.Contains(err.Error(), "sql: no rows"):
		return 404, APIError{Code: "not_found", Message: "Not found"}
	default:
		return 500, APIError{Code: "internal", Message: "Internal error"}
	}
}

func StringArg(v string) json.RawMessage { b, _ := json.Marshal(v); return b }
func IntArg(v string) json.RawMessage {
	if _, err := strconv.ParseInt(v, 10, 64); err != nil {
		return nil
	}
	return json.RawMessage(v)
}
