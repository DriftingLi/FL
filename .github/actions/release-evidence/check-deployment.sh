#!/usr/bin/env bash
# checkDeployment adapter：记录存在不等于成功，判定委托给 ReleaseEvidence 核心模块。
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec node "$SCRIPT_DIR/release-evidence.mjs" checkDeployment
