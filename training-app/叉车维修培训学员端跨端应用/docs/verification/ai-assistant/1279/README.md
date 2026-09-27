# #1279 ① 门真机取证 · 诊断助手 20260921 图片契约变更（①a 已完成，待开 PR）

- **票**：#1279（移动端跟进 · 两点自测）· **分支**：`fix/1279` · **取证日期**：before 2026-09-24（上一会话）；**after 2026-09-26 本会话完成**
- **设备**：after = Redmi `23049RAD8C`（marble / b32d8398，无线调试 `192.168.10.54:41077`）；before = 小米 `2510DRK44C`（annibale）。**跨设备如实声明**：a11y 文本计数判据（0/11 处路径串）与设备无关，截图形态属跨设备对照。
- **被测对象**：AI 诊断助手页（`pages/ai-assistant/ai-assistant` 专项态）→ 问出图问题 → 回答气泡正文 + 来源卡片区；构建 = `fix/1279`（head `5019d86d`）经 `npm run hx:run` 增量部署（设备侧 www mtime 前进、pid 16556）
- **取证方式**：`adb` 只读（`screencap` / `uiautomator dump`），见移动端 ADR-0008「①a 取证手法补遗」；问句由人手工输入（`adb input text` 对中文抛 NPE，现测不可注入）

## 票面两点自测与当前状态

| # | 自测点 | 状态 | 依据 |
| --- | --- | --- | --- |
| 1 | 来源标记新形状 `<<IMAGE:/assistant/static/fault_images/<中文目录>/…>>` 真机确认案例图能出 | **已收（真机）** | 预览行为判据（空 urls 直接 return ⇒ 能开预览即解析成功）`1 / 3 → 3 / 3`；**指纹定身份**：预览第 1 页与生产直取的 `fault_images/制动系统/1721219449286.png`（907×727, aspect 1.249）32×32 零均值归一化相关 = **0.997**（对照 manual_linde -0.003 / manual_byd 0.122）。详见 `after-04-machine-lines.txt` |
| 2 | 交付方 `\| 描述:xxx` 后缀截断（`split('\|')[0]` 同式） | **代码+测试已收** | `aiSourcesDisplay.uts` 的 `pathSegmentOf`；`aiSourcesDisplay.test.js` 自测点 2 用例组 + 必红变异（本会话 2026-09-26 实测：把 `bar` 改成 `-1` ⇒ 9 条红，还原后全绿） |

另有一处票面之外但同根的缺陷已由本分支修掉并真机复现过：**正文面**被后端归一成 `![alt](代理 URL)` 后，纯文本气泡把百分号编码路径串逐字露给学员（before dump 实测 11 处）。

## 判据与实测结果（可复算）

**before**（`04-before-answer-a11y-dump.xml`，BMS 一问）：

- `fault_images` 出现 **11** 次，全部形如 `![诊断配图](/api/ai-assistant/diagnosis/manual/fault_images/%E7%94%B5%E6%B1%A0_BMS%E7%B3%BB%E7%BB%9F/….png)`；
- `<<IMAGE:` 裸令牌 **0** 次 ⇒ 「后端不再吐裸令牌」这半成立，但归一产物本身是 markdown，正文面照样泄漏（根 ADR-0063 决策 1 订正块的实测来源）。

**after**（本分支构建，2026-09-26）：

- BMS 一问：`诊断配图` = **11**，路径串形态（`/api/ai-assistant|%E7%94%B5%E6%B1%A0|\.png`）= **0**；
- 制动异响一问（票面复现问句）：`诊断配图` = **3**，路径串形态 = **0**，`[IMG:`/`<<IMAGE:` = **0**（dump 见 `after-05-answer-a11y-dump.xml`）；
- 来源区：「资料来源（16 条）」正常渲染，标记不在任何 a11y 文本中出现；点图开全屏预览 `1 / 3`，第 1 页 = fault_images 案例图（corr 0.997，见上表）；
- logcat 近 3000 行 FATAL / `.invoke` / error18 命中 **0**（`displayContent` 包装守护未触发）。

## 产物

| 文件 | 内容 | 备注 |
| --- | --- | --- |
| `01-before-source-card-proxy-images.png` | 修复前：来源卡代理图状态截屏 | 上一会话产出；本会话无图像输入，**未逐像素复核** |
| `02-before-body-plain-text-tail.png` | 修复前：正文纯文本尾部（路径串外露） | 同上 |
| `03-before-body-citation-and-table-leak.png` | 修复前：引用与表格泄漏形态 | 同上 |
| `04-before-answer-a11y-dump.xml` | 修复前：回答态 uiautomator dump 原文 | **before 判据数据源**（11 处计数出于此） |
| `05-before-source-cards-a11y-dump.xml` | 修复前：来源区 dump —— **作废** | 实测只有 7 个 content-desc：dump 到的是**专项页表单态**（‹/标题/筛选/全部品牌/展开/➤/🔊），当时不在会话页、来源区不存在；由 after-02 取代 |
| `after-01-answer-bubble.png` | 修复后：制动异响回答气泡 | 正文以 `诊断配图` 呈现、无路径串（同屏 a11y 计数 3/0） |
| `after-02-sources-expanded.png` | 修复后：来源区展开（资料来源 16 条，head 近顶） | 卡片区起始 y≈431 |
| `after-03-source-preview-1-case-image.png` | 修复后：来源卡图全屏预览 **1 / 3** | **指纹 = fault_images 制动系统案例图（corr 0.997）** |
| `after-03b-source-preview-2.png` / `after-03c-source-preview-3.png` | 同卡 2 / 3、3 / 3 页 | 与已知三候选均不匹配（本轮检索的其他溯源图） |
| `after-04-machine-lines.txt` | ①a 机检行原文（计数 / 指纹 / logcat / 过程备注） | 复算入口 |
| `after-05-answer-a11y-dump.xml` | 修复后：制动异响回答态 a11y dump | after 计数数据源 |
| `after-06-mp-weixin-check.png` | ② 门 `build:mp-weixin-check` 的截图产物（`.ci-verify/current.png` 入仓） | PR ② 行的行内可核验产物 |

## 门状态（2026-09-26 本会话）

| 门 | 状态 | 机检行 / 产物 |
| --- | --- | --- |
| ③ `npm run test:unit` | 🟢 合并 master 后全绿：**135 suites / 2628 tests**；判别力实测见上表自测点 2 | `.ci-verify/test-unit-1279-postmerge.txt` |
| ④c `npm run build:kotlin-all` | 🟢 | `KOTLIN_ALL_RESULT errors=0 classes=1504 files=120 freshness=fresh`（`.ci-verify/kotlin-all.log`） |
| ② `build:mp-weixin-check` | 🟢 可验证子集（逐页导航组 = SKIP，reason=navigation-api-unsupported，按 ADR-0008 ② 段降级不计门失败） | `MP_WEIXIN_RESULT appid=wx38c3e31b16a7ced0 pageStack=1 errorsTotal=0 exceptionsTotal=0 navigation=skip(unsupported)`（`.ci-verify/mp-weixin.log`） |
| ①a 真机逐页取证 | 🟢 **2026-09-26 完成**（本 README「判据与实测结果」+ after 五件产物 + 机检行） | 见上表 after 产物 |
| 分支 CI | 🟢 `d3f940ba` → run 36220519542 success；`5019d86d` → run 36220788172 success | GitHub Actions |
| ①b 能力面 | 不命中（无指纹/权限弹窗/上传/厂商 ROM 交互） | — |

## 复算方式

1. ③：`npm run test:unit`；判别力抽查 = 把 `utils/aiSourcesDisplay.uts` 的 `pathSegmentOf` 里 `const bar = inner.indexOf(MARKER_CAPTION_SEP)` 改成 `const bar = -1` ⇒ `aiSourcesDisplay.test.js` 必红（2026-09-26 实测 9 failed），还原即绿。
2. ①a 正文计数：对 `after-05-answer-a11y-dump.xml` 复算 `诊断配图`（3）与路径串 pattern（0），与 before 的 11 处对照。
3. ①a 图身份：`Invoke-WebRequest` 直取 `https://www.gccsmile.com/api/ai-assistant/diagnosis/manual/fault_images/%E5%88%B6%E5%8A%A8%E7%B3%BB%E7%BB%9F/1721219449286.png`，与 `after-03-source-preview-1-case-image.png` 中按行/列密度裁出的主体做 32×32 零均值归一化相关（脚本原文见 `after-04-machine-lines.txt` 描述）。

## 待办

1. 证据入仓 → 开 PR（`## 验收证据` 按 ADR-0008 四门填；① 行执行人可写「agent 执行」，**② 行执行人栏须维护者原文**，agent 不得代填、不得写「已通过」）→ CI 绿 → 合并 → 关票。
