# buildybud - Claude Code Guide

## Must Follow
- Run `go vet ./...` before committing.
- Do not commit broken builds; run `task build` to verify.
- Never hardcode CDN credentials in source files.

## Essential Commands
- `task build` - compile to `./bin/buildybud`
- `task test` - run all tests
- `task doctor` - run `buildybud doctor` health check
- `task release` - publish new version via `mdrelease`
- `go vet ./...` - lint before commit

## Quick Facts
- Module: `github.com/jasonwillschiu/buildybud`
- Entry point: `main.go`
- Config file: `buildybud.toml` (repo root or Go SSR app root)
- Install: `go install github.com/jasonwillschiu/buildybud@v0.10.0`
- Primary target: repos using `go-datastar1` templates and the matching asset layout
- Taskfile remains the public command layer; buildybud owns reusable Go SSR web asset steps.

## Hard Invariants
- `envOrDefault` accepts variadic keys: checked in order, last arg is the fallback value.
- CDN env vars support two forms: unprefixed (`APP_BASE_URL`) and `BB_`-prefixed (`BB_APP_BASE_URL`). Unprefixed is checked first.
- `.env` file is auto-loaded before command execution.
- `buildybud init` also maintains `.env.example`: create it if missing, append missing documented `BB_` env vars, do not duplicate existing keys or legacy unprefixed aliases.
- `buildybud init --dry-run` writes no files, and `init` only updates `.env.example` when the generated config is inside the target repo.
- Config paths resolve relative to `buildybud.toml` unless `paths.repo_root` is absolute.
- Asset capability sections support `enabled = false`; `doctor` must only require enabled tools and paths. JS discovery disables build/audit without source or copy directories; minification and auditing are separate controls.
- `buildybud init` seeds JS dependency defaults for `selectbox`, `datepicker`, and `timepicker` with `input` + `popover`, and `tagsinput` with `popover`.
- `buildybud.toml` must exist or CLI tells user to run `buildybud init`.
- `buildybud assets` is a narrow asset macro, not a replacement for Taskfile CI orchestration.
- `buildybud --version` prints embedded CLI version; `buildybud version` prints changelog version.

## Project Structure
```
main.go                     CLI entry point
internal/app/               Root command, help, init, doctor, version
internal/cdn/               CDN plan/purge logic (Bunny + Cloudflare)
internal/config/            TOML config parsing + validation
internal/css/               Tailwind CSS build command wrapper
internal/envfile/           .env loader + .env.example writer
internal/favicon/           Favicon copy step
internal/js/                JS processing and size audits
internal/images/            Image optimization (requires vips)
internal/manifest/          Asset manifest generation
internal/og/                Open Graph PNG rendering (multi-layer composition)
internal/templuimap/        templui-map generate/suggest/check
```

## Key Paths
| Concern | Path |
|---------|------|
| CDN logic | `internal/cdn/cdn.go` |
| Config parsing | `internal/config/config.go` |
| CLI entry + init | `internal/app/app.go` |
| Env loading / example file | `internal/envfile/` |
| OG card rendering | `internal/og/og.go` + `internal/og/types.go` |
| Taskfile | `Taskfile.yml` |

## Reference Docs
- `docs/buildybud-plan.md` - Full architecture and feature plan
- `docs/jasonchiu-com4-migration-notes.md` - Migration context from jasonchiu-com4
- `README.md` - Full setup, commands, env vars, and config details
