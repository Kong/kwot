# kwot Quick Start Guide

Get up and running with kwot in 5 minutes.

## Installation


### Option 1: Build from source

```bash
git clone https://github.com/Kong/kwot.git
cd kwot
make build
sudo mv bin/kwot /usr/local/bin/
```

### Option 2: Download binary

```bash
curl -L https://github.com/Kong/kwot/releases/download/v1.0.0/kwot-darwin-arm64 -o kwot
chmod +x kwot
sudo mv kwot /usr/local/bin/
```

### Option 3: Use Docker

```bash
docker pull Kong/kwot:1.0.0
```

## Using Docker

Pull and run kwot in a container:

```bash
# Create config directory
mkdir -p config

# Run kwot in Docker (macOS/Windows - Kong on host machine)
docker run -it \
  --name kwot \
  -e KONG_ADDR=http://host.docker.internal:8001 \
  -e AUTH_METHOD=RBAC \
  -e ADMIN_TOKEN=your-admin-token \
  -v $(pwd)/config:/home/kwot/config \
  Kong/kwot:1.0.0 apply --dry-run

# Run kwot in Docker (Linux - Kong on host machine)
docker run -it \
  --name kwot \
  --network host \
  -e KONG_ADDR=http://localhost:8001 \
  -e AUTH_METHOD=RBAC \
  -e ADMIN_TOKEN=your-admin-token \
  -v $(pwd)/config:/home/kwot/config \
  Kong/kwot:1.0.0 apply --dry-run

# View help
docker run Kong/kwot:1.0.0 --help

# Check version
docker run Kong/kwot:1.0.0 --version
```

**Important:** When Kong is running on your host machine:
- **macOS/Windows:** Use `http://host.docker.internal:8001` (Docker Desktop feature)
- **Linux:** Use `--network host` flag to access localhost ports

**Docker Compose example (Kong + kwot on same network):**

```yaml
...
services:
  ...
  kwot:
    image: Kong/kwot:1.0.0
    environment:
      KONG_ADDR: http://kong:8001
      AUTH_METHOD: RBAC
      ADMIN_TOKEN: ${ADMIN_TOKEN}
      CONFIG_DIR: /home/kwot/config
    volumes:
      - ./config:/home/kwot/config
    command: apply --dry-run
  ...
```

## Setup (5 minutes)

**1. Create `.env` file**
```bash
cat > .env << 'EOF'
KONG_ADDR=http://localhost:8001
AUTH_METHOD=RBAC
ADMIN_TOKEN=your-admin-token-here
CONFIG_DIR=./config/
FEATURE_CREATE_RBAC_USERS=true
FEATURE_DELETE_EXISTING_ROLES=true
EOF
source .env
```

**2. Create workspace directory and config**
```bash
mkdir -p config/demo

cat > config/demo/workspace.yaml << 'EOF'
config:
rbac:
  - role: admin
    permissions:
      - endpoint: "*"
        negative: false
        actions: "create,update,read,delete"
  - role: readonlyrole
    permissions:
      - endpoint: "*"
        negative: false
        actions: "read"
EOF

cat > config/demo/workspace-rbac-user.yaml << 'EOF'
- name: demo_rbac_user
  roles:
    - admin
EOF
```

**3. Create groups and roles**
```bash
cat > config/groups-and-roles.yaml << 'EOF'
role_info:
  admin: &admin admin
  readonlyrole: &readonly readonlyrole

config:
  - group_name: demo-admin-group
    group_comment: Admin group
    roles:
      - workspace: demo
        role: *admin

  - group_name: demo-readonly-group
    group_comment: Read-only group
    roles:
      - workspace: demo
        role: *readonly
EOF
```

**4. Preview and apply**
```bash
kwot apply -w demo --dry-run      # Preview for demo workspace
kwot apply -w demo                # Apply to demo workspace
kwot apply --dry-run              # Verify all configuration
```

## Essential Commands

```bash
# Preview changes
kwot apply --dry-run

# Apply to specific workspace
kwot apply -w demo

# Check drift (what would change)
kwot diff

# Check drift for specific workspace
kwot diff -w demo

# Delete a workspace
kwot delete workspaces -n demo --force

# Apply configuration
kwot apply
```

## Common Flags

| Flag | Purpose |
|------|---------|
| `--dry-run` | Preview without applying |
| `-w, --workspace <name>` | Target specific workspace (optional scope) |
| `-n, --name <name>` | Select specific resource (role/workspace/group) |
| `--verbose` | Detailed output |
| `--force` | Confirm destructive operations (delete only) |

## Key Configuration Variables

| Variable | Purpose | Example |
|----------|---------|---------|
| `KONG_ADDR` | Kong Admin API URL | `http://localhost:8001` |
| `AUTH_METHOD` | `RBAC`, `PASSWORD`, or `COOKIE` | `RBAC` |
| `ADMIN_TOKEN` | API token (for RBAC) | `your-token-here` |
| `CONFIG_DIR` | Config directory path | `./config/` |
| `FEATURE_CREATE_RBAC_USERS` | Create users from config | `true` |
| `FEATURE_DELETE_EXISTING_ROLES` | Delete roles not in config | `true` |
| `MAX_CONCURRENT_WORKSPACES` | Parallel workspace processing (1-100) | `5` (default) |

### Concurrency Tuning

The `MAX_CONCURRENT_WORKSPACES` controls how many workspaces are processed in parallel:

```bash
# Slow but safe (one at a time)
MAX_CONCURRENT_WORKSPACES=1 kwot apply

# Default - good balance
kwot apply

# Faster - for larger deployments
MAX_CONCURRENT_WORKSPACES=20 kwot apply

# Maximum parallelism
MAX_CONCURRENT_WORKSPACES=100 kwot apply
```

**Choose based on:**
- **Your machine:** CPU cores, memory available
- **Network:** Bandwidth for parallel calls

**Recommendation:**
- 1-5 workspaces: Default (5) or 1
- 50+ workspaces: 10-20
- 500+ workspaces: 20-50
- 1000+ workspaces: 50-100

## Next Steps

- See [README.md](README.md) for detailed documentation
- See [MIGRATION.md](MIGRATION.md) for Node.js migration guide
- See [CHANGELOG.md](CHANGELOG.md) for version history

## Troubleshooting

```bash
# Test connection with dry-run
kwot apply --dry-run

# Check drift (no changes made)
kwot diff

# Enable verbose logging
kwot apply --dry-run --verbose

# Check syntax
kwot apply --dry-run
```

**Connection issues?**
- Verify `KONG_ADDR` is correct
- Check Kong is running: `curl http://localhost:8001`

**Auth failed?**
- Check credentials in `.env`
- Verify `AUTH_METHOD` is correct
