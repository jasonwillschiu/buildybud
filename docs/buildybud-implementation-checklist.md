# buildybud Implementation Checklist

## Phase 0 - Scaffold
- [x] Create Go module and CLI entrypoints (`main.go`, `cmd/buildybud/main.go`)
- [x] Create internal packages: `config`, `app`, `manifest`, `js`, `images`, `templuimap`, `doctor`
- [x] Provide `buildybud --help` and `--version`

## Phase 1 - Manifest
- [x] Port `tools/assets-manifest` hashing + manifest update behavior
- [x] Preserve sorted JSON and stale hashed-file cleanup behavior
- [x] Support TOML defaults with CLI flag overrides

## Phase 2 - JS
- [x] Port `tools/build-js` minification/hashing behavior
- [x] Preserve `manifest.json` semantics for `js/*` entries
- [x] Preserve templui usage scanning and dependency expansion
- [x] Preserve copy-only templui JS behavior for used components

## Phase 3 - Images
- [x] Port `tools/imageopt` vips-based variant generation
- [x] Preserve output naming and manifest format compatibility
- [x] Preserve `cache.json` semantics and stale prune behavior
- [x] Keep hard requirement on `vips`

## Phase 4 - templui-map
- [x] Implement declarative `templui-map generate` from TOML rules
- [x] Keep generated `core/templui/generated_routes.go` contract
- [x] Implement `templui-map check`
- [x] Implement `templui-map suggest` scanner helper

## Phase 5 - Integration Spec for jasonchiu-com4
- [x] Provide exact `buildybud.toml` baseline for `jasonchiu-com4`
- [x] Document Taskfile migration mapping from legacy `tools/*`
- [x] Update `/Users/jasonchiu/Documents/WVC/jasonchiu-com4/Taskfile.yml` to call `buildybud`
- [x] Add temporary fallback strategy for old `tools/*` commands (legacy tools kept in repo while Taskfile uses `buildybud`)

## Phase 6 - Hardening
- [x] Add golden-style tests for generated artifacts
- [x] Add CI matrix for module checks (`go test`, `go vet`)
- [x] Add `doctor` checks for config, paths, templui map, and `vips`
