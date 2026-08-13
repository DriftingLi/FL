# Git 工作流规范（FL 叉车维修培训学员端）

> 适用范围：FL 仓库全体前端（training-app / uni-app）与后端（Go）协作。
> 目标：主干始终可发布、提交原子可追溯、PR 易审易回滚、新人 10 分钟上手。
> 生效日期：2026-07-25 ｜ 维护人：团队约定，全员遵守

---

## 0. 一句话总纲

**基于 `main` 主干的 Trunk-Based 精简流**：所有人从最新 `main` 拉短期功能分支，本地用 `rebase` 保持线性，PR 用 **Squash Merge** 合成一个 conventional commit 进 `main`，合并即删分支。`main` 永远处于可发布状态。

```
main  ──●────●────●────●────●────●───  (永远可发布)
         \  /    \  /    \  /
          ●        ●      ●            (feat/fix 短期分支，最长 2~3 天)
```

---

## 1. 分支命名规范

| 类型 | 前缀 | 示例 | 说明 |
|------|------|------|------|
| 新功能 | `feat/` | `feat/exam-timer` | 用户可见的新能力 |
| 缺陷修复 | `fix/` | `fix/login-redirect` | 修 bug |
| 重构 | `refactor/` | `refactor/chapter-progress` | 不改行为，只改结构 |
| 样式 | `style/` | `style/course-card` | 纯格式/UI 微调，无逻辑 |
| 文档 | `docs/` | `docs/readme-api` | 注释/文档 |
| 测试 | `test/` | `test/exam-flow` | 增删测试 |
| 构建/依赖 | `chore/` / `build/` | `chore/deps-update` | 构建脚本、依赖、配置 |
| CI | `ci/` | `ci/add-lint` | CI 流程改动 |
| 性能 | `perf/` | `perf/list-render` | 性能优化 |
| 发布 | `release/` | `release/v1.2.0` | 打 tag 前的发布准备 |

**硬性规则**
- ❌ 禁止使用个人名分支（`zhengcookie`、`luohao` 之类）——这是当前仓库最大病灶。
- ✅ 分支名全小写、连字符分隔：`feat/user-auth`，不要中文、不要空格。
- ✅ 一个分支只做一件事；跨主题请拆分支。
- ✅ 功能分支生命周期 ≤ 2~3 天；超过就拆小或 rebase 同步。

---

## 2. 分支生命周期（标准动作）

```bash
# ① 开工前，确保本地 main 是最新的
git fetch origin
git checkout main
git pull --ff-only origin main

# ② 拉功能分支（从最新 main 起）
git checkout -b feat/exam-timer origin/main

# ③ 开发……本地多次原子提交
git add <具体文件>
git commit -m "feat(exam): 新增答题计时器"

# ④ 推送并提 PR
git push -u origin feat/exam-timer
# 然后去 GitHub 提 PR（目标分支选 main，合并方式选 Squash）

# ⑤ 合并后清理
git checkout main
git pull --ff-only origin main
git branch -d feat/exam-timer          # 删本地
git push origin --delete feat/exam-timer  # 删远程
```

> 多任务并行？用 `git worktree` 开独立工作区，互不污染：
> `git worktree add ../fl-hotfix fix/login-redirect`

---

## 3. 提交规范（Conventional Commits）

格式：`<type>(<scope>): <subject>`

```
feat(exam): 新增答题计时与自动交卷
fix(auth): 修复 token 过期后未跳登录页
refactor(chapter): 重构学习进度上报逻辑
```

| 字段 | 规则 |
|------|------|
| `type` | 见上表，必须小写 |
| `scope` | 可选，模块名：`auth` / `exam` / `chapter` / `course` … |
| `subject` | 祈使句、不加句号、≤ 50 字、说明"为什么"而非"改了啥" |

**反例（禁止）**
```
❌ chore: 代码清洗
❌ chore: 代码整理
❌ fix: 改了个 bug
❌ 修复（无 type / 无 scope）
❌ update
```

**原子性铁律**
- 一个 commit 只做一件事，可独立 revert。
- 不要"先随便提、后面补"——用 `git commit --amend` / `rebase -i` 在推送前整理好。
- 同一问题不要出现 3 条近似提交（当前 `fix: 忽略缓存失效操作的返回错误` ×3 就是教训）。

---

## 4. 同步策略：禁止"同步合并"

**不要**这样制造 merge 噪音：
```
git merge origin/main     # ❌ 产生 "Merge branch 'origin/main'"
```

**要**这样保持线性：
```bash
git fetch origin
git rebase origin/main    # ✅ 把你的提交"重放"到最新 main 之上
```

遇到冲突：rebase 会逐个提交暂停，解决后 `git add <file>` → `git rebase --continue`。
完成后若已推过远程，用安全强推：
```bash
git push --force-with-lease origin feat/exam-timer
```
> ⚠️ `--force-with-lease` 比 `--force` 安全：若别人在你之后推过，会被拒绝而非覆盖。
> ⚠️ **永远不要**对 `main` 强推。

---

## 5. PR 与 Code Review 规则

| 项目 | 要求 |
|------|------|
| 目标分支 | `main` |
| 合并方式 | **Squash and merge**（多个提交压成 1 个） |
| 最小审批 | ≥ 1 人（luohao / DriftingLi / 指定 reviewer） |
| 关联 Issue | PR 描述里写 `Closes #123` |
| CI 状态 | 待接入 CI 后，必须全绿才允许合并 |
| 自审 | 合并前自己 rebase 最新 main、本地跑通构建 |

PR 描述用模板（见 `.github/PULL_REQUEST_TEMPLATE.md`），至少包含：
- 改了什么 / 为什么改
- 测试方式（怎么验证）
- 风险点 / 影响范围

---

## 6. 分支保护设置（GitHub 后台）

当前仓库**没有任何保护规则**，请仓库 Owner（DriftingLi）按以下路径开启：

`仓库 Settings → Branches → Branch protection rules → Add rule`

- Branch name pattern：`main`
- ✅ Require a pull request before merging
  - ✅ Required approvals：`1`
  - ✅ Dismiss stale approvals when new commits are pushed
- ✅ Require status checks to pass（CI 接入后勾选对应 check）
- ✅ Require branches to be up to date before merging
- ✅ Do not allow bypassing the above settings
- ❌ 不要勾 "Allow force pushes"（main 禁强推）

> 这样可彻底杜绝"直接 push 乱提交"的历史问题。

---

## 7. 发布流程

- 版本号遵循 `vMAJOR.MINOR.PATCH`（如 `v1.2.0`）。
- 发布前从 `main` 拉 `release/vX.Y.Z` 做最后校验，校验完在 `main` 上打 tag：
  ```bash
  git tag -a v1.2.0 -m "release: v1.2.0 学员端跨端应用"
  git push origin v1.2.0
  ```
- 紧急线上 bug：从 `main`/对应 tag 拉 `fix/xxx`，走正常 PR，合并后补打 patch tag。

---

## 8. 救火手册（常见翻车恢复）

> 所有恢复前先 `git status` + `git log --oneline -5` 看清楚现状。

| 场景 | 安全做法 |
|------|----------|
| 误 commit，想改信息 | `git commit --amend`（未推送前） |
| 想合并最近几次提交 | `git rebase -i HEAD~3` → 把后两条 `pick` 改 `squash` |
| rebase 搞砸了 | `git rebase --abort` 立即退出 |
| 误 `reset --hard` | `git reflog` 找到丢失的 HEAD，`git reset --hard <sha>` 找回 |
| 推错分支/想撤回远程提交 | `git push --force-with-lease`（仅限自己的功能分支） |
| 冲突卡住 | 解决后 `git add` → `git rebase --continue`；放弃则 `git rebase --abort` |

---

## 9. 当前仓库整改清单（针对现状）

基于 2026-07-25 实况，建议按顺序处理：

1. **收敛双 remote**：小团队改用共享仓库，统一只保留 `origin`（DriftingLi/FL）；`myfork` 可保留为个人备份但不作为协作入口。
2. **迁移 `zhengcookie` 个人分支**：把当前未提交改动提交到一个规范的 `feat/xxx` 分支（见下方即时剧本），原 `zhengcookie` 分支归档删除。
3. **清理重复提交**：`fix: 忽略缓存失效操作的返回错误` ×3 在历史里 squash 成 1 条（仅限未合并的本地/个人分支，已合入 main 的不动）。
4. **补分支保护**：按第 6 节开启 main 保护。
5. **后续接入 CI**：uni-app 构建检查 + commitlint 校验 + PR 模板（配置见 `commitlint.config.js`，待启用）。

---

## 10. 新人 5 分钟上手

1. `git clone <origin>` → `git checkout main` → `git pull --ff-only`
2. `git checkout -b feat/我的任务 origin/main`
3. 写代码 → 原子提交（conventional commits）
4. `git fetch origin && git rebase origin/main` 保持线性
5. `git push -u origin feat/我的任务` → GitHub 提 PR（Squash）→ 等审批 → 合并 → 删分支

---

_配套文件：`docs/GIT_CHEATSHEET.md`（命令速查）、`.github/PULL_REQUEST_TEMPLATE.md`（PR 模板）、`commitlint.config.js`（提交校验，待接入）。_
