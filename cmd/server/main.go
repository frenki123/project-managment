package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	assets "cad-development"
	"cad-development/internal/db"
	"cad-development/internal/handlers"
	"cad-development/internal/web"
)

// version is set at build time with -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	level := slog.LevelInfo
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

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
			slog.Error("get working directory", "err", err)
			os.Exit(1)
		}
	}

	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		slog.Error("create data directory", "err", err)
		os.Exit(1)
	}

	dbPath := filepath.Join(dataDir, "app.db")
	migrations, err := fs.Sub(assets.FS, "sql/migrations")
	if err != nil {
		slog.Error("load migrations", "err", err)
		os.Exit(1)
	}
	staticFS, err := fs.Sub(assets.FS, "static")
	if err != nil {
		slog.Error("load static files", "err", err)
		os.Exit(1)
	}
	database, err := db.OpenDatabase(db.DatabaseConfig{
		DSN:          dbPath + "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_txlock=immediate",
		Migrations:   migrations,
		MaxOpenConns: 4,
	})
	if err != nil {
		slog.Error("open database", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		slog.Error("invalid port", "port", port)
		os.Exit(1)
	}
	addr := "127.0.0.1:" + port

	mux := http.NewServeMux()
	web.RegisterStatic(mux, staticFS)
	handlers.Register(mux, database.Q)

	slog.Info("listening", "addr", addr)
	server := &http.Server{
		Addr:                addr,
		Handler:             web.Recover(mux),
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
			slog.Error("server error", "err", err)
		}
	case <-stop.Done():
		slog.Info("shutdown requested", "cause", context.Cause(stop))
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("server shutdown", "err", err)
		}
		if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "err", err)
		}
	}
}
