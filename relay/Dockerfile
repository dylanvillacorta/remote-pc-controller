# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy dependency definition
COPY go.mod ./
RUN go mod download

# Copy source code
COPY src ./src

# Compile statically linked binary with stripped symbols
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /relay ./src

# Runtime stage
FROM alpine:3.21

# Install CA certificates and timezone data for TLS outbound calls and logs
RUN apk --no-cache add ca-certificates tzdata

# Create dedicated non-root user and group
RUN addgroup -g 10001 -S appgroup && \
    adduser -u 10001 -S appuser -G appgroup

WORKDIR /app

# Copy executable from builder
COPY --from=builder /relay /app/relay

# Set ownership
RUN chown -R appuser:appgroup /app

# Use non-root user
USER 10001:10001

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/health || exit 1

ENTRYPOINT ["/app/relay"]
