#!/usr/bin/env bash
# Coverage gate. Usage: scripts/coverage.sh [threshold]
set -euo pipefail
THRESHOLD="${1:-80}"
go test ./... -coverprofile=cover.out -covermode=atomic
pct=$(go tool cover -func=cover.out | tail -1 | awk '{print $3}' | tr -d '%')
printf 'total %s%%\n' "$pct"
awk -v p="$pct" -v t="$THRESHOLD" 'BEGIN{exit !(p+0 < t)}' && { echo "FAIL: below ${THRESHOLD}%"; exit 1; }
exit 0
