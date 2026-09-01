#!/bin/bash
# ============================================================================
# create-issue.sh
# 从 docs/plans/{feature-name}/ 自动生成 GitHub Issue + Git 分支
#
# 用法：
#   ./docs/plans/scripts/create-issue.sh <feature-name>
#
# 示例：
#   ./docs/plans/scripts/create-issue.sh job-report
#
# 前置条件：
#   - gh CLI 已安装且已登录
#   - 当前在项目根目录
#   - docs/plans/{feature-name}/ 下有 plan.md、PRD.md、issues.md、tasks.md
#
# 参考：
#   - AGENTS.md 发布流程
#   - docs/GIT_WORKFLOW.md 分支命名规范
# ============================================================================

set -euo pipefail

# ---- 参数校验 ----
FEATURE="${1:-}"
if [ -z "$FEATURE" ]; then
    echo "❌ 用法: $0 <feature-name>"
    echo "   示例: $0 job-report"
    exit 1
fi

PLAN_DIR="docs/plans/$FEATURE"
if [ ! -d "$PLAN_DIR" ]; then
    echo "❌ 目录不存在: $PLAN_DIR"
    exit 1
fi

# ---- 检查必要文件 ----
for f in plan.md PRD.md issues.md tasks.md; do
    if [ ! -f "$PLAN_DIR/$f" ]; then
        echo "❌ 缺少文件: $PLAN_DIR/$f"
        exit 1
    fi
done

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

# ---- 从 tasks.md 提取任务列表，生成 checklist ----
echo "📋 解析 tasks.md..."

# 提取所有 ### Task N.M: 开头的行，生成 - [ ] 格式
TASKS_CHECKLIST=$(grep -E '^### Task [0-9]+\.[0-9]+' "$PLAN_DIR/tasks.md" | \
    sed 's/^### //' | \
    sed 's/^/- [ ] /')

if [ -z "$TASKS_CHECKLIST" ]; then
    echo "⚠️  未找到任务，跳过 checklist 生成"
    TASKS_CHECKLIST="- [ ] 无任务"
fi

# ---- 从 PRD.md 提取问题陈述 ----
PROBLEM_STATEMENT=$(awk '/^## 问题陈述/,/^## /' "$PLAN_DIR/PRD.md" | head -n -1 | tail -n +2)

# ---- 构建 Issue body（符合 GIT_WORKFLOW.md PR 描述模板） ----
cat > /tmp/issue-body.md << EOF
## $FEATURE

### 📋 规划文档

| 文档 | 说明 |
|---|---|
| [plan.md]($PLAN_DIR/plan.md) | 自由形式计划 |
| [PRD.md]($PLAN_DIR/PRD.md) | 结构化 PRD |
| [issues.md]($PLAN_DIR/issues.md) | 垂直切片 issues |
| [tasks.md]($PLAN_DIR/tasks.md) | 任务分解 |

### 🎯 问题陈述

$PROBLEM_STATEMENT

### ✅ Tasks

$TASKS_CHECKLIST

### 📝 验证方式

见 [issues.md]( $PLAN_DIR/issues.md ) 中各 issue 的「如何验证」部分。

### 🔗 关联

- 分支: \`feat/$FEATURE\`
- PR: (创建后填写)
EOF

# ---- 创建 GitHub Issue ----
echo "🔨 创建 GitHub Issue..."
ISSUE_URL=$(gh issue create \
    --title "feat: $FEATURE" \
    --body-file /tmp/issue-body.md \
    --label "enhancement" 2>&1)

echo "✅ Issue 已创建: $ISSUE_URL"

# ---- 提取 Issue 编号 ----
ISSUE_NUMBER=$(echo "$ISSUE_URL" | grep -oE '[0-9]+$')
echo "📌 Issue 编号: #$ISSUE_NUMBER"

# ---- 创建 Git 分支（从最新主分支） ----
BRANCH_NAME="feat/$FEATURE"
echo "🔀 创建分支: $BRANCH_NAME"

# 检查分支是否已存在
if git show-ref --verify --quiet "refs/heads/$BRANCH_NAME"; then
    echo "⚠️  分支已存在: $BRANCH_NAME"
    echo "   切换到已有分支..."
    git checkout "$BRANCH_NAME"
else
    # 确保在主分支上
    CURRENT_BRANCH=$(git branch --show-current)
    if [ "$CURRENT_BRANCH" != "$MAIN_BRANCH" ]; then
        echo "⚠️  当前不在 $MAIN_BRANCH 分支（当前: $CURRENT_BRANCH）"
        echo "   切换到 $MAIN_BRANCH..."
        git checkout "$MAIN_BRANCH"
        git pull --ff-only
    fi

    # 从最新主分支创建并切换到新分支
    git fetch origin
    git checkout -b "$BRANCH_NAME" "origin/$MAIN_BRANCH"
    echo "✅ 分支已创建: $BRANCH_NAME（基于最新 origin/$MAIN_BRANCH）"
fi

# ---- 清理临时文件 ----
rm -f /tmp/issue-body.md

# ---- 输出摘要 ----
echo ""
echo "=========================================="
echo "✅ 完成！"
echo "=========================================="
echo ""
echo "📌 Issue:  $ISSUE_URL"
echo "🔀 分支:   $BRANCH_NAME"
echo ""
echo "下一步:"
echo "  1. 执行任务: 逐个完成 tasks.md 中的任务"
echo "  2. 提交代码（遵循 Conventional Commits）:"
echo "     git add <files>"
echo "     git commit -m 'feat($FEATURE): <描述>'"
echo ""
echo "  ⚠️  Commit 格式: <type>(<scope>): <subject>"
echo "     type: feat | fix | refactor | style | docs | test | chore"
echo "     scope: 可选，模块名"
echo "     subject: 祈使句、不加句号、≤ 50 字"
echo ""
echo "  3. 同步主分支（保持线性历史）:"
echo "     git fetch origin"
echo "     git rebase origin/$MAIN_BRANCH"
echo ""
echo "  4. 推送分支:"
echo "     git push -u origin $BRANCH_NAME"
echo ""
echo "  5. 创建 PR:"
echo "     ./docs/plans/scripts/create-pr.sh $FEATURE $ISSUE_NUMBER"
echo ""
echo "⚠️  注意事项（来自 GIT_WORKFLOW.md）："
echo "  - 只 add 本次改动的文件，勿 git add -A"
echo "  - 不要用 timeout N 包裹 git/gh 写操作"
echo "  - 一个分支只做一件事，生命周期 ≤ 2~3 天"
echo "  - 分支 push 触发 CI，PR 事件不触发 CI/CD"
echo ""
