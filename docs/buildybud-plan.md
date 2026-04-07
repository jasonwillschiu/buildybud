# buildybud Plan (Repository-Specific Handoff)

This document is a planning handoff for implementing `buildybud`, a shared Go build tool extracted from this repo's local `tools/*` pipeline.

Repository context:
- Repo path: `/Users/jasonchiu/Documents/WVC/jasonchiu-com4`
- read from repo path as needed
- Current local tools to extract: `tools/assets-manifest`, `tools/build-js`, `tools/imageopt`, `tools/templui-map`, `tools/cdn`
- Existing orchestrator: `Taskfile.yml`

Design goals:
1. One installable tool: `buildybud`
2. One config file per repo: `buildybud.toml`
3. Declarative `templui-map` behavior as source of truth (scanner only as assistant)
4. Keep per-repo `Taskfile.yml` as orchestration layer, not as logic layer
5. Preserve existing asset/SEO/runtime contracts in this repo

---

## 1) Product Direction

`buildybud` is a multi-command CLI distributed from one repo and one version stream.

Initial command set:
- `buildybud js`
- `buildybud manifest`
- `buildybud images`
- `buildybud templui-map generate`
- `buildybud templui-map suggest`
- `buildybud templui-map check`
- `buildybud doctor`

Optional later:
- `buildybud cdn plan`
- `buildybud cdn purge`
- `buildybud cdn plan-and-purge`
- `buildybud build` (macro command that calls other commands)

Non-goals (v1):
- Replacing `task`
- Replacing `templ generate` or Tailwind directly
- Runtime server concerns unrelated to asset/build pipeline

---

## 2) Why TOML

Decision: use `buildybud.toml`.

Reasoning for this repo pattern:
- Human-edited config for template-derived repos is primary need.
- TOML maps cleanly to Go structs.
- Readability for route/component mapping is better than JSON for long-lived manual edits.
- Avoid YAML complexity for strict config semantics.

Validation approach:
- Parse TOML into typed structs.
- Enforce strict semantic checks in `buildybud doctor` and per-command `check` modes.

---

## 3) Repo-Specific Constraints to Preserve

These constraints come from this repo and must remain true post-migration:

1. Internal imports remain `jasonchiu-com4/...` inside this repo.
2. Generated output path for templui route map remains `core/templui/generated_routes.go` unless explicitly changed.
3. Embedded assets contract remains compatible with `assets/embed/assets/manifest.json` and existing runtime lookups.
4. Do not modify generated `assets/embed/` manually; generation remains command-driven.
5. Image variants and manifest format remain compatible with `core/images` runtime expectations.
6. Datastar selector rule remains respected in JS code (`[data-on\\:click]` style escapes).

---

## 4) Single Config Contract (`buildybud.toml`)

v1 schema proposal (single file only):

```toml
schema_version = 1
module_path = "jasonchiu-com4"
strict = true

[paths]
repo_root = "."
assets_root = "assets/embed/assets"
manifest_path = "assets/embed/assets/manifest.json"

[js]
out_dir = "assets/embed/assets/js"
hash_length = 8
src_dirs = ["assets/src/js"]
copy_dirs = ["assets/src/templui/assets/js"]
scan_template_dirs = ["ui", "feature"]
templui_component_dir = "assets/src/templui/assets/js"

[js.dependencies]
sheet = ["dialog"]
dropdown = ["popover"]
selectbox = ["input", "popover"]
datepicker = ["input", "popover"]
timepicker = ["input", "popover"]
tagsinput = ["popover"]

[manifest]
hash_length = 8
cleanup_stale = true

[images]
config_path = "tools/imageopt/config.json"
# Optional inline override support later:
# source_dir = "assets/src/images"
# output_dir = "assets/embed/assets/images"
# formats = ["jpeg", "avif"]
# sizes = [800, 1200]

[templui_map]
mode = "declarative" # declarative|hybrid
out = "core/templui/generated_routes.go"
component_dir = "assets/src/templui/assets/js"
default_components = ["dialog"]
longest_prefix_match = true
fail_on_missing_component = true

[[templui_map.rule]]
prefix = "/"
components = ["dialog"]

[[templui_map.rule]]
prefix = "/blog"
components = ["dialog", "popover"]

[[templui_map.rule]]
prefix = "/projects"
components = ["dialog", "selectbox"]

[templui_map.suggest]
enabled = true
scan_router = "core/router/router.go"
scan_dirs = ["ui", "feature"]
```

Validation rules:
1. `schema_version` must match supported versions.
2. Every templui component in rules must exist in `component_dir`.
3. No duplicate `templui_map.rule.prefix`.
4. `prefix` must start with `/`.
5. Hash lengths must be in safe range.
6. Required directories/files must exist unless explicitly optional.
7. Config parse should fail on unknown keys in strict mode.

---

## 5) Architecture Options Revisited (for implementation path)

### A. Pure declarative templui map (recommended default)
- `generate` uses only `templui_map.rule` from TOML.
- `suggest` is advisory and prints candidate updates.

Pros:
- Explicit and predictable across repos.
- No hard dependency on AST/router conventions.
- Easy for template repos where manual config edits are acceptable.

Cons:
- Requires config updates when routes/components change.

### B. Hybrid declarative + scanner
- Declarative config is authoritative.
- Scanner suggests diffs and optionally auto-writes with explicit flag.

Pros:
- Reduces manual maintenance load.

Cons:
- Slightly more behavior to understand.

Decision:
- Start with A + `suggest` command from B.

---

## 6) Detailed Migration Plan

### Phase 0: Scaffold `buildybud` repo
1. Create new repo `buildybud`.
2. Create command layout:
   - `cmd/buildybud/main.go`
   - `internal/config`
   - `internal/js`
   - `internal/manifest`
   - `internal/images`
   - `internal/templuimap`
   - `internal/doctor`
3. Add CLI framework or stdlib flag/subcommand dispatcher.

Deliverable:
- `buildybud --help` with subcommands.

### Phase 1: Port assets-manifest
1. Move logic from `tools/assets-manifest/main.go` into `internal/manifest`.
2. Keep file hashing and cleanup semantics.
3. Support CLI flags overriding TOML values.

Deliverable:
- `buildybud manifest` equivalent to current behavior.

### Phase 2: Port JS builder
1. Move logic from `tools/build-js/main.go` into `internal/js`.
2. Preserve minification behavior and manifest updates.
3. Preserve templui dependency expansion semantics.

Deliverable:
- `buildybud js` replaces `go run ./tools/build-js ...` in this repo.

### Phase 3: Port image optimization
1. Port `tools/imageopt/main.go` to `internal/images`.
2. Keep compatibility with existing image manifest structure.
3. Keep cache semantics (`cache.json`) and stale-prune behavior.
4. Continue requiring `vips` installed.

Deliverable:
- `buildybud images` behavior parity for this repo.

### Phase 4: Replace templui-map with declarative model
1. Implement `templui-map generate` from TOML rules.
2. Keep generated output format compatible with current runtime lookup expectations.
3. Implement `templui-map check`.
4. Implement `templui-map suggest` (non-authoritative helper).

Deliverable:
- `core/templui/generated_routes.go` generated from config, not AST-only inference.

### Phase 5: Repo integration (`jasonchiu-com4`)
1. Add `buildybud.toml` to this repo root.
2. Update `Taskfile.yml` to call `buildybud ...`.
3. Keep old local tools briefly as fallback (or remove in same PR if confidence is high).
4. Update README commands and architecture notes.

Deliverable:
- This repo builds with `task build2` using `buildybud` under the hood.

### Phase 6: Harden and scale to template repos
1. Add golden tests for generated artifacts.
2. Add CI matrix with sample repos if available.
3. Define breaking-change policy and changelog rules.

Deliverable:
- repeatable adoption path for all similar web app repos.

---

## 7) Taskfile Integration (Target State for This Repo)

Current commands in this repo should migrate approximately to:
- `task js` -> `buildybud js`
- `task images` -> `buildybud images`
- `task templui-map` -> `buildybud templui-map generate`
- `task css` post-step manifest call -> `buildybud manifest --input ... --logical ...`

Notes:
- Keep `task templ` unchanged.
- Keep `task lint` unchanged.
- Keep `task build2` orchestration pattern unchanged.

---

## 8) Risk Register and Mitigations

1. Version drift across repos
- Mitigation: pin install version in template instructions and bump intentionally.

2. Silent behavior changes in generated assets
- Mitigation: golden tests + `check` commands + deterministic ordering.

3. templui component mismatch
- Mitigation: `templui-map check` fails on unknown/missing components.

4. Path assumptions break in other repos
- Mitigation: all paths in TOML; no hardcoded `jasonchiu-com4` assumptions in tool logic.

5. External dependency (`vips`) variability
- Mitigation: `doctor` checks binary availability and surfaces actionable errors.

---

## 9) Acceptance Criteria (for first end-to-end implementation)

1. This repo can run `task build2` successfully with `buildybud` replacing local tools.
2. Output artifacts remain compatible:
   - `assets/embed/assets/manifest.json`
   - `assets/embed/assets/images/manifest.json`
   - `core/templui/generated_routes.go`
3. `task lint` still passes.
4. `buildybud doctor` catches misconfigurations with clear errors.
5. No generated file contracts are broken for runtime behavior.

---

## 10) Immediate Next Steps for the Next LLM

1. Create `docs/buildybud-implementation-checklist.md` with concrete checklist items tied to this plan.
2. Propose exact `buildybud.toml` contents for this repo from current `Taskfile.yml` and `tools/*` defaults.
3. Draft the first migration PR scope:
   - port `manifest` and `js` only
   - wire `Taskfile` updates
   - add docs and rollback notes.
4. After merge, continue with `images`, then `templui-map` declarative migration.
