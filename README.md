# Ofan

**Self-hosted orchestration for Valheim dedicated servers — declare the servers you want, Ofan converges your cluster onto them.** Under active development.

## What's Ofan?

Hosting your own Valheim co-op shouldn't be a hassle and should be just as economical as paying for a third-party hosting service. Ofan bundles all the infrastructure needed for running your own Valheim servers and provides a lightweight web UI for convenient server and user management.

Instead of managing machines, ports, worlds, and mods by hand, Ofan provides a compact declarative platform -- a Go service backed by SQLite and Kubernetes (k3s) that provisions, repairs, and tears down game servers based on what the user defines in the web UI.

## What works today

- **Declarative controller** — SQLite rows are desired state, the cluster is actual state; a reconcile loop (30s ticker + handler pokes) converges the two. API handlers never touch Kubernetes directly — they update the row and poke.
- **Full server lifecycle** — create, start, stop, and crash-safe delete via tombstones (`desired_state='deleting'`), with persistent storage preserved by default and explicit purge paths.
- **Self-healing escalation ladder** — every pass applies idempotent soft fixes; every 5th consecutive API failure triggers a graceful hard reset (teardown → bounded wait → fresh rebuild, PVC preserved). No failure gates, no manual retries; the loop never gives up.
- **Drift handling** — cluster resources with no DB row are torn down after consecutive passes (storage preserved); the controller never adopts or invents rows. Deliberate `kubectl delete`s get re-provisioned because the DB is the source of truth.
- **Auth that fits both machines and browsers** — JWT via `Bearer` header or `HttpOnly` cookie, role-based access (root/admin/user) plus row-level ownership, self-targeting guards, and mandatory root password bootstrap on fresh databases.
- **Live config updates** — editable settings diff against a live config-hash annotation and apply in the same pass; server ports and world names are frozen after creation (change requires recreate).
- **Web UI (first slice live)** — cookie login with first-run password-change flow, auto-polling server list with live status/health, logout. Server-rendered HTMX fragments; one endpoint serves JSON to machines and HTML to browsers via content negotiation.
- **Operational visibility** — pod informer surfaces waiting reasons, restart counts, and node IPs; `CrashLoopBackOff` escalates to `failed` health.

## Design decisions (the interesting parts)

- **Tombstones over direct deletes** — deletes are state transitions the controller consumes, so crashes mid-teardown resume safely instead of leaking resources.
- **No orphaned resource adoption** — a cluster resource without a DB row is drift to be removed, not state to be learned. Reattachment is a deliberate act (recreate the same-named server onto the preserved PVC).
- **Ownership lives only in the database** — no ownership labels in Kubernetes; the row is the single authority.
- **Boring frontend** — HTMX + Go templates, no JS framework. The server renders both the page and its live fragments from one template, so first paint and polled updates can never drift apart.

## Architecture

```
Browser / API clients (JSON ↔ HTML via Accept header)
        │ HTTPS
┌───────▼────────┐    poke     ┌──────────────────┐
│  HTTP handlers │ ─────────▶  │    Controller    │
│  (rows + Poke) │             │ (rows → cluster) │
└───────┬────────┘             └───────┬──────────┘
        │                              │
        ▼                              ▼
   SQLite (desired)            k3s via client-go (actual)
   servers · users             Deployments · Services · PVCs · ConfigMaps
```

## Stack

- Go 1.26
- testify throughout (`go test ./... -race -count=1` green)
- client-go (informers + registry)
- modernc SQLite
- JWT + Argon2id
- HTMX + Go templates (Tailwind pipeline landing this slice)

## Getting started (current state)

Prerequisites: Go 1.26+, a k3s cluster with kubeconfig. Build with `go build ./...`, run the server, and exercise the full lifecycle against a live cluster with `scripts/smoke.sh` (auth → lifecycle → delete/purge). First boot creates a `root` user with mandatory password change before any other route unlocks. Schema changes during development: delete `data/*.db` and let migrations rebuild (dev only).

## Roadmap

- **This slice**: Tailwind styling pass, then create-form and server-detail slices (live config editing, delete with storage choice).
- **Shutdown safety + drain**: graceful pod termination matched to the game’s save timeout, optional scale-to-zero drain on server shutdown.
- **Deploy anywhere**: Dockerfile, Kubernetes manifests, and a setup script for local or existing clusters.
- **Later**: live player counts/metrics dashboard, out-of-game (Discord/webhook) notifications, readiness reporting.

## Status

Solo project, in active development, MVP expected before the end of 2026. Feedback and questions welcome via GitHub issues.
