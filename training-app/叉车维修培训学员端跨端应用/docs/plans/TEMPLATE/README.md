# 功能规划模板

用于规范化新功能的规划流程，确保「用写作思考，而不是用代码」。

## 使用步骤

```bash
# 1. 复制模板
cp -r docs/plans/TEMPLATE docs/plans/{feature-name}

# 2. 填写 plan.md（自由形式计划）
vim docs/plans/{feature-name}/plan.md

# 3. 在 AI 对话中生成 PRD
# 输入：基于 docs/plans/{feature-name}/plan.md 生成 PRD

# 4. 在 AI 对话中生成 issues
# 输入：基于 docs/plans/{feature-name}/PRD.md 生成垂直切片 issues

# 5. 在 AI 对话中生成 tasks
# 输入：基于 docs/plans/{feature-name}/issues.md 生成任务分解

# 6. 创建 Issue + 分支
./docs/plans/scripts/create-issue.sh {feature-name}

# 7. 执行任务（每个任务一条）
# 在 AI 对话中：execute task 1.1

# 8. 提交代码（遵循 Conventional Commits）
git add <files>
git commit -m 'feat({feature-name}): <描述>'

# 9. 同步主分支（保持线性历史）
git fetch origin
git rebase origin/master

# 10. 推送分支
git push -u origin feat/{feature-name}

# 11. 创建 PR
./docs/plans/scripts/create-pr.sh {feature-name} <issue-number>

# 12. 代码审查
# 在 AI 对话中：review-code

# 13. 合并
gh pr merge <pr-number> --squash --delete-branch
```

## 文件说明

| 文件 | 说明 | 填写者 |
|---|---|---|
| `plan.md` | 自由形式计划 | 人类 |
| `PRD.md` | 结构化 PRD | AI（基于 plan.md） |
| `issues.md` | 垂直切片 issues | AI（基于 PRD.md） |
| `tasks.md` | 任务分解 | AI（基于 issues.md） |

## 命名约定

- 目录名：kebab-case（如 `job-report`、`ui-spec`）
- 文件名：固定为 `plan.md` / `PRD.md` / `issues.md` / `tasks.md`

## 工作流原则

1. **先思考，后编码**：plan.md 是思考材料，不是交付物
2. **垂直切片**：每个 issue 是端到端路径，不是水平层
3. **单任务单会话**：每个 task = 一次 AI 会话
4. **审查权在人类**：AI 加速生产，审查永远是你的工作

## 规范遵循

本工作流遵循以下规范：

| 规范 | 文件 | 要求 |
|---|---|---|
| Git 工作流 | `docs/GIT_WORKFLOW.md` | 分支命名、Conventional Commits、Rebase 同步 |
| 发布流程 | `AGENTS.md` | Squash merge、CI 触发、生产部署 |
| 提交规范 | `commitlint.config.js` | `<type>(<scope>): <subject>` 格式 |

### Conventional Commits 格式

```
feat(exam): 新增答题计时器
fix(auth): 修复 token 过期后未跳登录页
refactor(chapter): 重构学习进度上报逻辑
```

| type | 说明 |
|---|---|
| feat | 新功能 |
| fix | 缺陷修复 |
| refactor | 重构（不改行为） |
| style | 样式调整 |
| docs | 文档更新 |
| test | 测试相关 |
| chore | 构建/依赖/配置 |
