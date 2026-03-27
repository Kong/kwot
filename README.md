# kwot — Kong Workspace Onboarding Tool

[![Build Status](https://github.com/Kong/kwot/actions/workflows/ci.yml/badge.svg)](https://github.com/Kong/kwot/actions/workflows/ci.yml)

A powerful, single-binary CLI for automating Kong Gateway workspace, group, role, and RBAC configuration management. Written in Go with zero runtime dependencies.

🚀 [Quick Start](QUICKSTART.md) • 📚 [Commands](CHEATSHEET.md) • 🔄 [Migration](MIGRATION_FROM_NODEJS.md) • 📋 [Changelog](CHANGELOG.md)

## Overview

`kwot` automates Kong Gateway configuration management using infrastructure-as-code principles. Define your complete team structure, access controls, and workspace setup in YAML files and apply them consistently across environments.

## What kwot Solves

**Platform teams automating workspace setup** — Define workspaces, users, roles, and permissions in YAML. No more manual clicking in Kong Manager.

**Permission management at scale** — Update a role's permissions once in your config; apply consistently across all workspaces with one command.

**Configuration drift detection** — Run `kwot diff` to see exactly what's different between your approved YAML and what's actually running in Kong.

## What kwot Manages

kwot specializes in **Workspace and RBAC configuration management** for Kong Gateway. It focuses on organizational structure, access control, and user management while providing comprehensive cleanup of workspace-scoped resources during deletion.

**Philosophy:** kwot is purpose-built for **admin and team onboarding**, not API configuration. It lets platform teams manage workspace isolation, user access, and permissions—so developers can focus on what matters: building APIs. For managing services, routes, plugins, and other API-related configuration, we recommend using [decK](https://github.com/Kong/deck).

### Why kwot When decK Exists?

**The Gap:** [decK](https://github.com/Kong/deck) is the excellent all-in-one Kong tool, but it lacks **RBAC groups and admins** management—critical for enterprise deployments. kwot fills this gap.

**The Vision:** We believe decK will eventually include all Kong configuration (platform + API layers). Until then, kwot provides specialized RBAC and workspace tooling. When decK gains these capabilities, consolidate into a single tool.

**How They Work Together:**
- **kwot:** Workspaces, RBAC roles/users/groups (admin onboarding)
- **decK:** Services, routes, consumers, certificates, plugins (API layer)
- **Combined:** Complete infrastructure-as-code for Kong

### Primary Entities (Workspace & RBAC)

| Entity | Managed |
|--------|---------|
| **Workspaces** | ✅ Create, update, delete with safe cleanup |
| **RBAC Roles** | ✅ Create, update, delete with endpoint permissions |
| **RBAC Users** | ✅ Create, update, delete (Super Admin, Admin, Plain User types) |
| **RBAC Groups** | ✅ Create, update, delete with group-to-role mappings |
| **Plugins** | ✅ Create, update, delete (workspace-level plugins in YAML) |

### Secondary Entities (Workspace-Scoped Cleanup)

When deleting workspaces, kwot automatically cleans up all workspace-scoped child resources to ensure complete workspace removal:

| Entity | Cleanup Support |
|--------|---------|
| **Services, Routes, Consumers** | ✅ Auto-deleted with workspace |
| **Plugins, Consumer Groups, ACLs** | ✅ Auto-deleted with workspace |
| **Certificates, SNIs, Upstreams** | ✅ Auto-deleted with workspace |
| **Credentials & Vaults** (basic-auth, key-auth, hmac-auth, jwt, oauth2, mtls-auth) | ✅ Auto-deleted with workspace |
| **Keys & Key Sets** (Kong Enterprise 3.1+) | ✅ Auto-deleted with workspace |
| **Group-role mappings & empty groups** | ✅ Auto-cleaned with workspace (see below) |

#### Group cleanup during workspace deletion

Kong groups are **global** — a single group can hold role mappings across multiple workspaces. When you delete a workspace, kwot handles group cleanup automatically as Step 0 of the deletion sequence:

1. **Role mappings for this workspace are removed** from every group that references it.
2. **Groups that become empty** (all their role mappings belonged to this workspace) are **deleted automatically**.
3. **Multi-workspace groups** that still have role mappings in other workspaces are **left untouched** — only the mappings for the deleted workspace are removed.

Example log output during `delete workspaces -n teamA --force`:
```
Step 0: Removing group-role assignments for workspace teamA
Removed role assignment from group 'teamA-admin-group' (workspace: teamA)
Group 'teamA-admin-group' has no remaining role assignments after workspace teamA removal — deleting group
Removed role assignment from group 'platform-admins' (workspace: teamA)
Group 'platform-admins' retains 2 role assignment(s) in other workspace(s) — group preserved
```

**Note:** For comprehensive API configuration management (services, routes, plugins at scale), use [decK](https://github.com/Kong/deck). kwot handles workspace-level plugin declarations; decK handles API-layer resource management.

### Key Features

**Drift Detection** — Compare your configuration against Kong's current state to identify manual changes or deployment drift.

**Safe Operations** — Dry-run mode for validation, explicit `--force` flag for destructive operations, and protected resources prevent accidental failures.

**Pagination Support** — Handle environments with 100+ groups, users, and resources across all pages.

## Bottom Line

Manually onboarding teams? Configurations drifting between environments? `kwot` turns that into config files and one command.

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `KONG_ADDR` | `http://localhost:8001` | Kong Admin API address |
| `AUTH_METHOD` | `RBAC` | Authentication method (RBAC or PASSWORD) |
| `ADMIN_TOKEN` | - | Admin API token for RBAC authentication |
| `CONFIG_DIR` | `./config/` | Directory containing configuration files |
| `MAX_CONCURRENT_WORKSPACES` | `5` | Number of workspaces to process in parallel |
| `MAX_RETRY_ATTEMPTS` | `5` | Retry attempts for resource availability checks during creation |

**About `MAX_RETRY_ATTEMPTS`:** Handles Kong Enterprise replication lag where API calls return success (201) but resources aren't immediately available for dependent operations. Uses linear backoff (50ms, 100ms, 150ms, 200ms, 250ms) to verify workspace, role, and RBAC user creation before proceeding with dependent operations.

## Table of Contents

1. [Installation](#installation)
2. [Quick Start](#quick-start)
3. [Configuration](#configuration)
4. [Features](#features)
5. [Documentation](#documentation)
6. [Disclaimer](#disclaimer)
7. [Acknowledgments](#acknowledgments)

## Installation

Get started with kwot in minutes:

```bash
# Download the latest binary
curl -L https://github.com/Kong/kwot/releases/latest/download/kwot-darwin-amd64 -o kwot
chmod +x kwot
./kwot --version
```

Or use Docker:

```bash
# Pull the latest image
docker pull Kong/kwot:latest

# Run with environment variables
docker run -it \
  -e KONG_ADDR=http://kong:8001 \
  -e AUTH_METHOD=RBAC \
  -e ADMIN_TOKEN=your-token \
  -v $(pwd)/config:/home/kwot/config \
  Kong/kwot:latest apply --dry-run
```

See [QUICKSTART.md](QUICKSTART.md) for detailed installation and 5-minute setup guide.

## Quick Start

```bash
kwot apply --dry-run   # Preview changes
kwot apply             # Apply configuration  
kwot diff              # Check for drift
```

See [CHEATSHEET.md](CHEATSHEET.md) for all commands and flags.

## Configuration

```bash
KONG_ADDR=https://kong-admin.example.com
AUTH_METHOD=RBAC
ADMIN_TOKEN=<your-token>
CONFIG_DIR=./config/
```

See [CHEATSHEET.md](CHEATSHEET.md#environment-configuration) for all environment variables.

## Configuration File Reference

kwot reads three types of YAML files from your `CONFIG_DIR`. This section documents every field so you know exactly what to write.

---

### `workspace.yaml` — Workspace, Roles, and Plugins

One file per workspace directory. Defines the workspace settings, the RBAC roles that exist inside it, and any workspace-level plugins to apply.

```yaml
# workspace config settings
config:
  portal: false           # true | false — enable Dev Portal for this workspace

# RBAC roles to create inside this workspace
rbac:
  - role: readonlyrole    # role name (string, must be unique within the workspace)
    permissions:
      - endpoint: "*"     # Kong endpoint pattern — "*" means all endpoints
        negative: false   # false = ALLOW, true = DENY
        actions: "read"   # comma-separated: read, create, update, delete

  - role: admin
    permissions:
      - endpoint: "*"
        negative: false
        actions: "create,update,read,delete"
      - endpoint: "/rbac/*"   # deny RBAC management to prevent privilege escalation
        negative: true
        actions: "create,update,read,delete"

# Workspace-level plugins (optional)
plugins:
  - name: "file-log"          # Kong plugin name
    config:
      path: /dev/stdout       # plugin-specific config keys
      reopen: false

  - name: "rate-limiting-advanced"
    config:
      limit: [10, 20]
      window_size: [60, 120]
      strategy: redis
      redis:
        host: redis.svc.cluster.local
        port: 6379
      sync_rate: 1
```

**Notes:**
- `permissions` can reference a file path instead of an inline array (for shared permission sets).
- `plugins` is optional — omit the key entirely if no plugins are needed.
- Roles defined here are what you reference in `groups-and-roles.yaml`.

---

### `workspace-rbac-user.yaml` — Workspace RBAC Users

One file per workspace directory. Defines workspace-scoped RBAC users and the roles they are assigned.

```yaml
- name: workspace-admin-user   # RBAC username (string)
  roles:
    - admin                    # role name — must exist in this workspace's rbac: block

- name: readonly-svc-account
  roles:
    - readonlyrole
```

**Notes:**
- Each entry creates one RBAC user and assigns it the listed roles within the workspace.
- Roles must already exist (kwot applies roles before users).
- This file is **optional** — omit it if you don't need workspace-scoped RBAC users.
- Feature-flagged: requires `FEATURE_CREATE_RBAC_USERS=true` to take effect.

---

### `groups-and-roles.yaml` — Group-to-Role Mappings

Maps IdP groups (LDAP/OIDC) to Kong RBAC roles across workspaces. Supports two formats.

**Format 1 — Direct array (recommended for simple setups):**

```yaml
- group_name: platform-admins        # IdP group name (string)
  group_comment: "Platform admin team"  # optional description
  roles:
    - workspace: demo1               # workspace name
      role: admin                    # role name — must exist in that workspace
    - workspace: demo2
      role: admin

- group_name: platform-readonly
  group_comment: "Read-only access across all workspaces"
  roles:
    - workspace: demo1
      role: readonlyrole
    - workspace: demo2
      role: readonlyrole
```

**Format 2 — Structured with YAML anchors (for DRY role name reuse):**

```yaml
# Define role name aliases once, reuse with YAML anchors
role_info:
  wk_admin: &wk_admin admin
  readonly:  &readonly readonlyrole

# The actual group definitions — same schema as Format 1
config:
  - group_name: platform-admins
    group_comment: "Platform admin team"
    roles:
      - workspace: demo1
        role: *wk_admin          # expands to "admin"
      - workspace: demo2
        role: *wk_admin

  - group_name: platform-readonly
    roles:
      - workspace: demo1
        role: *readonly          # expands to "readonlyrole"
```

**Notes:**
- Both formats produce identical results — choose whichever suits your team.
- Different workspace files can use different formats; they don't need to match.
- `group_name` must match the exact IdP group name configured in Kong.
- `group_comment` is optional.
- A single group can map to roles in multiple workspaces (multiple entries under `roles:`).

---

### File Layout Summary

```
CONFIG_DIR/
  groups-and-roles.yaml          # optional — global fallback for all workspaces
  <workspace>/
    workspace.yaml               # required — workspace config, roles, plugins
    workspace-rbac-user.yaml     # optional — workspace RBAC users
    groups-and-roles.yaml        # optional — per-workspace groups (overrides global)
```

Apply order: **workspaces → roles → RBAC users → groups**. Each step depends on the previous one existing in Kong.

---

## Group Configuration Layout

`groups-and-roles.yaml` can live either at the config root (global) or inside a workspace subdirectory (per-workspace). Per-workspace takes priority.

**Option A — Global file (single file for all workspaces):**
```
config/
  groups-and-roles.yaml   # all workspaces' groups defined here
  teamA/
  teamB/
```

**Option B — Per-workspace files (one file per workspace, no global):**
```
config/
  teamA/
    groups-and-roles.yaml   # teamA groups only
    workspace.yaml
  teamB/
    groups-and-roles.yaml   # teamB groups only
    workspace.yaml
```

**Option C — Mixed (per-workspace overrides, global as fallback):**
```
config/
  groups-and-roles.yaml     # fallback for workspaces without a local file
  teamA/
    groups-and-roles.yaml   # teamA uses this, global is ignored for teamA
  teamB/                    # teamB falls back to global
    workspace.yaml
```

Both YAML formats are supported in any location:
- **Direct array:** `- group_name: ...`
- **Structured (with `role_info` anchors):** `role_info: {...}` + `config: [...]`

## Performance Tuning: Concurrency

kwot processes workspaces concurrently for faster execution. Control parallelism with `MAX_CONCURRENT_WORKSPACES`:

```bash
# Conservative: Single workspace at a time (slowest, safest)
MAX_CONCURRENT_WORKSPACES=1 kwot apply

# Balanced: Default - 5 concurrent workspaces (recommended)
kwot apply  # No env var needed

# Aggressive: 20 concurrent workspaces (for medium deployments)
MAX_CONCURRENT_WORKSPACES=20 kwot apply

# Maximum: 100 concurrent workspaces (for large deployments with 8+ CPU cores)
MAX_CONCURRENT_WORKSPACES=100 kwot apply
```

### Tuning Guide

**Choose based on:**
- **Machine resources** (CPU cores, available memory)
- **Network bandwidth** (parallel API calls)

| Scenario | Setting | Why |
|----------|---------|-----|
| **Local testing** | 1-2 | Single Kong instance, limited bandwidth |
| **Small deployments** (< 50 workspaces) | 5 (default) | Good balance of speed and safety |
| **Medium deployments** (50-500 workspaces) | 10-20 | Uses available CPU/network efficiently |
| **Large deployments** (500+ workspaces) | 50-100 | Maximize throughput on enterprise Kong clusters |
| **Limited resources** (shared environment) | 1-5 | Don't overwhelm Kong API or network |

**Performance impact (measured on 500 workspace deployment):**
```
MAX_CONCURRENT=1   → ~30 minutes (sequential)
MAX_CONCURRENT=5   → ~6 minutes
MAX_CONCURRENT=20  → ~5 minutes
MAX_CONCURRENT=50  → ~5 minutes (diminishing returns due to network/CPU limits)
```

⚠️ **Note:** Maximum is 100 (safety boundary). Most deployments work best with 5-50. Only use 100+ for large clusters with 8+ CPU cores and sufficient network bandwidth.

## Features

✅ **Infrastructure as Code** — YAML-based configuration  
✅ **Multi-Workspace** — Manage many Kong workspaces  
✅ **Drift Detection** — `kwot diff` for configuration tracking  
✅ **Safety First** — Dry-run and explicit `--force` flags  
✅ **Performance** — 5-10x faster than Node.js version  
✅ **Idempotent** — Safe to run repeatedly  
✅ **Comprehensive Workspace Deletion** — Automatically clean up all workspace-scoped resources (services, routes, consumers, credentials, plugins, roles, RBAC users, vaults, SNIs, certificates, upstreams, etc.) before workspace deletion  
✅ **Pagination Support** — Handle environments with thousands of groups, users, and resources

## Documentation

- [QUICKSTART.md](QUICKSTART.md) — Setup guide  
- [CHEATSHEET.md](CHEATSHEET.md) — Command reference  
- [MIGRATION_FROM_NODEJS.md](MIGRATION_FROM_NODEJS.md) — Upgrade from Node.js  
- [CONTRIBUTING.md](CONTRIBUTING.md) — Development setup
- [BOM.md](BOM.md) — Dependencies and vulnerabilities  

## License

Licensed under the Apache License 2.0. See [LICENSE](LICENSE) for details.

## ⚠️ Disclaimer

This code is the result of "vibe coding" and should be tested thoroughly in a non-production environment before using in production. While all code quality checks pass (linting, testing, race detection), please verify it works correctly for your specific Kong setup before deploying to production.

## Acknowledgments

kwot is inspired by the original [Kong workspace-config-apply-nodejs](https://github.com/Kong/workspace-config-apply-nodejs) project. We thank the original authors for introducing infrastructure-as-code for Kong configuration management.