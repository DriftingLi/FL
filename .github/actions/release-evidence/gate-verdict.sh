#!/usr/bin/env bash
# gateVerdict adapter：只有 SHA 对齐的 run success + 最新 deployment success 才放行。
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec node "$SCRIPT_DIR/release-evidence.mjs" gateVerdict
