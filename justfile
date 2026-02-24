# Justfile for pmk CLI project
# Run `just --list` to see available commands

# Default recipe: run all quality checks
default: check

# Run all unit tests (excludes generated acceptance tests)
test:
    go test $(go list ./... | grep -v '/generated-acceptance-tests$')

# Run tests with verbose output (excludes generated acceptance tests)
test-verbose:
    go test -v $(go list ./... | grep -v '/generated-acceptance-tests$')

# Run tests with coverage report (excludes main package)
test-cover:
    go test -coverprofile=coverage.out $(go list ./... | grep -v '^github.com/eykd/prosemark-go$')
    go tool cover -func=coverage.out

# Run tests with HTML coverage report
test-cover-html: test-cover
    go tool cover -html=coverage.out -o coverage.html
    @echo "Coverage report: coverage.html"

# Check test coverage meets threshold (excludes main package)
# Note: *Impl functions are filtered out because they wrap external commands
# or OS-level operations that require fault injection to test.
# See .claude/skills/go-tdd/references/coverage.md for exemption guidelines
#
# Policy: 100% coverage required for all non-Impl, non-main functions
test-cover-check:
    #!/usr/bin/env bash
    set -uo pipefail
    PACKAGES=$(go list ./... | grep -v '^github.com/eykd/prosemark-go$' | grep -v '/cmd/pipeline$' | grep -v '/generated-acceptance-tests$')
    go test -coverprofile=coverage.out $PACKAGES || exit 1
    # Check that all non-Impl functions are at 100%
    # Filter out: Impl functions (exempt), main (exempt), total line, and 100% functions
    UNCOVERED=$(go tool cover -func=coverage.out | grep -v "Impl" | grep -v "^.*main.go:" | grep -v "100.0%" | grep -v "^total:" || true)
    if [ -n "$UNCOVERED" ]; then
        echo "The following non-Impl functions are not at 100% coverage:"
        echo "$UNCOVERED"
        exit 1
    fi
    TOTAL=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')
    echo "Coverage: ${TOTAL} (100% of non-Impl functions covered)"

# Run go vet
vet:
    go vet ./...

# Run staticcheck linter
lint:
    staticcheck ./...

# Format code
fmt:
    gofmt -w .

# Check formatting (fails if not formatted)
fmt-check:
    @test -z "$(gofmt -l .)" || (echo "Files not formatted:"; gofmt -l .; exit 1)

# Run all quality gates
check: fmt-check vet lint test-cover-check

# Build the binary
build:
    go build -o bin/pmk .

# Smoke-test: errors are visible, not silent
smoke: build
    #!/usr/bin/env bash
    set -euo pipefail
    BIN="$(pwd)/bin/pmk"
    DIR=$(mktemp -d)
    trap "rm -rf $DIR" EXIT
    # Must print error, not be silent
    OUTPUT=$("$BIN" list 2>&1) && { echo "FAIL: expected non-zero exit"; exit 1; } || true
    if [ -z "$OUTPUT" ]; then
        echo "FAIL: 'pmk list' outside project produced no output"
        exit 1
    fi
    echo "Smoke test passed"

# Clean build artifacts
clean:
    rm -rf bin/ coverage.out coverage.html generated-acceptance-tests/ acceptance-pipeline/ir/

# Install dependencies
deps:
    go mod download
    go mod tidy

# Run a single test by name (usage: just test-one TestName)
test-one NAME:
    go test -v -run {{NAME}} ./...

# Full acceptance pipeline: parse specs -> generate tests -> run
acceptance:
    ./run-acceptance-tests.sh

# Parse specs/*.txt to IR JSON
acceptance-parse:
    go run ./acceptance/cmd/pipeline -action=parse

# Generate Go tests from IR JSON
acceptance-generate:
    go run ./acceptance/cmd/pipeline -action=generate

# Run generated acceptance tests only
acceptance-run:
    go test -v ./generated-acceptance-tests/...

# Force-regenerate all acceptance stubs (destroys bound implementations!)
acceptance-regen:
    rm -rf generated-acceptance-tests/ acceptance-pipeline/ir/
    ./run-acceptance-tests.sh

# Run conformance tests against the pmk binary
conformance-run:
    go test -v ./conformance/...

# Run both unit tests and acceptance tests
test-all: test acceptance conformance-run
