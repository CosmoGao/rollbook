package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
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
	ip, _, splitErr := net.SplitHostPort(r.RemoteAddr)
	if splitErr != nil {
		ip = r.RemoteAddr
	}
	if peer, parseErr := netip.ParseAddr(ip); parseErr == nil {
		for _, trusted := range e.TrustedProxyCIDRs {
			if !trusted.Contains(peer) {
				continue
			}
			// 仅采信受信任的单跳代理追加的最后一个地址。
			forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
			if pos := strings.LastIndex(forwarded, ","); pos >= 0 {
				forwarded = strings.TrimSpace(forwarded[pos+1:])
			}
			if client, err := netip.ParseAddr(forwarded); err == nil {
				ip = client.String()
			}
			break
		}
	}
	session, csrf, p, err := e.Auth.LoginWithIP(r.Context(), input.Login, input.Password, ip)
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
	case strings.HasPrefix(path, "/api/hosts/") && strings.HasSuffix(path, "/offline") && r.Method == http.MethodGet:
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/api/hosts/"), "/offline")
		preview, err := e.Catalog.OfflinePreview(r.Context(), p, id)
		if err != nil {
			WriteError(w, err)
			return
		}
		op, err := e.Catalog.LatestOfflineOperation(r.Context(), p, id)
		if err != nil {
			WriteError(w, err)
			return
		}
		WriteJSON(w, 200, map[string]any{"preview": preview, "operation_id": op})
		return
	case strings.HasPrefix(path, "/api/hosts/") && strings.HasSuffix(path, "/offline") && r.Method == http.MethodPost:
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/api/hosts/"), "/offline")
		var input struct {
			Rev     int64                  `json:"rev"`
			Reason  string                 `json:"reason"`
			Options catalog.OfflineOptions `json:"options"`
		}
		if err := decodeBody(w, r, &input); err != nil {
			WriteError(w, err)
			return
		}
		op, err := e.Catalog.OfflineHost(r.Context(), p, id, input.Rev, input.Options, input.Reason)
		if err != nil {
			WriteError(w, err)
			return
		}
		WriteJSON(w, 200, map[string]string{"operation_id": op})
		return
	case strings.HasPrefix(path, "/api/hosts/") && strings.HasSuffix(path, "/restore") && r.Method == http.MethodPost:
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/api/hosts/"), "/restore")
		var input struct {
			OperationID string `json:"operation_id"`
			Reason      string `json:"reason"`
		}
		if err := decodeBody(w, r, &input); err != nil {
			WriteError(w, err)
			return
		}
		result, err := e.Catalog.RestoreHost(r.Context(), p, id, input.OperationID, input.Reason)
		if err != nil {
			WriteError(w, err)
			return
		}
		WriteJSON(w, 200, result)
		return
	case strings.HasPrefix(path, "/api/assets/") && r.Method == http.MethodDelete:
		id := strings.TrimPrefix(path, "/api/assets/")
		var input struct {
			Rev    int64  `json:"rev"`
			Reason string `json:"reason"`
		}
		if err := decodeBody(w, r, &input); err != nil {
			WriteError(w, err)
			return
		}
		if err := e.Catalog.DeleteAsset(r.Context(), p, id, input.Rev, input.Reason); err != nil {
			WriteError(w, err)
			return
		}
		WriteJSON(w, 200, map[string]bool{"ok": true})
		return
	case path == "/api/auth/session" && r.Method == http.MethodGet:
		WriteJSON(w, 200, map[string]string{"login": p.Login})
		return
	case path == "/api/auth/change-password" && r.Method == http.MethodPost:
		var input struct {
			OldPassword string `json:"old_password"`
			NewPassword string `json:"new_password"`
		}
		if err := decodeBody(w, r, &input); err != nil {
			WriteError(w, err)
			return
		}
		if len(input.NewPassword) < 12 {
			WriteJSON(w, 422, APIError{Code: "validation", Message: "Validation failed", Fields: []catalog.FieldError{{Field: "new_password", Reason: "密码至少 12 个字符"}}})
			return
		}
		if err := e.Auth.ChangePassword(r.Context(), p, input.OldPassword, input.NewPassword); err != nil {
			if errors.Is(err, auth.ErrUnauthorized) {
				WriteJSON(w, 422, APIError{Code: "validation", Message: "Validation failed", Fields: []catalog.FieldError{{Field: "old_password", Reason: "当前密码不正确"}}})
			} else {
				WriteError(w, err)
			}
			return
		}
		http.SetCookie(w, auth.ClearSessionCookie(e.AllowInsecureHTTP))
		http.SetCookie(w, csrfCookie("", -1, e.AllowInsecureHTTP))
		WriteJSON(w, 200, map[string]bool{"ok": true})
		return
	case path == "/api/auth/logout-all" && r.Method == http.MethodPost:
		if err := e.Auth.LogoutAll(r.Context(), p); err != nil {
			WriteError(w, err)
			return
		}
		http.SetCookie(w, auth.ClearSessionCookie(e.AllowInsecureHTTP))
		http.SetCookie(w, csrfCookie("", -1, e.AllowInsecureHTTP))
		WriteJSON(w, 200, map[string]bool{"ok": true})
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
