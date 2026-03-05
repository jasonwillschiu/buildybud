# buildybud

`buildybud` is a single Go CLI that consolidates the local `tools/*` build pipeline used in `jasonchiu-com4` into one installable command.

## Install

```bash
go install github.com/jasonwillschiu/buildybud@latest
```

## Commands

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

## Config

`buildybud` expects a `buildybud.toml` file at repo root by default.

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
