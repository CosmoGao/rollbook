package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
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
	} else {
		err = runCLI(os.Args[1:])
	}
	if err != nil {
		log.Fatal(err)
	}
}

func bootstrapUser(args []string) error {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		return fmt.Errorf("usage: rollbook bootstrap-user <login>")
	}
	path := os.Getenv("ROLLBOOK_DB_PATH")
	if path == "" {
		path = "./data/rollbook.db"
	}
	database, err := db.Open(context.Background(), path)
	if err != nil {
		return err
	}
	defer database.Close()
	pepper := os.Getenv("ROLLBOOK_AUTH_PEPPER")
	if len(pepper) < 32 {
		return fmt.Errorf("ROLLBOOK_AUTH_PEPPER must contain at least 32 bytes")
	}
	fmt.Fprint(os.Stderr, "Password (at least 12 characters): ")
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
	_, err = (auth.Service{DB: database, Pepper: []byte(pepper)}).BootstrapUser(context.Background(), args[0], string(password))
	return err
}

func serve() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	path := os.Getenv("ROLLBOOK_DB_PATH")
	if path == "" {
		path = "./data/rollbook.db"
	}
	database, err := db.Open(ctx, path)
	if err != nil {
		return err
	}
	defer database.Close()
	pepper := os.Getenv("ROLLBOOK_AUTH_PEPPER")
	if len(pepper) < 32 {
		return fmt.Errorf("ROLLBOOK_AUTH_PEPPER must contain at least 32 bytes")
	}
	engine := httpapi.Engine{Auth: auth.Service{DB: database, Pepper: []byte(pepper)}, Catalog: catalog.Service{Store: catalog.Store{DB: database}}}
	checker := &versioncheck.Checker{DB: database}
	engine.VersionChecker = checker
	checker.Start(ctx)
	go periodicBackup(ctx, database, filepath.Join(filepath.Dir(path), "rollbook-latest.yaml"))
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

func handler(engines ...httpapi.Engine) http.Handler {
	var engine httpapi.Engine
	if len(engines) > 0 {
		engine = engines[0]
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", engine.Handler())
	mux.Handle("/mcp", mcp.Server{Engine: engine})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte("ok\n"))
	})
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		http.FileServer(http.FS(web.Files)).ServeHTTP(w, r)
	}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 资源和接口均不缓存，避免页面显示失效的会话或旧数据。
		w.Header().Set("Cache-Control", "no-store")
		mux.ServeHTTP(w, r)
	})
}

// periodicBackup persists a private complete snapshot on startup and every day.
func periodicBackup(ctx context.Context, database *sql.DB, path string) {
	run := func() {
		data, err := (backup.Service{DB: database}).Export(ctx, "full")
		if err == nil {
			err = backup.WriteAtomic(path, data)
		}
		if err != nil && ctx.Err() == nil {
			log.Printf("backup failed: %v", err)
		}
	}
	run()
	ticker := time.NewTicker(24 * time.Hour)
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
