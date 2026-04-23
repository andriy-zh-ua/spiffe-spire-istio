# ---------- Build stage ----------
FROM golang:1.26-bookworm AS builder

# Build static binary for the target platform
ARG TARGETOS=linux
ARG TARGETARCH=arm64

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build static binary
RUN CGO_ENABLED=0 \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    GO111MODULE=auto \
    go build -v \
             -ldflags="-w -s -extldflags '-static'" \
             -o /app/client .

# ---------- Runtime stage ----------
FROM debian:bookworm-slim  AS runtime

# Install only what is needed
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates file \
    && rm -rf /var/lib/apt/lists/* \
    && apt-get clean

# Create group and user with login shell enabled
RUN groupadd -g 10001 appuser \
    && useradd -u 10001 -g appuser \
               -m \
               -s /bin/bash \
               appuser

# Set working directory
WORKDIR /app

# Copy the built binary from builder stage
COPY --from=builder --chown=appuser:appuser /app/client /app/client

# Make the binary executable
RUN chmod 755 /app/client

# Run as non-root user
USER appuser

# Run the binary
CMD ["/app/client"]