# jasonchiu-com4 Migration Notes

## Applied Task Mappings
- `task css` now calls `buildybud manifest --input ... --logical ...` after Tailwind build.
- `task js` now calls `buildybud js`.
- `task templui-map` now calls `buildybud templui-map generate`.
- `task images` now calls `buildybud images`.
- `task cdn-purge-bunny` can now call `buildybud cdn plan-and-purge`.

## Added Files in jasonchiu-com4
- `buildybud.toml` (single source of truth for build settings)

## Rollback Plan
1. Revert `Taskfile.yml` command lines back to `go run ./tools/*` invocations.
2. Keep `buildybud.toml` in place (safe to leave unused) or remove it.
3. Continue using existing `tools/*` binaries/scripts until a fixed `buildybud` version is installed.
