# 守护的两类：行为守护 vs 接线守护

> 移动端 ③ 门（**两腿**：`npm run test:unit` 接线腿 + `npm run test:behavior` 行为腿；落盘入口
> `npm run test:unit:gate`）的判据说明。真源是**分类器**，不是本文件里的清单：
> `node scripts/classify-guards.mjs`（人读）/ `--json`（机读 / 供 `utils/guardClassification.test.js` 断言）。
> 立此文的起因：issue **#1156**（验证层「永远绿」三层整改，规格 `docs/spec-永绿整改.md` 随 PR #1154 落地）。

## 为什么「全绿」不是证据

2026-09-18 实测（一手）：

```
把 utils/format.uts 的 `return '0s'` 改坏   → format.test.js 照样 8 passed（绿）
把 utils/checkinCalendar.uts 的算法改坏     → checkinCalendar.test.js 照样绿
```

两个文件都是**手抄的镜像实现**（`format` 连「镜像同步」那段都没有）⇒ 「全绿」可以恒真。
⇒ **③ 门的判据不是「测试绿了」，而是「我故意弄坏被测物，它会不会红」。**

## 三类判据（③ 门用第一条；后两条是它的两部分）

| # | 判据 | 含义 |
|---|---|---|
| ① | **改坏它，会不会红？** | 判别力。答不上就不算验证层 |
| ② | **它测的是该测的那一支吗？** | 条件编译 / 镜像 / 分支选错时，「跑起来了」但测的是另一支 |
| ③ | **只跑通过的那一次，不算验收** | 成对取证：必不红 / 必红各一条 |

## 两类守护

两条判据**同时是 ③ 门拆腿的切分轴**（#1702）：同一份分类结果既回答「它承不承重」，也决定这个套件落在哪一腿 ——
`behavior` ⇒ `test:behavior`（`jest.config.behavior.js`），`wiring` ⇒ `test:unit`（`jest.config.unit.js`）。
两腿的 `testMatch` 都由 `scripts/lib/guard-split.js` 现算，**没有第二份手写清单**（血账 #974 的形态）。

| 类 | 判据（代码级事实） | 对 ③ 门 |
|---|---|---|
| **行为守护 behavior** | 文件里**有代码级的执行调用**（`child_process` / `execFileSync` / `spawnSync` / `utsHarness` / 动态 `import()`），**且**引用了至少一个仓内载体（`.uts` / `.uvue` / `.ps1` / `.mjs` / `.json`） | **承重**：只有它回答得了判据 ①；落在 `test:behavior` 腿（带逐套件心跳行） |
| **接线守护 wiring** | 读源码文本断言调用点、常量、命名、顺序 | **不构成 ③ 门证据**：算法被改坏而字面量还在时**不会红**；落在 `test:unit` 腿 |

判据是**代码事实**，不是文件名约定、不是注释里提到过什么、也不是作者自报：
文件名叫 `*Behavior.test.js` 而实际只 `expect(src).toContain(...)` 的，照样归接线守护。

> 分类器第一版就栽在「注释里提到 `.ps1` 也算真跑」上：`screenshotDiffContract` / `devFinishContract` /
> `hxRunContract` / `envCheckContract` / `testCompileContract` / `uvuePageScrollContract` 等 **9 个**
> 文件被误判成行为守护（20/78）⇒ 修正后 **13 行为 / 86 接线**。**判据必须落在代码级调用上。**

## 现状（快照 2026-10-18；**真源永远是分类器**，不要照抄本节数字）

- **实测：行为守护 70 · 接线守护 120 · 合计 190**（`node scripts/classify-guards.mjs`，`feat/1702-split-unit-behavior`，
  拆腿 PR 内按新进程取数；含该 PR 新增的 `utils/jestSplitContract.test.js`）。拆腿后这两腿就是 ③ 门的两个入口面。
- **`utils/guardClassification.test.js` 的 H2 钉的是「下界」，不是「现值」**：`BEHAVIOR_MIN = 70` / `TOTAL_MIN = 190`
  （2026-10-18 按新进程读数上调；此前 35 / 138）。迁移镜像会让 behavior **上升**（**允许**），下降才红
  ⇒ 本节的数字**本来就会随 master 前进而过期**（#1160 合并后即如此：master 又多了 4 个套件）。
  **要现行值请跑分类器，别读这里。**
- **点名：目前只是接线守护的 7 个手抄镜像模块**（迁移对象，spec §④ 的 S6 系列）：
  `checkinCalendar` · `faqDisplay` · `format` · `notebookDisplay` · `pointsDisplay` ·
  `searchDisplay` · `secureStorage`。
  其中 **`format` / `checkinCalendar` / `profileMetrics`** 连仓内载体都不引用（纯镜像手抄），由 H4 显式登记。
  > **已迁走：`aiSourcesDisplay`（#1279，2026-09-24）** —— 镜像换成 `utsHarness.loadUts` 真执行
  > `.uts` 本体，并按 spec §④ 更正 2 补了「逐条注入坏实现 ⇒ 红」六条（其中一条的坏实现就是
  > 修正**之前**的真实状态：只认历史 `<<IMAGE:>>`、不认后端归一后的正文 markdown 图）；
  > `guardClassification.test.js` 的 H5 台账同步划掉。**先例是 #1237**（收藏落点守护切到 utsHarness）：迁移**不必**等 S6a 单独开票，
  > 谁的票的 ③ 承重证据落在这个模块上，谁就顺手迁。

## 例外登记：行为缝射程外的原生支

- **`systemBrowser.uts` 的 APP-ANDROID 原生 Intent 支 —— 无行为守护（#2035 登记）。**
  原因：该文件 import 段是 `import Intent from 'android.content.Intent'`（UTS 原生类逐名**默认导入**形态），
  `utsHarness.importedNames` 对默认导入 **fail-closed**（只认 `import { … }`）⇒ 整文件进不了行为缝；
  且该支在非 App 端不编译、其余端是空实现，node 里无从观察原生效果。
  **行为面承重**：`externalLinkDispatchBehavior.test.js`（编排真执行：空链接闸 / 弹窗字样 / 确认闸 /
  副作用落点，成对取证）；**接线面钉死**：`systemBrowserWiringContract.test.js`（具名导入 /
  唯一调用点在 APP-ANDROID 区内 / null 静默 return 先于 startActivity）。
  协议闸与域名投影另有 `markdownInlineBehavior.test.js` ④/域名组（不在本例外射程）。

## 新增守护时要回答的（写进 PR 正文即可）

1. 它是**行为**还是**接线**守护？（跑 `node scripts/classify-guards.mjs` 看它落在哪一类）
2. 若是行为守护：**成对**断言在哪？（必不红 / 必红各一条）
3. 若是接线守护：它守的是哪条**接线**？以及这条接线由**哪个行为守护**兜底行为？（两者都要，缺前者会漂、缺后者会假绿）
4. 它**吃不吃得到环境**？（#2078 CI 血账，2026-10-29）纯逻辑（前缀分批 / 页集合 / 计数）只许从**纯函数**的调用面断言，
   **不要穿过**需要 adb / 设备 / HBuilderX 的完整链路：那条链在没有该环境的机器上会**在纯逻辑之前**早退，集合恒空
   ⇒ 得到一条「本地永远绿、CI 永远红」的守护（#2078 连红三次的现场）。反过来，每个「环境缺失」分支
   （找不到 adb / cli / pages.json）都要能在**本机造出来**跑（stub 解析器返回 `$null`），别让它只在上游显形。
