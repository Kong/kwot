# kwot — Kong Workspace Onboarding Tool

[![CI](https://github.com/Kong/kwot/actions/workflows/ci.yml/badge.svg)](https://github.com/Kong/kwot/actions/workflows/ci.yml)

Automates Kong Gateway workspace, RBAC, and group setup from YAML. One binary, zero runtime dependencies.

---

## Requirements

| Requirement | Minimum version |
|-------------|----------------|
| Kong Gateway Enterprise | **3.4.0** |
| Go (to build from source) | **1.26** |

Kong Gateway 3.4.0 introduced the `cascade=true` flag on workspace deletion, which kwot uses to atomically remove a workspace and all its child resources (including Dev Portal files, RBAC roles, plugins, and more) in a single API call. Versions older than 3.4.0 are not supported.

---

## What

`kwot` is a CLI that applies your team's Kong configuration — workspaces, roles, users, and groups — from a set of YAML files. Run it once or in CI; it converges Kong to the declared state.

**What kwot manages:**
- Workspaces (create, update, delete)
- RBAC roles and endpoint permissions
- RBAC users per workspace
- LDAP/OIDC group-to-role mappings
- Workspace-level plugins
- Safe workspace deletion (cleans up all child resources)

**What kwot does NOT manage:** services, routes, consumers, certificates at scale — use [decK](https://github.com/Kong/deck) for that.

> **Why not just decK?** decK doesn't manage RBAC groups or admin users — the critical gap for enterprise team onboarding. kwot fills that gap. The two tools complement each other.

---

## Why

| Problem | kwot answer |
|---------|-------------|
| Manual workspace setup in Kong Manager | YAML → `kwot apply` |
| Permission drift between environments | `kwot diff` shows what's changed |
| Onboarding a new team takes hours | One PR to config, one command to apply |
| Workspace deletion leaves orphaned resources | Single cascade API call removes workspace + all child resources atomically |

---

## How It Works

```
YAML config files
      │
      ▼
┌─────────────┐     ┌──────────────────────────────────────┐
│ kwot apply  │────▶│ 1. Workspaces                        │
└─────────────┘     │ 2. Roles (per workspace)              │
                    │ 3. RBAC Users (depend on roles)       │
                    │ 4. Groups + role mappings             │
                    └──────────────────────────────────────┘
                                    │
                                    ▼
                           Kong Admin API
```

Order matters: each step depends on the previous one existing in Kong.

---

## Quick Start

**1. Install**

```bash
curl -L https://github.com/Kong/kwot/releases/latest/download/kwot-darwin-arm64 -o kwot
chmod +x kwot && sudo mv kwot /usr/local/bin/
kwot --version
```

**2. Configure**

```bash
# env/.env
KONG_ADDR=https://kong-admin.example.com
AUTH_METHOD=RBAC
ADMIN_TOKEN=<your-token>
CONFIG_DIR=./config/
```

**3. Set up your config**

```
config/
  groups-and-roles.yaml        # group → role mappings (global)
  my-workspace/
    workspace.yaml             # workspace + roles + plugins
    workspace-rbac-user.yaml   # RBAC users (optional)
    groups-and-roles.yaml      # per-workspace override (optional)
```

**4. Run**

```bash
kwot apply --dry-run   # preview
kwot apply             # apply
kwot diff              # check drift
```

---

## Config File Reference

### `workspace.yaml`

```yaml
config:
  portal: false

rbac:
  - role: admin
    permissions:
      - endpoint: "*"
        negative: false
        actions: "create,update,read,delete"

plugins:
  - name: file-log
    config:
      path: /dev/stdout
```

### `workspace-rbac-user.yaml`

```yaml
- name: workspace-admin-user
  roles:
    - admin
- name: ci-pipeline-user
  user_token: my-static-token   # optional — if omitted, a random UUID is generated
  roles:
    - workspace-admin
```

The `user_token` field is **optional**. When set, kwot will use that value as the RBAC token for the user. When omitted, a random UUID is generated at creation time and not stored. Use a static token when you need a predictable, pre-shared secret (e.g. for CI pipelines or service accounts).

### `groups-and-roles.yaml`

```yaml
- group_name: platform-admins
  roles:
    - workspace: my-workspace
      role: admin
```

Place this at the config root (global) or inside a workspace folder (takes priority over global).

---

## Key Commands

| Command | What it does |
|---------|-------------|
| `kwot apply` | Apply all config (workspaces → roles → users → groups) |
| `kwot apply -w <name>` | Apply one workspace only |
| `kwot apply --dry-run` | Preview changes, no mutations |
| `kwot diff` | Show drift between config and Kong |
| `kwot delete workspaces -n <name> --force` | Delete workspace + all child resources (cascade) |

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `KONG_ADDR` | `http://localhost:8001` | Kong Admin API address |
| `AUTH_METHOD` | `RBAC` | `RBAC` or `PASSWORD` |
| `ADMIN_TOKEN` | — | Token for RBAC auth |
| `ADMIN_USER` + `BASE64_UID_PWD` | — | Credentials for PASSWORD auth |
| `CONFIG_DIR` | `./config/` | Config files location |
| `MAX_CONCURRENT_WORKSPACES` | `5` | Parallel workspace processing |

---

## Safety

- `--dry-run` on all apply commands — no live mutations
- `--force` required for all delete operations
- Feature flags gate destructive capabilities (`FEATURE_FORCE_WIPE_WORKSPACE`, etc.)
- `default` workspace is protected from deletion

### Workspace deletion and Dev Portal

If a workspace has the Dev Portal enabled, it accumulates portal files (HTML pages, CSS, JS, images, email templates, YAML config). These are not visible in the standard entity counts shown by most Admin API endpoints, but they block workspace deletion.

kwot uses `DELETE /workspaces/{id}?cascade=true` (Kong 3.4.0+) which removes the workspace and **all** its child resources — including Dev Portal files — in a single atomic operation. No manual pre-cleanup is needed.

---

## License

Apache 2.0 — see [LICENSE](LICENSE).

> **Disclaimer:** Test in a non-production environment before deploying. Verify behavior against your specific Kong setup.
