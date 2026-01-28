# Multi-stage build for single container image
FROM node:20-alpine AS web-builder

WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.24-alpine AS backend-builder

WORKDIR /backend
COPY backend/go.mod backend/go.sum* ./
RUN go mod download || true
COPY backend/ ./
RUN CGO_ENABLED=0 go build -o /app/server .

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

# Install hauler CLI (pinned version 1.3.2, multi-arch aware)
ARG TARGETPLATFORM
ARG TARGETARCH
RUN apk add --no-cache curl && \
    HAULER_VERSION="1.3.2" && \
    curl -sfSL "https://github.com/hauler-dev/hauler/releases/download/v${HAULER_VERSION}/hauler_${HAULER_VERSION}_checksums.txt" -o /tmp/checksums.txt && \
    curl -sfSL "https://github.com/hauler-dev/hauler/releases/download/v${HAULER_VERSION}/hauler_${HAULER_VERSION}_linux_${TARGETARCH}.tar.gz" -o /tmp/hauler.tar.gz && \
    CHECKSUM=$(awk '$2 == "hauler_'${HAULER_VERSION}'_linux_'${TARGETARCH}'.tar.gz" {print $1}' /tmp/checksums.txt) && \
    echo "${CHECKSUM}  /tmp/hauler.tar.gz" | sha256sum -c - && \
    tar -xzf /tmp/hauler.tar.gz -C /tmp && \
    install -m 755 /tmp/hauler /usr/local/bin/hauler && \
    rm -f /tmp/hauler.tar.gz /tmp/hauler /tmp/checksums.txt && \
    apk del curl

# Create /data directory for persistent storage
# This directory should be mounted as a volume for:
# - HAULER_STORE_DIR (default: /data/store)
# - HAULER_TEMP_DIR (default: /data/tmp)
# - Docker auth config (default: /data/.docker/config.json)
RUN mkdir -p /data/store /data/tmp /data/.docker && \
    chmod 755 /data /data/store /data/tmp /data/.docker

WORKDIR /app

# Copy built backend
COPY --from=backend-builder /app/server /app/server

# Copy built frontend assets
COPY --from=web-builder /web/dist /app/web

EXPOSE 8080

ENV PORT=8080

# Hauler directory configuration
ENV HAULER_DIR=/data
ENV HAULER_STORE_DIR=/data/store
ENV HAULER_TEMP_DIR=/data/tmp

# Docker config location for registry credentials
ENV HOME=/data
ENV DOCKER_CONFIG=/data/.docker

VOLUME ["/data"]

CMD ["/app/server"]
