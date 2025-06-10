# Build stage
FROM golang:1.23-bookworm AS builder

# Install build dependencies
RUN apt-get update && apt-get install -y \
    git \
    ca-certificates \
    gcc \
    libc6-dev \
    libsqlite3-dev \
    && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
ARG VERSION=dev
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o unifi-gate-opener ./cmd/main.go

# Runtime stage
FROM ubuntu:24.04

# Install runtime dependencies
RUN apt-get update && apt-get install -y \
    ca-certificates \
    sqlite3 \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN groupadd --gid 1001 appgroup && \
    useradd --uid 1001 --gid appgroup --shell /bin/bash --create-home appuser

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/unifi-gate-opener .

# Create data directory with proper permissions
RUN mkdir -p data && chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/ || exit 1

# Default environment variables
ENV GIN_MODE=release
ENV DATABASE_PATH=/app/data/gate_opener.db
ENV CONFIG_PATH=/app/data/config.yaml

# Run the application
CMD ["./unifi-gate-opener", "-config", "/app/data/config.yaml"]