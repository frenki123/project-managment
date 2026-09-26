package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"cad-development/internal/app"
	"cad-development/internal/handlers"
)

func main() {
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
	database, err := app.OpenDatabase(app.DatabaseConfig{
		DSN:           dbPath + "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_txlock=immediate",
		MigrationsDir: filepath.Join(root, "sql", "migrations"),
		MaxOpenConns:  4,
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
	app.RegisterStatic(mux, root)
	handlers.Register(mux, database.Q)

	log.Printf("listening on http://%s", addr)
	server := &http.Server{Addr: addr, Handler: mux}
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
