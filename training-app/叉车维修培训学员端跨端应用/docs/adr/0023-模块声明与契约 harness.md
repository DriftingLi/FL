# 0023 - 模块声明与契约 harness：把「模块是什么」收成一份数据

**状态**：已接受（2026-09-20，来源：移动端架构评审（仅移动端范围）候选 01+05 合并，经 grilling 逐条定案）

## ① 目标与范围

要解决的是**同一条模块事实被各写一遍**，不是补守覆盖面。移动端 `utils/` 现有 **77 份 `*Contract.test.js` / 16,234 行**，其中 **75 份各自重建仓库根**、**41 份各自定义 `read()`**、**13 份各自实现目录遍历**、**15 份各自写死 600**、**12 份靠解析守护脚本源码文本取 `GUARD_ALLOWLIST`**。后果不是「重复」而是**同一条事实可以有互相矛盾的版本**：已实证 `api/forum.uts` 618 行躺在自称「已达标、无过度碎片化」的 forum 模块里——该模块的预算只扫 `pages/forum/**`，只有 exam 用一句 `.concat('api/mockExam.uts')` 手工把域 api 塞进扫描面。

范围是**结构事实的声明与执法**：模块归属、预算、目录深度、拆出物接线、allowlist 归属。**不含**读取机制（归 0019）、不含守护分类（归 `docs/agents/guards.md`）、不含 `.uts` 编译与四门口径（归 0008）。

## ② 选型 + 一句理由

| 决策 | 一句话理由 |
| --- | --- |
| **① 事实与机制分家**：`utils/modules.js` 只放数据（零逻辑零断言），`utils/contractHarness.js` 只放机制 | 深化动作 = 把真相从调用点原文收成**一份数据**（0019 在读取层、0007 §#914 在门脚本家族各做过一次）；合成一份会把「改了模块边界」与「改了检查逻辑」混进同一个 diff |
| **② 模块键 = `pages/` 下的目录名**，一对一是默认规则；另设 `extraDirs`（目录外的家）与 `crossModuleConsumers`（本模块的件被谁消费） | 沿用既有 `xContract.test.js` 命名与目录树，零命名争论；语义名会与目录脱钩，改名要改两处 |
| **③ 共享件归主消费方**：`components/ai-chat/**` 归 `ai-assistant`，`search` 登记为跨模块消费者 | `search` 是**使用**（0014 的落点渲染）而非**拥有**；独立模块键会让一个无页面、无生命周期的目录与页面模块平级 |
| **④ harness 出口 = 纯事实，断言留在测试里** | 先例是 `gateCommonLibContract` 的纯函数 `scanContract(sources)`；断言进 harness 会让「为什么红」变成读 harness 源码，而失败信息质量正是本仓的门面 |
| **⑤ 预算按全文件计数，允许声明逐文件覆盖**（`budgetOverrides`，须带理由与 issue） | 分段计数要在 uvue 里判定 template/script/style 边界（Vapor 下未验证）；逐文件覆盖以极低成本解决样式型超限，且例外**写在声明里可见** |
| **⑥ 全仓 23 个模块都建声明**，未达标模块预算列写 `pending`（登记不执法） | 「队列外的模块不受约束也没人看得见」是本次要治的痛点；`pending` 让进度变成表上一列，执法面仍留在有证据处 |
| **⑦ 对账双向 + 跨模块唯一**：目录枚举 ⊆ 声明（漏登记红）、声明 ⊆ 实际存在（幽灵声明红）、同一文件不得被两个模块声明 | 「声明是事实源」新造的失效面是**静默漏扫**；现状用「数量下限」兜底，可被「删一个加一个」绕过，集合相等才关得掉 |
| **⑧ `GUARD_ALLOWLIST` 搬进 `utils/guardAllowlist.js`**（守护脚本与 12 份测试同源） | 把一个值变成一段要 `indexOf`/`slice` 去抠的文本，是本次深化最典型的反面教材——那 12 份自己写着「终止符缺失会静默扩扫全文」 |
| **⑨ `utsAndroidCompile.test.js` 的 20 条规则原样不动** | 那是全仓语言级纪律（含注入自检与 `readdir` 顺序无关锁），与「模块是什么」正交；只搬走它被重依赖的两个事实（allowlist、仓库根） |
| **⑩ 不把本 harness 塞进 `mobile-test` 必检 job** | 0019 §⑤ 已记明代价：往必检 job 加 fail-closed 步骤 = 新增「红了全仓都不能合」的单点（#1177 期间真的堵过两次）。本 harness 走**本地锁 + 定期全量**，且判据不得依赖平台 |

## ③ 明确不做的事

- **不重做读取层归一**：一律用 0019 的 `utils/utsHarness.js#readText`；不在测试里再抄第二份 `normalizeEol`。
- **不改守护分类口径**：不动 `scripts/classify-guards.mjs` 与 `docs/agents/guards.md`；不把本 harness 的断言混进 ③ 门证据。
- **不改四门**：0008 的触发面、证据四字段、`pr-evidence` 校验器改动量 = 0。
- **不为已达标模块写 `budgetOverrides` 来买绿**：`api/forum.uts` 618 行**不覆盖、直接判红**——它是「达标即锁」失效的实证。
- **不按模块一票一 PR**：不把 77 份测试的收敛拆成六次重复劳动。
- **不动 UI**：不顺手改任何 `.uvue` 观感；分割产生的像素漂移按 0007 的 UI 冻结纪律处理。

## ④ 拆分步骤

**票 A —— 声明与机制（不执法）**：新增 `utils/modules.js`（23 个模块的键 / 归属目录 / 必需文件 / 接线 / 拆出物 / 预算 / 深度 / allowlist 归属；未达标标 `pending`）与 `utils/contractHarness.js`（纯事实出口：目录枚举、行数、深度、接线对账、零孤儿、allowlist 查询，并供给 `ROOT` 与 `readText`）；新增 `utils/guardAllowlist.js` 并把守护脚本改为从它取；新增一条声明面自检（双向对账 + 跨模块唯一 + 注入违规自检）。**此票不删任何既有断言**，故全绿。

**票 B —— 消掉唯一一处活违例**：`api/forum.uts` 618 → ≤600（forum 模块预算执法面随之上线）。**本票动运行时面（`.uts`）**，单独走四门。

**票 C —— 已达标模块迁到声明**：`courses / dashboard / exam / forum / jobs / mall / points / practice / profile / recruiter / resources / resume / search` 十三份模块契约删各自复写的 `ROOT` / `read` / `walk` / `600` / allowlist 解析，保留本模块独有断言（行为保持点、删除禁区、接口对账）；**每删一条重复断言须在 PR 描述里列出**，任何**弱化**须引用 issue（0007 纪律）。**不动运行时面**。

**票 D —— 未达标模块登记入表 + 全表对账开跑**：10 个未达标模块补齐 `pending` 行的文件清单，跑一次全表对账（含跨模块唯一性）。

**依赖**：A 无依赖 → B、C 依赖 A → D 依赖 C 的删除动作定型。B 与 C 可并行（B 动运行时面、C 不动）。

## ⑤ 约束（不能动的）

- **读取层归一是唯一真源**：`readText` / `normalizeEol` 只在 `utils/utsHarness.js`，本决策的任何新读者都必须消费它（0019 已把这条钉成硬约束）。
- **接口不变**：`utils/utsHarness.js` 的既有导出（`loadUts` / `importedNames` / `exportedNames` / `normalizeEol` / `readText`）签名不动；新增面只加不改。
- **模块边界是显式决定**：在模块目录里新增文件后必须同步声明；缺登记由 harness 判红并直接指出该加哪一行——**不得**以放宽对账来消红。
- **不得引入第二份模块清单**：`modules.js` 是唯一事实源；测试里不得再出现硬编码的模块文件清单（迁移未完成的模块除外，且以 `pending` 显式登记）。
- **`budgetOverrides` 须带理由与 issue 引用**；不得成批使用它把未达标模块「买绿」（那正是本决策要关闭的失效形态）。
- **确认机制不得依赖平台**：对账与自检在 ubuntu 与 Windows 上结论必须相同（同 0019 §⑤，避免只在 Windows 暴露的假绿）。
- **不重开已决**：0019 的读取层归一、`docs/agents/guards.md` 的守护分类、0008 的门语义、0007 的 UI 冻结与「禁删/弱化断言」，一概不重开。

## ⑥ 验收标准

1. **唯一真源**：`utils/modules.js` 是模块归属 / 预算 / 深度的唯一声明；迁移完成后全仓 `utils/*Contract.test.js` 里 `path.join(__dirname` 命中数 = 0。
2. **判别力（注入自检）**：① 从声明删掉一个归属目录下的真实文件 ⇒ 对账判红；② 声明里写一个不存在的文件 ⇒ 判红；③ 同一文件被两个模块声明 ⇒ 判红；④ 某模块预算改小到低于实况 ⇒ 预算判红。
3. **活违例清零**：`api/forum.uts` ≤600，且纳入 forum 模块预算的执法面。
4. **零回归**：`npm run test:unit` 全绿（110 个测试文件），且 `node scripts/classify-guards.mjs --json` 的分类计数前后一致。
5. **不新增阻塞单点**：`mobile-test` job 的步骤清单不变；`ci-summary` 的 `needs` 不变。
6. **例外可见**：任何 `budgetOverrides` 都能在 `modules.js` 里一眼看到理由与 issue 引用。

---

## 附：现状（实测，出处可复核）

实测基准 = `origin/master` @ `20e199e0`（2026-09-20 22:21），在基于该提交的干净 worktree 上逐文件量取。

| 事实 | 值 | 口径 |
| --- | --- | --- |
| 移动端测试文件 | **110**（契约类 **77** / 非契约 **33**） | `utils/*.test.js` 计数 |
| 测试总行数 | **23,758**（契约类 **16,234**） | 逐文件行数求和 |
| 契约测试里自建 `ROOT` | **75 / 77** | 匹配 `path.join(__dirname` |
| 契约测试里自建 `read()` | **41 / 77** | 匹配 `const read = (` |
| 契约测试里自建 walk / `*SourceFiles(` | **13 / 77** | 匹配 `function walk(` 或 `SourceFiles(` |
| 契约测试里出现 600 预算 | **15 / 77** | 匹配字面量 `600` |
| 契约测试里解析 `GUARD_ALLOWLIST` 源码 | **12 / 77** | 匹配 `GUARD_ALLOWLIST` |
| `pages/` 模块目录 | **23** | `pages/*/` 目录数 |
| **有模块契约的模块** | **13** | courses · dashboard · exam · forum · jobs · mall · points · practice · profile · recruiter · resources · resume · search |
| **无模块契约的模块** | **10** | ai-assistant · exam-info · featured · forgot-password · guide · index · login · notifications · profile-setup · register |
| 超 600 行的源文件 | **9** | login 870 · resume-edit 835 · ai-assistant 737 · register 639 · recruit.uts 628 · aiAssistant.uts 626 · forum.uts 618 · search 604 · recruiter/resume-detail 604 |
| 读取层归一的先例 | `utils/utsHarness.js#readText`，消费面 **98** 个测试文件 | 0019 票 A / C 的产物 |

> 口径说明：「模块契约」= `utils/` 里以该模块命名的契约套件（17 个文件名命中 13 个模块，`pointsRealApiContract` 之类自带限定词）。77 − 这套模块契约 − 守护/门脚本/域级契约 = 跨切面契约，**不在本决策的归属面内**，它们没有「模块」可声明。

## 关联

- **0007-渐进式重构手册** —— 600 软预算、契约测试纪律、「禁删 / 弱化断言」、UI 冻结；本决策是它的**机制化**（把各自复写的事实收成一份声明）。0007 §#914 已对门脚本家族做过同一动作（`New-GatePlan` → `-DryRun` 断言计划 JSON），本决策把该手法扩到模块面。
- **0019-契约测试换行符盲区与读取层归一** —— 读取层归一是本决策的前置依赖与唯一读者来源；其 §⑤「新门即全仓阻塞单点」的代价账是决策 ⑩ 的直接依据。
- **0008-移动端验收门与证据** —— ③ 门判据与「接线守护不构成 ③ 证据」的裁定；本决策不改其语义。
- **`docs/agents/guards.md`** —— 守护分类口径；本决策新增的对账与 harness 自检按**接线守护**归类。
- `utils/utsHarness.js` · `utils/modules.js`（新增） · `utils/contractHarness.js`（新增） · `utils/guardAllowlist.js`（新增） · `scripts/classify-guards.mjs`
- 来源报告：移动端架构评审（仅移动端范围）。**该报告的基线是落后 89 个提交的旧树，其数字作废，一律以本附录为准。**