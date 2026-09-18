# 存量 uvue 页面 `scroll-view` 上的 `white-space` 死声明清零（#1113）—— ①a Android 真机截图取证

- **PR**：#待补（按维护者裁定**先验后开**：本次取证先于 PR，目录名在开 PR 后即改为 PR 号）
- **复测对象**：分支 `fix/1113-uvue-white-space` HEAD **`89daf017`**（其下为 `4f9b7933`；两次提交之后未再改运行时面，本目录随之入库）
- **日期**：2026-09-18
- **设备**：`b32d8398`（Xiaomi `23049RAD8C` / Android 15，USB）
- **包名**：`io.dcloud.uniappx`（uni-app-x 调试基座）
- **项目目录**：取证时用唯一名 `fl-mobile-wt1113`（HBuilderX 按**项目名**解析；本机 `cli project list` 有 8 个同名项目，不改名会误 publish 到别的工作树）
- **执行人**：agent 执行（本目录只承载 ①a；**①b 未命中能力面** —— 本票 6 个改动文件均**不在** `scripts/lib/capability-surface.ps1` 的路径白名单内，故无人工签收项）

## 被验的改动（本票全部运行时面）

6 处 `white-space: nowrap` 写在 `<scroll-view>` 上（uvue 原生端该属性**只支持 `<text>` / `<button>`**，写在别处会被渲染层判错并**忽略** —— 真机日志原文见 #1081 的 ①a 实测）。逐个核过承载标签后删除，注释里解释该声明的句子同步删/改：

| # | 文件 | class | 承载标签（模板行） | 改动 |
| --- | --- | --- | --- | --- |
| 1 | `pages/profile/favorites.uvue` | `.filter-scroll` | `<scroll-view …>`:14 | 删声明 |
| 2 | `pages/profile/records.uvue` | `.filter-scroll` | `<scroll-view …>`:34 | 删声明 |
| 3 | `pages/profile/practice-records.uvue` | `.filter-scroll` | `<scroll-view …>`:39 | 删声明 |
| 4 | `pages/featured/featured-list.uvue` | `.filter-scroll` | `<scroll-view …>`:14 | 删声明 |
| 5 | `pages/ai-assistant/ai-feature.uvue` | `.diag-chips` | `<scroll-view …>`:43 **与** :49（同 class 两处） | 删声明 + 改注释 |
| 6 | `pages/exam/mock-exam.uvue` | `.palette-scroll` | `<scroll-view …>`:43（答题中分支） | 删声明 |

**不动**：`pages/profile/personal-info.uvue` 的 `.code-btn-text`（承载是 `<text>`，**合法**）—— 本票只把它从 `AGENTS.md` 的「存量缺陷」名单里摘出来写成合法正例，见 #1113 的 Q1=(A) 裁定。

## 取证手法

- **部署**：`npm run hx:run`（真运行）→ 机检行
  `HX_RUN mode=incremental compile=110 deploy=0 total=122 exit=ok`；
  `HX_RUN_DEPLOY deployed=true www=/sdcard/Android/data/io.dcloud.uniappx/apps/__UNI__1C1D180/www=1789698671->1789700791 pid_before=io.dcloud.uniappx=29411 pid_after=io.dcloud.uniappx=30629 foreground=io.dcloud.uniappx reason=资源已落到设备：… 的 mtime 相对基线前进`（判据是**设备侧事实相对基线前进**，不是「前台是不是基座」）。
- **进页**：复用 `scripts/lib/auto-screenshot.ps1` 的 `Start-NavLaunchDetached`（分离派发）+ `Wait-NavSettled`
  （页身份 + 非全黑 + 相邻帧宽容一致 ≤0.5%），自己拼 `--pageQuery` —— 该库只拼 `--pagePath`（ADR-0008 补遗第 5 条），
  而本票有一页是参数化页（`pages/ai-assistant/ai-feature?featureKey=fault_diagnosis`，无键时**故意 fail-closed 弹回**）。
  6 次导航**全部落定**（41 / 91 / 85 / 85 / 90 / 93 秒，相邻帧差异**均为 0%**），页身份逐次命中请求页。
- **截页**：`adb exec-out screencap -p`（PNG）→ JPEG q75 / 宽 720 入库；每张旁边配一份 `<同名>.content-desc.txt`：
  uvue 的文字在 a11y 树里走 **`content-desc`**、不走 `text`（ADR-0008 补遗第 6 条），故文本判据一律取它。
- **横滑判据**：先看同一行 chip 的 `bounds` —— **y 完全相同 = 不换行**、右缘触到屏宽 = 溢出；
  若有溢出则再 `adb shell input swipe` 横滑一次并重取 dump，看**可见 chip 编号是否前移**（位移证据）。
- **`input` 可注入性每次现测**：本次 `input tap` / `input swipe` 均可用（`shell "input tap …; echo TAP_RC=$?"` → `TAP_RC=0`，
  logcat 无 `INJECT_EVENTS`）—— 继 ADR-0008 补遗第 3 条在本设备上的第 3 个数据点。

## 逐页结果（after）

| # | 页面 | 被删声明的 class | 截图 / 文本判据 | 不换行判据（chip 的 `bounds` y 区间） | 溢出与横滑 |
| --- | --- | --- | --- | --- | --- |
| 01 | `pages/profile/favorites` | `.filter-scroll` | `01-favorites-chips-after.jpg` / `.content-desc.txt`（28 条） | **6 个 chip 全部 y=312..358** | 末个 `题目` 右缘 **936 < 屏宽 1080** ⇒ 当前数据下无溢出 |
| 02 | `pages/profile/records` | `.filter-scroll` | `02-records-chips-after.jpg`（41 条） | **4 个 chip 全部 y=499..549** | 末个 `近90天` 右缘 809 ⇒ 无溢出 |
| 03 | `pages/profile/practice-records` | `.filter-scroll` | `03-practice-records-chips-after.jpg`（25 条） | **4 个 chip 全部 y=506..556** | 末个 `判断题` 右缘 792 ⇒ 无溢出 |
| 04 | `pages/featured/featured-list` | `.filter-scroll` | `04-featured-list-chips-after.jpg`（14 条） | **5 个 chip 全部 y=312..358** | 末个 `资讯` 右缘 986 ⇒ 无溢出 |
| 05 | `pages/ai-assistant/ai-feature`（诊断，展开筛选后） | `.diag-chips` | `05-ai-feature-diagnosis-chips-after.jpg`（折叠态 7 条）+ `05b-…-model-row.content-desc.txt` | **品牌行 4 个 chip 全部 y=1847..1893**；车型行同 y=1847..1893 | 品牌行末个 `林德叉车 (Linde)` 右缘 **1045 触屏宽被裁**（后端品牌表不止 4 个）⇒ 溢出；横滑后可见项变为 `杭叉 / 林德 / 比亚迪 (BYD)`（见 `05c-…-swiped.content-desc.txt`，同一 y=1847..1893） |
| 06 | `pages/exam/mock-exam`（答题中分支） | `.palette-scroll` | `06-mock-exam-palette-after.jpg`（22 条） | **可见 chip 1..11 全部 y=370..416**（共 40 题） | 溢出（40 × ~98px ≫ 1080）；横滑 `input swipe 900 393 → 150 393` 后可见项由 **1..11 变为 12..23**（`06b-…-before-swipe` / `06c-…-after-swipe` 两份 dump 逐字对照），**y 仍恒为 370..416** |

**「不换行 + 可横滑」的判据形态（写实）**：6 处横滑行里，**4 处 `.filter-scroll` 在本设备（1080px 宽）上 chip 行不溢出**
（末个 chip 右缘 792–986 < 1080）⇒ 当前数据下**没有可滚动的内容**，「可横滑」在这 4 页上只到「不换行」这一半；
**溢出与真实横滑位移**在 `ai-feature` 的两个 `.diag-chips`（品牌行 / 车型行）与 `mock-exam` 的 `.palette-scroll` 上都有实测证据。
横滑本身与 `white-space` 无关（该属性被渲染层忽略），靠的是同一 class 上的 `flex-direction: row` + 子项 `flex-shrink: 0` —— 这两条在本票里**一行未动**。

## 应用控制台错误行（after，按 ADR-0012 口径在 launch 日志里找）

| 页面 | launch 日志行数 | `style property` 错行 | 页身份命中 | `FAIL` / `Uncaught` / `Exception` / `TypeError` |
| --- | --- | --- | --- | --- |
| favorites | 32 | **0** | 1 | 0 |
| records | 150 | **0** | 1 | 0 |
| practice-records | 150 | **0** | 1 | 0 |
| featured-list | 145 | **0** | 1 | 0 |
| ai-feature（含展开筛选） | 144 | **0** | 1 | 0 |
| mock-exam（答题中） | 161 | **0** | 1 | 0 |

## 两页「带状态」的取证细节（#1113 Q3 要求：不给态就是空跑假绿）

- `ai-feature` 的 `.diag-chips` 在 `v-if="filterOpen"` 面板内（`filterOpen = ref(false)`）⇒ 进页不渲染。
  本次先深链 `--pageQuery featureKey=fault_diagnosis` 落定，再**点「展开」**（坐标取 dump 的 bounds 中心 `1000,2021`），
  展开后 dump 里出现**两行 chip**（品牌行 + 选品牌后的车型行）—— 证明该态确实被造出来了。
- `mock-exam` 的 `.palette-scroll` 只在**答题中分支**（`<scroll-view v-else class="content-scroll">`）内 ⇒ 进页不一定渲染。
  本次进页后 `boot()` 组卷成功（dump 里 `1 / 40`、`已答 0 / 40`、题号 1..11 可见）—— 同样是「态已造出」的证据。

## 诚实声明（本目录**没有**验证到的面）

1. **改前对照（红控制）本轮没做成，故「改前同页会打这条 error 行」在本目录里没有直接证据。**
   尝试过：`git checkout origin/master -- <6 文件>` 还原声明 → 重编译部署，但 **HBuilderX 的增量编译复用了旧产物**
   （实测 `unpackage/dist/build/.uvue/…/favorites.uvue` 与 `unpackage/cache/.app-android/src/…/favorites.kt` 里
   `white-space` 计数恒为 0，与源文件里确有该声明不符；删掉这两处 per-page 产物后再跑、以及 `hx:run -Full`
   （干净缓存重建，`compile=110`）之后，**设备侧 `www/pages/**/classes.dex` 里仍为 0**）。⇒ 本目录只主张
   **「改后这 6 页的 launch 日志里该错行为 0，且改动的行确实渲染了」**；「改前为 ≥1」的依据是 **#1081 的同源实测**
   （同一台设备族、同一属性、同一承载标签 `<scroll-view>`，日志原文见 #1110 的 `docs/verification/help-center/1110/README.md`）
   与**本票改动集本身**（把这几处声明逐行删掉即该错行的唯一来源），**不是**本目录亲自跑出来的红控制。
2. **4 页 `.filter-scroll` 没有「真的横滑起来」的证据**（理由见上表的写实判据：当前数据下不溢出）。
   要造成溢出需要更窄的屏或更多筛选项，本次没造 —— 「不换行」这半有逐 chip 的 y 判据。
3. **`ai-feature` 品牌行 / 车型行与 `mock-exam` 题号条的横滑位移只有 `content-desc` 判据，没有单独截图**：
   截图是静态的，位移结论来自前后两份 dump 的逐字对照（`05c` / `06b` / `06c`）。
4. **截图内容 agent 不据此宣称已核对**：可核的是上面那份 `content-desc` 文本判据与日志判据；截图的视觉判断留给签收人。
5. **H5 / 小程序端未验**：本仓该判据的对象是**原生端**（uvue 渲染器的行为），H5 端本次未实测、也不宣称。
6. **`pages/profile/personal-info.uvue` 的合法点未上机**：本票**没动**该文件（Q1=(A)），它的 `<text>` 承载由
   `utils/uvueWhiteSpaceContract.test.js` 的登记表 + 源码判据覆盖，不在本次真机范围。

## 复现命令

```powershell
# 0) 项目目录先改成唯一名（HBuilderX 按项目名解析；本机有 8 个同名项目）
& "D:\软件\HBuilderX.5.23.2026080626\HBuilderX\cli.exe" project close --path D:\FL\wt-1113\training-app\fl-mobile-wt1113
Rename-Item "D:\FL\wt-1113\training-app\叉车维修培训学员端跨端应用" "fl-mobile-wt1113"

# 1) 部署本分支到真机（真运行；设备侧事实相对基线前进才算到）
npm run hx:run        # 见上方 HX_RUN / HX_RUN_DEPLOY 机检行

# 2) 逐页深链 + 落定 + 截图 + dump（复用库里已验证的落定判据；本票页表写死在脚本里）
pwsh -NoProfile -ExecutionPolicy Bypass -File .ci-verify/wt1113-capture.ps1 -All -MinSeconds 20

# 3) 两个「带状态」页的交互（坐标一律取 dump 的 bounds 中心）
$adb = "D:\android-sdk\platform-tools\adb.exe"
& $adb -s b32d8398 shell "input tap 1000 2021; echo TAP_RC=$?"            # ai-feature 的「展开」
& $adb -s b32d8398 shell input swipe 900 1870 200 1870 300               # 品牌行横滑
& $adb -s b32d8398 shell input tap 396 1870                              # 选一个品牌 → 车型行出现
& $adb -s b32d8398 shell input swipe 900 393 150 393 300                 # mock-exam 题号条横滑
& $adb -s b32d8398 shell uiautomator dump --compressed /sdcard/wt1113.xml
& $adb -s b32d8398 exec-out screencap -p > shot.png

# 4) 收口后改回原名（改名被项目自身的常驻 cli 箝住时，只结束命令行里引用本项目路径的那个 cli.exe）
cli.exe project close --path D:\FL\wt-1113\training-app\fl-mobile-wt1113
Rename-Item "D:\FL\wt-1113\training-app\fl-mobile-wt1113" "叉车维修培训学员端跨端应用"
```

> **会话式坑位披露（两条，均按 ADR-0008 补遗第 8 点的最小面执行）**
> 1. **改名被拒 → 只结束 1 个 `cli.exe`**：`cli project close --path …` 返回「项目关闭完成」后 `Rename-Item` 仍报
>    `being used by another process`；逐个核对 `CommandLine` 后确认只有 **1 个** cli 引用本项目
>    （`launch app-android --project …fl-mobile-wt1113 --cleanCache true --deviceId b32d8398`，pid 33528），**只结束了它**。
>    全程**未碰** `HBuilderX.exe`（PID 22100 自始至终存活）、**未碰**任何别的会话的 cli。
> 2. **`pwsh -File` 传数组实参是坑**：`-Pages 'a','b'` 会被当成**一个**字符串（带引号）传进来，页面路径全废；
>    故本票的页表写死在取证脚本里（`-All`），命令行只传开关。同一族坑位还让 6 个页名里的最后一个带上尾引号。
