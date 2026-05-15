set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

mkdir -p bin
go build -o ./bin/mygrep ./cmd/mygrep

: > ./mygrep.pids
for port in 8081 8082 8083; do
  ./bin/mygrep -mode=server -listen=":${port}" -id="w${port}" &
  pid=$!
  echo "${pid}" >> ./mygrep.pids
  echo "worker :${port} запущен (pid ${pid})"
done

echo
echo "Кластер поднят. Остановить: xargs kill < mygrep.pids && rm mygrep.pids"
