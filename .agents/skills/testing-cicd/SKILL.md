---
name: testing-cicd
description: Writing tests and CI/CD for tailrelay — Go unit tests, Python integration tests, CI pipeline jobs, and test infrastructure. Use when adding new tests, extending the integration suite, modifying ci.yml, or improving test coverage for any Go package or the container behaviour.
reviewed_at: b7ce114
---

# Tests & CI/CD

## Overview

tailrelay has three layers of testing:

1. **Go unit tests** — package-level tests in `webui/internal/` and `webui/internal/handlers/`
2. **Python integration tests** — pytest suite in `tests/integration/` that builds a Docker image, starts Compose, and runs HTTP checks inside the container
3. **CI pipeline** — GitHub Actions in `.github/workflows/ci.yml` with three jobs: `frontend`, `backend`, `integration`

All test locations are under version control. There are no legacy root-level test scripts remaining.

---

## Test Layout

```
webui/
├── internal/
│   ├── auth/
│   │   └── middleware_test.go          ✓ exists
│   ├── backup/
│   │   └── backup_test.go              ✓ exists
│   ├── caddy/
│   │   ├── manager_test.go             ✓ exists
│   │   ├── metrics_parser_test.go      ✓ exists
│   │   ├── metrics_store_test.go       ✓ exists
│   │   └── proxy_manager_test.go       ✓ exists
│   ├── handlers/
│   │   ├── auth_test.go                ✓ exists
│   │   ├── backup_test.go              ✓ exists
│   │   ├── caddy_test.go               ✓ exists
│   │   └── socat_test.go               ✓ exists
│   ├── socat/                          ✗ no tests yet
│   ├── tailscale/                      ✗ no tests yet
│   └── web/
│       └── server_test.go              ✓ exists
tests/
├── __init__.py
└── integration/
    ├── __init__.py
    ├── conftest.py                     session-scoped Docker fixtures
    ├── helpers.py                      subprocess + container_exec utilities
    └── test_integration.py             pytest test classes (332 lines)
```

**Packages without tests** — prioritise these when adding coverage:
- `webui/internal/socat/` — relay lifecycle, start/stop, config parsing
- `webui/internal/tailscale/` — status parsing, cache behaviour, client mocking
- `webui/internal/config/` — YAML parsing edge cases
- `webui/internal/handlers/dashboard.go`, `tailscale.go` — handler coverage

---

## Running Tests

### Go Unit Tests

```bash
# All packages
cd webui && go test ./...

# Verbose output
cd webui && go test -v ./...

# Single package
cd webui && go test ./internal/caddy/...
cd webui && go test ./internal/handlers/...

# With coverage
cd webui && go test -cover ./...
cd webui && go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out   # open browser report
```

### Python Integration Tests

```bash
# Full suite (builds image, starts Compose, runs tests, tears down)
pytest tests/integration/ -v

# Skip image build (use pre-built image)
BUILD_IMAGE=false pytest tests/integration/ -v

# Compact tracebacks
pytest tests/integration/ -v --tb=short

# Single test class
pytest tests/integration/test_integration.py::TestWebUI -v

# Stop on first failure
pytest tests/integration/ -v -x
```

### CI Locally (act)

```bash
# Requires act (https://github.com/nektos/act)
act -j backend
act -j integration
```

---

## Writing Go Unit Tests

### Conventions

- File naming: `<source_file>_test.go` in the same package directory
- Package declaration: use `package <pkg>` (same package) for whitebox tests, `package <pkg>_test` for blackbox
- Test function naming: `Test<Function>_<scenario>_<expected>` or `Test<Function>` with subtests
- Use `testing.T` for simple tests, `httptest` for handler tests
- Prefer table-driven tests for multiple input/output combinations

### Handler Tests (`internal/handlers/`)

All handlers receive `http.ResponseWriter` and `*http.Request`. Use `net/http/httptest`:

```go
package handlers_test

import (
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestMyHandler_Success(t *testing.T) {
    req := httptest.NewRequest(http.MethodGet, "/api/something", nil)
    w   := httptest.NewRecorder()

    handler := NewMyHandler(/* deps */)
    handler.ServeHTTP(w, req)

    resp := w.Result()
    if resp.StatusCode != http.StatusOK {
        t.Errorf("want 200, got %d", resp.StatusCode)
    }
}
```

### Table-Driven Tests

```go
func TestParseRelays(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr bool
        wantLen int
    }{
        {"empty input", "", false, 0},
        {"single relay", "tcp:8080:host:9090", false, 1},
        {"invalid format", "not-a-relay", true, 0},
    }

    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            relays, err := ParseRelays(tc.input)
            if (err != nil) != tc.wantErr {
                t.Fatalf("wantErr=%v, got err=%v", tc.wantErr, err)
            }
            if len(relays) != tc.wantLen {
                t.Errorf("want %d relays, got %d", tc.wantLen, len(relays))
            }
        })
    }
}
```

### Mocking External Services

For packages that call external processes (Caddy Admin API, `tailscale` CLI, `socat`), use interface mocking:

```go
// Define interface in production code
type CaddyClient interface {
    AddRoute(route CaddyRoute) error
    DeleteRoute(id string) error
}

// Use a fake in tests
type fakeCaddyClient struct {
    routes map[string]CaddyRoute
}
func (f *fakeCaddyClient) AddRoute(r CaddyRoute) error { ... }
```

Existing patterns to follow: `webui/internal/caddy/manager_test.go`, `webui/internal/handlers/caddy_test.go`.

---

## Writing Integration Tests

Integration tests live in `tests/integration/test_integration.py`. They interact with a real running container via `wget` inside the container and `docker exec`.

### Infrastructure

**`helpers.py`** — import these utilities:
```python
from tests.integration.helpers import (
    container_exec,       # run sh -c inside container, returns CompletedProcess
    container_exec_check, # same, raises AssertionError on non-zero exit
    CONTAINER_NAME,       # from TAILRELAY_HOST env var (default: tailrelay-test)
    run_cmd,              # run subprocess from repo root
    run_cmd_check,        # same, raises RuntimeError on non-zero exit
)
```

**`conftest.py`** — session fixtures:
- `docker_image` — builds the dev Docker image once per session
- `running_container` — starts Compose stack, waits for services, yields container name, tears down

### Test Structure

```python
class TestMySubsystem:
    """Tests for <subsystem> behaviour."""

    def test_mysubsystem_feature_succeeds(self, running_container: str) -> None:
        """<what this test verifies>."""
        exit_code, output = wget(running_container, "http://127.0.0.1:8021/api/endpoint")
        assert exit_code == 0
        data = json.loads(output)
        assert data["key"] == "expected_value"
```

Use `wget()` (defined in `test_integration.py`) for HTTP requests inside the container:

```python
def wget(container: str, url: str, extra_flags: str = "") -> tuple[int, str]:
    """Run wget inside the container, returns (exit_code, output)."""
```

### Test ID Pattern

```
test_<subsystem>_<what>_<expected_outcome>

Examples:
  test_webui_health_returns_200
  test_caddy_proxy_add_persists_after_restart
  test_socat_relay_invalid_port_rejected
```

### Addresses Inside the Container

| Service | Address |
|---------|---------|
| Web UI | `http://127.0.0.1:8021` |
| Tailscale health | `http://127.0.0.1:9002/healthz` |
| Tailscale metrics | `http://127.0.0.1:9002/metrics` |
| Caddy Admin API | `http://127.0.0.1:2019` |

### Environment Variables for Integration Tests

Set in `.env` (copy from `.env.example`) or export before running pytest:

```bash
COMPOSE_FILE=compose-test.yml        # which Compose file to use
TAILRELAY_HOST=tailrelay-test        # container name
TAILNET_DOMAIN=example.com           # mock tailnet domain
BUILD_IMAGE=true                     # set false to skip docker build
IMAGE_TAG=sudocarlos/tailrelay:dev   # image to test
STARTUP_WAIT=8                       # seconds to wait after start
```

---

## CI Pipeline (`.github/workflows/ci.yml`)

Triggers: push to `main`, push of `v*.*.*` tags, PR to `main`, published releases.

### Current Jobs

| Job | Runner | Working Dir | What It Does |
|-----|--------|-------------|-------------|
| `frontend` | ubuntu-latest | `webui/frontend` | Node 20 → `npm install` → `npm run build` |
| `backend` | ubuntu-latest | `webui` | Node 20 + Go 1.24 → `npm install` + `npm run build` (for `//go:embed all:web/dist`) → `go vet ./...` → `go test -v ./...` → `go build -v ./...` |
| `integration` | ubuntu-latest | repo root | Node 20 + Docker Buildx + Python 3.12 → `pytest tests/integration/ -v` |
| `release` | ubuntu-latest | repo root | Runs only on `v*.*.*` tags after all three above pass; builds multi-platform image, pushes to Docker Hub + GHCR, creates GitHub Release |

### Adding a New CI Job

Add jobs to `ci.yml` following this template:

```yaml
  my-new-job:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.24'
          cache-dependency-path: webui/go.sum

      - name: Run my checks
        working-directory: webui
        run: go run mychecker ./...
```

### Recommended CI Additions

| Job | Purpose | How |
|-----|---------|-----|
| `security` | Go module vulnerability scan | `govulncheck ./...` |
| `lint` | Static analysis | `golangci-lint run` |
| `coverage` | Upload coverage report | `go test -coverprofile=... && upload to codecov` |
| `trivy` | Container image CVE scan | `aquasecurity/trivy-action` |
| `multi-arch` | Verify arm64 build | `docker buildx build --platform linux/amd64,linux/arm64` |

Example security job:

```yaml
  security:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: webui
    steps:
      - uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.24'

      - name: Install govulncheck
        run: go install golang.org/x/vuln/cmd/govulncheck@latest

      - name: Run govulncheck
        run: govulncheck ./...
```

---

## Test Coverage Goals

| Package | Current | Priority |
|---------|---------|----------|
| `internal/auth` | ✓ `middleware_test.go` | Maintain |
| `internal/backup` | ✓ `backup_test.go` | Maintain |
| `internal/caddy` | ✓ `manager_test.go`, `proxy_manager_test.go`, `metrics_parser_test.go`, `metrics_store_test.go` | Maintain |
| `internal/handlers` | ✓ auth, backup, caddy, socat | Add: dashboard, tailscale handlers |
| `internal/web` | ✓ `server_test.go` | Maintain |
| `internal/socat` | ✗ none | **High** |
| `internal/tailscale` | ✗ none | **High** |
| `internal/config` | ✗ none | Medium |
| `internal/logger` | ✗ none | Low |

---

## Common Pitfalls

1. **Integration tests require Docker** — they cannot run without a working Docker daemon; CI uses `docker/setup-buildx-action@v3` for this.
2. **`BUILD_IMAGE=false` in CI** — if the integration job depends on a build job that has already produced the image, skip the rebuild by setting this env var.
3. **Startup wait time** — `STARTUP_WAIT` defaults to 8 seconds; flaky tests often need this increased in slow CI environments.
4. **Go test caching** — use `go clean -testcache` if tests appear to pass without running (cached results).
5. **Handler tests need real dependencies** — avoid testing handler methods in isolation by mocking at the interface boundary, not at the struct level.
