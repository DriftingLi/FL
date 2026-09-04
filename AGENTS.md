# 叉车维修培训与残值评估系统

面向叉车维修培训与叉车残值评估的全栈系统。架构、领域词汇与评审记录见下方文件：

- **领域词汇表**：`CONTEXT.md`（repo 根）
- **架构决策记录（ADRs）**：根仓库 `docs/adr/`（编号 `ADR-0001-…`）；**移动端项目**（`training-app/叉车维修培训学员端跨端应用/docs/adr/`）**另有一套独立 ADR**（`0001-…` 起编号，记录 uni-app-x 侧决策：SSE 流式传输、轻量状态管理、手动 JSON 映射、生物识别、安全存储等）——两套编号体系互不相关，引用时注明「根仓库 / 移动端」
- **AI/agent 工作约定**：`docs/agents/`（根仓库暂无该目录时，现存文件在移动端项目 `docs/agents/`：issue-tracker / triage-labels / domain）

## Agent skills

### Issue tracker

Issues 存放在 GitHub Issues（使用 `gh` CLI）。See `docs/agents/issue-tracker.md`.

### Triage labels

五个 canonical triage roles，label 与 role 同名（`needs-triage` 等）。See `docs/agents/triage-labels.md`.

### Domain docs

Single-context：root `CONTEXT.md` + `docs/adr/`。See `docs/agents/domain.md`.

### Security scan

AI 安全审计用 DeepSec（Shield）。See `docs/agents/security-scan.md`.

## 前端 UI 约定

页面保持整洁：不要写冗余的小标题、装饰性提示与说明性 hint 文本，有的话就清理，仅保留必要的功能性提示。删除 hint 时同步删除对应的 CSS class 与 scoped style，避免残留死代码。

### Tailwind 增量共存四条边界规则

项目已引入 Tailwind CSS v4，与既有 `<style scoped>` 长期共存（详细背景见 `.workbuddy/plans/student-ui-redesign.md`）。共存期间遵守：

| 规则 | 内容 |
|---|---|
| **R1 原子类区** | `src/components/ui/**` 与已纳入改造的页面：模板只用原子类；`<style scoped>` 仅保留伪元素、`:deep()` 改 Element Plus、keyframes、媒体查询 |
| **R2 冻结区** | 未列入当期改造的页面与组件，scoped 样式**一行不动**。改共用件时新特性一律走 prop + 默认值等于现状，让未传值的调用方零 diff |
| **R3 禁双写** | 同一元素同一属性不允许既有 scoped 类又有原子类。需覆盖 Element Plus 外观时二选一：① scoped 内 `:deep()`；② 原子类加 `!` 后缀（`!px-4`） |
| **R4 迁移动作** | 页面改用原子类后，删除 scoped 块中已被替代的规则（沿用上一段「删 hint 同步删 CSS」的约定） |

**变体一律追加覆盖，不改写现有规则**：新增外观分支写成 `.xx.is-dark { … }` 这类多一个类的选择器追加在样式块末尾，特异性天然高于原单类规则，无需 `!important`。

**样式入口只有一个**：`src/assets/styles/tailwind.css`，新增全局样式写进它或它 `@import` 的文件，不要在 `main.ts` 里再加 import。

### 不得触碰的边界

- `--color-brand-*` 属**残值域**专用（`assets/styles/valuation-tokens.css` 在 `.valuation-root` 内定义），**禁止提升为全局变量** —— 会击穿 `layouts/ValuationLayout.vue` 与 `pages/ai-assistant/*` 两处依赖「变量未定义 → 走 fallback」的写法。培训域品牌色用 `--color-primary-*`。
- 残值模块（`pages/student/valuation/**`、`components/valuation/**`）本轮冻结，批量替换色值等机械操作时记得排除。

## 测试与检查流程

改动后**必须**跑完对应栈的检查，全绿才能提交：

- **后端（`backend/`）**：Go 工具链在 `~/go/bin`（`export PATH=/home/root86155/go/bin:$PATH`）
  - `gofmt -l .`（应无输出）
  - `go vet ./...`
  - `golangci-lint run ./...`（errcheck 等静态检查）
  - `go test ./...`
  - 已知例外：`internal/api` 的 `TestStaticOtherResource` 在 WSL 下因 `static/favicon.ico` 权限问题失败，与改动无关，可忽略
- **前端（`frontend/`）**：`cd frontend`
  - `npm run type-check`（vue-tsc）
  - `npm test`（vitest）
- **部署配置**：改 `docker-compose*.yml` / `deploy.sh` 后可用 `docker compose -f docker-compose.prod.yml config -q` 做语法校验
- **安全检测**：改动触及认证/授权/密钥/DB 连接/AI 生成代码时，跑 `python -m deepsec shield scan backend frontend/src`，确认无新增 critical/high（已知误报见 `docs/agents/security-scan.md`）。

## 发布流程（push / PR / merge）

master 有仓库 ruleset「protect master」保护（直接 push 会被拒，`push declined due to repository rule violations`），且限定 squash 合并。发布必须走分支 + PR：

1. **本地提交**（只 add 本次改动的文件，勿 `git add -A`）。
2. **建分支推送**：若提交已在本地 master 上，`git branch feat/xxx` 后 `git reset --hard origin/master` 还原本地 master；然后 `git push -u origin feat/xxx`。
3. **分支 push = CI + testing 冒烟**：push 触发 `ci.yml` 全量 CI（不再同时跑 `pull_request`，一次改动只跑一遍）。CI 全绿**且该分支已有开启的 PR** 时，`ci-summary` 用 `workflow_dispatch --ref <分支>` 派发 `cd.yml` 部署 testing（起栈 → 健康检查 → 立即 down）。孤儿分支/无 PR 分支不占 testing。
4. **创建 PR**：`gh pr create --base master --head feat/xxx --title "..." --body "..."`。**PR 事件不触发任何 CI/CD**；PR 页上的 `ci-summary` 等检查来自第 3 步分支 push 的同 commit run。若 CI 在开 PR 之前就已经跑完（纯文档改动约 20 秒），该 commit 会缺冒烟记录，`testing-smoke.yml` 会在 PR 开启时补发一次（仅此一种条件下动作，不重跑 CI）。
5. **等门禁**：`gh run watch <id> --exit-status` 等 CI 全绿（纯前端改动时 backend-* / migration-check 跳过属正常，`ci-summary` 仍会 success）。ruleset 把 `ci-summary` 设为必检并要求分支 up-to-date —— 未跑绿、或 master 已前进时不能合并（后者要 `git merge origin/master` 同步后重推、重跑 CI）。
6. **Squash merge → 直发 production**：`gh pr merge <n> --squash --delete-branch`。master 的 push **不跑 CI**，直接触发 `cd.yml` 的 `gate` job：从 commit 主题解析 `(#N)` → 校验该 PR head 的 `ci-summary=success`、该 commit 的 testing 冒烟 `success`（冒烟可能晚于合并，gate 最多轮询 15 分钟）→ 通过后才构建镜像并部署 production。若报 "requirements have not been met"，用 `gh pr view <n> --json statusCheckRollup` 排查。
7. **收尾**：`git fetch --prune` → `git checkout master && git pull --ff-only` → 删除本地 feat 分支（若 gh 已自动删）。

**应急通道**：gate 阻断但确认可以上生产时，手动放行 `gh workflow run cd.yml -f environment=production -f ref=<master sha>`（dispatch 不经门禁）。若是 testing 冒烟失败，先到 Actions 重跑该 CD run，再重新合并或走应急通道。

> ⚠️ 本次流水线变更**合并前**切出的分支：其 `ci-summary` 受旧条件约束（仅 master 上报），在 PR 上显示 skipped，既不满足必检也会被 gate 判为 `other`。先 `git merge origin/master` 重推、等 CI 重跑，再走合并流程。

> ⚠️ **不要用 `timeout N` 包裹 git/gh 的写操作**（merge / push / rebase / checkout）。被 SIGTERM 杀掉的是**执行到一半**的操作，比失败更糟：曾因 `timeout 180 gh pr merge --squash --delete-branch` 被中断，残留 `.git/index.lock` 且分支清理删了一半，`frontend/src` 下 265 个文件被删。这类操作一律用后台任务跑并等其自然结束。

## 多 Agent 并发与 git 隔离

多个 agent 会话（含 aider / 自动化监控）若**共用同一份 checkout 操作 git，会互相踩踏**：曾发生并发会话擅自 `checkout master` 并清掉分支，把另一会话刚 `commit`、尚未 `push` 的提交甩成无引用游离状态；`add`/`commit`/`push` 也会抢 `.git/index.lock`。遵守：

- **一会话一 worktree 一分支**：动代码前先 `git worktree add ../wt-<task> -b feat/<task> origin/master`，全程在 `../wt-<task>` 内改、提交、push、开 PR；HEAD 与 index 天然隔离，避免并发 checkout/reset 互踩。
- **提交前验分支归属**：`git branch --show-current` + `git log --oneline origin/master..HEAD`，确认 HEAD 与待推提交属本会话任务，不在别的会话占用的分支上落 commit。
- **发现游离提交先取证、勿覆盖**：工作区被并发切走、自己的 commit 在 `git branch` 里消失时，用 `git log --oneline <sha>` + `git diff <sha> origin/master -- <paths>` 只读确认内容是否已在 master；已在则不 push、不 `reset`，交 gc 自然回收，绝不 `cherry-pick` 造冲突。
- **绝不 `git add -A`**：共享工作区常混有其他会话/工具的未提交改动（如 `forum.uts`、`.aider-desk/`、`.monitor/`），只 `git add <本次文件>`。
