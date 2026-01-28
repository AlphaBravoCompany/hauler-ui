# Container Persistence Model

## Overview

Hauler UI uses `/data` as the persistent root directory for all stored data. This directory must be mounted as a volume to ensure data persists across container restarts.

## Why /data Must Be Mounted

All application state is stored within `/data`:
- **Store content** - Downloaded images, charts, and files
- **Registry credentials** - Docker config.json with auth tokens
- **Database** - Job history, settings, and saved manifests
- **Temporary files** - Working space during sync/save/load operations

Without a volume mount, **all data is lost** when the container is removed or restarted.

## Required Volume Mount

The container expects a volume mount at `/data`:

```bash
docker run -v ./data:/data -p 8080:8080 hauler-ui:latest
```

Or in docker-compose.yml:

```yaml
services:
  hauler-ui:
    build: ..
    ports:
      - "${PORT:-8080}:8080"   # Main UI
      - "5000:5000"            # Registry serve
      - "5001:5001"            # Fileserver serve
    volumes:
      - ./data:/data
    environment:
      - PORT=${PORT:-8080}
      - HAULER_UI_PASSWORD=${HAULER_UI_PASSWORD:-}
      - HAULER_LOG_LEVEL=${HAULER_LOG_LEVEL:-info}
      - HAULER_IGNORE_ERRORS=${HAULER_IGNORE_ERRORS:-false}
      - HAULER_RETRIES=${HAULER_RETRIES:-3}
```

> **Note**: The authoritative docker-compose configuration is in `deploy/docker-compose.yml` with environment variable defaults in `deploy/.env.example`.

## Directory Structure

Within `/data`, the following structure is created:

```
/data/
├── store/          # HAULER_STORE_DIR - Hauler content store
├── tmp/            # HAULER_TEMP_DIR - Temporary files
├── .docker/
│   └── config.json # Docker registry credentials
└── app.db          # SQLite database (jobs, settings, manifests)
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `HAULER_DIR` | `/data` | Base directory for hauler data |
| `HAULER_STORE_DIR` | `/data/store` | Location where hauler stores downloaded images, charts, and files |
| `HAULER_TEMP_DIR` | `/data/tmp` | Temporary storage for operations like sync and extract |
| `DOCKER_CONFIG` | `/data/.docker` | Directory containing Docker registry authentication |
| `HOME` | `/data` | Set to `/data` so that `hauler login` writes credentials to the persistent volume |

For the complete list of environment variables, see `deploy/.env.example`.

## Registry Login Storage

### How It Works

When you run `hauler login <registry>` (via CLI or UI), credentials are stored following the standard Docker auth pattern:

- **Container path**: `/data/.docker/config.json` (or `~/.docker/config.json` which resolves to the same location since `HOME=/data`)
- **Host mount path**: `./data/.docker/config.json`
- **Format**: Standard Docker config.json with base64-encoded auth tokens

### Login Flow via UI

1. User navigates to "Registry Login" page
2. Enters registry URL, username, and password
3. Backend creates an async job running `hauler login`
4. Credentials are passed via environment variables (not CLI args) for security
5. Hauler writes credentials to `/data/.docker/config.json`
6. Credentials persist for future operations (sync, copy, etc.)

### Viewing Stored Credentials

On the host:

```bash
cat ./data/.docker/config.json
```

Inside the container:

```bash
docker exec hauler-ui cat /data/.docker/config.json
```

### Clearing Credentials

```bash
# Remove the config file
rm ./data/.docker/config.json

# Or logout via UI or CLI
docker exec hauler-ui hauler logout registry.example.com
```

### Credential Security

- **In UI**: Passwords are never stored in the SQLite database
- **In transit**: Credentials passed via environment variables to child process
- **At rest**: Stored in Docker config.json format (base64 encoded)
- **Logs**: Job logs redact sensitive information

### Multiple Registries

You can be logged into multiple registries simultaneously. Each login adds an entry to the config.json:

```json
{
  "auths": {
    "registry.example.com": {
      "auth": "base64(username:password)"
    },
    "ghcr.io": {
      "auth": "base64(user:token)"
    }
  }
}
```

## Permissions

The container creates directories with `755` permissions. Ensure the host mount directory is writable by the container user (typically root or the user specified in the image).

## Storage Requirements

- **Store size**: Proportional to content added (images, charts, files)
- **Temp size**: Temporary space during sync/save/load operations
- **Docker config**: Negligible (a few KB)

Plan host volume space accordingly based on your intended usage.
