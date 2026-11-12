package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/backup"
	"github.com/CosmoGao/rollbook/internal/catalog"
	"github.com/CosmoGao/rollbook/internal/db"
	"github.com/CosmoGao/rollbook/internal/httpapi"
	"github.com/CosmoGao/rollbook/internal/mcp"
	"github.com/CosmoGao/rollbook/internal/versioncheck"
	"github.com/CosmoGao/rollbook/web"
	"golang.org/x/term"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: rollbook serve|bootstrap-user|search|get|update|export|import")
		os.Exit(2)
	}
	var err error
	if os.Args[1] == "serve" {
		err = serve()
	} else if os.Args[1] == "bootstrap-user" {
		err = bootstrapUser(os.Args[2:])
	} else if os.Args[1] == "reset-password" {
		err = resetPassword(os.Args[2:])
	} else {
		err = runCLI(os.Args[1:])
	}
	if err != nil {
		log.Fatal(err)
	}
}

func resetPassword(args []string) error {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		return fmt.Errorf("usage: rollbook reset-password <login>")
	}
	path := os.Getenv("ROLLBOOK_DB_PATH")
	if path == "" {
		path = "./data/rollbook.db"
	}
	if _, err := os.Stat(path); err != nil {
		return err
	}
	pepper, err := authPepper()
	if err != nil {
		return err
	}
	database, err := db.Open(context.Background(), path)
	if err != nil {
		return err
	}
	defer database.Close()
	fmt.Fprint(os.Stderr, "New password (8+ characters; 3 of uppercase, lowercase, digits, symbols): ")
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return err
	}
	defer func() {
		for i := range password {
			password[i] = 0
		}
	}()
	return (auth.Service{DB: database, Pepper: pepper}).ResetPassword(context.Background(), args[0], string(password))
}

func loginPolicy() auth.LoginPolicy {
	readInt := func(name string) int {
		n, err := strconv.Atoi(os.Getenv(name))
		if err != nil || n <= 0 {
			return 0
		}
		return n
	}
	readDuration := func(name string) time.Duration {
		d, err := time.ParseDuration(os.Getenv(name))
		if err != nil || d <= 0 {
			return 0
		}
		return d
	}
	return auth.LoginPolicy{Threshold: readInt("ROLLBOOK_LOGIN_THRESHOLD"), MaxBackoff: readDuration("ROLLBOOK_LOGIN_MAX_BACKOFF"), LockDuration: readDuration("ROLLBOOK_LOGIN_LOCK_DURATION")}
}

func trustedProxyCIDRs() ([]netip.Prefix, error) {
	var result []netip.Prefix
	for _, part := range strings.Split(os.Getenv("ROLLBOOK_TRUSTED_PROXY_CIDRS"), ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(part)
		if err != nil {
			return nil, fmt.Errorf("ROLLBOOK_TRUSTED_PROXY_CIDRS: %w", err)
		}
		result = append(result, prefix.Masked())
	}
	return result, nil
}

func bootstrapUser(args []string) error {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		return fmt.Errorf("usage: rollbook bootstrap-user <login>")
	}
	path := os.Getenv("ROLLBOOK_DB_PATH")
	if path == "" {
		path = "./data/rollbook.db"
	}
	pepper, err := authPepper()
	if err != nil {
		return err
	}
	database, err := db.Open(context.Background(), path)
	if err != nil {
		return err
	}
	defer database.Close()
	fmt.Fprint(os.Stderr, "Password (8+ characters; 3 of uppercase, lowercase, digits, symbols): ")
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return err
	}
	defer func() {
		for i := range password {
			password[i] = 0
		}
	}()
	_, err = (auth.Service{DB: database, Pepper: pepper}).BootstrapUser(context.Background(), args[0], string(password))
	return err
}

func serve() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	path := os.Getenv("ROLLBOOK_DB_PATH")
	if path == "" {
		path = "./data/rollbook.db"
	}
	pepper, err := authPepper()
	if err != nil {
		return err
	}
	database, err := db.Open(ctx, path)
	if err != nil {
		return err
	}
	defer database.Close()
	if err := bootstrapFromEnvironment(ctx, database, pepper); err != nil {
		return err
	}
	allowInsecureHTTP := insecureHTTPEnabled()
	trustedProxies, err := trustedProxyCIDRs()
	if err != nil {
		return err
	}
	engine := httpapi.Engine{Auth: auth.Service{DB: database, Pepper: pepper, LoginPolicy: loginPolicy()}, Catalog: catalog.Service{Store: catalog.Store{DB: database}}, AllowInsecureHTTP: allowInsecureHTTP, TrustedProxyCIDRs: trustedProxies}
	if allowInsecureHTTP {
		log.Print("警告：已允许通过 HTTP 登录；实际为 HTTP 的请求会显示风险提示，口令和 API Key 可能在网络中明文传输，仅可用于可信内网。")
	}
	checker := &versioncheck.Checker{DB: database}
	engine.VersionChecker = checker
	checker.Start(ctx)
	go periodicBackup(ctx, database, backupPath(path))
	go func() {
		service := backup.Service{DB: database}
		if err := service.CleanupExpired(ctx); err != nil {
			log.Printf("导入过期清理失败: %v", err)
		}
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := service.CleanupExpired(ctx); err != nil {
					log.Printf("导入过期清理失败: %v", err)
				}
			}
		}
	}()
	addr := os.Getenv("ROLLBOOK_LISTEN_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	server := &http.Server{Addr: addr, Handler: handler(engine), ReadHeaderTimeout: 5 * time.Second}
	result := make(chan error, 1)
	go func() { result <- server.ListenAndServe() }()
	log.Printf("Rollbook listening on %s", addr)
	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
}

func bootstrapFromEnvironment(ctx context.Context, database *sql.DB, pepper []byte) error {
	login, hasLogin := os.LookupEnv("ROLLBOOK_BOOTSTRAP_USER")
	password, hasPassword := os.LookupEnv("ROLLBOOK_BOOTSTRAP_PASSWORD")
	passwordFile, hasFile := os.LookupEnv("ROLLBOOK_BOOTSTRAP_PASSWORD_FILE")
	if !hasLogin && !hasPassword && !hasFile {
		return nil
	}
	if hasPassword && hasFile {
		return fmt.Errorf("ROLLBOOK_BOOTSTRAP_PASSWORD and ROLLBOOK_BOOTSTRAP_PASSWORD_FILE cannot both be set")
	}
	if !hasLogin || strings.TrimSpace(login) == "" {
		return fmt.Errorf("ROLLBOOK_BOOTSTRAP_USER must be set when a bootstrap password is configured")
	}
	if !hasPassword && !hasFile {
		return fmt.Errorf("ROLLBOOK_BOOTSTRAP_PASSWORD or ROLLBOOK_BOOTSTRAP_PASSWORD_FILE must be set with ROLLBOOK_BOOTSTRAP_USER")
	}
	if hasFile && passwordFile == "" {
		return fmt.Errorf("ROLLBOOK_BOOTSTRAP_PASSWORD_FILE must name a readable file")
	}

	// 已有用户时不读取口令文件，也不修改任何凭据。最终创建仍由 BootstrapUser 的事务判断。
	var count int
	if err := database.QueryRowContext(ctx, "SELECT count(*) FROM users").Scan(&count); err != nil {
		return fmt.Errorf("check bootstrap users: %w", err)
	}
	if count != 0 {
		log.Print("已存在用户，忽略首次管理员自动引导")
		return nil
	}
	passwordSource := "ROLLBOOK_BOOTSTRAP_PASSWORD"
	if hasFile {
		passwordSource = "ROLLBOOK_BOOTSTRAP_PASSWORD_FILE"
		content, err := os.ReadFile(passwordFile)
		if err != nil {
			var pathErr *os.PathError
			if errors.As(err, &pathErr) {
				err = pathErr.Err
			}
			return fmt.Errorf("read %s: %w", passwordSource, err)
		}
		password = strings.TrimSuffix(string(content), "\n")
		password = strings.TrimSuffix(password, "\r")
	}
	_, err := (auth.Service{DB: database, Pepper: pepper}).BootstrapUser(ctx, login, password)
	if errors.Is(err, auth.ErrForbidden) {
		log.Print("已存在用户，忽略首次管理员自动引导")
		return nil
	}
	if errors.Is(err, auth.ErrInvalid) {
		return fmt.Errorf("invalid %s: %w", passwordSource, err)
	}
	if err != nil {
		return fmt.Errorf("create bootstrap user: %w", err)
	}
	log.Print("首次管理员已创建；首次登录后请移除引导环境变量和口令文件")
	return nil
}

func authPepper() ([]byte, error) {
	value, hasValue := os.LookupEnv("ROLLBOOK_AUTH_PEPPER")
	file, hasFile := os.LookupEnv("ROLLBOOK_AUTH_PEPPER_FILE")
	if hasValue && hasFile {
		return nil, fmt.Errorf("ROLLBOOK_AUTH_PEPPER and ROLLBOOK_AUTH_PEPPER_FILE cannot both be set")
	}
	if hasFile {
		pepper, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read ROLLBOOK_AUTH_PEPPER_FILE: %w", err)
		}
		if len(pepper) < 32 {
			return nil, fmt.Errorf("ROLLBOOK_AUTH_PEPPER_FILE must contain at least 32 bytes")
		}
		return pepper, nil
	}
	if len(value) < 32 {
		return nil, fmt.Errorf("ROLLBOOK_AUTH_PEPPER must contain at least 32 bytes")
	}
	return []byte(value), nil
}

func backupPath(databasePath string) string {
	dir := os.Getenv("ROLLBOOK_BACKUP_DIR")
	if dir == "" {
		dir = filepath.Dir(databasePath)
	}
	return filepath.Join(dir, "rollbook-latest.yaml")
}

func insecureHTTPEnabled() bool {
	return os.Getenv("ROLLBOOK_ALLOW_INSECURE_HTTP") == "true"
}

func handler(engines ...httpapi.Engine) http.Handler {
	var engine httpapi.Engine
	if len(engines) > 0 {
		engine = engines[0]
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api", http.NotFound)
	mux.Handle("/api/", engine.Handler())
	mux.Handle("/mcp", mcp.Server{Engine: engine})
	mux.HandleFunc("/manifest.webmanifest", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/manifest+json; charset=utf-8")
		data, err := web.Files.ReadFile("dist/manifest.webmanifest")
		if err != nil {
			http.Error(w, "manifest unavailable", http.StatusInternalServerError)
			return
		}
		if r.Method == http.MethodGet {
			_, _ = w.Write(data)
		}
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte("ok\n"))
	})
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "api" || strings.HasPrefix(path, "api/") || path == "mcp" || strings.HasPrefix(path, "mcp/") || path == "healthz" || strings.HasPrefix(path, "healthz/") {
			http.NotFound(w, r)
			return
		}
		if path == "" {
			path = "index.html"
		}
		data, err := web.Files.ReadFile("dist/" + path)
		if err != nil {
			if strings.HasPrefix(path, "assets/") || strings.Contains(path, ".") {
				http.NotFound(w, r)
				return
			}
			path = "index.html"
			data, err = web.Files.ReadFile("dist/" + path)
		}
		if err != nil {
			http.NotFound(w, r)
			return
		}
		http.ServeContent(w, r, path, time.Time{}, bytes.NewReader(data))
	}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 资源和接口均不缓存，避免页面显示失效的会话或旧数据。
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self'; img-src 'self' data:; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		mux.ServeHTTP(w, r)
	})
}

// periodicBackup 在启动及可配置的周期内保存完整快照。
func periodicBackup(ctx context.Context, database *sql.DB, path string) {
	interval := backupDuration("ROLLBOOK_BACKUP_INTERVAL", 24*time.Hour)
	weeklyInterval := backupDuration("ROLLBOOK_BACKUP_WEEKLY_INTERVAL", 7*24*time.Hour)
	dailyKeep := backupCount("ROLLBOOK_BACKUP_DAILY_KEEP", 7)
	weeklyKeep := backupCount("ROLLBOOK_BACKUP_WEEKLY_KEEP", 4)
	run := func() {
		data, err := (backup.Service{DB: database}).Export(ctx, "full")
		if err == nil {
			err = backup.SaveManaged(filepath.Dir(path), data, time.Now(), dailyKeep, weeklyKeep, weeklyInterval)
		}
		if err != nil && ctx.Err() == nil {
			log.Printf("backup failed: %v", err)
		}
	}
	run()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

func backupDuration(key string, fallback time.Duration) time.Duration {
	v, err := time.ParseDuration(os.Getenv(key))
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}
func backupCount(key string, fallback int) int {
	v, err := strconv.Atoi(os.Getenv(key))
	if err != nil || v < 1 {
		return fallback
	}
	return v
}
