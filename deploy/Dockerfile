# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Use Aliyun mirror for faster downloads in China
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

# Install dependencies
RUN apk add --no-cache git

# Use multi-proxy fallback chain for better reliability.
# NOTE: use "|" (not ",") so Go can fallback on transient upstream errors.
ENV GOPROXY=https://mirrors.aliyun.com/goproxy/|https://goproxy.cn|https://proxy.golang.org|direct

# Copy go mod files
COPY go.mod go.sum ./
RUN set -eux; \
    for i in 1 2 3; do \
      go mod download && break; \
      echo "go mod download failed (attempt ${i}), retrying..."; \
      sleep $((i * 2)); \
    done

# Copy source code
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/server ./cmd/server

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Use Aliyun mirror for faster downloads in China
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata curl

# Set timezone
ENV TZ=Asia/Shanghai

# Copy binary from builder
COPY --from=builder /bin/server /app/server

# Expose port
EXPOSE 8000

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8000/health || exit 1

# Run
CMD ["/app/server"]
