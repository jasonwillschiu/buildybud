# buildybud

`buildybud` is a single Go CLI that consolidates the local `tools/*` build pipeline used in `jasonchiu-com4` into one installable command.

## Install

```bash
go install github.com/jasonwillschiu/buildybud@v0.2.0
```

## Set Up In A Repo

1. Install the CLI.
2. `cd` into the repo you want to wire up.
3. Run `buildybud init`.
4. Review the generated `buildybud.toml` and adjust any repo-specific paths.
5. Run `buildybud doctor`.
6. Start replacing local build steps with `buildybud js`, `buildybud images`, `buildybud manifest`, or `buildybud templui-map generate`.

`buildybud init` scans the repo for common directories such as `assets/embed/assets`, `assets/src/js`, `assets/src/templui/assets/js`, `ui`, `feature`, `core/router/router.go`, and `tools/imageopt/config.json`, then writes a starter `buildybud.toml`.

Use `buildybud init --force` to overwrite an existing config. Use `--config` on subcommands if the file lives elsewhere.

## Commands

- `buildybud init`
- `buildybud manifest`
- `buildybud js`
- `buildybud images`
- `buildybud cdn plan`
- `buildybud cdn purge`
- `buildybud cdn plan-and-purge`
- `buildybud templui-map generate`
- `buildybud templui-map suggest`
- `buildybud templui-map check`
- `buildybud doctor`
- `buildybud version`

## Help And Version

- `buildybud --help` prints a short quick-start guide, command list, examples, and global flags.
- `buildybud --version` prints the installed CLI version as `buildybud version vX.Y.Z`.
- `buildybud version` prints the latest changelog version as plain semver.

## Config

`buildybud` expects a `buildybud.toml` file at repo root by default.

If the file is missing, `buildybud` tells you to run `buildybud init` or pass `--config <path>`.

Override config path with `--config` on all subcommands.

## CDN

`buildybud cdn` supports Bunny and Cloudflare purges.

- `plan` computes purge paths and URLs from git diffs.
- `purge` purges explicit paths or absolute URLs.
- `plan-and-purge` computes and executes the purge in one step.
- Use `--provider bunny|cloudflare` or `CDN_PROVIDER` to choose the purge backend.

Environment variables:

- `APP_BASE_URL`
- `CDN_PROVIDER` (optional; defaults to `bunny`)
- `CDN_PURGE_HOSTS` (optional comma-separated hostnames)
- `BUNNY_API_KEY` (required for Bunny)
- `CF_API_TOKEN` (required for Cloudflare)
- `CF_ZONE_ID` (required for Cloudflare)

`buildybud` auto-loads a local `.env` file before command execution. If a required CDN variable is missing, the error tells you which env var or flag to set.

## Taskfile wiring target

Use these task mappings in `jasonchiu-com4`:

- `task js` -> `buildybud js`
- `task images` -> `buildybud images`
- `task templui-map` -> `buildybud templui-map generate`
- `task cdn-purge-bunny` -> `buildybud cdn plan-and-purge ...`
- CSS manifest post-step -> `buildybud manifest --input ... --logical ...`

## Notes

- `images` requires `vips` in `PATH`.
- `templui-map generate` is declarative from TOML rules.
- `templui-map suggest` is advisory scanner output.
