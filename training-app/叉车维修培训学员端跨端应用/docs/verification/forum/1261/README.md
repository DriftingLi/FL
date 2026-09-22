# ① 真机验证 · PR #1261（#1240 P3 论坛输入区形态：共享输入区组件 + 工具栏 + **单入口图片**）

设备：Xiaomi `23049RAD8C`（`marble`），Android 15，HBuilderX 调试基座（包 `io.dcloud.uniappx`）——
按移动端 `docs/adr/0008-移动端验收门与证据.md` 的 ① 门（**①a：agent 出证**）留档。

**证据绑定的树**：分支 `feat/1240` 的 `2f4f74e1`（含形态回退提交 `f0ef4b8e`）。这是**第二次分支收口**：上一批证据绑在 `fa6881e8`，判的是**已被 ADR-0025 ⑨ 取代的「拍照 / 相册双入口」形态**，随回退整体作废 ⇒ 本批全部重取。

**接入方式**：无线调试（`adb mdns services` 发现 `adb-b32d8398-ul54S3._adb-tls-connect._tcp` @ `192.168.0.212`）；设备已登录，本轮**未触碰任何凭证**。

**导航方式**：本轮**不用**深链 `--pagePath`（见坑位 2），走**真实 UI 路径**：首页 → `交流` → `发帖`。

**部署判据（不是门，是 ①a 的部署事实）**：`HX_RUN_DEPLOY deployed=true www=/sdcard/Android/data/io.dcloud.uniappx/apps/__UNI__1C1D180/www=1790064062->1790064702 pid_before=… pid_after=… foreground=io.dcloud.uniappx reason=资源已落到设备…mtime 相对基线前进`；本轮另用 `cli launch app-android --cleanCache true` 做了一次**干净重建**（见坑位 1）。

## 承载面与判据（不靠肉眼看「像不像」，都可机检）

承载面 = **发帖页 `pages/forum/forum-create.uvue` 的输入区**（形态由共享组件 `pages/forum/components/forum-markdown-input.uvue` 渲染）。
a11y 文本一律取自**未压缩** `uiautomator dump`（`--compressed` 会把长 `content-desc` 丢成空串，P2 已实测踩到）。

| # | 判据 | 机检原文 |
| --- | --- | --- |
| 1 | **纯文本档不出现 tab / 工具栏 / 边界提示行**（ADR-0025 ⑥-5） | `01-forum-create-text.content-desc.txt`：有 `纯文本` / `Markdown`（数据档位胶囊）与 `发布内容会显示 IP 属地`，**没有** `编写` / `预览` / `###` / `-` / `>` / ` ``` ` / `---` / 边界提示行 |
| 2 | 切到 Markdown 档后**三者齐现**：编写\|预览 tab + 工具栏 + 只讲边界的提示行 | `02-forum-create-markdown.content-desc.txt`：多出 `编写` `预览` **与恰好五枚** `###` `-` `>` ` ``` ` `---`，以及 `表格与图表显示原文；粗体、链接不出样式；图片用下方入口` |
| 3 | **图片区是单入口**（⑨ 取代 ⑦）：**一个** `＋` 方块 + `n/9` 计数，**没有**「拍照」「相册」两个常驻方块 | `01` / `02` 两份 dump 均为 `＋` + `0/9`；本批**所有** dump 里都没有 `拍照` / `相册` 这两个入口（旧形态的实证留在 git 历史里的上一批证据） |
| 4 | **工具栏插入在真机生效**（按钮承诺的语法真被写进正文） | `03-forum-create-toolbar-insert.content-desc.txt`：正文 `text=` 由 `E01code` 变为 **`### E01code`**，计数由 `0/10000` 变为 **`11/10000`**（+4 = `### ` 的长度） |
| 5 | **编写/预览可切，且预览与发布同源** | `04-forum-create-preview.content-desc.txt`：切「预览」后 **`### E01code` 整串消失**，块渲染产出 `E01code`（heading 块文本）；工具栏同时收起（只在编写档） |
| 6 | **点 `＋` 弹的是「含两个来源的选择器」，不再直达相机**（⑨ 的核心差异） | `05-image-entry-picker.content-desc.txt`：出现 `拍摄` / `从相册选择` / `取消` 三项 —— 旧形态（`sourceType: ['camera']`）是**直达** `com.android.camera/OneShotCamera` |
| 7 | **相册入口能进，且应用内无权限弹窗** | `06-album-picker.content-desc.txt`：`io.dcloud.uts.dmcbig.mediapicker.PickerActivity`、标题 `选择图片`、右上 `完成(0/9)`（额度 9 与传给 `chooseImage` 的 `count` 一致）；前台窗口是该 picker、**没有**应用内权限申请弹窗 |
| 8 | **选图 → 回应用 → 上传成功**（①b 的写路径；本轮经维护者授权由 agent 执行） | `07-after-upload.content-desc.txt`：回到应用后缩略图出现（带 `×` 删除钮），计数变 **`1/9`** |
| 9 | **属地披露未被形态迁移弄丢**（ADR-0045 明确例外） | `01` ~ `04`、`07` 五份 dump **都**含 `发布内容会显示 IP 属地` |
| 10 | **logcat**：不写「E = 0」，写实数与相关性 | `logcat.txt`：窗口内共 **4743** 行 / `E/` **106** 行 / **app 进程 20 行**；这 20 行里命中本次改动关键词（forum\|markdown\|toolbar\|image\|picker\|upload\|input\|textarea\|chooseImage）的条数 = **0**；逐条形态已列（`UniDomManager` 批次耗时行等运行时既存噪声） |

## 图片

| 文件 | 说明 |
| --- | --- |
| `01-forum-create-text.jpg` | 发帖页**纯文本档** —— 可见数据档位胶囊、**一个 `＋` 方块 + `0/9`**、属地提示；**无** tab / 工具栏 / 边界提示行 |
| `02-forum-create-markdown.jpg` | 同一页切到 **Markdown 档** —— 编写\|预览 tab、五枚语法记号工具栏、边界提示行 |
| `03-forum-create-toolbar-insert.jpg` | 点工具栏 `###` 之后的正文（`### E01code`，计数 11/10000） |
| `04-forum-create-preview.jpg` | 「预览」档 —— 源串 `### ` 不再显示，标题按块渲染 |
| `05-image-entry-picker.jpg` | 点图片区 `＋` 之后 —— 弹出**含两个来源的选择器**（拍摄 / 从相册选择 / 取消） |
| `06-album-picker.jpg` | 「从相册选择」之后 —— uni-app x 图片选择器（`完成(0/9)`） |
| `07-after-upload.jpg` | 选 1 张「完成」后回到应用 —— 缩略图 + `×` + 计数 `1/9`（上传成功） |

> 压缩口径：本机无 WebP 编码器，退 **JPEG q75、宽 720**（7 张 / 合计约 439 KB，最大 102 KB）。

## 手法（可复现）

```powershell
# 1) 无线调试接入（mDNS 自动发现；端口每次都变，别照抄地址）
adb mdns services                       # -> adb-b32d8398-…._adb-tls-connect._tcp  192.168.0.212:<新端口>
# 2) 部署本分支到真机（真运行）。改过 composable 返回类型签名时**必须**先清缓存重建（见坑位 1）：
npm run hx:run
& "$hbx\cli.exe" launch app-android --project <项目> --deviceId <mDNS transport> --cleanCache true
# 3) 导航：**不用**深链，走真实 UI（首页 → 交流 → 发帖）；坐标一律取 uiautomator dump 的 bounds 中心
adb shell uiautomator dump /sdcard/cap.xml ; adb pull /sdcard/cap.xml page.xml   # 未压缩
adb shell screencap -p /sdcard/cap.png   ; adb pull /sdcard/cap.png page.png
adb shell "input tap <cx> <cy>; echo TAP_RC=$?"       # 先现测可注入
adb shell input text E01code ; adb shell input keyevent 111   # 敲字后先收键盘再找工具栏
# 4) 写路径（本轮由维护者授权）：点 ＋ → 从相册选择 → 点第一格 → 点「完成(1/9)」→ 回应用看缩略图/计数
# 5) logcat：**先 logcat -c**，再取窗口内的全量
adb logcat -c ; adb logcat -d > logcat.txt
```

**坑位（本轮实测，比上一批多两条，均已复现）**：

1. **改 composable 的返回类型签名后，内循环增量编译不重编调用方 ⇒ 该页白屏**。实证：`java.lang.NoSuchMethodError: No virtual method getOnAddImage()Lkotlin/jvm/functions/Function1; in class Luni/UNI1C1D180/UseForumImagePickerResult; … at GenPagesForumForumCreate.setup$lambda$13(forum-create.kt:51)` + `navigateTo: fail /pages/forum/forum-create?scope=discussion locked` —— 页面侧仍按**旧签名**（`onAddImage: (String) -> Unit`）编译，而 picker 侧已是 `() -> Unit`。**对策**：`cli launch app-android … --cleanCache true`（= GUI 的「干净缓存重建」）后重跑；`hx:run` 按设计**不做**干净重建 ⇒ 这类签名改动**必须**显式清缓存，否则会误判成「代码坏了」。
2. **深链 `cli launch --pagePath` 本轮不可靠**：app 会停在**空白的 `UniAppActivity`**（本轮等了 8 分钟未切页）；**且中途 kill 会把设备上的 `www` 推成半截 ⇒ 整个 app 白屏**（重跑一次 `npm run hx:run` 才恢复，实证：`www` mtime `1790062579→1790063449`）。⇒ 取证一律走真实 UI 导航，深链的编译段**绝不 kill**。
3. **软键盘会盖住工具栏**：在正文里敲完字后直接找 `###` 的 bounds 会**找不到节点**（a11y 树里没有）。先 `adb shell input keyevent 111`（ESC）收键盘再 dump。
4. **无线调试会话会掉**（手机息屏 / 闲置后 `adb devices` 与 `adb mdns services` **同时**变空）：`adb kill-server` 再 `adb start-server`、等几秒即重新发现，**不必重新配对**。

## 未覆盖面（如实登记，勿读成「已覆盖」）

1. **回复栏（`forum-detail` 底部）与「我的动态」的回复原文本轮仍未取证**：两者的形态与发帖页**共用同一个组件**（形态本身已由上面 7 份 dump 承重），但回复栏的**版面**、`≤3` 限额与发送闸门（`canSubmitReply`）仍需人看；「我的动态」的回复原文需要一篇带回复的帖子（本轮列表里已有帖子，但未做写操作发回复 —— 发帖/回复是对生产论坛的**内容**写操作，不在本轮授权内）。
2. **长按中文提示仍无法机检**（沿用上一批的对照实验：已知 toast 路径在像素面上有 345 个采样差异，而注入的长按 0 差异；uvue 的 toast **不产生独立窗口**、a11y 也读不到）⇒ **交 ①b 人工确认**：按住工具栏任一按钮约 1 秒应弹出其中文名（`###`→标题、`-`→列表、`>`→引用、` ``` `→代码块、`---`→分隔线）。**本批之后，①b 需要人眼的只剩这一条。**
3. **列表卡片正文预览**（80 字截断）仍直出源串：P2 既定裁定「属另一批」，本段如实登记（见移动端 ADR-0025 实施回记 P3 的「已知偏差」）。
