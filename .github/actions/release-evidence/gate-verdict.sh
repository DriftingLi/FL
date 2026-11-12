#!/usr/bin/env bash
# gateVerdict：组合 SHA 校验 + run 状态 + deployment 记录，输出 verdict=green/red。
# 拥有轮询语义（间隔/超时），调用方不需要自己写循环。
set -euo pipefail

SHA="${INPUT_SHA:?INPUT_SHA is required}"
ENVIRONMENT="${INPUT_ENVIRONMENT:-testing}"
RUN_ID="${INPUT_RUN_ID:-}"
TIMEOUT="${INPUT_TIMEOUT:-480}"
POLL_INTERVAL="${INPUT_POLL_INTERVAL:-30}"

# ── 1. SHA 校验（有 run-id 时）────────────────────────────────────────────
if [ -n "$RUN_ID" ]; then
  HEAD_SHA=$(gh run view "$RUN_ID" --repo "$GITHUB_REPOSITORY" \
             --json headSha --jq '.headSha' || echo 'unknown')
  if [ "$HEAD_SHA" != "$SHA" ] && [ "$HEAD_SHA" != "unknown" ]; then
    echo "verdict=red" >> "$GITHUB_OUTPUT"
    echo "reason=SHA mismatch: run ${RUN_ID} head ${HEAD_SHA} != expected ${SHA}" >> "$GITHUB_OUTPUT"
    echo "::error::SHA 错位：run ${RUN_ID} head_sha=${HEAD_SHA} 与期望 ${SHA} 不一致，拒绝假证据"
    exit 1
  fi
fi

# ── 2. 轮询 deployment 记录 ───────────────────────────────────────────────
DEADLINE=$(( $(date +%s) + TIMEOUT ))

while :; do
  DEP_ID=$(gh api "repos/$GITHUB_REPOSITORY/deployments?per_page=100" \
           --jq "[.[] | select(.sha==\"$SHA\" and .environment==\"$ENVIRONMENT\")] | (.[0].id // empty)" || true)
  STATE=""
  if [ -n "$DEP_ID" ]; then
    STATE=$(gh api "repos/$GITHUB_REPOSITORY/deployments/$DEP_ID/statuses" \
            --jq 'if length == 0 then "none" else (sort_by(.created_at) | last | .state) end' || true)
  fi

  case "$STATE" in
    success)
      echo "verdict=green" >> "$GITHUB_OUTPUT"
      echo "reason=deployment ${DEP_ID} success (sha=${SHA:0:8}, env=${ENVIRONMENT})" >> "$GITHUB_OUTPUT"
      echo "✅ 门禁通过：deployment ${DEP_ID} ${STATE}"
      exit 0
      ;;
    failure|error)
      echo "verdict=red" >> "$GITHUB_OUTPUT"
      echo "reason=deployment ${DEP_ID} ${STATE}" >> "$GITHUB_OUTPUT"
      echo "::error::commit ${SHA:0:8} 的 ${ENVIRONMENT} 冒烟为 ${STATE}，阻断"
      exit 1
      ;;
  esac

  if [ "$(date +%s)" -ge "$DEADLINE" ]; then
    echo "verdict=red" >> "$GITHUB_OUTPUT"
    echo "reason=timeout waiting for deployment (sha=${SHA:0:8}, env=${ENVIRONMENT}, dep=${DEP_ID:-none}, state=${STATE:-unknown})" >> "$GITHUB_OUTPUT"
    echo "::error::等不到 commit ${SHA:0:8} 的 ${ENVIRONMENT} 冒烟成功（deployment=${DEP_ID:-无记录} 状态=${STATE:-未知}）"
    exit 1
  fi

  echo "${ENVIRONMENT} 冒烟未就绪（deployment=${DEP_ID:-无记录} 状态=${STATE:-未知}），${POLL_INTERVAL}s 后重试…"
  sleep "$POLL_INTERVAL"
done
