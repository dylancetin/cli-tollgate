#!/usr/bin/env bash
# End-to-end check of tollgatectl against an engine that is already running
# as a service. It mints two throwaway grants, checks what their keys may do
# from HOST (e.g. the Tailscale IP, to prove remote clients work), revokes them,
# and checks that the engine is back where it started.
#
#   scripts/verify-tollgatectl.sh PATH/TO/tollgatectl [HOST] [CONFIG]
#
# HOST defaults to 127.0.0.1, CONFIG to ~/.config/cliproxyapi/config.yaml.
# Note: the engine rewrites config.yaml on each grant or revoke; keep a backup
# if its exact formatting matters to you.
# Prints one TSV row per check (check, expected, actual, verdict) and exits
# non-zero if any check fails. Keys are never printed.
set -uo pipefail

CTL=${1:?usage: verify-tollgatectl.sh PATH/TO/tollgatectl [HOST] [CONFIG]}
HOST=${2:-127.0.0.1}
CONFIG=${3:-$HOME/.config/cliproxyapi/config.yaml}
PORT=${TOLLGATE_PORT:-8317}
BASE="http://$HOST:$PORT"
NAME_WEB="verify-tollgatectl-web-$$"
NAME_MODELS="verify-tollgatectl-models-$$"
fails=0

row() { # check expected actual
  local verdict=VERIFIED
  [[ "$2" == "$3" ]] || { verdict="NOT VERIFIED"; fails=$((fails + 1)); }
  printf '%s\t%s\t%s\t%s\n' "$1" "$2" "$3" "$verdict"
}
code() { curl -s -m 60 -o /dev/null -w '%{http_code}' "$@"; }
key_count() { awk '/^ *api-keys:/{f=1;next} f&&/^ *- /{n++} f&&!/^ *- /{f=0} END{print n+0}' "$CONFIG"; }
models() { code "$BASE/v1/models" -H "Authorization: Bearer $1"; }
search() {
  code -X POST "$BASE/v1/tools/web_search" -H "Authorization: Bearer $1" \
    -H 'Content-Type: application/json' -d '{"query":"go programming language","max_results":1}'
}
grant_id() { "$CTL" apps | awk -v n="$1" '$2 == n {print $1}'; }

# The engine's management save re-serializes config.yaml (comments can move,
# and the web-tools defaults get written out), so compare parsed settings.
snapshot=$(mktemp)
trap 'rm -f "$snapshot"' EXIT
cp "$CONFIG" "$snapshot"
same_settings() {
  python3 -I -c 'import sys, yaml
a, b = (yaml.safe_load(open(f)) or {} for f in sys.argv[1:3])
for c in (a, b): c.pop("web-tools", None)
print("yes" if a == b else "no")' "$snapshot" "$CONFIG"
}

pid_before=$(systemctl --user show -p MainPID --value cliproxyapi.service 2>/dev/null || echo unknown)
keys_before=$(key_count)
printf 'check\texpected\tactual\tverdict\n'

key_web=$("$CTL" grant --name "$NAME_WEB" --scopes models,web 2>/dev/null)
row "grant models,web prints a key" yes "$([[ -n "$key_web" ]] && echo yes || echo no)"
key_models=$("$CTL" grant --name "$NAME_MODELS" 2>/dev/null)
row "grant models prints a key" yes "$([[ -n "$key_models" ]] && echo yes || echo no)"
row "api-keys grew by 2" "$((keys_before + 2))" "$(key_count)"

row "web key: GET /v1/models via $HOST" 200 "$(models "$key_web")"
row "web key: POST /v1/tools/web_search via $HOST" 200 "$(search "$key_web")"
row "models key: GET /v1/models via $HOST" 200 "$(models "$key_models")"
row "models key: web_search refused (no web scope)" 403 "$(search "$key_models")"

id_web=$(grant_id "$NAME_WEB")
id_models=$(grant_id "$NAME_MODELS")
row "apps lists the web grant with scopes models,web" "models,web" \
  "$("$CTL" apps | awk -v n="$NAME_WEB" '$2 == n {print $4}')"
row "apps lists the models grant" yes "$([[ -n "$id_models" ]] && echo yes || echo no)"

"$CTL" revoke "$id_web" 2>/dev/null
"$CTL" revoke "$id_models" 2>/dev/null
sleep 2 # the engine reloads its config asynchronously after a save
row "revoked web key: GET /v1/models" 401 "$(models "$key_web")"
row "revoked models key: GET /v1/models" 401 "$(models "$key_models")"
row "apps no longer lists either grant" "" "$(grant_id "$NAME_WEB")$(grant_id "$NAME_MODELS")"
row "revoke of an unknown id fails" 1 "$("$CTL" revoke no-such-grant >/dev/null 2>&1; echo $?)"

row "api-keys count back to baseline" "$keys_before" "$(key_count)"
row "other settings in config.yaml unchanged" yes "$(same_settings)"
row "engine process not restarted" "$pid_before" "$(systemctl --user show -p MainPID --value cliproxyapi.service 2>/dev/null || echo unknown)"

exit $((fails > 0))
