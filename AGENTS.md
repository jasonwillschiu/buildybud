# buildybud - Agent Reference

## Build
- `task build` to compile; `task test` to run tests
- `go vet ./...` before any commit

## Structure
- Entry: `main.go` -> `internal/app/app.go`
- CDN: `internal/cdn/cdn.go`
- Config: `internal/config/config.go`
- CSS: `internal/css/` (native minification, external compilation, and bundle audits)
- Favicon: `internal/favicon/favicon.go`
- OG: `internal/og/og.go` + `internal/og/types.go`
- Env loader: `internal/envfile/envfile.go`

## Rules
- This CLI is built for repos using `go-datastar1` templates and matching asset layout.
- Taskfile stays the main command layer; buildybud owns repeatable Go SSR web asset steps.
- Config paths resolve relative to `buildybud.toml` unless `paths.repo_root` is absolute.
- Asset capability sections support `enabled = false`; `doctor` must only require enabled tools/paths. JS discovery disables build/audit when no source or copy directory exists; `js.minify` and `js.audit.enabled` remain independent controls.
- CDN is Cloudflare-only; keep route inventory in typed `[cdn]` TOML and credentials in env.
- CDN override precedence is flag, `BB_` env name, legacy unprefixed env name, then TOML.
- Plan the union of all public routes at immutable old and new commit SHAs so deleted and renamed routes are purged.
- `.env` auto-loaded before command execution
- `buildybud init --dry-run` writes no files
- `buildybud init` writes `.env.example` only when config is generated inside the target repo; document only Cloudflare credential vars and remove obsolete managed provider config
- `buildybud init` seeds JS dependency defaults for `selectbox`, `datepicker`, `timepicker`, and `tagsinput`
- `buildybud.toml` required at repo/app root; `buildybud init` generates it
- `assets` macro runs enabled asset steps; keep CI/build orchestration in Taskfile
- `--version` = embedded CLI version; `version` subcommand = changelog version
- `css.command = "native"` copies or minifies plain CSS without an external CLI, inlining relative `@import`s (layer/supports/media wrapped, `url()` rebased to the entry, remote imports hoisted; cycles, missing files and package or root-relative imports fail). Other command values must resolve in PATH and receive Tailwind-compatible flags. Keep `single` mode compatible; `bundles` mode must compile and audit all entries before one atomic manifest publish.
- `images` command requires `vips` in PATH
- Keep image settings inline in `[images]` for new configs. Treat `config_path` as a legacy alternative, never mix both modes, and keep `images.output_dir` below `paths.assets_root`.
- `og` render uses multi-step composition: cover → wash → brand → card text → footer URL; title is required, text auto-fits via font size reduction (68→52px) + ellipsis truncation
- Never hardcode CDN credentials

## Docs
- Full details: `README.md`
- Architecture: `docs/buildybud-plan.md`
