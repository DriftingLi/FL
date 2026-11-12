#!/usr/bin/env bash
# waitForRun：轮询一个 GitHub Actions run 直到完成，校验 headSha 与期望 SHA 一致。
# 拥有轮询语义（间隔/超时/重试），调用方不需要自己写循环。
set -euo pipefail

RUN_ID="${INPUT_RUN_ID:?INPUT_RUN_ID is required}"
EXPECTED_SHA="${INPUT_SHA:?INPUT_SHA is required}"
TIMEOUT="${INPUT_TIMEOUT:-1800}"
POLL_INTERVAL="${INPUT_POLL_INTERVAL:-30}"

DEADLINE=$(( $(date +%s) + TIMEOUT ))

while :; do
  RESULT=$(gh run view "$RUN_ID" --repo "$GITHUB_REPOSITORY" \
           --json status,conclusion,headSha \
           --jq '"\(.status)/\(.conclusion // "-")/\(.headSha)"' || echo 'unknown/-/unknown')

  STATUS=$(echo "$RESULT" | cut -d/ -f1)
  CONCLUSION=$(echo "$RESULT" | cut -d/ -f2)
  HEAD_SHA=$(echo "$RESULT" | cut -d/ -f3)

  # SHA 校验：run 的 headSha 必须与期望一致，否则拒绝
  if [ "$HEAD_SHA" != "$EXPECTED_SHA" ] && [ "$HEAD_SHA" != "unknown" ]; then
    echo "::error::run ${RUN_ID} head_sha=${HEAD_SHA} 与期望 ${EXPECTED_SHA} 不一致（SHA 错位），拒绝生成假证据"
    echo "verdict=red" >> "$GITHUB_OUTPUT"
    echo "reason=SHA mismatch: ${HEAD_SHA} != ${EXPECTED_SHA}" >> "$GITHUB_OUTPUT"
    exit 1
  fi

  if [ "$STATUS" = "completed" ]; then
    echo "conclusion=${CONCLUSION}" >> "$GITHUB_OUTPUT"
    echo "head_sha=${HEAD_SHA}" >> "$GITHUB_OUTPUT"
    if [ "$CONCLUSION" = "success" ]; then
      echo "✅ run ${RUN_ID} completed/success (sha=${HEAD_SHA:0:8})"
      exit 0
    fi
    echo "::error::run ${RUN_ID} completed/${CONCLUSION}（不是 success）⇒ 不放行"
    exit 1
  fi

  if [ "$(date +%s)" -ge "$DEADLINE" ]; then
    echo "::error::等 run ${RUN_ID} 超过 ${TIMEOUT}s 仍未完成（当前 ${STATUS}/${CONCLUSION}）⇒ fail-closed 不绿灯"
    echo "verdict=red" >> "$GITHUB_OUTPUT"
    echo "reason=timeout waiting for run ${RUN_ID}" >> "$GITHUB_OUTPUT"
    exit 1
  fi

  echo "run ${RUN_ID} ${STATUS}/${CONCLUSION}，${POLL_INTERVAL}s 后重试…"
  sleep "$POLL_INTERVAL"
done
