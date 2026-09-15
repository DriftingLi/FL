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
| [`docs/agents/checks.md`](docs/agents/checks.md) | 测试与检查流程：后端四件套（**Windows 本机** / WSL 双环境）、前端 type-check + vitest、部署配置校验、DeepSec 安全检测 | 每次提交前 |
| [`docs/agents/release.md`](docs/agents/release.md) | 发布流程：分支 + PR + ruleset 门禁 + squash 直发 production，含应急通道与「禁 timeout 包 git/gh」铁律 | push / PR / merge 前 |
| [`docs/agents/multi-agent-git.md`](docs/agents/multi-agent-git.md) | 多 Agent 并发与 git 隔离：worktree 一会话一分支、游离提交取证、`git add` 纪律 | 多会话/自动化并发操作仓库时 |

## 验收门（合并前置）

**执行面是仓库级**：`.github/workflows/pr-evidence.yml` 对**每一个 PR** 校验正文的 `## 验收证据` 段 —— **缺段直接判红**（校验器 L113–115），与改动是否命中运行时面无关（未命中时该段写 `免（未命中运行时面）`，先例 #908）。所以本节的适用范围不限于移动端。

**开 PR 的时机（2026-09-15 裁定）**：一个 PR = **一件做完且已在目标环境验证过的事** —— 先把事做完、验完，**再**开 PR；不得边做边开、不得把 PR 当进度容器，也不得把「未命中运行时面 ⇒ 免四门」当成「不需要验证」（工具链 / 脚本类改动的验收判据是「**它要达成的那个行为在真实链路上成立**」，例如改截图判据就得跑一次真机 🟡 并留下产物）。完成定义（DoD）、例外通道与反例见 `docs/agents/release.md`。

移动端改动触及运行时面时，适用四门验收与证据要求：**签收在人、合并不限人**——人工门（①）由人签收（「执行人」栏由人给出原文，agent 代录），签齐后 **agent 直接合并**，不必停在「待人工签收」；例外通道（「已接受未验证风险」）仍由人执行合并。

规则全文见 `training-app/叉车维修培训学员端跨端应用/AGENTS.md`「验收门与合并纪律」；**四门判据与原因**见 `training-app/叉车维修培训学员端跨端应用/docs/adr/0008-移动端验收门与证据.md`（**须写全路径**：它属移动端编号体系，根仓库另有一个同名的 `ADR-0008`）。合并流程里的位置见 `docs/agents/release.md`。