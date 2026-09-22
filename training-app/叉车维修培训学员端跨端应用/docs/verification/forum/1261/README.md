# ① 真机验证 · PR #1261（#1240 P3 论坛输入区形态：共享输入区组件 + 工具栏 + 拍照/相册双入口）

设备：Xiaomi `23049RAD8C`（`marble`），Android 15，HBuilderX 调试基座（包 `io.dcloud.uniappx`）——
按移动端 `docs/adr/0008-移动端验收门与证据.md` 的 ① 门（**①a：agent 出证**）留档。

**证据绑定的树**：分支 `feat/1240` 的 head **`dffe7f75`**（`/code-review` 双轴评审的修复提交之后**重跑**）。
上一版证据绑定 `36ba80c6`；那之后图片流水线被抽成共享 picker、限额收到 `constants/app`、
档位值改为赋值处归一 —— **触及本页的交互面，故 ①a 按「一次分支收口」重跑一遍**，不跨版本沿用。

**接入方式**：**无线调试**（`adb mdns services` 发现 `adb-b32d8398-ul54S3._adb-tls-connect._tcp` @ `192.168.0.212`）。
设备当时已登录（基座内直接进到 tab 首页），本轮**未触碰任何凭证**。

**部署判据（不是门、是①a 的部署事实）**：
`HX_RUN_DEPLOY deployed=true www=/sdcard/Android/data/io.dcloud.uniappx/apps/__UNI__1C1D180/www=1790039361->1790042002
pid_after=io.dcloud.uniappx=19848 foreground=io.dcloud.uniappx reason=资源已落到设备…mtime 相对基线前进`；
`HX_RUN mode=incremental compile=150 deploy=0 total=198 exit=ok`。

## 承载面与判据（不靠肉眼看「像不像」，都可机检）

承载面 = **发帖页 `pages/forum/forum-create.uvue` 的输入区**（形态由共享组件
`pages/forum/components/forum-markdown-input.uvue` 渲染）。a11y 文本一律取自**未压缩**
`uiautomator dump`（`--compressed` 会把长 `content-desc` 丢成空串，P2 已实测踩到）。

| # | 判据 | 机检原文 |
| --- | --- | --- |
| 1 | **纯文本档不出现 tab / 工具栏 / 边界提示行**（ADR-0025 ⑥-5） | `01-forum-create-text.content-desc.txt`：有 `纯文本` / `Markdown`（数据档位胶囊）与 `拍照` / `相册`，**没有** `编写` / `预览` / `###` / `-` / `&gt;` / ` ``` ` / `---` / 边界提示行 |
| 2 | 切到 Markdown 档后**三者齐现**：编写\|预览 tab + 工具栏 + 只讲边界的提示行 | `02-forum-create-markdown.content-desc.txt`：多出 `编写` `预览` **与恰好五枚** `###` `-` `&gt;` ` ``` ` `---`，以及 `表格与图表显示原文；粗体、链接不出样式；图片用下方入口` |
| 3 | **工具栏按钮集 = 本档成员声明集**（无加粗/斜体/链接按钮 —— 本档不做行内渲染） | 同上：五枚即 `SUBSET_MEMBERS_FORUM`（heading / list / quote / code / divider），**没有**任何行内格式按钮 |
| 4 | **工具栏插入在真机生效**（按钮承诺的语法真被写进正文） | `03-forum-create-toolbar-insert.content-desc.txt`：正文 `text=` 由 `E01code` 变为 **`### E01code`**，计数由 `7/10000` 变为 **`11/10000`**（+4 = `### ` 的长度） |
| 5 | **编写/预览可切，且预览与发布同源** | `04-forum-create-preview.content-desc.txt`：切「预览」后 `text=` 里 **`### E01code` 整串消失**，块渲染产出 `E01code`（heading 块文本）；工具栏同时收起（只在编写档） |
| 6 | **图片双入口**（拍照 / 相册各一个直达入口） | `01` / `02` 两份 dump 都含 `拍照` 与 `相册` 两个节点（旧形态是单枚 `＋` 走系统选择器） |
| 7 | **属地披露未被形态迁移弄丢**（ADR-0045 明确例外） | `01` ~ `04` 四份 dump **都**含 `发布内容会显示 IP 属地` |
| 8 | **「拍照」直达系统相机、且应用内无权限弹窗**（能力面线索，供 ①b 用） | `05-camera-entry-after-tap.content-desc.txt` + 前台判据：点击后 `mCurrentFocus=Window{… com.android.camera/com.android.camera.OneShotCamera}`，a11y 是相机自己的 UI（`拍摄` / `前后置切换,后置` / `闪光灯，自动状态` / `1.0倍变焦`）—— **没有**出现应用内的权限申请弹窗 |
| 9 | **logcat**：不写「E = 0」，写实数与相关性 | `logcat.txt`：全量 15284 行 / 带 `E/` 834 行 / **app 自身 tag 73 行**；这 73 行里命中本次改动关键词（forum\|markdown\|toolbar\|textarea\|compose\|input\|picker\|image）的条数 = **0**；逐条形态已列（`No package ID …` 与 `UniDomManager` 批次耗时行，均为运行时既存噪声） |

## 图片

| 文件 | 说明 |
| --- | --- |
| `01-forum-create-text.jpg` | 发帖页**纯文本档**（默认档）—— 可见数据档位胶囊与图片双入口，**无** tab / 工具栏 / 边界提示行 |
| `02-forum-create-markdown.jpg` | 同一页切到 **Markdown 档** —— 编写\|预览 下划线 tab、五枚语法记号工具栏、边界提示行 |
| `03-forum-create-toolbar-insert.jpg` | 点工具栏 `###` 之后的正文（`### E01code`，计数 11/10000） |
| `04-forum-create-preview.jpg` | 「预览」档 —— 源串 `### ` 不再显示，标题按块渲染 |
| `05-camera-entry-after-tap.jpg` | 点「拍照」后 —— 前台已切到系统相机（应用内没有权限弹窗） |

> 压缩口径：本机无 WebP 编码器，退 **JPEG q75、宽 720**（5 张 / 合计约 271 KB，均 ≤150 KB）。

## 手法（可复现）

```powershell
# 1) 无线调试接入（mDNS 自动发现；旧端口每次都变，别照抄地址）
adb mdns services                       # -> adb-b32d8398-…._adb-tls-connect._tcp  192.168.0.212:<新端口>
# 2) 部署本分支到真机（真运行，不传 --compile）
npm run hx:run                          # 机检行 HX_RUN / HX_RUN_DEPLOY
# 3) 深链到目标页（不必点着导航走）；注意这会触发一次增量编译
cli.exe launch app-android --project <项目> --deviceId <mDNS transport> --pagePath pages/forum/forum-create
# 4) 取证：坐标一律取 uiautomator dump 的 bounds 中心，不用截图目测
adb exec-out uiautomator dump /dev/tty > page.xml     # 未压缩
adb exec-out screencap -p > page.png
adb shell "input tap <cx> <cy>; echo TAP_RC=$?"       # 先现测可注入
```

**坑位（两轮实测，值得记）**：
- **软键盘会盖住工具栏** —— 在正文里敲完字后直接找 `###` 的 bounds 会**找不到节点**（a11y 树里没有）。
  先 `input keyevent 111`（ESC）收键盘再 dump。
- **深链会重新编译**：`cli launch --pagePath` 之后要等（本轮增量编译 150 秒）才切页，早 dump 只会拿到上一页；
  用「轮询 a11y 直到文本集合变化」判到位，不要用固定 sleep。
- **无线调试会话会掉**：手机息屏 / 闲置后 `adb devices` 变空、`mdns services` 也消失；
  `adb kill-server && adb start-server` 后等几秒即重新发现（不必重新配对）。

## 未覆盖面（如实登记，勿读成「已覆盖」）

1. **回复栏（`forum-detail` 底部）本轮仍未取证**：该环境的论坛**一篇帖子都没有** ——
   广场（`全部帖`）、知识问答、备考经验三个 tab 逐一实测都渲「还没有帖子，来发第一帖吧」，
   于是**打不开详情页**。回复栏与发帖表单**共用同一个组件**（本票的设计），故形态本身已由上面四份
   dump 承重，`useReplyComposer` 的图片域也已由 `utils/forumImagePickerBehavior.test.js` 真执行覆盖；
   但**回复栏的版面、`≤3` 限额与发送闸门（`canSubmitReply`）仍需 ①b 人工确认**。
   本轮**没有**为取证去发测试帖（那是对生产论坛的写操作，不在授权范围内）。
2. **「拍照」拍下之后的那一段未验**：本轮只证到「入口直达系统相机、应用内无权限弹窗」；
   **按下快门 → 回到应用 → 上传成功**这一段需要人操作（按快门会把照片落到设备、并把它上传到生产后端，
   属写操作，不在本轮授权内）⇒ 归 **①b**。
3. **长按中文提示仍无法机检**：`@longpress` 是本仓**首用**（全仓零先例），且按钮位于 `scroll-view` 内
   （长按与滚动容器的手势冲突是已知问题域）。本轮在 **`dffe7f75`** 上重做的实验：
   - 稳定性对照（无输入、相隔 4 秒两帧）：差异 **39** 个采样点，bbox `(756,30)-(795,42)`（右上角时钟），基线稳定；
   - 被测（在 `###` 的精确 bounds `162,1776` 上按住 1.6 秒）：差异 **2553** 个采样点，bbox `(150,30)-(795,1281)`
     —— 范围**覆盖整页**而幅度极低（bbox 内仅约 2.8% 采样点变化），与「2px 横向滚动带来的重绘」**同形**；
     而一个居中的 toast 应当是**紧凑、高对比**的一块。⇒ **无法据此判定长按触发与否**（既不判失效、也不判生效）。
   - 另实测 uvue 的 toast **不产生独立窗口**（已知 toast 前后 `dumpsys window windows` 集合无变化），
     且**已实测 `adb` 看不见已知 toast**（对照：空标题点「发布」触发的既知 `showToast` 在像素面上有 345 个采样差异，
     故探针有效、但窗口/无障碍两条路都读不到 toast 文本）。
   ⇒ **交 ①b 人工长按确认**：按住工具栏任一按钮约 1 秒，应弹出该按钮的中文名
   （`###`→标题、`-`→列表、`>`→引用、` ``` `→代码块、`---`→分隔线）。具名映射本身已有用例钉住
   （`utils/markdownToolbarBehavior.test.js` 反向对账组）。
4. **「我的动态」的回复原文**（本票同批收口的 P2 缺口）同样取不到证 —— 同上，环境里没有帖子 / 回复。
5. **列表卡片正文预览**（80 字截断）仍直出源串：P2 既定裁定属另一批，本票如实登记为未覆盖面
   （见移动端 ADR-0025 实施回记 P3 的「已知偏差」）。
