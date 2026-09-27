package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	assets "cad-development"
	"cad-development/internal/app"
	"cad-development/internal/handlers"
)

// version is set at build time with -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}

	root := os.Getenv("DEVENV_ROOT")
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			log.Fatal(err)
		}
	}

	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatal(err)
	}

	dbPath := filepath.Join(dataDir, "app.db")
	migrations, err := fs.Sub(assets.FS, "sql/migrations")
	if err != nil {
		log.Fatal(err)
	}
	staticFS, err := fs.Sub(assets.FS, "static")
	if err != nil {
		log.Fatal(err)
	}
	database, err := app.OpenDatabase(app.DatabaseConfig{
		DSN:          dbPath + "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_txlock=immediate",
		Migrations:   migrations,
		MaxOpenConns: 4,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		log.Fatalf("invalid PORT %q", port)
	}
	addr := "127.0.0.1:" + port

	mux := http.NewServeMux()
	app.RegisterStatic(mux, staticFS)
	handlers.Register(mux, database.Q)

	log.Printf("listening on http://%s", addr)
	server := &http.Server{
		Addr:                addr,
		Handler:             app.Recover(mux),
		ReadTimeout:         30 * time.Second,
		ReadHeaderTimeout:   5 * time.Second,
		IdleTimeout:         60 * time.Second,
		MaxHeaderValueCount: 64,
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.ListenAndServe() }()

	stop, stopSignal := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignal()
	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server error: %v", err)
		}
	case <-stop.Done():
		log.Printf("shutdown requested: %v", context.Cause(stop))
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("server shutdown: %v", err)
		}
		if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server error: %v", err)
		}
	}
}
