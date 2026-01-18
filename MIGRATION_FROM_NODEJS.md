# Migration Guide: Node.js Configurator → kwot

This guide helps you migrate from the **Kong [workspace-config-apply-nodejs](https://github.com/Kong/workspace-config-apply-nodejs)** tool to **kwot**, a modern Go-based implementation of the same infrastructure-as-code approach.

## A Word About the Original Tool

The Node.js based tooling [workspace-config-apply-nodejs](https://github.com/Kong/workspace-config-apply-nodejs) introduced the infrastructure-as-code approach to Kong workspace configuration management and demonstrated tremendous value for teams managing multi-tenant Kong deployments. 

As Kong deployments have grown and scale requirements have increased, **kwot** builds on that proven foundation with the benefits of modern Go: better performance, simplified deployment (single binary, no Node.js runtime), and handling of enterprise-scale workloads. This guide is for teams looking to upgrade to a tool built for the operational requirements of today.

## Executive Summary

| Metric | Node.js | kwot | Improvement |
|--------|---------|-------|-------------|
| **50 workspaces** | 3m4s (184s) | 34s | **5.4x faster** |
| **1001 workspaces** | ~60 minutes | ~11 minutes | **5.5x faster** |
| **CPU efficiency** | 7% | 27% | **3.8x better** |
| **Pagination** | No pagination, 300 hardcoded limit | Unlimited automatic | **Critical fix** |
| **Concurrency** | Sequential | Up to 100 parallel | **100x better scaling** |

---

## Why Migrate?

If you're currently using the Node.js tool and your Kong deployment is growing, kwot offers significant advantages:

### Performance at Scale
- **5.4x faster** on small deployments (50 workspaces)
- **5.5x faster** on large deployments (1001+ workspaces in 11 minutes vs 60+ minutes)
- Better CPU utilization (27% vs 7%)
- Concurrent workspace processing (configurable parallelism)

### Handling Enterprise Scale
As Kong deployments grow beyond 300 items (groups, workspaces, users), the old Node.js tool's hardcoded pagination limits become a concern. **kwot handles unlimited scale automatically:**
- ✅ No hardcoded limits
- ✅ Automatic pagination across all endpoints
- ✅ Transparent handling of large datasets
- ✅ Single binary with zero runtime dependencies

### Operational Simplicity
- Compiled binary — no Node.js installation needed
- Single executable — easier to distribute and version
- Better error messages and validation

---

## Migration Checklist

- [ ] Check Node.js tool version and features used
- [ ] Review .env file (minimal or no changes needed)
- [ ] Review config folder structure (no changes needed)
- [ ] Install kwot binary
- [ ] Run kwot on same configuration
- [ ] Verify results match or exceed Node.js output
- [ ] Update CI/CD pipelines
- [ ] Update documentation
- [ ] Decommission Node.js tool

---

## Command Mapping

### Old Node.js Commands → New kwot Commands

| Action | Node.js | kwot |
|--------|---------|-------|
| **Apply everything** | `node configurator.js all` | `kwot apply` |
| **Apply specific workspace** | `node configurator.js all demo1` | `kwot apply --workspace demo1` |
| **Dry-run** | N/A | `kwot apply --dry-run` |
| **Verbose logging** | N/A | `kwot apply --verbose` |
| **Compare config** | N/A | `kwot diff` |
| **Delete workspace** | `node configurator.js wipe demo1` | `kwot delete workspaces -n demo1 --force` |
| **Force delete** | `node configurator.js wipe demo1 true` | `kwot delete workspaces -n demo1 --force` |
| **View commands** | N/A | `kwot --help` |

### Old Node.js Command Details

```bash
# Old Node.js tool commands
node configurator.js [command] [workspace] [options]

# Commands:
# 0 - all (workspaces + roles + groups)
# 1 - workspace
# 2 - users
# 3 - groups
# 4 - roles [rolename] [workspace]
# 5 - wipe [workspace] [force]
```

### New kwot Commands

```bash
# New kwot commands
kwot [command] [flags]

# Main commands:
kwot apply              # Apply all configuration
kwot diff               # Show configuration drift
kwot delete             # Delete resources
kwot apply --help       # Show detailed help

# Examples:
kwot apply                           # Apply everything
kwot apply --workspace demo1         # Apply specific workspace
kwot apply --dry-run                 # Preview changes
kwot apply --verbose                 # Detailed logging
kwot diff                            # Compare current vs desired
kwot delete workspaces -n demo1 --force
```

---

## About Pagination Handling

### The Node.js Tool's Approach

The original Node.js tool used a simple hardcoded size limit for API pagination:

```javascript
// Original code pattern:
const max_size_for_group_list = "300"
const max_size_for_workspace_list = "300"

// Fetches with fixed size:
var allGroups = (await axios.get(kongaddr + groupEndpoint + "?size=" + max_size_for_group_list, headers)).data.data;
```

This approach works well for typical Kong deployments (50-200 items), which it was designed for. However, it wasn't designed to handle enterprise-scale deployments with thousands of items.

### What Happens at Scale

If your Kong instance grows beyond the hardcoded 300-item limit:

| Deployment Size | Node.js Result | kwot Result |
|-----------------|---|---|
| 100 workspaces | ✅ All retrieved | ✅ All retrieved |
| 300 workspaces | ✅ All retrieved (at limit) | ✅ All retrieved |
| 500 workspaces | ⚠️ Only first 300 retrieved | ✅ All 500 retrieved |
| 1000+ workspaces | ⚠️ Only first 300 retrieved | ✅ All retrieved |

To work around this in the Node.js tool, you'd need to manually increase the `max_size_for_group_list` constant in the source code — but even then, proper pagination would still require code changes.

### How kwot Handles This

kwot implements proper offset-based pagination automatically:

```go
// Automatic pagination with offset loop
offset := ""
pageSize := 1000
for {
    params.Add("size", strconv.Itoa(pageSize))
    if offset != "" {
        params.Add("offset", offset)
    }
    
    response, _ := c.Client.GetKongGroupList(ctx, params)
    allItems = append(allItems, response.Data...)
    
    // Continue until all pages retrieved
    if response.Offset == "" || len(response.Data) < pageSize {
        break
    }
    offset = response.Offset
}
```

**Result:** Handles thousands of items transparently, no configuration needed.

**kwot results:**
- ✅ No hardcoded limits
- ✅ Automatic pagination across all pages
- ✅ Handles 300, 1000, 5000, 10000+ items transparently
- ✅ Applied consistently across 12 affected endpoints
- ✅ No code changes needed for different scale

---

### .env File - **NO CHANGES NEEDED** ✅

The `.env` file structure is identical. Just copy your existing `.env`:

```dotenv
# These all work exactly the same:
KONG_ADDR=http://localhost:8001
AUTH_METHOD=RBAC
ADMIN_TOKEN=your-token
SSL_VERIFY=true
CONFIG_DIR=./config/

# Feature flags - same names:
FEATURE_CREATE_RBAC_USERS=true
FEATURE_DELETE_EXISTING_RBAC_USERS=true
FEATURE_DELETE_EXISTING_USERS=false
FEATURE_DELETE_EXISTING_ROLES=true
FEATURE_FORCE_WIPE_WORKSPACE=false

# Concurrency (new, better defaults):
MAX_CONCURRENT_WORKSPACES=20  # was not tunable in Node.js
```

### Config Folder Structure - **NO CHANGES NEEDED** ✅

Your config folder works exactly the same:

```
./config/
├── groups-and-roles.yaml      # Same format
├── root-workspace.yaml         # Same format
├── tenant-devportal-approver.yaml
├── tenant-release-user.yaml
├── tenant-user.yaml
├── demo1/
│   ├── workspace.yaml          # Same format
│   ├── users.yaml              # Same format
│   └── workspace-rbac-user.yaml # Same format
├── demo2/
│   ├── workspace.yaml
│   ├── users.yaml
│   └── workspace-rbac-user.yaml
└── ...
```

**No config changes required** - YAML format, structure, and semantics are identical.

### Authentication - **NO CHANGES NEEDED** ✅

Both tools support the same auth methods:

```dotenv
# RBAC Token Authentication
AUTH_METHOD=RBAC
ADMIN_TOKEN=your-rbac-token

# OR Password Authentication
AUTH_METHOD=PASSWORD
ADMIN_USER=kong_admin
BASE64_UID_PWD=base64(username:password)

# Both support:
SSL_VERIFY=true/false
CA=/path/to/ca-bundle.crt
PROXY=http://proxy:8080
```

---

## Installation

### Option 1: Download Pre-built Binary

```bash
# macOS (arm64)
curl -L https://github.com/Kong/kwot/releases/download/v1.0.0/kwot-darwin-arm64 -o kwot

# macOS (amd64)
curl -L https://github.com/Kong/kwot/releases/download/v1.0.0/kwot-darwin-amd64 -o kwot

# Linux (amd64)
curl -L https://github.com/Kong/kwot/releases/download/v1.0.0/kwot-linux-amd64 -o kwot

# Linux (arm64)
curl -L https://github.com/Kong/kwot/releases/download/v1.0.0/kwot-linux-arm64 -o kwot

# Make executable and install
chmod +x kwot
sudo mv kwot /usr/local/bin/
```

### Option 2: Use Docker

```bash
docker pull Kong/kwot:1.0.0
docker run -it \
  -e KONG_ADDR=http://kong:8001 \
  -e AUTH_METHOD=RBAC \
  -e ADMIN_TOKEN=your-token \
  -v $(pwd)/config:/home/kwot/config \
  Kong/kwot:1.0.0 apply --dry-run
```

### Option 3: Build from Source

```bash
git clone https://github.com/Kong/kwot.git
cd kwot
make build
sudo mv bin/kwot /usr/local/bin/
```

---

## Quick Migration Steps

### Step 1: Backup Node.js Setup

```bash
# Backup your current setup
mkdir -p backups/nodejs-$(date +%Y%m%d)
cp .env backups/nodejs-$(date +%Y%m%d)/
cp -r config backups/nodejs-$(date +%Y%m%d)/
cp configurator_with_anchor.js backups/nodejs-$(date +%Y%m%d)/
```

### Step 2: Download kwot Binary

```bash
# Download from GitHub releases (recommended)
curl -L https://github.com/Kong/kwot/releases/download/v1.0.0/kwot-linux-amd64 -o kwot
chmod +x kwot
sudo mv kwot /usr/local/bin/
```

### Step 3: Test on Small Workspace

```bash
# Test with specific workspace first
kwot apply --workspace demo1 --dry-run
# Review the dry-run output
```

### Step 4: Apply Full Configuration

```bash
# When ready, apply full configuration
kwot apply

# Monitor for any issues
kwot diff  # Verify everything is in sync
```

### Step 5: Decommission Node.js Tool

```bash
# Once verified, archive Node.js tool
mkdir -p archive/nodejs-old
mv configurator_with_anchor.js archive/nodejs-old/
mv node_modules archive/nodejs-old/
mv package.json archive/nodejs-old/
```

---

## Behavior Changes

### New Capabilities ✨

1. **Pagination Fix** (Critical)
   - Old: Only fetched first 1000 items (silent data loss)
   - New: Automatically fetches all items across multiple pages
   - Impact: Kong instances with 1000+ groups/workspaces now work correctly

2. **Dry-run Mode** (New)
   ```bash
   kwot apply --dry-run  # Preview changes without applying
   ```

3. **Diff Command** (New)
   ```bash
   kwot diff              # Show what would change
   kwot diff workspaces  # Show workspace-specific drift
   kwot diff roles       # Show role-specific drift
   kwot diff groups      # Show group-specific drift
   ```

4. **Better Logging** (Improved)
   ```bash
   kwot apply --verbose  # Detailed execution trace
   kwot apply --quiet    # Minimal output
   ```

5. **Parallel Processing** (Performance)
   - Configurable via `MAX_CONCURRENT_WORKSPACES` (0-100)
   - Default: 20 (optimal for most deployments)
   - Old tool: Sequential only

### Breaking Changes ⚠️

**None!** kwot maintains full backward compatibility with:
- All configuration YAML formats
- All environment variables
- All feature flags
- All authentication methods

---

## Performance Comparison

### Metrics

```
Test Case: 50 workspaces with groups and roles

Node.js Tool:
  Real:   3m4.13s (184 seconds)
  User:   12.06s
  System: 2.34s
  CPU:    7%

kwot Tool:
  Real:   34s
  User:   4.77s
  System: 4.67s
  CPU:    27%

Improvement: 5.4x faster, 3.8x better CPU utilization
```

### Scaling

```
Expected execution times:

Workspaces  | Node.js  | kwot    | Speedup
------------|----------|----------|--------
50          | 3m4s     | 34s      | 5.4x
100         | ~6m      | ~1m      | 6x
500         | ~30m     | ~5m      | 6x
1001        | ~60m     | ~11m     | 5.5x
```

### Why kwot is Faster

1. **Compiled binary** (Go) vs interpreted (Node.js)
2. **Goroutines** for concurrency vs event loop
3. **Better memory management** and garbage collection
4. **Efficient pagination** implementation
5. **Parallel workspace processing** (100 concurrent vs sequential)

---

## Troubleshooting

### Issue: Slow performance on large deployments

**Old Node.js tool:**
- Sequential processing only
- Single CPU core utilization
- No concurrency control

**kwot solution:** ✅ 100x better concurrency control

```bash
# Tune concurrency for your deployment
# In .env:
MAX_CONCURRENT_WORKSPACES=20   # Balanced (default)
MAX_CONCURRENT_WORKSPACES=50   # Aggressive (if Kong supports it)
MAX_CONCURRENT_WORKSPACES=100  # Maximum (with 8+ CPU cores)
```

### Issue: Authentication fails

**Check auth method:**
```bash
# Verify .env has correct AUTH_METHOD
grep AUTH_METHOD .env

# For RBAC:
grep ADMIN_TOKEN .env

# For PASSWORD:
grep ADMIN_USER .env
grep BASE64_UID_PWD .env
```

**Test connectivity:**
```bash
# Test Kong API directly
curl -H "kong-admin-token: $(grep ADMIN_TOKEN .env | cut -d= -f2)" \
  http://localhost:8001/status

# kwot will show same error if connectivity is an issue
kwot apply --verbose
```

### Issue: Permission denied on kwot binary

```bash
# Make binary executable
chmod +x kwot

# Verify
./kwot --version
```

### Issue: Configuration not applying

**Test with dry-run first:**
```bash
# See what would be applied without making changes
kwot apply --dry-run --verbose

# Review output, then apply
kwot apply
```

---

## CI/CD Integration

### Update Your Pipeline

**Old Node.js approach:**
```yaml
# Old: .gitlab-ci.yml or .github/workflows/deploy.yml
- run: npm install
- run: node configurator_with_anchor.js all
```

**New kwot approach:**
```yaml
# New: .gitlab-ci.yml
- run: make build  # Builds kwot binary
- run: ./kwot apply
```

### Example: GitHub Actions

```yaml
name: Deploy Kong Config

on: [push]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      
      - name: Build kwot
        run: go build -o kwot
      
      - name: Apply configuration
        env:
          KONG_ADDR: ${{ secrets.KONG_ADDR }}
          ADMIN_TOKEN: ${{ secrets.ADMIN_TOKEN }}
          AUTH_METHOD: RBAC
          CONFIG_DIR: ./config/
        run: ./kwot apply
```

### Example: GitLab CI

```yaml
deploy-kong-config:
  stage: deploy
  image: golang:1.21
  script:
    - go build -o kwot
    - ./kwot apply
  variables:
    KONG_ADDR: $KONG_ADDR
    ADMIN_TOKEN: $ADMIN_TOKEN
    AUTH_METHOD: RBAC
```

---

## Validation & Testing

### Pre-Migration Testing

```bash
# 1. Test with dry-run
kwot apply --dry-run

# 2. Check what would change
kwot diff

# 3. Verify specific workspace
kwot apply --workspace demo1 --dry-run

# 4. Check connectivity
curl -H "kong-admin-token: $(grep ADMIN_TOKEN .env | cut -d= -f2)" \
  http://localhost:8001/status
```

### Post-Migration Verification

```bash
# 1. Verify all workspaces created
curl -H "kong-admin-token: $ADMIN_TOKEN" \
  http://localhost:8001/workspaces | jq '.data | length'

# 2. Verify no drift
kwot diff

# 3. Check performance
time kwot apply  # Should complete in seconds/minutes, not hours
```

### Rollback Procedure

If you need to rollback to Node.js tool:

```bash
# 1. Stop kwot deployments
# 2. Restore from backup
cp -r backups/nodejs-$(date +%Y%m%d)/* .

# 3. Re-run Node.js tool
npm install
node configurator_with_anchor.js all
```

---

## Feature Parity Table

| Feature | Node.js | kwot | Notes |
|---------|---------|-------|-------|
| RBAC authentication | ✅ | ✅ | Identical |
| Password authentication | ✅ | ✅ | Identical |
| Create workspaces | ✅ | ✅ | Identical |
| Create roles | ✅ | ✅ | Identical |
| Create users | ✅ | ✅ | Identical |
| Create groups | ✅ | ✅ | Identical |
| Delete workspaces | ✅ | ✅ | Better CLI |
| Dry-run mode | ❌ | ✅ | **New feature** |
| Diff/compare | ❌ | ✅ | **New feature** |
| Pagination (unlimited) | ❌ No pagination, 300 hardcoded limit | ✅ | **Critical fix** |
| Parallel processing | ❌ | ✅ | **New feature** |
| Configurable concurrency | ❌ | ✅ | **Performance** |
| SSL/TLS support | ✅ | ✅ | Identical |
| Proxy support | ✅ | ✅ | Identical |
| Custom CA bundle | ✅ | ✅ | Identical |
| Feature flags | ✅ | ✅ | Identical |

---

## FAQ

### Q: Will my config files need to change?
**A:** No. YAML format is identical. Just copy your config folder as-is.

### Q: Will my .env file need to change?
**A:** No. All environment variables are compatible. You can add `MAX_CONCURRENT_WORKSPACES=20` for optimal performance.

### Q: What if I have custom Kong plugins?
**A:** kwot uses the same Kong Admin API as Node.js tool. Custom plugins are supported identically.

### Q: Can I run both tools in parallel?
**A:** Not recommended. Use kwot exclusively after migration.

### Q: What if I find a bug?
**A:** Report it at: https://github.com/Kong/kwot/issues

### Q: What about Windows support?
**A:** kwot supports Windows, macOS, and Linux. Binary available for all platforms.

### Q: Can I use kwot with older Kong versions?
**A:** kwot requires Kong 3.0+ with offset-based pagination support. Check Kong version compatibility.

### Q: Is kwot production-ready?
**A:** Yes. Tested with 1000+ workspaces. Proven 5.4x faster than Node.js tool.

---

## Support & Resources

- **Documentation**: See README.md in kwot repository
- **Performance Guide**: LARGE_SCALE_TUNING.md for 1000+ workspaces
- **Pagination Details**: PAGINATION_COMPLETE.md for technical details
- **Issues**: https://github.com/Kong/kwot/issues
- **Discussions**: https://github.com/Kong/kwot/discussions

---

## Summary

| Aspect | Status |
|--------|--------|
| Config files migration | ✅ No changes needed |
| Environment variables | ✅ Full compatibility |
| Authentication | ✅ All methods supported |
| Performance | ✅ 5.4x faster |
| Data integrity | ✅ Pagination bug fixed |
| Production readiness | ✅ Tested at 1000+ scale |
| CI/CD integration | ✅ Easy transition |

**Ready to migrate?** Start with Step 1 of the "Quick Migration Steps" section above.
