# #1279 ① 门真机取证 · 诊断助手 20260921 图片契约变更（进行中）

- **票**：#1279（移动端跟进 · 两点自测）· **分支**：`fix/1279` · **取证日期**：before 2026-09-24（上一会话）；after **待做**（真机离线，维护者裁定留下会话）
- **设备**：小米 `2510DRK44C`（代号 annibale，无线调试 `192.168.0.212:37611`；2026-09-26 实测 `adb connect` 失败、`mdns services` 无发现 ⇒ 设备不在线）
- **被测对象**：AI 诊断助手页 → 问「电动叉车制动系统异响…」类出图问题 → 回答气泡正文 + 来源卡片区
- **取证方式**：`adb` 只读（`screencap` / `uiautomator dump`），见移动端 ADR-0008「①a 取证手法补遗」

## 票面两点自测与当前状态

| # | 自测点 | 状态 | 依据 |
| --- | --- | --- | --- |
| 1 | 来源标记新形状 `<<IMAGE:/assistant/static/fault_images/<中文目录>/…>>` 真机确认案例图能出 | **待真机** | 单测层已锁（`extractImagePaths` 把 `fault_images/…` 整段留给后端代理）；后端代理同 URL 实测 `HTTP 200 / image/png`（根 ADR-0063 订正块）；**屏上渲染未在设备确认** |
| 2 | 交付方 `\| 描述:xxx` 后缀截断（`split('\|')[0]` 同式） | **代码+测试已收** | `aiSourcesDisplay.uts` 的 `pathSegmentOf`；`aiSourcesDisplay.test.js` 自测点 2 用例组 + 必红变异（本会话 2026-09-26 实测：把 `bar` 改成 `-1` ⇒ 9 条红，还原后全绿） |

另有一处票面之外但同根的缺陷已由本分支修掉并真机复现过：**正文面**被后端归一成 `![alt](代理 URL)` 后，纯文本气泡把百分号编码路径串逐字露给学员（before dump 实测 11 处）。

## 判据（before 侧，可复算）

`04-before-answer-a11y-dump.xml`（回答态无障碍树）：

- `fault_images` 出现 **11** 次，且 **11** 次全部形如 `![诊断配图](/api/ai-assistant/diagnosis/manual/fault_images/%E7%94%B5%E6%B1%A0_BMS%E7%B3%BB%E7%BB%9F/….png)`；
- `<<IMAGE:` 裸令牌出现 **0** 次 ⇒ 「后端不再吐裸令牌」这半成立，但归一产物本身是 markdown，正文面照样泄漏（根 ADR-0063 决策 1 订正块的实测来源）。

修复后（本分支构建）的预期 after 判据：同一问下正文 dump 中上述两条形态均为 **0**，代之以 alt 文本 `诊断配图`（或后端 caption）；来源区出现 `fault_images` 子路径的卡片且图可加载（HTTP 200）。

## 产物

| 文件 | 内容 | 备注 |
| --- | --- | --- |
| `01-before-source-card-proxy-images.png` | 修复前：来源卡代理图状态截屏 | 上一会话产出；本会话无图像输入，**未逐像素复核**，仅按文件名登记，after 复拍时一并对照 |
| `02-before-body-plain-text-tail.png` | 修复前：正文纯文本尾部（路径串外露） | 同上 |
| `03-before-body-citation-and-table-leak.png` | 修复前：引用与表格泄漏形态 | 同上 |
| `04-before-answer-a11y-dump.xml` | 修复前：回答态 uiautomator dump 原文 | **判据数据源**（11 处 `![诊断配图](…)` 计数即出于此） |
| `05-before-source-cards-a11y-dump.xml` | 修复前：来源区 dump —— **作废** | 实测仅 7 个 content-desc（‹/智能维修诊断/筛选/全部品牌/展开/➤/🔊），**没捕到来源卡**（dump 时来源区未展开），after 须复拍 |

## 门状态（2026-09-26 本会话）

| 门 | 状态 | 机检行 / 产物 |
| --- | --- | --- |
| ③ `npm run test:unit` | 🟢 合并 master 后全绿：135 suites / 2628 tests；判别力实测见上表自测点 2 | `.ci-verify/test-unit-1279-postmerge.txt` |
| ④c `npm run build:kotlin-all` | 见 PR 正文（本会话重跑） | `.ci-verify/kotlin-all.log` |
| ② `build:mp-weixin-check` | ⏳ 上一会话 2026-09-24 15:50 判 `errors=env reason=automation-not-ready`（非门不过，是自动化端口未就绪） | `.ci-verify/mp-weixin.log` |
| ①a 真机逐页取证 | ⏳ **设备不在线**（2026-09-26 实测）；维护者裁定「先收口其余，①a 留下会话」 | 本 README |
| ①b 能力面 | 不命中（无指纹/权限弹窗/上传/厂商 ROM 交互） | — |

## 复算方式

1. ③：`npm run test:unit`；判别力抽查 = 把 `utils/aiSourcesDisplay.uts` 的 `pathSegmentOf` 里 `const bar = inner.indexOf(MARKER_CAPTION_SEP)` 改成 `const bar = -1` ⇒ `aiSourcesDisplay.test.js` 必红（2026-09-26 实测 9 failed），还原即绿。
2. ①a after：真机在线后 `adb exec-out screencap` 逐屏截屏 + `uiautomator dump`，对 dump 复算上表两条形态计数（均应为 0），并与 before 的 11 处对照。

## 待办（下一会话接手时从这里开始）

1. 真机上线 → 部署 `fix/1279` → 同一问复拍 after（正文 0 路径串 + 来源案例图渲染 + 复拍作废的 05）；
2. ② 门重跑（微信开发者工具需已登录）；
3. 证据入仓后开 PR（`## 验收证据` 按 ADR-0008 四门填，① 行执行人可写「agent 执行」）→ CI 绿 → 合并 → 关票。
