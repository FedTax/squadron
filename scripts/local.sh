#!/usr/bin/env bash
set -euo pipefail
repo=$(cd "$(dirname "$0")/.." && pwd)
root=${SQUADRON_TEST_ROOT:-"$repo/.local-testing"}
mkdir -p "$root"
root=$(cd "$root" && pwd)
binary="$root/bin/squadron"
action=${1:-test}
shift || true
build() {
  mkdir -p "$root/bin"
  go -C "$repo" build -o "$binary" ./cmd/cli
}
run_scenario() {
  local name=$1
  local scenario="$repo/examples/local-testing/$name.json"
  if [[ ! -f "$scenario" ]]; then
    echo "Unknown scenario: $name" >&2
    exit 2
  fi
  local output
  mkdir -p "$root/runs"
  output=$(mktemp -d "$root/runs/$name.XXXXXX")
  python3 "$repo/scripts/local-test.py" "$scenario" --binary "$binary" --output "$output/result"
}
case "$action" in
  build) build ;;
  test)
    build
    go -C "$repo" test ./config/... ./mission/... ./agent/... ./aitools/... ./store/... ./humaninput/... ./gateway/... ./mcphost/...
    for scenario in routing delegation failure missing-input; do run_scenario "$scenario"; done
    ;;
  run)
    build
    if [[ $# -eq 0 ]]; then set -- routing delegation failure missing-input; fi
    for scenario in "$@"; do run_scenario "$scenario"; done
    ;;
  verify|start)
    if [[ $# -ne 1 ]]; then echo "Usage: $0 $action CONFIG_DIRECTORY" >&2; exit 2; fi
    config=$(cd "$1" && pwd)
    build
    cd "$config"
    if [[ "$action" == verify ]]; then
      exec "$binary" --squadron-home "$config/.squadron" verify .
    else
      exec "$binary" --squadron-home "$config/.squadron" engage --foreground --headless --init -c .
    fi
    ;;
  *) echo "Usage: $0 {build|test|run [SCENARIO...]|verify CONFIG|start CONFIG}" >&2; exit 2 ;;
esac
