# Changelog - kwot

All notable changes to this project will be documented in this file.

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