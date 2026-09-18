# 守护的两类：行为守护 vs 接线守护

> 移动端 ③ 门（`npm run test:unit` 全绿）的判据说明。真源是**分类器**，不是本文件里的清单：
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

| 类 | 判据（代码级事实） | 对 ③ 门 |
|---|---|---|
| **行为守护 behavior** | 文件里**有代码级的执行调用**（`child_process` / `execFileSync` / `spawnSync` / `utsHarness` / 动态 `import()`），**且**引用了至少一个仓内载体（`.uts` / `.uvue` / `.ps1` / `.mjs` / `.json`） | **承重**：只有它回答得了判据 ① |
| **接线守护 wiring** | 读源码文本断言调用点、常量、命名、顺序 | **不构成 ③ 门证据**：算法被改坏而字面量还在时**不会红** |

判据是**代码事实**，不是文件名约定、不是注释里提到过什么、也不是作者自报：
文件名叫 `*Behavior.test.js` 而实际只 `expect(src).toContain(...)` 的，照样归接线守护。

> 分类器第一版就栽在「注释里提到 `.ps1` 也算真跑」上：`screenshotDiffContract` / `devFinishContract` /
> `hxRunContract` / `envCheckContract` / `testCompileContract` / `uvuePageScrollContract` 等 **9 个**
> 文件被误判成行为守护（20/78）⇒ 修正后 **13 行为 / 86 接线**。**判据必须落在代码级调用上。**

## 现状（2026-09-18，运行 `node scripts/classify-guards.mjs` 取现行值）

- **行为守护 13 · 接线守护 86 · 合计 99**（实测下界，被 `utils/guardClassification.test.js` 的 H2 钉住；
  迁移动镜像只会让它**上升**，下降就红）。
- **点名：目前只是接线守护的 8 个手抄镜像模块**（迁移对象，spec §④ 的 S6 系列）：
  `aiSourcesDisplay` · `checkinCalendar` · `faqDisplay` · `format` · `notebookDisplay` · `pointsDisplay` ·
  `searchDisplay` · `secureStorage`。
  其中 **`format` / `checkinCalendar` / `profileMetrics`** 连仓内载体都不引用（纯镜像手抄），由 H4 显式登记。

## 新增守护时要回答的（写进 PR 正文即可）

1. 它是**行为**还是**接线**守护？（跑 `node scripts/classify-guards.mjs` 看它落在哪一类）
2. 若是行为守护：**成对**断言在哪？（必不红 / 必红各一条）
3. 若是接线守护：它守的是哪条**接线**？以及这条接线由**哪个行为守护**兜底行为？（两者都要，缺前者会漂、缺后者会假绿）
