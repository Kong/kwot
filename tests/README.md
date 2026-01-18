# kwot Test Suite

This directory contains integration and end-to-end tests for kwot.

## Unit Tests

Unit tests are located alongside the code:
- `internal/kong/*_test.go` - Kong client error handling tests
- `internal/config/*_test.go` - Configuration parsing tests
- `internal/validation/*_test.go` - Input validation tests
- `internal/logger/*_test.go` - Logging functionality tests
- `cmd/*_test.go` - CLI command tests

Run all unit tests:
```bash
go test ./...
```

Run with coverage:
```bash
go test ./... -cover
```

## Integration Tests

Integration tests that require Kong Gateway to be running:

### test.sh
Main integration test suite. Runs against the complete Kong setup with all workspaces, roles, and groups.

```bash
./tests/test.sh
```

### test_safety_model.sh
Tests the safety mechanisms:
- Deletion protection (prevents deletion of "default" workspace)
- Required `--force` flag for destructive operations
- Idempotent operations (safe to run multiple times)

```bash
./tests/test_safety_model.sh
```

### test_error_messages.sh
Tests improved error messages:
- System workspace protection: `cannot delete workspace 'default': it is a system workspace and cannot be removed`
- Non-existent resource: `workspace nonexistent not found`
- Missing required flags: `deletion requires --force flag to prevent accidents`
- Input validation: Empty field checks before Kong API calls

```bash
./tests/test_error_messages.sh
```

#### Example Error Messages

| Scenario | Error Message |
|----------|---------------|
| Delete system workspace | `cannot delete workspace 'default': it is a system workspace and cannot be removed` |
| Delete non-existent workspace | `failed to delete workspace 'nonexistent': workspace nonexistent not found` |
| Missing --force flag | `deletion requires --force flag to prevent accidents` |
| Duplicate workspace | `Resource already exists: workspace name 'demo1' already exists` |
| Duplicate role | `Resource already exists: role 'admin' already exists in workspace` |
| Invalid value | `Invalid value: name must be alphanumeric` |

### test_acme_anchors.sh
Tests ACME workspace-specific functionality.

```bash
./tests/test_acme_anchors.sh
```

### test_apply.sh
Quick smoke test for the apply command.

```bash
./tests/test_apply.sh
```

## Requirements

- Kong Gateway Enterprise 3.0+ (with RBAC enabled)
- kwot binary built (`make build`)
- `.env` file configured with Kong connection details

## Test Coverage

Current unit test coverage:
- `cmd`: 10.9%
- `config`: 51.2%
- `kong`: 13.5%
- `logger`: 67.9%
- `validation`: 48.6%

## Debugging

To see verbose output from tests, use the `--verbose` flag with kwot:
```bash
kwot --verbose apply --dry-run
```

For test debugging, check:
1. Kong Admin API connectivity: `curl http://localhost:8001/status`
2. RBAC configuration: `curl http://localhost:8001/rbac/roles`
3. Workspaces: `curl http://localhost:8001/workspaces`
