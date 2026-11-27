#!/usr/bin/env bash
# waitForRun adapter：轮询与状态解释委托给 ReleaseEvidence 核心模块。
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec node "$SCRIPT_DIR/release-evidence.mjs" waitForRun
