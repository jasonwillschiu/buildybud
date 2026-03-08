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
- Config file: `buildybud.toml` (repo root)
- Install: `go install github.com/jasonwillschiu/buildybud@v0.4.0`
- Primary target: repos using `go-datastar1` templates and the matching asset layout

## Hard Invariants
- `envOrDefault` accepts variadic keys: checked in order, last arg is the fallback value.
- CDN env vars support two forms: unprefixed (`APP_BASE_URL`) and `BB_`-prefixed (`BB_APP_BASE_URL`). Unprefixed is checked first.
- `.env` file is auto-loaded before command execution.
- `buildybud init` also maintains `.env.example`: create it if missing, append missing documented env vars, do not duplicate existing keys.
- `buildybud.toml` must exist or CLI tells user to run `buildybud init`.
- `buildybud --version` prints embedded CLI version; `buildybud version` prints changelog version.

## Project Structure
```
main.go                     CLI entry point
internal/app/               Root command, help, init, doctor, version
internal/cdn/               CDN plan/purge logic (Bunny + Cloudflare)
internal/config/            TOML config parsing + validation
internal/envfile/           .env loader + .env.example writer
internal/js/                JS bundling
internal/images/            Image optimization (requires vips)
internal/manifest/          Asset manifest generation
internal/templamap/         templui-map generate/suggest/check
```

## Key Paths
| Concern | Path |
|---------|------|
| CDN logic | `internal/cdn/cdn.go` |
| Config parsing | `internal/config/config.go` |
| CLI entry + init | `internal/app/app.go` |
| Env loading / example file | `internal/envfile/` |
| Taskfile | `Taskfile.yml` |

## Reference Docs
- `docs/buildybud-plan.md` - Full architecture and feature plan
- `docs/jasonchiu-com4-migration-notes.md` - Migration context from jasonchiu-com4
- `README.md` - Full setup, commands, env vars, and config details
