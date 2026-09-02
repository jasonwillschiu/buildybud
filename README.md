# buildybud

`buildybud` is a single Go CLI that consolidates the local `tools/*` build pipeline used in `jasonchiu-com4` into one installable command.
It is specifically built for repos using `go-datastar1` templates and the surrounding asset pipeline.

## Install

```bash
	go install github.com/jasonwillschiu/buildybud@v0.12.0
```

## Set Up In A Repo

1. Install the CLI.
2. `cd` into the repo you want to wire up.
3. Run `buildybud init`.
4. Review the generated `buildybud.toml` and adjust any repo-specific paths.
5. Run `buildybud doctor`.
6. Start replacing local build steps with `buildybud assets`, or with individual commands such as `buildybud css`, `buildybud js`, `buildybud og`, `buildybud images`, `buildybud favicon`, and `buildybud templui-map generate`.

`buildybud init` scans the repo for common directories such as `assets/embed/assets`, `assets/src/css/input.css`, `assets/src/js`, `assets/src/templui/assets/js`, `assets/src/favicon`, `ui`, `feature`, `core/router/router.go`, `assets/src/images/imageopt.config.json`, and `tools/imageopt/config.json`, then writes a starter `buildybud.toml`.
It also creates `.env.example` when the generated config is written inside the target repo, and appends the Cloudflare credential variables used by CDN purges.
The generated JS dependency defaults include `sheet -> dialog`, `dropdown -> popover`, `selectbox -> input + popover`, `datepicker -> input + popover`, `timepicker -> input + popover`, and `tagsinput -> popover`.

Use `buildybud init --force` to overwrite an existing config, or `buildybud init --dry-run` to preview generated TOML without writing files. The `.env.example` update is additive: missing documented `BB_` keys are appended to the end, and existing keys are left in place. Use `--config` on subcommands if the file lives elsewhere; relative config paths now resolve repo paths from the config file's directory, so monorepo app configs can live under paths such as `apps/web/buildybud.toml`.

## Commands

- `buildybud init`
- `buildybud assets`
- `buildybud css`
- `buildybud css audit`
- `buildybud manifest`
- `buildybud js`
- `buildybud js audit`
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

Image optimization can live directly in `buildybud.toml`:

```toml
[images]
enabled = true
source_dir = "assets/src/images"
output_dir = "assets/embed/assets/images"
formats = ["jpeg", "avif"]
sizes = [480, 768, 1200]
quality = { jpeg = 80, avif = 60 }
```

Inline image paths resolve from the app root selected by `paths.repo_root`. `output_dir` must be a child of `paths.assets_root` because the image stage removes stale files it owns. Widths must be unique positive values in ascending order, and quality values must be between 1 and 100.

Existing repositories may keep `config_path = "tools/imageopt/config.json"` while migrating. Do not combine `config_path` with inline image fields in the same configuration. New `buildybud init` output uses inline fields unless it discovers an existing imageopt JSON file.

`buildybud css` supports the backward-compatible `single` mode and a `bundles` mode for explicit named entry points. Use `command = "native"` to copy or minify plain CSS with Buildybud's built-in processor and no external CLI. Other command values invoke an external compiler with Tailwind-compatible flags. Bundle mode compiles every entry into a staging directory, audits the staged output, and publishes all fingerprints behind one atomic manifest update. If any compile or audit fails, the current manifest and shipped CSS remain unchanged. `buildybud css audit` checks the currently published bundle files without rebuilding them.

```toml
[css]
enabled = true
mode = "bundles"
command = "tailwindcss"
minify = true

[[css.bundle]]
name = "home"
input = "assets/src/css/entries/home.css"
output = "assets/embed/assets/css/home.css"

[[css.bundle]]
name = "app"
input = "assets/src/css/entries/app.css"
output = "assets/embed/assets/css/app.css"

[css.audit]
enabled = true

[[css.audit.group]]
name = "home-initial"
bundles = ["home"]
gzip_budget_bytes = 28000
```

Each audit group reports raw and gzip bytes. A zero gzip budget reports size without enforcing a limit. Named outputs must be unique and live beneath `paths.assets_root`; audit groups may only reference configured bundle names.

`buildybud og` renders configured route and blog Open Graph PNG sources. `buildybud images` runs enabled OG generation first, then creates optimized image variants. `buildybud favicon` copies favicon files into the embedded assets directory. `buildybud assets` runs enabled asset steps in order: CSS, JS, OG/images, favicon, and templui map generation. Use `buildybud assets --only css,js` to run a subset.

## JavaScript processing and size audits

Buildybud processes each `.js` source independently, fingerprints it, and updates the asset manifest. It does not concatenate files or force a module bundler. Set `minify = false` to preserve each source file byte-for-byte before hashing.

Repos without JavaScript can set `enabled = false`. `buildybud init` does this automatically when it finds neither `assets/src/js` nor a configured JavaScript copy directory. The `js` and `assets` commands then skip JavaScript output entirely.

`buildybud js` runs the configured build and then the audit when both are enabled. `buildybud js audit` runs only the audit, which also supports audit-only projects with `js.enabled = false`. With no named audit groups, the audit reports every `js/` entry in the asset manifest. Named groups can enforce separate gzip budgets for globally loaded code and larger route-only code. `files` includes vendor files that do not live in the manifest. A zero budget reports size without failing.

```toml
[js]
enabled = true
minify = true
out_dir = "assets/embed/assets/js"
hash_length = 8
src_dirs = ["assets/src/js"]

[js.audit]
enabled = true
inline_dirs = ["ui", "feature"]
max_inline_raw_bytes = 8192

[[js.audit.group]]
name = "global"
assets = ["js/app.js", "js/router.js"]
gzip_budget_bytes = 24576

[[js.audit.group]]
name = "maps"
assets = ["js/map.js"]
files = ["assets/embed/assets/js/map-vendor.js"]
gzip_budget_bytes = 0
```

The inline scan covers `.templ`, `.html`, `.gohtml`, and `.tmpl` files under `inline_dirs`. It ignores external `<script src>` tags and JSON data scripts such as JSON-LD.

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

`buildybud cdn` plans and executes Cloudflare cache purges from a typed route inventory in `buildybud.toml`.

- `plan` resolves both Git refs to commit SHAs and returns the union of every public route at both revisions. Including the old revision ensures deleted and renamed routes are purged.
- `purge` purges explicit paths or absolute URLs.
- `plan-and-purge` computes and executes the purge in one step.

```toml
[cdn]
enabled = true
base_url = "https://example.com"
hosts = ["example.com", "www.example.com"]
static_paths = ["/", "/about", "/blog", "/projects", "/tags", "/sitemap.xml", "/robots.txt"]
tag_prefix = "/tags"
redirects_file = "AppConstants.toml"
redirects_table = "redirects"

[[cdn.content]]
source_dir = "content/blog"
route_prefix = "/blog"
include_tags = true

[[cdn.content]]
source_dir = "content/projects"
route_prefix = "/projects"
include_tags = true
```

The planner reads Markdown `slug`, `tags`, and `draft` front matter plus the configured TOML redirects table at each commit. Draft content is excluded. Redirect targets retain query strings. Relative purge arguments expand across every configured host; absolute URLs are rejected unless their host appears in `cdn.hosts`.

Environment variables:

- `BB_CF_API_TOKEN` or `CF_API_TOKEN` (required for Cloudflare)
- `BB_CF_ZONE_ID` or `CF_ZONE_ID` (required for Cloudflare)

`buildybud` auto-loads a local `.env` file before command execution. The `BB_` names are preferred; unprefixed credential names remain supported for compatibility. Non-secret `base_url` and `hosts` values belong in TOML, with `BB_APP_BASE_URL`/`APP_BASE_URL` and `BB_CDN_PURGE_HOSTS`/`CDN_PURGE_HOSTS` available only as runtime overrides. Flags take precedence over env overrides, which take precedence over TOML.

When `buildybud init` writes config inside the target repo, it documents only the two secret Cloudflare values in `.env.example`. Purges are sent in batches of 100 URLs with a 15-second HTTP timeout. `--dry-run` prints the planned URLs without requiring credentials or making network calls.

## Taskfile wiring target

Use these task mappings in `jasonchiu-com4`:

- `task assets` or web asset deps -> `buildybud assets`
- `task css` -> `buildybud css`
- `task js` -> `buildybud js`
- `task og` -> `buildybud og`
- `task images` -> `buildybud images`
- `task favicon` -> `buildybud favicon`
- `task templui-map` -> `buildybud templui-map generate`
- `task cdn-purge-cloudflare` -> `buildybud cdn plan-and-purge ...`
- Standalone manifest post-step -> `buildybud manifest --input ... --logical ...`
- `task release` -> `mdrelease` (publish a new version to GitHub)

## Notes

- `css` requires the configured external command in `PATH`; `command = "native"` has no external dependency.
- `images` requires `vips` in `PATH`.
- `og` requires configured background, logo, font, route, and blog paths when enabled.
- `templui-map generate` is declarative from TOML rules.
- `templui-map suggest` is advisory scanner output.
- The repo assumptions and scan defaults are tuned for `go-datastar1` template layouts.
