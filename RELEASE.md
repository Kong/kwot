# Release Guide - kwot

Complete guide for releasing new versions of kwot with all best practices and automated checks.

## Release Process Overview

Our release process is fully automated via GitHub Actions but requires manual steps to initiate. All validation, testing, and building happens automatically.

## Pre-Release Checklist (Manual)

Before triggering a release, ensure:

### 1. Code Quality
- [ ] All tests pass: `make test`
- [ ] Linting passes: `make lint`
- [ ] Code is formatted: `make fmt`
- [ ] No security vulnerabilities: `go list -json -m all | go run golang.org/x/vuln/cmd/govulncheck@latest ./...`

```bash
# Run all checks at once
make all
```

### 2. Version & Documentation Updates
- [ ] Update version in [CHANGELOG.md](CHANGELOG.md)
  - Add release date (today's date)
  - List new features, fixes, and breaking changes
  - Keep changelog minimal and focused

- [ ] Update [BOM.md](BOM.md)
  - Run security audit: `make test`
  - Update security audit date to today
  - Verify dependency versions are current: `go list -u -m all`
  - Update binary size if changed: `ls -lh bin/kwot`

- [ ] Update version in [README.md](README.md) if applicable
  - Installation instructions
  - Feature mentions if new major features added

- [ ] Verify all documentation is accurate and up-to-date
  - Check all links are valid
  - Ensure examples still work

### 3. Build Verification
- [ ] Test build for current platform: `make build`
- [ ] Verify binary works: `./bin/kwot --version`
- [ ] Test key features with sample configs

### 4. Commit Changes
After all checks pass:

```bash
# Stage documentation updates
git add CHANGELOG.md BOM.md README.md

# Commit with semantic message
git commit -m "docs: prepare release v2.0.0"

# Push to main/master
git push origin main
```

## Triggering a Release

### Option 1: Push a Git Tag (Automatic)

The easiest and most reliable method:

```bash
# Create and push version tag
git tag -a v2.0.0 -m "Release kwot v2.0.0"
git push origin v2.0.0
```

The GitHub Actions workflow will automatically:
1. Validate the version format
2. Run full test suite
3. Build binaries for all platforms
4. Create checksums
5. Generate GitHub release with artifacts

### Option 2: Manual Workflow Dispatch

Via GitHub web UI:
1. Go to Actions → Release workflow
2. Click "Run workflow"
3. Enter version (e.g., `v2.0.0`)
4. Workflow runs same steps as tag-based release

## Release Workflow (Automated)

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
