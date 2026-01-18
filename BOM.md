# Bill of Materials (BOM) - kwot

Last Updated: 2025-01-16

## Build

- **Language:** Go 1.24.11
- **Binary Size:** 8.0MB (with `-s -w` and `-trimpath` flags)
- **CVEs:** 0 known vulnerabilities (verified via `govulncheck`)
- **Platforms:** Linux (amd64, arm64, 386), macOS (amd64, arm64), Windows (amd64)
- **License:** Not specified (likely internal/proprietary)

## Dependency Summary

- **Direct Dependencies:** 5
- **Total Dependencies:** 14 (including transitive)
- **Vulnerability Scanning:** Enabled in CI/CD via `govulncheck`
- **Update Cycle:** Automated with manual review

## Direct Dependencies (Required)

| Dependency | Version | License | Used For | Status |
|------------|---------|---------|----------|--------|
| `github.com/spf13/cobra` | v1.8.0 | Apache 2.0 | CLI framework | ✓ Active |
| `gopkg.in/yaml.v3` | v3.0.1 | Apache 2.0 | YAML parsing | ✓ Active |
| `github.com/joho/godotenv` | v1.5.1 | MIT | .env file loading | ✓ Active |
| `github.com/fatih/color` | v1.16.0 | MIT | Colored output | ✓ Active |
| `github.com/google/uuid` | v1.5.0 | BSD 3-Clause | UUID generation | ✓ Active |

## Transitive Dependencies

| Dependency | Version | License | Parent | Status |
|------------|---------|---------|--------|--------|
| `github.com/inconshreveable/mousetrap` | v1.1.0 | Apache 2.0 | cobra | ✓ Active |
| `github.com/spf13/pflag` | v1.0.5 | BSD 3-Clause | cobra | ✓ Active |
| `github.com/mattn/go-colorable` | v0.1.13 | MIT | fatih/color | ✓ Active |
| `github.com/mattn/go-isatty` | v0.0.20 | MIT | fatih/color | ✓ Active |
| `golang.org/x/sys` | v0.14.0 | BSD 3-Clause | fatih/color, go-isatty | ✓ Active |
| `github.com/cpuguy83/go-md2man/v2` | v2.0.3 | MIT | cobra (help text) | ✓ Active |
| `github.com/russross/blackfriday/v2` | v2.1.0 | BSD 2-Clause | go-md2man | ✓ Active |
| `gopkg.in/check.v1` | v0.0.0-20161208181325-20d25e280405 | BSD 2-Clause | yaml.v3 | ✓ Active |

## Dependency Analysis

### Unused Dependencies
✓ **NONE** - All dependencies are actively used:
- **cobra**: CLI framework (root.go, all.go, delete.go, diff.go)
- **yaml.v3**: Config parsing (config.go)
- **godotenv**: Environment loading (config.go)
- **fatih/color**: Colored logging (logger.go)
- **google/uuid**: UUID generation (validators.go, workspace processor)

### License Compliance
✓ **All permissive licenses:**
- Apache 2.0: 3 packages
- MIT: 5 packages
- BSD: 4 packages
- **No GPL/LGPL/AGPL dependencies**

## Security

### Vulnerability Scanning
- **Tool:** `govulncheck` (official Go vulnerability scanner)
- **Frequency:** On every CI run (push to main, pull requests)
- **Location:** `.github/workflows/ci.yml`
- **Last Scan:** Passes (January 16, 2025)

### Dependency Updates
- **Strategy:** Regular updates to latest stable versions
- **Cadence:** Monthly review recommended
- **Management:** `go get -u` with testing before merge

### Known Issues
None currently.

## How to Update Dependencies

```bash
# Check for available updates
go list -u -m all

# Update all dependencies
go get -u ./...

# Update specific dependency
go get github.com/spf13/cobra@latest

# Verify no vulnerabilities
go run golang.org/x/vuln/cmd/govulncheck@latest ./...

# Clean up unused ones
go mod tidy
```

