# Hauler UI — Enterprise Readiness Plan

**Status:** Reference roadmap
**Audience:** maintainers, platform/security teams evaluating Hauler UI for production
**Baseline:** the app as of the multi-haul + publish + single-user-hardening work
(isolated per-haul stores, host-routed registry publishing, TLS, graceful
shutdown, auto-restart, boot cleanup, job concurrency cap).

This document describes what it takes to move Hauler UI from a **robust
single-operator tool** to a **multi-user, enterprise-grade** platform. It is a
gap analysis plus a phased, prioritized roadmap with acceptance criteria — not a
commitment of dates.

---

## 1. Where we are today (honest baseline)

**Solid for single-user / single-instance, trusted-network use:**
- First-class isolated hauls; each operation scoped to a haul's own store.
- Background jobs with streamed logs; concurrency-capped processor.
- Multi-haul serving: host-routed registry reverse proxy on one port, direct
  file serving, TLS (provided cert or self-signed fallback, hot-reloadable).
- Operational hygiene: graceful shutdown with child reaping, auto-restart of
  published registries, boot cleanup of interrupted jobs/processes.
- Validated end-to-end against the real `hauler` CLI.

**Explicitly not yet enterprise-grade** (drives the rest of this plan):
- AuthN/AuthZ: single shared password, SHA-256 hashing, no users/roles/SSO.
- The registry and `/h/` file endpoints are unauthenticated.
- Single process + SQLite (`maxOpenConns=1`): no horizontal scale or HA.
- One `hauler serve` subprocess per published haul: limits fan-out.
- No metrics/tracing/structured logs; minimal health signal.
- Some destructive migrations: unsafe upgrade path for existing data.
- No automated test coverage on newer packages; no enforced CI gates.

---

## 2. Gap analysis by domain

### 2.1 Identity, authentication, authorization (highest priority)
Current: one `HAULER_UI_PASSWORD`, SHA-256, cookie session in SQLite; serving
endpoints open.

Required for enterprise:
- **SSO** via OIDC (and ideally SAML) with the org IdP; short-lived sessions,
  refresh, logout/SLO.
- **Users & RBAC**: roles (viewer / operator / admin) and per-haul authorization
  (who can read, modify, publish, serve, delete a haul).
- **Local accounts** fallback with a real KDF (bcrypt/argon2id), password
  policy, and lockout — for air-gapped sites without an IdP.
- **Registry/file authN**: token or basic auth on the published registry and
  `/h/` endpoints (today they are open). Support pull-only credentials.
- **API tokens / service accounts** for automation (CI pulling/pushing hauls),
  scoped and revocable.
- **Audit log** of who did what (publish, delete, login, cert change), tamper-
  evident and exportable.

### 2.2 Secrets management
Current: registry creds flow through job args/env (log-redacted); TLS keys on
disk (0600); Docker config on the data volume.

Required:
- Pluggable secret backends (Vault / cloud KMS / k8s Secrets) for registry
  creds and TLS keys; avoid passing secrets via process args.
- Encryption at rest for the DB and credential material; key rotation.
- Scoped, audited access to stored credentials.

### 2.3 Scale, availability, multi-tenancy
Current: single container, single process, SQLite single-writer, subprocess per
published haul.

Required:
- **Datastore**: move to an embedded server-grade store or external Postgres so
  multiple replicas can share state; remove the single-writer bottleneck.
- **Horizontal scale / HA**: stateless app replicas behind a load balancer;
  shared object storage for haul stores (S3/MinIO) or RWX volumes; leader
  election for singletons (job processor, publish manager).
- **In-process registry**: replace the per-haul `hauler serve` subprocess with
  a single in-process read-only OCI registry serving every haul from its store
  (documented in `docs/multi-haul-serving.md`). Eliminates process fan-out and
  makes serving replica-friendly.
- **Job execution**: a durable work queue with leasing/visibility timeouts,
  retries, backpressure, and per-tenant quotas (CPU/disk/concurrency), so a
  busy tenant can't starve others.
- **Resource governance**: per-haul disk quotas, store GC, temp-space guards.

### 2.4 Reliability & operations
Current: graceful shutdown, auto-restart of published registries, boot cleanup.

Required:
- Liveness **and** readiness probes (readiness gated on DB + hauler availability
  + migrations applied).
- Health/auto-recovery for all long-running children; circuit-breaking on the
  registry proxy.
- Configurable timeouts, request size limits, and rate limiting on all public
  endpoints (registry, files, API).
- Backup/restore runbook for `/data` (DB + stores + certs) and DR objectives
  (RPO/RTO).

### 2.5 Data management & upgrades
Current: forward-only numbered migrations; a few are destructive
(`store_contents`, `saved_manifests` are dropped/recreated).

Required:
- **Non-destructive, reversible migrations** with a tested upgrade path from
  every released version; CI test that migrates a populated DB.
- Schema/version compatibility policy; pre-flight migration check on boot.
- Data export/import (hauls, manifests, settings) for backup and migration
  between instances.

### 2.6 Observability
Current: ad-hoc `log.Printf`, `/healthz` only.

Required:
- **Structured logging** (JSON, levels, correlation/request IDs), no secrets.
- **Metrics** (Prometheus): request rates/latency/errors, job
  queue depth/duration/failures, published-haul/registry health, store sizes,
  proxy throughput.
- **Tracing** (OpenTelemetry) across API → job → hauler subprocess.
- Dashboards + alert rules shipped as reference.

### 2.7 Security hardening & supply chain
Required:
- Container: non-root, read-only rootfs where possible, dropped capabilities,
  pinned base image, minimal packages.
- SBOM generation, image signing (cosign), provenance/SLSA attestations, vuln
  scanning gate in CI (the repo already gets Dependabot alerts).
- Dependency policy and routine patching cadence.
- Security headers, CSRF protection for the UI, strict CORS, input validation
  fuzzing on upload/parse paths.
- Pen-test / threat model before GA.

### 2.8 Compliance & governance (as applicable to target customers)
- Audit retention, log shipping (syslog/SIEM), FIPS-validated crypto option
  (relevant given the airgap/government audience), STIG/CIS hardening guide.
- Configurable data residency for stores.

### 2.9 Testing, CI/CD, quality gates
Current: partial Go tests; new `publish`/`hauls`/`manifests` packages untested;
no enforced CI.

Required:
- Unit tests for all handlers/managers; the jobrunner test must not require a
  hardcoded `/data` (inject working dir).
- **Integration tests against the real `hauler` binary** in CI (add/sync/save/
  load/serve/publish/TLS) — we already do this manually; automate it.
- Frontend tests (component + a smoke E2E of the haul lifecycle).
- CI gates: build, vet, lint, unit + integration, migration test, image
  scan/sign; required for merge.
- Release process: versioned, signed images + changelog + upgrade notes.

### 2.10 Deployment & configuration
Required:
- Official **Helm chart** (and/or Operator) with HA values, ingress, TLS,
  secrets wiring, probes, resource requests/limits, PDB, network policies.
- Externalized config with validation; documented matrix of env/flags.
- Reverse-proxy/ingress guidance for the host-routed registry (wildcard DNS +
  cert), including airgap `/etc/hosts` patterns.

---

## 3. Phased roadmap

Each phase is independently shippable and ordered by enterprise-blocking value.

### Phase 1 — Security foundation (blocker)
- OIDC SSO + local accounts with argon2id; sessions hardened.
- RBAC: viewer/operator/admin + per-haul authorization checks on every mutating
  endpoint.
- AuthN on the registry and `/h/` endpoints (pull tokens); API tokens for
  automation.
- Audit logging of security-relevant actions.
- **Acceptance:** no unauthenticated mutating or content path; SSO login works;
  roles enforced by tests; audit entries emitted and queryable.

### Phase 2 — Reliability & observability (operational readiness)
- Readiness probe; structured logging; Prometheus metrics; OTel tracing.
- Rate limiting + request/size limits on public endpoints.
- Backup/restore runbook; DR objectives.
- **Acceptance:** dashboards show live metrics; readiness flips correctly on
  dependency loss; load test sustains target RPS within latency SLO.

### Phase 3 — Data & upgrade safety
- Convert destructive migrations to non-destructive/reversible; CI migration
  test on a populated DB; pre-flight check on boot.
- Export/import for hauls/manifests/settings.
- **Acceptance:** upgrade from each prior release preserves data in CI; rollback
  documented.

### Phase 4 — Scale & HA
- Datastore option for Postgres; stateless replicas; shared store backend
  (object storage/RWX); leader election for singletons.
- In-process OCI registry replacing per-haul subprocesses.
- Durable job queue with quotas/backpressure.
- **Acceptance:** N replicas serve concurrently behind an LB; failover keeps
  published registries available; publishing 100+ hauls uses bounded resources.

### Phase 5 — Supply chain, compliance, packaging
- Non-root hardened image; SBOM + cosign signing + SLSA provenance; CI scan
  gate.
- Helm chart/Operator with HA; CIS/STIG hardening guide; optional FIPS crypto.
- Threat model + external pen test sign-off.
- **Acceptance:** signed/attested releases; chart deploys HA cleanly; security
  review passed.

---

## 4. Cross-cutting principles
- **Secure by default**: deny-by-default auth, TLS on, least privilege.
- **Backward-compatible upgrades**: never lose user data on version bumps.
- **Stateless app, stateful backends**: all durable state in DB/object storage.
- **Everything observable and tested**: no feature ships without metrics + tests.
- **Airgap-first**: every capability must work fully offline (no callouts).

---

## 5. Suggested first steps (small, high-leverage)
1. Add authN to the registry and `/h/` endpoints (close the open-content gap).
2. Stand up structured logging + a readiness probe + a metrics endpoint.
3. Make migrations non-destructive and add a CI migration test.
4. Add unit tests for `publish`/`hauls`/`manifests` and an automated
   real-hauler integration test (we already run these by hand).
5. Publish a non-root hardened image + a basic Helm chart.

These are independently mergeable and de-risk the larger phases.
