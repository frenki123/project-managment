package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"cad-development/internal/db"
	"cad-development/internal/handlers"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
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
	conn, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	if err := conn.Ping(); err != nil {
		log.Fatal(err)
	}

	if err := goose.SetDialect("sqlite3"); err != nil {
		log.Fatal(err)
	}
	if err := goose.Up(conn, filepath.Join(root, "sql", "migrations")); err != nil {
		log.Fatal(err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := "127.0.0.1:" + port

	mux := http.NewServeMux()
	handlers.Register(mux, db.New(conn), root)

	log.Printf("listening on http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
