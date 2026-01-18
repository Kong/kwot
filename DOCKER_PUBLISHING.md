# Docker Hub Publishing Setup

To publish kwot Docker images to Docker Hub automatically on releases, follow these steps:

## Prerequisites

1. Docker Hub account (https://hub.docker.com/)
2. GitHub repository with write access

## Setup Instructions

### 1. Create Docker Hub Access Token

1. Go to https://hub.docker.com/settings/security
2. Click "New Access Token"
3. Name it: `kwot-github-actions`
4. Select "Read & Write" permissions
5. Copy the token (you'll only see it once)

### 2. Add GitHub Secrets

1. Go to your GitHub repository settings
2. Navigate to **Secrets and variables** → **Actions**
3. Click **New repository secret**

Add these two secrets:
- `DOCKER_USERNAME`: Your Docker Hub username
- `DOCKER_PASSWORD`: The access token from step 1

### 3. Update Repository Names (Optional)

If you want to use a different Docker Hub repository name, edit `.github/workflows/docker.yml`:

```yaml
env:
  IMAGE_NAME: your-username/your-image-name  # Change this
```

## How It Works

- **Trigger**: Workflow automatically runs when you push a git tag (e.g., `git tag -a v1.0.0`)
- **Build**: Docker image is built using the `Dockerfile`
- **Push**: Image is pushed to Docker Hub with:
  - Version tag: `your-username/kwot:1.0.0`
  - Latest tag: `your-username/kwot:latest`

## Usage After Setup

```bash
# Create a release
git tag -a v1.0.0 -m "Release kwot v1.0.0"
git push origin v1.0.0

# The workflow will automatically:
# 1. Validate the version
# 2. Run tests and linting
# 3. Build the Docker image
# 4. Push to Docker Hub
```

## Pulling Images

```bash
# Latest version
docker pull Kong/kwot:latest

# Specific version
docker pull Kong/kwot:1.0.0

# Run kwot in Docker
docker run -it \
  -e KONG_ADDR=http://kong:8001 \
  -e AUTH_METHOD=RBAC \
  -e ADMIN_TOKEN=your-token \
  -v $(pwd)/config:/app/config \
  Kong/kwot:latest apply --dry-run
```

## Troubleshooting

### Docker image push fails with "unauthorized"

- Verify `DOCKER_USERNAME` and `DOCKER_PASSWORD` secrets are set correctly
- Check that the Docker Hub access token hasn't expired
- Ensure the Docker Hub repository is public or your token has access

### Image not appearing on Docker Hub

- Check the GitHub Actions workflow logs for errors
- Verify the git tag format is correct: `v*` (e.g., `v1.0.0`)
- Ensure the repository is properly connected to Docker Hub
