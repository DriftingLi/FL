# ① 真机验证截图 · PR #1257（#1240 P2 论坛正文格式消费：正文 / 回复卡渲染接入）

设备：Redmi 23049RAD8C（`b32d8398`），Android 15，HBuilderX 调试基座 —— 按移动端
`docs/adr/0008-移动端验收门与证据.md` 的 ① 门（**①a：agent 出证**）留档。

**承载面 = 论坛详情页**（`pages/forum/forum-detail.uvue` + `forum-topic-body` + `forum-reply-list`）：
- 话题「**测试markdown渲染**」（作者 张三十，2026-09-12 11:33）—— 正文是四段式 Markdown 夹具：
  `## 测试 Markdown 渲染` / `- 无序列表` / `1. 有序列表` `2. 你好` / ` ```python ` 代码块 / 三条 `---`；回复里另有
  `# 一级标题 …` 与公式 `$$…$$`。
- 话题「**1**」（作者 张三，2026-09-17）—— 纯文本档（`content_format=text`）的**零回归对照**。

设备当时已由上一会话登录（基座内 `Token validated, isLoggedIn = true`），本轮**未触碰凭证**：靠
`adb shell input tap`（坐标取自 `uiautomator dump` 的 bounds，先现测 `TAP_RC=True`）逐页走到目标页。

## 图片

| 文件 | 构建 | 说明 |
| --- | --- | --- |
| `01-forum-list-after.jpg` | 本分支 head（21:09 部署） | 论坛列表（入口页）：Markdown 话题在列（卡片预览仍是截断纯文本，见下「未覆盖面」） |
| `02-forum-detail-markdown-before.jpg` | 设备上**既有旧构建**（不含本票改动） | Markdown 话题正文**直出源串**：整段是一个满宽文本节点（`## 测试 Markdown 渲染` `- 无序列表` `---` ` ```python `…）—— 本票要治的降级 |
| `02-forum-detail-markdown-after.jpg` | 本分支 head | **同一话题同一位置**按块渲染：标题 / 项目符号列表 / 有序列表 / 分隔线 / 代码块 |
| `03-forum-detail-replies-after.jpg` | 本分支 head | 回复卡同样走格式轴：回复 `# 一级标题…` 渲染成三级标题，公式回复按**声明口径**保留源码 |
| `05-forum-detail-text-after.jpg` | 本分支 head | 纯文本话题「1」正文 —— 与改前**逐字不变**（零回归对照） |

## 判据（不靠肉眼看「像不像」，都可机检）

1. **改前正文 = 源串**：`02-before-ui-dump.xml`（未压缩 `uiautomator dump`）里正文节点
   `content-desc='## 测试 Markdown 渲染&#10;&#10;- 无序列表&#10;&#10;---&#10;&#10;1. 有序列表…```python…```…---'`
   ⇒ `## ` / `- 无序列表` / ` ```python ` / `---` 四条源串**全部命中**（`02-before-content-desc.txt`）。
2. **改后正文 = 块**：同一话题（`02-after-ui-dump.xml` / `02-after-content-desc.txt`）四条源串**全部为 0**，取而代之的是块级文本：
   `测试 Markdown 渲染`（标题）、`•` + `无序列表`、`•` + `有序列表`、`•` + `你好`、
   `print("hello world")`（代码块，节点 `content-desc='print("hello world")'`）；三条 `---` 变成三对 1px 满宽
   `ViewGroup`（`bounds=[46,832][1034,833]` / `[46,1191][1034,1192]` / `[46,1508][1034,1509]`，无文本）
   ⇒ **分隔线不再是字面 `---`**。
3. **零回归**：纯文本话题「1」的正文节点，改前与改后**同 bounds**（`[46,467][1034,545]`）**同文本**（`1`）
   （`05-after-text-topic-content-desc.txt`）。
4. **回复卡同口径**：`03-after-replies-content-desc.txt` 里回复正文为 `一级标题` / `二级标题` / `三级标题`
   （不再是 `# 一级标题` 那种源串）；公式回复保留 `$$ … $$` 源码 —— 这是 ADR-0025 ③ 具名登记的**真降级**，不是本轮的破口。
5. **部署相对基线前进**（不用「前台是不是基座」这种恒真判据）：`hx-run.ps1` 机检行
   `HX_RUN_DEPLOY deployed=true www=…/apps/__UNI__1C1D180/www=1789979684->1789996138 pid_before=…14298 pid_after=…17854
   reason=资源已落到设备…mtime 相对基线前进`；`HX_RUN mode=incremental compile=210 deploy=0 total=265 exit=ok`。
6. **日志无 error**：取证窗口内 app 进程（`io.dcloud.uniappx`）**E 级行 = 0**（剔 `AccessibilityNodeInfoDumper`
   这类取证自身噪声）；`logcat.txt` 为去重后的 app 相关行（170 行）。

## 手法（可复现）

```powershell
# 1) 部署本分支到真机（真运行，不传 --compile；项目目录须先改成唯一名 —— HBuilderX 按项目名解析）
npm run hx:run                     # 机检行 HX_RUN / HX_RUN_DEPLOY
# 2) 逐页取证：坐标一律取 uiautomator dump 的 bounds 中心，不用截图目测
adb -s b32d8398 shell "input tap <x> <y>"        # 先 shell "input tap …; echo TAP_RC=$?" 现测可注入
adb -s b32d8398 exec-out screencap -p > page.png
adb -s b32d8398 exec-out uiautomator dump /dev/tty > page.xml
```

> ⚠️ **本轮实测坑位（值得记）**：`uiautomator dump --compressed` 会把这条**长** `content-desc`（整段正文源串）
> 丢成空串，看着像「正文没渲染」；换**未压缩** `uiautomator dump` 才拿到原文。上面所有文本判据都取自未压缩 dump。

## 未覆盖面（如实登记，勿读成「已覆盖」）

- **列表卡片 / 我的动态的正文预览**（`utils/forumDisplay.getContentPreview`：截断 80 字 + 去换行）仍**直出原串** ——
  它是预览不是正文承载面，票面 P2 只点名正文与回复卡；要收口须把格式下发到列表 DTO 与卡片 props（另一批改动）。
- 图片墙 / 属地 / 采纳等既有面本轮未改（只做正文与回复卡的格式轴接线）。
