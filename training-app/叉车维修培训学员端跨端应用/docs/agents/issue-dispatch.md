# Issue 派活模板与执行契约

**用法（派活时只需这一行）**：

```
按 training-app/叉车维修培训学员端跨端应用/docs/agents/issue-dispatch.md 执行 https://github.com/DriftingLi/FL/issues/<N>
```

（在移动端树里作业时，本文件即该树的 `docs/agents/issue-dispatch.md`，相对路径同样可用。）

**适用面（为什么它落在移动端层）**：本文件服务**移动端（uni-app-x 学员端）那条线**的派活与执行。「阶段闸门 / 进度契约 / 收尾口径」是**这条线的工作方式**，**不是全仓约定** —— 实测全仓带 `## 进度：N%` 的票绝大多数是移动端票。故它随移动端 `AGENTS.md` 导航；**前后端会话不读它、也不受它约束**。（根层 `docs/agents/` 只放跨栈的 tracker / 文档纪律。）

本文件是那份长清单的**唯一事实源** —— 派活不再手写检查表。手写会漂移：模板曾长期以 `/wayfinder` 开头。

> **2026-10-10 更新（当天两次）**：上一句的下半段（「而本仓**没有 map**，于是每个新会话都要重新发现一次」）**作废** —— 它是自我实现的判据（同文末）。同日按维护者裁定试建了首张 map `#1585`，**当天又关闭**（流程选错：能一手握住的任务不该建 map，且让 map 承载执行会让它的 `Destination` 自指下游产物）。`wayfinder:*` 五个 label 仍在，sub-issues 与原生 `blocked_by` 边均实测可用，但**本仓实际票不挂它们**。**§0 首条与文末节同步改写。**

## 0. 形态判定（先做，30 秒）

- 本票是 **issue**。它**不带** `wayfinder:*` label，本仓的票一律是 **`/to-tickets` 的执行切片**，按本文件走。
- **若票顶出现 `Part of #<map>` 或 `wayfinder:*` label**：那是**外来形状**，不是本仓的规范形态 —— 按文末节「什么时候用 wayfinder」判断它是误用还是真遇迷雾；`wayfinder:grilling` / `wayfinder:prototype` 是**对话票**，须与提出它的人一起解，agent 不代答他那一侧。
- 票面若带 `needs-triage`：那是**前置步骤**，先诊断（见 §3），不许跳过直接实施。
- **波级 issue（无票号的 `refactor epic:`）有两种形态，别按感觉挑**（2026-09-21 立，先例：`#1201` 第十三波 = spec 形；`#1221` 模块声明与契约 harness = 索引形）：
  - **spec 形** —— 波级 issue **自己就是 spec**：`Problem Statement` / `Solution` / `User Stories` / `Implementation Decisions` / `Testing Decisions` / `Out of Scope`；落地后**不再维护**，学到的东西回写在 ADR 的「实施回记」段。
  - **索引形** —— 波级 issue **只是索引**：正文首行写明「规范面 = `<ADR 路径>`」，不复制 ADR 的内容。
  - **四问定形态**（都问「这一波长什么样」，不问偏好）：① 有**用户可感知**的故障吗？② 票能**独立开工**（各自 worktree / 各自会话）吗？③ 各票的**收口判据一样**吗？④ 会**一个 PR** 落地还是多个？
    ① 有 ⇒ spec 形的 `Problem`/`User Stories` 才有真东西可写；① 无（纯结构税 / 维护性深化）⇒ 写出来只是把 ADR 的决策句换个主语，多出一份要与 ADR 对齐的第二文档 ⇒ 取**索引形**。
    ③ 不一样（如某票命中运行时面走四门、其余免四门）⇒ 判据必须**下沉到票面**，波级 issue 不要复述。

## 1. 读现状

- [ ] **认领**：`gh issue view <N> --json assignees`；未认领则 `gh issue edit <N> --add-assignee @me`。**先认领再动手**。
- [ ] 读 **Description / 评论 / 标签 / 依赖（`blocked_by` 是否已解）/ 进度区（`## 进度：N%`）**。
- [ ] 确认**交付物**与**验收标准**（票面写了什么才算做完）。
- [ ] **查承载面**（本会话新立）：本票要碰的文件/页面，有没有 **open PR 或 open issue** 已经在动？
      `gh pr list --state open --json number,files` + 票面声明的承载面 ⇒ **有交集就等（挂 blocking）或并入，不要并行开第二张**。
      （反例：#1045 出生即死 —— 开票时没看 #1059 正在改同一个文件的同一段。）
- [ ] **状态一律现测**：票是否已关、PR 是否已合、依赖是否已解，**先 `gh issue view` / `gh pr view` 再开口**，不按记忆走。实测踩过：凭记忆断言某票 OPEN，而它 3 小时前已随另一次切票关闭。

## 2. 执行

- [ ] 目标不清 / 需维护者定夺 → 用 **grilling** 澄清（**不猜**）。
- [ ] 制定方案 → 实施 → 按验收标准**对抗式**自查。
- [ ] 自查里必须问一句：**「这条判据会不会放过坏实现？」** —— 断言**产物**，不要断言片段或源码文本（反例：`toContain("'/diagnosis/manual/'")` 是坏实现的超集，绿着放过了一个 404）。

## 3. 阶段闸门（动作开始前必读，是动作的一部分，不是可选项）

- [ ] 先读票面现状（进度区 / 已有实施记录 / 评论 / 标签），判断它处于**哪个阶段**。
- [ ] 带 `needs-triage` ⇒ **必须先完成诊断**；诊断完**摘掉该标签**后才允许进入实施。
- [ ] 诊断时判断当前进展：
  - 已有实施且**真实** ⇒ 核验是否符合验收标准，属实则维持 **95% 待确认** + 摘标签（转 `ready-for-agent`）；
  - 已有实施但**虚假/半成品** ⇒ 进度**据实回调**到真实值，继续诊断；
  - **未动工** ⇒ 正常诊断（复现 → 根因 → 方案 → 写入票面）。

## 4. 收尾

- [ ] 验收通过 ⇒ `## 进度：100%` + **close**。
- [ ] 未完成 ⇒ 按进度契约**如实**更新（含下一步）。**95% 必须写明待确认什么；未确认不得 close。**
- [ ] 若本票被判定**重复** ⇒ 按 `docs/agents/triage-labels.md` 的「重复票的处置」（`close --reason "not planned"` + 一条指向原票的评论）。
- [ ] 若本票带 `Part of #<map>` ⇒ 收尾时**向 map 的 `## Decisions so far` 追加一行**（标题带链接 + 一句 gist），再 close。**别在 map 正文里复述细节** —— 细节住它的票或 ADR。**没带 `Part of` ⇒ 不必找 map。**

## 5. 正文格式（写 / 改票面时必须遵守）

- [ ] 用**真实换行**：每个 `## 章节` 独占一行，段落间留空行。
- [ ] **禁止**字面 `\n`（把换行写成两个字符）；**禁止** BOM 开头。首字节**不是固定 `## `**——正文可以以 `# 标题` 开头（此时首字节是 `23 20`）；判据是「文件不以 `EF BB BF` 开头，且字面 `\n` 计数为 0」。
- [ ] 以**文件方式**提交：`gh issue edit <N> --body-file <文件>`（文件内是真换行），不要内联转义字符串。
- [ ] 写入后**回读校验**：首字节、`(?m)^## 进度` 是否命中、字面 `\n` 计数是否为 0。
- 正例：`## 进度：90%` 独占一行，空行后接 `下一步：xxx`。反例：`## 进度：90%\n下一步：xxx`。

## 6. 文档分层（写文档时）

约定 / 决策进 `docs/adr/`、`docs/agents/`（**入库、只增不删**，过时用「取代」表达）；临时产物（handoff、探针、PR 正文草稿）进 `.scratch/`（不入库）。详见 `docs/agents/domain.md`。

## 什么时候用 wayfinder：判据是「能不能一手握住」，不是「任务大不大」（2026-10-10 两次改写）

**第一次改写**：本节原为「为什么不用 wayfinder：准入条件未命中」，据维护者裁定启用 wayfinder 而改写。**第二次改写**：当天的实践证伪了那次改写里的一处关键论断，见下。

### 准入判据

`/wayfinder` 在 `/ask-matt` 的路由表里是 **on-ramp**，不在 main flow 上：

> Where **`/grill-with-docs`** sharpens an idea you can hold in one session, wayfinder is for the idea you can't.

**真正的判据是「能不能一手握住」—— 连第一个该决定什么都说不出，才需要 map。** 任务大本身不构成理由：60 页 UI 改造照样能一手握住（先 grep 出 58 页 `navigationStyle: custom`、57 处 `.status-bar`、`getMenuButtonBoundingClientRect` 全仓零命中这些**事实**，再 grill 十八问，一个会话收敛）。判据一旦退化成规模感，map 就变成「任务大所以要」的仪式。

### 本仓为什么用不上 map（map 与 ADR 是分工，分工线在这里）

| | wayfinder map | ADR | 本仓实况 |
|---|---|---|---|
| 装什么 | **索引 + 迷雾 + 范围边界**：Destination / Notes / Decisions so far（每条一行 gist + 链接）/ Not yet specified / Out of scope | **决策的完整论证与约束**：六段结构、证据、验收判据 | **ADR 已覆盖迷雾**——「③ 明确不做的事」就是本仓版本的 Out of scope，「② 十八问」覆盖 Decisions so far |
| 谁读 | 每会话开场一次（低分辨率） | 冷启动会话**必读** | 冷启动读「票 + 对应 ADR + `refactor-decisions.md`」，已够 |
| 在哪 | GitHub（label `wayfinder:map`） | 仓内 `docs/adr/` | 仓内那份是冷启动必读面，GitHub 那份是第二份低分辨率视图 |

**「ADR 就是本仓的 map」那句是准确的** —— 本仓的迷雾不长在票上，长在 ADR 的 ③ 里。**再开一层 map 是第三份事实源。**

### 反面实证（#1585，2026-10-10，已关闭）

当天的实践给了一次反面证据，值一条：

**把「承载执行」覆盖进 map，会让地图的 Destination 自指下游产物。** 当时的覆盖条款让子票标 `wayfinder:task`、交付物即收口物；但 wayfinder 的地图是**决策地图**，`ask-matt:46` 明写它「hands off, it doesn't build」——清完雾要 merge 到 `/to-spec` 收拢成计划。**一旦让票自己去交付，地图就失去了它的下游出口**：`Not yet specified` 里那三片迷雾还没等 `/to-spec` 收拢就消失了，`Destination` 只好写「一份能交接的 spec」——**那是 `/to-spec` 的产物，是地图自己下游的东西**。一张终点指向自己下游的地图没有可走之路。

**所以那处覆盖是错的。** 结论三条：

1. **本仓的票属 main flow 的 `/to-tickets`，不属 wayfinder。** 主流程是 `grill-with-docs` → ADR → `/to-spec` → `/to-tickets` → `/implement`；ADR-0035 之后**本该有一步 `/to-spec`**，这一步不能省（ADR 说「怎么做对」，spec 说「做成什么样」，五张票全是前者）。
2. **票据纪律不因建图而放宽**：原生 `blocked_by` 边是依赖的唯一表达，不靠正文里的「Blocked by」行；`/to-tickets:67` 的「Do NOT close or modify any parent issue」对 map 同样适用。
3. **五个 `wayfinder:*` label 已建**（第一次改写时创建），但**本仓的实际票不挂它们**；若将来真出现「连第一个决定是什么都说不清」的情况，用法照 `/wayfinder` SKILL.md 原样，**不要带本节的那处覆盖**。