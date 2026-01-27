.PHONY: build test lint clean docker-build docker-run help

# Default target
help:
	@echo "Available targets:"
	@echo "  build       - Build backend and frontend"
	@echo "  test        - Run all tests"
	@echo "  lint        - Run all linters"
	@echo "  clean       - Clean build artifacts"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run  - Run Docker container"

# Install dependencies
deps:
	cd web && npm ci

# Build backend
build-backend:
	cd backend && go build -o server .

# Build frontend
build-frontend:
	cd web && npm run build

# Build all
build: build-backend build-frontend

# Test backend
test-backend:
	cd backend && go test -v ./...

# Test frontend
test-frontend:
	cd web && npm run test

# Test all
test: test-backend test-frontend

# Lint backend
lint-backend:
	cd backend && golangci-lint run --disable=errcheck

# Lint frontend
lint-frontend:
	cd web && npm run lint

# Lint all
lint: lint-backend lint-frontend

# Clean artifacts
clean:
	rm -f backend/server
	rm -rf web/dist
	rm -rf web/node_modules

# Docker build
docker-build:
	docker build -t hauler-ui:latest .

# Docker run
docker-run: docker-build
	docker run -p 8080:8080 hauler-ui:latest
