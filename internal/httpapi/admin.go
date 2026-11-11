package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/catalog"
)

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && u.Host == r.Host && (u.Scheme == "http" || u.Scheme == "https")
}
func decodeBody(w http.ResponseWriter, r *http.Request, v any) error {
	body := http.MaxBytesReader(w, r.Body, 1<<20)
	defer body.Close()
	d := json.NewDecoder(body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return catalog.ErrInvalid
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return catalog.ErrInvalid
	}
	return nil
}
func csrfCookie(value string, maxAge int, allowInsecureHTTP bool) *http.Cookie {
	return &http.Cookie{Name: "rollbook_csrf", Value: value, Path: "/", Secure: !allowInsecureHTTP, SameSite: http.SameSiteStrictMode, MaxAge: maxAge}
}
func (e Engine) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteJSON(w, 405, APIError{Code: "method_not_allowed", Message: "Method not allowed"})
		return
	}
	if !sameOrigin(r) {
		WriteError(w, auth.ErrForbidden)
		return
	}
	var input struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if err := decodeBody(w, r, &input); err != nil {
		WriteError(w, err)
		return
	}
	session, csrf, p, err := e.Auth.Login(r.Context(), input.Login, input.Password)
	if err != nil {
		WriteError(w, err)
		return
	}
	http.SetCookie(w, auth.SessionCookie(session, e.AllowInsecureHTTP))
	http.SetCookie(w, csrfCookie(csrf, 12*3600, e.AllowInsecureHTTP))
	WriteJSON(w, 200, map[string]string{"login": p.Login})
}
func (e Engine) admin(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if !sameOrigin(r) {
		WriteError(w, auth.ErrForbidden)
		return
	}
	path := r.URL.Path
	switch {
	case path == "/api/auth/session" && r.Method == http.MethodGet:
		WriteJSON(w, 200, map[string]string{"login": p.Login})
		return
	case path == "/api/auth/logout" && r.Method == http.MethodPost:
		cookie, err := r.Cookie("rollbook_session")
		if err != nil {
			WriteError(w, auth.ErrUnauthorized)
			return
		}
		if err = e.Auth.Logout(r.Context(), cookie.Value); err != nil {
			WriteError(w, err)
			return
		}
		http.SetCookie(w, auth.ClearSessionCookie(e.AllowInsecureHTTP))
		http.SetCookie(w, csrfCookie("", -1, e.AllowInsecureHTTP))
		WriteJSON(w, 200, map[string]bool{"ok": true})
		return
	case path == "/api/keys" && r.Method == http.MethodGet:
		keys, err := e.Auth.Keys(r.Context(), p)
		if err != nil {
			WriteError(w, err)
			return
		}
		WriteJSON(w, 200, keys)
		return
	case path == "/api/keys" && r.Method == http.MethodPost:
		var input struct {
			Name      string     `json:"name"`
			Level     auth.Level `json:"level"`
			ExpiresAt string     `json:"expires_at"`
			Note      string     `json:"note"`
		}
		if err := decodeBody(w, r, &input); err != nil {
			WriteError(w, err)
			return
		}
		var expiry *time.Time
		if input.ExpiresAt != "" {
			t, err := time.Parse(time.RFC3339, input.ExpiresAt)
			if err != nil {
				WriteError(w, auth.ErrInvalid)
				return
			}
			expiry = &t
		}
		key, token, err := e.Auth.IssueKey(r.Context(), p, input.Name, input.Level, expiry, input.Note)
		if err != nil {
			WriteError(w, err)
			return
		}
		WriteJSON(w, 201, map[string]any{"key": key, "token": token})
		return
	case strings.HasPrefix(path, "/api/keys/"):
		parts := strings.Split(strings.TrimPrefix(path, "/api/keys/"), "/")
		if len(parts) != 2 || parts[0] == "" {
			break
		}
		var err error
		switch {
		case parts[1] == "level" && r.Method == http.MethodPatch:
			var input struct {
				Level auth.Level `json:"level"`
			}
			if err = decodeBody(w, r, &input); err == nil {
				err = e.Auth.SetLevel(r.Context(), p, parts[0], input.Level)
			}
		case parts[1] == "revoke" && r.Method == http.MethodPost:
			err = e.Auth.Revoke(r.Context(), p, parts[0])
		default:
			WriteJSON(w, 404, APIError{Code: "not_found", Message: "Not found"})
			return
		}
		if err != nil {
			WriteError(w, err)
			return
		}
		WriteJSON(w, 200, map[string]bool{"ok": true})
		return
	case strings.HasPrefix(path, "/api/changes/"):
		parts := strings.Split(strings.TrimPrefix(path, "/api/changes/"), "/")
		if len(parts) != 2 || parts[0] == "" || r.Method != http.MethodPost {
			break
		}
		if parts[1] != "approve" && parts[1] != "reject" {
			break
		}
		result, err := e.Catalog.Review(r.Context(), p, parts[0], parts[1] == "approve")
		if err != nil {
			WriteError(w, err)
			return
		}
		WriteJSON(w, 200, result)
		return
	}
	WriteJSON(w, 404, APIError{Code: "not_found", Message: "Not found"})
}
