# Contributing to kwot

Thank you for your interest in contributing to kwot! This guide will help you get started with development, testing, and submitting improvements.

## Table of Contents

- [Getting Started](#getting-started)
- [Project Structure](#project-structure)
- [Development Workflow](#development-workflow)
- [Running Tests](#running-tests)
- [Code Quality](#code-quality)
- [Making Changes](#making-changes)
- [Submitting Changes](#submitting-changes)
- [Releasing](#releasing)

## Getting Started

### Prerequisites

- **Go 1.24.11+** - Download from [golang.org](https://golang.org/dl/)
- **Git** - For version control
- **Make** - For running build tasks
- **A Kong instance** - For integration testing (optional for unit tests)

### Setup Development Environment

```bash
# Clone the repository
git clone https://github.com/Kong/kwot.git
cd kwot

# Install dependencies
go mod download

# Verify setup
make help  # Shows all available make targets
```

## Project Structure

```
kwot/
├── cmd/                          # CLI commands
│   ├── all.go                   # apply command
│   ├── delete.go                # delete command
│   ├── diff.go                  # diff command
│   ├── root.go                  # root command & global flags
│   ├── delete_test.go           # tests for delete command
│   └── validators.go            # input validation helpers
│
├── internal/                     # Core logic (not exported)
│   ├── config/                  # Configuration loading & validation
│   │   ├── config.go            # Config struct & loader
│   │   ├── validator.go         # Config validation
│   │   └── validator_test.go    # Validation tests
│   │
│   ├── kong/                    # Kong Admin API client
│   │   ├── client.go            # HTTP client & API methods
│   │   ├── client_test.go       # Client tests
│   │   └── pagination.go        # Pagination helper
│   │
│   ├── models/                  # Data structures
│   │   ├── workspace.go         # Workspace model
│   │   ├── role.go              # RBAC role model
│   │   ├── group.go             # Group model
│   │   └── user.go              # User model
│   │
│   ├── workspace/               # Workspace operations
│   │   └── processor.go         # Workspace CRUD logic
│   │
│   ├── roles/                   # Role operations
│   │   └── processor.go         # Role CRUD logic
│   │
│   ├── groups/                  # Group operations
│   │   └── processor.go         # Group CRUD logic
│   │
│   ├── logger/                  # Logging utilities
│   │   ├── logger.go            # Logger with colors
│   │   └── logger_test.go       # Logger tests
│   │
│   └── validation/              # Generic validation
│       ├── validator.go         # Validation helpers
│       └── validator_test.go    # Validation tests
│
├── config/                       # Example YAML configurations
│   ├── root-workspace.yaml      # Root workspace config
│   ├── groups-and-roles.yaml    # Groups & roles config
│   └── acme/, demo1/, demo2/    # Example workspace configs
│
├── tests/                        # Integration/end-to-end tests
│   ├── test.sh                  # Main test script
│   ├── test_apply.sh            # Apply command tests
│   ├── test_error_messages.sh   # Error message tests
│   ├── test_safety_model.sh     # Safety feature tests
│   └── test_acme_anchors.sh     # Complex scenario tests
│
├── main.go                      # Entry point
├── go.mod                       # Dependencies
├── Makefile                     # Build tasks
├── Dockerfile                   # Docker image
├── .env.example                 # Example configuration
└── README.md                    # User documentation
```

## Development Workflow

### Conventional Commits Required

This project uses **Conventional Commits** for automated changelog generation and semantic versioning. All commits MUST follow this format:

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Valid types:** `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `chore`, `ci`

**Example:**
```
feat(workspace): add availability check with exponential backoff

Add GET endpoint verification to handle Kong Enterprise replication lag
when plugins fail immediately after workspace creation.

Fixes #42
```

See [CONVENTIONAL_COMMITS.md](./CONVENTIONAL_COMMITS.md) for comprehensive guidelines.

### Setup Commit Validation

Validation happens automatically at two levels:

**Local validation (before commit):**
```bash
# Install husky hooks (one time)
npm install husky --save-dev
npx husky install

# Install commitlint (one time)
npm install --save-dev @commitlint/config-conventional @commitlint/cli

# Now your commits are validated automatically before they're created
git commit -m "feat: add new feature"  # ✓ Accepted
git commit -m "added new feature"      # ✗ Rejected - wrong format
```

**CI validation (on pull requests):**
- GitHub Actions automatically validates all commits in PRs
- PR will fail if commits don't follow conventional format
- You'll get helpful feedback on what needs to be fixed

### Amend Commit Messages

If your commit was rejected:

```bash
# Fix the last commit message
git commit --amend -m "feat(scope): corrected message"

# Force push (only on your branch!)
git push origin your-branch-name --force-with-lease
```

## Development Workflow

### 1. Make Changes

```bash
# Create a feature branch
git checkout -b feature/your-feature-name

# Edit code in cmd/ or internal/ directories
vim internal/kong/client.go

# Verify changes compile
go build -o bin/kwot main.go
```

### 2. Run Tests Locally

```bash
# Run all unit tests
make test

# Run tests with verbose output
go test -v ./...

# Run tests for specific package
go test -v ./internal/kong

# Run tests with coverage report
make test-coverage
```

### 3. Check Code Quality

```bash
# Format code (auto-fix)
make fmt

# Run linter (identifies issues)
make lint

# Build the binary
make build

# Verify it works
./bin/kwot --help
```

## Running Tests

### Unit Tests

Unit tests are located in `*_test.go` files alongside the code they test:

```bash
# Run all unit tests
make test

# Run with coverage
make test-coverage

# Run specific test
go test -v -run TestDeleteFlagValidation ./cmd

# Run tests in verbose mode to see test names
go test -v -count=1 ./...  # -count=1 disables caching
```

**Test Files:**
- `cmd/delete_test.go` - Delete command tests
- `internal/kong/client_test.go` - Kong API client tests
- `internal/config/validator_test.go` - Config validation tests
- `internal/logger/logger_test.go` - Logger tests
- `internal/validation/validator_test.go` - Validation helper tests

### Integration Tests

Integration tests in `tests/` directory test real workflows against a Kong instance:

```bash
# Run integration tests (requires running Kong instance)
./tests/test.sh

# Run specific test
./tests/test_apply.sh

# Run with debug output
DEBUG=true ./tests/test.sh
```

**Integration Test Files:**
- `tests/test_apply.sh` - Test apply workflow
- `tests/test_error_messages.sh` - Verify error messages
- `tests/test_safety_model.sh` - Test safety features (--force, --dry-run)
- `tests/test_acme_anchors.sh` - Complex multi-workspace scenarios

### Testing Checklist

Before submitting changes, ensure:

- ✅ `make test` passes (all unit tests)
- ✅ `make lint` passes (no code style issues)
- ✅ `make build` succeeds (code compiles)
- ✅ `./bin/kwot --help` works
- ✅ `./bin/kwot apply --dry-run` runs without errors
- ✅ Integration tests pass if you modified core logic

## Code Quality

### Code Style

We follow Go conventions:

```bash
# Auto-format code
make fmt

# Equivalent to:
go fmt ./...
```

### Linting

```bash
# Run linter (catches common issues)
make lint

# Equivalent to:
golangci-lint run

# Fix some issues automatically
golangci-lint run --fix
```

### Error Handling

Always handle errors properly:

```go
// ❌ Don't ignore errors
_ = someFunc()  // WRONG - silent failures

// ✅ Do handle errors
if err != nil {
    return fmt.Errorf("operation failed: %w", err)
}

// ✅ Or explicitly suppress if safe
_ = file.Close()  // Already checked in defer
```

## Making Changes

### Code Organization

- **cmd/** - CLI command logic only, keep minimal
- **internal/** - Core business logic, reusable code
- **models/** - Data structures
- **tests/** - Integration/end-to-end tests only

### Adding Features

1. **Write tests first** - Define expected behavior in tests
2. **Implement logic** - In appropriate `internal/` package
3. **Update CLI** - Add command/flags in `cmd/`
4. **Test everything** - Unit tests + integration tests
5. **Update docs** - README, CHEATSHEET, etc if user-facing

### Example: Adding a New Flag

```go
// In cmd/root.go
var newFlag string

func init() {
    rootCmd.PersistentFlags().StringVar(&newFlag, "new-flag", "", "description")
}

// In cmd/all.go (or relevant command)
func someCommand() error {
    if newFlag != "" {
        // Use the flag
    }
}
```

### Example: Adding Kong API Support

1. **Add to internal/kong/client.go**
   ```go
   func (c *Client) GetNewResource(ctx context.Context, name string) (*Model, error) {
       // Implementation
   }
   ```

2. **Add unit test in internal/kong/client_test.go**
   ```go
   func TestGetNewResource(t *testing.T) {
       // Test cases
   }
   ```

3. **Use in internal/workspace/processor.go or similar**

## Submitting Changes

### Commit Messages

Write clear, descriptive commit messages:

```bash
# Good
git commit -m "Add support for COOKIE authentication"
git commit -m "Fix pagination bug in workspace list"

# Avoid
git commit -m "fix"
git commit -m "update stuff"
```

### Pull Request Process

1. **Fork and branch**
   ```bash
   git checkout -b feature/descriptive-name
   ```

2. **Make changes and test**
   ```bash
   make fmt lint test
   ```

3. **Commit with clear messages**
   ```bash
   git commit -m "Add feature description"
   ```

4. **Push and create PR**
   ```bash
   git push origin feature/descriptive-name
   ```

5. **PR checklist:**
   - ✅ Tests pass (`make test`)
   - ✅ Code lints (`make lint`)
   - ✅ Builds successfully (`make build`)
   - ✅ Documentation updated if user-facing
   - ✅ Clear commit messages
   - ✅ Addresses a specific issue or feature

## Common Tasks

### Debug a Failing Test

```bash
# Run with verbose output
go test -v -run TestName ./package

# Run with output
go test -v ./... 2>&1 | less

# Run single test with debugging
go test -run TestName -v -count=1 ./cmd
```

### Check Test Coverage

```bash
# Generate coverage report
go test -cover ./...

# Detailed coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out  # Opens in browser
```

### Test Against Specific Kong Instance

```bash
# Set Kong connection
export KONG_ADDR=http://localhost:8001
export AUTH_METHOD=RBAC
export ADMIN_TOKEN=your-token

# Run integration tests
./tests/test_apply.sh
```

### Build for Different Platforms

```bash
# macOS ARM64
GOOS=darwin GOARCH=arm64 go build -o bin/kwot-darwin-arm64

# Linux x86_64
GOOS=linux GOARCH=amd64 go build -o bin/kwot-linux-amd64

# Windows
GOOS=windows GOARCH=amd64 go build -o bin/kwot.exe
```

## Getting Help

- **Questions?** Check [README.md](README.md) and [CHEATSHEET.md](CHEATSHEET.md)
- **Need clarification?** Open an issue on GitHub
- **Found a bug?** Report it with steps to reproduce

## Code Review Tips

When your PR is reviewed:

- **Be open to feedback** - Code review improves quality
- **Respond to comments** - Address all review feedback
- **Re-run tests** - After making requested changes
- **Ask questions** - If feedback is unclear

---

## Releasing

### Pre-Release Checklist

Before creating a release, ensure:

1. **Code Quality**
   ```bash
   make lint    # No linting issues
   make test    # All tests pass
   make fmt     # Code is formatted
   ```

2. **Documentation Updates**
   - [ ] Update [CHANGELOG.md](CHANGELOG.md) with release date and changes
   - [ ] Update [BOM.md](BOM.md) with binary size if changed (`ls -lh bin/kwot`)
   - [ ] Update version in [README.md](README.md) if applicable
   - [ ] Verify all docs are accurate

3. **Build Verification**
   ```bash
   make build
   ./bin/kwot --version      # Verify it works
   ```

4. **Commit Changes**
   ```bash
   git add CHANGELOG.md BOM.md README.md
   git commit -m "docs: prepare release v1.0.0"
   git push origin main
   ```

### Creating a Release

**Option 1: Using Git Tags (Recommended)**

```bash
# Create annotated tag
git tag -a v1.0.0 -m "Release kwot v1.0.0"

# Push tag to GitHub
git push origin v1.0.0
```

The GitHub Actions workflow will automatically:
- Run all tests and linting
- Build binaries for all platforms (Linux, macOS, Windows)
- Create a GitHub Release with artifacts
- Generate checksums for verification

### Release Workflow Details

See [RELEASE.md](RELEASE.md) for complete information about:
- Automated testing and building
- Multi-platform binary generation
- GitHub Release creation
- Release artifact verification

---

Thank you for contributing to kwot! 🙏
