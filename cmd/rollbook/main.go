package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/CosmoGao/rollbook/internal/auth"
	"github.com/CosmoGao/rollbook/internal/catalog"
	"github.com/CosmoGao/rollbook/internal/db"
	"github.com/CosmoGao/rollbook/internal/httpapi"
	"github.com/CosmoGao/rollbook/internal/mcp"
	"github.com/CosmoGao/rollbook/web"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: rollbook serve|search|get|update")
		os.Exit(2)
	}
	var err error
	if os.Args[1] == "serve" {
		err = serve()
	} else {
		err = runCLI(os.Args[1:])
	}
	if err != nil {
		log.Fatal(err)
	}
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
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		body, err := web.Files.ReadFile("index.html")
		if err != nil {
			http.Error(w, "page unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(body)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 占位页不缓存，后续页面部署即可直接生效。
		w.Header().Set("Cache-Control", "no-store")
		mux.ServeHTTP(w, r)
	})
}
