# #1071 / PR #1075 —— ①a 真机逐页截图取证

- **设备**：`b32d8398`（Xiaomi 23049RAD8C / `marble`），adb `D:\android-sdk\platform-tools\adb.exe`
- **构建**：分支 `fix/1071-icon-glyph-restore`，HBuilderX `cli launch app-android` 编译并下发
  （设备侧凭证：`apps/__UNI__1C1D180` mtime 前进、`Token validated, isLoggedIn=true`、后端各接口 `200`）
- **取图方式**：`adb exec-out screencap -p`（**只读**；未注入输入事件、未 kill 进程、未清 logcat）

> ⚠️ **本轮取证走过的三处弯路，已订正（留痕以免后续会话重踩）**
>
> 1. **取图时刻必须等于被验证的树**：首轮五张图摄于 `5b9d3c93`，而其后提交又改了
>    `my-forum`（💬/👁）与 `forum-topic-header`（💬 回复数）⇒ 图片与被验代码对不上。
>    按 ADR-0008「一次真机运行只反映一棵树」，**已在 HEAD 重采全部页面**。
> 2. **`cli launch --pagePath` 的 query 键必须按页面 `onLoad` 的取值写**：`course-detail.uvue`
>    读的是 `options['id']`（**不是** `course_id`）。首轮传 `?course_id=8` ⇒ `id` 缺失 ⇒ 页面走空态，
>    采到的是「没数据的课程详情」。已改用 `?id=8`（`forum-detail` 同为 `?id=<topicId>`）。
> 3. **参数化页面要先用真实数据确认参数可用**：`chapter_id=1` 实测后端 **404**
>    （`章节不属于该课程`，课程 8 的真实章节是 **22–28**）⇒ 采到 15 KB 的错误空页。
>    已改用 `chapter_id=22` 重采。

## 为什么不用 `scripts/device-capture.ps1`

其只读路径**只采当前前台那一页**，而切页分支在本机机械不可用（`input` 注入被系统硬拒；
`uniapp://` deep link 实测让 App 抛未捕获异常退出）。故改用本会话实测的
`cli launch app-android --pagePath <path>`（直接编译 + 下发 + 打开目标页）。
**是否据此更换 ①a 的切页机制待维护者裁定** —— 本目录只记录事实，未改 `scripts/`。

## 页身份判据（为什么不用 `uiautomator`）

本 App 用 uniapp 渲染，`uiautomator dump` 只暴露一个 `ViewGroup`、**所有 `text` 为空**
⇒ 元素树判不出页面。故页身份以**该页 `onLoad` 会发出的 API 请求**为准（确定性）：

| 文件 | 请求的 `--pagePath` | 判定依据（该页特有的请求） |
| --- | --- | --- |
| `notifications.png` | `pages/notifications/notifications` | `GET /api/notifications` |
| `forum.png` | `pages/forum/forum` | 论坛列表接口 + 分类树 |
| `my-forum.png` | `pages/forum/my-forum` | 我的帖子 / 我的回复接口 |
| `forum-detail.png` | `pages/forum/forum-detail?id=15` | `GET /api/forum/topics/15`（**注意**：详情路由是 `/forum/topics/<id>`，不是 `/forum/topic/<id>`；后者 404） |
| `course-detail.png` | `pages/courses/course-detail?id=8` | `GET /api/course/8` + `/student/courses/8` + `/favorites/check?target_type=course&target_id=8` |
| `chapter-view.png` | `pages/courses/chapter-view?course_id=8&chapter_id=22` | `GET /api/course/8/chapter/22` + `/api/course/8` |

⚠️ **`chapter_id` 必须取真实存在的章节**：课程 8 的章节 id 是 **22–28**（不是 1）。
首次用 `chapter_id=1` 实测后端返回 **404**（`章节不属于该课程`）⇒ 采到的是错误空页（15 KB），已重采。
（这条也是本目录第 3 条弯路：**参数化页面要先用真实数据确认参数可用**，否则「图有了」其实什么都没验到。）

六张 PNG 的 SHA256 互不相同 ⇒ 排除 ADR-0008 记录的「切页静默失效 ⇒ 多页同哈希」形态。
**页身份均可由上表的该页特有请求确认**（不再有存疑页）。

## 本目录**未**主张的事（重要）

1. **本会话没有人目视核对过图内容**：取证会话所用模型不支持读图，OCR 与 vision 问答两条端点
   均被「未充值只能试 10 次」挡回。本目录只记录「图在哪、怎么来、该页发出了哪些请求」，
   **不主张图里图标已正确显示** —— 这一步必须由人打开图确认。
2. **章节附件类型图标（🎬 📄 📊 🖼️ 📎）本轮无法目视验证**：实测 `course_id=8` 的章节详情里
   **`file_url` 全为空、响应中也没有 `files` 条目** ⇒ 章节学习页的「附件资料」区块**无行可渲染**。
   （注意区分：响应里 7 处 `content_type: "document"` 是**章节自身**的记录，不是附件。）
   要验证这一族，需挑一门**章节挂了附件**的课，或造一条附件数据。
3. **课程详情封面大图标本就验不出差别**：后端已退役课程 `category`（`CourseDTO` 无该字段，
   `api/course.uts:39` 的 `toStr(obj['category'])` 恒为 `''`）⇒ `getCategoryIcon` 的 9 个分类分支
   **在真实数据下不可达**，该页固定走默认分支（📚）。此点已另立 **#1087**。
   旁证：`course-detail.png` 的封面底色取色为 `#E8F4FD`，正落在 `getCoverColor` 的**默认分支**
   （`#e3f2fd` 一族）—— 与「category 恒空」的结论一致。
4. **未覆盖 ①b 能力面**：本票未命中能力面（无指纹 / 运行时权限弹窗 / 真机上传 / 厂商 ROM 交互），按 ADR-0008 免。

## 请人复核清单（① 的判据在人）

逐张打开确认（重点看**图标字形是否真的画出来了**）：

| 图 | 看什么 | 对应 #1071 的哪一处 |
| --- | --- | --- |
| `notifications.png` | 通知条目左侧类型图标（💬 🚩 🗑️ 📝 🔔 之一）；列表为空时看空态 🔔 | `getTypeIcon` 6 分支 + 空态 |
| `forum.png` | 列表为空时看空态 💬；卡片操作栏的 💬 / 👁 | 空态 + `action-icon` |
| `forum-detail.png` | 统计行是否为 `♥/♡ 赞数 · ☆/★ 收藏 · 💬 回复数 · 👁 浏览数` | 本轮新增的 💬 回复数 + 补回的 👁 |
| `my-forum.png` | 计数行是否为 `❤️ 赞 · 💬 回复 · 👁 浏览` | 本轮补的 💬/👁 |
| `course-detail.png` | 封面大图标应为 **📚**（预期就是默认，见「未主张 3」） | 不可达分支（另见 #1087） |
| `chapter-view.png` | **预期看不到附件图标**（该课无附件，见「未主张 2」）；可看章节正文/导航是否正常 | 本轮无法覆盖 |

若某码位在真机渲染为豆腐块，按 ADR-0007 先例**只改该处字面量**（守护测试不钉死具体码位，无需改测试）。
