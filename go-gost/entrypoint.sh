#!/bin/sh
# PID 1 entrypoint for a node container without systemd/OpenRC.
# The container must mount /etc/gost and provide the gost binary/config.
set -u

INSTALL_DIR="${GOST_INSTALL_DIR:-/etc/gost}"
GOST_BIN="$INSTALL_DIR/gost"
SINGBOX_BIN="$INSTALL_DIR/sing-box"
SINGBOX_CONFIG="$INSTALL_DIR/sing-box.json"
GOST_LOG="$INSTALL_DIR/gost.log"
GOST_PID_FILE="$INSTALL_DIR/gost.pid"
SINGBOX_LOG="$INSTALL_DIR/sing-box.log"
SINGBOX_PID_FILE="$INSTALL_DIR/sing-box.pid"
GOST_PID=""
SINGBOX_PID=""
STOPPING=0

stop_children() {
  STOPPING=1
  for pid in "$GOST_PID" "$SINGBOX_PID"; do
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null || true
    fi
  done
  sleep 1
  for pid in "$GOST_PID" "$SINGBOX_PID"; do
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
      kill -9 "$pid" 2>/dev/null || true
    fi
  done
  rm -f "$SINGBOX_PID_FILE"
  rm -f "$GOST_PID_FILE"
}
is_singbox_pid() {
  local pid="$1"
  [ -n "$pid" ] || return 1
  kill -0 "$pid" 2>/dev/null || return 1
  tr '\0' ' ' < "/proc/$pid/cmdline" 2>/dev/null | grep -q 'sing-box'
}
trap 'stop_children; exit 0' INT TERM HUP

if [ ! -x "$GOST_BIN" ]; then
  echo "gost binary is missing: $GOST_BIN" >&2
  exit 1
fi

mkdir -p "$INSTALL_DIR"
cd "$INSTALL_DIR" || exit 1
"$GOST_BIN" >> "$GOST_LOG" 2>&1 &
GOST_PID=$!
printf '%s\n' "$GOST_PID" > "$GOST_PID_FILE"

while [ "$STOPPING" -eq 0 ]; do
  if ! kill -0 "$GOST_PID" 2>/dev/null; then
    echo "gost exited; stopping node container" >&2
    stop_children
    exit 1
  fi

  if [ -x "$SINGBOX_BIN" ] && [ -s "$SINGBOX_CONFIG" ]; then
    FILE_PID=""
    if [ -f "$SINGBOX_PID_FILE" ]; then
      FILE_PID=$(cat "$SINGBOX_PID_FILE" 2>/dev/null || true)
    fi
    if is_singbox_pid "$FILE_PID"; then
      SINGBOX_PID="$FILE_PID"
    elif ! is_singbox_pid "$SINGBOX_PID"; then
      "$SINGBOX_BIN" run -c "$SINGBOX_CONFIG" >> "$SINGBOX_LOG" 2>&1 &
      SINGBOX_PID=$!
      printf '%s\n' "$SINGBOX_PID" > "$SINGBOX_PID_FILE"
    fi
  fi
  sleep 2
done

stop_children
exit 0
