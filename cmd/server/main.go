package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

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
		DSN:           dbPath + "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)",
		MigrationsDir: filepath.Join(root, "sql", "migrations"),
	})
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := "127.0.0.1:" + port

	mux := http.NewServeMux()
	handlers.Register(mux, database.Q, root)

	log.Printf("listening on http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
