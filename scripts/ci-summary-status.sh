#!/usr/bin/env bash
# ===== ci-summary 结论聚合（单点实现，ADR-0053 §10 / spec #1053）=====
#
# 背景：`cd.yml` 的 gate job 与 `testing-smoke.yml` 的补发判定各自内联了**逐字相同**的一段 jq
# ——「同一 SHA 可能有多条 ci-summary 记录（旧版 push+pull_request 双跑），按『任一 failure 即失败
# / 有 success 即通过』聚合，避免 head -1 取到 skipped 误判」。改一处忘另一处，会让两条流水线
# 对同一份结论给出不同摘要。
#
# 用法：ci-summary-status.sh <sha>
#   输出（stdout，单行）：success | failure | missing | other
#   退出码：0（判定本身不判红；调用方按输出决定阻断与否）
#
# 依赖：gh（已认证，CI 里用 GH_TOKEN）、环境变量 GITHUB_REPOSITORY。
set -euo pipefail

SHA="${1:-}"
if [[ -z "$SHA" ]]; then
  echo "用法: $0 <sha>" >&2
  exit 2
fi
if [[ -z "${GITHUB_REPOSITORY:-}" ]]; then
  echo "缺少 GITHUB_REPOSITORY（本脚本按 check-runs API 判定 ci-summary）" >&2
  exit 2
fi

gh api "repos/$GITHUB_REPOSITORY/commits/$SHA/check-runs?per_page=100" --jq '
  [.check_runs[] | select(.name=="ci-summary") | .conclusion]
  | if any(. == "failure") then "failure"
    elif any(. == "success") then "success"
    elif length == 0 then "missing"
    else "other" end'
