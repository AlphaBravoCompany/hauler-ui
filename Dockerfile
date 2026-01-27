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

WORKDIR /app

# Copy built backend
COPY --from=backend-builder /app/server /app/server

# Copy built frontend assets
COPY --from=web-builder /web/dist /app/web

EXPOSE 8080

ENV PORT=8080

CMD ["/app/server"]
