# 0.2.0 - Add: Repo bootstrap and clearer CLI guidance
- Add `buildybud init` to scan a repo and generate a starter `buildybud.toml`.
- Expand root `--help` with setup steps, examples, and documented global flags.
- Keep `buildybud --version` aligned with the embedded CLI version contract.
- Auto-load a local `.env` file and improve missing config / missing CDN env error messages.

# 0.1.0 - Initial buildybud release
- Add `js`, `manifest`, `images`, `templui-map`, and `doctor` commands.
- Add strict `buildybud.toml` configuration parsing and validation.
- Port behavior from jasonchiu-com4 local build tools into one CLI.
