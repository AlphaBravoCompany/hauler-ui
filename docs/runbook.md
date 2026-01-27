# Hauler UI Runbook

## Overview

Hauler UI is a single-container web interface for the Rancher Government Hauler CLI. It provides full operational parity with Hauler's command-line interface including store management, registry operations, and artifact synchronization.

**Port**: 8080 (HTTP)
**Persistent Data**: `/data` (required volume mount)

## Quick Start

### Using Docker Compose (Recommended)

Create a `docker-compose.yml`:

```yaml
services:
  hauler-ui:
    image: hauler-ui:latest
    ports:
      - "8080:8080"
    volumes:
      - ./data:/data
    environment:
      - HAULER_UI_PASSWORD=your-optional-password
```

Then run:

```bash
docker compose up -d
```

Access the UI at http://localhost:8080

### Using Docker Run

```bash
# Create data directory
mkdir -p ./data

# Run the container
docker run -d \
  --name hauler-ui \
  -p 8080:8080 \
  -v $(pwd)/data:/data \
  -e HAULER_UI_PASSWORD=your-optional-password \
  hauler-ui:latest
```

## Building from Source

### Prerequisites

- Go 1.23+
- Node.js 20+
- Docker (for container build)

### Build Steps

```bash
# Clone the repository
git clone <repository-url>
cd hauler-ui

# Using Make (recommended)
make build

# Or manually
cd backend && go build -o server .
cd ../web && npm install && npm run build
```

### Build Docker Image

```bash
# Using Make
make docker-build

# Or manually
docker build -t hauler-ui:latest .
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP port for the UI |
| `HAULER_DIR` | `/data` | Base directory for hauler data |
| `HAULER_STORE_DIR` | `/data/store` | Content storage location |
| `HAULER_TEMP_DIR` | `/data/tmp` | Temporary files location |
| `DOCKER_CONFIG` | `/data/.docker` | Docker auth config directory |
| `HAULER_UI_PASSWORD` | (empty) | Optional password for UI access |
| `DATABASE_PATH` | `/data/app.db` | SQLite database path |

### Volume Mounts

The `/data` directory is **required** for persistence:

```bash
-v /host/path/data:/data
```

Without this mount, all data (store contents, registry credentials, settings) will be lost when the container restarts.

## Accessing the UI

1. Open http://localhost:8080 in your browser
2. If `HAULER_UI_PASSWORD` is set, enter the password
3. The dashboard shows available operations and current job status

## Common Workflows

### Beginner Wizards

The UI includes guided workflows for common airgap scenarios:

1. **Build Store** - Add images, charts, or files and sync from manifests
2. **Package Haul** - Save store to archive with checksum
3. **Deploy in Airgap** - Load archive and serve or copy to registry

### Advanced Operations

Each wizard links to detailed pages for:
- **Store** - Add/remove images, charts, files
- **Manifests** - Create and manage sync manifests
- **Serve** - Run embedded registry or fileserver
- **Copy/Export** - Copy store to external registry or directory
- **Registry Login** - Authenticate to container registries
- **Settings** - Configure global flags and defaults

## Authentication

### UI Authentication

Setting `HAULER_UI_PASSWORD` enables single-user password protection:

```bash
docker run -d \
  -v ./data:/data \
  -p 8080:8080 \
  -e HAULER_UI_PASSWORD=mys3cr3t \
  hauler-ui:latest
```

- Sessions expire after 24 hours
- Cookie is httpOnly and SameSite=Strict
- No multi-user support - single shared password

### Registry Login

Registry credentials are stored in Docker config format:

```bash
# Via UI: Use the "Registry Login" page
# Via CLI:
hauler login registry.example.com --username user --password pass
```

Credentials are stored at `/data/.docker/config.json` (inside container) or `./data/.docker/config.json` (on host).

## Data Management

### Backup

To backup your hauler data:

```bash
# Stop the container
docker stop hauler-ui

# Backup the data directory
tar czf hauler-backup-$(date +%Y%m%d).tar.gz ./data

# Restart the container
docker start hauler-ui
```

### Restore

```bash
# Stop the container
docker stop hauler-ui

# Restore the data directory
rm -rf ./data
tar xzf hauler-backup-YYYYMMDD.tar.gz

# Restart the container
docker start hauler-ui
```

### Migration to New Container

```bash
# Stop old container
docker stop hauler-ui
docker rm hauler-ui

# Pull new image
docker pull hauler-ui:new-version

# Start with same volume mount
docker run -d \
  --name hauler-ui \
  -p 8080:8080 \
  -v $(pwd)/data:/data \
  hauler-ui:new-version
```

## Logs

View container logs:

```bash
# Follow logs
docker logs -f hauler-ui

# Last 100 lines
docker logs --tail 100 hauler-ui
```

Job logs are also available in the UI under "Job History".

## Troubleshooting

### Container Won't Start

1. **Port 8080 already in use**
   ```bash
   # Check what's using port 8080
   lsof -i :8080
   # Or use a different port
   docker run -p 9090:8080 ...
   ```

2. **Permission denied on /data**
   ```bash
   # Fix permissions
   chmod 755 ./data
   ```

### Registry Login Fails

1. **Incorrect credentials** - Verify username and password
2. **Registry URL** - Include the full registry URL (e.g., `registry.example.com`, not `https://registry.example.com`)
3. **Existing auth conflict** - Remove old credentials:
   ```bash
   rm ./data/.docker/config.json
   docker restart hauler-ui
   ```

### Store Operations Fail

1. **Out of disk space** - Check volume space:
   ```bash
   df -h ./data
   ```

2. **Temp directory full** - Clear temp files:
   ```bash
   rm -rf ./data/tmp/*
   ```

3. **Network issues** - Check connectivity to registries from within container:
   ```bash
   docker exec hauler-ui wget -O- https://registry.example.com/v2/
   ```

### Job Stuck Running

1. Check job logs in the UI
2. Cancel the job from the Job History page
3. If unresponsive, restart the container:
   ```bash
   docker restart hauler-ui
   ```

### Serve Operations Not Accessible

The embedded registry/fileserver run on separate ports:

- **Registry**: Default port 5000 (needs `-p 5000:5000`)
- **Fileserver**: Default port 8080 (conflicts with UI!)

Run with additional port mappings:

```bash
docker run -d \
  -p 8080:8080 \
  -p 5000:5000 \
  -v ./data:/data \
  hauler-ui:latest
```

## Development

For development with live reload:

```bash
# Backend
cd backend
go run .

# Frontend (separate terminal)
cd web
npm run dev
```

The frontend dev server runs on port 5173.

### Quality Checks

```bash
make lint   # Run all linters
make test   # Run all tests
make build  # Verify build succeeds
```

## Additional Resources

- [Persistence Documentation](./persistence.md)
- [Ralph TUI Development](./development.md#ralph-tui)
- Rancher Government Hauler CLI documentation
