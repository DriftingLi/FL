# #1443 ①a 真机取证 · AI 助手助手气泡接 markdown 块渲染

- **票**：#1443 · **PR**：#1444 · **分支**：`feat/1443`（head `0f9dcf90`）· **取证日期**：2026-09-30
- **设备**：Redmi `23049RAD8C`（marble / arm64-v8a / Android 15），无线调试 `192.168.10.51:37175`
- **被测对象**：AI 专项页 `pages/ai-assistant/ai-feature`（`featureKey=fault_diagnosis`，即「智能维修诊断」）→ 问出诊断问题 → 助手回答气泡正文经 `components/ai-chat/ai-chat-bubble.uvue` 调 `utils/markdown.uts` 的 `parseMarkdown`（`SUBSET_FORUM`）渲染
- **改动性质**：纯渲染层。助手气泡把后端返回的 markdown 解析为块级结构（标题/列表/粗体/行内代码）；`expandImageMarkers`（#1279）保持在解析之前；用户气泡（`role == 'user'`）走 `displayContent` 纯文本、逐字节不变。
- **取证方式**：`cli launch --pagePath --pageQuery` 一步到带参诊断页；`adb` 只读截图（`screencap`）+ `uiautomator dump`；问句经 **ADB Keyboard IME 广播注入**（`am broadcast -a ADB_INPUT_TEXT`，本设备现测可用——较 #1279 记的「adb input text 对中文抛 NPE」为改进）。见移动端 ADR-0008「①a 取证手法补遗」。

## 判据与实测结果

| 判据 | before（旧构建） | after（feat/1443） |
| --- | --- | --- |
| 助手气泡正文裸 markdown 标记 | 逐字露出 `**…`、`# …`、`- **…**`、`` ` ``、`[citation:…]` | a11y dump 统计：`**`=0、行首`#`=0、反引号=0、行首`- `=0、`citation:`=0 |
| 标题/列表结构 | 混在纯文本里 | 各成独立文本节点（「标准排查作业指导书 (SOP)」「一、…」「二、…」+「•」列表项） |
| 用户气泡 | 纯文本 | 纯文本，逐字节不变（未过解析） |
| 崩溃 / 渲染 error | — | logcat 取证窗口 FATAL/AndroidRuntime/SIGSEGV = 0；无 render error |

## 文件清单

- `before-01-old-build-answer-raw-markdown.png` — 旧构建（不含 #1443）诊断回答，正文裸 markdown（另一问句，对照渲染现象）
- `after-01-answer-bubble-top.png` — feat/1443 回答顶部：标题 + 子标题 + 列表 + 诊断配图占位，均渲染
- `after-02-answer-bubble-tail.png` — 同回答尾部（五/六/七节），标题加粗、列表圆点，无裸标记
- `after-03-answer-a11y-dump.xml` — 渲染后无障碍树（31 文本/描述节点，供复算标记计数）
- `after-04-machine-lines.txt` — 机检行（到设备 / 页面可达 / 无崩溃 / 标记计数）

> ①a 为 agent 出证（ADR-0016）；本票不命中「能力面」（无指纹 / 运行时权限弹窗 / 真机上传 / 厂商 ROM 交互），①b 不适用。
