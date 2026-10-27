> 本文件是移动端 `AGENTS.md`「开发内循环（移动端 UI 迭代）」与「Qoder 托管 worktree 的使用边界（2026-09-28 立）」两节的**按需层全文**（#1509 根线式分层）。
> 常驻层只留判据与对策，**实测数字、时间账与坑位叙述在这里**。
> 另含三节常驻层没有全文的家：**「全量 ③ 门落盘与入口」**（`npm run test:unit:gate` 那条命令的入口指针，#1546 落地、#1556 登记；#1702 起该入口跑**两腿** —— `test:unit` 接线 + `test:behavior` 行为）、
> **「全量 ③ 门该不该跑」**（常驻层「成本账反模式」那句指针所指的判据全集，#1556 立）
> 与 **「agent 判读纪律（慢≠卡死 / 机检行 / 重跑半径）」**（一条命令在跑 / 跑完 / 跑红之后的读数判据，#1656 立；**不复述门脚本细节**）。
> **引用本文件时引小节名，不引行号。**内容从常驻层逐字搬入，口径未变。

# 开发内循环与工作树边界（按需层）

## 什么时候读哪节（按票只读，别整读）

| 要做的事 | 读这节 |
| --- | --- |
| 想知道时间花在哪、慢的是哪一样 | 时间账与三样慢 |
| 跑 `hx:run` 真运行（收口取证前部署） | 真运行会话常驻与收口 |
| 问「为什么移动端的活不能在宿主树做」 | 三条原因的实测原文 |
| 想重跑门之前（贴证据之前先判改动集） | 成本账反模式 |
| 要跑全量单元测试门、取结果行 | 全量 ③ 门落盘与入口 |
| 判「这次到底要不要跑全量门」 | 全量 ③ 门该不该跑（C2 四档） |
| 门在跑 / 跑完 / 跑红，怎么读数 | agent 判读纪律 |
| 连真机做逐页取证 | 真机无线调试入口 |
| 建树 / 用树遇到怪问题 | 工作树与会话坑位原文 |
| 要用 `hx-run --compile` 参数 | `--compile` 语义坑位（血账 #917） |
| 改完样式要一行结论 | 样式内循环 |

## 时间账与三样慢（原文，2026-09-13 实测，多次取数）

**时间花在哪（2026-09-13 实测，多次取数）**：基座 APK（95.7 MB）**只装一次**；**编译 205–261 秒**（冷启 HBuilderX 那次 610 秒）、
**推送 + 启动 15–21 秒**、一轮真运行合计 **236–287 秒**；全量编译 **945 秒（≈15.75 分钟；2026-10-26 实测 `COMPILECHECK_WALL_SEC=945`、ready in 932957 ms，head 4b37aa05，#1989；复跑 966 秒，#1997）**（旧口径 **4.8–8 分钟**（2026-09-13 实测）**已退役**，#2002）。
旧口径的「增量约 1–2 分钟」**只覆盖编译段**，「约 10–11 分钟」是拿一次冷启 + 9 分钟推送的极值当常态 —— **两个都别再引用**，以本段为准。
**慢的从来不是编译，而是这三样（同日一笔账）**：**返工**（编译期诊断拖到真机才发现，两次各约 5 分钟）、
**排队**（等另一会话的 `hx-agent.lock`；旧口径「一次约 10 分钟」（2026-09-13 实测）**已退役**，#2002——2026-10-26 实测 125 次取锁：wait=0 ×89、1–28 s ×28、454/501/592 s 各 1、120 s 超时 ×1，均值 15 s、中位 0 s，#1987）、**卡死**（launch 挂住，白等约 20 分钟才到上限）。
后两样现在都有对策：**忙等待默认 600→120 秒**、**部署停滞 300 秒即提前判环境不可用**（`-DeployStallSeconds`）；
返工的对策是**诊断走 `hx:compile-only` 拦在本地**。
`hx-run.ps1` 会打印分段表与机检行 `HX_RUN mode=… compile=… deploy=… total=… exit=…`，另加部署判据行 `HX_RUN_DEPLOY deployed=… www=… pid_after=…`。

## 真运行会话常驻与收口（原文，2026-09-13 实测）

**真运行会话常驻，但下一次 launch 会顶掉旧的（2026-09-13 实测）**：不传 `--compile` 的真运行 `cli.exe` **不会自己收口**
—— 实测一个会话活了 **51 分钟**，期间每约 10 分钟重试一次 `wakeUpDevice`（被设备以 `INJECT_EVENTS` 拒绝，**非致命**）；
它最终是在**下一次 `launch` 发出后约 10 秒**打出 `已停止运行...` 才收口 ⇒ **不需要手工清理**。据此 `hx-run.ps1`
对 launch 步**不等待**：发出去 → 有界轮询设备侧事实 → 判完即返回（要提前停就在 HBuilderX 里点「停止」；脚本绝不 kill）。

## 三条原因的实测原文（2026-09-28 立）

三条原因（都是实测，不是偏好）：

1. **③ 门在宿主树里静默假绿，且这是永久属性。** 宿主落点是 `<userHome>/.qoder/worktree/…`，父段段首带点，正踩 #1144 的 jest-util glob 机制；**该落点不可配置**。实测同一份内容只差父段名：点父段 `--listTests` **0 套件**、无点父段 **141 套件**，且 0 那次 **exit 0、不报错**。CI 侧（ubuntu）不受此坑 ⇒ 最后一道防线还在，但**本地反馈环失效**，而 `.github/workflows/pr-evidence.yml` 顶部校验器的 P1 注释明写「判据只为『结构与 sha 绑定』，不校真伪」⇒ 把宿主树里的假绿抄进正文是过得去的。
2. **HBuilderX 那条不是宿主树专属的阻断（2026-09-28 更正）。** 取证时把项目目录改成唯一名是既定常规动作（先例见 `docs/verification/*/*/README.md` 里的 `fl-mobile-wt<票号>` 命名，逐例可复算、不活标数量；同名项目数现算：`cli project list` 数同名条目，别照抄历史数字）；真正的代价在「改回来会被锁卡住」（`docs/agents/multi-agent-git.md` 的「Windows 上用 worktree 的注意事项」→ HBuilderX 唯一项目名那条），而**这个代价闸门树同样有** ⇒ 它不构成「移动端不用宿主树」的理由，第 1 条才是。
3. **树会被自动回收。** `worktreeMaxCount`（现值**读** `%APPDATA%\Qoder\SharedClientCache\cache\app-config.json` 的 `worktreeMaxCount` 键，别冻结一个数字）超限即删最旧，**包括别的编辑器正开着的那一棵**；`git worktree remove` 还会连带删掉 gitignored 的 `.ci-verify/`（原始判据输入，删了不可复算）。

**回退与复用的边界**：宿主树是普通 git worktree ⇒ **提交并推了分支就能回退**，任何编辑器/命令行 `cd` 进去都能接着干（`git worktree list` 一定列得出它）。反过来，**没提交的改动与 gitignored 证据，树没了就没了**。所以进树第一件事是建分支、尽早推 origin，判据输入当场拷到树外或直接入 `docs/verification/<模块>/<PR号>/`。

**初始化单一真源**：`pwsh training-app/叉车维修培训学员端跨端应用/scripts/wt-bootstrap.ps1`（主树位置由 `git rev-parse --git-common-dir` 反推，不依赖任何工具私有变量）。**宿主「本地任务的 Worktree 配置」框由宿主维护、PR 审查不到、本仓文档不校验其现值**：框里**必须且只能填上面这一行调用**——填逻辑就等于开了第二真源。要确认它已同步，读回 `%APPDATA%\Qoder\logs\<run>\questWindow\renderer.log` 里 `worktreeSetup` 键的值与上面这行比对（**别拿「应该已填」当作已填**）。

## 成本账反模式（2026-10-03 立）

- **sha 一无害前进就重跑整道门**（2026-10-03 立，实测账见 `docs/design/1494-mobile-dev-wallclock-audit.md`）——绑定判据的**真源是 `.github/workflows/pr-evidence.yml` 的校验器源码**（2026-09-12 放宽那次就是为了「免去 master 前进一次就重跑重贴的 churn」，PR 模板也照此写），本条**不复述那份文件清单**（复述品会漂移，模板顶部已为此写过警告）。本条只补**手法**：贴证据前现测一次 `git diff --name-only <旧证据sha> <head>` 看改动集，判据不满足才重跑；**「我觉得没改」不算判据**（校验器对 compare 拿不到 / 超大 diff 截断是 fail-closed，那种情况仍须重跑）。**为什么还要立这条**：放宽早于本仓的实测返工——#1443 为绑 `c3adc58` 重跑了一次完整 ④c（publish 53 s + kotlinc 全模块 ≈2.9 min），而其间只动了一个 `docs/verification/*.jpg`；#1467 的 merge 只带进两个 Go 域搬包，② 与 ④c 仍各重跑一次。判据改了，取门侧没有跟着改，默认动作就退化成「重跑」。
- **② 门失败后不带 `-SkipBuild` 重试**（同上账本）——② 的失败轮实测 **9–26 分钟/次**（旧口径已退役，#2002；现测 mp-weixin-check 单次 126–327 s、中位 ≈240 s，#1987；`mp-weixin-check.ps1:6` 自述的「≈4.5 分钟」是**成功那次**、不是预期值），构成是 `close→open→auto` 整段重试（`-AutoAttempts`，默认 3）里**每轮都重新 publish**。产物没变时，第二次起带 `-SkipBuild` 复用产物重试**实测 ≈2 分钟**（#1443 01:14→01:16、#1422 第三次靠它才过）。**边界**：门结论只对当前那份产物成立（脚本 `:33` 自述）⇒ 改动碰了 `mp-weixin` 构建面就不许带，宁可重编。

## 全量 ③ 门落盘与入口（#1546 落地 · #1556 登记指针）

**入口 = `npm run test:unit:gate`**（= `pwsh scripts/dev-finish.ps1 -UnitGate [-PostToPr <PR号>]`；命令注册在
`package.json`，本体是 `scripts/dev-finish.ps1` 的 `-UnitGate` 分支）。#1702 之后它跑的是**两腿门命令本身**
——`npm run test:unit`（接线腿）与 `npm run test:behavior`（行为腿），分支里不抄第二份 jest 参数，
**只多干一件事：把那一次跑的痕迹留在盘上，让人能引用它。** 两腿都跑完才退出，退出码取**先红者**
（一条腿红不掐掉另一条腿的判读信息 —— 与 CI 的 `fail-fast: false` 同一取舍）。
**入口的第一行就是预期时长**（#1702 验收 ③）：慢 ≠ 卡死，读者一开始就该知道多久算正常。

**为什么入口要写进文档**：#1546 把落盘件合进 master 之后，`test:unit:gate` 这个串在 tracked 文件里只有三处命中
（`package.json` 的 scripts、`dev-finish.ps1` 头部的 `.EXAMPLE`、贴评论时的复现行），两份 `AGENTS.md` 与
全部 `docs/**` **零命中**（2026-10-05 现测）⇒ 门有出口但没人指路，下一个会话照旧选「再跑一遍」。本节就是那个家。

**产物（跑完就在，绿跑也在）** —— 两腿各一份，路径里带腿名：

- `.ci-verify/test-unit.log` / `.ci-verify/test-behavior.log` —— 完整输出**原文**，机检行追加在尾部
  ⇒ 一份文件同时是物证与结论。行为腿那份**自带逐套件心跳行**（`JEST_SUITE i=k/N status=… secs=…`）
- `.ci-verify/test-unit-failure.txt` / `.ci-verify/test-behavior-failure.txt` —— **只随红跑**产出的人读摘要
  （≤2 KB，不作门禁输入）；绿跑不产，而磁盘上若还留着上一跑的红摘要，
  机检行以 `staleSummary=<mtime>` 指名「那不是本次结论」
- 出 PR 的出口：`-PostToPr <PR号>` 按 ②④ 的同格式贴 **sha 绑定**评论（红跑才附摘要正文；两腿的机检行与
  摘要正文都进同一条评论，标题写「两腿」）

**机检行的读法**（`key=value` 一行，字段名固定；**两腿各一行，首 token 不同 ⇒ 机检可区分**）：
`TEST_UNIT_RESULT exit=… suites=… suitesFailed=… tests=… testsFailed=… time=… wall=…s log=… summary=… head=<sha> dirty=clean|dirty`
`TEST_BEHAVIOR_RESULT exit=… suites=… …log=.ci-verify/test-behavior.log …`（行为腿同款字段）

- `exit=` 是**唯一判据**（jest 的退出码原样透传；摘要、机检行与下面两枚都不参与任何判红路径）
- `head=` ＝ 那一次跑所在的 HEAD sha；`dirty=` ＝ 当时**整棵工作树**（含未跟踪文件）有没有未提交改动
- ⚠️ `dirty=` 报的是**整棵树**，不是「本票的改动集」：实测 `git -C <移动端目录> status --porcelain` 与在仓根跑
  **逐字节相同**（`--porcelain` 恒输出仓根相对路径，也不按 cwd 收窄）⇒ 多会话共享一棵树时，别处
  （`frontend/`、根 `.ci-verify/`）的未提交改动同样把它读成 `dirty`。它的用途是「`head=` 相同**不代表内容相同**」
  这一条警示，不是「我这票脏不脏」的读数。
- `dirty=` 是**枚举**：只认 `clean` / `dirty` 两个逐字节字面值，比较用 `-ccontains`（区分大小写）。
  大小写变体（`CLEAN`）归一为 `?` —— 同一枚字段出现两种写法时，按 `dirty=clean` 收窄读的人会整腿漏读。
- 取不到写 `head=?` / `dirty=?`。`?` 是「没读到」，**不是**「没有改动」——把它读成 clean 就是把「看不见」
  换成了「看得见但看错」；字段本身**永远在**（省略与补空串都不许，与 `missing=` 同一口径）
- 数字全部取自那一跑自己的 `Test Suites:` / `Tests:` / `Time:` 汇总行 ⇒ 拿同一份日志能复算出同一串

⚠️ `.ci-verify/` 是 gitignored，且**随工作树回收一起消失**（见上面「三条原因的实测原文」第 3 条）⇒ 能被下一个
会话引用的只有两处分发出口：入 `docs/verification/<模块>/<PR号>/` 的产物，或 `-PostToPr` 贴出的那条评论。
「日志还在」的前提是**树还在**。

守护：`utils/devFinishContract.test.js` 的 U 系列 —— U1–U10 钉 #1546 那件产物的形状与可复算性，
U11–U14 钉 `head=` / `dirty=` 的行为（种进去的值要出现、取不到要写 `?`、非法值与大小写变体归一、
两枚不产出绿/红结论、返回的那两枚必须与结论行同串），U15 钉 #1702 的腿进首 token
（默认腿逐字节仍是 `TEST_UNIT_RESULT`，`-Leg behavior` 发 `TEST_BEHAVIOR_RESULT`，与 `-Mode local` 叠加成
`TEST_BEHAVIOR_LOCAL_RESULT`）。
U8 钉调用点接线（取法只有共享库那一份；`dirty=` 的值取自 `git status --porcelain` 实读而不是常量；
两腿跑的都是 `package.json` 那两条字面命令、**且必须边跑边吐** —— 见下「判读纪律」里 `Out-String` 那条 ——
这一条是**接线**，不构成 ③ 的行为证据，行为面看 U11–U15 与真链路那两腿）。
L 系列钉 `-Mode` / `-Leg` 两维的机检可判读性（L1a–L1c 成对腿、L2 脏输出判读、L3 快路径两腿接线与合成退出码）。


## 全量 ③ 门该不该跑（C2 四档判据，2026-10-05 立 · #1556）

**这一节的来路**：常驻层那条「成本账反模式两条 ⇒ 判据与实测数字在 dev-loop.md」的指针落在上面
「成本账反模式（2026-10-03 立）」一节——那一条讲的是「贴证据前现测一次 diff 再决定重跑」，
本节把同一件事扩展到「要不要跑第一次」，并把四档的输入写死到可核。
⚠️ 两节**中间还隔着**「全量 ③ 门落盘与入口」那一节（#1556 同批登记的入口指针，2026-10-05 插在两者之前）⇒
按节名找，别按「紧邻下一节」找（复核 #1556 时此句为假，已改掉；这也正是本仓「引小节名不引行号」那条约定的原因）。

**先划边界，这一节只管「本机跑几次、跑过的痕迹能不能被引用」，不管「门要不要过」。**
③ 仍是 ③；CI 的 `mobile-test` 仍会为**每一个**动到 `training-app/**` 的分支在那个 head 上再跑一遍这道门
（判据源 `.github/workflows/ci.yml`：`changes` job 里 `mobile` 那条过滤器对目录下**任何**文件都为真——含只改一份
文档；job 本体、它的 `if:` 与那行 `run: npm run test:unit` 在同一文件。⚠️ 触发面现测是**分支 push**
（`on.push.branches-ignore: [master]`），PR 事件不跑 CI，所以那句「CI 会再跑一遍」的准确说法是
「推分支就会跑」——结论不变，机制别写错）。命中运行时面仍走四门。
**把下面的判据读成「文档票免 ③」就是用错了**：本机不跑只是把结论的**来源**从「再跑一遍」换成
「引用上一次落盘的机检行」，CI 那道一模一样、判红照拦。

**判据一句话**：跑之前先答——**「这一次跑，能得出上一次那行结论里没有的什么？」**
答不出 ⇒ 不跑，引用上一次落盘的那行（读法见上一节）。

**输入只有三类既存事实**，都不出仓、都不靠会话记忆：

1. **改动集文件清单** —— 「落在哪个面」只认 `scripts/lib/level-detect.ps1` 的 `Get-DetectLevel`（🟢 / 🟡 / 🔴 三级），
   「③ 的射程」只认 `scripts/lib/contract-tests.ps1` 的 `Get-ContractTestPattern`（token 条数会随新守护增长 ⇒
   **别把条数当判据**，要用就现读那份返回值；它仍是收窄面的唯一真源）。
   **不新建第三份被测面清单**：双真源漂移在本仓有血账（#974，两份 `$testPattern` 漂了三个 token，
   起门禁作用的是窄的那份）。自查口径 = 新增文本里不得出现**并列的**扩展名枚举串——那份清单的所在地是
   `level-detect.ps1` 里 `$toolchainFiles` 那一行，别处再抄一份就是第二真源。
   ⚠️ **两件事不能拿 `Get-DetectLevel` 直接判**（2026-10-06 复核实测，别按字面猜它的射程）：
   ① 它只 diff `HEAD` 与暂存区 ⇒ 回答的是「**现在**有什么没提交」，不是「两个 sha 之间改了什么」——
   要范围差就自己跑 `git diff --name-only <A> <B>` 再套**同一套**分类，别另立清单；
   ② 它那条 🟢 分支的判据是「**至少有一个**工具链 / 测试 / 文档类文件」——哪些扩展名算一类，判据就是上面
   点名的 `$toolchainFiles` 那一行，本节不抄（抄了就是第二真源，正是这条要防的）。**「至少有一个」不等于
   「仅有」**⇒ 混合改动集也会落那一支，那句「未命中运行时面」因此**不能**当「没有被测代码」用
   （三份配置 json 另有先行的 `full` 分支兜住，`$configPatterns`，走不到这里）。
2. **上一次落盘绿跑的机检行** —— `.ci-verify/test-unit.log` 尾部那一行，含 `head=` 与 `dirty=`。
3. **CI 是否已为当前 head 跑过 `mobile-test`** —— 读 run 的结论，不靠「我记得跑过」。

四档各写成「输入 → 结论」：

- **C2a 只改了文档 ⇒ 本机不跑全量。** 输入 = 事实 1 的**两条一起**：改动集与 `Get-ContractTestPattern` 的射程
  **无交集**，且 `Get-DetectLevel` 报 quick。结论 = 引用上一次那行；**CI 那一跑一次不少**（上面那段边界），
  所以本机跑不跑**不改变合入判据**，只改变上下文体积。
  ⚠️ **只看 `Get-DetectLevel` 报 quick 就当 C2a 是用错了**（2026-10-06 复核立的修正；那一支是「至少一个」而非
  「仅有」，见上面事实 1 的第 ② 条）：只改 `scripts/*.ps1` 与 `utils/*.test.js` 的改动集**同样**落 quick，
  可那正是 ③ 自己的被测物 ⇒ 与射程**有**交集 ⇒ 归「该跑」那一档。反例就是本票自己：#1556 的改动集
  （`dev-finish.ps1` + `devFinishContract.test.js` + `docs/agents/dev-loop.md`）按修正前的措辞会被判「不跑」，
  而它必须先红后绿。出处账：#1545 §3 的 C2a 一档 13 次 / 3,422.3 s = 57.0 min。
- **C2b 只有 git / 建树动作 ⇒ 看 diff。** 输入 = 事实 2 里的 `head=` ＋ 当前 head ＋ 事实 2 的 `dirty=`：
  现测 `git diff --name-only <那次绿跑的 head> <当前 head>`，改动集按事实 1 的分类**没有**落到被测面 ⇒ 引用；
  落到了 ⇒ 该跑，跑完留下新的一行（新的 `head=` 才是下一次的基准）。
  ⚠️ **`dirty=dirty` 时 `head=` 相同不代表内容相同**——没 `git add` 的新文件不进 sha，只进工作树；
  这一腿正是 `dirty=` 这枚字段存在的理由。**「我觉得没改」不算判据**（与移动端
  `AGENTS.md`「反模式（逐条禁）」里那条「成本账反模式」同一口径，也即本节上面「这一节的来路」那句）。出处账：C2b 4 次 / 806.8 s。
  ⚠️ **反向那一半同样要看 `dirty=`**：`dirty=dirty` 的那一次绿跑，覆盖的是「`head=` ＋ 当时那批未提交改动」＝
  一份**内容**，而不是一个 sha。把那批改动**原样**提交成新 sha 之后，`git diff` 看着非空、内容却一个字没变
  ⇒ 仍然引用，别为「sha 前进了一格」重跑（本票收口时就走了这一读：脏跑在先、提交在后，diff 非空而引用成立）。
  若提交时夹带了新改的东西，那一腿就回到上面「落到了 ⇒ 该跑」。
- **C2c 什么都没动 ⇒ 引用。** 输入 = 事实 1 的改动集为空 ＋ 事实 2 在场。
  「想再确认一次绿」不是理由：成对取证里「必不红」那一腿现在由落盘产物 + 它的 `log=` 承担，不需要每次重落。
  出处账：C2c 6 次 / 904.3 s。
- **C2d 只动了别条面 ⇒ 与本门无关。** 输入 = 事实 1：改动集全部落在移动端被测面之外（Web / 后端 / 根文档）。
  结论 = 不跑，**且别把 ③ 当成交付判据去跑**——那条面的交付判据在它自己的线上。出处账：C2d 3 次 / 343.6 s。

**四档合起来就是本票立项的那笔账**：C2 家族 26 次 / 5,477.0 s = 91.3 min / 16,663 字符
（#1545 §3.1 与 §8.3 第 3 条，原句「这就是本读数要交给下一票的最主要一项」）。
**目标不是「省满 91.3 min」，是四档各有可核判据**——#1545 §10 第 5 条写明归因只在同一份转录内计数，
跨会话（A 会话改码、B 会话跑门）会漏 ⇒ 那 26 次是**上界偏乐观**。

**这条判据不新增第二个绿面**：`head=` / `dirty=` 只作可见性。「上次绿跑的 head 与当前不同就判红」这类实现
不在射程内——口径承接 #1546：机检行把传入的退出码**原样**写出来，不产出任何绿/红的新结论；该跑的那一档跑完，
判据仍然只是那一个 `exit=`。


## 行为腿长跑的环境性死亡与拼接读数（2026-10-24 立，#1845-#1848 复审修复会话实证）

行为腿全量（72 套件 `-i` 串行，~24-36 分钟）偶发**进程性死亡**：无 jest 汇总行、exit 1、两次复现死点固定（60/72 后、61 号开跑前）而死点套件单跑即绿——按**环境死亡**归因（长跑累积负载），不是用例失败。判据三件：① 日志有逐套件进度行（如 `JEST_SUITE`）而非全空；② 死点及之后套件单跑/小批跑全绿；③ 两次以上死点可复现且单跑绿。

处置（**拼接读数**，如实标注）：列出已完成套件清单 → 未跑到的尾段打包单跑 → 两份拼合 = 完整覆盖读数，PR/报告写明「全量 N 绿 + 尾段 M 套件批补 M 绿，合并覆盖 N+M、0 失败」。死亡本身（loadUts 累积？内存？）**值得调查票但别在交付票里恋战**——先拼接收口，调查另开票。


## agent 判读纪律（慢≠卡死 / 机检行 / 重跑半径）（#1656，2026-10-14 立）

**上下文只带结果行（2026-10-15 立，2026-10-18 补两腿）**：跑完门，进上下文的只有结果行（`TEST_UNIT_RESULT` / `TEST_BEHAVIOR_RESULT` / `MP_WEIXIN_RESULT` / `KOTLIN_ALL_RESULT`）；完整日志留在 `.ci-verify/*.log`，要细节按小节名回本文件读，**不把日志原文贴进对话**。

**这一节只管一件事**：一条命令在跑、跑完、或跑红之后**怎么读**。判据**不住在这里**——住在脚本与
`docs/adr/0036-工具链交付慢与怪错的三条处置.md`（移动端编号体系，须写全路径），本节只给冷启动读数：
三类误判各一条判据、③ 门的两条出口、② 门的三分语义。**不复述门脚本细节**（写了必漂，ADR-0036 ③ 明令），
上面「时间账与三样慢」「全量 ③ 门落盘与入口」「全量 ③ 门该不该跑」已有的读数，这里只引小节名、不抄回来。
⚠️ 本节数字分两批：**2026-10-14 现测**（head `4d4f22ed`，178 套件单腿）与 **2026-10-18 两腿实测**（head `632a30e0` 工作树，190 套件，本机 12 核，见 `docs/adr/0042-测试门拆腿与逐套件心跳.md`）——**别拿 #1410 当年的账当现测**。

### 三类误判，各一条判据

1. **慢 ≠ 卡死**。判据 = **看心跳行进到第几套、看入口第一行给的预期时长**，不看「输出有没有在动」（#1702 起两者一致了）。
   ③ 门现在是两腿：入口**第一行**先给预期时长，随后逐套件心跳 `JEST_SUITE i=k/N status=… secs=… elapsed=…` 连续出现，
   一条腿收尾落一行结果行，两腿都跑完才按先红者退出。**2026-10-18 实测**：接线腿 **42 s / 120 套件**、
   行为腿 **1129 s / 70 套件**（两腿合计 1172 s）；最慢单套 `utils/mpWeixinGateResultLineBehavior.test.js` **496 s**，
   其后 `kotlinAllProcessCaptureBehavior` 76 s、`mpWeixinGateRetryBehavior` 73.5 s；`jest.setTimeout` 在 `utils/*.test.js`
   里仍有 9 处取 **120–600 秒** ⇒ **单个超长套件内静默数分钟仍是设计内**（`-i` 串行，#1410），
   但「整跑零输出」那种形态已经不存在了。
   真源：`docs/adr/0036-…` 处置一（那一跑被读成「卡死」，实为 `exit 0`）与 `docs/adr/0042-测试门拆腿与逐套件心跳.md`（拆法与基线）。
2. **刷屏 ≠ 失败**。判据 = **只认结论行的 `exit=`，噪音不参与判红**。
   现测噪音量级：快路径入口那一跑，落盘日志 858 行里 **828 行是 pwsh 自己的**
   `node.exe: 无法处理来自 "Error" 的 … 流中的 XML: Data at the root level is invalid.` 解码噪音
   （**不是被测物写的**），一条 `PASS` 都没抓到 ⇒ 结论行老实写 `missing=suites|tests|time`，
   失败摘要件自己也写着「本次输出里没抓到 Tests 汇总行」。
   ⚠️ **捕获层已是流式，但落盘件仍不全等于 jest 原始输出**（2026-10-18 修订）：入口现在边跑边吐
   （`Invoke-GateLegCommand`：`Tee-Object -Variable … | Out-Host`，`Out-String` 会把整段攒到最后，实测见 ADR-0042 决策 4），
   可**汇总行仍可能被 CLIXML 噪音挤掉**——同日两腿那一跑，行为腿机检行老实写 `missing=suites|tests|time`
   （心跳行 `JEST_SUITE_DONE total=70 passed=67 failed=3` 反而是那跑唯一完整的计数来源）。
   ⇒ 先问「结论行在不在」，别问「输出好不好看」；**要读 jest 汇总就绕开门入口直跑那一腿的 `npm run test:*:local` 并重定向到文件**。
   ⚠️ 这条**曾经**被写成「换个捕获方式治不了」——那个结论只对「攒到最后再吐」成立；**判读的可见性**靠流式，
   **落盘件的完整性**靠绕开入口读，两件事分开记。
   ⚠️ **缺字段 ≠ 跑失败**：上面那跑串行门 `exit=0` 也照样 `missing=suites|tests|time`——落盘件按汇总行取数，
   取不到就写 `?`（口径见「全量 ③ 门落盘与入口」里 `?` 那条：`?` 是「没读到」，不是「没有」）。
   ⇒ 看 `exit=` 判成败；`missing=` 只说明**读不到汇总**，不改变成败。
3. **红 ≠ 重跑整道门**。判据 = **先分「墙钟红」与「逻辑红」，重跑半径是单套件**。
   现测一组同树同 head 的对照：并行快路径 `exit=1`，3 套红（`deviceCaptureTierBehavior` DCT11 期望 `False` 得 `True`、
   `wirelessDebugBehavior` B2/B3、`styleLoopBehavior` E3 期望 `2` 得 `-1`）；**这 3 套单独并行重跑 66 秒全绿**
   （`Test Suites: 3 passed, 3 total` / `Tests: 53 passed, 53 total`），**在串行 `-i` 全量门里也全绿**（`exit=0`）
   ⇒ 那一跑的红是**争抢下的计时读数**，不是逻辑变了。先例 #1410（`utils/chapterNotFoundBehavior.test.js` A4
   `Exceeded timeout of 5000 ms`，单套件重跑 13/13 绿；2026-10-14 复现一次同样绿）。
   ⇒ 处置顺序：**单套件重跑 → 绿 ⇒ 原样记红、不扩大重跑**；**仍红 ⇒ 才去查环境**（这三套都碰 adb / 设备 / 入口进程，
   本机无设备时读数本就不稳）。整道门重跑的成本账归上面「成本账反模式（2026-10-03 立）」，「我觉得没改」不算判据。

### ③ 门的四条出口：两腿 × 入口与结果行

出口从两条变**四条**（#1702 拆腿 × ADR-0036 ③ 的门/快路径）：首 token = `TEST_<LEG>_…`，`LEG ∈ {UNIT, BEHAVIOR}`，
快路径行尾多一枚 `mode=local` ⇒ **四者机检可区分，不许混读**（门是全量的、快路径是本地日常）：

- **全量门** `npm run test:unit:gate`（= `scripts/dev-finish.ps1 -UnitGate`，**两腿都跑、串行 `-i`**）⇒ 两行：
  `TEST_UNIT_RESULT`（接线腿，`.ci-verify/test-unit.log`）与 `TEST_BEHAVIOR_RESULT`（行为腿，`.ci-verify/test-behavior.log`）；
  2026-10-18 实测 **42 s + 1129 s = 1172 s / exit=0**（拆腿前单腿 880 s / 178 套件）。
- **本地快路径** `npm run test:unit:gate:local`（= `-UnitLocal`，**并行、不带 `-i`**，同为两腿）⇒ `TEST_UNIT_LOCAL_RESULT` /
  `TEST_BEHAVIOR_LOCAL_RESULT` 各带 `mode=local`，产物 `test-{unit,behavior}-local.log` 与对应的 `-failure.txt`；
  单腿现测 **`wall=300s`**（同树绕开入口直跑 `npm run test:unit:local` 是 325 秒、jest 自报 `Time: 320.282 s`——
  差的那一截是入口自己那层捕获，不是被测面变慢）。
  它**不进九步管线、不取 HBuilderX 锁、不碰设备**。
- ⚠️ **两枚同给时快路径赢**（`-UnitLocal` 分支排在 `-UnitGate` 之前）⇒ 别指望「顺手带上 `-UnitGate` 就能贴 PR」。
- ⚠️ **`-PostToPr` 只在 `-UnitGate` 生效**：快路径带它会打印「`-PostToPr` 只在 -UnitGate（全量门）生效；
  本地快路径不贴 PR 评论」然后照常跑完 ⇒ **要出 PR 证据就切全量门那条**，快路径的结论进不了 `## 验收证据`。
- **快路径的红不等于门的红**：`-i` 的取舍与真因写在 `jest.config.unit.js` 文件头（12 核实测 5 次并行出现 1 次墙钟红），
  本节不复述。日常内循环走快路径，收口与 PR 证据走全量门。
- **时间账只认这一跑**：2026-10-18 两腿实测 = 接线腿 42 s + 行为腿 1129 s（全量门 1172 s）；快路径单腿 300 s。
  比 #1410 记的「并行 60–90 s / 串行 236 s」与 2026-10-14 的「178 套件 880 s」又大了一截——
  套件从 146 → 178 → 190，**行为套件变重**（`mpWeixinGateResultLineBehavior` 从 265 s 涨到 496 s）是主因
  ⇒ **旧账不能当现测引**，引之前先现跑一次，并把新数写回 `docs/adr/0042`。

### ② 门的 env-blocked：三分语义与处置

判据源 = `scripts/lib/mp-weixin-env.ps1`（结论行由它的两个构造器产出，**互斥由构造器保证，不靠调用方自觉**）。
本节只给读法：

- **通过** = metric 行里**没有** `errors=` 这枚 token（判别靠**缺席**），exit 0。
  现例（#1655 收口记录）：`MP_WEIXIN_RESULT appid=… pageStack=1 entry=pages/index/index errorsTotal=0 …`
  ——里面有 `errorsTotal=0`，**但没有 `errors=`**，这两件事就是「通过」的长相。
- **门未过** = `errors=<N> failures=…`（N 是数字），**exit 1** ⇒ 回去改产物。
- **环境受阻** = `errors=env-blocked reason=<slug>[ failures=…][ <metricLine>]`，**exit 2** ⇒ 换环境，别改代码。
  现测构造行：`MP_WEIXIN_RESULT errors=env-blocked reason=devtools-open-timeout failures=环境不可用：… appid=… pageStack=1`。
- ⚠️ **产物坏了判 1、不判 2**：判据锚是失败条目的**固定前缀「环境不可用：」**（取 `StartsWith`，不是 `Contains`）。
  现测分类：`环境不可用：端口未监听` ⇒ `kind=env-blocked`；`pageStack 读取失败：空`（不带那个前缀）⇒ `kind=gate-failed`
  ⇒ 把 1 读成 2，就是拿「环境问题」洗掉一个真红。
- `reason=` 的值域是**闭集**：构造器只收 `^[a-z0-9-]+$`，非法值当场抛（现测 `bad reason` ⇒
  `env reason 只允许小写连字符 slug`）⇒ 看到没见过的 slug 是**脚本坏了**，不是新类别。
  全集 26 项落在 `scripts/mp-weixin-check.ps1` 的 `GATE_PLAN.branches`，**本节不抄**（抄了就是第二真源）。
- **处置 = 转人工，或改跑不需要 HBuilderX / DevTools 的检查**——这一列**不是本节新造的**，
  `scripts/hx-run.ps1` 的环境不可用分支与 `scripts/lib/hx-busy.ps1` 的 `Write-HxBusyHint` 已经在打印它：
  `npm run test:unit` / `npm run build:kotlin-all -SkipPublish` / `npm run smoke:emulator`。
  ⚠️ `-SkipPublish` 只跑 kotlinc 段（不依赖主程序），**publish 那一步仍需 HBuilderX 主程序可达**
  （判据源 `scripts/kotlin-all-check.ps1` 头部；它还不适用新鲜度判据，结果行会显式记）。
- ⚠️ **绝不 kill 主程序**：`hx-run.ps1` 与 `hx-busy.ps1` 自述「本脚本**绝不 kill** HBuilderX / cli / 主程序
  （忙就等，超时就退出）」——HBuilderX 是**单实例串行资源、GUI 优先**（见移动端 `AGENTS.md`「并发会话纪律（多会话并行时必读）」）。
  env-blocked 是**一等出口，不是 bug**（ADR-0036 ③）⇒ 别把它当失败去重跑整道门，也别去抢那把锁。

### 这一节不装什么

门脚本的细节（字段怎么拼、重试几轮、锁怎么排队、退出码怎么透传）**不住这里**：判据住脚本与 ADR-0036，
本节只负责「冷启动时读得懂现场」。上面每一条都点名了它的真源文件——**改判据请改脚本，别在本节加叙述**。
分层口径见移动端 `AGENTS.md`「三层节奏」与本文件头部那句「引用本文件时引小节名，不引行号」。

## ② 贴证前两核（#1845 实证，2026-10-24 立）

② 门跑完、证据贴出**之前**，两样各核一遍——路由级判据过了不算内容取证成立：

1. **`auth=` 读数**：结果行 `MP_WEIXIN_RESULT … auth=<值>`——`auth=injected` / `auth=existing` 才是会话有效；`auth=none(<reason>)` 出现时，storage 里可能还躺着**过期 token**（app 恢复后业务层 401 弹「登录已过期」，路由仍停目标页、探针照判 reached）。此时改动页截图不得当内容取证成立。
2. **截图内容目视**：改动页截图看一眼是不是目标页真形态（#1845 实拍「登录已过期」Toast 仍宣称取证通过的完整链与更正记录见 #1845 票评论）。

处置：会话失效 ⇒ 降级重取——注入自愈时 `-Auth auto`（幂等 existing/injected），或人工登录后 `-Auth off`（不盖帽不注入，注意入口页会因 #1478 reLaunch 判 miss、如实记 skip）。**会话注入失败仍可路由级判绿**的机器侧缺陷登记于 #1863（修好前本节两核是唯一防线）。

## 真机无线调试入口（#1564，2026-10-07 落地）

一句话：**每次真机前先跑 `npm run connect:device`**，它接上设备并把「门该用哪个 `-Device`」打在结论行里；
之后开一个 `npm run connect:device -- -Action watch` 当后台保活，收工 `-Action stop`。

四条不看代码就会踩的判据：

1. **mDNS 候选必须逐个现测**。`adb mdns services` 的表里会**同时挂着已经 10061 的旧端口**
   （2026-10-07 现测：活端口 39527 与死端口 37611 并列）⇒ 「记录里有」不等于「能用」；
   `already connected` 与 `state=device` 都不算数，判据是连上之后 `shell echo ok` 真有回话。
2. **防不住手机侧重置**。重启 / 撤销授权 / 人手动关掉开关之后一个 adb 通道都没有，
   任何脚本都开不了那个开关——那一刻仍要人在手机上点一次；脚本保证的是**点完这一次不用再读屏**。
3. **绝不 `adb kill-server`**（server 与 HBuilderX 共享）。这一条与 `docs/adr/0025-论坛正文格式与输入区形态.md`
   「mDNS 变空就 kill-server 重来」的旧处置**相互冲突**，本工具按 `scripts/device-capture.ps1` 的只读禁令取「不 kill」，
   把「整表变空」判为需人介入的态。冲突的裁决与代价记在 #1564。
4. **它是载体不是门**：结论行叫 `WIRELESS_DEBUG`，**不进** `## 验收证据`；机读只用 ASCII 行，
   中文全文在 `%LOCALAPPDATA%\adb-wireless\last-run.txt`（本仓实测：pwsh 子进程 stdout 被上层按 GBK 解码必乱码）。

---

## 工作树与会话坑位原文

**HBuilderX 回写坑位同样适用**：跑完任何 HBuilderX 步骤（含 `hx-run.ps1`）先 `git status` 看 `manifest.json` **与 `pages.json`** 是否被改脏，脏了就还原再继续 —— 后者会被写入一段 `condition`（GUI 选的启动页，注释自述「仅开发期间生效」），**属本地开发配置、禁止提交**；而 `pages.json` 同时是「运行时面 / 打包面」判据来源，误提交会让 PR 凭空命中 ④b 云打包门（详见 `docs/adr/0008-移动端验收门与证据.md`）。

**另一条收尾坑位（2026-09-13 一天踩到两次）**：`hx:run` 的**真运行会话会常驻**，并把它自己写的 `.ci-verify\launch-*.out/.err` 攥在手里 ⇒ **只要会话还活着，那个项目目录既删不掉也改不了名**（报 `being used by another process`）。**删 worktree / 给项目目录改名之前**，先 `cli project close --path <项目>` 结束会话（或发起下一次 launch 把它顶掉）。`hx:compile-only` 不留常驻会话，因此**没有这个副作用**。

## `--compile` 语义坑位（血账 #917，原文）

**`--compile` 的语义坑位（2026-09-13，血账 #917）**：官方帮助里 `--compile` 是「**仅编译代码**」（默认 false），
不是「先编译再运行」⇒ **`--compile true` 只允许出现在两处**：全量编译门 `compile-check.ps1`，以及 `hx-run.ps1`
的 `-CompileOnly` 分支；**真运行路径一个都不许有** —— 从前它照抄了门的调用，于是**从未真正请求过运行**，
「编译成功 → 已停止运行...」被误读成「运行失败」并追了整整一票。**反过来说，「仅编译」正是拿编译期诊断的捷径**。
**判据必须带基线**：判「有没有到设备」要运行前后各取一次设备侧事实相对比（如资源目录 mtime 前进）；
拿「当前前台是不是基座」当判据，在**重复运行到同一台机器**时恒为真 ⇒ 假绿。

## 样式内循环：一条命令、一行结论

（2026-10-05 立，#1543；同帧三条腿读数 2026-10-07。载体 = `npm run style:loop`，仓内入口 `scripts/style-loop.ps1`。）

`npm run style:loop`（= `scripts/style-loop.ps1`）把「拿编译期诊断 → 真运行到设备 → 逐页截图 → 与基线做像素比」
收成**一个进程、一行机检结论**：

```
STYLE_LOOP gate=none verdict=… phase=… changed=… shot=… log=… pages=… shots=… reason=… secs=…
```

* 红 ⇒ 非零退出，且 `shot=` **行内点名是哪一页变的**；绿 ⇒ 退出码 0。退出码三分：`0` 绿 / `1` 判据判出的红（回去改代码）/ `2` 环境不可用（换环境再看）。
* 一行以外的东西**全量落** `.ci-verify/style-loop.log`（表头绑 head 与 `dirty_files=`）——「会话只读一行」不等于少留证据。
* **它不是门**：行内恒带 `gate=none`，不进 CI、不改四门的触发面与归属。**内循环的绿不等于门过了。**

常用形态：

```
npm run style:loop -- -Device <serial>                                  # 改动集里已有 pages/ 改动
npm run style:loop -- -Device <serial> -Pages 'pages/profile/settings'  # 首轮建基线（改动集里还没有 .uvue 时）
npm run style:loop -- -Device <serial> -IgnoreTopRows 120               # 收掉状态栏噪声（真机建议）
npm run style:loop -- -Device <serial> -UpdateBaseline                  # 人看过 diff、确认是有意改动之后刷基线
npm run style:loop -- -DryRun                                           # 只看计划与参数，不执行（verdict=plan，不是判据）
```

**时间账（2026-10-07 实测，设备 `23049RAD8C` / 1080×2400，1 页 `settings`，head `bee02190`）**：顺利一轮 = 链自报 **235 秒**（驱动侧墙钟 237.8 秒），构成分段：编译期诊断 `HX_RUN mode=compile-only total=66` ⇒ 装机 `mode=incremental total=78 deployed=True` ⇒ 导航 `NAV_SAMPLE settled=True seconds=83 samples=13 callTimeouts=0` ⇒ 截图 + 像素比。对照件：同设备同页的现读全链路 `dev:finish -Level standard` 实测 **1007 秒**（**10-05 记**，其间含 151 秒排队等另一会话的锁；它自报的「17 分 46 秒」分钟数由 `[int](总秒/60)` 四舍五入、与秒位不自洽，记账取外层时间戳差）。差的 772 秒来自**不做** ④c 整模块编译门、契约测试与证据生成——那三件是中循环与收口门的活。**串行资源一个没多**：全程只取一把 `Wait-HxFree`（ADR-0011 的 A-3，覆盖「仅编译 → 真运行 → 逐页截图」）。

**判据没有第二真源**：编译诊断 = `hx-run -CompileOnly`；部署判定 = `HX_RUN_DEPLOY deployed=`（设备侧事实相对基线前进，取 `Deployed` **不取 `Ok`**）；「这一轮截哪些页」的唯一真源仍是 `lib/auto-screenshot.ps1`（从 git diff 推导并按 `pages.json` 校验）；像素判据 = `lib/screenshot-gate.ps1` 的 `Get-PngDiffVerdict`（阈值 0.005，与 `dev:finish` 步骤 7 同一个函数、同一个「本轮产物」起点，`Select-ThisRunShots` 不重写）。入口自带的预检只看「`pages/` 下有没有改动」，**故意取成推导集合的超集**且**没有判红权**：读不到 git 一律放行，由后面的推导去判。

**基线卫生：同帧三条腿**（2026-10-07 立，取代「隔天复跑」）。跨天基线会让「改坏」与「没改」**分不开**：同一棵零改动的树，昨天绿腿读到 0.266%、今天读到 0.919%，而阈值是 0.5%。协议因此固定为：**红腿**（真改一处色值 ⇒ 必 `exit 1` 且点名）→ **刷基线腿**（`-UpdateBaseline`，把同帧画面钉成参考图）→ **绿腿**（同帧复跑 ⇒ 必 `exit 0`）。现读：红腿像素差 **27.122%**、绿腿 **0.324%**，阈值 0.5% 正好落在两者之间。⚠️ 别用 `-UpdateBaseline` 把噪声压进基线——它只在「人看过 diff、确认是有意改动」之后才允许用。三条腿**别写同一个输出目录**（`.ci-verify/screenshots/` 会被后一条腿覆盖，先例：#1560 的 LEG_C 覆盖了 LEG_A 的 png）。

**残留噪声的定位**（本地复算，不占设备）：拿同帧两枚图跑 `node scripts/lib/png-diff.mjs --a … --b … --ignore-top-rows N` —— `N=0` ⇒ `different=8387 / ratio=0.003236`；`N=120/300/600/1200` ⇒ **`different=0`** ⇒ 那 0.324% **全部落在顶部 120 行以内**（状态栏一带：时钟、网速读数），第 120 行以下逐像素相同。处置口径：**要收噪声动的是 `-IgnoreTopRows`（取状态栏高度），不是把 `PixelThreshold` 抬高**——抬阈值会把红腿的判据一起钝掉；阈值本身不在该入口射程（默认 0.005 与 `dev:finish` 同源，不 fork）。

**工作树纪律**：进临界区前给 `manifest.json` / `pages.json` / `platformConfig.json` 拍**字节**快照，退出时（含异常与 `exit` 路径）按字节写回。`lib/build-deploy.ps1` 那层还原比的是**文本**、用 `Set-Content` 写回 ⇒ BOM / 行尾变了它看不出来而 git 看得出来；这一层是兜底，不是重复。三条腿各自跑完 `git status --porcelain` 行数 = 0，链尾 `配置字节还原 restored= appeared= unchanged=manifest.json,pages.json,platformConfig.json`。

**坑位（自查轮实测，不是推测）**：

* `$ErrorActionPreference='Stop'` **不会**让原生命令非零退出抛异常（本机 pwsh 7.6.6、`$PSNativeCommandUseErrorActionPreference=False`：`git -C <非仓库>` 返回**空串** + `$LASTEXITCODE=128`，`catch` 根本不触发）⇒ 想按「读不到 git」做 fail-open 必须**显式认退出码**；只写 `catch` 就是注释承诺 fail-open、代码走 fail-closed。
* 包装层（父）与链体（子 `-Inner`）必须分段：`Wait-HxFree` 超时是自己 `exit 2` 的，链体内联在父进程里会被**就地截断**——那一轮既没有结论行也没有日志。
* 转发实参只许一份（一张表派生链体入参与子进程命令行）：手抄清单**漏抄一个不报错**，子进程静默用默认值 ⇒ 判据就不再是人以为的那个。
* 日志落不了盘时那一行必须**改口**（`log=unwritten`）而不是继续声称产物存在；细节走 stderr，stdout 仍恰好一行。
* 首次运行的基线落盘**动作归入口执行**（判定表 `write-baseline` 那一支只给结论）；只转述不执行 ⇒ 基线永远为空、每轮都「首次运行」、判据永不咬（同一死循环由 `0008` 的「2026-09-18 修订（`dev:finish` 步骤 7 从空转修成真的）」第 3 条掐掉）。

**守护**：`utils/styleLoopBehavior.test.js`（真源 `node scripts/classify-guards.mjs` 判 `[行为]`，33 用例，八组 L/V/W/R/C/D/E/P）。断言的是**产物形状**——那一行的字节数与字段、退出码、还原后的文件 sha256、真跑入口的终端行数——不是脚本源码原文；成对断言（必不红 / 必红）逐组配齐。E 组一律带 `-Device <不存在序列号>` + `-ProjectDir <临时目录>` ⇒ 在任何机器上跑都不会抢 HBuilderX。

**决策与失败语义全文**见 `docs/adr/0034-样式内循环的一行结论与包装层失败语义.md`（移动端编号体系，须写全路径）；锁纪律与并发语义仍归 `docs/adr/0011-一键完成的并发与失败语义.md`。
