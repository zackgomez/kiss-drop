# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY *.go ./
COPY templates/ ./templates/
COPY static/ ./static/

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o kiss-drop .

# Final stage
FROM alpine:3.21

# Add ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/kiss-drop .

# Create data directory
RUN mkdir -p /data

# Environment defaults
ENV PORT=8080
ENV DATA_DIR=/data

EXPOSE 8080

# Run as non-root user
RUN adduser -D -u 1000 kissuser
RUN chown -R kissuser:kissuser /data
USER kissuser

ENTRYPOINT ["/app/kiss-drop"]
