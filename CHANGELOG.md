# Changelog - kwot

All notable changes to this project will be documented in this file.

## [1.0.13] - 2026-06-03

### Added

- **Env var interpolation in `user_token`** — `workspace-rbac-user.yaml` now supports `${VAR_NAME}` and `$VAR_NAME` syntax in the `user_token` field. kwot resolves the reference from the environment at runtime, enabling CI/CD pipelines (e.g. GitHub Actions secrets) to inject tokens without pre-processing YAML files. If the referenced env var is unset, kwot logs a warning and falls back to a random UUID (closes #31).
- **Role comment support** — `workspace.yaml` RBAC entries now accept an optional `comment` field that is forwarded to Kong on role creation, making roles self-documenting in Kong Manager.

### Security

- Update Go toolchain from `go1.26.3` to `go1.26.4` to fix 2 Go standard library vulnerabilities:
  - **GO-2026-5039**: Arbitrary inputs included in errors without escaping in `net/textproto`
  - **GO-2026-5037**: Inefficient candidate hostname parsing in `crypto/x509`

### Changed

- `go.mod` toolchain pin updated from `go1.26.3` to `go1.26.4`
- `Dockerfile` builder stage updated from `golang:1.26.3-alpine` to `golang:1.26.4-alpine`

## [1.0.12] - 2026-05-26

### Security

- Update Go toolchain from `go1.26.2` to `go1.26.3` to fix 2 Go standard library vulnerabilities (closes #24):
  - **GO-2026-4971**: Panic in `Dial` and `LookupPort` when handling NUL byte on Windows in `net`
  - **GO-2026-4918**: Infinite loop in HTTP/2 transport when given bad `SETTINGS_MAX_FRAME_SIZE` in `net/http`

- Bump `fast-uri` (transitive npm dev dependency via `@commitlint/cli`) from `3.1.0` → `3.1.2` to fix:
  - **CVE-2026-6321** (GHSA-q3j6-qgpj-74h6): Path traversal via percent-encoded dot segments in URI normalization
  - **CVE-2026-6322** (GHSA-v39h-62p7-jpjc): Host confusion via percent-encoded authority delimiters

### Changed

- `go.mod` toolchain pin updated from `go1.26.2` to `go1.26.3`
- `Dockerfile` builder stage updated from `golang:1.26.2-alpine` to `golang:1.26.3-alpine`
- `package-lock.json` updated to resolve `fast-uri` to `3.1.2`

## [1.0.11] - 2026-04-15

### Fixed

- **Workspace deletion timeout on large deployments** — cascade delete (`DELETE /workspaces/{id}?cascade=true`) now respects the new `KONG_REQUEST_TIMEOUT` env var (default 30 s) instead of the previous hardcoded 10-second HTTP client timeout. On clusters with many entities per workspace the deletion was succeeding server-side (Kong returned HTTP 204) but kwot had already given up and reported `context deadline exceeded`. Increase `KONG_REQUEST_TIMEOUT` in `.env` to give Kong enough time (e.g. `120` for 500+ entity workspaces).
- **Plugin 404 immediately after workspace creation on multi-node CP clusters** — on deployments with multiple Kong Control Plane nodes, a newly created workspace is written to the database immediately but CP nodes rebuild their routing cache asynchronously. Plugin POST requests load-balanced to a node whose cache has not yet caught up returned `404 Workspace not found` even though the workspace existed. kwot now retries plugin creation on 404 responses with a 200 ms × attempt backoff (controlled by `MAX_RETRY_ATTEMPTS`, default 5).
- **Misleading error hint on delete timeout** — the error message for a timed-out cascade delete previously appended `"cascade=true requires Kong Gateway 3.4.0+; verify your Kong version"`, which was incorrect when the actual cause was a timeout. The message now distinguishes timeout errors and instructs the operator to raise `KONG_REQUEST_TIMEOUT`.

### Added

- New env var `KONG_REQUEST_TIMEOUT` (integer, seconds, default `30`) — configures the HTTP client timeout for all Kong Admin API calls. Previously hardcoded to 10 s; raising it is required for reliable cascade deletion of large workspaces.

### Changed

- `.env.example` and README updated to document `KONG_REQUEST_TIMEOUT` and the multi-node plugin retry behaviour.

## [1.0.10] - 2026-04-10

### Security

- Update Go toolchain from 1.26.1 to 1.26.2 to fix 4 Go standard library vulnerabilities:
  - **GO-2026-4947**: Unexpected work during chain building in `crypto/x509`
  - **GO-2026-4946**: Inefficient policy validation in `crypto/x509`
  - **GO-2026-4870**: Unauthenticated TLS 1.3 KeyUpdate record causes DoS in `crypto/tls`
  - **GO-2026-4866**: Case-sensitive `excludedSubtrees` name constraints auth bypass in `crypto/x509`

### Changed

- `go.mod` continues to declare `go 1.26` and now pins `toolchain go1.26.2` for reproducible patch-level builds
- All CI workflows (`ci.yml`, `release.yml`, `scheduled-security-scan.yml`) now use `go-version-file: go.mod` instead of a hardcoded version string — future Go upgrades only require a single change in `go.mod`
- `Dockerfile` builder stage updated from `golang:1.24.12-alpine` to `golang:1.26.2-alpine`
- `dependabot.yml` remains in report-only mode (`open-pull-requests-limit: 0`) so dependency issues are surfaced without opening automated PRs

## [1.0.9] - 2026-04-05

### Added

- Optional `user_token` field in `workspace-rbac-user.yaml` — when set, kwot uses the provided value as the RBAC token for that user instead of generating a random UUID
  - Use case: CI pipelines, service accounts, or any scenario requiring a predictable pre-shared RBAC token
  - When omitted, existing behavior is preserved (a random UUID is generated at creation time and never stored)
  - A debug log line confirms the configured token was used after a successful create
  - **Note:** `user_token` is only applied on initial creation. If the user already exists (409), the token is **not** reconciled — the warning log will say `configured user_token was not applied` in that case; delete and recreate the user to change its token

### Changed

- `RBACUser` model now has `yaml:"user_token,omitempty"` tag so the field is correctly read from YAML config files

## [1.0.8] - 2026-04-01

### Fixed

- Workspace deletion now succeeds when the workspace has the Dev Portal enabled
  - Root cause: the `/partials` endpoint used in previous versions does not exist in Kong 3.4+; Dev Portal files live under `/files` and were never cleaned up, causing a `400 Bad Request` on the final workspace DELETE
  - Fix: workspace deletion now uses `DELETE /workspaces/{id}?cascade=true` (Kong 3.4.0+) which atomically removes the workspace and all its child resources — including Dev Portal files — in a single API call

### Changed

- `DeleteWorkspace` simplified from a 22-step manual entity cleanup sequence to two steps: cascade workspace delete followed by group-role assignment cleanup
- Group-role assignment cleanup now runs **after** the cascade delete succeeds, preventing partial state where the workspace still exists but its group mappings have already been stripped
- Removed ~1,400 lines of individual entity deletion helpers (`deleteAllWorkspace*`, `deleteWorkspaceRoles`, `deleteWorkspaceUsers`, `countWorkspaceEntities`, `debugLogRemainingEntities`) — all superseded by cascade

### Added

- Minimum Kong Gateway Enterprise version documented as **3.4.0** in README (required for `cascade=true` workspace deletion)
- README: new `## Requirements` section with Kong and Go version requirements
- README: new `### Workspace deletion and Dev Portal` section explaining cascade behavior

## [1.0.7] - 2026-03-27

### Added

- Auto-cleanup of group-role assignments and empty groups during workspace deletion
  - When deleting a workspace, kwot now removes all group-role mappings for that workspace (Step 0)
  - Groups that become fully empty after removal are automatically deleted
  - Multi-workspace groups retain their role mappings for other workspaces
- Per-workspace `groups-and-roles.yaml` support — place a `groups-and-roles.yaml` inside a workspace directory and it takes priority over the global file
- Configuration File Reference section in README documenting all YAML schemas (`workspace.yaml`, `workspace-rbac-user.yaml`, `groups-and-roles.yaml`)
- `IsDebugEnabled()` helper in logger to gate expensive debug-only API calls
- Workspace deletion now cleans up additional Kong Enterprise entity types:
  - Custom plugin schemas (`/custom-plugins`)
  - Standalone DeGraphQL routes (`/degraphql_routes`)
  - OIDC JWK sets (`/oic_jwks`)
  - Dev Portal partials (`/partials`)

### Fixed

- `ProcessRoles` now validates that the workspace exists before proceeding, preventing silent success for nonexistent workspaces
- `errors.Is(err, os.ErrNotExist)` used throughout group config loading to correctly unwrap wrapped errors from `fmt.Errorf("%w", ...)`
- `countWorkspaceEntities` returns -1 on any pagination error to prevent acting on partial/unreliable counts
- `debugLogRemainingEntities` gated behind `IsDebugEnabled()` to avoid unnecessary Kong API calls on non-verbose runs
- Global `groups-and-roles.yaml` in all-mode now filters roles per workspace rather than skipping entire groups

## [1.0.6] - 2026-02-18

### Security

- Upgraded Go toolchain from 1.25.5 to 1.26.0, resolving two `crypto/tls` vulnerabilities:
  - **GO-2026-4340**: Handshake messages may be processed at the incorrect encryption level (fixed in Go 1.25.6+)
  - **GO-2026-4337**: Unexpected TLS session resumption (fixed in Go 1.25.7+)
- Updated `go.mod` to `go 1.26`
- Updated all CI/CD workflow Go versions (`ci.yml`, `release.yml`, `scheduled-security-scan.yml`) to `1.26`
- Pinned `golangci-lint-action` to `v6` with `v2.10.1` (built with Go 1.26) to fix linter version mismatch
- Verified clean `govulncheck ./...` output: `No vulnerabilities found`

### Added

- Added `make start` / `make stop` targets as convenience wrappers for `docker compose up/down` in the `perf-test` directory
- Added `make reset` target to reset Kong DB (migrations reset + bootstrap + container restart)
- Added `config/demo5` workspace configuration

## [1.0.5] - 2026-01-29

### Fixed

- Fixed PASSWORD authentication to properly handle Kong's 302 redirect response from `/auth` endpoint
- Session cookies are now correctly captured and used in subsequent authenticated requests
- PASSWORD auth now works correctly with Kong SSO/Manager

### Added

- Added `.env.auth.password.example` configuration file documenting PASSWORD auth setup
- Comprehensive unit tests for PASSWORD and RBAC authentication methods
- Tests validate redirect handling, header setup, and cookie management

## [1.0.4] - 2026-01-29

### Security

- Updated Dockerfile and CI/CD workflows to build with Go 1.24.12 for consistency
- Note: GO-2026-4340 (`crypto/tls` handshake encryption level vulnerability) was not fully resolved until Go 1.25.6+; fully addressed in v1.0.6

## [1.0.3] - 2026-01-27

### Fixed

- Improved error handling in availability checks to capture and return actual Kong API errors instead of generic timeout messages
- Corrected retry backoff terminology from "exponential" to "linear" throughout documentation and code comments
- Eliminated code duplication by centralizing `MaxRetryAttempts` configuration in `config.Config` instead of duplicating env parsing in processors
- Enhanced error diagnostics by wrapping GetJSON errors in returned error messages (using `%w` format specifier)

## [1.0.2] - 2026-01-27

### Added

- Added availability checks for role, workspace, and RBAC user creation to handle Kong Enterprise replication lag
- New `MAX_RETRY_ATTEMPTS` environment variable to configure retry attempts for resource availability checks (default: 5)
- Linear backoff strategy (50ms, 100ms, 150ms, 200ms, 250ms) for availability verification

### Fixed

- Fixed race condition where plugins fail to create immediately after workspace creation with "Workspace not found" error
- Fixed potential race condition where permissions fail to apply immediately after role creation
- Fixed potential race condition where role assignments fail immediately after RBAC user creation

## [1.0.1] - 2026-01-26

### Fixed

- Fixed Kong audit_log errors on GET/DELETE requests by only setting `Content-Type: application/json` header when request body is present


## [1.0.0] - 2026-01-06

Initial stable release of kwot - Kong Onboarding Control Tool.

### Added

- Infrastructure as Code for Kong configuration in YAML files
- Multi-workspace management for isolated Kong workspaces to manage teams/unit
- RBAC configuration with roles, users, and permissions
- Group management with workspace-specific role mappings
- Drift detection via `kwot diff` command
- Safe operations with dry-run mode and explicit `--force` flag for destructive operations
- Cross-platform support (macOS, Linux, Windows)
- Comprehensive workspace deletion with automated cleanup of all workspace-scoped resources
  - ACLs, credentials (basic-auth, hmac-auth, key-auth, JWT, oauth2, mtls-auth)
  - Services, routes, consumers, consumer groups, upstreams
  - Certificates, CA certificates, SNIs, vaults, plugins
  - RBAC roles, admin users, and RBAC users
  - Group-role associations for workspaces
- Pagination support for handling 100+ groups across all pages
- Dry-run mode (`--dry-run`) to preview changes without applying
- Explicit `--force` flag required for destructive operations
- Operator confirmation for deletions
- Protected resources mechanism to prevent accidental deletion
- 5-second countdown confirmation for bulk operations
- `kwot apply` - Apply configuration to Kong
- `kwot diff` - Show configuration drift
- `kwot delete` - Delete resources (requires `--force` flag)
- Global flags: `--dry-run`, `--verbose`, `--quiet`, `-w/--workspace`
- [CHEATSHEET.md](CHEATSHEET.md) with complete command reference