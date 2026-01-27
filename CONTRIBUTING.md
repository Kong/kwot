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
- [Getting Help](#getting-help)

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

This project uses **Conventional Commits** for automated changelog generation. See [CONVENTIONAL_COMMITS.md](./CONVENTIONAL_COMMITS.md) for complete guidelines.

**Setup validation (automatic on npm install):**
```bash
# Clone and setup (this automatically installs commit hooks)
git clone https://github.com/Kong/kwot.git
cd kwot
npm install  # ← Automatically installs husky commit message hooks

# Verify hooks are installed
ls -la .husky/
```

**Commit format:**
```
<type>(<scope>): <subject>

<body>

<footer>
```

**Valid types:** `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `chore`, `ci`

Validation happens automatically at two levels:
- **Local:** Git hook rejects invalid messages before commit (runs instantly)
- **CI:** GitHub Actions validates all PR commits with detailed feedback

If a commit message is rejected:
- Review the error message shown by the hook
- Fix your commit message format
- Try committing again

### Making Changes

```bash
# Create a feature branch
git checkout -b feature/your-feature-name

# Edit code
vim internal/kong/client.go

# Verify it compiles
make build

# Run tests
make test

# Check code quality
make fmt lint

# Verify it works
./bin/kwot --help
```

## Running Tests

### Unit Tests

```bash
# Run all tests
make test

# With coverage
make test-coverage

# Specific package
go test -v ./internal/kong

# Specific test
go test -v -run TestName ./cmd
```

### Integration Tests

Requires a running Kong instance:

```bash
# Run all integration tests
./tests/test.sh

# Specific test
./tests/test_apply.sh

# With debug output
DEBUG=true ./tests/test.sh
```

### Before Submitting

- ✅ `make test` passes (all unit tests)
- ✅ `make lint` passes (no style issues)
- ✅ `make build` succeeds (compiles)
- ✅ Integration tests pass if you changed core logic

## Code Quality

```bash
# Format code
make fmt

# Run linter
make lint

# Equivalent commands
go fmt ./...
golangci-lint run
```

**Always handle errors:**
```go
// ✅ Do this
if err != nil {
    return fmt.Errorf("failed: %w", err)
}

// ❌ Not this
_ = someFunc()  // Silent failures
```

## Making Changes

### Code Organization

- **cmd/** - CLI commands
- **internal/** - Core business logic
- **tests/** - Integration tests

### Adding a Feature

1. Write tests first (define expected behavior)
2. Implement logic in `internal/` packages
3. Add CLI support in `cmd/` if user-facing
4. Run `make test lint build` before submitting
5. Update docs if needed

## Submitting Changes

1. Create a feature branch: `git checkout -b feature/name`
2. Make changes following conventional commits format
3. Run tests: `make test lint build`
4. Push and create a PR
5. Respond to review feedback
6. We'll merge when ready!

**PR checklist:**
- ✅ Tests pass
- ✅ Code lints
- ✅ Conventional commit format
- ✅ Addresses a specific issue

## Getting Help

- **Questions?** Check [README.md](README.md) and [CHEATSHEET.md](CHEATSHEET.md)
- **Issues?** Open a GitHub issue with reproduction steps
- **Review feedback?** Be open to suggestions, ask for clarification

---

Thank you for contributing to kwot! 🙏
