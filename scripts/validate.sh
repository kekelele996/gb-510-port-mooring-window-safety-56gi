#!/usr/bin/env sh
set -eu
project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_root"
set -a
if [ -f .env ]; then . ./.env; else . ./.env.example; fi
set +a
(command -v jq >/dev/null 2>&1) || { echo "jq is required for API validation" >&2; exit 1; }
(cd backend && go test ./... && go build ./...)
(cd frontend && npm install --no-audit --no-fund && npm run build)
docker compose config --quiet
docker compose up -d --build
cleanup() { docker compose down -v --remove-orphans; }
if [ "${KEEP_RUNNING:-0}" = "1" ]; then
  trap cleanup INT TERM
else
  trap cleanup EXIT INT TERM
fi
i=0
until curl -fsS "http://127.0.0.1:${BACKEND_PORT:-19510}/healthz" >/dev/null; do
  i=$((i+1)); [ "$i" -lt 60 ] || { docker compose logs; exit 1; }; sleep 2
done
curl -fsS "http://127.0.0.1:${FRONTEND_PORT:-18510}/" >/dev/null
token=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT:-19510}/api/auth/login" -H 'Content-Type: application/json' -d '{"username":"admin","password":"Admin123!"}' | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')
[ -n "$token" ]
viewer_token=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT:-19510}/api/auth/login" -H 'Content-Type: application/json' -d '{"username":"viewer","password":"Admin123!"}' | jq -er '.data.token')
operator_token=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT:-19510}/api/auth/login" -H 'Content-Type: application/json' -d '{"username":"operator","password":"Admin123!"}' | jq -er '.data.token')
reviewer_token=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT:-19510}/api/auth/login" -H 'Content-Type: application/json' -d '{"username":"reviewer","password":"Admin123!"}' | jq -er '.data.token')
curl -fsS "http://127.0.0.1:${BACKEND_PORT:-19510}/api/overview" -H "Authorization: Bearer $token" >/dev/null
curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/session" -H "Authorization: Bearer $token" | jq -e '.data.role == "admin" and (.data.requestId | length > 0)' >/dev/null
curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/runtime" -H "Authorization: Bearer $token" | jq -e '.data.appName and .data.databaseDriver and (.data.requestLimit > 0)' >/dev/null
paths=$(sed -n "s/.*path: '\\([^']*\\)'.*/\\1/p" frontend/src/types/status.ts)
for path in $paths; do
  curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/$path?page=1&pageSize=20" -H "Authorization: Bearer $token" | jq -e '.data | type == "array"' >/dev/null
done
entity_config=$(sed -n "s/.*path: '\\([^']*\\)'.*statuses: \\['\\([^']*\\)', '\\([^']*\\)'.*/\\1|\\2|\\3/p" frontend/src/types/status.ts | head -n 1)
resource=$(printf '%s' "$entity_config" | cut -d '|' -f 1)
initial_status=$(printf '%s' "$entity_config" | cut -d '|' -f 2)
next_status=$(printf '%s' "$entity_config" | cut -d '|' -f 3)
now=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
code="SMOKE-$(date +%s)"
payload=$(printf '{"code":"%s","name":"Runtime smoke record","description":"Automated Compose workflow validation","facility":"Validation Lab","owner":"admin","category":"smoke","riskLevel":"low","metricValue":1,"metricUnit":"unit","effectiveAt":"%s","evidence":"scripts/validate.sh","relatedCode":"SMOKE"}' "$code" "$now")
created=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/$resource" -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d "$payload")
id=$(printf '%s' "$created" | jq -er '.data.id')
version=$(printf '%s' "$created" | jq -er '.data.version')
printf '%s' "$created" | jq -e --arg status "$initial_status" '.data.status == $status' >/dev/null
transition=$(printf '{"status":"%s","expectedVersion":%s,"reason":"automated runtime validation"}' "$next_status" "$version")
curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/$resource/$id/transition" -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d "$transition" | jq -e --arg status "$next_status" '.data.status == $status' >/dev/null

viewer_status=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "http://127.0.0.1:${BACKEND_PORT}/api/vessels" -H "Authorization: Bearer $viewer_token" -H 'Content-Type: application/json' -d "$payload")
[ "$viewer_status" = "403" ]

clearance_code="CLEARANCE-SMOKE-$(date +%s)"
clearance_payload=$(printf '{"code":"%s","name":"Two-person clearance validation","description":"Independent reviewer workflow","facility":"Validation Berth","owner":"operator","category":"safety","riskLevel":"high","metricValue":18,"metricUnit":"kn","effectiveAt":"%s","evidence":"Mooring lines and rollback checked","relatedCode":"WW-SMOKE","windowVersion":1}' "$clearance_code" "$now")
clearance=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/clearance" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "$clearance_payload")
clearance_id=$(printf '%s' "$clearance" | jq -er '.data.id')
clearance_version=$(printf '%s' "$clearance" | jq -er '.data.version')
submit_payload=$(printf '{"status":"cleared","expectedVersion":%s,"reason":"operator submitted safety package","windowVersion":11}' "$clearance_version")
submitted=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/clearance/$clearance_id/transition" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "$submit_payload")
printf '%s' "$submitted" | jq -e '.data.status == "pending" and .data.submittedBy == "operator" and .data.confirmedBy == "" and .data.windowVersion == 11' >/dev/null
submitted_version=$(printf '%s' "$submitted" | jq -er '.data.version')
self_payload=$(printf '{"status":"cleared","expectedVersion":%s,"reason":"operator attempted self approval","windowVersion":11}' "$submitted_version")
self_status=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "http://127.0.0.1:${BACKEND_PORT}/api/clearance/$clearance_id/transition" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "$self_payload")
[ "$self_status" = "422" ]
reviewed=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/clearance/$clearance_id/transition" -H "Authorization: Bearer $reviewer_token" -H 'Content-Type: application/json' -d "$self_payload")
printf '%s' "$reviewed" | jq -e '.data.status == "cleared" and .data.submittedBy == "operator" and .data.confirmedBy == "reviewer" and .data.windowVersion == 11' >/dev/null

curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/audits?page=1&pageSize=100" -H "Authorization: Bearer $token" | jq -e '.meta.total >= 2' >/dev/null
curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/audits?page=1&pageSize=100" -H "Authorization: Bearer $token" | jq -e --argjson id "$clearance_id" '.data | any(.entityType == "SafetyClearance" and .entityId == $id and .windowVersion == 11 and (.requestId | length > 0))' >/dev/null
curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/audit-summary?windowHours=24" -H "Authorization: Bearer $token" | jq -e '.data.total >= 2 and .data.transitions >= 1' >/dev/null
docker compose ps
[ "${KEEP_RUNNING:-0}" = "1" ] && echo "KEEP_RUNNING=1: containers left running for browser validation"
