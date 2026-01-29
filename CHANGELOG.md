# Changelog - kwot

All notable changes to this project will be documented in this file.

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

- Verified and recommend building with Go 1.24.12, which patches critical TLS vulnerability GO-2026-4340 in the crypto/tls package
- Updated Dockerfile and CI/CD workflows to build with Go 1.24.12 for consistency

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