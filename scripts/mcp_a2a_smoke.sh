#!/usr/bin/env bash
set -euo pipefail

# Smoke test for api-server MCP ingress -> A2A bridge.
#
# Usage:
#   API_SERVER_BASE_URL=http://127.0.0.1:8080 \
#   MCP_SERVER_NAME=foo \
#   BKAPI_JWT=<jwt-from-blueking-gateway> \
#   ./scripts/mcp_a2a_smoke.sh
#
# Gateway (public path on BK APIGW):
#   API_SERVER_BASE_URL=https://<bk-hcm-apigw-host>/<stage> \
#   MCP_ROUTE_MODE=gateway \
#   MCP_SERVER_NAME=bk-hcm-devhk-tcloud-ziyan-cvm \
#   BK_APP_CODE=bk-hcm BK_APP_SECRET=<secret> BK_USERNAME=admin \
#   ./scripts/mcp_a2a_smoke.sh
#
# NOTE:
#   To avoid ambiguity, this script uses a single default prefix:
#   /api/v1/mcp/servers for both gateway/direct.
#   If your environment is different, set MCP_PATH_PREFIX explicitly.
#
# Direct api-server (internal ingress path):
#   API_SERVER_BASE_URL=http://127.0.0.1:8080 \
#   MCP_ROUTE_MODE=direct \
#   BK_USERNAME=admin BK_APP_CODE=bk-hcm BK_TENANT_ID=default \
#   ./scripts/mcp_a2a_smoke.sh

API_SERVER_BASE_URL="${API_SERVER_BASE_URL:-http://127.0.0.1:8080}"
MCP_SERVER_NAME="${MCP_SERVER_NAME:-foo}"
BK_USERNAME="${BK_USERNAME:-admin}"
BK_APP_CODE="${BK_APP_CODE:-bk-hcm}"
BK_APP_SECRET="${BK_APP_SECRET:-}"
BK_TENANT_ID="${BK_TENANT_ID:-default}"
BK_TICKET="${BK_TICKET:-}"
ACCESS_TOKEN="${ACCESS_TOKEN:-}"
BKAPI_JWT="${BKAPI_JWT:-}"
BKAPI_AUTHORIZATION="${BKAPI_AUTHORIZATION:-}"
RID="${RID:-mcp-a2a-smoke-$(date +%Y%m%d%H%M%S)}"
SMOKE_TEXT="${SMOKE_TEXT:-你好，请简单介绍 HCM 智能助手。}"
LOG_ROOT_DIR="${LOG_DIR:-./logs/mcp_a2a_smoke}"
LOG_DIR="${LOG_ROOT_DIR%/}/${RID}"
MCP_URL="${MCP_URL:-}"
MCP_PATH_PREFIX="${MCP_PATH_PREFIX:-}"
MCP_ROUTE_MODE="${MCP_ROUTE_MODE:-auto}"
MCP_PATH_PREFIX_USER_SET="false"
if [[ -n "$MCP_PATH_PREFIX" ]]; then
  MCP_PATH_PREFIX_USER_SET="true"
fi

mkdir -p "$LOG_DIR"

if [[ -z "$MCP_URL" ]]; then
  if [[ -z "$MCP_PATH_PREFIX" ]]; then
    case "$MCP_ROUTE_MODE" in
      gateway|direct|auto)
        MCP_PATH_PREFIX="/api/v1/mcp/servers"
        ;;
      *)
        echo "invalid MCP_ROUTE_MODE: ${MCP_ROUTE_MODE}, expected one of: gateway|direct|auto" >&2
        exit 2
        ;;
    esac
  fi
  MCP_URL="${API_SERVER_BASE_URL%/}${MCP_PATH_PREFIX}/${MCP_SERVER_NAME}/mcp/"
fi

echo "[config] rid=${RID}, log_dir=${LOG_DIR}, route_mode=${MCP_ROUTE_MODE}, path_prefix=${MCP_PATH_PREFIX}, path_prefix_user_set=${MCP_PATH_PREFIX_USER_SET}, mcp_url=${MCP_URL}"

build_bkapi_authorization() {
  python3 - <<'PY'
import json
import os

payload = {
    "bk_app_code": os.environ.get("BK_APP_CODE", ""),
    "bk_app_secret": os.environ.get("BK_APP_SECRET", ""),
}

username = os.environ.get("BK_USERNAME", "")
if username:
    payload["bk_username"] = username

ticket = os.environ.get("BK_TICKET", "")
if ticket:
    payload["bk_ticket"] = ticket

token = os.environ.get("ACCESS_TOKEN", "")
if token:
    payload["access_token"] = token

print(json.dumps(payload, ensure_ascii=False, separators=(",", ":")))
PY
}

header_args=(
  -H "Content-Type: application/json"
  -H "Accept: application/json, text/event-stream"
  -H "X-Bkapi-Request-Id: ${RID}"
  -H "X-Bk-Tenant-Id: ${BK_TENANT_ID}"
)
if [[ -n "$BKAPI_JWT" ]]; then
  header_args+=(-H "X-Bkapi-JWT: ${BKAPI_JWT}")
elif [[ -n "$BKAPI_AUTHORIZATION" ]]; then
  header_args+=(-H "X-Bkapi-Authorization: ${BKAPI_AUTHORIZATION}")
elif [[ -n "$BK_APP_SECRET" ]]; then
  BKAPI_AUTHORIZATION="$(build_bkapi_authorization)"
  header_args+=(-H "X-Bkapi-Authorization: ${BKAPI_AUTHORIZATION}")
else
  header_args+=(
    -H "X-Bkapi-User-Name: ${BK_USERNAME}"
    -H "X-Bkapi-App-Code: ${BK_APP_CODE}"
  )
fi

write_payload() {
  local file="$1"
  local method="$2"
  local params="${3:-null}"

  if [[ "$params" == "null" ]]; then
    cat >"$file" <<EOF
{"jsonrpc":"2.0","id":1,"method":"${method}"}
EOF
    return
  fi

  cat >"$file" <<EOF
{"jsonrpc":"2.0","id":1,"method":"${method}","params":${params}}
EOF
}

print_response_summary() {
  local resp_file="$1"
  if [[ ! -s "$resp_file" ]]; then
    echo "[verify] response body is empty"
    return
  fi

  local first_line
  first_line="$(tr -d '\r' <"$resp_file" | sed -n '1p' | cut -c 1-240)"
  echo "[verify] response first line: ${first_line}"
}

call_jsonrpc() {
  local step="$1"
  local method="$2"
  local params="${3:-null}"
  local req_file="${LOG_DIR}/${step}_request.json"
  local resp_file="${LOG_DIR}/${step}_response.txt"
  local code_file="${LOG_DIR}/${step}_status.txt"

  write_payload "$req_file" "$method" "$params"
  echo "[${step}] ${method} -> ${MCP_URL}"
  curl -sS -N -X POST "${header_args[@]}" \
    --data-binary "@${req_file}" \
    -w "%{http_code}" \
    -o "$resp_file" \
    "$MCP_URL" >"$code_file"

  local status
  status="$(<"$code_file")"
  echo "[${step}] HTTP ${status}, response: ${resp_file}"
  print_response_summary "$resp_file"
  if [[ "$status" -lt 200 || "$status" -ge 300 ]]; then
    echo "[${step}] failed, response body:" >&2
    sed -n '1,120p' "$resp_file" >&2
    return 1
  fi

  if ! validate_non_jsonrpc_error_response "$resp_file"; then
    echo "[${step}] failed: non JSON-RPC error response detected" >&2
    return 1
  fi
}

validate_non_jsonrpc_error_response() {
  local resp_file="$1"
  python3 - "$resp_file" <<'PY'
import json
import sys
from pathlib import Path

path = Path(sys.argv[1])
text = path.read_text(encoding="utf-8", errors="ignore").strip()
if not text:
    sys.exit(0)

candidates = []
stripped = text.strip()
if stripped.startswith("{"):
    candidates.append(stripped)
for line in text.splitlines():
    line = line.strip()
    if not line:
        continue
    if line.startswith("data:"):
        payload = line[5:].strip()
        if payload.startswith("{"):
            candidates.append(payload)
    elif line.startswith("{"):
        candidates.append(line)

for c in candidates:
    try:
        obj = json.loads(c)
    except Exception:
        continue

    if not isinstance(obj, dict):
        continue

    if "jsonrpc" in obj:
        continue

    if "code" in obj and "message" in obj:
        message = str(obj.get("message", ""))
        print(f"[verify] non-jsonrpc error response: code={obj.get('code')}, message={message}", file=sys.stderr)
        if "received unknown url path" in message:
            print("[verify] diagnosis: request reached api-server REST proxy fallback, but did not hit MCP ingress.", file=sys.stderr)
            print("[verify] check: api-server mcp.ingress.enable, mcp.ingress.basePath, deployed binary/config, and BK APIGW backend service target.", file=sys.stderr)
        sys.exit(1)

sys.exit(0)
PY
}

extract_jsonrpc_response() {
  local resp_file="$1"
  python3 - "$resp_file" <<'PY'
import json
import sys
from pathlib import Path

path = Path(sys.argv[1])
text = path.read_text(encoding="utf-8", errors="ignore")

candidates = []
for line in text.splitlines():
    line = line.strip()
    if not line:
        continue
    if line.startswith("data:"):
        payload = line[5:].strip()
        if payload:
            candidates.append(payload)
    elif line.startswith("{"):
        candidates.append(line)

if text.strip().startswith("{"):
    candidates.append(text.strip())

parsed = []
non_jsonrpc_errors = []
for c in candidates:
    try:
        obj = json.loads(c)
    except Exception:
        continue
    if not isinstance(obj, dict):
        continue
    if "jsonrpc" in obj or "result" in obj or "error" in obj:
        parsed.append(obj)
        continue
    if "code" in obj and "message" in obj:
        non_jsonrpc_errors.append(obj)

if non_jsonrpc_errors:
    obj = non_jsonrpc_errors[-1]
    print(f"[verify] non-jsonrpc error response: code={obj.get('code')}, message={obj.get('message')}", file=sys.stderr)
    sys.exit(2)

if not parsed:
    print("", end="")
    sys.exit(0)

print(json.dumps(parsed[-1], ensure_ascii=False))
PY
}

validate_tools_list_response() {
  local resp_file="$1"
  local expected_tool="$2"
  local parsed
  if ! parsed="$(extract_jsonrpc_response "$resp_file")"; then
    echo "[verify] tools/list parse failed: response is not a valid JSON-RPC success payload: ${resp_file}" >&2
    return 1
  fi
  if [[ -z "$parsed" ]]; then
    echo "[verify] tools/list parse failed: no valid JSON-RPC payload found in ${resp_file}" >&2
    return 1
  fi

  python3 - "$parsed" "$expected_tool" <<'PY'
import json
import sys

obj = json.loads(sys.argv[1])
expected = sys.argv[2]

if obj.get("error"):
    raise SystemExit(f"[verify] tools/list has jsonrpc error: {obj['error']}")

result = obj.get("result")
if not isinstance(result, dict):
    raise SystemExit("[verify] tools/list missing result object")

tools = result.get("tools")
if not isinstance(tools, list):
    raise SystemExit("[verify] tools/list result.tools is not a list")

names = [t.get("name") for t in tools if isinstance(t, dict)]
if expected not in names:
    raise SystemExit(f"[verify] tools/list missing expected tool: {expected}; got={names}")

print(f"[verify] tools/list OK, found expected tool: {expected}")
PY
}

validate_tools_call_response() {
  local resp_file="$1"
  local parsed
  if ! parsed="$(extract_jsonrpc_response "$resp_file")"; then
    echo "[verify] tools/call parse failed: response is not a valid JSON-RPC success payload: ${resp_file}" >&2
    return 1
  fi
  if [[ -z "$parsed" ]]; then
    echo "[verify] tools/call parse failed: no valid JSON-RPC payload found in ${resp_file}" >&2
    return 1
  fi

  python3 - "$parsed" <<'PY'
import json
import sys

obj = json.loads(sys.argv[1])
if obj.get("error"):
    raise SystemExit(f"[verify] tools/call has jsonrpc error: {obj['error']}")

result = obj.get("result")
if not isinstance(result, dict):
    raise SystemExit("[verify] tools/call missing result object")

if result.get("isError") is True:
    raise SystemExit(f"[verify] tools/call returned isError=true: {result}")

ok = False
content = result.get("content")
if isinstance(content, list):
    for item in content:
        if isinstance(item, dict):
            text = item.get("text")
            if isinstance(text, str) and text.strip():
                ok = True
                break

if not ok:
    structured = result.get("structuredContent")
    if isinstance(structured, dict) and structured:
        ok = True

if not ok:
    raise SystemExit("[verify] tools/call result has no usable content (text/structuredContent)")

print("[verify] tools/call OK, got usable result content")
PY
}

initialize_params='{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"hcm-smoke","version":"1.0.0"}}'
call_jsonrpc "01_initialize" "initialize" "$initialize_params"

# MCP initialized is a notification and has no response body requirement. It is
# still recorded to verify the api-server transport accepts the standard flow.
initialized_req="${LOG_DIR}/02_initialized_request.json"
initialized_resp="${LOG_DIR}/02_initialized_response.txt"
initialized_code="${LOG_DIR}/02_initialized_status.txt"
cat >"$initialized_req" <<'EOF'
{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}
EOF
echo "[02_initialized] notifications/initialized -> ${MCP_URL}"
curl -sS -N -X POST "${header_args[@]}" \
  --data-binary "@${initialized_req}" \
  -w "%{http_code}" \
  -o "$initialized_resp" \
  "$MCP_URL" >"$initialized_code"
initialized_status="$(<"$initialized_code")"
echo "[02_initialized] HTTP ${initialized_status}, response: ${initialized_resp}"
print_response_summary "$initialized_resp"
if [[ "$initialized_status" -lt 200 || "$initialized_status" -ge 300 ]]; then
  echo "[02_initialized] failed, response body:" >&2
  sed -n '1,120p' "$initialized_resp" >&2
  exit 1
fi

if ! validate_non_jsonrpc_error_response "$initialized_resp"; then
  echo "[02_initialized] failed: non JSON-RPC error response detected" >&2
  exit 1
fi

call_jsonrpc "03_tools_list" "tools/list"
validate_tools_list_response "${LOG_DIR}/03_tools_list_response.txt" "send_message"

tool_params=$(SMOKE_TEXT_VALUE="$SMOKE_TEXT" RID_VALUE="$RID" python3 - <<'PY'
import json
import os
print(json.dumps({
    "name": "send_message",
    "arguments": {
        "text": os.environ["SMOKE_TEXT_VALUE"],
    },
    "_meta": {
        "progressToken": os.environ["RID_VALUE"] + "-progress"
    },
}, ensure_ascii=False))
PY
)
call_jsonrpc "04_tools_call_send_message" "tools/call" "$tool_params"
validate_tools_call_response "${LOG_DIR}/04_tools_call_send_message_response.txt"

echo "Smoke test finished. Logs are saved in ${LOG_DIR}"