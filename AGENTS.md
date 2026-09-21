# 叉车维修培训与残值评估系统

面向叉车维修培训与叉车残值评估的全栈系统。**本文件只做导航**——工作约定按块拆在 `docs/agents/` 下，动手前先读对应块。

## 领域文件

- **领域词汇表**：`CONTEXT.md`（repo 根）

- **架构评审范围**：architecture review 覆盖 Web 前端（`frontend/`）、后端（`backend/`）与部署面；**移动端**（`training-app/叉车维修培训学员端跨端应用/`）由 @zhengcookie 负责，其深化机会不在本仓评审范围——评审不重复提议移动端候选，移动端自身决策见其独立 ADR 体系。

- **架构决策记录（ADRs）**：根仓库 `docs/adr/`（编号 `ADR-0001-…`）；**移动端项目**（`training-app/叉车维修培训学员端跨端应用/docs/adr/`）**另有一套独立 ADR**（`0001-…` 起编号，记录 uni-app-x 侧决策：SSE 流式传输、轻量状态管理、手动 JSON 映射、生物识别、安全存储等）——两套编号体系互不相关，引用时注明「根仓库 / 移动端」

## Agent skills

### Issue tracker

Issues 存放在 GitHub Issues（使用 `gh` CLI）。See `docs/agents/issue-tracker.md`.

### Triage labels

五个 canonical triage roles，label 与 role 同名（`needs-triage` 等）。See `docs/agents/triage-labels.md`.

### Domain docs

Single-context：root `CONTEXT.md` + `docs/adr/`。See `docs/agents/domain.md`.

### Security scan

AI 安全审计用 DeepSec（Shield）。See `docs/agents/security-scan.md`.

## 工作约定（按块导航）

| 文件 | 内容 | 什么时候读 |
| --- | --- | --- |
| [`docs/agents/ui-conventions.md`](docs/agents/ui-conventions.md) | 前端 UI 约定：UI 词汇（封装层/分段控件/空态两级/筛选栏/表格/确认框）、Tailwind 共存四条边界规则（R1-R4）、不得触碰的边界（brand 色域/冻结区/裸 hex/主题入口） | 改前端模板或样式前 |
| [`docs/agents/checks.md`](docs/agents/checks.md) | 测试与检查流程：后端四件套（**Windows 本机 golangci-lint 暂不可用，由 CI 兜底** / WSL 双环境）、生成链顺序（swagger → gen-apitypes → 再跑测试）、PG 契约测试纪律（随机 schema / 目录查询按 `current_schema()` 收窄）、前端 type-check + vitest、部署配置校验、DeepSec 安全检测 | 每次提交前 |
| [`docs/agents/release.md`](docs/agents/release.md) | 发布流程：分支 + PR + ruleset 门禁 + squash 直发 production，含应急通道与「禁 timeout 包 git/gh」铁律 | push / PR / merge 前 |
| [`docs/agents/multi-agent-git.md`](docs/agents/multi-agent-git.md) | 多 Agent 并发与 git 隔离：worktree 一会话一分支、游离提交取证、`git add` 纪律 | 多会话/自动化并发操作仓库时 |

> **建 worktree 一律走闸门**：`pwsh training-app/叉车维修培训学员端跨端应用/scripts/new-worktree.ps1 -Task <票号>`（**别裸用 `git worktree add`**）。它在创建处校验参数、并在新目录里实测 `jest --listTests` 必须列出套件 —— 目录名不合规会让 ③ 门**静默匹配 0 个套件**（血账 #1144；闸门见 #1185）。

## 验收门（合并前置）

**执行面是仓库级**：`.github/workflows/pr-evidence.yml` 对**每一个 PR** 运行，但**只对命中运行时面**（`*.uvue` / `*.uts` / training-app 下的三份 json）**的 PR 校验正文的 `## 验收证据` 段** —— 未命中时校验器在校验段**之前**就放行（`runtime.length === 0` 早退，用例见 `.github/scripts/pr-evidence-check.test.mjs`「非运行时面 PR：即使正文为空也通过」），命中而缺段则直接判红；未命中运行时面时仍**约定**写 `免（未命中运行时面）`（先例 #908）。所以本节的适用范围不限于移动端。

**开 PR 的时机（2026-09-15 裁定）**：一个 PR = **一件做完且已在目标环境验证过的事** —— 先把事做完、验完，**再**开 PR；不得边做边开、不得把 PR 当进度容器，也不得把「未命中运行时面 ⇒ 免四门」当成「不需要验证」（工具链 / 脚本类改动的验收判据是「**它要达成的那个行为在真实链路上成立**」，例如改截图判据就得跑一次真机 🟡 并留下产物）。完成定义（DoD）、例外通道与反例见 `docs/agents/release.md`。

**两条不看源码就读不出来的硬口径（2026-09-20 实测；判据源 `.github/workflows/pr-evidence.yml`）**：

- **「低风险运行时面」的豁免是逐文件的**（`:121-130`）：改动集里**每一个**文件都必须落在白名单内（`*.uts` / `*test.js` / `*.md` / `jest.config*.js`）。顺手改一处 `training-app/**/scripts/*.ps1`（**哪怕只是给契约套件补一条 token 注册**）⇒ 整个 PR 被降级为常规运行时面、**① 真机与 ② 微信开发者工具双双变必过**。这个排除是**故意**的（改门脚本本身不该拿低风险豁免）⇒ 正解是**把工具改动拆成独立 PR**（非运行时面 ⇒ 免四门，可立即合并），而不是在同一个 PR 里硬塞。
- **「结论」栏引用的可核验产物必须写在该行的行内**：仓库内 `docs/verification/<模块>/<PR号>/<页名>.<ext>`（PR 号须为数字）、GitHub 附件/Markdown 图片、或 sha 绑定的门评论链接（`#issuecomment-<id>`）。把它写在**子条目**（`  - …`）里、或只写裸本地产物路径（`.ci-verify/*.png`）、或任意 http(s) 链接，校验器**一律不认**（判「未引用可核验截图」）。

移动端改动触及运行时面时，适用四门验收与证据要求：**签收在人、合并不限人**——**人工门已收缩为 ①b**（只在命中「能力面」时必过：指纹 / 运行时权限弹窗 / 真机上传 / 厂商 ROM 交互），其「执行人」栏由**人**给出原文（agent 代录）；**①a**（agent 出证的逐页截图 + 机检行，按**一次分支收口**跑）的「执行人」栏允许写「agent 执行」（2026-09-16 修订，见移动端 `docs/adr/0016-真机门的人工性收缩与按批取证.md`）。签齐后 **agent 直接合并**，不必停在「待人工签收」；例外通道（「已接受未验证风险」）仍由人执行合并。

规则全文见 `training-app/叉车维修培训学员端跨端应用/AGENTS.md`「验收门与合并纪律」；**四门判据与原因**见 `training-app/叉车维修培训学员端跨端应用/docs/adr/0008-移动端验收门与证据.md`（**须写全路径**：它属移动端编号体系，根仓库另有一个同名的 `ADR-0008`）；**① 门的现行触发面、签收语义与取证节奏**见同目录 `0016-真机门的人工性收缩与按批取证.md`（2026-09-16 修订）。合并流程里的位置见 `docs/agents/release.md`。
