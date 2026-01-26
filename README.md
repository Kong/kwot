# kwot — Kong Workspace Onboarding Tool

[![Build Status](https://github.com/Kong/kwot/actions/workflows/ci.yml/badge.svg)](https://github.com/Kong/kwot/actions/workflows/ci.yml)
[![Version](https://img.shields.io/github/tag/Kong/kwot?label=latest%20version)](https://github.com/Kong/kwot/tags)

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

**About `MAX_RETRY_ATTEMPTS`:** Handles Kong Enterprise replication lag where API calls return success (201) but resources aren't immediately available for dependent operations. Uses exponential backoff (50ms, 100ms, 150ms, 200ms, 250ms) to verify workspace, role, and RBAC user creation before proceeding with dependent operations.

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