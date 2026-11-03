# #2249 探针脚本（阶段 0 基线测量的原件）

本目录是 issue #2249 阶段 0 三件测量的**可重跑原件**，不是被测物的一部分，也不进任何门。
测量结论见 `docs/design/2249-ruler-changeset-and-startup-account.md` 与
`docs/design/2249-slow-suite-per-case-account.md`。

放在 `docs/verification/tooling/<票号>/` 是按仓内先例（`1562/stale-read-probe.ps1`、
`1568/emulator-tier-probe.ps1` 等）：探针随票归档，否则笔记里的读数无法复现。

## 使用约定

- **工作目录必须是移动端工作区**：`training-app/叉车维修培训学员端跨端应用/`。
  路径写仓根相对路径会让 jest 当那个套件不存在（0 套件，长得像绿）。
- 全部脚本**只读**——不改门脚本、不改套件、不改夹具、不改 `behavior-suite-deps.mjs`。
- 这些脚本会产生一次性端口与临时子进程；跑完确认没有残留监听
  （`roster`：`Get-NetTCPConnection -State Listen -LocalPort <p>` 应回到 0）。
- 原测量时脚本放在 `scripts/tmp-*.mjs|cjs`，提交前由本目录取代（`tmp-` 前缀不入库）。

## 逐脚本

| 脚本 | 对应笔记节 | 做什么 |
| --- | --- | --- |
| `ruler-table.mjs` | 尺子卡片 §1 | 对 9 类代表文件各跑一次 `behavior-suite-deps.mjs`（含/不含 b′），生成分档表 |
| `subset-size.mjs` | 尺子卡片 §1 | 单个文件的子集大小速查 |
| `bench-startup.cjs` | 启动拆账 §2 | A/B/C 三形态 × n 次跑同一个轻量套件，各组自检离散度 |
| `bench-interleave.cjs` | 启动拆账 §2.5 | A、B 相邻穿插（先 A 后 B），取同一时间窗差值算 npm 层净成本 |
| `poll-budget.cjs` | 慢套件卡片 §3 | 复刻 `Wait-MpWeixinAutoPortReleased` 的轮询循环，量「轮数 × 单次查询」的真实墙钟 |
| `port-release.cjs` | 慢套件卡片 §3 | 端口释放场景的第一版探针（**保留原因见下**） |
| `port-release2.cjs` | 慢套件卡片 §3.1–3.2 | 修正版：异步 `spawn` 派生独立监听器进程，到时自灭 |

## `port-release.cjs` 为什么留着（它是作废读数，但有教育价值）

第一版用 `execFileSync` 跑同一场景，把监听器放在**同一条被阻塞的事件循环**里，
于是监听器脚本的 `setTimeout` 永远轮不到执行 ⇒ 监听器根本不老死，测出
`LOOP_TOTAL_MS=184172 rounds=60`（跑满 60 轮）的假象。改用异步 `spawn`
（`port-release2.cjs`）后才是 `63 969 ms / 19 轮`。

这正是票面假设②「付满迭代预算」的错误形态。留原件是为了让任何人能复现这个坑，
而不是只看结论。**不要用 `port-release.cjs` 的读数做任何判断。**
