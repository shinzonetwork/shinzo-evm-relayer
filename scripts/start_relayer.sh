#!/usr/bin/env bash
set -euo pipefail

# Ensure we run from repo root
cd "$(dirname "$0")/.."

BIN="./build/shinzo-evm-relayer"

EVM_RPC="${EVM_RPC:-ws://127.0.0.1:8585}"
OUTPOST="${OUTPOST:-0x5FbDB2315678afecb367f032d93F642f64180aa3}"
CHAIN_ID="${CHAIN_ID:-9001}"
KEY_NAME="${KEY_NAME:-relayer}"
HD_PATH="${HD_PATH:-"m/44'/60'/0'/0/0"}"

if [ ! -x "$BIN" ]; then
  echo ">> binary not found at $BIN — building"
  if ! command -v make >/dev/null 2>&1; then
    echo "make is required to build the binary (target: build)" >&2
    exit 1
  fi
  make build
fi

MNEM_FILE="$(mktemp -t shinzo_mnemonic_XXXXXX.txt)"
cleanup() { rm -f "$MNEM_FILE" || true; }
trap cleanup EXIT

echo "divert tenant reveal hire thing jar carry lonely magic oak audit fiber earth catalog cheap merry print clown portion speak daring giant weird slight" > "$MNEM_FILE"

echo ">> importing relayer key from temp mnemonic file"
"$BIN" keys add \
  --name "$KEY_NAME" \
  --mnemonic-file "$MNEM_FILE" \
  --hd-path "$HD_PATH"

rm -f "$MNEM_FILE"

echo ">> starting relayer"
exec "$BIN" start \
  --evm.rpc "$EVM_RPC" \
  --evm.contract "$OUTPOST" \
  --cosmos.chain_id "$CHAIN_ID"
