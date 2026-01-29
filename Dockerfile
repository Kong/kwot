# Build stage - use specific Go version for reproducibility and security
FROM golang:1.24.12-alpine AS builder

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build arguments for version info
ARG VERSION=dev

# Build the application with version info
RUN CGO_ENABLED=0 GOOS=linux go build \
    -a -installsuffix cgo \
    -ldflags="-X main.Version=${VERSION}" \
    -o kwot .

# Runtime stage - use lightweight Alpine with latest security patches
FROM alpine:3.21.2

# Build arguments for labels
ARG VERSION=dev

# Labels for container metadata
LABEL maintainer="Kong"
LABEL description="kwot - Kong Workspace Onboarding Tool"
LABEL version="${VERSION:-dev}"

# Install only runtime dependencies
RUN apk add --no-cache ca-certificates

# Create non-root user for security
RUN addgroup -g 1000 kwot && \
    adduser -D -u 1000 -G kwot kwot

WORKDIR /home/kwot

# Copy binary from builder
COPY --from=builder /app/kwot /usr/local/bin/kwot

# Copy default config directory
COPY --from=builder /app/config ./config

# Set file ownership to non-root user
RUN chown -R kwot:kwot /home/kwot

# Switch to non-root user
USER kwot

# Set entrypoint
ENTRYPOINT ["kwot"]

# Default command
CMD ["--help"]

# Health check (validates binary works)
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD kwot --version || exit 1
