# ① 真机验证 · PR #1261（#1240 P3 论坛输入区形态：共享输入区组件 + 工具栏 + 拍照/相册双入口）

设备：Xiaomi `23049RAD8C`（`marble`），Android 15，HBuilderX 调试基座（包 `io.dcloud.uniappx`）——
按移动端 `docs/adr/0008-移动端验收门与证据.md` 的 ① 门（**①a：agent 出证**）留档。

**接入方式**：**无线调试**（`adb mdns services` 发现 `adb-b32d8398-ul54S3._adb-tls-connect._tcp` →
`192.168.0.212`）。`hx-run.ps1` 机检行见下「部署判据」。设备当时已登录（基座内直接进到 tab 首页），
本轮**未触碰任何凭证**。

**部署判据（不是门、是①a 的部署事实）**：
`HX_RUN_DEPLOY deployed=true www=/sdcard/Android/data/io.dcloud.uniappx/apps/__UNI__1C1D180/www=1789997633->1790038141
pid_before=io.dcloud.uniappx=25261 pid_after=io.dcloud.uniappx=27030 foreground=io.dcloud.uniappx
reason=资源已落到设备…mtime 相对基线前进`；`HX_RUN mode=incremental compile=130 deploy=0 total=144 exit=ok`。

## 承载面与判据（不靠肉眼看「像不像」，都可机检）

承载面 = **发帖页 `pages/forum/forum-create.uvue` 的输入区**（形态由本票新增的共享组件
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
| 7 | **属地披露未被形态迁移弄丢**（ADR-0045 明确例外） | `01` / `02` / `03` / `04` 四份 dump **都**含 `发布内容会显示 IP 属地` |
| 8 | **logcat**：不写「E = 0」，写实数与相关性 | `logcat.txt`：全量 8170 行 / 带 `E/` 207 行 / **app 自身 tag 40 行**；这 40 行里命中本次改动关键词（forum\|markdown\|toolbar\|textarea\|compose\|input）的条数 = **0**；逐条形态已列（`No package ID …` × 23 与 `UniDomManager` 批次耗时行 × 17，均为运行时的既存噪声，与输入区无关） |

## 图片

| 文件 | 说明 |
| --- | --- |
| `01-forum-create-text.jpg` | 发帖页**纯文本档**（默认档；已登录，`content_format` 记忆位读不到 ⇒ 纯文本）—— 可见数据档位胶囊与图片双入口，**无** tab / 工具栏 / 边界提示行 |
| `02-forum-create-markdown.jpg` | 同一页切到 **Markdown 档** —— 编写\|预览 下划线 tab、五枚语法记号工具栏、边界提示行 |
| `03-forum-create-toolbar-insert.jpg` | 点工具栏 `###` 之后的正文（`### E01code`，计数 11/10000） |
| `04-forum-create-preview.jpg` | 「预览」档 —— 源串 `### ` 不再显示，标题按块渲染 |
| `05-forum-create-longpress-held.jpg` | 长按「按住」帧 —— 与同基线帧**逐像素 0 差异**（见下「未覆盖面」的长按对照实验） |

> 压缩口径：本机无 WebP 编码器，退 **JPEG q75、宽 720**（5 张 / 合计约 297 KB，均 ≤150 KB）。

## 手法（可复现）

```powershell
# 1) 无线调试接入（mDNS 自动发现；旧端口每次都变，别照抄地址）
adb mdns services                       # -> adb-b32d8398-…._adb-tls-connect._tcp  192.168.0.212:<新端口>
# 2) 部署本分支到真机（真运行，不传 --compile）
npm run hx:run                          # 机检行 HX_RUN / HX_RUN_DEPLOY
# 3) 深链到目标页（不必点着导航走）；注意这会触发一次增量编译（约 2 分钟）
cli.exe launch app-android --project <项目> --deviceId <mDNS transport> --pagePath pages/forum/forum-create
# 4) 取证：坐标一律取 uiautomator dump 的 bounds 中心，不用截图目测
adb exec-out uiautomator dump /dev/tty > page.xml     # 未压缩
adb exec-out screencap -p > page.png
adb shell "input tap <cx> <cy>; echo TAP_RC=$?"       # 先现测可注入
```

**坑位（本轮实测，值得记）**：
- **软键盘会盖住工具栏** —— 在正文里敲完字后直接找 `###` 的 bounds 会**找不到节点**（a11y 树里没有）。
  先 `input keyevent 111`（ESC）收键盘再 dump。
- **深链会重新编译**：`cli launch --pagePath` 之后要等约 2 分钟（增量编译）才切页，
  早 dump 只会拿到上一页；用「轮询 a11y 直到文本集合变化」判到位，不要用固定 sleep。

## 未覆盖面（如实登记，勿读成「已覆盖」）

1. **回复栏（`forum-detail` 底部）本轮取不到证**：该环境的论坛**一篇帖子都没有** ——
   广场（`全部帖`）、知识问答、备考经验三个 tab 逐一实测都渲「还没有帖子，来发第一帖吧」，
   于是**打不开详情页**、回复栏进不去。回复栏与发帖表单**共用同一个组件**（本票的设计），
   故形态本身已由上面四份 dump 承重；**回复栏的版面与限额（≤3）留给 ①b 人工确认**。
   本轮**没有**为了取证去发一条测试帖（那是对生产论坛的写操作，不在授权范围内）。
2. **长按中文提示未能证明触发**：`@longpress` 是**本仓首用**（全仓零先例），且该按钮位于
   `scroll-view` 内（长按与滚动容器的手势冲突是已知问题域）。会话内实测的**对照实验**：
   - 探针有效性（对照）：空标题点「发布」触发既知 `uni.showToast('请输入标题')` ⇒ 与基线帧
     **345 个采样点有差异**（`bbox=(495,30)-(1062,1197)`）⇒ **探针看得见 toast**；
   - 被测：在 `###` 的精确 bounds（`162,1776`）上按住 1s（`input motionevent DOWN` … `UP`）⇒
     与基线帧 **0 个采样点差异**；`input swipe`（同点 / 2px 位移，800/1400/2000 ms）同样 0。
   - 另实测：uvue 的 toast **不产生独立窗口**（已知 toast 前后 `dumpsys window windows` 集合无变化），
     故 adb 的无障碍/窗口两条路都看不见 toast 文本。
   ⇒ 结论只到「**注入的长按在会话内未产生任何可见变化**」，**不能**据此断言 `@longpress` 失效
   （也**不能**断言它生效）。**交 ①b 人工长按确认**：按住工具栏任一按钮约 1 秒，应弹出该按钮的中文名
   （`###`→标题、`-`→列表、`>`→引用、` ``` `→代码块、`---`→分隔线）。
3. **拍照 / 相册点下去之后的相机与运行时权限弹窗**属**能力面** ⇒ **①b 由人签**（本行的 ①a 只覆盖
   「两个入口在场」）。已知事实：`manifest.json` 里**没有任何 android 权限声明**，改动前四处
   `chooseImage` 走的都是 `['album','camera']` 系统选择器；本票首次出现**单独的相机直达入口**
   （`sourceType: [source]`），故相机权限弹窗**必须**在 ①b 里人眼过一遍。
4. **「我的动态」的回复原文**（本票同批收口的 P2 缺口）同样取不到证 —— 同上，环境里没有帖子/回复。
5. **列表卡片正文预览**（80 字截断）仍直出源串：P2 既定裁定属另一批，本票如实登记为未覆盖面
   （见移动端 ADR-0025 实施回记 P3 的「已知偏差」）。
