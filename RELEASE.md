# Release Guide - kwot

Complete guide for releasing new versions of kwot with all best practices and automated checks.

## Release Process Overview

Our release process is **fully automated via GitHub Actions**:

1. **Version Bump** → Update `VERSION` in `Makefile`
2. **PR Merge** → Merge to main branch
3. **Auto-Detection** → Workflow detects version change
4. **Signed Tag** → Automatically creates GPG-signed tag
5. **Release Build** → Builds binaries for all platforms
6. **Release Creation** → Creates GitHub release with artifacts

**No manual tag creation needed!** The workflow handles everything.

## Pre-Release Checklist (Manual)

Before submitting PR, ensure:

### 1. Code Quality
- [ ] All tests pass: `make test`
- [ ] Linting passes: `make lint`
- [ ] Code is formatted: `make fmt`

```bash
# Run all checks at once
make all
```

### 2. Version & Documentation Updates
- [ ] **Update version in Makefile** (Required!)
  ```bash
  # Edit Makefile: VERSION=X.Y.Z
  # Semantic versioning: MAJOR.MINOR.PATCH
  ```

- [ ] Update [CHANGELOG.md](CHANGELOG.md)
  - Add release date (today's date)
  - List new features, fixes, and breaking changes
  - Keep changelog minimal and focused

- [ ] Verify all documentation is accurate
  - README.md
  - Examples in config files
  - BOM.md if dependencies changed

### 3. Build Verification
- [ ] Test build: `make build`
- [ ] Verify binary: `./bin/kwot --version`
- [ ] Test key features with sample configs

### 4. Create PR and Merge
```bash
# Push feature branch
git push origin feature-branch

# Create PR on GitHub
# Wait for CI to pass
# Merge PR to main
```

**After PR merges to main:**
- ✅ Auto-release workflow detects version change
- ✅ Creates GPG-signed git tag automatically
- ✅ Triggers release build workflow
- ✅ Builds and publishes release

## Automatic Release Flow

### What Happens When You Merge

1. **Version Detection**
   - Workflow compares `Makefile` VERSION with previous commit
   - Detects if version was bumped

2. **Tag Creation**
   - Creates annotated git tag automatically
   - Tag includes CHANGELOG entry in message
   - (Optionally signed with GPG if secrets are configured)

3. **Release Build**
   - Validates version format
   - Runs all tests and linting
   - Builds binaries for all platforms:
     - Linux (amd64, arm64, 386)
     - macOS (amd64, arm64)
     - Windows (amd64)
   - Creates checksums
   - Creates GitHub Release with artifacts

4. **Verification**
   - All artifacts downloadable from GitHub Releases
   - Version propagates to package managers
   - (Optional: Tag shows "Verified" badge if GPG-signed)

## Setting Up GPG Signing (Optional)

GPG signing of tags adds an extra verification layer showing a "Verified" badge on GitHub. **This is optional but recommended for security.**

### To Enable GPG Signing

If you want signed releases, run the setup script:

```bash
bash scripts/setup-gpg-signing.sh
```

This will:
1. Generate a GPG key pair
2. Guide you through exporting the private key
3. Show you where to add GitHub secrets

### What It Does

- ✅ Tags are GPG-signed when created
- ✅ Releases show "Verified" badge on GitHub
- ✅ Provides cryptographic proof of who created the release

### If You Skip GPG Signing

- ✅ Releases still work normally
- ✅ Artifacts are still built and published
- ✅ No "Verified" badge, but still safe (artifacts from GitHub's infrastructure)
- ✅ You can add GPG signing anytime later

**Recommendation:** For Kong organization releases, GPG signing is best practice. For testing, you can skip it initially.

## Manual Release Process (Legacy)

If auto-release workflow fails, you can manually create a release:

### Option 1: Push a Git Tag

```bash
# Create and push version tag
git tag -a v2.0.0 -m "Release kwot v2.0.0"
git push origin v2.0.0
```

### Option 2: Manual Workflow Dispatch

Via GitHub web UI:
1. Go to Actions → Release workflow
2. Click "Run workflow"
3. Enter version (e.g., `v2.0.0`)

## Release Workflow Details

The `.github/workflows/release.yml` handles:

### Step 1: Validation
- ✅ Validates semantic version format
- ✅ Extracts version information
- ✅ Fails fast on invalid versions

### Step 2: Tests (All Platforms)
- ✅ Runs `make lint`
- ✅ Runs `make test-coverage`
- ✅ Ensures code quality before build
- ✅ Fails if any tests don't pass

### Step 3: Build
Builds optimized binaries for all supported platforms:
- Linux: amd64, arm64, 386
- macOS: amd64 (Intel), arm64 (Apple Silicon)
- Windows: amd64

Each build:
- Uses `-ldflags="-s -w"` for size optimization
- Includes version, build date, and commit SHA
- Disables CGO for static linking
- Creates SHA256 checksums for verification

### Step 4: Package
- Creates tarball archives (Linux/macOS)
- Creates ZIP archives (Windows)
- Includes README and CHANGELOG with each archive
- Creates SHA256 checksum files for verification

### Step 5: Release
- Creates GitHub Release
- Uploads all binaries and checksums
- Generates comprehensive release notes
- Includes installation instructions
- Links to documentation

### Step 6: Notification
- Displays release summary
- Confirms all artifacts are available
- Provides next steps for users

## Checking Build Status

Monitor the release progress:

1. **GitHub Actions Page:**
   - Go to repository Actions tab
   - Find the release workflow run
   - View logs for each platform's build

2. **Command Line (with GitHub CLI):**
   ```bash
   gh workflow view release
   ```

3. **Watch in Real-Time:**
   - Workflow typically completes in 5-10 minutes
   - Each platform build takes ~2 minutes

## Post-Release

### 1. Verify Release Artifacts
```bash
# Download and test the release
wget https://github.com/Kong/kwot/releases/download/v2.0.0/kwot-v2.0.0-linux-amd64.tar.gz

# Extract and verify checksum
tar xzf kwot-v2.0.0-linux-amd64.tar.gz
sha256sum -c kwot-v2.0.0-linux-amd64.sha256

# Test the binary
./kwot-v2.0.0/linux-amd64/kwot --version
```

### 2. Announcement
- [ ] Post release announcement to team
- [ ] Update any tracking systems
- [ ] Notify downstream projects if applicable

### 3. Rollback (If Needed)
If critical issues are found:

```bash
# Delete the git tag
git tag -d v2.0.0
git push origin :v2.0.0

# Delete the GitHub Release
gh release delete v2.0.0 -y

# Fix the issue and create a patch release
# E.g., v2.0.1
```

## Supported Platforms

The release includes binaries for:

| Platform | Architecture | Format |
|----------|--------------|--------|
| Linux | x86_64 (amd64) | tar.gz |
| Linux | ARM64 | tar.gz |
| Linux | 32-bit (386) | tar.gz |
| macOS | Intel (amd64) | tar.gz |
| macOS | Apple Silicon (arm64) | tar.gz |
| Windows | x86_64 (amd64) | .zip |

Users can choose the appropriate binary for their system.

## Version Numbering

We follow [Semantic Versioning](https://semver.org/):

- **MAJOR.MINOR.PATCH** (e.g., `v1.2.3`)
- **Major:** Breaking changes, significant feature releases
- **Minor:** New features, backwards compatible
- **Patch:** Bug fixes, security updates

Examples:
- `v1.0.0` - Initial release
- `v1.1.0` - New features added
- `v1.1.1` - Bug fix
- `v2.0.0` - Breaking changes

## Automated Checks During Release

The release workflow performs:

✅ **Version Validation**
- Format check (semantic versioning)
- Tag format validation

✅ **Code Quality**
- Linting with golangci-lint
- Test coverage verification
- All tests must pass

✅ **Build Verification**
- Build on ubuntu-latest, macos-latest for native builds
- Cross-compilation for other architectures
- No compile errors allowed

✅ **Binary Verification**
- Binary size logged
- SHA256 checksums created
- Version information embedded

✅ **Documentation Completeness**
- README included in archives
- CHANGELOG included in archives
- Links included in release notes

## Troubleshooting

### Release Workflow Failed

1. **Check workflow logs:**
   ```bash
   gh workflow view release --log
   ```

2. **Common failures:**
   - Invalid version format → Ensure `v` prefix and semantic version
   - Tests failed → Fix issues and retry
   - Build failed → Check Go version compatibility

### Binary Size Issues

If binary is larger than expected:
- Run: `go list -m all` to check dependencies
- Verify `-ldflags="-s -w"` is being used
- Check for cgo dependencies being linked

### Checksum Verification Fails

Users can verify binaries:
```bash
sha256sum -c kwot-v2.0.0-linux-amd64.sha256
# Output should show "OK"
```

If checksum doesn't match:
- File was corrupted during download
- Binary was modified after upload
- Contact maintainers with checksums provided

## Dependencies Management

### Checking for Updates
```bash
go list -u -m all
```

### Keeping Dependencies Current
- Run periodically during development
- Update outdated deps with: `go get -u <package>`
- Run `go mod tidy` to clean up
- Test thoroughly after updating

### Security Scanning
```bash
# Check for known vulnerabilities
go list -json -m all | go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

## Release Frequency

- **Patch releases** (bug fixes): As needed
- **Minor releases** (features): Monthly or quarterly
- **Major releases** (breaking changes): As needed, communicated in advance

Always prioritize quality over speed. Run all checks before releasing.

## Questions or Issues?

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and testing guidelines.
