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

**Canonical memory**: AGENTS.md is the repo-committed canonical record of
decisions, rules, and roadmap. TODO.md is gitignored local scratch — task state
only, transient, not part of the repo.

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

Hard architectural contracts. A reviewer must flag any change that violates
these. These are decisions, not suggestions.

- **Declarative controller model** — DB = desired state, cluster = actual state
  (registry), controller converges. Handlers make **zero k8s calls**; they
  mutate the `servers` row and `Poke()`. The lone documented exception is
  `HandlerDeletePVC` (orphaned storage has no row to converge).
- **Ownership is DB-only** — `servers.owner` FK → `users.username`. No ownership
  labels in k8s. `Owner` lives on the row / `ServerView`, never on
  `ServerState`.
- **Delete = tombstone** — `desired_state='deleting'` + `purge_storage` flag;
  the controller consumes it after teardown. Crash-safe; PVC preserved by
  default.
- **Drift → auto-teardown** — registry entries with no DB row after 3
  consecutive passes get `InsertOrphanTombstone` (PVC preserved). `MarkDeleting`
  is UPDATE-only and must never be used for rowless orphans.
- **No orphan adoption** — the controller never writes rows for cluster state;
  a rowless registry entry is never re-attached to the DB. Orphan resolution is
  drift teardown (PVC preserved) + a human deliberately creating a matching-name
  server to reattach the preserved PVC.
- **Unhealthy-server escalation ladder** — no failure gate, no manual retry.
  `consecutive_failures` is the "something is wrong" indicator (API-error-only),
  not a terminal state; it climbs until DB ↔ k8s align (success reset) or an
  admin's start/stop/delete corrects the state. The loop never gives up: soft
  fix every pass (idempotent `CreateAll` + `ensureReplicas`); on
  `failures % 5 == 0` a hard reset rebuilds non-PVC resources — graceful
  `DeleteAll(storage=false)` → wait for deletion (150s = 120s grace + headroom)
  → fresh `CreateAll`. `deleting` executes unconditionally (bypasses the
  registry-missing skip so zombie tombstones are consumed).
- **Cadence** — 30s ticker + cap-1 poke channel from handlers.
- **PVC policy** — root deletes any server's PVC; others only their own;
  non-owner admin must transfer ownership first. Orphaned-PVC purge via
  `POST /api/v1/system/purge-storage/{name}` (root-only, `confirm:true`).
- **Self-targeting guards** — `rejectSelf` on suspend/demote/delete/start/stop.
- **`Poke` is a field, not a method** — nil-guard every handler call.
- **NodePort auto-assign** — user-configurable `node_port` removed.
  `BuildService` always emits `nodePort: 0`; `ServerState.NodePort/QueryPort`
  are informer-sourced actuals only.
- **List contract** — `map[string]ServerView` keyed by server name; empty case
  returns `{}`; rowless uptime uses `IsZero()` fallback to registry `CreatedAt`.
- **Auth model** — role enforced via middleware; ownership enforced in the
  handler. Root can never be deleted or demoted.

## Workflow

### Task lifecycle

Agents do not write code; the lifecycle is a guide/review loop:

1. **Intake** — clarify the goal. Read the relevant files and trace the flow
   end-to-end before advising. Never advise on a partial picture.
2. **Guide** — explain step by step. Function signatures, data structures, and
   test tables are fair game. Avoid code snippets and complete files/functions.
3. **Review** — verify user-written code against the Locked domain rules and the
   Review checklist. Recommend test cases for gaps (see Testing philosophy).
4. **Closeout** — only AGENTS.md and TODO.md are ever written. Keep the roadmap
   current as decisions land or change.

### Review checklist (rule → check)

A code-review aid mapping each locked rule to its observable target. Rows
reference the rule numbers in Locked domain rules.

| # | Rule | When reviewing, verify |
|---|------|------------------------|
| 1 | Handlers make zero k8s calls | No `Clientset`/`Registry` mutation in handlers except `HandlerDeletePVC` (documented exception). Handlers mutate the row + `Poke()`. |
| 2 | DB = desired, cluster = actual | New state transitions route through `desired_state` + reconcile; nothing sets k8s replicas outside `convergeRow`/`ensureReplicas`. |
| 3 | Ownership is DB-only | `Owner` never added to `ServerState`; ownership reads come from the `servers` row via `GetServer`. No ownership labels in k8s. |
| 4 | Delete = tombstone | Delete paths set `desired_state='deleting'` (+ `purge_storage`); no direct k8s delete except via `convergeRow` → `DeleteAll`. |
| 5 | Drift → auto-teardown, no adoption | Orphan teardown goes through `InsertOrphanTombstone` (PVC preserved); `MarkDeleting` (UPDATE-only) is never used for rowless orphans; no handler/controller path writes a row for a rowless registry entry. |
| 6 | Foreign resources untouched | Informer upserts still filter `LabelManagedBy`; nothing strips that guard or adds foreign deployments to the registry. |
| 7 | Unhealthy-server escalation ladder | No gate/retry logic re-introduced: `consecutive_failures` only ever resets via success reset, never a hard stop. `failures % 5 == 0` triggers a hard reset (graceful `DeleteAll(storage=false)` → bounded wait ≤150s → fresh `CreateAll`, PVC preserved). `deleting` executes unconditionally (bypasses the registry-missing skip). |
| 8 | `Poke` nil-guarded | Every handler calling `c.Poke()` first checks `c.Poke != nil`. |
| 9 | NodePort auto-assign | `BuildService` still emits `nodePort: 0`; no user-configurable port re-introduced; `NodePort`/`QueryPort` remain informer-sourced actuals. |
| 10 | List contract | List response stays `map[string]ServerView` keyed by name; empty case returns `{}`; rowless uptime uses `IsZero()` fallback. |
| 11 | Auth = middleware, ownership = handler | New endpoints gate role in middleware and ownership (`srvRec.Owner` vs `userCtx`) in the handler; `rejectSelf` still guards self-targeting ops. |

### Testing philosophy

- Recommend specific test cases when there are gaps in coverage.
- Provide test cases in a table format
  - Name, Setup, Action, Assert

## Commands

No Makefile or CI exists. Plain Go commands:

- `go test ./...` — primary verification. Agent-safe to run (tests use in-memory
  sqlite; non-destructive). Add `-race -count=1` for a full run.
- `go build ./...` — compile check.
- `go vet ./...` — static check.
- `go mod tidy` — when dependencies change.

Agents are authorized to run the verification commands above, including
package-scoped variants (e.g. `go test ./internal/controller/`,
`go vet ./internal/db`, `go build ./cmd/server`). Read-only build/test
operations — tests use in-memory sqlite, non-destructive.

Agents never run `go run`, `kubectl apply`, or any DB-mutating command. The user
runs those.

- `scripts/smoke.sh` — end-to-end smoke test (auth/reset → server lifecycle →
  delete/purge) against a live k3s. User-run only: needs the server running and
  mutates the DB + cluster. Agents never run it.

Agents never commit, amend, push, or create PRs. All VCS operations are the
user's to perform.

## Known gotchas

- `opts.Namespace = c.namespace` injection is required in the controller —
  empty namespace makes k8s reject resource creation.
- `CreateAll` has a service-existence pre-check (`Get` → create only on
  `IsNotFound`) to avoid the "provided port is already allocated" failure
  spiral.
- The escalation ladder sits *inside* the `running`/`stopped` switch (soft fix
  vs `% 5` hard reset); `deleting` executes unconditionally so teardown always
  wins over any in-flight reconcile.
- `deleting` also bypasses the registry-missing skip in `convergeRow` — the
  skip is scoped to the `running`/`stopped` branches, so a zombie tombstone
  (deleting row with no registry entry) is still consumed. `running` rows with
  no registry entry remain skipped (zombie re-provision is open backlog).
- Escalation ladder: `consecutive_failures` has no terminal state — it climbs
  (API-error-only) and resets only via a successful converge (or row removal).
  The `% 5 == 0` hard reset deletes the deployment gracefully, so the rebuild
  must wait out the full termination grace (≤150s) before `CreateAll` — a
  shorter wait strands a mid-save pod: poll timeout → registry entry drops →
  zombie skip → never re-provisioned.
- `stateActions` (controller) takes a `targetReplicas` param — never hardcode a
  replica literal inside it (`maxReplicas` is a var; a literal breaks if it
  changes).
- Tests that exercise the ladder's hard reset must shrink `recreatePollInterval`
  (controller package var, default 5s) or each case sleeps a real 5s against the
  fake clientset.
- `deploymentStatus` treats nil `Spec.Replicas` as 1 (matches k8s) so a
  nil-replicas deployment reports `provisioning`, not `stopped`.
- Rowless uptime: `IsZero()` fallback to registry `CreatedAt` when the DB row
  is missing.
- sqlite: `SetMaxOpenConns(1)`, WAL + `foreign_keys` pragmas; FK-safe delete
  order is `servers` before `users`.
- `MarkDeleting` is UPDATE-only — rowless orphans need `InsertOrphanTombstone`
  (INSERT tombstone with `purge_storage=0`, config carrying `server_name`).
- Root is bootstrapped with `must_change_password=1` (fresh DB or
  `/api/v1/system/reset`). `AuthMiddleware` 403s every route except
  `/api/v1/auth/password` and `/api/v1/auth/logout` until changed — API tests
  and the smoke script must change the password before hitting server endpoints.
- Dev DBs are throwaway on schema change — delete `data/*.db` and let migrate
  rebuild.
- Service informer has no `DeleteFunc` — a manually-deleted service leaves stale
  `NodePort`/`QueryPort` in the registry until its deployment is deleted.

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
- Running-row zombie: a `running` DB row whose registry entry was removed (e.g. manual `kubectl delete` of the deployment) is skipped by `convergeRow` (`!registry.Get` → `nil`) and never re-provisioned. Accepted for v1 — clean DB makes it moot — but worth revisiting. *(The delete-path half — deleting tombstones with no registry entry get consumed — is fixed by C4.)*
- Periodic reconcile loop option — startup pass + poke-driven passes first; add a slow background ticker later if drift-from-external-changes needs faster cleanup than 30s ticks... (already 30s; revisit only if needed).