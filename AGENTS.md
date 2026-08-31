# AGENTS.md

Guide for agentic workflows on the ofan project. Read fully before starting work.

## Project overview

Ofan is a self-hosted video game server that allows you to easily deploy, start,
stop, and delete instances of multiplayer/co-op video games that require a server
to host cooperative play sessions. Currently this project is focused on the
survival game Valheim.

The role of agents in this project is purely for guidance and code review. Agents
are forbidden from writing to production files in this project. Agents assisting in this project will act as a Senior Developer/Code Reviewer:
they will provide guidance, answer questions, help with project direction and ideas. The agents will avoid providing code snippets and complete files/functions.
The agents may provide function signatures, data structures, steps for
implementing the code, but WILL NOT write the code to files themselves.

The only files the agents may contribute to directly are AGENTS.md and TODO.md

## Tech stack & policies

- Go 1.26+, module `github.com/CodeZeroSugar/ofan`
- local sqlite database at ./data/ofan.db
- Kubernetes cluster using k3s and client-go for interacting with k8s resources
- Tests: testify is the preferred testing framework. Table-driven when practical.

## Architecture map

```
.
├── cmd
│   └── server                  Where application launches from
├── data                        Where local database lives
├── go.mod
├── go.sum
├── infra
├── internal
│   ├── api                     Endpoint handlers that deal with k8s resources creation and database interactions live here.
│   ├── auth                    Password hashing with argon2id
│   ├── controller              Controls the reconciliation loop that assess desired server states from db and applies that to k8s
│   ├── db                      database interaction methods. uses modernc sqlite driver
│   └── k8s                     Files that define the game server configurations and functions that actually create the k8s resources, using the config structs.
├── k3s                         Initial testing k8s yamls
├── README.md
├── scripts
├── TODO.md
└── web                         Where the frontend will be
    ├── embed.go
    └── static
        └── index.html
```

## Locked domain rules

## Workflow

### Task lifecycle

### Testing philosophy

- Recommend specific test cases when there are gaps in coverage.
- Provide test cases in a table format
  - Name, Setup, Action, Assert

## Commands

## Known gotchas

## Roadmap — future (high level)

- **Shutdown safety + drain** (two changes):
  - `terminationGracePeriodSeconds: 120` on the pod spec in `BuildDeployment()` — constant, matches the lloesche image's `--stop-timeout 120`. The container already saves the world on graceful shutdown (SIGTERM/SIGINT → `kill -INT` → game saves). This single constant prevents k8s from SIGKILLing a saving pod. No code beyond this constant handles the save.
  - `DrainOnShutdown` toggle (`OFAN_DRAIN_ON_SHUTDOWN`, default `false`): on `ctx.Done()`, before `srv.Shutdown`, a controller method scales all `desired_state='running'` deployments to 0 replicas. Does NOT mutate `desired_state` — startup reconcile restores intent. Timeout: 150s (120 grace + 30 headroom). Only affects `running` — `stopped` is already 0, `deleting` continues normal teardown. Tradeoff: every Go restart stops all servers (players kicked). Dev-friendly default: off.
  - **In-game notifications**: infeasible in vanilla Valheim (no RCON, no server console, no broadcast). Discord/webhook out-of-game notifications deferred to a future pass.
- **Config updates on running servers**: `PUT /api/v1/servers/{name}/config` → validate → update row's `config_json` → poke → reconciler diffs stored vs live and applies (rollout via annotation bump). **Ports frozen** (auto-assigned post-create; change = delete+recreate). Fold in `SERVER_PASS` ≥5 char rule.
- **Web GUI**: login → server list (poll; shows status/health/desired) → create form (config editor) → detail with live config editing + delete. SPA embedded via existing `web/embed.go`.
- **Deploy anywhere**: Dockerfile, k8s manifests (Deployment + Service + ClusterRole + PVC for sqlite + secret for session secret/admin), `scripts/setup.sh` (k3s local or existing cluster).

## Backlog

- Pod-lister refinement (crashloops, node IPs for game clients) — re-add pod informer + `PodLister` when this lands.
- Live metrics/player counts for the web dashboard.
- `handlerReadiness` — actually report informer sync state instead of always 200.
- `ServerOpts.StorageSize` dead field — either wire into the builder or remove.
- Service informer has no `DeleteFunc` — a manually-deleted service leaves stale `NodePort`/`QueryPort` in the registry until its deployment is deleted. Harmless in the normal flow (deployment delete clears the entry), but a footgun for out-of-band cleanup.
- Running-row zombie: a `running` DB row whose registry entry was removed (e.g. manual `kubectl delete` of the deployment) is skipped by `convergeRow` (`!registry.Get` → `nil`) and never re-provisioned. Accepted for v1 — clean DB makes it moot — but worth revisiting.
- Periodic reconcile loop option — startup pass + poke-driven passes first; add a slow background ticker later if drift-from-external-changes needs faster cleanup than 30s ticks... (already 30s; revisit only if needed).
