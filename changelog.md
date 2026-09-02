# 0.13.0 - Change: Cloudflare-only CDN ownership
- Replace Bunny/provider branching with one Cloudflare purge path and remove provider flags and environment settings.
- Move public route inventory into typed `[cdn]` TOML, including static routes, Markdown content roots, tag routes, and redirects.
- Plan the union of routes at immutable old and new commit SHAs so deleted, renamed, draft, tag, and redirect changes purge safely.
- Restrict absolute purge URLs to configured hosts, preserve redirect query strings, batch Cloudflare requests at 100 URLs, and apply a 15-second timeout.
- Keep only Cloudflare credentials in generated `.env.example` files while retaining legacy unprefixed credential aliases.

# 0.12.0 - Add: Inline image configuration
- Add inline image source, output, format, width, and quality settings to `buildybud.toml`.
- Keep legacy JSON configuration available as a mutually exclusive migration path.
- Resolve image filesystem paths from the configured app root while preserving repo-relative manifest and cache paths.
- Validate image widths, qualities, and prune-safe output directories.

# 0.11.2 - Update: Readme version pin
- v0.11.2 now

# 0.11.1 - Add: Native plain-CSS builds
- Add a built-in `css.command = "native"` processor for copying or minifying plain CSS without Tailwind or another external CLI.

# 0.11.0 - Add: Transactional named CSS bundles
- Add explicit named CSS entry points with independent fingerprinted manifest assets.
- Add CSS gzip audit groups and enforce configured budgets before publishing.
- Publish a complete CSS bundle set through one atomic manifest update so failed builds retain the last good assets.

# 0.10.0 - Add: Configurable JS size audits
- Add `buildybud js audit` with named gzip budgets, direct vendor files, and inline-script checks.
- Add optional JS minification while preserving per-file fingerprinting and manifest output.
- Disable JS build and audit during repo discovery when no source or copy directory exists.
- Run enabled audits after `buildybud js` and the JS stage of `buildybud assets`.

# 0.9.0 - Update: Rich OG card composition with text fitting
- Refactor OG card rendering to multi-layer pipeline: cover, wash, brand, card text, footer URL
- Add auto-fitting title/subtitle with font size reduction (68→52px) and ellipsis truncation
- Add brand rendering (site name + logo) and footer site URL
- Add validation that OG card title is required

# 0.8.0 - Add: Declarative Open Graph image generation
- Add `buildybud og` for declarative route and blog Open Graph PNG generation.
- Add `[og]` config for brand assets, route card TOML, blog hero front matter, generated-card manifests, and custom-hero watermarking.
- Run enabled OG generation before image optimization in `buildybud images` and `buildybud assets`.

# 0.7.1 - Update: Version update in README
- Update version to install in README.md

# 0.7.0 - Add: Cleanup removed embed images that are no longer in src
- Prune optimized image variants and cache entries when their configured source images have been deleted.

# 0.6.0 - Add: Harden web asset orchestration
- Add `css`, `favicon`, and `assets` commands so Taskfiles can delegate repeatable Go SSR web asset steps to buildybud.
- Resolve config-relative paths from the `buildybud.toml` directory, improving monorepo app support.
- Add capability `enabled` flags and make `doctor` check only enabled tools and paths.
- Add `init --dry-run` and avoid `.env.example` updates when generating config outside the target repo.

# 0.5.0 - Update: JS dependency defaults
- Expand the generated JS dependency map so `selectbox`, `datepicker`, and `timepicker` pull in `input` plus `popover`, and `tagsinput` pulls in `popover`.
- Keep the repo docs and init output aligned with the new default dependency set.

# 0.4.1 - Fix: Canonicalize BB_ env example vars
- Change `.env.example` generation to emit only `BB_`-prefixed buildybud variables.
- Treat legacy unprefixed env names as already documented so reruns do not append duplicate `BB_` entries.
- Clarify in docs that `.env.example` uses buildybud-scoped variables.

# 0.4.0 - Add: Init writes .env.example
- Add `.env.example` generation during `buildybud init`, creating the file when missing.
- Append missing CDN-related env vars with comments at the end instead of duplicating existing keys.
- Document `buildybud` as a CLI built for `go-datastar1` template repos.

# 0.3.1 - Update: Docs prompt versioning rule
- Add rule to update `go install` version to exact semver (not `latest`) in README.md.

# 0.3.0 - Add: BB_-prefixed env var aliases and release task
- Support `BB_` prefixed env vars (e.g., `BB_APP_BASE_URL`) alongside unprefixed names for clearer repo-local scoping.
- Refactor `envOrDefault` to accept variadic keys with ordered fallback.
- Add `task release` wiring via `mdrelease`.
- Update error messages and help text to mention both env var forms.

# 0.2.0 - Add: Repo bootstrap and clearer CLI guidance
- Add `buildybud init` to scan a repo and generate a starter `buildybud.toml`.
- Expand root `--help` with setup steps, examples, and documented global flags.
- Keep `buildybud --version` aligned with the embedded CLI version contract.
- Auto-load a local `.env` file and improve missing config / missing CDN env error messages.

# 0.1.0 - Initial buildybud release
- Add `js`, `manifest`, `images`, `templui-map`, and `doctor` commands.
- Add strict `buildybud.toml` configuration parsing and validation.
- Port behavior from jasonchiu-com4 local build tools into one CLI.
