# CLAUDE.md

This file gives LLM agents a practical working guide for the `kwot` repository.

## Project Identity

- Name: `kwot` (Kong Workspace Onboarding Tool)
- Language: Go
- Type: single-binary CLI (Cobra-based)
- Purpose: manage Kong Enterprise workspace onboarding via YAML-driven Infrastructure as Code
- Scope: workspaces, RBAC roles, RBAC users, groups, workspace-level plugins, drift detection, deletion
- Repo: https://github.com/Kong/kwot

## What This Tool Solves

`kwot` removes manual Kong Manager setup for platform teams.

It automates:
- Workspace creation and updates
- Role creation and permission assignment
- RBAC user creation
- Group creation and role mapping per workspace
- Drift detection (`kwot diff`)
- Safe teardown (`kwot delete ... --force` with feature flags)

It is intentionally focused on workspace/admin onboarding and RBAC. For API-layer entities (services/routes/plugins at scale), use decK.

## High-Level Flow

Entry point:
- `main.go` sets build-time `Version` and calls `cmd.Execute()`.

Main command pipeline:
1. `kwot apply` (default applies all)
2. Workspaces processed first
3. Roles processed second
4. RBAC users applied after roles
5. Groups processed last

Why this order matters:
- Groups need roles and workspaces to already exist
- RBAC users depend on roles existing

## Command Surface

Defined in `cmd/`:
- `root.go`: global flags, config init, logging mode, version/banner
- `all.go`: `apply` command family
- `diff.go`: drift detection (`all|workspaces|roles|groups`)
- `delete.go`: destructive flows with safety checks and required `--force`

Global flags:
- `--workspace, -w` (default `all`)
- `--dry-run`
- `--verbose`
- `--quiet`
- `--config` for env file path

## Configuration Model

Loaded from env/.env in `internal/config/config.go`.

Important env vars:
- `KONG_ADDR` (default `http://localhost:8001`)
- `AUTH_METHOD` (`RBAC` or `PASSWORD`)
- `ADMIN_TOKEN` (required for RBAC)
- `ADMIN_USER` + `BASE64_UID_PWD` (required for PASSWORD)
- `CONFIG_DIR` (default `./config/`)
- `MAX_CONCURRENT_WORKSPACES` (default `5`)
- `MAX_RETRY_ATTEMPTS` (default `5`)

Feature flags:
- `FEATURE_DELETE_EXISTING_USERS`
- `FEATURE_DELETE_EXISTING_ROLES`
- `FEATURE_CREATE_RBAC_USERS`
- `FEATURE_DELETE_EXISTING_RBAC_USERS`
- `FEATURE_FORCE_WIPE_WORKSPACE`
- `FEATURE_DELETE_ALL_ENABLED`

## Files and Data Layout

Expected config structure under `CONFIG_DIR`:
- `groups-and-roles.yaml` — **optional** global group definitions and role mappings (fallback)
- `<workspace>/groups-and-roles.yaml` — **optional** per-workspace group definitions (takes priority over global)
- `<workspace>/workspace.yaml` for workspace/rbac/plugin definitions
- `<workspace>/workspace-rbac-user.yaml` for workspace RBAC users
- shared permission files like `tenant-user.yaml`, `tenant-release-user.yaml`

`groups-and-roles.yaml` lookup precedence (workspace-local wins):
- For `-w <workspace>`: checks `<CONFIG_DIR>/<workspace>/groups-and-roles.yaml` first, then `<CONFIG_DIR>/groups-and-roles.yaml`
- For `all`: each workspace dir is scanned — per-workspace file is used if present; the global file fills in any workspaces that lack a local file
- Both formats are supported in either location: direct array or `role_info`+`config` structured format

Processor ownership:
- `internal/workspace/processor.go`
- `internal/roles/processor.go`
- `internal/groups/processor.go`

Model types:
- `internal/models/`

Validation:
- `internal/validation/validator.go`

## Kong Client Behavior

`internal/kong/client.go`:
- wraps HTTP methods (`GET`, `POST`, `PATCH`, `DELETE`)
- supports RBAC token and PASSWORD auth
- for PASSWORD auth, acquires cookies via `/auth`
- supports custom CA bundle, TLS verify toggle, and proxy
- normalizes Kong API errors into clearer messages

## Concurrency and Pagination

Concurrency:
- workspace and role processing use bounded worker patterns
- concurrency controlled by `MAX_CONCURRENT_WORKSPACES`
- some per-role/per-group operations use additional semaphores

Pagination:
- list operations are paginated (`size=1000`, `offset` loop)
- used for workspaces, groups, and roles where applicable

## Safety Model

Destructive commands require `--force`.
Additional delete capabilities require feature flags.

Examples:
- workspace delete requires `FEATURE_FORCE_WIPE_WORKSPACE=true`
- `delete all` requires `FEATURE_DELETE_ALL_ENABLED=true`

`default` workspace is protected from deletion.

## Development Commands

Common Make targets:
- `make deps`       download/tidy modules
- `make fmt`        format Go code
- `make lint`       run golangci-lint
- `make test`       run unit tests
- `make test-coverage`
- `make build`      build `bin/kwot`
- `make build-all`  cross-platform builds

Recommended local validation sequence before merging:
1. `make fmt`
2. `make lint`
3. `make test`
4. `make build`

## Testing and QA Notes

Test files exist in:
- `cmd/*_test.go`
- `internal/**/_test.go`
- shell-level tests in `tests/`

When changing processors or CLI flow, prioritize:
- dry-run behavior
- workspace-scoped filtering (`-w`)
- error clarity (especially role/group assignment failures)
- backward compatibility of YAML formats

## Editing Guidance for LLM Agents

When making changes:
- Preserve command behavior and apply order unless explicitly requested
- Keep safety checks intact for delete paths
- Maintain dry-run semantics (no live mutation in dry-run)
- Avoid introducing breaking schema changes in YAML unless required
- Prefer minimal targeted patches over broad refactors

If touching group-role mapping logic, verify:
- role names referenced in `groups-and-roles.yaml` actually exist in target workspace RBAC
- group names can be arbitrary, but assigned role must exist in workspace context
- `loadGroupConfig` resolves per-workspace file first, global fallback second — do not break this precedence
- `parseGroupConfigFile` handles both YAML formats; keep it as the single parsing entry point

## Common Pitfalls

- Using built-in role names (`admin`, `readonlyrole`) in a workspace that defines only tenant roles (`Tenant-User`, etc.)
- Forgetting required auth env vars for chosen auth mode
- Expecting group deletion to be workspace-scoped (groups are global in Kong)
- Running destructive commands without enabling required feature flags

## Quick Operator Examples

Preview changes:

```bash
kwot apply --dry-run
```

Apply all:

```bash
kwot apply
```

Apply only roles for one workspace:

```bash
kwot apply roles -w demo1
```

Diff everything:

```bash
kwot diff
```

Delete one workspace (if feature enabled):

```bash
kwot delete workspaces -n demo1 --force
```

---

If you are an LLM agent, start by reading:
1. `README.md`
2. `cmd/root.go`
3. `cmd/all.go`
4. `internal/config/config.go`
5. relevant processor in `internal/workspace`, `internal/roles`, or `internal/groups`

Then make the smallest change that satisfies the user request.
