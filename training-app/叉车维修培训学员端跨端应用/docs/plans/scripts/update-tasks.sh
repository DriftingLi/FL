#!/bin/bash
# ============================================================================
# update-tasks.sh
# 更新 GitHub Issue 中的 task checklist 状态
#
# 用法：
#   ./docs/plans/scripts/update-tasks.sh <issue-number> <task-pattern> [done|undone]
#
# 示例：
#   ./docs/plans/scripts/update-tasks.sh 42 "Task 1.1" done
#   ./docs/plans/scripts/update-tasks.sh 42 "Task 1" done
#   ./docs/plans/scripts/update-tasks.sh 42 "Task 1.1" undone
#
# 参数：
#   issue-number  - GitHub Issue 编号
#   task-pattern  - 要匹配的任务模式（如 "Task 1.1" 或 "Task 1"）
#   task-status   - done（默认）| undone
# ============================================================================

set -euo pipefail

# ---- 参数校验 ----
ISSUE_NUMBER="${1:-}"
TASK_PATTERN="${2:-}"
TASK_STATUS="${3:-done}"

if [ -z "$ISSUE_NUMBER" ] || [ -z "$TASK_PATTERN" ]; then
    echo "❌ 用法: $0 <issue-number> <task-pattern> [done|undone]"
    echo "   示例: $0 42 'Task 1.1' done"
    exit 1
fi

if [ "$TASK_STATUS" != "done" ] && [ "$TASK_STATUS" != "undone" ]; then
    echo "❌ task-status 必须是 done 或 undone"
    exit 1
fi

# ---- 读取当前 Issue body ----
echo "📋 读取 Issue #$ISSUE_NUMBER..."
ISSUE_BODY=$(gh issue view "$ISSUE_NUMBER" --json body -q '.body')

if [ -z "$ISSUE_BODY" ]; then
    echo "❌ 无法读取 Issue body"
    exit 1
fi

# ---- 更新 checklist ----
echo "🔄 更新任务状态: $TASK_PATTERN → $TASK_STATUS"

# 根据 task-status 替换 [ ] 或 [x]
if [ "$TASK_STATUS" = "done" ]; then
    # 将匹配的 - [ ] 替换为 - [x]
    UPDATED_BODY=$(echo "$ISSUE_BODY" | sed "s/- \[ \] $TASK_PATTERN/- [x] $TASK_PATTERN/")
else
    # 将匹配的 - [x] 替换为 - [ ]
    UPDATED_BODY=$(echo "$ISSUE_BODY" | sed "s/- \[x\] $TASK_PATTERN/- [ ] $TASK_PATTERN/")
fi

# ---- 更新 Issue ----
echo "📝 更新 Issue #$ISSUE_NUMBER..."
gh issue edit "$ISSUE_NUMBER" --body "$UPDATED_BODY"

echo "✅ 任务状态已更新"

# ---- 统计完成度 ----
TOTAL=$(echo "$UPDATED_BODY" | grep -cE '^\- \[[ x]\] Task [0-9]+\.[0-9]+' || true)
DONE=$(echo "$UPDATED_BODY" | grep -cE '^\- \[x\] Task [0-9]+\.[0-9]+' || true)

echo ""
echo "📊 进度: $DONE / $TOTAL 任务完成"
if [ "$TOTAL" -gt 0 ]; then
    echo "   完成率: $(( DONE * 100 / TOTAL ))%"
fi
