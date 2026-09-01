#!/bin/bash
# ============================================================================
# create-pr.sh
# 创建 PR 并关联 Issue（遵循 GIT_WORKFLOW.md + AGENTS.md 规范）
#
# 用法：
#   ./docs/plans/scripts/create-pr.sh <feature-name> <issue-number>
#
# 示例：
#   ./docs/plans/scripts/create-pr.sh job-report 42
#
# 前置条件：
#   - gh CLI 已安装且已登录
#   - 当前在 feat/<feature-name> 分支
#   - 代码已提交并推送
#
# 参考：
#   - docs/GIT_WORKFLOW.md PR 与 Code Review 规则
#   - AGENTS.md 发布流程
# ============================================================================

set -euo pipefail

# ---- 参数校验 ----
FEATURE="${1:-}"
ISSUE_NUMBER="${2:-}"

if [ -z "$FEATURE" ] || [ -z "$ISSUE_NUMBER" ]; then
    echo "❌ 用法: $0 <feature-name> <issue-number>"
    echo "   示例: $0 job-report 42"
    exit 1
fi

BRANCH_NAME="feat/$FEATURE"

# ---- 自动检测主分支名称 ----
echo "🔍 检测主分支..."
if git show-ref --verify --quiet "refs/heads/main"; then
    MAIN_BRANCH="main"
elif git show-ref --verify --quiet "refs/heads/master"; then
    MAIN_BRANCH="master"
else
    echo "❌ 未找到 main 或 master 分支"
    exit 1
fi
echo "📌 主分支: $MAIN_BRANCH"

# ---- 检查当前分支 ----
CURRENT_BRANCH=$(git branch --show-current)
if [ "$CURRENT_BRANCH" != "$BRANCH_NAME" ]; then
    echo "❌ 当前不在 $BRANCH_NAME 分支（当前: $CURRENT_BRANCH）"
    echo "   请先切换到正确的分支: git checkout $BRANCH_NAME"
    exit 1
fi

# ---- 检查是否有未提交的更改 ----
if ! git diff --quiet || ! git diff --cached --quiet; then
    echo "❌ 有未提交的更改，请先提交"
    exit 1
fi

# ---- Rebase 同步主分支（保持线性历史） ----
echo "🔄 同步主分支（rebase）..."
git fetch origin
git rebase "origin/$MAIN_BRANCH"

if [ $? -ne 0 ]; then
    echo "❌ Rebase 失败，请解决冲突后重试"
    echo "   git rebase --abort  // 放弃 rebase"
    exit 1
fi

# ---- 推送分支 ----
LOCAL_COMMIT=$(git rev-parse HEAD)
REMOTE_COMMIT=$(git rev-parse "origin/$BRANCH_NAME" 2>/dev/null || echo "")

if [ "$LOCAL_COMMIT" != "$REMOTE_COMMIT" ]; then
    echo "⚠️  本地有未推送的提交"
    echo "   推送中..."
    git push -u origin "$BRANCH_NAME" --force-with-lease
fi

# ---- 从 issues.md 提取验收标准 ----
PLAN_DIR="docs/plans/$FEATURE"
ACCEPTANCE_CRITERIA=""
if [ -f "$PLAN_DIR/issues.md" ]; then
    ACCEPTANCE_CRITERIA=$(awk '/^\*\*验收标准\*\*/,/^[^*]/' "$PLAN_DIR/issues.md" | head -n -1)
fi

# ---- 从 tasks.md 提取任务完成情况 ----
TASKS_SUMMARY=""
if [ -f "$PLAN_DIR/tasks.md" ]; then
    TOTAL=$(grep -cE '^### Task [0-9]+\.[0-9]+' "$PLAN_DIR/tasks.md" || true)
    TASKS_SUMMARY="共 $TOTAL 个任务"
fi

# ---- 构建 PR body（符合 GIT_WORKFLOW.md 描述模板） ----
cat > /tmp/pr-body.md << EOF
## 关联

Closes #$ISSUE_NUMBER

## 改了什么 / 为什么改

基于 [plan-feature 工作流]($PLAN_DIR) 实现 $FEATURE 功能。

### 规划文档

| 文档 | 说明 |
|---|---|
| [PRD.md]($PLAN_DIR/PRD.md) | 需求定义 |
| [issues.md]($PLAN_DIR/issues.md) | 垂直切片 |
| [tasks.md]($PLAN_DIR/tasks.md) | 任务分解（$TASKS_SUMMARY） |

## 变更文件

\`\`\`
$(git diff --name-only "origin/$MAIN_BRANCH"...HEAD 2>/dev/null || git diff --name-only HEAD~5...HEAD)
\`\`\`

## 测试方式

- [ ] 功能测试通过
- [ ] 代码审查完成
- [ ] CI 全绿

### 验证步骤

\`\`\`bash
# 1. 启动开发服务器
npm run dev

# 2. 测试功能
# {填写具体测试步骤}

# 3. 检查代码
npm run type-check
npm test
\`\`\`

## 风险点 / 影响范围

- {低风险：新功能，不影响现有功能}
- {或：高风险：修改了 XXX 模块，可能影响 YYY}

---

> 由 plan-feature 工作流自动生成
> 基于 docs/GIT_WORKFLOW.md 规范
EOF

# ---- 创建 PR ----
echo "🔨 创建 PR..."
PR_URL=$(gh pr create \
    --base "$MAIN_BRANCH" \
    --head "$BRANCH_NAME" \
    --title "feat($FEATURE): <描述> (closes #$ISSUE_NUMBER)" \
    --body-file /tmp/pr-body.md 2>&1)

echo "✅ PR 已创建: $PR_URL"

# ---- 清理临时文件 ----
rm -f /tmp/pr-body.md

# ---- 输出摘要 ----
echo ""
echo "=========================================="
echo "✅ 完成！"
echo "=========================================="
echo ""
echo "📌 Issue:  #$ISSUE_NUMBER"
echo "🔀 分支:   $BRANCH_NAME"
echo "📄 PR:     $PR_URL"
echo ""
echo "下一步:"
echo "  1. 等待 CI 全绿（分支 push 触发，非 PR 事件）"
echo "  2. 执行代码审查（至少 1 人审批）"
echo "  3. 合并: gh pr merge <pr-number> --squash --delete-branch"
echo ""
echo "⚠️  注意事项（来自 GIT_WORKFLOW.md）："
echo "  - CI 由分支 push 触发，PR 事件不触发 CI/CD"
echo "  - 合并后 master 直接触发 production 部署"
echo "  - 不要用 timeout N 包裹 git/gh 写操作"
echo "  - 合并前确保分支 up-to-date（已 rebase）"
echo ""
