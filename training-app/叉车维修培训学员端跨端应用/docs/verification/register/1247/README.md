# #649（T11 register 模块手术）①a 真机取证

**页面**：`pages/register/register`（本票唯一改动页）
**设备**：`23049RAD8C` / Android 15 · adb `192.168.0.212:43057` · 前台 Activity `io.dcloud.uniappx/…UniPortraitPageActivity`
**构建**：本 worktree（`D:\FL\wt-649\training-app\叉车维修培训学员端跨端应用`），分支 `refactor/register`
**日期**：2026-09-21
**登录态**：取证前由维护者在真机上**退出登录** —— register 页在登录态下会被 `onLoad` 的 `reLaunch` 弹回仪表盘（ADR-0008「①a 选页规则」），登出后目标页才可达。

## 术前 / 术后怎么取的

| 轮次 | 树 | 手法 | 页身份（设备侧证据） |
| --- | --- | --- | --- |
| 术前 | `origin/master` 的 `register.uvue` 临时回填后部署（`npm run hx:run`） | `cli launch … --pagePath pages/register/register` | launch 日志 `16:22:09.496 进入页面:pages/register/register` |
| 术后 | 本分支构建（`dev:finish -Level standard` 部署） | 同上（`dev:finish` 步骤 6） | `导航已落定（243s，已进入目标页且画面稳定（相邻两次采样差异 0% ≤ 0.5%））` |

> 术前那一轮为什么临时回填：①a 要的是**前后对比**，而设备上只有一棵树。回填后 `npm run hx:run` 部署出的就是 master 的注册页（同一设备、同一密度、同一登录态），取完即 `git checkout --` 还原（工作树无残留）。

## 判据与结果（全部机器可复核）

### ① 页身份

两轮都在 launch 日志里命中**目标页**的页面进入行（`进入页面:"pages/register/register"`）；`dev:finish` 的落定判据同时要求「非全黑 + 画面稳定」，本轮 `相邻两次采样差异 0%`。

### ② 内容与几何：a11y 文本树（含 bounds）**逐字节相同**

`adb exec-out uiautomator dump --compressed /dev/tty` 的产物（uvue 的文字在 a11y 树里是 `content-desc`）：

```
SHA256  术前 = DB27314DFD73A43B6F38B9EEBD82C0E9F77F79AAA28946D402F23B66885B0D5A
SHA256  术后 = DB27314DFD73A43B6F38B9EEBD82C0E9F77F79AAA28946D402F23B66885B0D5A   ← 相同
```

树里含**每个节点的 bounds** ⇒ 元素集合、文本、排版几何三者一致。文本集合（两轮相同）：

```
注册 / 注册账号 / 手机号注册，验证码通过后自动登录 / 手机号注册 / 邮箱注册 / 请输入昵称 / +86 /
请输入 11 位手机号 / 请输入验证码 / 获取验证码 / 图形验证码 / 6 位数字验证码，5 分钟内有效 /
设置密码（6-20 位）/ 显示 / 请再次输入密码 / 同意 /《用户协议》/ 和 /《用户隐私》/ 注 册 /
已有账号？/ 返回登录
```

> 其中「手机号注册，验证码通过后自动登录」来自 composable 的 `subtitle` computed，「6 位数字验证码，5 分钟内有效」来自 `codeHint` 默认档 —— 即术后这些值确实由 `reg.<成员>.value` 形态取到并上屏。

### ③ 像素：正文零漂移（差异 = 每次进页随机生成的图形验证码）

`node scripts/lib/png-diff.mjs`（阈值 0.005）：

| 对比 | JSON 结论 |
| --- | --- |
| 术前 ↔ 术后（全帧） | `different=34450 ratio=0.013291 changed=true` |
| 术前 ↔ 术后（`--ignore-top-rows 90`，去掉状态栏时钟/网速） | `different=27090 ratio=0.010859 changed=true` |
| **对照 A**：同一页面加载连拍两张（同构建） | `different=0 ratio=0 changed=false` |
| **对照 B**：术前 ↔ 术前点图刷新验证码后（`refreshCaptcha`） | `different=28331 ratio=0.01093 changed=true` |

差异定位（逐像素包围盒 + 行带/列带分布）：

```
术前 ↔ 术后      : bbox x 67→1008  y 28→1594 | y0-99:7360  y1400-1499:2580  y1500-1599:24510
对照 B（刷验证码）: bbox x 145→998 y 32→1594 | y0-99:1241  y1400-1499:2580  y1500-1599:24510
```

⇒ 正文区域（`y1400-1599` / `x700-999`）在两行里是**同一处、同一数量（27,090 像素）**：那正是图形验证码图片（`.captcha-img`，180rpx×96rpx，位于 a11y 里 `图形验证码` 输入框右侧）——该图**每次进页由后端随机生成**，术前术后各拉了一张不同的图。状态栏（`y0-99`）是时钟/网速读数。
**结论：正文区域零像素漂移。**

### ④ logcat

`adb logcat -d` 全缓冲区：`FATAL EXCEPTION` = **0**、`ANR in` = **0**（当前窗口无崩溃、无 ANR）。

## 产物

- `register-before.jpg`（术前，master 构建）SHA256 `B0702499…`（原图 1080×2400 PNG）· `68 KB @720w q75`
- `register-after.jpg`（术后，分支构建）SHA256 `6CBCF6BC…`（同上）· `68 KB @720w q75`

> 入库形态按 ADR-0008 的截图纪律：宽 ≤720、单张 ≤150 KB（PNG 超限 ⇒ 无 WebP 编码器时退 JPEG q75）。

## 复现

```powershell
# 术前基线（可选，需先 place origin/master 的 register.uvue）
git -C D:\FL\wt-649 show origin/master:"training-app/叉车维修培训学员端跨端应用/pages/register/register.uvue" > <项目>\pages\register\register.uvue
npm run hx:run                                   # 部署术前树
pwsh -File <临时驱动>.ps1 -Phase before           # cli launch --pagePath pages/register/register + 落定 + 截图
# 术后
git -C D:\FL\wt-649 checkout -- "training-app/叉车维修培训学员端跨端应用/pages/register/register.uvue"
npm run dev:finish -- -Level standard -Device 192.168.0.212:43057
# 判据
node scripts/lib/png-diff.mjs --a <术前.png> --b <术后.png> --threshold 0.005 --ignore-top-rows 90
```
