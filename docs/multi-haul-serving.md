# Design: Multi-Haul Serving (Publish Layer)

**Status:** Implemented — host-routed registry proxy, direct file serving, and
TLS are live (`internal/publish`). The single-process registry in "Process
model" below remains a future option.
**Audience:** hauler-ui maintainers
**Related:** isolated per-haul stores (`/data/hauls/<slug>/store`)

## Process model & footprint

Today, **each published haul runs one `hauler store serve registry`
subprocess**, bound to a private `127.0.0.1` ephemeral port; hauler-ui
reverse-proxies to it by Host header. So **N published hauls = N hauler
processes**. This reuses hauler's own registry implementation rather than
reimplementing the OCI distribution API, at the cost of a process (and a small
warm-up + memory) per published haul. **Files do not spawn anything** — they are
served directly from the store by hauler-ui.

If the process-per-haul footprint becomes a concern, the future option is a
**single in-process read-only OCI registry** inside hauler-ui that serves every
haul from its store directory and host-routes internally — one process, no
subprocesses. That is more code (implement `/v2/` manifests + blobs against the
OCI layout) but eliminates the fan-out. The current host-routed proxy is
forward-compatible: swapping the per-haul backend for an in-process handler does
not change the client-facing contract.

## Problem

Each haul now owns an isolated store and can serve its own registry/fileserver
on its own port (validated: two hauls served concurrently on different ports
with isolated catalogs). That works, but it does not scale operationally:

- **N hauls = N ports** to allocate, expose, firewall, and document.
- A container deployment publishes a fixed set of ports; you cannot expose an
  unbounded number of per-haul registries.
- Air-gap consumers want **one stable endpoint**, not a port lookup table.

Goal: expose many hauls through a single front door, served by hauler-ui itself,
without sacrificing per-haul isolation.

## The hard constraint (read this first)

**Container registries must live at the host root `/v2/`.** Docker and
containerd derive the repository name from the request path, so a per-haul
**path prefix does not work**:

```
# DOES NOT WORK — client thinks the repo is "bundle/pause"
docker pull myhost/bundle/pause:3.9   ->  GET myhost/v2/bundle/pause/manifests/3.9
```

Therefore registries must be multiplexed by **Host header (virtual hosts)**, not
by path. Plain HTTP files have no such constraint and can be path-routed freely.

This split drives the whole design:

| Content | Routing model | Endpoint shape |
|---|---|---|
| Registries (images/charts as OCI) | **Host-based** | `<slug>.<domain>/v2/...` |
| Files (and chart `.tgz` downloads) | **Path-based** | `<ui-host>/h/<slug>/...` |

## Architecture

```
                          ┌────────────────────── hauler-ui process ──────────────────────┐
  docker pull             │                                                                │
  bundle.reg.example.com  │   Registry listener (:5000/:443)                               │
  ───────────────────────▶│   host-based reverse proxy ──┐                                 │
                          │     Host: bundle.reg... ──────┼──▶ 127.0.0.1:49xxx (haul A reg)│
                          │     Host: edge.reg...   ──────┼──▶ 127.0.0.1:49yyy (haul B reg)│
                          │                               │     (hauler store serve         │
                          │                               │      registry --store …A/store) │
                          │                                                                │
  curl …/h/bundle/cfg.yaml│   UI/API listener (:8080)                                      │
  ───────────────────────▶│   GET /h/<slug>/<name> ──────▶ stream blob from …<slug>/store  │
                          │                                                                │
                          │   Publish manager: allocate ports, start/stop internal          │
                          │   registries, host→haul route table, restart on boot           │
                          └────────────────────────────────────────────────────────────────┘
```

Two listeners:
- **`:8080`** — existing UI/API, plus the new path-routed file endpoint.
- **`:5000` (configurable, 443 in prod)** — a dedicated **registry** listener
  that only does host-based reverse-proxying to internal per-haul registries.
  This is the single port operators expose for *all* haul registries.

## Registry: host-based reverse proxy

### Internal registry manager
When a haul is **published**, hauler-ui starts an internal registry for it:

```
hauler store serve registry \
  --readonly \
  --store     /data/hauls/<slug>/store \
  --directory /data/hauls/<slug>/registry \
  --port      <auto-allocated, bound to 127.0.0.1>
```

- Port is allocated from an ephemeral range and **bound to 127.0.0.1** (never
  exposed directly).
- Lifecycle tracked in `serve_processes` (already has `haul_id`); add a
  `role` column (`'published'` vs ad-hoc `'manual'`) so the manager owns them.
- Crash detection + restart (the manager already monitors process exit).
- On server boot, re-start internal registries for all published hauls.

### Host → haul resolution
The registry listener resolves the incoming `Host` header to a haul:

1. **Subdomain pattern (default):** `<slug>.<HAULER_UI_REGISTRY_DOMAIN>` →
   `<slug>`. e.g. `bundle.registry.example.com` → haul `bundle`.
2. **Explicit hostname override** stored on the publish record (for custom
   per-haul hostnames).

Only known, currently-published slugs resolve; everything else gets `404`
(prevents Host spoofing to reach unintended hauls).

### The proxy
`net/http/httputil.ReverseProxy` with:
- a Director that rewrites the target to `127.0.0.1:<haul-internal-port>`,
  preserving the original path (`/v2/...`) and query;
- **streaming** behavior for large blob pulls: `FlushInterval` set, no response
  buffering, `Range` and `Content-Range` passed through unchanged;
- pass-through of registry auth headers (`Authorization`, `WWW-Authenticate`,
  `Docker-Content-Digest`, etc.).

### TLS
Container clients want HTTPS. Terminate TLS at the registry listener:
- **Wildcard cert** (`*.registry.example.com`) is the natural fit for the
  subdomain model. Provide via `HAULER_UI_REGISTRY_TLS_CERT/KEY`.
- Fallback: extend the existing `CertManager` to mint a self-signed **wildcard**
  cert for non-prod/testing.
- Document the client trust step (add the CA) and a `/etc/hosts` fallback for
  environments without DNS.

### Client UX (generated by the UI)
```
# Docker / nerdctl
docker pull bundle.registry.example.com/pause:3.9

# containerd hosts.toml (mirror)
# /etc/containerd/certs.d/bundle.registry.example.com/hosts.toml
server = "https://bundle.registry.example.com"
[host."https://bundle.registry.example.com"]
  capabilities = ["pull", "resolve"]
```

## Files: path-routed, served directly by hauler-ui

No subprocess. hauler-ui serves files straight from the haul's OCI store:

- `GET /h/<slug>/` → listing (JSON, and an HTML index) of `file` and `chart`
  artifacts in the haul.
- `GET /h/<slug>/<name>` → stream the artifact's content blob with the right
  `Content-Type` and `Content-Disposition: attachment; filename="<name>"`.

### Blob resolution (mirrors `hauler store extract`)
1. Read `<store>/index.json`; find the manifest whose
   `org.opencontainers.image.ref.name` (or `io.containerd.image.name`) == `<name>`
   and whose type is `file`/`chart`.
2. Read that OCI image manifest from `blobs/sha256/<digest>`.
3. Stream its content layer (`layers[0].digest`) from `blobs/sha256/<layer>`,
   supporting `Range` requests.

This keeps files on the **single UI port**, path-routed, with zero extra
processes or ports.

> Alternative considered: run hauler's own fileserver per published haul on
> 127.0.0.1 and reverse-proxy `/h/<slug>/...` with prefix stripping. Rejected as
> default because it adds a process + port per haul for content we can serve
> directly. Kept as a fallback if direct blob resolution proves brittle across
> hauler versions.

## Data model & API

New "publish" concept (a published haul = exposed through the front door):

- Reuse `serve_processes` for the internal registry, add `role TEXT` and
  optional `hostname TEXT`.
- API:
  - `POST   /api/hauls/{id}/publish`   `{hostname?}` → allocate port, start
    internal registry, register route.
  - `DELETE /api/hauls/{id}/publish`   → stop + deregister.
  - `GET    /api/publish`              → routes table (haul, registry hostname,
    file URL, status, internal port).

## UI

- A **Publish** toggle on each haul (Hauls list + Haul detail → Serve tab).
- A **Routes** view: table of haul → registry hostname + file URL + status,
  each with copy-paste client config (docker pull, containerd hosts.toml, curl).
- Surface the configured base domain and TLS status so misconfig is obvious.

## Configuration

| Env | Default | Purpose |
|---|---|---|
| `HAULER_UI_REGISTRY_PORT` | `5000` | Single host-routed registry listener port |
| `HAULER_UI_REGISTRY_DOMAIN` | (none) | Base domain for `<slug>.<domain>` routing |
| `HAULER_UI_REGISTRY_TLS_CERT` / `_KEY` | (none) | Wildcard cert for the registry listener |
| internal registry ports | `49152–65535` | Ephemeral, bound to `127.0.0.1` |

## Security

- Published registries are **read-only**.
- Internal registries bind to `127.0.0.1` only; the proxy is the sole ingress.
- Host resolution is allow-list only (known published slugs).
- Future: optional basic-auth / token passthrough per published registry; reuse
  the UI password or per-haul credentials.

## Constraints & gotchas

- **No path-based registry routing** (the `/v2` root rule). Host-based only.
- Subdomain model needs **DNS + wildcard TLS**; document the `/etc/hosts` +
  self-signed wildcard path for air-gapped/test setups.
- Large blob streaming: disable proxy buffering, set `FlushInterval`, honor
  `Range`.
- Internal registry warm-up: `hauler store serve registry` copies artifacts into
  its backend dir on start (observed ~1–2s for small images); publish should
  report `starting → ready`.
- File/chart blob layout may shift across hauler versions; pin the index/blob
  parsing to documented annotations and add a fallback to the fileserver-proxy
  mode.

## Rollout phases

1. **Publish manager + host-based registry proxy listener + publish API** (the
   core: one port, many haul registries).
2. **Direct path-routed file serving** (`/h/<slug>/...`).
3. **UI**: Publish toggle, Routes page, generated client snippets.
4. **TLS/wildcard** cert handling (provided cert → self-signed wildcard →
   optional ACME/autocert).

## Validation plan (real hauler)

- Publish two hauls; `docker pull` from each via distinct hostnames against the
  single registry port; confirm isolated catalogs.
- Pull a large multi-layer image through the proxy; confirm streaming + `Range`.
- `curl` a file artifact via `/h/<slug>/<name>`; confirm content + filename.
- Restart hauler-ui; confirm published hauls auto-republish.
- Unpublish; confirm route + internal process removed.
