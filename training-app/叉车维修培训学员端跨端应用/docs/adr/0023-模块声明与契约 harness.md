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

> **口径更正（2026-09-20，票 A #1217 实测，只增不删）**：上表「超 600 行的源文件」一行的数字是**非空行数**
> （PowerShell `Get-Content | Measure-Object -Line` 的口径），而**既有模块契约的 600 预算判的是总行数**
> （`readText(f).split('\n').length`，见 `coursesContract` 第 140 行一带）。两者在本仓差约 6%：
> `api/forum.uts` 总行 **654** / 非空 **618**。⇒ **票 B 的目标是「总行数 ≤600」**（约等于非空 565 行），
> 按 618 收工会留下 54 行的缺口。以 `utils/contractHarness.js#fileLines` 的口径为准。

---

## 实施记录

### 票 A —— 声明与机制（#1217，2026-09-20）

**落地物**：`utils/modules.js`（23 个模块的键 / 归属目录 / 必需文件 / 拆出物 / 消费者 / 预算 / 深度 / 豁免归属，纯数据）、
`utils/contractHarness.js`（纯事实出口：枚举 / 行数 / 深度 / 接线对账 / 零孤儿 / 消费者对账 / 豁免查询 / `reconcile(decls?)`）、
`utils/guardAllowlist.js`（`GUARD_ALLOWLIST` 的单一声明点）、`utils/modulesDeclarationContract.test.js`（自检 36 用例）。
**本票不执法**：既有断言一条未删；`utsAndroidCompile.test.js` 的 20 条规则原样不动，只把常量本体挪走。

**实测规模**：23 个模块 / 声明 **153** 个文件（目录内 **125** + 目录外的家 **37**，有重叠因共享件归主消费方）；
数字预算 **14** 个模块已锁定，`pending`（登记不执法）**9** 个。

| `pending` 模块 | 超预算文件（总行数） |
| --- | --- |
| ai-assistant | ai-assistant.uvue 839 · api/aiAssistant.uts 667 · ai-feature.uvue 629 · ai-settings.uvue 621 |
| forum | **api/forum.uts 654**（票 B 的唯一活违例；页面侧最大 583 已达标）→ **票 B 已收口（见下）** |
| login | login.uvue 977 |
| register | register.uvue 723 |
| resume | resume-edit.uvue 923（#1205 的 resume 手术已把它降到 426，同期把本行翻成 `BUDGET`） |
| recruiter | resume-detail.uvue 702 · api/recruit.uts 675 |
| search | search.uvue 702 |
| forgot-password | forgot-password.uvue 676 |
| points | task-center.uvue 617 |

**「未达标」的读法（本票的口径，供票 D 对齐）**：本票把 ②⑥ 的「未达标模块」读作**预算未达标**（声明面里存在超预算文件），
而**不是**「没有模块契约」。理由：后一种读法会把 `exam-info` / `featured` / `guide` / `index` / `notifications` / `profile-setup`
这 6 个**本来就在预算内**的模块也写成 `pending` —— 那是白送一条「不执法」，与 ②⑥ 要治的失效面同形。
故这 6 个**无契约模块已被本票锁进 600**（票 D 若要按「无契约」清点，这 6 项的状态是「已锁」而不是「待补」）。

**三处实测更正/发现**：

1. **行数口径**：见上「口径更正」。ADR 附录与本票的执法口径不同，票后一切数字以 `fileLines`（总行数）为准。
2. **③ 的 `search` 消费者未成立**：`components/ai-chat/**` 归 `ai-assistant` 已照做，但**实测 `pages/search/**`
   未 import 该目录任何文件**（`search.uvue` 只 import `components/app-chip` 与 `components/app-empty-state`）⇒
   登记面照实测写 `crossModuleConsumers: []`。**决策意图不以假声明保留**，改由对账兜底：
   `consumerFacts().unregisteredConsumers` 会在 `search` 真接线那天判红并要求登记。
3. **同名常量撞车**：`utils/navQueryKeyContract.test.js` 自有一份 `const GUARD_ALLOWLIST = []`（导航 query 键例外表，
   与守护规则豁免无关）⇒ 已改名 `NAV_QUERY_ALLOWLIST`；否则「全仓只有一个声明点」是**假命题**。

**边界写实（本票不处理，别当成已覆盖）**：跨切面基础设施**不在任何模块的归属面内** ⇒ 它们的行数不在预算面内。
其中 `api/request.uts` **611 行**是全仓唯一一处「超 600 行且无人认领」的源文件（`api/auth.uts` 508 / `api/helpers.uts` 72 等未超）。
归属面外的源文件共 58 个（`App.uvue` / `main.uts` / `types/**` / `utils/**` / `components/app-*`）。

**归属规则（写给票 C / 票 D）**：键 = `pages/<目录名>`；目录外的家（域 api、共享组件、共享 composable）**归主消费方** ——
① 该域有既有模块契约 ⇒ 归该契约模块；② 唯一消费者 ⇒ 归它；③ 多消费且无契约 ⇒ 归「页面所在模块」并在行内写理由。
其余消费者登记在 `crossModuleConsumers`（`utils/modules.js` 文件头有全文）。

**落锁**：`utils/modulesDeclarationContract.test.js` **A1–A12 / B1–B15 / C1–C5 / D1–D4**（36 用例）——
A 组守真源与磁盘一致（模块键与文件**双向对账**、跨模块唯一、深度、拆出物零孤儿、零死引用、豁免归属、消费者双向、预算、覆盖合法性），
B 组是**成对取证的判别力**（13 条注入各自「改坏必红 + 真实文件必不红」），C 组守 `GUARD_ALLOWLIST` 单点，D 组守 harness 自身约束。
分类器判为 **接线守护**（读文件清单与源码文本，不执行被测物）⇒ **不构成 ③ 门证据**（`docs/agents/guards.md`），
行为面由 B 组的注入自检自证。**未新增阻塞单点**：`mobile-test` 步骤清单与 `ci-summary.needs` 均未动，自检随 `npm run test:unit` 一起跑。

**门证据（本 PR）**：③ `111 suites / 2028 tests` 全绿（基线 110 / 1992，新增即本套件 36 例）；
`node scripts/classify-guards.mjs --json` = **17 行为 / 94 接线**（改动前 17 / 93）——**既有 110 个文件的分类零变化**，增量只有本守护。

**票 C 的入口**：`contractHarness` 已供给 `ROOT` / `read` / `readText` / `declaredFiles(key)` / `moduleOnDisk(key)` /
`fileLines` / `moduleDepth` / `orphanExtracts` / `deadImports` / `allowlistPaths` / `reconcile()`；
C 要删的 `path.join(__dirname`、自建 `read()`、自建 walk、写死 600、解析 allowlist 文本五类复写都有对应出口。

### 票 C —— 13 个已达标模块的契约迁到声明（#1219，2026-09-21）

**落地物**：改 **14 份测试文件**（13 个模块；`recruiter` 域有 `recruiterResumeContract` + `recruitWorkspaceContract` 两份）。
删掉的复写：`path.join(__dirname…)` 自建 ROOT（14/14）、自建 `read()`（12 份）、模块 walker
（`*SourceFiles()` / `collectFiles` / `walkUvue` / 内联 `walk`，共 11 处）、模块级 600 预算与目录深度断言（见下表）、
以及随之变成死代码的硬编码模块文件清单（`courses` / `exam` 的 `REQUIRED_SOURCE_FILES`、`resume` 的
`REQUIRED_SOURCE_FILES` + `BUDGET_FILES`）—— 后三者正是 ⑤ 禁的「第二份模块清单」。
allowlist 断言不动（票 A 起已是数据读取）。

**删掉的断言（逐条可复核，机器抽取 `it|test` 标题前后对照；共 18 条）**：

| 文件 | 模块级预算（类 1） | 目录 ≤2 层（类 2） | 必需文件清单（类 3） |
| --- | --- | --- | --- |
| coursesContract | ✅ | ✅ | ✅ |
| dashboardContract | ✅ | ✅ | — |
| examContract | ✅ | ✅ | ✅ |
| forumContract | **⚠️ 保留（见下）** | ✅ | — |
| mallPilotContract | ✅ | ✅ | — |
| practiceContract | ✅ | ✅ | — |
| profileContract | ✅ | ✅ | — |
| resumeContract | ✅ | ✅ | ✅ |

（`jobsMineEntry` / `pointsRealApi` / `recruiterResume` / `recruitWorkspace` / `resources` / `search` 六份**本来就没有**这三类断言，故零删除。）

**`forum` 的例外（本票唯一的判断，理由写实）**：`utils/modules.js` 里 forum 的预算是 `pending`（因为
`api/forum.uts` 654 行还超着 —— 那是票 B #1218 的活）。声明面 A10 对 `pending` 模块**不判红**，
所以此刻删掉 `forumContract` 那条 pages-only 预算检查，等于让已达标的 `pages/forum/**`（最大 583）
**失去唯一的预算锁** ⇒ 保留该条（已换成 harness 出口），并在原位写了理由。**票 B 把 forum 翻成数字预算后，
这条即为重复，应删**。同理核对过：`points` / `recruiter` / `search` 虽也是 `pending`，但它们没有类 1 断言，删无可删。
（**合并票 D #1227 后复验**：forum 仍是 `pending`、`api/forum.uts` 仍是 654 ⇒ 该例外依然必要，不是过时判断。）

**强度核对（为什么删了不弱化）**：数字预算模块的声明面 = 全部归属文件（目录内 + 域 api），
比原来各文件只扫 `pages/<key>/**` **更宽**（例：`profile` 21 → 24、`practice` 11 → 13；`courses` / `exam` / `resume` 相等）；
深度（A5）与必需文件（A3 双向对账）与预算是否 `pending` **无关**，故类 2 / 类 3 在任何模块都可安全删除。

**写实的三处微小口径差**（都已逐条核对为等价，记录备查）：① 单目录 `fs.readdirSync` 换成
`h.sourceFilesIn` 后，非源码杂项文件不再参与「目录里只剩 N 个文件」这类等式断言；
② `recruitWorkspaceContract` 的全仓枚举由自建 `collectFiles` 换成 `h.filesUnder` —— 扫描面略窄
（多跳过 `hybrid` / `.hbuilderx` / `.vscode` / `coverage` 等目录，本机实测这些目录**不存在**，故当前等价）；
③ 单目录列表里 `*.test.*` 被剔除（这些目录里没有测试文件）。

**门证据（本 PR）**：③ `npm run test:unit` = **112 suites / 2079 tests 全绿**（= 本票基线 **2086** − 删掉的 **18** 例
+ 票 D 并入的 E 组 **11** 例；2086 由本 worktree 实测：把 14 份文件 `git stash` 回改前逐文件比对得 **609 → 591**，
差 **18** 恰等于上表删除条数 —— 没有连带丢用例）；
`node scripts/classify-guards.mjs --json` = **112 总数 / 17 行为 / 95 接线**，与改动前**逐文件零变化**；
`node scripts/check-contract-read.mjs --all` = **exit 0**（112 个测试文件无未归一的仓内源码裸读）。
四门 **免（未命中运行时面）**：改动集只有 `utils/*.test.js` 与本文件。

**与票 D 的并存**：两票都改本段（实施记录），合并时按 **A → C → D** 顺序保留双方原文，不删不改对方内容。

### 票 D —— 全表对账：每个源文件都有归属（#1220，2026-09-21）

**本票要治的是 A 留下的那条缝**：A 的对账只覆盖**模块归属面**（`pages/<键>/**` + 显式登记的目录外的家），
于是**没进任何表的目录外文件是隐形的** —— `api/forum.uts` 当年正是这样躺在自称达标的 forum 模块里。
A 把**已知**的域 api 逐个登记了，但「下一次有人加一个目录外文件」仍然不会红。

**做法**：`utils/modules.js` 新增 `INFRA`（`dirs` 14 个 + `files` 6 个 + `oversized`），
`utils/contractHarness.js` 新增 `infraFiles()` / `infraFacts(decls, infra)` 并把五面并入 `reconcile(decls, infra)`：
`unregisteredSourceFiles`（**树上没归属的源文件**）· `infraPhantomDirs` / `infraPhantomFiles`（幽灵登记）·
`infraOverlaps`（既归模块又登记为基础设施）· `infraOversizedDrift`（超预算的跨界文件**双向**对账）。
判据：**树上的每个源文件，要么归某个模块、要么登记为基础设施，没有第三种** —— 实测 **218 个源文件 = 模块面 157 + 基础设施 61**。

**登记不执法**：基础设施的行数**不进**任何模块的预算面（ADR-0023 ① 的边界：它们的预算归各自独立的票）。
唯一一条超预算的跨界文件是 **`api/request.uts`（611 行）**，由 `INFRA.oversized` 显式登记、**本票不拆**
（同模块 `pending` 的口径）—— 它从「没人看得见」变成了「表上一行」。

**顺带的两处登记更正**（A 的归属规则当时只施加于域 api，漏了这两个）：`api/faq.uts`（帮助中心）与
`api/note.uts`（笔记）的唯一消费方都是 `profile`，按归属规则②**归 profile**（不再算基础设施）。

**规则适用面的澄清（写进 `modules.js` 文件头）**：归属规则①②③**只管域 api 与模块私有拆出物**；
`utils/**` / `types/**` / `stores/**` / `constants/**` / `config/**` / `components/app-*` / `uni_modules/**` /
`App.uvue` / `main.uts` 按**共享件**定位 —— 即使某个文件只有一个模块消费（例 `utils/forumDisplay.uts` 只被
forum 用）也不归它，而是登记为基础设施（模块契约里的「展示纯函数唯一实现」断言正是把它们当共用的唯一实现面）。

**「未达标」读法的延续**：票 A 把 ②⑥ 的「未达标模块」读作**预算未达标**（而不是「没有模块契约」），
故 10 个无契约模块里已在预算内的 6 个被直接锁进 600、另 4 个（ai-assistant / forgot-password / login / register）
登记 `pending`。本票维持该读法：**不**为这 6 个模块补写 `pending`（那等于白送一条不执法），并在票面记录了差异。

**落锁**：`utils/modulesDeclarationContract.test.js` 新增 **E1–E11**（全表覆盖面非空 / 零隐形文件 / 幽灵与重叠 /
超预算双向 / 6 条注入各自成对取证 / 报错信息指出文件）。**套件数不变**（只加用例、不加文件）。

**门证据（本 PR）**：③ `112 suites / 2097 tests` 全绿（基线 112 / 2086，增量即 E 组 11 例）；
分类计数 **112 / 17 行为 / 95 接线** 与改动前**逐项一致**。
### 票 B —— `api/forum.uts` 拆分至 ≤600 并上线 forum 预算执法（#1218，2026-09-21）

**落地物**：新增 `api/forumDto.uts`（DTO 构造层：`extract*` 4 + `build*` 5，共 9 个函数，**全部逐行照搬**，
只加 `export ` 前缀）；`api/forum.uts` 从 **654 → 413** 行（请求形态段**逐字未动**：20 个导出函数、路由、查询参数、
出口选择全原样）；`utils/modules.js` 的 forum 条目登记 `api/forumDto.uts` 并把预算从 `pending` 翻成 `600`（**执法上线**）。

**拆缝**：`响应 shape 的构造` ／ `请求形态` 之间。判据是机械可复核的 —— 用脚本比对拆分前后：
构造层 221 个非空行**逐行一致**（只差 `export ` 前缀），请求形态段 397 行**逐字未动**。
`utils/forumContract.test.js` 的**字段级**断言（`is_featured` / `is_experience` / IP 属地两字段 / 分页三元组 /
被回复人两字段 / `buildTopic` 各字段）改读 `api/forumDto.uts`（**等价强度**：同一条 `toContain` 换个读取目标），
另加 4 条**拆分锁**（请求侧必须从 `./forumDto` 取构造层、请求侧不得再自带 `function build*|extract*`、
构造侧九个函数都在且零请求出口、两个文件都 ≤600）。

**收口票 C 留下的例外**：票 C 因 forum 预算是 `pending` 而保留了 `forumContract` 的 pages-only 预算检查
（见票 C 段）。本票把 forum 翻成数字预算后，该条与声明面 A3/A10 完全重复 ⇒ **按票 C 自己写下的约定删除**。
预算现由声明面对**声明全集**执法（`pages/forum/**` + `api/forum.uts` + `api/forumDto.uts`，最大 583），比原条更宽。

**新坑位（本票实测，写给后来的预算执法面上线 PR）**：**「预算执法面上线」在结构上不可能拿低风险运行时面豁免**。
原因有两条，同时成立：
① 预算在 `utils/modules.js` 里，而它**不在** `pr-evidence` 的低风险白名单（`*.uts` / `*test.js` / `*.md` / `jest.config*.js`）里
⇒ 只要 PR 带它，整单降级为**常规运行时面**（① 必过）。
② 票 A 的 **A10** 判据要求「`pending` 模块必须真有一个超预算文件」⇒ 拆分把 654 降到 413 的那一刻，
`pending` 就成了假命题、③ **必红**，所以拆分与翻预算**必须在同一个 PR 里**，不能拆成两单。
⇒ 结论：这类 PR 按**常规运行时面**走门（本票即如此），**不要**按「仅 .uts 逻辑改动」估成本。

**门证据（本 PR #1229）**：③ `112 suites / 2090 tests` 全绿（基线 112 / 2086，增量即 forum 的 4 条拆分锁）；
④c `KOTLIN_ALL_RESULT errors=0 classes=1522 files=118`；①a 真机 `23049RAD8C` / Android 15 逐页截图
（列表 / 详情 / 打卡三页，入仓 `docs/verification/forum/1229/`）+ logcat 本窗口 **0 FATAL / 0 ANR**，
应用侧日志证明改后调用链真跑通（`getForumTopicsApi` 4 条列表、`getForumTopicDetailApi` 6 条回复，
`is_featured` / `is_experience` / `ip_province` / `parent_name` 等由被搬走的 builder 映射的字段全部上屏）；
② **免（未命中 MP-WEIXIN 面）**。分类计数 112 / 17 行为 / 95 接线，与改动前**逐项一致**。
**①a 范围写实**：`buildLikeResult`（点赞/取消）未经 UI 触达（回复点赞节点未在可视区命中）⇒ 由「逐行一致」+
模块契约的出口断言兜底；`forum-create` / `my-forum` 未逐页取图（本票是 API 层纯搬家，页面模板与样式零 diff）。

## 关联

- **0007-渐进式重构手册** —— 600 软预算、契约测试纪律、「禁删 / 弱化断言」、UI 冻结；本决策是它的**机制化**（把各自复写的事实收成一份声明）。0007 §#914 已对门脚本家族做过同一动作（`New-GatePlan` → `-DryRun` 断言计划 JSON），本决策把该手法扩到模块面。
- **0019-契约测试换行符盲区与读取层归一** —— 读取层归一是本决策的前置依赖与唯一读者来源；其 §⑤「新门即全仓阻塞单点」的代价账是决策 ⑩ 的直接依据。
- **0008-移动端验收门与证据** —— ③ 门判据与「接线守护不构成 ③ 证据」的裁定；本决策不改其语义。
- **`docs/agents/guards.md`** —— 守护分类口径；本决策新增的对账与 harness 自检按**接线守护**归类。
- `utils/utsHarness.js` · `utils/modules.js`（新增） · `utils/contractHarness.js`（新增） · `utils/guardAllowlist.js`（新增） · `scripts/classify-guards.mjs`
- 来源报告：移动端架构评审（仅移动端范围）。**该报告的基线是落后 89 个提交的旧树，其数字作废，一律以本附录为准。**