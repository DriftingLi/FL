# #1543 `style:loop` 真机三条腿（2026-10-07）

目录按**票号**归档（先例 `docs/verification/tooling/1560`）：票 **#1543**，承载它的 PR 是 **#1561**。

入口：`npm run style:loop -- -Device 192.168.10.51:39527 -Pages pages/profile/settings`
head：`bee0219005e01be106882b0ebec9b12fbc6f3675`（`feat/1543`，已同步 master `0dcfe937` = #1560 落地后的形状）
设备：Redmi `23049RAD8C`（marble）/ 1080×2400，**无线 adb**（USB 序列号 `b32d8398` 当时不可达，`adb devices` 只认这条通道）
节奏：三条腿**串行**跑，不与全量 ③ 并发（墙钟假红同族，见 `docs/agents/dev-loop.md`）

| 腿 | 本地时刻 | 退出 | 机检行（原文） | 像素比 |
| --- | --- | --- | --- | --- |
| leg1 **必红**（真改 `.container` 底色 `#f5f5f5`→`#e0e0e0`） | 14:02:37 → 14:07:09（驱动侧 271.6 s） | 1 | `STYLE_LOOP gate=none verdict=red phase=pixel changed=1 shot=settings.png log=.ci-verify/style-loop.log pages=1 shots=1 reason=pixel-changed secs=269`（148 B） | **27.122%** vs 阈值 0.5% |
| leg2 刷基线（`-UpdateBaseline`，干净树） | 14:08:52（264 s） | 0 | `STYLE_LOOP gate=none verdict=green phase=done changed=0 shot=- log=.ci-verify/style-loop.log pages=1 shots=1 reason=clean secs=261`（130 B） | 对旧基线 **0.362%**（无变化）⇒「基线已更新」 |
| leg3 **必不红**（同帧，间隔 120 s） | 14:15:16（237.8 s） | 0 | 同形状，`secs=235`（130 B） | **0.324%** |

**成对成立**：阈值 0.5% 落在 27.122% 与 0.324% 之间 ⇒ 「改坏」与「没改」分得开。这条判据昨天分不开（同树零改动读到 0.919%），当时以为是阈值问题，实际是**跨天基线**——所以协议改成「红腿 → `-UpdateBaseline` 刷同帧基线 → 绿腿」，本目录就是那三腿。

**AC1 的分段现读**（取自 leg3，`style-loop-appended-log.txt` :276-296）：编译期诊断 `HX_RUN mode=compile-only compile=65 deploy=0 total=66`（诊断为空）⇒ 装机 `mode=incremental total=78 deployed=True` ⇒ 导航 `NAV_SAMPLE settled=True seconds=83 samples=13 callTimeouts=0 callBudgetSeconds=15 budgetSeconds=420` ⇒ 截图 + 像素比。链自报 `secs=235`，驱动侧墙钟 237.8 s。

**#1560 在真链上的读数**：三条腿各 13 次采样、`callTimeouts=0` ⇒ 有界化后没有一次触到 15 秒单次预算；同一条导航在 10-06 20:06 那腿是 32 分钟零写入。

**文件归属（覆盖风险已核实）**

- `leg1-red-npm.txt` —— leg1 的 npm 侧整段输出（含机检行）。
- `style-loop-appended-log.txt` —— `.ci-verify/style-loop.log` 是**追加**文件，共 296 行：**206 行之后才是今天这三条腿**（第 206 行是 leg1 的分段头 `# ---- style:loop 2026-10-07 14:02:39 用时 270s head=bee02190 dirty_files=1 from_child=True exit=1 ----`，`dirty_files=1` 就是那枚色值夹具），206 行之前是 10-05 / 10-06 的历史腿（含 0.266% / 5.774% / 0.947% / 0.877% / 0.919% 那几读）。
- ⚠️ **leg1 的截图 PNG 没留住**：`.ci-verify/screenshots/settings.png` 会被后一条腿覆盖，leg1 只剩日志里的 `sha256=1CF829565F41DC1D…`（工具自己就把摘要截到 16 位，所以这枚只能当线索、不能独立复算）。
- `leg2-shot-settings.png` 与 `leg3-baseline-settings.png` 是**同一枚** sha256 `41cbc4462ad7f068cd17b55f135db85a223fabaa31538961baf6b6996dc2c634` ⇒ 这就是「同帧」的物证。
- `leg3-shot-settings.png` = `8f4ef9e490e455856f6386b0307011d195c889f0a284627369ff264de4eb39ea`。上面两枚之差已由 `node scripts/lib/png-diff.mjs --a … --b … --threshold 0.005` 独立复算：`different=8387 / total=2592000 / ratio=0.003236 / changed=false` ⇒ 与日志里的 0.324% 一致。

**工作树**：每条腿跑完 `git checkout -- pages/profile/settings.uvue` + `git status --porcelain` 行数 = 0；链尾另有 `配置字节还原 restored= appeared= unchanged=manifest.json,pages.json,platformConfig.json`。

**残留差异的定位（2026-10-07 18:20 本地复算，不占设备）**：拿本目录那两枚同帧图（`leg3-baseline-settings.png` 与 `leg3-shot-settings.png`）按行带忽略再比一次 —— `node scripts/lib/png-diff.mjs --a … --b … --ignore-top-rows N`：

| `N`（忽略顶部行数） | `different` | `ratio` |
| --- | --- | --- |
| 0 | 8387 | 0.003236 |
| 120 / 300 / 600 / 1200 | **0** | **0** |

⇒ 那 0.324% **全部落在顶部 120 行以内**，也就是状态栏一带（时钟与网速读数在这一带）；**第 120 行以下逐像素相同**，页面本体一点没动。这条读数的用处有两层：① 它说明绿腿离 0.5% 阈值的余量几乎全被状态栏吃掉 —— 真要收噪声，该动的是 `-IgnoreTopRows`（取状态栏高度），**不是**把 `PixelThreshold` 抬高（抬阈值会把红腿的判据一起钝掉）；② 它给「同帧」加了一条更强的复算：不止 `sha256` 相同的那一对，跨 120 秒的两帧在页面本体上是**完全一致**的。

**③**：本地两读在旧 head `92d33301`（`exit=0 suites=160 tests=3073`，见 PR #1561 正文与门评论 `#issuecomment-6017152996`）；同步后的新 head `bee02190` 由 CI `mobile-test` run `37579178112` success 覆盖，**未在本地重跑**——判据是 `docs/agents/dev-loop.md`「全量 ③ 门该不该跑」的 C2 档（同 head 树未动时不重跑；本地那遍已经跑过）。
