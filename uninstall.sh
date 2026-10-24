#!/usr/bin/env bash
set -Eeuo pipefail

export KUBECONFIG="${KUBECONFIG:-/etc/rancher/k3s/k3s.yaml}"

GAME_NS="ofan-games"
MANAGER_NS="ofan"
MANAGED_LABEL="ofan.io/managed-by=ofan"
K3S_UNINSTALL="/usr/local/bin/k3s-uninstall.sh"

DRY_RUN=0
ASSUME_YES=0
PURGE=0
BACKUP_DB=0
REMOVE_K3S=0
BACKUP_DIR=""
STEP="starting"

trap 'warn "failed during step: $STEP"' ERR

log()  { echo "==> $*"; }
warn() { echo "warning: $*" >&2; }
die()  { echo "error: $*" >&2; exit 1; }

usage() {
  cat <<'EOF'
usage: sudo bash uninstall.sh [options]

Removes the Ofan manager, all game servers, and their storage.
By default, game worlds are copied to a backup folder first.

  --yes          skip the typed confirmation word
  --purge        delete game worlds without a backup
  --backup-db    also copy the database (accounts and server records)
  --remove-k3s   also remove k3s from this machine (deletes all k3s data here)
  --dry-run      print the commands without running them
  -h, --help     show this message

Without --yes, you will be asked about the database and k3s.
Type 'uninstall' at the prompt to continue.
EOF
}

run() {
  if (( DRY_RUN )); then
    echo "[dry-run] $*"
  else
    "$@"
  fi
}

ask_yes_no() {
  local answer
  (( ASSUME_YES )) && return 1
  read -r -p "$1 [y/N] " answer || return 1
  [[ "$answer" =~ ^[Yy]$ ]]
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --yes)        ASSUME_YES=1 ;;
      --purge)      PURGE=1 ;;
      --backup-db)  BACKUP_DB=1 ;;
      --remove-k3s) REMOVE_K3S=1 ;;
      --dry-run)    DRY_RUN=1 ;;
      -h|--help)    usage; exit 0 ;;
      *)            die "unknown option: $1 (try --help)" ;;
    esac
    shift
  done
}

preflight() {
  STEP="preflight"
  (( EUID == 0 )) || die "run this with sudo: sudo bash uninstall.sh"
  command -v kubectl >/dev/null || die "kubectl not found"
  kubectl get nodes >/dev/null 2>&1 || die "cannot reach the cluster (checked $KUBECONFIG)"
}

confirm() {
  local answer
  (( ASSUME_YES )) && return 0
  read -r -p "Type 'uninstall' to continue: " answer || die "no input, nothing was changed"
  [[ "$answer" == "uninstall" ]] || die "aborted, nothing was changed"
}

default_backup_dir() {
  local user="${SUDO_USER:-root}" home
  home="$(getent passwd "$user" | cut -d: -f6)"
  echo "${OFAN_BACKUP_DIR:-$home/ofan-backup-$(date +%F-%H%M%S)}"
}

choose_options() {
  BACKUP_DIR="$(default_backup_dir)"
  if (( ! BACKUP_DB )) && ask_yes_no "Also back up the Ofan database (accounts and server records)?"; then
    BACKUP_DB=1
  fi
  if (( ! REMOVE_K3S )) && ask_yes_no "Also remove k3s from this machine? This deletes all k3s data here."; then
    REMOVE_K3S=1
  fi
}

print_plan() {
  echo
  log "this will remove the Ofan manager, every game server, and their storage"
  if (( PURGE )); then
    warn "--purge: game worlds will be deleted with no backup"
  else
    log "game worlds will be copied to $BACKUP_DIR before anything is deleted"
  fi
  if (( BACKUP_DB )); then
    log "the database will be copied to $BACKUP_DIR/database"
  fi
  if (( REMOVE_K3S )); then
    warn "k3s will be removed from this machine at the end"
  fi
  echo
}

namespace_exists() {
  kubectl get namespace "$1" >/dev/null 2>&1
}

wait_until_gone() {
  local ns="$1" selector="$2" label="$3" tries="$4" i
  for (( i = 0; i < tries; i++ )); do
    [[ -z "$(kubectl -n "$ns" get pods -l "$selector" -o name)" ]] && return 0
    sleep 2
  done
  die "$label still running after $((tries * 2)) seconds"
}

stop_manager() {
  STEP="stopping the manager"
  log "stopping the manager"
  if ! kubectl -n "$MANAGER_NS" get deployment ofan >/dev/null 2>&1; then
    log "manager deployment not found, skipping"
    return 0
  fi
  run kubectl -n "$MANAGER_NS" scale deployment/ofan --replicas=0
  (( DRY_RUN )) && return 0
  wait_until_gone "$MANAGER_NS" app=ofan "manager pod" 60
}

stop_game_servers() {
  STEP="stopping game servers"
  namespace_exists "$GAME_NS" || return 0
  log "stopping game servers so their worlds are not written during the copy"
  local deployments d
  deployments="$(kubectl -n "$GAME_NS" get deployment -l "$MANAGED_LABEL" -o name)"
  for d in $deployments; do
    run kubectl -n "$GAME_NS" scale "$d" --replicas=0
  done
  (( DRY_RUN )) && return 0
  wait_until_gone "$GAME_NS" "$MANAGED_LABEL" "game server pods" 90
}

host_path_for_pvc() {
  local ns="$1" pvc="$2" vol
  vol="$(kubectl -n "$ns" get pvc "$pvc" -o jsonpath='{.spec.volumeName}')"
  [[ -n "$vol" ]] || return 0
  kubectl get pv "$vol" -o jsonpath='{.spec.local.path}'
}

copy_volume() {
  local label="$1" src="$2" dest="$3" want got
  [[ -n "$src" ]] || die "no bound volume found for $label"
  if (( DRY_RUN )); then
    run mkdir -p "$dest"
    run cp -a "$src/." "$dest/"
    return 0
  fi
  [[ -d "$src" ]] || die "volume data missing for $label: $src"
  mkdir -p "$dest"
  cp -a "$src/." "$dest/"
  want="$(find "$src" | wc -l)"
  got="$(find "$dest" | wc -l)"
  [[ "$want" == "$got" ]] || die "backup of $label incomplete ($got of $want entries)"
  log "backed up $label to $dest"
}

backup_game_worlds() {
  STEP="backing up game worlds"
  namespace_exists "$GAME_NS" || return 0
  log "backing up game worlds"
  local pvcs pvc src
  pvcs="$(kubectl -n "$GAME_NS" get pvc -l "$MANAGED_LABEL" -o name)"
  for pvc in $pvcs; do
    pvc="${pvc#*/}"
    src="$(host_path_for_pvc "$GAME_NS" "$pvc")"
    copy_volume "$pvc" "$src" "$BACKUP_DIR/worlds/$pvc"
  done
}

backup_database() {
  STEP="backing up the database"
  log "backing up the database"
  local src dest="$BACKUP_DIR/database"
  src="$(host_path_for_pvc "$MANAGER_NS" ofan-data)"
  [[ -n "$src" ]] || die "no database volume found in namespace $MANAGER_NS"
  if (( DRY_RUN )); then
    run mkdir -p "$dest"
    run cp -a "$src/ofan.db"* "$dest/"
    return 0
  fi
  [[ -f "$src/ofan.db" ]] || die "database file not found at $src/ofan.db"
  mkdir -p "$dest"
  cp -a "$src"/ofan.db* "$dest/"
  log "database copied to $dest"
}

delete_game_objects() {
  STEP="deleting game server objects"
  namespace_exists "$GAME_NS" || return 0
  log "deleting game server objects"
  local kind
  for kind in deployment service configmap secret; do
    run kubectl -n "$GAME_NS" delete "$kind" -l "$MANAGED_LABEL" --ignore-not-found
  done
}

delete_game_volumes() {
  STEP="deleting game volumes"
  namespace_exists "$GAME_NS" || return 0
  log "deleting game volumes"
  run kubectl -n "$GAME_NS" delete pvc -l "$MANAGED_LABEL" --ignore-not-found
}

delete_manager_namespace() {
  STEP="deleting the manager namespace"
  log "deleting the manager namespace"
  run kubectl delete namespace "$MANAGER_NS" --ignore-not-found
}

delete_cluster_objects() {
  STEP="deleting cluster objects"
  log "deleting cluster-wide objects"
  run kubectl delete clusterrolebinding ofan-namespace-manager-binding --ignore-not-found
  run kubectl delete clusterrole ofan-namespace-manager --ignore-not-found
  run kubectl delete namespace "$GAME_NS" --ignore-not-found
}

hand_over_backups() {
  STEP="setting backup ownership"
  [[ -d "$BACKUP_DIR" ]] || return 0
  run chown -R "${SUDO_USER:-root}" "$BACKUP_DIR"
}

remove_k3s() {
  STEP="removing k3s"
  if (( ! REMOVE_K3S )); then
    log "k3s left in place"
    return 0
  fi
  [[ -x "$K3S_UNINSTALL" ]] || die "k3s uninstall script not found at $K3S_UNINSTALL"
  log "removing k3s"
  run "$K3S_UNINSTALL"
}

report() {
  STEP="report"
  echo
  log "uninstall finished"
  if (( PURGE )); then
    log "game worlds were deleted"
  else
    log "world backups: $BACKUP_DIR/worlds"
  fi
  if (( BACKUP_DB )); then
    log "database backup: $BACKUP_DIR/database"
  fi
  if (( REMOVE_K3S )); then
    log "k3s was removed"
  else
    log "k3s was left in place"
  fi
  if (( DRY_RUN )); then
    log "dry run complete, nothing was changed"
  fi
}

main() {
  parse_args "$@"
  preflight
  confirm
  choose_options
  print_plan

  stop_manager
  stop_game_servers

  if (( ! PURGE )); then
    : "${BACKUP_DIR:?BACKUP_DIR is empty}"
    backup_game_worlds
  fi
  if (( BACKUP_DB )); then
    backup_database
  fi

  delete_game_objects
  delete_game_volumes
  delete_manager_namespace
  delete_cluster_objects

  hand_over_backups
  remove_k3s
  report
}

main "$@"
