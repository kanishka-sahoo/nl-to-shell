# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build arguments for version information
ARG VERSION=0.1.0-dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown

# Build the binary with static linking
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X 'github.com/kanishka-sahoo/nl-to-shell/internal/cli.Version=${VERSION}' -X 'github.com/kanishka-sahoo/nl-to-shell/internal/cli.GitCommit=${GIT_COMMIT}' -X 'github.com/kanishka-sahoo/nl-to-shell/internal/cli.BuildDate=${BUILD_DATE}'" \
    -o nl-to-shell \
    ./cmd/nl-to-shell

# Final stage
FROM scratch

# Copy CA certificates for HTTPS requests
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy the binary
COPY --from=builder /app/nl-to-shell /nl-to-shell

# Set the binary as entrypoint
ENTRYPOINT ["/nl-to-shell"]

# Default command
CMD ["--help"]

# Metadata
LABEL org.opencontainers.image.title="nl-to-shell"
LABEL org.opencontainers.image.description="Convert natural language to shell commands using LLMs"
LABEL org.opencontainers.image.source="https://github.com/kanishka-sahoo/nl-to-shell"
LABEL org.opencontainers.image.licenses="MIT"