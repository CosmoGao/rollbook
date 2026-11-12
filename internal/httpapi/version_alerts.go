package httpapi

import (
	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/versioncheck"
	"net/http"
)

func (e Engine) versionAlerts(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	if !p.IsUser() {
		WriteError(w, auth.ErrForbidden)
		return
	}
	db := e.Catalog.Store.DB
	switch r.Method {
	case http.MethodGet:
		settings, err := versioncheck.GetAlertSettings(r.Context(), db)
		if err != nil {
			WriteError(w, err)
			return
		}
		WriteJSON(w, 200, settings)
	case http.MethodPut:
		if !sameOrigin(r) {
			WriteError(w, auth.ErrForbidden)
			return
		}
		var settings versioncheck.AlertSettings
		if err := decodeBody(w, r, &settings); err != nil {
			WriteError(w, err)
			return
		}
		if (settings.Format != "feishu" && settings.Format != "wecom") || !versioncheck.ValidWebhookURL(settings.WebhookURL) || (settings.Enabled && settings.WebhookURL == "") {
			WriteJSON(w, 422, APIError{Code: "invalid", Message: "请填写有效的 Webhook HTTP/HTTPS 地址"})
			return
		}
		if err := versioncheck.SaveAlertSettings(r.Context(), db, settings); err != nil {
			WriteError(w, err)
			return
		}
		WriteJSON(w, 200, settings)
	default:
		WriteJSON(w, 405, APIError{Code: "method_not_allowed", Message: "Method not allowed"})
	}
}
