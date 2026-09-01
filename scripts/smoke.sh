#!/usr/bin/env bash
set -euo pipefail

while IFS='=' read -r key val; do
  [[ "$key" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] && export "$key=$val"
done < .env

BASE_URL=http://localhost:$PORT
NS="${DEFAULT_NAMESPACE:-ofan-dev}"
RESP=$(mktemp)
trap 'rm -f "$RESP"' EXIT

assert_code() { # $1 = actual http code, $2 = expected, $3 = label
  if [[ "$1" != "$2" ]]; then
    echo "FAIL [$3]: expected HTTP $2, got $1" >&2
    [[ -n "${BODY:-}" ]] && echo "response body: $BODY" >&2
    exit 1
  fi
}

assert_jq() { # $1 = json, rest = jq args + expression
  local json=$1; shift
  if ! echo "$json" | jq -e "$@" >/dev/null 2>&1; then
    echo "FAIL: jq '$*'" >&2; echo "$json" >&2; exit 1
  fi
}

assert_eq() { # $1 = label, $2 = expected, $3 = actual
  [[ "$2" == "$3" ]] || { echo "FAIL: $1 - expected '$2', got '$3'" >&2; exit 1; }
}

kubectl_exists() { # $1 = kind, $2 = name; 0 = exists, 1 = absent
  kubectl -n "$NS" get "$1" "$2" >/dev/null 2>&1
}

wait_kubectl_gone() { # $1 = kind, $2 = name, $3 = timeout seconds
  local deadline=$(( $(date +%s) + $3 ))
  while [[ $(date +%s) -lt $deadline ]]; do
    if ! kubectl_exists "$1" "$2"; then
      return 0
    fi
    sleep 2
  done
  echo "FAIL: '$1/$2' still exists after ${3}s" >&2
  exit 1
}

root_login() {
  local body code resp token

  body=$(jq -n --arg u "$OFAN_ROOT_USER" --arg p "$OFAN_ROOT_PASS" '{username:$u, password:$p}')
  code=$(curl -s -o "$RESP" -w '%{http_code}' \
    -X POST "$BASE_URL/api/v1/auth/login" -H 'Content-Type: application/json' -d "$body")
  assert_code "$code" 200 "login"
  resp=$(cat "$RESP")
  token=$(echo "$resp" | jq -r .token)

  if [[ "$(echo "$resp" | jq -r .must_change_password)" == "true" ]]; then
    echo ">> must_change_password set -- clearing it" >&2
    body=$(jq -n --arg p "$OFAN_ROOT_PASS" '{new_password:$p}')
    code=$(curl -s -o /dev/null -w '%{http_code}' \
      -X POST "$BASE_URL/api/v1/auth/password" \
      -H 'Content-Type: application/json' \
      -H "Authorization: Bearer $token" -d "$body")
    assert_code "$code" 200 "change password"

    body=$(jq -n --arg u "$OFAN_ROOT_USER" --arg p "$OFAN_ROOT_PASS" '{username:$u, password:$p}')
    code=$(curl -s -o "$RESP" -w '%{http_code}' \
      -X POST "$BASE_URL/api/v1/auth/login" \
      -H 'Content-Type: application/json' -d "$body")
    assert_code "$code" 200 "re-login"
    token=$(echo "$(cat "$RESP")" | jq -r .token)
  fi
  echo "$token"
}

api() { # $1 = method, $2 = path, $3 = token, $4 = body (optional); sets CODE and BODY
  local method=$1 path=$2 token=$3 args=()
  args=( -s -o "$RESP" -w '%{http_code}' -X "$method" "$BASE_URL$path" -H "Authorization: Bearer $token" )
  if [[ -n "${4:-}" ]]; then
    args+=( -H 'Content-Type: application/json' -d "$4" )
  fi
  CODE=$(curl "${args[@]}")
  BODY=$(cat "$RESP")
}

wait_for() { # $1 = timeout seconds, then jq args + expression; sets BODY each poll
  local timeout=$1 deadline=$(( $(date +%s) + $1 )); shift
  while [[ $(date +%s) -lt $deadline ]]; do
    BODY=$(curl -s "$BASE_URL/api/v1/servers" -H "Authorization: Bearer $TOKEN")
    if echo "$BODY" | jq -e "$@" >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done
  echo "FAIL: condition not met within ${timeout}s" >&2
  [[ -n "${BODY:-}" ]] && echo "last response body: $BODY" >&2
  exit 1
}

if ! kubectl get ns "$NS" >/dev/null 2>&1; then
  echo "FAIL: namespace '$NS' not found" >&2; exit 1
fi

# 1. healthz
code=$(curl -s -o /dev/null -w '%{http_code}' "$BASE_URL/healthz")
assert_code "$code" 200 "healthz"
echo ">> health check successful"

# 2. reset for clean slate
TOKEN=$(root_login)
code=$(curl -s -o /dev/null -w '%{http_code}' \
  -X POST "$BASE_URL/api/v1/system/reset" \
  -H "Authorization: Bearer $TOKEN")
assert_code "$code" 200 "reset database"

TOKEN=$(root_login)
echo ">> fresh DB, token ready"

# 3. create server
SERVER_NAME="smoke-$(date +%s)"
BODY=$(jq -n --arg n "$SERVER_NAME" --arg p "secret123" '{name:$n, password:$p}')
api POST /api/v1/servers/create "$TOKEN" "$BODY"
assert_code "$CODE" 202 "create server"
assert_jq "$BODY" '.status == "provisioning"'
echo ">> server '$SERVER_NAME' created (provisioning)"

# 4. poll until running, capture node port
wait_for 180 --arg n "$SERVER_NAME" '.[$n].status == "running"'
assert_jq "$BODY" --arg n "$SERVER_NAME" '(.[$n].node_port | type) == "number" and .[$n].node_port > 0'
PORT_BEFORE=$(echo "$BODY" | jq -r --arg n "$SERVER_NAME" '.[$n].node_port')
echo ">> server running (node_port $PORT_BEFORE)"

# 5. stop
api POST /api/v1/servers/$SERVER_NAME/stop "$TOKEN"
assert_code "$CODE" 200 "stop server"
wait_for 180 --arg n "$SERVER_NAME" '.[$n].status == "stopped"'
echo ">> server stopped"

# 6. start
api POST /api/v1/servers/$SERVER_NAME/start "$TOKEN"
assert_code "$CODE" 200 "start server"
wait_for 180 --arg n "$SERVER_NAME" '.[$n].status == "running"'
PORT_AFTER=$(echo "$BODY" | jq -r --arg n "$SERVER_NAME" '.[$n].node_port')
echo ">> server started (node_port $PORT_AFTER)"

# 7. node port preserved through stop/start
assert_eq "node_port preserved through stop/start" "$PORT_BEFORE" "$PORT_AFTER"
echo ">> node_port preserved through stop/start"

# 8. delete server (preserve PVC)
BODY='{"delete_storage": false}'
api POST /api/v1/servers/$SERVER_NAME/delete "$TOKEN" "$BODY"
assert_code "$CODE" 202 "delete server"
wait_for 60 --arg n "$SERVER_NAME" 'has($n) | not'
echo ">> server deleted (row consumed)"

# 9. verify deployment gone, PVC preserved
wait_kubectl_gone deployment "$SERVER_NAME" 60
if ! kubectl_exists pvc "$SERVER_NAME-pvc"; then
  echo "FAIL: PVC was not preserved" >&2; exit 1
fi
echo ">> deployment gone, PVC preserved"

# 10. purge orphaned PVC
BODY='{"confirm": true}'
api POST /api/v1/system/purge-storage/$SERVER_NAME "$TOKEN" "$BODY"
assert_code "$CODE" 200 "purge storage"
wait_kubectl_gone pvc "$SERVER_NAME-pvc" 60
echo ">> orphaned PVC purged"

echo ">> smoke test PASSED"
