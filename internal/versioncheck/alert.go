package versioncheck

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type AlertSettings struct {
	Enabled    bool   `json:"enabled"`
	WebhookURL string `json:"webhook_url"`
	Format     string `json:"format"`
}

func ValidWebhookURL(value string) bool {
	if value == "" {
		return true
	}
	u, err := url.Parse(value)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.User == nil && u.Fragment == "" && !strings.ContainsAny(value, "\r\n\t")
}
func GetAlertSettings(ctx context.Context, db *sql.DB) (AlertSettings, error) {
	var v AlertSettings
	var enabled int
	err := db.QueryRowContext(ctx, `SELECT enabled,webhook_url,format FROM version_alert_settings WHERE id=1`).Scan(&enabled, &v.WebhookURL, &v.Format)
	v.Enabled = enabled == 1
	return v, err
}
func SaveAlertSettings(ctx context.Context, db *sql.DB, v AlertSettings) error {
	if v.Format == "" {
		v.Format = "feishu"
	}
	if (v.Format != "feishu" && v.Format != "wecom") || !ValidWebhookURL(v.WebhookURL) || (v.Enabled && v.WebhookURL == "") {
		return errors.New("invalid webhook URL")
	}
	enabled := 0
	if v.Enabled {
		enabled = 1
	}
	_, err := db.ExecContext(ctx, `UPDATE version_alert_settings SET enabled=?,webhook_url=?,format=? WHERE id=1`, enabled, v.WebhookURL, v.Format)
	return err
}

// DailyAlert stores one global context per UTC day, then sends at most three attempts per day.
func (c Checker) DailyAlert(ctx context.Context) error {
	today := c.now().Format("2006-01-02")
	rows, err := c.DB.QueryContext(ctx, `SELECT o.name,v.current_version,v.latest_version,v.pinned,v.eol_date FROM versions v JOIN services s ON s.id=v.service_id JOIN objects o ON o.id=s.id WHERE s.upstream_json IS NOT NULL AND s.upstream_json<>'' ORDER BY o.name`)
	if err != nil {
		return err
	}
	var lines []string
	for rows.Next() {
		var name string
		var current, latest, eol sql.NullString
		var pinned int
		if err = rows.Scan(&name, &current, &latest, &pinned, &eol); err != nil {
			break
		}
		state := State(current.String, latest.String, pinned, eol.String, c.now())
		if state == "有新版本" || state == "90 天内停止支持" {
			lines = append(lines, fmt.Sprintf("- %s：%s（当前 %s；上游 %s）", name, state, current.String, latest.String))
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return err
	}
	body := "版本提醒（" + today + "）\n"
	if len(lines) == 0 {
		body += "今天没有新版本或临近停止支持的服务。"
	} else {
		if len(lines) > 20 {
			body += strings.Join(lines[:20], "\n") + fmt.Sprintf("\n- 另有 %d 项，请在清册中查看", len(lines)-20)
		} else {
			body += strings.Join(lines, "\n")
		}
	}
	id := "context:version-alert:" + today
	now := c.now().Format(time.RFC3339Nano)
	_, err = c.DB.ExecContext(ctx, `INSERT OR IGNORE INTO objects(id,type,name,source,created_at,created_by,updated_at,updated_by) VALUES(?,'context',?,'自动发现',?,'versioncheck',?,'versioncheck')`, id, "版本提醒 "+today, now, now)
	if err != nil {
		return err
	}
	_, err = c.DB.ExecContext(ctx, `INSERT INTO contexts(id,scope,key,body_md,owner,related_ids_json,expires_at) VALUES(?,'全局',?, ?,'versioncheck','[]',?) ON CONFLICT(id) DO UPDATE SET body_md=excluded.body_md,expires_at=excluded.expires_at,archived_at=NULL`, id, "version-alert:"+today, body, c.now().AddDate(0, 0, 7).Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	settings, err := GetAlertSettings(ctx, c.DB)
	if err != nil {
		return err
	}
	if !settings.Enabled || settings.WebhookURL == "" || len(lines) == 0 {
		return nil
	}
	var attempts int
	var next, sent sql.NullString
	err = c.DB.QueryRowContext(ctx, `SELECT attempts,next_attempt_at,sent_at FROM version_alert_delivery WHERE day=?`, today).Scan(&attempts, &next, &sent)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if sent.Valid || attempts >= 3 {
		return nil
	}
	if next.Valid {
		t, e := time.Parse(time.RFC3339Nano, next.String)
		if e == nil && c.now().Before(t) {
			return nil
		}
	}
	_, err = c.DB.ExecContext(ctx, `INSERT INTO version_alert_delivery(day,attempts,next_attempt_at) VALUES(?,1,?) ON CONFLICT(day) DO UPDATE SET attempts=attempts+1,next_attempt_at=excluded.next_attempt_at`, today, c.now().Add(time.Hour).Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	var message any = map[string]any{"msg_type": "text", "content": map[string]string{"text": body}}
	if settings.Format == "wecom" {
		message = map[string]any{"msgtype": "text", "text": map[string]string{"content": body}}
	}
	payload, _ := json.Marshal(message)
	requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, settings.WebhookURL, bytes.NewReader(payload))
	if err == nil {
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect refused") }}
		var response *http.Response
		response, err = client.Do(req)
		if err == nil {
			data, readErr := io.ReadAll(io.LimitReader(response.Body, 4097))
			response.Body.Close()
			err = readErr
			if len(data) > 4096 {
				err = errors.New("webhook response too large")
			}
			if err == nil && response.StatusCode/100 != 2 {
				err = fmt.Errorf("HTTP %d", response.StatusCode)
			}
			if err == nil && len(data) > 0 {
				var result map[string]json.RawMessage
				if json.Unmarshal(data, &result) == nil {
					for _, key := range []string{"code", "errcode"} {
						if raw, ok := result[key]; ok {
							var code int
							if json.Unmarshal(raw, &code) == nil && code != 0 {
								err = errors.New("webhook rejected message")
							}
						}
					}
				}
			}

		}
	}
	if err != nil {
		_, e := c.DB.ExecContext(ctx, `UPDATE version_alert_delivery SET last_error=? WHERE day=?`, "webhook delivery failed", today)
		return e
	}
	_, err = c.DB.ExecContext(ctx, `UPDATE version_alert_delivery SET sent_at=?,last_error=NULL WHERE day=?`, c.now().Format(time.RFC3339Nano), today)
	return err
}
