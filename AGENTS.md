# AGENTS.md — plugin-unit

Standalone plugin repo for the `unit` typed-step verb (`verb:unit`). The plugin
is a Go module at `candy/plugin-unit/` (module path
`github.com/opencharly/plugin-unit/candy/plugin-unit`); the root `charly.yml`
only declares `discover: candy` so the repo is a project and its candy is
scanned.

Canonical files:

- `candy/plugin-unit/charly.yml` — the `plugin-unit:` candy entity (`plugin:`
  block, `plan:` check).
- `candy/plugin-unit/plugin.go` — the verb implementation (`CheckVerbProvider` +
  `ProvisionActor`) + `NewMeta()`.
- `candy/plugin-unit/schema/unit.cue` — the self-contained `#UnitInput`.
- `.github/workflows/tag-on-merge.yml` — CalVer tag + `CHANGELOG/` on merge.
- `README.md` — user overview only; never agent guidance.

## Load these skills first (R0)

- `/charly-internals:plugin` — the plugin authoring reference: the `plugin:`
  block, the unified Provider model, the typed-step contract, the per-plugin
  CUE-schema contract, placement. Load before touching the provider or schema.
- `/charly-core:service` — the service lifecycle surface this verb extends.
- `/charly-internals:git-workflow` — before any git/PR action.

## Build / validate / test

- `go build ./...` in `candy/plugin-unit/` — compile the plugin module.
- `go test ./...` in `candy/plugin-unit/` — the plugin's Go tests
  (`plugin_test.go`).
- `charly box validate` at the repo root — the structural check (the candy +
  `plugin:` block, CUE schema).
- The merge gate is the **org-wide** `charly/pr-validator` (required check
  `validate / validate`, defined in `opencharly/.github`); this repo has **no**
  per-repo candy gate.
- The `plan:` check probes `basic.target` through `systemctl` on a live
  deployment.

## Modify this repo

- Edit the `plugin-unit:` candy entity, the Go source, and `schema/unit.cue`
  **together** — the schema is the single source for the `params/` struct, so a
  field change not mirrored in the schema desyncs the generated types.
- This plugin is host-coupled on the SDK kit contract; keep the init a FIELD
  (`unit`), never a reserved word naming a concrete init.

## Landing

- PR-only. Every change lands through a pull request; the org-required
  `charly/pr-validator` validates the diff and body and arms native auto-merge on
  PASS. Direct pushes to `main` are blocked.
- History lives in `CHANGELOG/` (written by `tag-on-merge` at merge time); the PR
  body IS the changelog.
- The authoritative rulebook is the umbrella `AGENTS.md` in
  `opencharly/opencharly` and `charly/AGENTS.md` in the charly repo. Do not
  restate its rules here.
