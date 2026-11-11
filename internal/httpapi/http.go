package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/catalog"
)

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func WriteError(w http.ResponseWriter, err error) { status, v := Error(err); WriteJSON(w, status, v) }

func (e Engine) Principal(r *http.Request) (auth.Principal, error) {
	h := r.Header.Get("Authorization")
	if h != "" {
		parts := strings.Fields(h)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return auth.Principal{}, auth.ErrUnauthorized
		}
		return e.Auth.Authenticate(r.Context(), parts[1])
	}
	cookie, err := r.Cookie("rollbook_session")
	if err != nil {
		return auth.Principal{}, auth.ErrUnauthorized
	}
	return e.Auth.Session(r.Context(), cookie.Value, r.Header.Get("X-CSRF-Token"), r.Method != "GET" && r.Method != "HEAD")
}

func (e Engine) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/auth/login" {
			e.login(w, r)
			return
		}
		p, err := e.Principal(r)
		if err != nil {
			WriteError(w, err)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/backup/") {
			e.backup(w, r, p)
			return
		}
		if adminPath(r.URL.Path) || (r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/assets/")) {
			if !p.IsUser() {
				WriteError(w, auth.ErrForbidden)
				return
			}
			e.admin(w, r, p)
			return
		}
		op, a, ok := route(r)
		if !ok {
			WriteJSON(w, 404, APIError{Code: "not_found", Message: "Not found"})
			return
		}
		if r.Method != "GET" {
			body := http.MaxBytesReader(w, r.Body, 1<<20)
			defer body.Close()
			dec := json.NewDecoder(body)
			dec.DisallowUnknownFields()
			var bodyArgs Args
			if err = dec.Decode(&bodyArgs); err != nil {
				WriteError(w, catalog.ErrInvalid)
				return
			}
			for k, v := range bodyArgs {
				if k != "id" || len(a["id"]) == 0 {
					a[k] = v
				}
			}
			var extra any
			if err = dec.Decode(&extra); !errors.Is(err, io.EOF) {
				WriteError(w, catalog.ErrInvalid)
				return
			}
		}
		result, err := e.Call(r.Context(), p, op, a, "rest")
		if err != nil {
			WriteError(w, err)
			return
		}
		WriteJSON(w, 200, result)
	})
}

func adminPath(path string) bool {
	for _, prefix := range []string{"/api/keys", "/api/auth/", "/api/backup/"} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	if strings.HasPrefix(path, "/api/changes/") && path != "/api/changes/proposals" {
		return true
	}
	return false
}
func route(r *http.Request) (string, Args, bool) {
	path := strings.Trim(r.URL.Path, "/")
	s := strings.Split(path, "/")
	a := Args{}
	if len(s) < 2 || s[0] != "api" {
		return "", nil, false
	}
	q := r.URL.Query()
	for _, key := range []string{"q", "object_id", "state", "type"} {
		if v := q.Get(key); v != "" {
			a[key] = StringArg(v)
		}
	}
	if v := q.Get("archived"); v == "true" {
		a["archived"] = json.RawMessage("true")
	}
	if v := q.Get("depth"); v != "" {
		a["depth"] = IntArg(v)
	}
	if r.Method == "GET" {
		switch {
		case path == "api/search":
			return "search", a, true
		case path == "api/browse":
			return "browse", a, true
		case path == "api/versions/outdated":
			return "list_outdated", a, true
		case path == "api/assets/unverified":
			return "list_unverified", a, true
		case path == "api/changes":
			return "get_changes", a, true
		case path == "api/contexts":
			return "get_context", a, true
		case len(s) == 3 && s[1] == "assets":
			a["id"] = StringArg(s[2])
			return "get_asset", a, true
		case len(s) == 3 && s[1] == "services":
			a["id"] = StringArg(s[2])
			return "get_service", a, true
		case len(s) == 4 && s[1] == "services" && s[3] == "topology":
			a["id"] = StringArg(s[2])
			return "topology", a, true
		case len(s) == 4 && s[1] == "hosts" && s[3] == "inventory":
			a["id"] = StringArg(s[2])
			return "list_by_host", a, true
		case len(s) == 4 && s[1] == "networks" && s[3] == "inventory":
			a["id"] = StringArg(s[2])
			return "list_by_network", a, true
		case len(s) == 3 && s[1] == "sites":
			a["id"] = StringArg(s[2])
			return "get_site", a, true
		}
	}
	switch {
	case r.Method == "PUT" && len(s) == 3 && s[1] == "contexts":
		a["id"] = StringArg(s[2])
		return "put_context", a, true
	case r.Method == "POST" && path == "api/notes":
		return "record_note", a, true
	case r.Method == "POST" && path == "api/changes/proposals":
		return "propose_change", a, true
	case r.Method == "PATCH" && len(s) == 3 && s[1] == "assets":
		a["id"] = StringArg(s[2])
		return "update", a, true
	case r.Method == "POST" && len(s) == 4 && s[1] == "assets" && s[3] == "verify":
		a["id"] = StringArg(s[2])
		return "mark_verified", a, true
	case r.Method == "POST" && path == "api/versions/check":
		return "check_versions", a, true
	}
	return "", nil, false
}
