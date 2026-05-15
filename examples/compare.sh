set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

FILE="${1:-/usr/share/dict/words}"
PATTERN="${2:-^abc}"
WORKERS="http://localhost:8081,http://localhost:8082,http://localhost:8083"

[[ -x ./bin/mygrep ]] || go build -o ./bin/mygrep ./cmd/mygrep

echo "== file=${FILE}  pattern=${PATTERN} =="

SYS_OUT=$(grep -nE "${PATTERN}" "${FILE}" | sed "s#^#${FILE}:#")
MY_OUT=$(./bin/mygrep \
  -mode=coordinator \
  -workers="${WORKERS}" \
  -replication=3 -quorum=2 \
  -chunk=500 -parallelism=8 \
  -e="${PATTERN}" "${FILE}")

SYS_N=$(printf "%s\n" "${SYS_OUT}" | wc -l | tr -d ' ')
MY_N=$(printf "%s\n" "${MY_OUT}"  | wc -l | tr -d ' ')

echo "grep:   ${SYS_N} строк"
echo "mygrep: ${MY_N} строк"

if diff <(printf "%s\n" "${SYS_OUT}" | sort) <(printf "%s\n" "${MY_OUT}" | sort) >/dev/null; then
  echo "OK: выводы идентичны"
else
  echo "DIFF (первые 20 строк):"
  diff <(printf "%s\n" "${SYS_OUT}" | sort) <(printf "%s\n" "${MY_OUT}" | sort) | head -20
  exit 1
fi
