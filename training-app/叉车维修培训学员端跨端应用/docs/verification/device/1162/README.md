# ①a 真机验证截图 · PR #1162（issue #1140：章节页补收藏入口）

设备：小米 `23049RAD8C`（serial `b32d8398`），Android 15，**HBuilderX 调试基座**
（`npm run hx:run` 增量运行 → `cli.exe launch app-android --project <项目> --deviceId b32d8398
--pagePath … [--pageQuery …]` 深链进页）。判据口径：移动端 `docs/adr/0008-移动端验收门与证据.md`
（①a 由 agent 出证；「取证手法补遗」第 1/2/3/6 条）+ `0016`（①a 按「一次分支收口」跑一次）。

**被测树**：分支 `feat/1140-chapter-favorite`，head `d47a9d4a`。
**覆盖的验收面**：章节页可收藏 / 取消（进页状态回填正确）→ 「我的收藏 · 章节」出现该条 →
点开进入该章节且正文渲染（「正在加载...」消失）；复验后该页无新增错误行。

## 产物真换过（三处命中同一处改动，ADR-0008 手法 10）

| 处 | 判据 | 实测 |
| --- | --- | --- |
| 源文件 | `pages/courses/chapter-view.uvue` 含新增串 `toggleFavorite failed` | 命中 |
| 本地产物 | `unpackage/dist/build/**/pages/courses/chapter-view*` 含同一串 | 命中 |
| 设备产物 | `/sdcard/Android/data/io.dcloud.uniappx/apps/__UNI__1C1D180/www/pages/courses/chapter-view/classes.dex` 含同一串（`grep -r -l` 返回该文件） | 命中 |

部署判据（`hx-run.ps1` 机检行，**相对基线**）：`HX_RUN_DEPLOY deployed=true
www=/sdcard/…/www=1789715385->1789720011 pid_before=10537 pid_after=17821`，
`HX_RUN mode=incremental compile=110 deploy=0 total=132 exit=ok`。

## 图片

| 文件 | 说明 |
| --- | --- |
| `01-chapter-page-initial-heart-empty.png` | 深链进 `pages/courses/chapter-view?course_id=1&chapter_id=1`：正文渲染完成、头部右侧是**空心 ♡**（未收藏态；该态渲染的是 `'♡'`，不是空串 —— #1087 的教训） |
| `02-after-favorite-tap-heart-filled-toast.png` | 点 ♡ 之后：**实心 ♥** + toast「收藏成功」 |
| `03-favorites-list-chapter-row.png` | 「我的收藏」列表：**章节**行 `第一章 叉车分类与型号`（2026-09-18 新建的收藏）出现在列表首条（`ui-dump-favorites.content-desc.txt` 给出它的 bounds） |
| `04-chapter-view-rendered-from-favorite-row.png` | 点该章节行 → 进入 `chapter-view`：标题 + 「学习中 预计 40分钟」+ **正文渲染完成**（「正在加载...」已消失），且头部为 **♥** —— 即**进页状态回填正确**（这是一次全新的页面进入） |
| `05-after-cancel-tap-heart-empty-toast.png` | 在页面上点 ♥ 取消：回到**空心 ♡** + toast「已取消收藏」 |
| `06-favorites-after-cancel-row-gone.png` | 返回「我的收藏」：`第一章 叉车分类与型号` 行**已消失**（取消生效，同时把造出来的夹具还原） |

`ui-dump-favorites.content-desc.txt` 是 `uiautomator dump --compressed` 抽出的可见文案与 **bounds**
（点击坐标一律取 bounds 中心，**不按截图目测**）；`logcat-favorites-window.txt` 是同一时间窗的
设备侧接口事实。

## 判据（都是量出来的，不靠「像不像」）

1. **创建点真的落在 chapter 上**：logcat 的请求体逐字为
   `POST https://www.gccsmile.com/api/favorites  body: target_type=chapter target_id=1` → **201**。
   `target_type` 是本票最容易写错的一处（从课程详情抄接线会写成 `course`）。
2. **进页 / 切章后重查**：两次「进入 `chapter-view`」之后各有一次
   `GET /api/favorites/check?target_type=chapter&target_id=1` → 200；第二次进入（从收藏行点进来）
   的界面即为 ♥ ⇒ 状态来自后端而非本地记忆。
3. **列表消费面不再恒空**：「我的收藏」`GET /api/favorites?page=1&page_size=20` → 200，列表里
   出现该**章节**条目（截图 03）。
4. **取消走的是 favorite_id**：`DELETE /api/favorites/26` → 200（`26` = 后端返回的 `favorite_id`，
   不是章节 id）；取消后列表里该行消失（截图 06）。
5. **点击判据按画面 / ui dump 变化判**（不看退出码）：每步都以 `uiautomator dump` 的文案或
   截图变化为准。
6. **`input` 可注入性本次现测通过**：`input tap` / `input swipe` 均返回 `rc=True`、无
   `SecurityException`（ADR-0008 第 3 条要求每次现测）。
7. **无新增错误行**（本窗口，见 `logcat-favorites-window.txt`）：`FATAL EXCEPTION=0`、
   `ANR in io.dcloud.uniappx=0`、含 `[chapter-view]` 的告警/错误行 `=0`、
   `Favorite failed=0`、`[ERROR]/Uncaught/TypeError/ReferenceError=0`。窗口起点取**设备自身**最新
   日志行的 epoch 时间戳（`logcat -d -v epoch -t 1`），**未清缓冲**。

## 如实声明的边界（未做/未覆盖的，不在此宣称）

- **未命中能力面**（指纹 / 运行时权限弹窗 / 真机上传 / 厂商 ROM 交互）⇒ **①b 免**，本目录只有 ①a 产物。
- **未跑 ② 微信门 / ④b 云打包**：本 PR 未动 `manifest.json` / `pages.json` / `platformConfig.json`、
  无新增页面、无 `uni_modules` 改动 ⇒ 按现测免。
- **「未发布 / 未挂载课程的章节收藏被拒（400）」未在真机造出**：该判据由后端契约测试
  `backend/internal/api/favorite_chapter_visibility_contract_test.go`（PR #1135）断言，本票**只消费**
  （票面明确「不重复实现」），不据此声称真机验过。
- **本目录截图是原始分辨率的 PNG**（1080×2400，合计约 1.6 MB）：ADR-0008 的「宽 ≤720 / 单张 ≤150 KB /
  合计 ≤1.5 MB」是 `scripts/device-capture.ps1` **自动入库**那条路径的纪律；本目录是会话式 ①a 取证，
  沿 PR #1147 同一做法保留原始分辨率以便逐字核对。若后续要按自动入库口径收口，请重新压缩。
- **夹具已还原**：本轮造的数据只有「在章节 1 上建一条收藏」，验完取消（截图 06）⇒ 账号上的收藏列表
  与取证前一致（该账号原有的 `内燃机·总体构造与工作原理` 等条目未动）。
