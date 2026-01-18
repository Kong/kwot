# Changelog - kwot

All notable changes to this project will be documented in this file.

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