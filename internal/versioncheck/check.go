// Package versioncheck fetches upstream release metadata without changing inventory state.
package versioncheck

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const interval = 24 * time.Hour

var identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
var numeric = regexp.MustCompile(`^v?([0-9]+(?:\.[0-9]+){0,3})$`)

type Checker struct {
	DB                           *sql.DB
	Client                       *http.Client
	GitHubURL, DockerURL, EOLURL string // Test servers may replace the fixed public API origins.
	Now                          func() time.Time
}
type Config struct {
	Provider   string `json:"provider"`
	Repository string `json:"repository"`
	Image      string `json:"image"`
	Product    string `json:"product"`
	TagPrefix  string `json:"tag_prefix,omitempty"`
	TagRegex   string `json:"tag_regex,omitempty"`
}
type Result struct {
	ServiceID string `json:"service_id"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}
type Summary struct {
	Checked int      `json:"checked"`
	Failed  int      `json:"failed"`
	Skipped int      `json:"skipped"`
	Results []Result `json:"results"`
}

var ErrRunning = errors.New("version check already running")

type release struct{ Version, Source, URL, EOL, RawTag string }

func State(current, latest string, pinned, eol any, now time.Time) string {
	if pinned == int64(1) || pinned == int(1) || pinned == float64(1) || pinned == true {
		return "已锁定"
	}
	comparison, comparable := Compare(current, latest)
	if comparable && comparison > 0 {
		return "上游数据可疑，需人工确认"
	}
	if date, ok := eol.(string); ok && date != "" {
		if t, err := time.Parse("2006-01-02", date); err == nil && !t.After(now.AddDate(0, 0, 90)) {
			return "90 天内停止支持"
		}
	}
	if !comparable {
		return "无法判断"
	}
	if comparison < 0 {
		return "有新版本"
	}
	return "已是最新"
}

func ValidateConfig(cfg Config) error {
	_, err := tagMatcher(cfg)
	return err
}

func tagVersion(tag string, cfg Config) string {
	if _, ok := partsVersion(tag); ok {
		return tag
	}
	if cfg.TagRegex != "" {
		re, err := regexp.Compile(cfg.TagRegex)
		if err == nil {
			m := re.FindStringSubmatch(tag)
			if len(m) > 1 {
				if _, ok := partsVersion(m[1]); ok {
					return m[1]
				}
			}
		}
	}
	return ""
}

func tagMatcher(cfg Config) (func(string) bool, error) {
	if len(cfg.TagPrefix) > 80 || len(cfg.TagRegex) > 160 {
		return nil, errors.New("tag filter too long")
	}
	var re *regexp.Regexp
	if cfg.TagRegex != "" {
		var err error
		re, err = regexp.Compile(cfg.TagRegex)
		if err != nil {
			return nil, errors.New("invalid tag regex")
		}
		if re.MatchString("") || re.MatchString("latest") {
			return nil, errors.New("tag regex is too broad")
		}
	}
	return func(tag string) bool {
		return strings.HasPrefix(tag, cfg.TagPrefix) && (re == nil || re.MatchString(tag))
	}, nil
}

func (c Checker) now() time.Time {
	if c.Now != nil {
		return c.Now().UTC()
	}
	return time.Now().UTC()
}
func (c Checker) client() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	return &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("upstream redirect refused") }}
}
func origin(v, fallback string) string {
	if v != "" {
		return strings.TrimRight(v, "/")
	}
	return fallback
}
func pathParts(value string, count int) ([]string, error) {
	parts := strings.Split(value, "/")
	if len(parts) != count {
		return nil, errors.New("invalid upstream identifier")
	}
	for _, p := range parts {
		if !identifier.MatchString(p) || p == "." || p == ".." {
			return nil, errors.New("invalid upstream identifier")
		}
	}
	return parts, nil
}
func (c Checker) fetch(ctx context.Context, endpoint string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Rollbook/1")
	res, err := c.client().Do(req)
	if err != nil {
		return fmt.Errorf("upstream request: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusTooManyRequests {
		return errors.New("upstream rate limited (429)")
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("upstream HTTP %d", res.StatusCode)
	}
	dec := json.NewDecoder(io.LimitReader(res.Body, 1<<20))
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("upstream JSON: %w", err)
	}
	return nil
}
func (c Checker) lookup(ctx context.Context, cfg Config, current string) (release, error) {
	match, err := tagMatcher(cfg)
	if err != nil {
		return release{}, err
	}
	switch cfg.Provider {
	case "github_releases":
		parts, err := pathParts(cfg.Repository, 2)
		if err != nil {
			return release{}, err
		}
		type githubRelease struct {
			Tag        string `json:"tag_name"`
			URL        string `json:"html_url"`
			Draft      bool   `json:"draft"`
			Prerelease bool   `json:"prerelease"`
		}
		base := origin(c.GitHubURL, "https://api.github.com") + "/repos/" + parts[0] + "/" + parts[1] + "/releases"
		var best githubRelease
		if cfg.TagPrefix == "" && cfg.TagRegex == "" {
			if err := c.fetch(ctx, base+"/latest", &best); err != nil {
				return release{}, err
			}
		} else {
			var releases []githubRelease
			if err := c.fetch(ctx, base+"?per_page=100", &releases); err != nil {
				return release{}, err
			}
			for _, candidate := range releases {
				if candidate.Draft || candidate.Prerelease || !match(candidate.Tag) {
					continue
				}
				if _, ok := partsVersion(candidate.Tag); !ok {
					continue
				}
				if best.Tag == "" {
					best = candidate
				} else if cmp, ok := Compare(best.Tag, candidate.Tag); ok && cmp < 0 {
					best = candidate
				}
			}
		}
		if best.Tag == "" || best.Draft || best.Prerelease || !match(best.Tag) {
			return release{}, errors.New("no stable GitHub release matches tag filter")
		}
		if best.URL == "" {
			best.URL = "https://github.com/" + cfg.Repository + "/releases"
		}
		return release{Version: best.Tag, Source: "github_releases", URL: best.URL, RawTag: best.Tag}, nil
	case "docker_hub":
		parts, err := pathParts(cfg.Image, 2)
		if err != nil {
			return release{}, err
		}
		var v struct {
			Results []struct {
				Name string `json:"name"`
			} `json:"results"`
		}
		endpoint := origin(c.DockerURL, "https://hub.docker.com") + "/v2/namespaces/" + parts[0] + "/repositories/" + parts[1] + "/tags?page_size=100"
		if err = c.fetch(ctx, endpoint, &v); err != nil {
			return release{}, err
		}
		best, raw := "", ""
		for _, tag := range v.Results {
			if !match(tag.Name) {
				continue
			}
			version := tagVersion(tag.Name, cfg)
			if version == "" {
				continue
			}
			if best == "" {
				best, raw = version, tag.Name
			} else if cmp, ok := Compare(best, version); ok && cmp < 0 {
				best, raw = version, tag.Name
			}
		}
		if best == "" {
			return release{}, errors.New("no comparable Docker Hub tag")
		}
		return release{Version: best, Source: "docker_hub", URL: "https://hub.docker.com/r/" + cfg.Image + "/tags", RawTag: raw}, nil
	case "endoflife_date":
		parts, err := pathParts(cfg.Product, 1)
		if err != nil {
			return release{}, err
		}
		var v struct {
			Result struct {
				Releases []struct {
					Latest struct {
						Name string `json:"name"`
						Link string `json:"link"`
					} `json:"latest"`
					EOL *string `json:"eolFrom"`
				} `json:"releases"`
			} `json:"result"`
		}
		if err = c.fetch(ctx, origin(c.EOLURL, "https://endoflife.date")+"/api/v1/products/"+parts[0]+"/", &v); err != nil {
			return release{}, err
		}
		best := release{}
		currentParts, _ := partsVersion(current)
		var currentEOL string
		for _, r := range v.Result.Releases {
			releaseParts, valid := partsVersion(r.Latest.Name)
			if !valid || !match(r.Latest.Name) {
				continue
			}
			if len(currentParts) > 0 && len(releaseParts) > 0 && currentParts[0] == releaseParts[0] && (len(currentParts) < 2 || len(releaseParts) < 2 || currentParts[1] == releaseParts[1]) && r.EOL != nil {
				currentEOL = *r.EOL
			}
			if best.Version == "" {
				best.Version = r.Latest.Name
				best.URL = r.Latest.Link
				if r.EOL != nil {
					best.EOL = *r.EOL
				}
				continue
			}
			if cmp, ok := Compare(best.Version, r.Latest.Name); ok && cmp < 0 {
				best.Version = r.Latest.Name
				best.URL = r.Latest.Link
				best.EOL = ""
				if r.EOL != nil {
					best.EOL = *r.EOL
				}
			}
		}
		if best.Version == "" {
			return release{}, errors.New("no comparable endoflife.date release")
		}
		if len(currentParts) > 0 {
			best.EOL = currentEOL
		}
		best.Source = "endoflife_date"
		best.RawTag = best.Version
		if best.URL == "" {
			best.URL = "https://endoflife.date/" + cfg.Product
		}
		return best, nil
	default:
		return release{}, errors.New("unsupported upstream provider")
	}
}
func partsVersion(value string) ([]uint64, bool) {
	m := numeric.FindStringSubmatch(value)
	if m == nil {
		return nil, false
	}
	out := []uint64{}
	for _, part := range strings.Split(m[1], ".") {
		n, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return nil, false
		}
		out = append(out, n)
	}
	return out, true
}

// Compare accepts only plain numeric releases. Unknown formats are never called outdated.
func Compare(current, latest string) (int, bool) {
	a, ok := partsVersion(current)
	if !ok {
		return 0, false
	}
	b, ok := partsVersion(latest)
	if !ok {
		return 0, false
	}
	for i := 0; i < len(a) || i < len(b); i++ {
		var x, y uint64
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x < y {
			return -1, true
		}
		if x > y {
			return 1, true
		}
	}
	return 0, true
}
func (c Checker) record(ctx context.Context, serviceID string, value release, failure string) error {
	tx, err := c.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := c.now().Format(time.RFC3339Nano)
	if failure == "" {
		r, e := tx.ExecContext(ctx, `UPDATE versions SET latest_version=?,latest_source=?,latest_url=?,eol_date=?,checked_at=?,raw_tag=? WHERE service_id=?`, value.Version, value.Source, value.URL, null(value.EOL), now, value.RawTag, serviceID)
		if e != nil {
			return e
		}
		n, e := r.RowsAffected()
		if e != nil {
			return e
		}
		if n != 1 {
			return errors.New("version record missing")
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO version_checks(service_id,last_attempt_at,last_error) VALUES(?,?,?) ON CONFLICT(service_id) DO UPDATE SET last_attempt_at=excluded.last_attempt_at,last_error=excluded.last_error`, serviceID, now, null(failure))
	if err != nil {
		return err
	}
	return tx.Commit()
}
func null(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func (c Checker) ensureVersion(ctx context.Context, serviceID string) error {
	var count int
	if err := c.DB.QueryRowContext(ctx, `SELECT count(*) FROM versions WHERE service_id=?`, serviceID).Scan(&count); err != nil || count != 0 {
		return err
	}
	tx, err := c.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	id := "version:auto:" + serviceID
	now := c.now().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, `INSERT INTO objects(id,type,name,source,created_at,created_by,updated_at,updated_by) VALUES(?,'version',?,'自动发现',?,'versioncheck',?,'versioncheck')`, id, "版本 "+serviceID, now, now)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO versions(id,service_id) VALUES(?,?)`, id, serviceID)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO search_documents(object_id,name) VALUES(?,?)`, id, "版本 "+serviceID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// Run checks all configured services, or only records older than 24 hours during scheduled runs.
func (c Checker) Run(ctx context.Context, dueOnly bool) (Summary, error) {
	out := Summary{Results: []Result{}}
	if c.DB == nil {
		return out, errors.New("version checker requires database")
	}
	// One SQLite lease coordinates manual and scheduled calls across server processes.
	now := c.now()
	leaseUntil := now.Add(2 * time.Minute)
	lease := leaseUntil.Format(time.RFC3339Nano)
	r, err := c.DB.ExecContext(ctx, `INSERT INTO version_check_lease(id,expires_at) VALUES(1,?) ON CONFLICT(id) DO UPDATE SET expires_at=excluded.expires_at WHERE expires_at<?`, lease, now.Format(time.RFC3339Nano))
	if err != nil {
		return out, err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return out, ErrRunning
	}
	defer func() {
		_, _ = c.DB.ExecContext(context.Background(), `DELETE FROM version_check_lease WHERE id=1 AND expires_at=?`, lease)
	}()
	rows, err := c.DB.QueryContext(ctx, `SELECT s.id,s.upstream_json,COALESCE(v.checked_at,''),COALESCE(vc.last_attempt_at,''),COALESCE(v.current_version,'') FROM services s LEFT JOIN versions v ON v.service_id=s.id LEFT JOIN version_checks vc ON vc.service_id=s.id WHERE s.upstream_json IS NOT NULL AND s.upstream_json<>'' ORDER BY s.id`)
	if err != nil {
		return out, err
	}
	type item struct{ id, raw, checked, attempted, current string }
	items := []item{}
	for rows.Next() {
		var x item
		if err = rows.Scan(&x.id, &x.raw, &x.checked, &x.attempted, &x.current); err != nil {
			break
		}
		items = append(items, x)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return out, err
	}
	for _, x := range items {
		if ctx.Err() != nil {
			return out, ctx.Err()
		}
		if c.now().After(leaseUntil.Add(-30 * time.Second)) {
			next := c.now().Add(2 * time.Minute)
			updated, e := c.DB.ExecContext(ctx, `UPDATE version_check_lease SET expires_at=? WHERE id=1 AND expires_at=?`, next.Format(time.RFC3339Nano), lease)
			if e != nil {
				return out, e
			}
			n, e := updated.RowsAffected()
			if e != nil {
				return out, e
			}
			if n != 1 {
				return out, ErrRunning
			}
			leaseUntil = next
			lease = next.Format(time.RFC3339Nano)
		}
		if err = c.ensureVersion(ctx, x.id); err != nil {
			return out, err
		}
		if dueOnly {
			last := x.checked
			if x.attempted > last {
				last = x.attempted
			}
			if t, e := time.Parse(time.RFC3339Nano, last); e == nil && now.Sub(t) < interval {
				out.Skipped++
				continue
			}
		}
		var cfg Config
		err = json.Unmarshal([]byte(x.raw), &cfg)
		if err == nil {
			var value release
			value, err = c.lookup(ctx, cfg, x.current)
			if err == nil {
				err = c.record(ctx, x.id, value, "")
			}
		}
		result := Result{ServiceID: x.id, Status: "checked"}
		if err != nil {
			result.Status = "failed"
			result.Error = err.Error()
			out.Failed++
			if e := c.record(ctx, x.id, release{}, result.Error); e != nil {
				return out, e
			}
		} else {
			out.Checked++
		}
		out.Results = append(out.Results, result)
	}
	if err := c.DailyAlert(ctx); err != nil {
		return out, err
	}
	return out, nil
}

// Start checks stale services at startup and hourly thereafter. The context ends with the server.
func (c Checker) Start(ctx context.Context) {
	go func() {
		run := func() { _, _ = c.Run(ctx, true) }
		run()
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run()
			}
		}
	}()
}
