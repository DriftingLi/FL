# 工作流优化交付总览

> 生成日期：2026-07-25 ｜ 角色：Git Workflow Master
> 协作决策：**共享仓库+分支保护 / Trunk-Based / Rebase+Squash / 先出规范文档**

## 做了什么
针对 FL 仓库（叉车维修培训学员端跨端应用）Git 流程混乱的问题，输出了一套可落地的工作流规范与配套模板：

- **根因诊断**：个人名分支 `zhengcookie` 当工作分支、同步合并污染历史、3 条重复 fix 提交、提交规范不统一、无 CI / 无分支保护、双 remote 拓扑模糊、贡献集中于一人。
- **目标流程**：基于 `main` 的 Trunk-Based 精简流，短期 `feat/fix` 分支 + Squash 合并，rebase 保持线性，`main` 永远可发布。
- **配套资产**：分支命名表、提交规范、PR/Review 规则、分支保护点击路径、发布与救火手册、即时整改清单。

## 产出文件
| 文件 | 用途 |
|------|------|
| `docs/GIT_WORKFLOW.md` | 主规范（唯一事实来源） |
| `docs/GIT_CHEATSHEET.md` | 命令速查卡 |
| `.github/PULL_REQUEST_TEMPLATE.md` | PR 模板 |
| `.github/ISSUE_TEMPLATE/bug_report.md` | Bug 模板 |
| `.github/ISSUE_TEMPLATE/feature_request.md` | 功能模板 |
| `commitlint.config.js` | 提交校验（就绪，待接入 husky/CI） |

## 你的下一步
1. 仓库 Owner 按 `docs/GIT_WORKFLOW.md` 第 6 节开启 `main` 分支保护。
2. 把当前 `zhengcookie` 上的未提交改动迁移到规范 `feat/xxx` 分支（见下方即时剧本）。
3. 后续接入 CI 时启用 `commitlint.config.js`。

## 即时整改剧本（zhengcookie 现状，执行前请确认）
```bash
# 1) 暂存当前改动到规范分支
git stash                                   # 先收纳未提交改动
git fetch origin
git checkout -b feat/auth-and-pages origin/main
git stash pop                               # 恢复到新分支
# 2) 原子化提交（不要一把梭）
git add api/auth.uts types/index.uts utils/system.uts
git commit -m "feat(auth): 重构登录鉴权与类型定义"
git add pages/courses/courses.uvue pages/dashboard/dashboard.uvue \
        pages/exam/exam.uvue pages/profile/profile.uvue pages/register/register.uvue
git commit -m "feat(pages): 统一各页面布局与交互"
# 3) 推送并提 PR（Squash 合并）
git push -u origin feat/auth-and-pages
# 4) PR 合并后清理
git checkout main && git pull --ff-only origin main
git branch -d feat/auth-and-pages
git push origin --delete feat/auth-and-pages
git branch -d zhengcookie                   # 归档删除个人分支
```
