# justfile — THW-JugendOlympiade task runner
# Requires: just (https://just.systems), uv (https://astral.sh/uv), wails, go
#
# Usage:
#   just           → list all available recipes
#   just build     → production build
#   just test      → run all Go tests
#   just docs-serve → start live-reload docs server (http://localhost:7000)

set windows-shell := ["cmd.exe", "/c"]

uv    := "D:/saskia/develop/uv.exe"
wails := "wails"

# ─── Default: list recipes ────────────────────────────────────────────────────
default:
    @just --list

# ─── Go / Wails ───────────────────────────────────────────────────────────────

# Run the application in development mode with hot-reload
dev:
    {{wails}} dev

# Build a development binary with console window (output: build/bin/)
build:
    {{wails}} build


# Run all Go tests with verbose output
test:
    cd test && go test -v ./...

# Run tests with coverage report
test-cover:
    cd test && go test -cover ./...

# Download Go module dependencies
deps:
    go mod download

# ─── Documentation ────────────────────────────────────────────────────────────

# Start a local live-reload documentation server (http://localhost:7000)
docs-serve:
    {{uv}} sync --project assets
    {{uv}} run --project assets mkdocs serve --config-file assets/mkdocs.yml --dev-addr localhost:7000

# ─── Combined ─────────────────────────────────────────────────────────────────

# Run tests and build the app
all: test build
    @echo "All done."
