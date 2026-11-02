# 移动端专项 AI 工作流：技能落点与让位表（仅本线）

**适用面**：**只服务移动端（uni-app-x 学员端）这条线**。Web 前端与后端任务**不读本文件、也不受它约束**，各走 `docs/agents/checks.md` 与根 `docs/agents/skill-routing.md`「先分栈」表里各自那套。跨栈票分别执行各自适用的规则，**专项不扩散**。

**执行面不变**：一个运行时面 PR 从开工到收尾仍只有移动端 `AGENTS.md`「八步执行单」那一页。本文件只回答「叫哪个技能、哪些让位、哪些在本机跑不了」，**不新增步骤、不改任何门的判据、不扩大任何权限、不改既有合并裁定**。

**供给面在别处**：技能文件进不进库、装在哪个扫描根、改动怎么成 PR，见移动端 [`skills.md`](skills.md)；本文件管「选择与编排」，两份不重叠、不要互相抄。

**前提**：下面这些技能都是**本机用户级安装**（`~/.agents/skills`、`~/.dsh/skills`、`~/.qoder/skills`），**不在版本库里，也不是交付的前置依赖**。换机器、换开发者，装与不装都不影响交付正确性——八步执行单与 `scripts/` 里的原生脚本才是必需依赖。**加载不到就按无技能执行**：不得伪称执行、不得因缺技能另立流程、不得因此阻塞或降低门禁（同根 `docs/agents/skill-routing.md` 的通用诚实性一条）。

## 一句话分工

Matt 负责「把一件事做对」，tstack 只提供措辞与交接规范，FL 负责「能不能合」。**交付动作没有第二个入口**——提交 / 推送 / 开 PR / 合并 / 收尾一律按八步第 7、8 步与根 `docs/agents/release.md`；任何通用技能与 FL 规则冲突时**一律让位 FL**。

## Matt 开发层：职责与调用边界

| 技能 | 在本线干什么 | 边界（不许越的线） |
| --- | --- | --- |
| `grilling` | 需求澄清、把模糊处问清（人唤） | 已明确的票不必再烤；产出回写票或 ADR，别只留在会话里 |
| `to-spec` → `to-tickets` | 写成 spec、拆成票 | 票落 GitHub Issues（`gh`），label 按本线五个 triage role；**`implement-spec` 在本仓禁用** |
| `implement` | 按票实现：先 tdd，再 code-review | 它的收尾句是「把改动提交到当前分支」——**在本线必须落成八步第 2–6 步 + 走 `release.md`**，绝不在 `master` 上提交、绝不跳过门禁 |
| `tdd` | 红-绿-重构，先写测试 | 本线两腿（`jest.config.unit.js` 接线 / `jest.config.behavior.js` 行为）各有归属，选错腿 = 0 套件假绿，归属问 `node scripts/classify-guards.mjs` |
| `code-review`（Matt） | 评审：标准轴 + 规格轴，两路子 agent 并行 | 对象是「从某个基点以来的改动」；**与 tstack 的 `review-code` 是两个物件**（见「重复能力」节） |
| `diagnosing-bugs` | 困难 bug 与性能回退的归因循环 | 归因结论回写票；修完仍走八步 |

## tstack 交付层：FL 落点

| 技能 | FL 落点 | 处置 |
| --- | --- | --- |
| `full-stop` | 停在「PR 已建、门禁已满足、尚未合并」 | 可用作流程档位；**合并与否照八步第 7 步既有裁定**，本文件不新增授权步骤 |
| `full-send` | 一路做到合并 | 授权语义让位 FL：合并条件与例外通道全按 `release.md` 与本线 `AGENTS.md`「验收门与合并纪律」，**不削弱既有门禁** |
| `file-pr` | 开 PR 的动作 | **不得直接用**——它默认 base `origin/main` 且开 PR 早于取证；本线固定 `preflight-pr.ps1` 预检绿 → `submit-pr.ps1` 建 PR |
| `leaving-pr-comments` | PR 评论措辞 | 可用；评论里的读数口径仍按 `release.md` / `checks.md`，别自带判据 |
| `modernize` | 现代化既有文档/技能 | 见「`modernize` 适配约束」 |
| `takeover` | 接手别人的 PR / 分支 / agent 线程 | 见「`takeover` 触发条件」 |
| `babysit-pr` / `sol-loop` / `council` | — | **本机不可执行**，见下节 |

## 本机不可执行的技能（2026-11-02 实测，换机须重测）

| 技能 | 不可执行的原因 | 本线处置 |
| --- | --- | --- |
| `sol-loop` | 需要把子任务路由到 Sol 模型且**明确禁止替换模型**；本机子 agent 的 model/provider 覆盖实测全部返回 `null`（不带覆盖的对照跑通） | 报阻塞，**不用同模型冒充 Sol**；需要第二意见就走 `grilling` / `code-review` |
| `council` | 需要多模型并行聚合，同因（无可用外部路由） | 同上，不伪装成「已开过 council」 |
| `babysit-pr` | 其看守脚本 `pr-wait` 是 `#!/usr/bin/env python3` 且 `import fcntl`——Windows 无 `python3`、无 `fcntl`；其 `ready` 判据还引用本仓根本不存在的 bot | 本线用 `gh pr checks` / `gh pr view` 自看，或推完就收工；**技能本身禁止自写轮询替代**，本机也就别造替身 |

**不得伪装执行**——不许用同模型冒充 Sol、不许假装跑过 council、不许自写轮询顶替看门。**这三条只是这台机器的读数，不是本仓的结论**：别的机器若已配通 Sol 路由或装上 `python3`，相应技能照常可用，先自测再采信。

**裁定留痕**（2026-11-02）：**在这台机器上**不配置路由、不改 `pstack-dsh.json`、不替换模型，这三条就停在这张标注上。将来要让它们能跑，前提是先把路由真的调通并复测，**不是改技能文本**。

## 重复能力与冲突裁决

1. **`code-review`（Matt）vs `review-code`（tstack）**：前者是「标准 + 规格」双轴评审、带子 agent 并行；后者是六轮穷举扫描。**评审需求走 `code-review`，穷举体检走 `review-code`**，不叠着跑两次装作两次独立审查。
2. **`implement` vs 八步执行单**：`implement` 是内循环方法，八步是 FL 的交付面。**方法可以让位，门禁不能**——`implement` 说「提交到当前分支」时，落点是八步第 6 步的证据与 `release.md` 的 PR 流程。
3. **`file-pr` / `full-send` vs FL 发布权威**：开 PR 与合并的动作**只有 FL 脚本这一个入口**；tstack 技能只贡献措辞与档位。
4. **技能触发词 vs 用户指令**：用户与 FL 文档优先，技能描述里的触发词冲突时让位（同根路由表的规则）。

## `modernize` 适配约束

- **不重排 `AGENTS.md` 结构**、不添加未被裁定的新章节；移动端常驻层只有「八步执行单」一页（ADR-0048），别把参考表搬回去。
- 不重跑 `setup-matt-pocock-skills`（本机已是单份上游本体）。
- 产出只落在**本线 `docs/agents/` 已有条目或经裁定的新文件**里；碰根文档需单独授权。
- 它「现代化」的是文档表述，**不动任何门的判据、不动 CI、不动 Ruleset**。

## `takeover` 触发条件

- 触发：明确要接手**别人的** PR / 分支 / agent 线程（不是自己刚开的那张）。
- 接手前先读：票 + 对应 ADR + 移动端 `AGENTS.md`；确认工作树归属（`git worktree list`），**别在宿主树里改**。
- 并发约束：HBuilderX 是**单实例串行资源**——别在别人正跑 ②/④ 时抢设备；锁与时间预算见 [`concurrency.md`](concurrency.md)。
- **不得**：重开已合并的 PR、替别人决定合并、清理别人的 worktree（拆树只走 `remove-worktree.ps1` 该票对应的那棵）。

## `full-stop` / `full-send` 在本线的语义

两者是**同一套流程的两种停点**，不是两个编排器：`full-stop` 停在「PR 已建、门禁已满足、未合并」，`full-send` 继续走到合并。

**合并这一动作不在本文件里定义**——照本线 `AGENTS.md`「验收门与合并纪律」与根 `docs/agents/release.md` 的既有裁定执行。**本文件不新增授权步骤、不降低门禁、不改例外通道**；证据不齐或走例外通道时，自然就停在 `full-stop`。

## 派发子 Agent 的规则要求

1. **明写栈**：prompt 里注明「移动端（`training-app/叉车维修培训学员端跨端应用/`）」，否则子 agent 可能按前端/后端流程走。
2. **明写边界**：子 agent 只做被派的**那一步**（读票、定位、改代码、跑某一腿）；**不得**自行开 PR、合并、拆树、提分支——交付动作留在主会话按八步执行。
3. **隔离**：一会话一分支一树（[`../../../../docs/agents/multi-agent-git.md`](../../../../docs/agents/multi-agent-git.md)）；`git add` 只加明确列出的文件，**禁 `git add -A` / `git add .`**。
4. **设备串行**：同一个 HBuilderX 实例下**不得并行**派两个要跑 ②/④ 的子 agent；要并行就各走各树、设备排队。
5. **回报读数**：子 agent 回的是**读数行原文**（`TEST_*_RESULT` / `MP_WEIXIN_RESULT` 等），不是「应该没问题」。
