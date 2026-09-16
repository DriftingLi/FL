# ①a 真机取证 —— 专项页诊断筛选（#1042）

- **票**：#1042 `feat(mobile-ai): 专项页诊断筛选（品牌/车型/故障码检索）`
- **分支**：`feat/mobile-ai-1042-diagnosis-filter`
- **取证 head**：`d376b3b655a079cfcad6771c162d1384306b46b8`（`Merge remote-tracking branch 'origin/master' …`）
- **门**：①a（agent 出证）。①b **不命中能力面**（本改动无指纹 / 运行时权限弹窗 / 真机上传 / 厂商 ROM 交互）⇒ 免签，① 行执行人可写「agent 执行」（ADR-0016）。
- **设备 / 环境**：`b32d8398`（小米 2510DRK44C / marble），USB 调试基座，HBuilderX 5.24，adb `D:\android-sdk\platform-tools\adb.exe`，项目目录用唯一名 `fl-mobile-wt1042`（HBuilderX 按项目名解析）。
- **取证窗口**：2026-09-16 15:37:31（`logcat -c`）→ 15:53:58（最后一次 `logcat -d`）。
- **为什么在合并 head 上重跑**：首轮取证跑在合并**前**的树上，而 #1063（#1041）随后改了**同一页** `pages/ai-assistant/ai-feature.uvue`（加会话抽屉 + `loadSessions`）⇒ 按 ADR-0016 ⑥「收口后再改运行时面 ⇒ 相关页必须重跑」，首轮产物**作废、不入仓**（隔离在会话产物目录的 `contaminated-run1/`）。

## 证据摘要

判据全部取自 `uiautomator dump` 的 **`content-desc`**（uvue 的文字在 a11y 树里走 `content-desc`，不走 `text`）。**每次截图都取「前一瞬」与「后一瞬」两份 dump**，两份都过 gate 才记 `VERIFIED`。截图仅作产物：本会话模型不支持图片输入，**agent 未看图片内容**，故文本判据与截图成对取。

| 步骤 | 截图 | 字节 | 逐字 `content-desc` 断言 |
| --- | --- | --- | --- |
| 01 折叠态 | `01-diagnosis-filter-collapsed.png` | 65311 | `‹` / `智能维修诊断` / `筛选` / `全部品牌` / `展开` / `➤` / `🔊`；TEXT `提出你想要知道的问题` |
| 02 展开态 | `02-diagnosis-filter-expanded.png` | 91861 | `收起` 出现；品牌 chip：`全部品牌` / `中力叉车 (EP)` / `杭叉叉车 (Hangcha)` / `林德叉车 (Linde)`；`查询`；TEXT `搜索故障码或现象` |
| 03 选品牌 | `03-diagnosis-brand-selected.png` | 110613 | 头部摘要变为 `杭叉叉车 (Hangcha)`；**新增车型行** `全部车型` / `1-3.5t R 系列` / `2-2.5t R 系列` / `3-3.5t R 系列`（品牌→车型联动成立） |
| 04 故障码检索 | `04-diagnosis-fault-search.png` | 115425 | 新增 `05` + `提升接触器驱动回路开路`（真实故障码）；输入框内为 `05` |
| 05 负向 · 维保知识 | `05-maintenance-no-filter.png` | 52510 | 全量 desc **仅 4 条**：`‹` / `维保知识` / `➤` / `🔊`；`filter_marker_count=0` |
| 06 负向 · 图纸识别 | `06-drawing-no-filter.png` | 52951 | 全量 desc **仅 4 条**：`‹` / `图纸识别` / `➤` / `🔊`；`filter_marker_count=0` |
| 辅助 · 宫格 | `00-grid-my-build.png` | 172248 | 四个 tile 齐备（`智能维修诊断`/`维保知识`/`图纸识别`/`习题解答`）、`故障代码查询` 已移除 |
| 辅助 · 深链进入 | `00a-build-proof-fault-page.png` | 67155 | 深链 `featureKey=fault_diagnosis` 直接进入时筛选面板已在渲染 |

**负向断言的检查口径**：`筛选|全部品牌|展开|收起|搜索故障码或现象` 同时扫 **`content-desc` 与 `text`** 两处 ⇒ `filter_marker_count=0`。

**负向对照为什么可归因本构建**（这是本文件的关键，负向不能靠「截图里没有」）：

1. 两页均由**站内导航**到达（点 `‹` 回宫格 → 点「维保知识」/「图纸识别」tile），**不重启、不深链** ⇒ 全程同一个 app 实例：**`app_pid` start = 7236、end = 7236（`same_as_start=True`）**。
2. 站内导航链有 logcat 实证，全部来自 pid 7236：
   ```
   15:50:45.933 7236 进入页面:/pages/ai-assistant/ai-feature?featureKey=fault_diagnosis
   15:52:57.995 7236 进入页面:/pages/ai-assistant/ai-feature?featureKey=maintenance_knowledge
   15:53:34.609 7236 进入页面:/pages/ai-assistant/ai-feature?featureKey=drawing_recognition
   ```
   ⇒ 正例与两个负例出自**同一进程、同一构建**，缺口的归因成立。

## 构建新鲜度（证明跑的是合并后的包，而不是上一轮）

`base.apk` 的 md5 前后一致（`22a83978cd8ea106684d13a5d5371796`），**但这不能证明重编译** —— uni-app x 的运行基座是共享的，按页编译产物同步到设备侧 `apps/__UNI__1C1D180/www/`，与 apk 无关。故改用**设备侧按页产物**对比（基线在启动前采集）：

| 设备侧产物 | 基线（合并前） | 本轮启动后 | 变化 |
| --- | --- | --- | --- |
| `pages/ai-assistant/ai-feature/classes.dex` | `3fd555652130fc197db78bb949474529`（62348 B） | `803b8172fbe2ddbb49c411d83e1c288b`（67396 B） | **变了** |
| `pages/ai-assistant/ai-assistant/classes.dex` | `9b2cbe3dd1e97ec13b9a303617a965a9`（90832 B） | `005fd5cc5dfc90b2669f62d78bf5b889`（90792 B） | **变了** |
| `manifest.json` | `4c6450c84d919d680f798f1493cef176` | 同左 | 未变 |

HBuilderX 本轮 launch 输出：`15:37:34 项目 fl-mobile-wt1042 开始编译` → `15:40:03 编译成功` → `ready in 147874ms` → `15:40:08.765 进入页面:pages/ai-assistant/ai-feature?featureKey=fault_diagnosis`。两个 dex 的设备侧 mtime 亦落在该编译窗口内。

## logcat

- 窗口：`logcat -c` @ 15:37:31 → `logcat -d` @ 15:53:58（导出 15419274 B / 91469 行，本地产物 `logcat-run.txt`，未入仓）
- `FATAL EXCEPTION` = **0**
- `ANR`（朴素子串，大小写不敏感）= 2；`ANR`（`\bANR\b`）= 1；`ANR in ` = 0；`OverlayANRGuard` = 1
- 两条命中逐字，**都不是本 app 的 ANR 事件**：
  1. `09-16 15:45:30.643 … W OneTrack-Crashlytics-XCrash: Crashlytics won't catch ANR, processName: com.xiaomi.xmsf:privacy`
  2. `09-16 15:48:06.435 … D Launcher: OverlayANRGuard: active=false`
- ⇒ **纠正后的真实 ANR = 0**（两口径都如实列出：朴素子串会把 `OverlayANRGuard` 这类含 `ANR` 的标识符计入）
- 局限：`logcat` 是**设备级**缓冲，含其它进程的行。

## 机检行（逐字，完整见同批产物 `phase1.log` / `phase2.log` / `cleanup.log`）

```
HX_BUSY wait=0 result=free
HX_RUN_1A step=rename result=ok from='叉车维修培训学员端跨端应用' to='fl-mobile-wt1042'
HX_RUN_1A step=project_open exit=0 out='正在导入项目... 项目导入成功'
HX_RUN_1A step=gate tag=proof-a1 result=passed polls=18 required='筛选+全部品牌+展开'
HX_RUN_1A step=build_proof result=ok filter_panel_present=true app_pid=5504
HX_RUN_1A step=gate tag=grid-b1 result=timeout polls=35
HX_RUN_1A step=gate tag=grid-b2 result=passed polls=19 required='智能维修诊断+维保知识+图纸识别+习题解答'
HX_RUN_1A step=grid_up result=ok four_tiles=true removed_entry_present=False app_pid=7236
APP_PID_START pid='7236'
BUILD_FRESHNESS page=ai-feature baseline_md5=3fd555652130fc197db78bb949474529 device_md5=803b8172fbe2ddbb49c411d83e1c288b changed=True
BUILD_FRESHNESS page=ai-assistant baseline_md5=9b2cbe3dd1e97ec13b9a303617a965a9 device_md5=005fd5cc5dfc90b2669f62d78bf5b889 changed=True
HX_RUN_1A step=capture tag=00grid result=VERIFIED shot=00-grid-my-build.png bytes=172248 preDesc=19 postDesc=19 app_pid=7236
HX_RUN_1A step=capture tag=01 result=VERIFIED shot=01-diagnosis-filter-collapsed.png bytes=65311 preDesc=7 postDesc=7 app_pid=7236
CHECK_01 title_present=True header=True summary=True toggle=True verify=True
HX_RUN_1A step=capture tag=02 result=VERIFIED shot=02-diagnosis-filter-expanded.png bytes=91861 preDesc=12 postDesc=12 app_pid=7236
CHECK_02 verify=True collapse_toggle_present=True allBrands_present=True candidates='中力叉车 (EP) | 杭叉叉车 (Hangcha) | 林德叉车 (Linde)'
HX_RUN_1A step=capture tag=03 result=VERIFIED shot=03-diagnosis-brand-selected.png bytes=110613 preDesc=16 postDesc=16 app_pid=7236
CHECK_03 tapped='杭叉叉车 (Hangcha)' verify=True new_descs_vs_02='全部车型 | 1-3.5t R 系列 | 2-2.5t R 系列 | 3-3.5t R 系列'
HX_RUN_1A step=capture tag=04 result=VERIFIED shot=04-diagnosis-fault-search.png bytes=115425 preDesc=18 postDesc=18 app_pid=7236
CHECK_04 verify=True new_descs_vs_03='05 | 提升接触器驱动回路开路'
HX_RUN_1A step=capture tag=05 result=VERIFIED shot=05-maintenance-no-filter.png bytes=52510 preDesc=4 postDesc=4 app_pid=7236
CHECK_05 mode=in-app title_present=True verify=True filter_marker_count=0 markers=''
CHECK_05_FULL_DESC_LIST count=4 list='‹ | 维保知识 | ➤ | 🔊'
HX_RUN_1A step=capture tag=06 result=VERIFIED shot=06-drawing-no-filter.png bytes=52951 preDesc=4 postDesc=4 app_pid=7236
CHECK_06 mode=in-app title_present=True verify=True filter_marker_count=0 markers=''
CHECK_06_FULL_DESC_LIST count=4 list='‹ | 图纸识别 | ➤ | 🔊'
APP_PID_END pid='7236' same_as_start=True
LOGCAT FATAL_EXCEPTION=0 ANR_raw_substring=2 ANR_word_boundary=1 ANR_in=0 OverlayANRGuard_hits=1
HX_RUN_1A step=cleanup_project_close exit=0 out='项目关闭完成'
HX_RUN_1A step=cleanup_rename result=ok note='renamed on attempt 2'
HX_RUN_1A step=cleanup_dirs names='叉车维修培训学员端跨端应用'
HX_RUN_1A step=cleanup_hbuilderx_alive count=1 pids='26936'
```

## 取证后的工作树状态

- `git -C D:\wt-1042 status --short` → 空
- `D:\wt-1042\training-app` 下只有 `叉车维修培训学员端跨端应用`（目录名已还原；本轮 rename-back 在 **attempt 2** 成功，**未 kill 任何 `cli`**）
- `HBuilderX.exe` pid 26936 存活（全程未触碰）

## 局限 / 未覆盖（如实声明）

1. **截图内容未由 agent 核对**（模型不支持图片输入）⇒ 断言全部来自 `content-desc`；截图仅作产物。
2. **未覆盖 ①b 能力面**（本改动不命中）；**未**对 #1063 新增的左侧会话抽屉作任何断言（非本票对象）。
3. **`onUseFaultCode`「点故障码条目即发一轮」未实测**：只验到检索列表渲染出真实故障码，未点条目验证随后的发送行为。
4. 步骤 04 的提交走 `keyevent 66`；`查询` 按钮回退分支未被执行。
5. 合并后折叠态摘要格式为 `品牌 / 车型` 拼接（合并前只显示品牌），属 #1063 的预期差异，本文件不对它作断言。
6. 设备在 cleanup 后短暂 `device not found`、约 1–2 分钟自愈；发生在全部截图取完之后。
7. 真机为共享资源：本轮与 #1041 会话并发，故采用「静默窗 + 每次截图前后双 dump 归因」；`hx-agent.lock` 保护 HBuilderX 但**不保护设备**。
