# buildybud - Agent Reference

## Build
- `task build` to compile; `task test` to run tests
- `go vet ./...` before any commit

## Structure
- Entry: `main.go` -> `internal/app/app.go`
- CDN: `internal/cdn/cdn.go`
- Config: `internal/config/config.go`
- Env loader: `internal/envfile/envfile.go`

## Rules
- This CLI is built for repos using `go-datastar1` templates and matching asset layout.
- `envOrDefault(keys..., fallback)` - variadic, checked in order, last arg is fallback
- CDN env vars have two forms: `APP_BASE_URL` and `BB_APP_BASE_URL` (unprefixed checked first)
- `.env` auto-loaded before command execution
- `buildybud init` also writes `.env.example`; create if missing, append missing documented vars, do not duplicate keys
- `buildybud.toml` required at repo root; `buildybud init` generates it
- `--version` = embedded CLI version; `version` subcommand = changelog version
- `images` command requires `vips` in PATH
- Never hardcode CDN credentials

## Docs
- Full details: `README.md`
- Architecture: `docs/buildybud-plan.md`
