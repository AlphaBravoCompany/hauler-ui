# Container Persistence Model

## Overview

Hauler UI uses `/data` as the persistent root directory for all stored data. This directory must be mounted as a volume to ensure data persists across container restarts.

## Required Volume Mount

The container expects a volume mount at `/data`:

```bash
docker run -v ./data:/data -p 8080:8080 hauler-ui:latest
```

Or in docker-compose.yml:

```yaml
services:
  hauler-ui:
    image: hauler-ui:latest
    ports:
      - "8080:8080"
    volumes:
      - ./data:/data
```

## Directory Structure

Within `/data`, the following structure is created:

```
/data/
├── store/          # HAULER_STORE_DIR - Hauler content store
├── tmp/            # HAULER_TEMP_DIR - Temporary files
└── .docker/
    └── config.json # Docker registry credentials
```

## Environment Variables

### HAULER_DIR (default: `/data`)
Base directory for hauler data.

### HAULER_STORE_DIR (default: `/data/store`)
Location where hauler stores downloaded images, charts, and files.

### HAULER_TEMP_DIR (default: `/data/tmp`)
Temporary storage for operations like sync and extract.

### DOCKER_CONFIG (default: `/data/.docker`)
Directory containing Docker registry authentication.

### HOME (default: `/data`)
Set to `/data` so that `hauler login` writes credentials to the persistent volume.

## Docker Authentication

When you run `hauler login <registry>`, credentials are stored in:
- **Container path**: `/data/.docker/config.json` (or `~/.docker/config.json` which resolves to the same location)
- **Host mount path**: `./data/.docker/config.json`

This follows the standard Docker auth pattern used by the hauler CLI.

**Important**: The `~/.docker/config.json` file is **inside the container** at `/data/.docker/config.json`. It is persisted to your host via the volume mount.

## Permissions

The container creates directories with `755` permissions. Ensure the host mount directory is writable by the container user (typically root or the user specified in the image).

## Storage Requirements

- **Store size**: Proportional to content added (images, charts, files)
- **Temp size**: Temporary space during sync/save/load operations
- **Docker config**: Negligible (a few KB)

Plan host volume space accordingly based on your intended usage.
