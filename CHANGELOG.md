# Changelog

All notable changes to Hauler UI will be documented in this file.

## [0.1.0-alpha] - 2025-01-28

### Added

#### Store Operations
- Add container images to the store with platform specification
- Add Helm charts from repositories with dependency and image support
- Add local files or remote URLs to the store
- Sync store from manifest files with platform and registry overrides
- Save store as portable `.tar.zst` archives with checksums
- Load archives into the store with optional clear-before-load
- Copy store contents to external registries or directories
- Remove artifacts from the store using pattern matching
- View store contents with filtering by images, charts, and files
- Track source haul for each stored item

#### Registry Management
- Login to Docker Hub, GHCR, and other container registries
- Logout from registries with credential cleanup
- Secure credential storage using standard Docker auth pattern
- View stored credential information and paths

#### Serve Operations
- Start/stop embedded container registry (default port 5000)
- Start/stop embedded HTTP fileserver (default port 5001)
- TLS support with auto-generated self-signed certificates
- Custom TLS certificate and key paths
- Read-only mode for registry serving
- Configurable timeouts for fileserver
- Process monitoring with PID tracking and status display
- Access URLs for both registry and fileserver

#### Manifest Management
- Create, edit, and delete Hauler manifests
- Monaco Editor (VS Code editor) for YAML editing
- Manifest tagging for organization
- Download manifests as YAML files
- Store contents linked to manifests
- Support for product and product-registry manifests

#### Job Management
- Background job execution for all long-running operations
- Real-time log streaming via Server-Sent Events (SSE)
- Job history with timestamps and duration tracking
- Job status monitoring (queued, running, succeeded, failed)
- Exit code tracking and error reporting
- Bulk job deletion with confirmation
- Job detail pages with full command and output

#### Authentication & Security
- Optional password-based UI access control (`HAULER_UI_PASSWORD`)
- Session management with 24-hour expiration
- httpOnly and SameSite=Strict cookies
- Protected routes requiring authentication
- Password never stored in database (credential passthrough only)
- Path traversal protection in file operations

#### Settings & Configuration
- Global settings stored in SQLite database
- Environment variable precedence over database settings
- Configurable log level (debug, info, warn, error)
- Configurable retry count for failed operations
- Ignore errors flag for continued operations
- Default platform specification for multi-arch operations
- Default key path for signature verification
- Temporary directory path configuration

#### User Interface
- Dashboard with system health and Hauler CLI version
- Sidebar navigation organized by Main, Operations, and System
- Responsive design with mobile hamburger menu
- Job indicator showing running job count
- Form validation with error messages
- Loading states and progress indicators
- Success/error notification cards
- Confirmation dialogs for destructive actions
- Dark-themed UI with accent colors

#### API & Backend
- RESTful API with JSON responses
- SQLite database for persistence
- CRUD endpoints for jobs, settings, manifests, and store contents
- Health check endpoint (`/healthz`)
- Configuration endpoint showing all system paths
- Capabilities endpoint reporting Hauler CLI version
- Streaming log endpoints for real-time job output
- Archive download with range request support

#### Deployment
- Single-container Alpine-based Docker image
- Multi-stage build for minimal image size
- Docker Compose configuration with all ports exposed
- Environment variable configuration with defaults
- Persistent data volume at `/data`
- Comprehensive `.env.example` for configuration reference
- Makefile with build, lint, and test commands

### Documentation
- Comprehensive README with quick start guide
- Development guide with local setup instructions
- Runbook with common workflows and troubleshooting
- Persistence documentation explaining data storage
- Inline API documentation comments in Go backend

### Known Limitations
- Podman-generated tarballs are not supported for `store load`
- When copying to a registry path, login to the registry root first
- Large operations (sync, save) may require significant temporary space
- Self-signed certificates trigger browser security warnings

---

### Version Format

Changelog entries follow [Keep a Changelog](https://keepachangelog.com/en/1.0.0/) format:
- **Added** - New features
- **Changed** - Changes to existing functionality
- **Deprecated** - Features to be removed in future releases
- **Removed** - Features removed in this release
- **Fixed** - Bug fixes
- **Security** - Security vulnerability fixes
