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
