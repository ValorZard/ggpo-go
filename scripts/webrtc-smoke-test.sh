#!/usr/bin/env bash
# Smoke test for the WebRTC data channel transport: builds cmd/signaling and
# example/webrtc, runs a signaling server plus a hosting and a joining player,
# and verifies both peers synchronize and end on the same final state.
#
# Usage: ./scripts/webrtc-smoke-test.sh [frames]
#   frames   number of frames to run (default 180)
#
# Environment:
#   SIGNALING_PORT   port for the signaling server (default 3901)
set -euo pipefail

FRAMES="${1:-180}"
PORT="${SIGNALING_PORT:-3901}"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

EXE=""
if [ "$(go env GOOS)" = "windows" ]; then
    EXE=".exe"
fi

WORKDIR="$(mktemp -d)"
SIGNALING_PID=""
HOST_PID=""
JOIN_PID=""

cleanup() {
    for pid in "$SIGNALING_PID" "$HOST_PID" "$JOIN_PID"; do
        [ -n "$pid" ] && kill "$pid" 2>/dev/null || true
    done
    rm -rf "$WORKDIR"
}
trap cleanup EXIT

fail() {
    echo "FAIL: $*" >&2
    echo "--- signaling log ---" >&2; cat "$WORKDIR/signaling.log" >&2 || true
    echo "--- host log ---" >&2; cat "$WORKDIR/host.log" >&2 || true
    echo "--- join log ---" >&2; cat "$WORKDIR/join.log" >&2 || true
    exit 1
}

echo "Building binaries..."
go build -o "$WORKDIR/signaling$EXE" ./cmd/signaling
go build -o "$WORKDIR/webrtcdemo$EXE" ./example/webrtc

echo "Starting signaling server on port $PORT..."
"$WORKDIR/signaling$EXE" -addr "127.0.0.1:$PORT" >"$WORKDIR/signaling.log" 2>&1 &
SIGNALING_PID=$!

# Wait until the server answers /ping.
for _ in $(seq 1 50); do
    if curl -fs "http://127.0.0.1:$PORT/ping" >/dev/null 2>&1; then
        break
    fi
    kill -0 "$SIGNALING_PID" 2>/dev/null || fail "signaling server exited early"
    sleep 0.2
done
curl -fs "http://127.0.0.1:$PORT/ping" >/dev/null 2>&1 || fail "signaling server did not come up on port $PORT"

echo "Starting host..."
"$WORKDIR/webrtcdemo$EXE" -transport webrtc -signaling "http://127.0.0.1:$PORT" \
    -host -stun "" -frames "$FRAMES" >"$WORKDIR/host.log" 2>&1 &
HOST_PID=$!

# Wait for the host to print the lobby ID.
LOBBY=""
for _ in $(seq 1 50); do
    LOBBY="$(sed -n 's/.*share this ID: \([A-Za-z]*\).*/\1/p' "$WORKDIR/host.log" | head -n 1)"
    [ -n "$LOBBY" ] && break
    kill -0 "$HOST_PID" 2>/dev/null || fail "host exited before printing a lobby ID"
    sleep 0.2
done
[ -n "$LOBBY" ] && echo "Lobby ID: $LOBBY" || fail "host never printed a lobby ID"

echo "Starting joiner..."
"$WORKDIR/webrtcdemo$EXE" -transport webrtc -signaling "http://127.0.0.1:$PORT" \
    -join "$LOBBY" -stun "" -frames "$FRAMES" >"$WORKDIR/join.log" 2>&1 &
JOIN_PID=$!

echo "Waiting for both players to finish..."
wait "$HOST_PID" || fail "host exited with an error"
HOST_PID=""
wait "$JOIN_PID" || fail "joiner exited with an error"
JOIN_PID=""

grep -q "Synchronized, game running." "$WORKDIR/host.log" || fail "host never synchronized"
grep -q "Synchronized, game running." "$WORKDIR/join.log" || fail "joiner never synchronized"

HOST_FINAL="$(grep "Done. Final state:" "$WORKDIR/host.log")"
JOIN_FINAL="$(grep "Done. Final state:" "$WORKDIR/join.log")"
[ -n "$HOST_FINAL" ] || fail "host never printed a final state"
[ "$HOST_FINAL" = "$JOIN_FINAL" ] || fail "final states differ:
  host:   $HOST_FINAL
  joiner: $JOIN_FINAL"

echo "  host:   $HOST_FINAL"
echo "  joiner: $JOIN_FINAL"
echo "PASS: both peers synchronized and finished $FRAMES frames with identical state"
