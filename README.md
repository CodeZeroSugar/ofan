![alt text goes here](https://github.com/CodeZeroSugar/ofan/actions/workflows/ci.yml/badge.svg)

# Ofan

Ofan is a self-hosted manager for Valheim dedicated servers. You describe the servers you want in a web dashboard, and Ofan creates, runs, repairs, and removes them on a Kubernetes cluster.

**Status:** In active development. The install guide is written but has not yet been tested on a fresh machine. Expected MVP before the end of 2026.

## Why Ofan

Running your own Valheim server means handling ports, world files, mods, and restarts by hand. Hosted providers solve that but charge monthly. Ofan aims to give you the same convenience on hardware you already own, or on a leased cloud instance, with a simple web UI for creating and managing servers and the users who play on them.

## Features

- **Server lifecycle:** create, start, stop, and delete servers. Deleting keeps the world save unless you explicitly purge it.
- **Self-repair:** if a server breaks, Ofan retries fixes on every pass and performs a clean rebuild after repeated failures. Saved worlds are kept during rebuilds.
- **Live settings:** change most server settings on a running server and they apply automatically. Ports and world names are fixed at creation, so changing them means creating a new server.
- **Accounts:** a root administrator, admins, and regular users. Users manage only their own servers unless an admin transfers ownership.
- **Web dashboard:** server cards with live status, a form to create servers with defaults or full customization, a detail page for editing settings, an admin page for users and storage, and a page to change your password.
- **Status reporting:** shows when a server is starting, running, or failing, along with restart counts and how long it has been running.

## Getting started

Ofan runs on a computer in your home. You install one small system tool (k3s), run one script, and then manage your servers in a web browser. Plan on about 30 minutes, plus download time.

### What you need

- **A computer running Ubuntu Server 26.04 LTS.** [Download it here](https://ubuntu.com/download/server). Ubuntu Desktop also works, but this guide is written for Server.
- **Memory and processor:** at least 4 GB of RAM and 2 processor cores for the dashboard and one Valheim server. The Valheim server's own recommendation is 8 GB and 4 cores, and more players need more. See [the Valheim server documentation](https://github.com/community-valheim-tools/valheim-server-docker).
- **Disk space:** about 20 GB free. That covers the system, the Valheim download (about 1 GB), and saved worlds. This is an estimate and will be refined after testing.
- **A wired network connection** is recommended. Wi-Fi works, but computers on Wi-Fi more often get a new address.
- **A second computer** on the same network with a web browser, to open the dashboard.

Ofan is designed for your home network. Do not expose it to the internet.

### Step 1: Install k3s

k3s is a small Kubernetes system. Ofan uses it to run the dashboard and your game servers. Open a terminal on the server (or connect to it over SSH) and run:

```bash
curl -sfL https://get.k3s.io | sudo sh -
```

`sudo` means "run as administrator," so it will ask for your password. The install takes a minute or two. To confirm it worked:

```bash
sudo k3s kubectl get nodes
```

You should see one line ending in `Ready`. If it says `NotReady`, wait a minute and run the command again.

The [k3s quick start](https://docs.k3s.io/quick-start) has more detail on what the install does.

### Step 2: Open the firewall

Ubuntu's firewall, [UFW](https://help.ubuntu.com/community/UFW), is off by default. Check its status:

```bash
sudo ufw status
```

- If it says `Status: inactive`, skip the rest of this step.
- If it says `Status: active`, allow SSH first so you do not lock yourself out, then open the dashboard port:

```bash
sudo ufw allow OpenSSH
sudo ufw allow 30080/tcp
```

Each Valheim server gets its own port, between 30000 and 32767. The port is shown on the server's card in the dashboard, and you need to allow it for friends to connect. For example, if the card shows `192.168.1.50:30643`:

```bash
sudo ufw allow 30643/udp
```

### Step 3: Get Ofan

Install `git`, then download Ofan and move into its folder:

```bash
sudo apt update && sudo apt install -y git
git clone https://github.com/CodeZeroSugar/ofan.git
cd ofan
```

### Step 4: Run the installer

```bash
sudo bash setup.sh
```

The installer prints a banner, then sets up Ofan. It can take several minutes while the first images download. When it finishes, it prints the address of your dashboard.

On the first install, it also prints your **root password**. Copy it somewhere safe now. If you run the installer again later, it will say the existing password is unchanged, and it will not print it again.

If the installer stops with an error, it prints the most recent log lines from Ofan. See [Troubleshooting](#troubleshooting) below.

### Step 5: Log in and change your password

Open the address from Step 4 in a browser on your other computer. It looks like `http://192.168.1.50:30080`.

- **Username:** `ofan-root`
- **Password:** the root password from Step 4

You will be asked to choose a new password before anything else unlocks. Write the new password down. The installer only knows the original password, so after you change it, the one it printed no longer works.

### Step 6: Create your first server

1. Click **Create**.
2. Choose a name and a server password. Leave the other settings at their defaults for now.
3. Click **Create**, then start the server.

The first start downloads Valheim, about 1 GB, so it can take several minutes. The card shows **Running** when it's ready, and it shows the address to connect to.

To join from Valheim, choose **Join Game**, then **Join IP**, and enter the address from the card. Use the server password you set.

### Step 7: Keep it working

- **Deleting a server keeps its world** unless you choose to purge it. You can create a server with the same name later to get the world back.
- **After a network change**, the server's address may change, and the dashboard may stop responding. Run this to restart the system:

  ```bash
  sudo systemctl restart k3s
  ```

  Then wait a minute and reload the dashboard. k3s also starts again automatically after a reboot.

- **Prevent address changes** by giving the server a fixed address in your router. Look in your router's manual for "DHCP reservation," "static lease," or "address reservation." Set it for this computer's MAC address.

### Troubleshooting

| What you see                                   | What to do                                                                                                                                                                               |
| ---------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Permission denied` when running `setup.sh`    | Run it as `sudo bash setup.sh` instead.                                                                                                                                                  |
| `The connection to the server ... was refused` | k3s is not running. Run `sudo systemctl restart k3s`, wait a minute, and try again.                                                                                                      |
| The installer stops with an error              | It prints recent Ofan logs. To see them again, run `sudo k3s kubectl -n ofan logs deploy/ofan --tail=30`.                                                                                |
| The dashboard does not load                    | Check the address with `ip address`. If `ufw` is active, make sure port 30080 is allowed (Step 2).                                                                                       |
| The root password is not accepted              | If you changed it in the dashboard, use the new one. The installer's password only works until you change it.                                                                            |
| A friend cannot connect to your server         | Make sure the server's UDP port is allowed in UFW (Step 2). If the friend is outside your home network, your router must forward that port to this computer. Check your router's manual. |
| The server stays on "starting" for a long time | The first start downloads about 1 GB. Wait 10 minutes before assuming there is a problem.                                                                                                |

If you forget the root password, Ofan cannot reset it without deleting its stored data. Keep your password written down.

### Safety notes

- Ofan is for your home network only. Do not forward port 30080 from your router.
- The dashboard uses plain HTTP, which means traffic is not encrypted. Anyone on your network who can watch traffic could read a password as it is typed. Use a password you do not use anywhere else.
- Only run installers you trust. `setup.sh` runs as administrator.

### Helpful links

**Setup tools**

- [Ubuntu Server download](https://ubuntu.com/download/server)
- [k3s quick start](https://docs.k3s.io/quick-start)
- [UFW firewall guide](https://help.ubuntu.com/community/UFW)

**Valheim**

- [Valheim on Steam](https://store.steampowered.com/app/892970/Valheim/)
- [Valheim server container documentation](https://github.com/community-valheim-tools/valheim-server-docker), covering system requirements, ports, backups, and finding your server

**Ofan**

- [Source code](https://github.com/CodeZeroSugar/ofan)
- [Report a problem](https://github.com/CodeZeroSugar/ofan/issues)

## Developer setup

This is for working on Ofan itself. You need Go and a development cluster.

### Prerequisites

- Go 1.26 or newer
- A Kubernetes cluster, such as k3s, with a working kubeconfig
- A `.env` file in the project root. The full list of settings is in `cmd/server/config.go`. The essentials are:

```bash
# A long random string used to sign sessions
OFAN_SESSION_SECRET=
# Initial password for the root account
OFAN_ROOT_PASS=
# Path to your kubeconfig file
KUBECONFIG_PATH=
# Kubernetes namespace game servers are created in
DEFAULT_NAMESPACE=
```

### Run it

```bash
go build ./...
./ofan
```

Open `http://localhost:8080/login` and sign in as `ofan-root` with the password from your `.env`. You will be asked to choose a new password before anything else unlocks.

The server runs over plain HTTP. If you expose it beyond your own machine, put it behind a reverse proxy that handles TLS.

## Stack

- Go 1.26
- Kubernetes client-go (informers and typed clients)
- SQLite via `modernc.org/sqlite` (no CGO)
- Argon2id password hashing, JWT sessions
- HTMX, Go `html/template`, Tailwind CSS

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

## Roadmap

- **Deployment:** container image, Kubernetes manifests, and installer script (done). Install guide written; fresh-machine test pending.
- **Graceful drain:** optionally scale servers down cleanly when Ofan itself shuts down. Pod shutdown timing is already set to give worlds time to save.
- **Later:** live player counts and metrics, Discord or webhook notifications, readiness reporting

## License and contact

Solo project. Feedback, bug reports, and questions are welcome through GitHub issues.

Ofan is licensed under the [Apache License 2.0](LICENSE). Copyright 2026 CodeZeroSugar.

Ofan is not affiliated with, endorsed by, or sponsored by Iron Gate AB or Coffee Stain Publishing. Valheim is developed by Iron Gate AB and published by Coffee Stain Publishing. Ofan does not include any Valheim game files. Each server downloads the game from Steam when it starts, using the community-maintained [valheim-server-docker](https://github.com/community-valheim-tools/valheim-server-docker) image.
