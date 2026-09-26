set shell := ["bash", "-eu", "-o", "pipefail", "-c"]

default:
    @just --list

# Generate templ and sqlc code.
generate:
    templ generate
    sqlc generate

# Tidy module dependencies.
tidy:
    go mod tidy

# Format Go source files.
fmt:
    git ls-files '*.go' | xargs gofmt -w

# Show Go's reviewed modernization suggestions without changing files.
modernize:
    go fix -diff ./...

# Generate code and build the local server binary.
build: generate
    mkdir -p tmp
    go build -o ./tmp/server ./cmd/server

# Generate code and run all Go tests.
test: generate
    go test ./...

# Run Go's static analysis after generating code.
vet: generate
    go vet ./...

# Run tests and build the application.
check: test vet build

# Reset and recreate the development database.
db-reset:
    goose reset
    goose up

# Reset the database, then start Air.
dev: db-reset
    air -c .air.toml

# Build and run the local server.
run: build
    ./tmp/server
