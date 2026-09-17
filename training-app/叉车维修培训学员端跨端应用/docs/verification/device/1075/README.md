# #1071 / PR #1075 —— ①a 真机逐页截图取证

- **设备**：`b32d8398`（Xiaomi 23049RAD8C / `marble`），Android，adb `D:\android-sdk\platform-tools\adb.exe`
- **构建**：分支 `fix/1071-icon-glyph-restore`（HEAD `0e748e04`），HBuilderX `cli launch app-android` 编译并下发
  （设备侧凭证：`apps/__UNI__1C1D180` 目录 mtime 前进到 09:40；`Token validated, isLoggedIn=true`；后端 `200 /api/notifications`）
- **取图方式**：`adb exec-out screencap -p`（**只读**；未注入输入事件、未 kill 进程、未清 logcat）
- **取图时间**：2026-09-17 09:58–10:33 +08:00

## 为什么不用 `scripts/device-capture.ps1`

其只读路径**只采当前前台那一页**，而它的切页分支在本机机械不可用（`input` 注入被系统硬拒；
`uniapp://` deep link 实测让 App 抛未捕获异常退出）。本次改用本会话实测可用的路径：
`cli launch app-android --project <abs> --deviceId <serial> --pagePath <path>` —— 它直接编译+下发+打开目标页。
（此路径为 `scripts` 未采用的机制，**是否据此更换 ①a 的切页机制待维护者裁定**；本文件只记录事实。）

## 五张图与「页身份」判据

页身份以 **logcat 的 `进入页面:` 行为准**（`uiautomator dump` 在本 App 上暴露不出文本节点：
uniapp 渲染成单个 `ViewGroup`，`text` 全空，故无法用元素树判页）。

| 文件 | 请求的 `--pagePath` | logcat `进入页面:` | 页身份 |
| --- | --- | --- | --- |
| `notifications.png` | `pages/notifications/notifications` | `pages/notifications/notifications` | ✅ 确认 |
| `forum.png` | `pages/forum/forum` | `pages/forum/forum` | ✅ 确认 |
| `my-forum.png` | `pages/forum/my-forum` | `pages/forum/my-forum` | ✅ 确认 |
| `course-detail.png` | `pages/courses/course-detail?course_id=8` | 该次会话未捕获到对应行（见下「页身份存疑」） | ⚠️ **存疑** |
| `chapter-view.png` | `pages/courses/chapter-view?course_id=8&chapter_id=1` | 曾出现 `pages/courses/chapter-view?course_id=8&chapter_id=1`（证明 query 参数可传递），但取图邻域另见 `pages/courses/courses` | ⚠️ **存疑** |

**页身份存疑（如实记，不外推为"已确认"）**：对 `course-detail` / `chapter-view` 两页，取图时刻的
logcat 邻域里同时出现 `pages/courses/courses`，无法确定截图落在目标页还是落了课程列表页。
另有两点旁证支持「五张确实各不相同」：**五张 SHA256 互不相同**
（`D6E7297E…` / `C565B7EC…` / `A871C96C…` / `B376CE6D…` / `A2CB4B6F…`），
且字节数不同 —— 这排除了 ADR-0008 记录的「切页静默失效 ⇒ 多页同哈希」形态。

## 本文件**未**主张的事（重要）

1. **未做目视核对**：本次取证会话所用模型不支持读图，且 OCR / vision 问答两条端点均被
   「未充值只能试 10 次」挡回 ⇒ **没有任何人（或 agent）看过这五张图的内容**。
   本文件只记录「图在哪、怎么来的、logcat 说打开了哪一页」，**不主张图里图标已正确显示**。
2. **未覆盖「课程详情封面大图标随分类变化」**：后端已退役课程 `category` 字段
   （`CourseDTO` 无该字段，`api/course.uts:39` 的 `toStr(obj['category'])` 恒为 `''`），
   `getCategoryIcon` 的分类分支**在真实数据下不可达**，该页固定走默认分支（📚）。
   即：该页属于「字形已修好、但真机上本就看不出差别」，本条验收目标的达成需要**另立票**
   （改用真实字段 `specialty_id` / `level_id` 驱动分类图标）。
3. **未覆盖 ①b 能力面**：本票未命中能力面（无指纹 / 运行时权限弹窗 / 真机上传 / 厂商 ROM 交互），按 ADR-0008 免。

## 请人复核（①a/①b 的判据在人）

打开上面五张 PNG，逐页确认：

- `notifications.png`：通知条目的**类型图标**（💬 🚩 🗑️ 📝 🔔 之一）是否显示为**彩色字形**（非空白方块、非豆腐块）；空态是否为 🔔
- `forum.png` / `my-forum.png`：空态图标是否为 💬；论坛卡片操作栏的评论/浏览图标是否为 💬 / 👁
- `course-detail.png`：封面大图标是否为 📚（**预期就是 📚**，见上「未主张」第 2 条）
- `chapter-view.png`：若有「附件资料」区块，每行左侧类型图标是否为 🎬 / 📄 / 📊 / 🖼️ / 📎（取决于该章是否有附件）

若某码位在真机渲染为豆腐块，按 ADR-0007 先例只改该处字面量即可（守护测试不钉死具体码位）。
