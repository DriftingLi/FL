# AGENTS.md

This file provides guidance to Qoder (qoder.com) when working with code in this repository.

# 叉车维修培训与残值评估系统

面向叉车维修培训与叉车残值评估的全栈系统。**本文件只做导航**——工作约定按块拆在 `docs/agents/` 下，动手前先读对应块。

## 领域文件

- **系统总览 / 技术栈 / 架构大图 / 运行与部署 / 全量命令**：`README.md`（第一次接触本仓先读它建立大图——子域名多工作区、统一账号双令牌、三部署面、CI 触发模型等——再回本文件读工作约定）

- **领域词汇表**：**两份，各辖一侧** —— 根 `CONTEXT.md`（全站 / Web / 后端 / 跨端契约）· 移动端 `training-app/叉车维修培训学员端跨端应用/CONTEXT.md`（uni-app x 侧：生物识别、快捷登录、凭据存储等）。**无** `CONTEXT-MAP.md`；两份之间**未声明仲裁规则**，按改动所属侧取用那一份。

- **架构评审范围**：architecture review 覆盖 Web 前端（`frontend/`）、后端（`backend/`）与部署面；**移动端**（`training-app/叉车维修培训学员端跨端应用/`）由 @zhengcookie 负责，其深化机会不在本仓评审范围——评审不重复提议移动端候选，移动端自身决策见其独立 ADR 体系。

- **架构决策记录（ADRs）**：根仓库 `docs/adr/`（编号 `ADR-0001-…`）；**移动端项目**（`training-app/叉车维修培训学员端跨端应用/docs/adr/`）**另有一套独立 ADR**（`0001-…` 起编号，记录 uni-app-x 侧决策：SSE 流式传输、轻量状态管理、手动 JSON 映射、生物识别、安全存储等）——两套编号体系互不相关，引用时注明「根仓库 / 移动端」

## Agent skills

### Skill routing

本环境**三套技能并存**（`~/.qoder/skills` 的 Matt Pocock 集 / `~/.agents/skills` / superpowers 插件），同一意图常有多个入口，而 superpowers 每次会话注入的是「有 1% 可能就必须调用」——**只有本文件压得住它**（该技能自述用户指令优先于技能）。三条最高频的默认入口在此常驻：**澄清需求 → `grilling`（人唤）· 写成 spec / 拆票 → `to-spec` → `to-tickets` · 调试归因 → `diagnosing-bugs`**。表未覆盖的意图可自主选择；其余六个意图、多问技能的落点改写与逐项实测见 [`docs/agents/skill-routing.md`](docs/agents/skill-routing.md)。

### Issue tracker

Issues 存放在 GitHub Issues（使用 `gh` CLI）。See `docs/agents/issue-tracker.md`.

### Triage labels

五个 canonical triage roles，label 与 role 同名（`needs-triage` 等）。See `docs/agents/triage-labels.md`.

### Domain docs

词表与 ADR 都是「根 / 移动端」**两套并行**：动哪一侧就读那一份，落点见上方「领域文件」；通用规则看 `docs/agents/domain.md`。

### Security scan

AI 安全审计用 DeepSec（Shield）。See `docs/agents/security-scan.md`.

## 工作约定（按块导航）

| 文件 | 内容 | 什么时候读 |
| --- | --- | --- |
| [`docs/agents/ui-conventions.md`](docs/agents/ui-conventions.md) | 前端 UI 约定：UI 词汇（封装层/分段控件/空态两级/筛选栏/表格/确认框）、Tailwind 共存四条边界规则（R1-R4）、不得触碰的边界（brand 色域/冻结区/裸 hex/主题入口） | 改前端模板或样式前 |
| [`docs/agents/checks.md`](docs/agents/checks.md) | 测试与检查流程：后端四件套（**Windows 本机 golangci-lint 暂不可用，由 CI 兜底** / WSL 双环境）、生成链顺序（swagger → gen-apitypes → 再跑测试）、PG 契约测试纪律（随机 schema / 目录查询按 `current_schema()` 收窄）、前端 type-check + vitest、部署配置校验、DeepSec 安全检测 | 每次提交前 |
| [`docs/agents/release.md`](docs/agents/release.md) | 发布流程：分支 + PR + ruleset 门禁 + squash 直发 production，含应急通道与「禁 timeout 包 git/gh」铁律；**证据主键 = commit 评论、先预检绿再开 PR**（见其「证据主键」段）；**合并后的分支与 worktree 收尾顺序**见其 :13 | push / PR / merge / 清理工作树前 |
| [`docs/agents/multi-agent-git.md`](docs/agents/multi-agent-git.md) | 多 Agent 并发与 git 隔离：worktree 一会话一分支、游离提交取证、`git add` 纪律 | 多会话/自动化并发操作仓库时 |
| [`docs/agents/comments.md`](docs/agents/comments.md) | 注释标准：三类判据（保留 / 删除 / 不可动）、本仓先例、「只减不增」棘轮锁哪些类与为什么 | 增删改注释前（后端 `backend/**/*.go`；底数与逐包实测见 [`docs/design/1445-p4-comment-audit.md`](docs/design/1445-p4-comment-audit.md)） |

> **建 worktree 一律走闸门**：`pwsh training-app/叉车维修培训学员端跨端应用/scripts/new-worktree.ps1 -Task <票号>`（**别裸用 `git worktree add`**——闸门在创建处校验参数并实测 `jest --listTests` 必须列出套件，目录名不合规会让 ③ 门**静默匹配 0 个套件**，血账 #1144）。**移动端交付不用 Qoder 托管树**；宿主「本地任务的 Worktree 配置」框只填 `wt-bootstrap.ps1` 一行调用——血账、框现值核对方法与移动端边界，见移动端 `AGENTS.md`「Qoder 托管 worktree 的使用边界」。

## 会话纪律（降本四条，#1705）

成本按**实付 token**（`Σ(input − cached)`）/请求量，基线 8,773（#1694），复算口径与前后 run 对比见 [`docs/agents/checks.md`](docs/agents/checks.md)「成本量尺」。四条：**① 一模型一上游**——一个 run 内不换模型、不换上游，要换就结束本 run、另开新 run 并在台账注明换模型（混跑会让「模型档位」这个基线失去意义）；**② 优先续跑**——上下文还在就继续同一 run，别为「干净」重启（首帧重建是纯成本，实测 #1694 首帧 116,519）；**③ 长日志不整读**——只带结果行 + 按需索引（`acp_status` / `search_context`），别把日志与整文件读进上下文；**④ 折叠要赚钱**——`compress` 只为换回**足够体积**的旧内容才做，不为「看着干净」而折叠（实测同系列 run 折叠 460 → 129，成本反降一个量级）。

## 命令速查（只补 README / checks.md 没写的「跑单个测试」）

构建 / lint / 全量测试 / 生成链 / 迁移 / 部署的**权威命令**在 [`README.md`](README.md)（常用命令、快速开始）与 [`docs/agents/checks.md`](docs/agents/checks.md)（提交前四件套与守卫）。本处**不重复那些表**——它们靠指路而非抄写，抄过来正是历史上漂移的来源。这里只补一件两表都没写、又几乎每次会话都要用的：**如何只跑一个测试**。

- **后端**（`backend/`；全量 `make test` = `go test ./... -race -cover`）
  - 单包：`go test ./internal/core/ -race`
  - 单用例：`go test ./internal/core/ -run TestXxx -race`（`-run` 接正则，可前缀匹配一簇）
  - ⚠️ 命中 `testutil.NewPostgresDB` 的 Postgres 契约用例，本机无 `DATABASE_URL` 会**干净 skip** ⇒ 看到 `ok` ≠ 测过，首跑在 CI（详见 checks.md「两条纪律」）。
- **前端**（`frontend/`；全量 `npm test` = `vitest run`）
  - 单文件：`npx vitest run src/api/__tests__/page.spec.ts`
  - 按名：`npx vitest run -t "用例名片段"`（vitest 5 起匹配的是 `describe > test` 全名，跨分段时用 ` > ` 或 `.*`，别用空格，详见 checks.md）
- **移动端**（`training-app/叉车维修培训学员端跨端应用/`；全量 `npm test` = `npm run test:unit && npm run test:behavior` —— ③ 门自 #1702 起**两腿**：`test:unit` 接线腿（`jest.config.unit.js`）+ `test:behavior` 行为腿（`jest.config.behavior.js`），两腿各自 `-i` 串行）
  - 单文件：`npx jest --config <该套件所在腿的 config> <相对路径>`；按名加 `-t "片段"`。**选错腿的 config 会让 jest 当那个套件不存在**（0 套件，长得像绿）—— 套件归哪一腿问 `node scripts/classify-guards.mjs`
  - 运行时面（① 真机 / ② 微信开发者工具 / ③ 契约门）另走别处、需真机环境，不在本命令射程——见下方「验收门」与移动端 `training-app/…/AGENTS.md`。

## 验收门（合并前置）

**执行面与机器语义（仓库级）**：`.github/workflows/pr-evidence.yml` 每个 PR 都跑，**只对命中运行时面**（`*.uvue` / `*.uts` / training-app 下三份 json）的 PR 校验正文 `## 验收证据` 段——未命中早退放行（约定仍写 `免（未命中运行时面）`，先例 #908）、命中缺段判红；适用范围不限于移动端。证据来源不止 PR 正文与 PR 评论：**commit 评论（#1770 起的证据主键）也在候选集**，票（issue）评论不进判据。逐行机制（早退用例、白名单行号）在 `docs/agents/checks.md`「运行时面判定与证据校验的机器语义」。

**开 PR 的时机（2026-09-15 裁定）**：一个 PR = **一件做完且已在目标环境验证过的事**——先做完、验完，**再**开 PR；不当进度容器，也不把「未命中 ⇒ 免四门」当「不需要验证」（工具链/脚本类改动的验收判据是「它要达成的那个行为在真实链路上成立」）。完成定义（DoD）、例外通道与反例见 `docs/agents/release.md`。**#1770 起顺序固定**：证据先出在 **commit 上**（门脚本 `-PostToIssue <票号>` 贴 commit 判据评论、同一份正文的展示副本贴票/PR）→ `preflight-pr.ps1` 预检绿 → 最后由 `submit-pr.ps1` 建 PR（③ run 链接由它从 `ci-summary` check 自动发现），首次推送即判绿。

**两条不看源码就读不出来的硬口径（2026-09-20 实测；判据源 `.github/scripts/lib/gate-validator.mjs` —— `pr-evidence.yml` 与 `mobile-gates.yml` 都不 checkout，用 `repos.getContent` 取同一份模块）**：

- **「低风险运行时面」豁免是逐文件白名单，且它与「①② 变必过」是两个不同源的过滤器**——**别写成一句因果**（工具改动拆独立 PR 仍是正解，代价只是多跑一次移动端测试门）。因果链、判据源行号与反例在 `docs/agents/checks.md`「运行时面判定与证据校验的机器语义」。
- **「结论」栏引用的可核验产物必须写在该行的行内**：仓库内 `docs/verification/<模块>/<数字>/<页名>.<ext>`（数字 = **票号**；旧 PR 号目录仍是同一数字段、照样被接受）、GitHub 附件/Markdown 图片、或 sha 绑定的门评论链接（`#issuecomment-<id>` —— **commit 评论**是判据评论、**票/PR 评论**是展示副本，两种 id 同源）。把它写在**子条目**（`  - …`）里、或只写裸本地产物路径（`.ci-verify/*.png`）、或任意 http(s) 链接，校验器**一律不认**（判「未引用可核验截图」）。

移动端改动触及运行时面时，四门与证据要求的**规则全文**见 `training-app/叉车维修培训学员端跨端应用/AGENTS.md`「验收门与合并纪律」与 `training-app/叉车维修培训学员端跨端应用/docs/adr/0008-移动端验收门与证据.md`（**须写全路径**，它属移动端编号体系、根仓库另有同名 `ADR-0008`）。本处只留三句：**签收在人、合并不限人**——人工签收只有能力面真机签（「执行人」栏由人给出原文、agent 代录），其余门允许「agent 执行」，证据填齐后 **agent 直接合并**，不必停在「待人工签收」（证据齐且不走例外通道时，「是否合并 / 是否清理工作树」不是问题，直接执行；合并前一行披露 production 部署口径见 `docs/agents/release.md`:16）；**窗口口径 2026-10-15 收紧**（真机逐页取证窗口内不跑、触及能力面的 PR 窗口内**不合并**、排队到退出后）与冲突裁定，见移动端 `docs/adr/0040-小程序优先窗口的验收门槛口径（2026-10-15 裁定）.md`——① 门触发面 / 签收语义 / 取证节奏见同目录 `0016-真机门的人工性收缩与按批取证.md`，窗口细则见 `0039`；**例外通道**（「已接受未验证风险」）仍由人执行合并。合并流程里的位置见 `docs/agents/release.md`。
