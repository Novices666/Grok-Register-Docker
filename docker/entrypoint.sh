#!/usr/bin/env bash
set -euo pipefail

export GROK_HOME="${GROK_HOME:-/data/grok}"
export GROK_PYTHON="${GROK_PYTHON:-/opt/cloakbrowser-venv/bin/python}"
export GROK_TURNSTILE_SCRIPT="${GROK_TURNSTILE_SCRIPT:-/usr/local/share/grok-reg/turnstile_mint.py}"
export GROK_TURNSTILE_POOL_SCRIPT="${GROK_TURNSTILE_POOL_SCRIPT:-/usr/local/share/grok-reg/turnstile_pool.py}"
export CLOAKBROWSER_SUPPRESS_FONT_WARNING="${CLOAKBROWSER_SUPPRESS_FONT_WARNING:-1}"

write_config() {
  mkdir -p "${GROK_HOME}"
  if [ -f "${GROK_HOME}/config.env" ]; then
    return
  fi
  cat > "${GROK_HOME}/config.env" <<EOF
EMAIL_MODE=${EMAIL_MODE:-tempmail}
EMAIL_DOMAIN=${EMAIL_DOMAIN:-}
EMAIL_API=${EMAIL_API:-http://127.0.0.1:8080}
TESTMAIL_API_KEY=${TESTMAIL_API_KEY:-}
TESTMAIL_NAMESPACE=${TESTMAIL_NAMESPACE:-}
TESTMAIL_DOMAIN=${TESTMAIL_DOMAIN:-inbox.testmail.app}

CLEARANCE_ENABLED=${CLEARANCE_ENABLED:-1}
CLEARANCE_MODE=${CLEARANCE_MODE:-auto}
CLEARANCE_AUTO_STOP=${CLEARANCE_AUTO_STOP:-0}
CF_IMPERSONATE=${CF_IMPERSONATE:-chrome_131}
CF_IMPERSONATE_FALLBACK=${CF_IMPERSONATE_FALLBACK:-chrome_124,chrome_120}
REGISTER_PROXY=${REGISTER_PROXY:-http://privoxy:8118}
FLARESOLVERR_URL=${FLARESOLVERR_URL:-http://flaresolverr:8191}
CLEARANCE_PROXY=${CLEARANCE_PROXY:-http://privoxy:8118}
CLEARANCE_URLS=${CLEARANCE_URLS:-https://accounts.x.ai,https://x.ai,https://status.x.ai,https://console.x.ai,https://auth.x.ai}

TURNSTILE_PROVIDER=${TURNSTILE_PROVIDER:-browser}
TURNSTILE_MODE=${TURNSTILE_MODE:-offscreen}
LITE_SOLVER_URL=${LITE_SOLVER_URL:-http://127.0.0.1:5072}

PROTOCOL_HTTP=${PROTOCOL_HTTP:-1}
HTTP_POOL_SIZE=${HTTP_POOL_SIZE:-8}
TEMPMAIL_LOL_RETRIES=${TEMPMAIL_LOL_RETRIES:-30}
TEMPMAIL_LOL_MIN_INTERVAL_MS=${TEMPMAIL_LOL_MIN_INTERVAL_MS:-1500}
OAUTH_MIN_INTERVAL_SEC=${OAUTH_MIN_INTERVAL_SEC:-4}
OAUTH_RETRY_SEC=${OAUTH_RETRY_SEC:-45}
OAUTH_WORKERS=${OAUTH_WORKERS:-0}
PROBE_ENABLED=${PROBE_ENABLED:-1}
PROBE_WARMUP_SEC=${PROBE_WARMUP_SEC:-1.5}

HTTPS_PROXY=${HTTPS_PROXY:-http://privoxy:8118}
HTTP_PROXY=${HTTP_PROXY:-http://privoxy:8118}
NO_PROXY=${NO_PROXY:-127.0.0.1,localhost,privoxy,flaresolverr,warp-proxy}

OUTPUT_SSO_ENABLED=${OUTPUT_SSO_ENABLED:-1}
OUTPUT_GROK2API_SSO_ENABLED=${OUTPUT_GROK2API_SSO_ENABLED:-1}
OUTPUT_CPA_ENABLED=${OUTPUT_CPA_ENABLED:-1}
PHYSICAL_CAP=${PHYSICAL_CAP:-0}
TURNSTILE_WORKERS=${TURNSTILE_WORKERS:-2}

CPA_UPLOAD_ENABLED=${CPA_UPLOAD_ENABLED:-0}
CPA_MANAGEMENT_BASE=${CPA_MANAGEMENT_BASE:-http://host.docker.internal:8317/v0/management}
CPA_MANAGEMENT_KEY=${CPA_MANAGEMENT_KEY:-}
CPA_UPLOAD_TIMEOUT_SEC=${CPA_UPLOAD_TIMEOUT_SEC:-30}
CPA_UPLOAD_RETRIES=${CPA_UPLOAD_RETRIES:-2}
CPA_UPLOAD_NAME_TEMPLATE=${CPA_UPLOAD_NAME_TEMPLATE:-{email}.json}
CPA_UPLOAD_VERIFY=${CPA_UPLOAD_VERIFY:-1}
CPA_UPLOAD_MODE=${CPA_UPLOAD_MODE:-multipart}
EOF
  chmod 600 "${GROK_HOME}/config.env"
}

stop_grok() {
  echo "[docker] stopping grok..."
  grok stop >/dev/null 2>&1 || true
}

start_web() {
  if [ "${WEB_ENABLED:-1}" != "1" ]; then
    return
  fi
  if [ -z "${WEB_PASSWORD:-}" ]; then
    echo "[docker] WEB_PASSWORD is required when WEB_ENABLED=1" >&2
    exit 1
  fi
  export WEB_ADDR="${WEB_ADDR:-:8090}"
  export WEB_USERNAME="${WEB_USERNAME:-admin}"
  export GROK_BIN="${GROK_BIN:-/usr/local/bin/grok}"
  echo "[docker] starting web console on ${WEB_ADDR}"
  grok-web &
  web_pid="$!"
}

case "${1:-run}" in
  run|start)
    write_config
    mkdir -p "${GROK_HOME}/logs" "${GROK_HOME}/outputs"
    rm -f "${GROK_HOME}/run.pid" "${GROK_HOME}/run.lock"
    web_pid=""
    start_web

    trap 'stop_grok; if [ -n "${web_pid}" ]; then kill "${web_pid}" >/dev/null 2>&1 || true; fi' TERM INT

    if [ -n "${web_pid}" ]; then
      echo "[docker] web console is ready; start registration from the browser"
      wait "${web_pid}" 2>/dev/null || true
    else
      echo "[docker] WEB_ENABLED=0; container is idle"
      while true; do sleep 3600; done
    fi
    ;;
  status|stop|logs|upload|help|version)
    write_config
    exec grok "$@"
    ;;
  *)
    exec "$@"
    ;;
esac
