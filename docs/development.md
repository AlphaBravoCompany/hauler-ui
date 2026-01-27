# Development Guide

## Overview

Hauler UI is developed using Ralph TUI for task orchestration. The project uses `prd.json` to define user stories and acceptance criteria.

## Ralph TUI Workflow

### Prerequisites

Install Ralph TUI following the official documentation.

### Running with prd.json

The `prd.json` file in the project root contains all user stories for the Hauler UI project:

```json
{
  "name": "Hauler Web UI (single-user)",
  "description": "A single-container, beginner-friendly web UI...",
  "branchName": "ralph/hauler-webui",
  "userStories": [...]
}
```

### Converting PRD to Beads

To use Ralph TUI's bead-based workflow:

```bash
# Convert PRD to beads
ralph beads create prd.json

# This creates an epic with child beads for each user story
```

### Converting PRD to JSON Tasks

Alternatively, use the JSON task format:

```bash
# Convert PRD to prd.json format for ralph-tui execution
ralph json create prd.json
```

### Story Status

Track user story completion in `prd.json`:

```json
{
  "id": "US-024",
  "title": "Documentation: runbook and troubleshooting",
  "passes": true,
  "completionNotes": "Documentation completed"
}
```

## Local Development

### Quick Start

```bash
# Install dependencies
make deps

# Run backend (from project root)
cd backend && go run .

# Run frontend (in another terminal)
cd web && npm run dev
```

The frontend dev server runs on http://localhost:5173 and proxies API requests to the backend on port 8080.

### Development with Docker

```bash
# Build and run with hot-reload
docker-compose -f deploy/docker-compose.dev.yml up
```

Note: A dev compose file would need to be created for this workflow.

## Project Structure

```
hauler-ui/
├── backend/                 # Go backend
│   ├── internal/
│   │   ├── auth/           # Authentication and sessions
│   │   ├── config/         # Configuration management
│   │   ├── hauler/         # Hauler CLI integration
│   │   ├── jobrunner/      # Async job execution
│   │   ├── manifests/      # Manifest CRUD
│   │   ├── registry/       # Registry login/logout
│   │   ├── serve/          # Serve operations
│   │   ├── settings/       # Settings management
│   │   ├── sqlite/         # Database operations
│   │   └── store/          # Store operations
│   ├── main.go             # Entry point
│   └── go.mod              # Go dependencies
├── web/                    # React frontend
│   ├── src/
│   │   ├── components/     # Reusable components
│   │   ├── pages/          # Page components
│   │   └── lib/            # Utilities
│   ├── package.json        # Node dependencies
│   └── vite.config.ts      # Vite configuration
├── deploy/
│   └── docker-compose.yml  # Production compose
├── docs/                   # Documentation
│   ├── runbook.md
│   ├── persistence.md
│   ├── limitations.md
│   └── development.md
├── Dockerfile              # Multi-stage build
├── Makefile               # Build automation
└── prd.json               # Ralph TUI stories
```

## Adding a New User Story

1. Edit `prd.json` and add a new user story:
```json
{
  "id": "US-XXX",
  "title": "Feature: description",
  "description": "As a user, I want...",
  "acceptanceCriteria": [
    "Criterion 1",
    "Criterion 2"
  ],
  "priority": N,
  "passes": false,
  "dependsOn": ["US-YYY"]
}
```

2. Convert to beads or JSON tasks for Ralph TUI

3. Implement following the acceptance criteria

4. Update `passes: true` when complete

## Code Quality

### Running Tests

```bash
make test
```

### Running Linters

```bash
make lint
```

### Building

```bash
make build
```

## Git Workflow

Feature branches follow the pattern `ralph/hauler-webui` or feature-specific names.

### Commit Messages

Follow conventional commit format:
```
feat: US-XXX - Description
fix: US-XXX - Bug fix description
docs: US-XXX - Documentation updates
```

### Before Committing

1. Ensure `make build` passes
2. Ensure `make lint` passes
3. Ensure `make test` passes
4. Update `prd.json` with completion status

## API Endpoints

### Public Routes
- `GET /healthz` - Health check
- `GET /api/config` - Current configuration
- `POST /api/login` - UI authentication (if password set)
- `POST /api/logout` - Clear session

### Authenticated Routes (if password set)
- `GET /api/jobs` - List jobs
- `POST /api/jobs` - Create job
- `GET /api/jobs/:id/stream` - SSE job logs
- `POST /api/registry/login` - Registry login
- `POST /api/registry/logout` - Registry logout
- And more...

## Troubleshooting Development Issues

### Port 8080 Already in Use

Change the backend port:
```bash
export PORT=9090
cd backend && go run .
```

### Frontend API Proxy Issues

Edit `web/vite.config.ts` to point to the correct backend port.

### Hauler CLI Not Found

The container includes Hauler CLI. For local development, ensure Hauler is installed:
```bash
which hauler
hauler version
```

## Contributing

1. Create a new user story in `prd.json`
2. Implement following acceptance criteria
3. Add tests for new functionality
4. Update documentation
5. Submit with conventional commit message
