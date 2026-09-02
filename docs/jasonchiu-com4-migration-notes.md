# jasonchiu-com4 Migration Notes

## Applied Task Mappings
- `task css` now calls `buildybud manifest --input ... --logical ...` after Tailwind build.
- `task js` now calls `buildybud js`.
- `task templui-map` now calls `buildybud templui-map generate`.
- `task images` now calls `buildybud images`.
- `task cdn-purge-cloudflare` calls `buildybud cdn plan-and-purge` with route inventory read from `buildybud.toml`.

## Added Files in jasonchiu-com4
- `buildybud.toml` (single source of truth for build settings)

## Rollback Plan
1. Pin the previous known-good Buildybud version in deployment and local setup.
2. Revert the matching `Taskfile.yml` and `buildybud.toml` changes together.
3. Do not restore deleted local tools unless the previous Buildybud release cannot cover the required operation.
