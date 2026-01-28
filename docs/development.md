# Development Guide

## Overview

This guide covers setting up a local development environment, building, testing, and contributing to Hauler UI.

## Prerequisites

- **Go** 1.24+ — Backend development
- **Node.js** 20+ — Frontend development
- **Make** — Convenient build commands (optional)
- **Git** — Version control
- **Docker** — Container builds and local testing

## Local Development Setup

### 1. Clone the Repository

```bash
git clone https://github.com/rancherfederal/hauler-ui.git
cd hauler-ui
```

### 2. Install Dependencies

```bash
# Install Go dependencies
cd backend && go mod download

# Install Node dependencies
cd web && npm install
```

Or use the make command:

```bash
make deps
```

### 3. Run Development Servers

The backend and frontend run separately in development.

**Backend (terminal 1):**
```bash
cd backend && go run .
```

The backend serves on `http://localhost:8080` by default.

**Frontend (terminal 2):**
```bash
cd web && npm run dev
```

The frontend dev server runs on `http://localhost:5173` and proxies API requests to the backend.

## Project Structure

```
hauler-ui/
├── backend/                 # Go backend server
│   ├── internal/
│   │   ├── auth/           # Authentication & sessions
│   │   ├── config/         # Configuration management
│   │   ├── hauler/         # Hauler CLI integration
│   │   ├── jobrunner/      # Background job execution
│   │   ├── manifests/      # Manifest CRUD operations
│   │   ├── registry/       # Registry login/logout
│   │   ├── serve/          # Registry & fileserver serving
│   │   ├── settings/       # Global settings management
│   │   ├── sqlite/         # Database operations
│   │   └── store/          # Store operations
│   ├── main.go             # Application entry point
│   └── go.mod              # Go dependencies
├── web/                    # React frontend
│   ├── src/
│   │   ├── components/     # Reusable UI components
│   │   ├── contexts/       # React Context providers
│   │   ├── pages/          # Page components
│   │   └── lib/            # Utilities and API client
│   ├── package.json        # Node dependencies
│   └── vite.config.ts      # Vite configuration
├── deploy/                 # Deployment configurations
│   └── docker-compose.yml  # Production compose file
├── docs/                   # Additional documentation
├── Dockerfile              # Multi-stage container build
└── Makefile               # Build automation
```

## Build Commands

### Using Make (Recommended)

```bash
# Install all dependencies
make deps

# Build everything (backend + frontend)
make build

# Build backend only
make build-backend

# Build frontend only
make build-frontend

# Build Docker image
make docker-build

# Run tests
make test

# Run linters
make lint

# Clean build artifacts
make clean
```

### Manual Build

**Backend:**
```bash
cd backend
go build -o ../bin/hauler-ui .
```

**Frontend:**
```bash
cd web
npm run build
```

**Docker:**
```bash
docker build -t hauler-ui:latest .
```

## Testing

### Backend Tests

```bash
cd backend
go test ./...
```

### Frontend Tests

```bash
cd web
npm test
```

### End-to-End Tests

```bash
npm run test:e2e
```

## Configuration

Development configuration can be set via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Backend HTTP server port |
| `HAULER_DIR` | `./data` | Hauler working directory |
| `HAULER_STORE_DIR` | `./data/store` | Store directory path |
| `HAULER_TEMP_DIR` | `./data/tmp` | Temporary files directory |
| `HAULER_UI_PASSWORD` | (none) | Optional UI password |

For local development, create a `.env` file in the backend directory:

```
PORT=8080
HAULER_DIR=./data
```

## API Endpoints

### Public Routes
- `GET /healthz` — Health check
- `GET /api/config` — Current configuration
- `POST /api/login` — UI authentication (if password set)
- `POST /api/logout` — Clear session

### Authenticated Routes (if password set)
- `GET /api/jobs` — List jobs
- `POST /api/jobs` — Create job
- `GET /api/jobs/:id/stream` — SSE job logs
- `DELETE /api/jobs/:id` — Delete job
- `POST /api/registry/login` — Registry login
- `POST /api/registry/logout` — Registry logout
- `GET /api/store` — Store contents
- `POST /api/store/add` — Add to store
- `POST /api/store/save` — Save archive
- `POST /api/store/load` — Load archive
- `POST /api/store/extract` — Extract archive
- `GET /api/manifests` — List manifests
- `POST /api/manifests` — Create manifest
- `PUT /api/manifests/:id` — Update manifest
- `DELETE /api/manifests/:id` — Delete manifest
- `GET /api/settings` — Global settings
- `PUT /api/settings` — Update settings
- `POST /api/serve/registry` — Start registry
- `POST /api/serve/fileserver` — Start fileserver
- `DELETE /api/serve/:type` — Stop serve operation

## Troubleshooting

### Port 8080 Already in Use

Change the backend port:
```bash
export PORT=9090
cd backend && go run .
```

Then update `web/vite.config.ts` to proxy to the new port.

### Hauler CLI Not Found

For local development outside the container, install Hauler CLI:
```bash
# Follow installation instructions at:
# https://github.com/rancherfederal/hauler
```

### Frontend Build Errors

Clear the node_modules and reinstall:
```bash
cd web
rm -rf node_modules package-lock.json
npm install
```

### Database Issues

Reset the local development database:
```bash
rm -f backend/data/app.db
```

## Contributing

We welcome contributions! Please follow these guidelines:

### Git Workflow

1. **Fork** the repository on GitHub
2. **Create a feature branch** from `main`
   ```bash
   git checkout -b feature/your-feature-name
   ```
3. **Make your changes** and test thoroughly
4. **Commit** with clear, descriptive messages
5. **Push** to your fork
6. **Create a pull request**

### Commit Messages

Follow conventional commit format:

```
feat: add support for custom registry URLs
fix: resolve job timeout issue
docs: update API documentation
refactor: simplify store operations
test: add integration tests for auth
```

### Before Submitting

1. **Build passes** — `make build` succeeds
2. **Tests pass** — `make test` succeeds
3. **Linting passes** — `make lint` succeeds
4. **Documentation updated** — Update relevant docs if needed

### Code Style

- **Go**: Follow standard Go formatting (`gofmt`)
- **JavaScript/TypeScript**: Use ESLint configuration
- **Comments**: Document public functions and complex logic
- **Naming**: Use clear, descriptive names

## Getting Help

- **Issues**: [GitHub Issues](https://github.com/rancherfederal/hauler-ui/issues)
- **Documentation**: [Hauler Docs](https://rancherfederal.github.io/hauler/)
- **Community**: [Rancher Government Slack](https://ranchergovernment.slack.com/)
