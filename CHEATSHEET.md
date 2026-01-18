# kwot Command Cheatsheet

Quick reference for all kwot commands, flags, and environment variables.

## Global Flags

| Flag | Purpose | Example |
|------|---------|---------|
| `-w, --workspace <name>` | Target workspace (default: all) | `kwot apply -w demo1` |
| `--dry-run` | Preview without applying | `kwot apply --dry-run` |
| `-v, --verbose` | Detailed output | `kwot apply -v` |
| `-q, --quiet` | Suppress output | `kwot apply -q` |
| `--config <path>` | Custom config file | `kwot apply --config prod.env` |
| `--version` | Show version | `kwot --version` |
| `--help` | Show help | `kwot --help` |

## Commands

### Apply Configuration
```bash
kwot apply                                    # Apply everything (all workspaces)
kwot apply workspaces                        # Create/update workspaces only
kwot apply roles                             # Create/update roles only
kwot apply groups                            # Create/update groups only
kwot apply -w demo1                          # Apply to specific workspace
kwot apply --dry-run                         # Preview changes first
kwot apply --dry-run -v                      # Preview with details
kwot apply workspaces --dry-run --force      # Preview workspace creation with dry-run
kwot apply roles -v --quiet                  # Apply roles with verbose but quiet output (verbose takes precedence)
kwot apply groups -w staging --dry-run       # Preview group changes for specific workspace
kwot apply --config prod.env                 # Use custom environment file
```

### Check Drift
```bash
kwot diff                                    # Show all drifts
kwot diff workspaces                        # Workspace drift only
kwot diff roles                             # Role drift only
kwot diff groups                            # Group drift only
kwot diff -w demo1                          # Drift for specific workspace
kwot diff -v                                # Show drift with details
kwot diff --workspace staging -v            # Verbose drift for specific workspace
kwot diff roles -w demo1                    # Role drift for specific workspace
```

### Delete Resources
```bash
kwot delete workspaces -n ws1 --force            # Delete specific workspace
kwot delete roles -n admin-role --force          # Delete specific role
kwot delete groups -n group1 --force             # Delete specific group
kwot delete --force                              # Delete ALL (requires feature flags)
kwot delete workspaces -n ws1 --force --dry-run # Preview workspace deletion
kwot delete --force -v                           # Delete all with verbose output
kwot delete -w demo1 --force                     # Delete all resources in workspace
kwot delete workspaces -n ws1 -w demo1 --force  # Delete workspace from specific workspace context
```

### Global Options
```bash
kwot apply --help                             # Show help for apply command
kwot delete --help                            # Show help for delete command
kwot diff --help                              # Show help for diff command
kwot --version                                # Show kwot version
kwot --help                                   # Show general help
```

## Environment Variables

### Connection
```bash
KONG_ADDR=http://localhost:8001
AUTH_METHOD=RBAC                    # RBAC, PASSWORD, or COOKIE
ADMIN_TOKEN=your-token
SSL_VERIFY=true
```

### Configuration
```bash
CONFIG_DIR=./config/
```

### Feature Flags
```bash
FEATURE_CREATE_RBAC_USERS=true
FEATURE_DELETE_EXISTING_RBAC_USERS=true
FEATURE_DELETE_EXISTING_ROLES=true
FEATURE_FORCE_WIPE_WORKSPACE=false  # Enable to allow workspace deletion
FEATURE_DELETE_ALL_ENABLED=false    # Enable to allow delete all
```

### Performance
```bash
MAX_CONCURRENT_WORKSPACES=5         # Increase for faster processing
```

## Common Patterns

**Setup new environment:**
```bash
kwot diff                    # Check current state
kwot apply --dry-run         # Preview changes
kwot apply                   # Apply everything
```

**Update specific workspace:**
```bash
kwot diff workspaces -w ws-1
kwot apply -w ws-1 --dry-run
kwot apply -w ws-1
```

**Add new role:**
```bash
kwot apply roles -n admin-role --dry-run
kwot apply roles -n admin-role
```

**Detect drift:**
```bash
kwot diff                    # See all changes
kwot diff workspaces -v      # Detailed workspace drift
```

**Safe production deployment:**
```bash
kwot diff -v > before.log
kwot apply --dry-run -v > changes.log
# Review both logs carefully
kwot apply --quiet           # Apply silently
```

**Delete everything (test environment):**
```bash
# Requires feature flags enabled in .env:
# FEATURE_DELETE_ALL_ENABLED=true
# FEATURE_FORCE_WIPE_WORKSPACE=true

kwot delete --dry-run --force -v    # Preview first
kwot delete --force                 # Delete
```

## Flag Combinations

```bash
kwot apply workspaces -w ws-1 --dry-run    # Preview workspace to prod
kwot apply roles -n admin -w ws-1 -v       # Apply admin role to prod (verbose)
kwot delete roles -n old-role --force      # Delete role from all workspaces
kwot diff -w staging                        # Check staging drift only
```

## Performance Tips

For large deployments (100+ workspaces), increase concurrency:
```bash
# .env
MAX_CONCURRENT_WORKSPACES=20        # Default is 5, increase for parallelization
```

Use workspace-specific operations when possible:
```bash
kwot apply -w specific-ws        # Much faster than all workspaces
kwot diff -w staging             # Faster drift detection
```

## See Also

- [README](README.md) - Overview and features
- [QUICKSTART](QUICKSTART.md) - Installation and setup guide
- [MIGRATION_FROM_NODEJS](MIGRATION_FROM_NODEJS.md) - Upgrade from Node.js version
- [CHANGELOG](CHANGELOG.md) - Release history
