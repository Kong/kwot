# Performance Testing

This directory contains tools and configuration for performance testing kwot against Kong.

## Prerequisites

- `kwot` binary installed in PATH
- `docker-compose` installed
- `curl` installed
- Valid `KONG_LICENSE_DATA` environment variable (required by docker-compose)

## Quick Start

Run the complete performance test workflow (generate config + start Kong + run kwot apply):

```bash
make perf-full NUM_WORKSPACES=50
```

## Individual Commands

### Generate Workspace Configuration

Create N workspace configurations for testing:

```bash
make generate-workspaces NUM_WORKSPACES=50 CONFIG_DIR=config-perf-test
```

**Parameters:**
- `NUM_WORKSPACES`: Number of workspaces to generate (default: 50)
- `CONFIG_DIR`: Directory to generate config in (default: config-perf-test)

### Start Kong Services

Kong services are started automatically by the perf-test script. Alternatively, start them manually:

```bash
cd perf-test
docker-compose up -d
```

Wait for Kong to be ready:

```bash
# Health check endpoint becomes available when Kong is ready
curl http://localhost:8001/status
```

### Run Performance Test

Run kwot apply against the generated configuration:

```bash
make perf-test
```

Or with custom config directory:

```bash
CONFIG_DIR=custom-config make perf-test
```

#### Skip Cleanup (Keep Kong Running for Verification)

To keep Kong running after the test for verification, pass the `--no-cleanup` flag:

```bash
bash perf-test.sh --no-cleanup
```

Or via make:

```bash
CONFIG_DIR=config-perf-test bash perf-test/perf-test.sh --no-cleanup
```

Then verify the setup:

```bash
# Check Kong is running and get version
curl -H "kong-admin-token: password" http://localhost:8001 | jq .version

# Count workspaces created
curl -H "kong-admin-token: password" http://localhost:8001/workspaces | jq '.data | length'

# List first few workspaces
curl -H "kong-admin-token: password" http://localhost:8001/workspaces | jq '.data[0:3]'

# Cleanup when done
cd perf-test && docker-compose down
```

### Clean Up

Remove generated configurations and test results:

```bash
make perf-clean
```

Stop Kong services:

```bash
cd perf-test
docker-compose down
```

## Environment Setup

### Kong License

The docker-compose file requires a valid Kong license. Set the environment variable:

```bash
export KONG_LICENSE_DATA='your-license-data-here'
```

Or add to `.env` file in the perf-test directory:

```dotenv
KONG_LICENSE_DATA=your-license-data-here
```

### Kong Docker Image

Change the Kong image by setting `KONG_DOCKER_IMAGE`:

```bash
# In .env file
KONG_DOCKER_IMAGE=kong/kong-gateway:3.6.1.7-rhel
```

Or via environment:

```bash
export KONG_DOCKER_IMAGE=kong/kong-gateway:3.6.1.7-rhel
make perf-full
```

## Test Results

Results are saved to `perf-test/test-results/`:

- `kwot_test_TIMESTAMP.txt` - Full output from kwot apply
- `kwot_test_TIMESTAMP_summary.txt` - Timing summary and metrics

View the latest results:

```bash
ls -lh perf-test/test-results/
tail -f perf-test/test-results/kwot_test_*_summary.txt
```

## Example Workflows

### Full End-to-End Test

```bash
# Generate 50 workspaces, start Kong, run kwot, and cleanup
make perf-full NUM_WORKSPACES=50

# View results
cat perf-test/test-results/kwot_test_*_summary.txt | tail -20
```

### Custom Workspace Count

```bash
# Test with 100 workspaces
make perf-full NUM_WORKSPACES=100 CONFIG_DIR=config-perf-100

# Test with 20 workspaces
make perf-full NUM_WORKSPACES=20 CONFIG_DIR=config-perf-20
```

### Reuse Generated Configuration

```bash
# Generate once
make generate-workspaces NUM_WORKSPACES=50

# Run multiple times without regenerating
make perf-test
make perf-test
make perf-test
```

### Manual Kong Control

```bash
# Start Kong separately
cd perf-test
docker-compose up -d

# Run tests independently
make perf-test
make perf-test

# Keep Kong running between tests
make perf-test

# Stop Kong when done
docker-compose down
```

## Performance Baseline

Reference metrics from test runs:

- **50 workspaces**: ~58.7 seconds
- **Configuration generation**: <1 second
- **Kong startup**: ~10-15 seconds
- **Groups created**: 250 (5 per workspace)

## Troubleshooting

### Kong fails to start

Check logs:
```bash
cd perf-test
docker-compose logs kong-gateway-cp
```

Ensure `KONG_LICENSE_DATA` is set:
```bash
echo $KONG_LICENSE_DATA
```

### kwot not found

Ensure kwot is built and in PATH:
```bash
make build install
which kwot
```

### Connection refused

Kong may still be starting. Wait and retry:
```bash
sleep 15
curl http://localhost:8001/status
```

### Port already in use

Another Kong instance is running. Stop it:
```bash
cd perf-test
docker-compose down
```

Or clean up all containers:
```bash
docker-compose -f perf-test/docker-compose.yaml down -v
```

## Files

- `perf-test.sh` - Main test Onboarding script
- `docker-compose.yaml` - Kong services configuration
- `.env` - Environment variables (Kong license, image, etc.)
- `skeleton/` - Template files for workspace configuration
- `test-results/` - Output directory for test results
