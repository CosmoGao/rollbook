package httpapi

import (
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/backup"
	"github.com/CosmoGao/rollbook/internal/catalog"
)

func (e Engine) backup(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if p.IsUser() && !sameOrigin(r) {
		WriteError(w, auth.ErrForbidden)
		return
	}
	service := backup.Service{DB: e.Auth.DB}
	path := r.URL.Path
	switch {
	case path == "/api/backup/files" && r.Method == http.MethodGet:
		if !p.IsUser() {
			WriteError(w, auth.ErrForbidden)
			return
		}
		dir, err := backup.Directory(r.Context(), e.Auth.DB)
		if err != nil {
			WriteError(w, err)
			return
		}
		files, err := backup.ListManaged(dir)
		if err != nil {
			WriteError(w, err)
			return
		}
		WriteJSON(w, 200, files)
	case strings.HasPrefix(path, "/api/backup/files/"):
		if !p.IsUser() {
			WriteError(w, auth.ErrForbidden)
			return
		}
		remainder := strings.TrimPrefix(path, "/api/backup/files/")
		name, action, ok := strings.Cut(remainder, "/")
		if !ok || strings.Contains(action, "/") {
			WriteError(w, backup.ErrInvalid)
			return
		}
		name, err := url.PathUnescape(name)
		if err != nil {
			WriteError(w, backup.ErrInvalid)
			return
		}
		dir, err := backup.Directory(r.Context(), e.Auth.DB)
		if err != nil {
			WriteError(w, err)
			return
		}
		data, err := backup.ReadManaged(dir, name)
		if err != nil {
			WriteError(w, err)
			return
		}
		if action == "download" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
			w.Header().Set("Content-Disposition", "attachment; filename="+name)
			w.Write(data)
			return
		}
		if action == "restore" && r.Method == http.MethodPost {
			item, err := service.Submit(r.Context(), data, p)
			if err != nil {
				WriteError(w, err)
				return
			}
			WriteJSON(w, 201, item)
			return
		}
		WriteError(w, backup.ErrInvalid)
	case path == "/api/backup/export" && r.Method == http.MethodGet:
		kind := r.URL.Query().Get("kind")
		if kind == "" {
			kind = "catalog"
		}
		if kind != "catalog" && kind != "full" {
			WriteError(w, backup.ErrInvalid)
			return
		}
		if kind == "full" && !p.IsUser() {
			WriteError(w, auth.ErrForbidden)
			return
		}
		data, err := service.Export(r.Context(), kind)
		if err != nil {
			WriteError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=rollbook-"+kind+".yaml")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(data)
	case path == "/api/backup/import" && r.Method == http.MethodPost:
		if !p.IsUser() && p.Level == auth.Read {
			WriteError(w, auth.ErrForbidden)
			return
		}
		data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, backup.MaxSize))
		if err != nil {
			WriteError(w, backup.ErrInvalid)
			return
		}
		f, _, err := backup.Parse(data)
		if err != nil {
			WriteError(w, err)
			return
		}
		if !p.IsUser() && f.Kind != "merge" {
			WriteError(w, auth.ErrForbidden)
			return
		}
		item, err := service.Submit(r.Context(), data, p)
		if err != nil {
			WriteError(w, err)
			return
		}
		WriteJSON(w, 201, item)
	case path == "/api/backup/imports" && r.Method == http.MethodGet:
		if !p.IsUser() {
			WriteError(w, auth.ErrForbidden)
			return
		}
		items, err := service.List(r.Context())
		if err != nil {
			WriteError(w, err)
			return
		}
		WriteJSON(w, 200, items)
	case strings.HasPrefix(path, "/api/backup/imports/") && r.Method == http.MethodPost:
		if !p.IsUser() {
			WriteError(w, auth.ErrForbidden)
			return
		}
		part := strings.TrimPrefix(path, "/api/backup/imports/")
		id, action, ok := strings.Cut(part, "/")
		if !ok || len(id) != 32 || (action != "approve" && action != "reject") {
			WriteError(w, catalog.ErrInvalid)
			return
		}
		err := service.Review(r.Context(), id, action == "approve", p.Login)
		if err != nil {
			WriteError(w, err)
			return
		}
		WriteJSON(w, 200, map[string]bool{"ok": true})
	default:
		WriteJSON(w, 404, APIError{Code: "not_found", Message: "Not found"})
	}
}
