#!/usr/bin/env bash
# checkDeployment：轮询 deployment 记录 + status，直到 success 或超时。
# 拥有轮询语义（间隔/超时），调用方不需要自己写循环。
set -euo pipefail

SHA="${INPUT_SHA:?INPUT_SHA is required}"
ENVIRONMENT="${INPUT_ENVIRONMENT:-testing}"
TIMEOUT="${INPUT_TIMEOUT:-480}"
POLL_INTERVAL="${INPUT_POLL_INTERVAL:-30}"

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
      echo "deployment_id=${DEP_ID}" >> "$GITHUB_OUTPUT"
      echo "deployment_status=${STATE}" >> "$GITHUB_OUTPUT"
      echo "✅ deployment ${DEP_ID} ${STATE} (sha=${SHA:0:8}, env=${ENVIRONMENT})"
      exit 0
      ;;
    failure|error)
      echo "::error::commit ${SHA:0:8} 的 ${ENVIRONMENT} 冒烟为 ${STATE}，阻断"
      echo "deployment_id=${DEP_ID}" >> "$GITHUB_OUTPUT"
      echo "deployment_status=${STATE}" >> "$GITHUB_OUTPUT"
      exit 1
      ;;
  esac

  # TIMEOUT=0 表示单次快查：只检查记录存在（#2120 语义：record exists = green）
  if [ "$TIMEOUT" -eq 0 ]; then
    if [ -n "$DEP_ID" ]; then
      echo "deployment_id=${DEP_ID}" >> "$GITHUB_OUTPUT"
      echo "deployment_status=${STATE:-none}" >> "$GITHUB_OUTPUT"
      echo "✅ deployment ${DEP_ID} 记录存在 (sha=${SHA:0:8}, env=${ENVIRONMENT})"
      exit 0
    fi
    echo "::error::commit ${SHA:0:8} 的 ${ENVIRONMENT} 无部署记录"
    echo "deployment_id=" >> "$GITHUB_OUTPUT"
    echo "deployment_status=none" >> "$GITHUB_OUTPUT"
    exit 1
  fi

  if [ "$(date +%s)" -ge "$DEADLINE" ]; then
    echo "::error::等不到 commit ${SHA:0:8} 的 ${ENVIRONMENT} 冒烟成功（deployment=${DEP_ID:-无记录} 状态=${STATE:-未知}）"
    echo "deployment_id=${DEP_ID:-}" >> "$GITHUB_OUTPUT"
    echo "deployment_status=${STATE:-unknown}" >> "$GITHUB_OUTPUT"
    exit 1
  fi

  echo "${ENVIRONMENT} 冒烟未就绪（deployment=${DEP_ID:-无记录} 状态=${STATE:-未知}），${POLL_INTERVAL}s 后重试…"
  sleep "$POLL_INTERVAL"
done
