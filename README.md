# buildybud

`buildybud` is a single Go CLI that consolidates the local `tools/*` build pipeline used in `jasonchiu-com4` into one installable command.
It is specifically built for repos using `go-datastar1` templates and the surrounding asset pipeline.

## Install

```bash
	go install github.com/jasonwillschiu/buildybud@v0.9.0
```

## Set Up In A Repo

1. Install the CLI.
2. `cd` into the repo you want to wire up.
3. Run `buildybud init`.
4. Review the generated `buildybud.toml` and adjust any repo-specific paths.
5. Run `buildybud doctor`.
6. Start replacing local build steps with `buildybud assets`, or with individual commands such as `buildybud css`, `buildybud js`, `buildybud og`, `buildybud images`, `buildybud favicon`, and `buildybud templui-map generate`.

`buildybud init` scans the repo for common directories such as `assets/embed/assets`, `assets/src/css/input.css`, `assets/src/js`, `assets/src/templui/assets/js`, `assets/src/favicon`, `ui`, `feature`, `core/router/router.go`, `assets/src/images/imageopt.config.json`, and `tools/imageopt/config.json`, then writes a starter `buildybud.toml`.
It also creates `.env.example` when the generated config is written inside the target repo, and appends any missing CDN-related env vars with comments so repo setup documents the required `.env` shape.
The generated JS dependency defaults include `sheet -> dialog`, `dropdown -> popover`, `selectbox -> input + popover`, `datepicker -> input + popover`, `timepicker -> input + popover`, and `tagsinput -> popover`.

Use `buildybud init --force` to overwrite an existing config, or `buildybud init --dry-run` to preview generated TOML without writing files. The `.env.example` update is additive: missing documented `BB_` keys are appended to the end, and existing keys are left in place. Use `--config` on subcommands if the file lives elsewhere; relative config paths now resolve repo paths from the config file's directory, so monorepo app configs can live under paths such as `apps/web/buildybud.toml`.

## Commands

- `buildybud init`
- `buildybud assets`
- `buildybud css`
- `buildybud manifest`
- `buildybud js`
- `buildybud og`
- `buildybud images`
- `buildybud favicon`
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
Paths in the config are resolved relative to the directory containing `buildybud.toml` unless `paths.repo_root` is absolute. This allows a root Taskfile to call `buildybud --config apps/web/buildybud.toml assets` without first changing directories.

The asset capability sections support `enabled = false` for repos that do not use a step. `doctor` only checks tools and paths for enabled capabilities.

`buildybud css` runs the configured Tailwind command, writes the configured CSS output, hashes it, and updates the asset manifest. `buildybud og` renders configured route and blog Open Graph PNG sources. `buildybud images` runs enabled OG generation first, then creates optimized image variants. `buildybud favicon` copies favicon files into the embedded assets directory. `buildybud assets` runs enabled asset steps in order: CSS, JS, OG/images, favicon, and templui map generation. Use `buildybud assets --only css,js` to run a subset.

## Open Graph Images

Enable `[og]` to let buildybud generate reusable route and blog Open Graph image sources before image optimization.

```toml
[og]
enabled = true
site_name = "Example"
site_url = "example.com"
background = "assets/src/images/og/background.png"
logo = "assets/src/favicon/web-app-manifest-192x192.png"
font = "assets/src/fonts/og/OpenSans-Bold.ttf"
routes_file = "content/seo/routes.toml"
route_output_dir = "assets/src/images/og"
route_manifest = "assets/src/images/og/generated-manifest.json"
blog_dir = "content/blog"
blog_image_dir = "assets/src/images/blog"
raw_blog_image_dir = "assets/src/raw-images/blog"
blog_manifest = "assets/src/images/blog/generated-manifest.json"
```

Routes are declared in TOML.

```toml
[[route]]
path = "/"
image_name = "og-home"
title = "Find schools"
subtitle = "Compare public data clearly."
```

Blog posts use front matter fields `hero_image` and `hero_image_source`. Use `hero_image_source: generated` to render a card from `title` plus `excerpt` or `description`. Use `hero_image_source: custom` for local custom art under `raw_blog_image_dir`; buildybud watermarks the public `hero_image` PNG with the configured logo and site URL. Absolute custom URLs are skipped.

The OG card rendering pipeline composes these layers:
- **Cover** — background image scaled to fill the 1200x630 canvas
- **Wash** — gradient overlay (left-to-right white fade, bottom-to-top blue tint) for readability
- **Brand** — logo (62px square) + site name (uppercase, 26px) pinned to the top-left
- **Card text** — title (required) and subtitle auto-fitted with font size reduction (68px→52px, 29px→24px) and line truncation with ellipsis to fit within the text column
- **Footer URL** — site URL (20px) rendered at the bottom-left

## CDN

`buildybud cdn` supports Bunny and Cloudflare purges.

- `plan` computes purge paths and URLs from git diffs.
- `purge` purges explicit paths or absolute URLs.
- `plan-and-purge` computes and executes the purge in one step.
- Use `--provider bunny|cloudflare` or `CDN_PROVIDER` to choose the purge backend.

Environment variables:

- `BB_APP_BASE_URL` or `APP_BASE_URL`
- `BB_CDN_PROVIDER` or `CDN_PROVIDER` (optional; defaults to `bunny`)
- `BB_CDN_PURGE_HOSTS` or `CDN_PURGE_HOSTS` (optional comma-separated hostnames)
- `BB_BUNNY_API_KEY` or `BUNNY_API_KEY` (required for Bunny)
- `BB_CF_API_TOKEN` or `CF_API_TOKEN` (required for Cloudflare)
- `BB_CF_ZONE_ID` or `CF_ZONE_ID` (required for Cloudflare)

`buildybud` auto-loads a local `.env` file before command execution. The `BB_` names are the preferred repo-local form so CDN-related variables are clearly scoped; the unprefixed names remain supported for compatibility. If a required CDN variable is missing, the error tells you which env var or flag to set.

When `buildybud init` writes config inside the target repo, these variables are documented into `.env.example` with comments using the `BB_` names so it is explicit they belong to `buildybud`. If the file already exists, only missing keys are appended.

## Taskfile wiring target

Use these task mappings in `jasonchiu-com4`:

- `task assets` or web asset deps -> `buildybud assets`
- `task css` -> `buildybud css`
- `task js` -> `buildybud js`
- `task og` -> `buildybud og`
- `task images` -> `buildybud images`
- `task favicon` -> `buildybud favicon`
- `task templui-map` -> `buildybud templui-map generate`
- `task cdn-purge-bunny` -> `buildybud cdn plan-and-purge ...`
- Standalone manifest post-step -> `buildybud manifest --input ... --logical ...`
- `task release` -> `mdrelease` (publish a new version to GitHub)

## Notes

- `css` requires `tailwindcss` in `PATH` unless disabled in config.
- `images` requires `vips` in `PATH`.
- `og` requires configured background, logo, font, route, and blog paths when enabled.
- `templui-map generate` is declarative from TOML rules.
- `templui-map suggest` is advisory scanner output.
- The repo assumptions and scan defaults are tuned for `go-datastar1` template layouts.
