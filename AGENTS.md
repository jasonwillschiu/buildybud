# buildybud - Agent Reference

## Build
- `task build` to compile; `task test` to run tests
- `go vet ./...` before any commit

## Structure
- Entry: `main.go` -> `internal/app/app.go`
- CDN: `internal/cdn/cdn.go`
- Config: `internal/config/config.go`
- CSS: `internal/css/css.go`
- Favicon: `internal/favicon/favicon.go`
- Env loader: `internal/envfile/envfile.go`

## Rules
- This CLI is built for repos using `go-datastar1` templates and matching asset layout.
- Taskfile stays the main command layer; buildybud owns repeatable Go SSR web asset steps.
- Config paths resolve relative to `buildybud.toml` unless `paths.repo_root` is absolute.
- Asset capability sections support `enabled = false`; `doctor` must only require enabled tools/paths.
- `envOrDefault(keys..., fallback)` - variadic, checked in order, last arg is fallback
- CDN env vars have two forms: `APP_BASE_URL` and `BB_APP_BASE_URL` (unprefixed checked first)
- `.env` auto-loaded before command execution
- `buildybud init --dry-run` writes no files
- `buildybud init` writes `.env.example` only when config is generated inside the target repo; create if missing, append missing documented `BB_` vars, do not duplicate keys or legacy unprefixed aliases
- `buildybud init` seeds JS dependency defaults for `selectbox`, `datepicker`, `timepicker`, and `tagsinput`
- `buildybud.toml` required at repo/app root; `buildybud init` generates it
- `assets` macro runs enabled asset steps; keep CI/build orchestration in Taskfile
- `--version` = embedded CLI version; `version` subcommand = changelog version
- `css` command requires `tailwindcss` in PATH
- `images` command requires `vips` in PATH
- Never hardcode CDN credentials

## Docs
- Full details: `README.md`
- Architecture: `docs/buildybud-plan.md`
