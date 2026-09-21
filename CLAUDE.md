# Project Instructions

## Tech Stack
| Layer | Technology | Version |
|-------|-----------|---------|
| Language | Go | 1.24.6 |
| TUI framework | tview | latest (rivo) |
| Terminal layer | tcell | v2 |
| CLI framework | cobra | v1.9.1 |
| Logging | zap (sugared) | v1.27.0 |
| SSH config parsing | kevinburke/ssh_config (fork) | v1.4.2 |
| Build/Release | GoReleaser | v2 |
| Linting | golangci-lint | v1.64.2 |
| Formatting | gofumpt | v0.7.0 |
| Static analysis | staticcheck | 2024.1.1 |

## Architecture
Hexagonal (Ports & Adapters) pattern. The `internal/core` layer is the application
heart — it defines domain types, port interfaces, and service implementations.
Adapters in `internal/adapters` depend on the core but never the reverse.

```
cmd/main.go                          — entry point, wires everything together
  │
  ├── internal/core/
  │   ├── domain/                    — Server struct (domain model)
  │   ├── ports/                     — ServerRepository & ServerService interfaces
  │   └── services/                  — ServerService impl + os-specific sysprocattr
  │
  ├── internal/adapters/
  │   ├── data/ssh_config_file/     — ServerRepository impl (reads/writes SSH config)
  │   └── ui/                        — tview TUI (App, handlers, forms, list)
  │
  └── internal/logger/               — zap SugaredLogger factory
```

## Code Style
- Every Go file starts with the Apache 2.0 license header (see `.golangci.yml`
  `goheader` template — enforced by linter).
- File naming: `snake_case` (e.g. `server_service.go`, `crud.go`, `validation_test.go`).
- Package names: short, single-word (`domain`, `ports`, `services`, `ui`, `logger`).
- Go formatting: `gofumpt` + `go fmt` (run `make fmt`).
- Imports ordered: stdlib first, then external, then internal (goimports grouping).
- Structs use exported fields with inline comments for non-obvious domain fields.

## Testing
- Run tests: `make test` (runs `go test -race -coverprofile=coverage.out ./...`)
- Verbose: `make test-verbose`
- Short mode: `make test-short`
- Coverage report: `make coverage` (opens `coverage.html`)
- Benchmarks: `make benchmark`
- Test files: `*_test.go` live alongside source files; table-driven with `t.Run`
  subtests; use only stdlib `testing` (no external test frameworks).
- Tests touching filesystem state use `t.TempDir()` and `t.Cleanup()`.
- Linting excludes `dupl` and `lll` for the `internal/*` path; stricter rules
  apply to test files (see `.golangci.yml`).

## Build & Run
- Dev server (live from source): `make run`
- Build binary: `make build` → `bin/lazyssh`
- Install to GOBIN: `make install`
- Format code: `make fmt`
- Lint: `make lint` (golangci-lint)
- Static analysis: `make check` (staticcheck)
- All quality checks: `make quality` (fmt + vet + lint)
- Cross-compile: `make build-all` (linux/windows/darwin, amd64/arm64)

## Project Structure
| Directory | Purpose |
|-----------|---------|
| `cmd/main.go` | Entry point, dependency wiring |
| `internal/core/domain/` | Domain model — `Server` struct with all SSH config fields |
| `internal/core/ports/` | Port interfaces — `ServerRepository`, `ServerService` |
| `internal/core/services/` | Application services — ServerService impl, SSH/exec logic |
| `internal/adapters/data/ssh_config_file/` | Data adapter — reads/writes `~/.ssh/config` and `~/.ssh/config.d/*.conf` |
| `internal/adapters/ui/` | TUI adapter — tview primitives, event handlers, forms |
| `internal/logger/` | Logger factory (zap SugaredLogger → `~/.lazyssh/lazyssh.log`) |
| `docs/` | Screenshots for README |

## Conventions
- Git: feature branches from `main`; semantic PR titles
  (`type(scope): subject` — types: feat, fix, improve, refactor, docs, test, ci,
  chore, revert; scopes: ui, cli, config, parser).
- CI: `.github/workflows/go.yml` runs `make build` + `make test` on PRs/pushes to
  `main`; `release.yml` runs GoReleaser on tags; `semantic-prs.yml` enforces PR titles.
- SSH config files: `~/.ssh/config` is managed by lazyssh with non-destructive
  writes + atomic saves + backups. `~/.ssh/config.d/*.conf` files are read-only
  (grouped in UI, source tracked on each Server via `SourceFile` field).
- Metadata (pins, SSH counts, last seen) stored in `~/.lazyssh/metadata.json`.
- Error handling: Go idiomatic — return errors, log with zap SugaredLogger
  (`Errorw`, `Warnw`, `Infow`), no panics at runtime.
- SSH connections use system `ssh`/`ssh -G` binary (no Go SSH library).
