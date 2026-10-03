set shell := ["bash", "-eu", "-o", "pipefail", "-c"]
set export

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
    git ls-files -co --exclude-standard -z -- '*.go' | while IFS= read -r -d '' file; do test -f "$file" && gofmt -w "$file"; done

# Show Go's reviewed modernization suggestions without changing files.
modernize:
    go fix -diff ./...

# Count source lines for the refactor line-count gate.
# Handwritten counts Go, templ, and SQL that are not generated; the total counts everything tracked.
loc:
    @printf 'handwritten go/templ/sql (non-generated, incl tests): '
    @git ls-files -z '*.go' '*.templ' '*.sql' | grep -zvE '(_templ\.go$|\.sql\.go$)' | xargs -0 wc -l | tail -1
    @printf 'total tracked source: '
    @git ls-files -z | xargs -0 wc -l | tail -1

# Generate code and build the local binaries.
build: generate
    mkdir -p tmp
    go build -ldflags "-X main.version=$(git describe --tags --always)" -o ./tmp/server ./cmd/server
    go build -ldflags "-X main.version=$(git describe --tags --always)" -o ./tmp/pmctl ./cmd/pmctl

# Cross-compile release binaries and checksums into dist/.
dist VERSION="dev": generate
    [[ "$VERSION" =~ ^[A-Za-z0-9][A-Za-z0-9._+-]*$ ]] || { printf 'invalid release version: %s\n' "$VERSION" >&2; exit 1; }
    mkdir -p dist
    rm -f dist/*
    CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -trimpath -ldflags="-s -w -X main.version=$VERSION" -o dist/server-linux-amd64 ./cmd/server
    CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w -X main.version=$VERSION" -o dist/server-windows-amd64.exe ./cmd/server
    CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -trimpath -ldflags="-s -w -X main.version=$VERSION" -o dist/pmctl-linux-amd64 ./cmd/pmctl
    CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w -X main.version=$VERSION" -o dist/pmctl-windows-amd64.exe ./cmd/pmctl
    cd dist && sha256sum * > SHA256SUMS

# Generate code and run all Go tests.
test: generate
    go test ./...

# Run Go's static analysis after generating code.
vet: generate
    go vet ./...

# Run staticcheck and the dropped-value analyzer; drops need a reason comment.
lint: generate
    staticcheck ./...
    go run ./tools/droppedcheck ./...

# Run tests, build the application, and lint.
check: test vet build lint

# Reset and recreate the development database.
db-reset:
    goose reset
    goose up

# Reset the development database and fill it with example data.
seed: db-reset generate
    go run ./tools/devseed

# Start Air without changing the development database.
dev:
    air -c .air.toml

# Reset the database, seed example data, then start Air.
dev-reset: seed dev

# Build and run the local server.
run: build
    ./tmp/server
