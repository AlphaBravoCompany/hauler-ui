# Multi-stage build for single container image
FROM node:20-alpine AS web-builder

WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.23-alpine AS backend-builder

WORKDIR /backend
COPY backend/go.mod backend/go.sum* ./
RUN go mod download || true
COPY backend/ ./
RUN CGO_ENABLED=0 go build -o /app/server .

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

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
