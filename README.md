# Ofan

Ofan is a self-hosted manager for Valheim dedicated servers. You describe the servers you want in a web dashboard, and Ofan creates, runs, repairs, and removes them on a Kubernetes cluster.

**Status:** In active development. Not yet ready for general use. Expected MVP before the end of 2026.

## Why Ofan

Running your own Valheim server means handling ports, world files, mods, and restarts by hand. Hosted providers solve that but charge monthly. Ofan aims to give you the same convenience on hardware you already own, with a simple web UI for creating and managing servers and the users who play on them.

## How it works

You manage servers from a web dashboard. Ofan stores what you asked for in a small database. A background controller compares that record with what is actually running in the cluster and makes the two match, every 30 seconds or sooner after a change.

```
Browser / API client
        │ HTTP
┌───────▼────────┐  poke   ┌──────────────────┐
│  Web + API     │ ──────▶ │    Controller    │
│  (writes rows) │         │ (makes it true)  │
└───────┬────────┘         └───────┬──────────┘
        ▼                          ▼
  SQLite database          Kubernetes (k3s)
  (what you want)          (what is running)
```

## Features

- **Server lifecycle:** create, start, stop, and delete servers. Deleting keeps the world save unless you explicitly purge it.
- **Self-repair:** if a server breaks, Ofan retries fixes on every pass and performs a clean rebuild after repeated failures. Saved worlds are kept during rebuilds.
- **Live settings:** change most server settings on a running server and they apply automatically. Ports and world names are fixed at creation, so changing them means creating a new server.
- **Accounts:** a root administrator, admins, and regular users. Users manage only their own servers unless an admin transfers ownership.
- **Web dashboard:** server cards with live status, a form to create servers with defaults or full customization, a detail page for editing settings, an admin page for users and storage, and a page to change your password.
- **Status reporting:** shows when a server is starting, running, or failing, along with restart counts and how long it has been running.

## Design decisions

These choices shape how Ofan behaves. They are kept deliberately simple so the system stays predictable.

- **The database is the source of truth.** Ofan records what you asked for in SQLite. The cluster is only ever brought into line with that record, never the other way around. If someone deletes a server's resources by hand in Kubernetes, Ofan recreates them.
- **Deletes are recorded, not performed immediately.** Removing a server marks it as "to be deleted" in the database. The controller carries out the teardown and clears the record when finished. If Ofan restarts mid-delete, it picks up where it left off instead of leaking resources.
- **Unknown resources are removed, not adopted.** A Kubernetes resource with no matching database record is treated as leftover and cleaned up, with its storage kept. Ofan never invents records for resources it finds. To reuse preserved storage, you create a server with the same name on purpose.
- **Ownership lives in the database only.** Kubernetes objects carry no owner labels. The database row is the single authority for who owns what.
- **Plain web frontend.** Server-rendered Go templates, HTMX for live page fragments, and small JavaScript for form submissions. There is no JavaScript framework and no build step beyond CSS.
- **Authentication:** sessions use signed tokens, sent as an `HttpOnly` cookie for browsers or a `Bearer` header for API clients. The first boot forces a root password change before any other page unlocks.

## Architecture

- **Web and API:** HTTP handlers validate requests, update the database, and wake the controller. They never change Kubernetes directly. The one exception is deleting a leftover storage volume on request.
- **Controller:** runs a reconcile loop that compares database records with the cluster and applies changes. It uses Kubernetes informers to watch the cluster efficiently rather than polling it.
- **Storage:** SQLite (pure Go, no external database server) holds servers and users. Each game server keeps its world on a Kubernetes persistent volume, which survives restarts and redeployments.

## Stack

- Go 1.26
- Kubernetes client-go (informers and typed clients)
- SQLite via `modernc.org/sqlite` (no CGO)
- Argon2id password hashing, JWT sessions
- HTMX, Go `html/template`, Tailwind CSS

## Getting started

Installation is not yet packaged. A one-command install guide is planned as part of deployment work. For now, you need a development setup.

### Prerequisites

- Go 1.26 or newer
- A Kubernetes cluster, such as k3s, with a working kubeconfig
- A `.env` file in the project root. The full list of settings is in `cmd/server/config.go`. The essentials are:
  - `OFAN_SESSION_SECRET`: a long random string for signing sessions
  - `OFAN_ROOT_PASS`: the initial root password
  - `KUBECONFIG_PATH`: path to your kubeconfig
  - `DEFAULT_NAMESPACE`: the Kubernetes namespace game servers are created in

### Run it

```bash
go build ./...
./ofan
```

Open `http://localhost:8080/login` and sign in as `ofan-root` with the password from your `.env`. You will be asked to choose a new password before anything else unlocks.

The server runs over plain HTTP. If you expose it beyond your own machine, put it behind a reverse proxy that handles TLS.

## Roadmap

- **Deployment:** container image (done), Kubernetes manifests, and a guided install script for local or existing clusters
- **Graceful drain:** optionally scale servers down cleanly when Ofan itself shuts down. Pod shutdown timing is already set to give worlds time to save.
- **Later:** live player counts and metrics, Discord or webhook notifications, readiness reporting

## License and contact

Solo project. Feedback, bug reports, and questions are welcome through GitHub issues.
