# #650（T12 forgot-password 模块手术）①a 真机取证

- **PR**：#1263（base `master` `40cb8c12`，head `7a025b84`）
- **页面**：`pages/forgot-password/forgot-password`（本票的**手术目标页**唯一 —— 分支另有 2 处共享出口的机械迁移，
  即 `pages/login/login.uvue` 与 `pages/register/composables/useRegisterForm.uts` 各 2 行取值形态改写，**不在本轮取证射程**。
  本行原写「本票唯一改动页」不准确，订正理由与判据见 ADR-0007:556；该缺口由 #1270 跟踪，其中 login 侧已由 #1288 的取证顺带闭合）
- **设备**：小米 `23049RAD8C` / `marble` / Android 15 · adb `192.168.0.212:39183`（无线调试；**端口已从 T09/T11 的 `:43057` 轮换**，由 `adb mdns services` 现测得到）· 包 `io.dcloud.uniappx` · 前台 Activity `io.dcloud.uniappx/io.dcloud.uniapp.UniAppActivity`
- **构建**：本 worktree（`D:\FL\wt-650\training-app\叉车维修培训学员端跨端应用`），分支 `refactor/forgot-password`
- **日期**：2026-09-22
- **执行人**：agent 执行（**①b 能力面未命中** ⇒ 无人工门）
- **登录态**：**本页无登录门控**，两轮都用 `cli launch … --pagePath pages/forgot-password/forgot-password` **深链直达**
  ⇒ 不需要 T11 register 那种「请维护者先在真机上登出」的人工作业。

## 术前 / 术后怎么取的

| 轮次 | 树 | 部署 / 切页 | 页身份（设备侧证据） |
| --- | --- | --- | --- |
| 术前 | `40cb8c12` 的四个运行时面文件临时回填（`git checkout 40cb8c12 -- <page/api/register-composable/login>`）+ `npm run hx:run` | `cli launch … --pagePath pages/forgot-password/forgot-password` | 日志原文 `14:33:52.660 进入页面:pages/forgot-password/forgot-password` |
| 术后 | 本分支构建（`npm run hx:run`） | 同上 | 日志原文 `14:08:57.327 进入页面:pages/forgot-password/forgot-password` |

> 「术前」为什么是临时回填：设备上只有一棵树，而 ①a 要的是**前后对比**；回填后 `npm run hx:run` 部署出的就是
> 术前源码（同一设备、同一密度、同一登录态），取完即 `git checkout HEAD --` 还原（`git status` 复验为空）。
> 只回填这四个文件的原因：`getCaptchaApi` 的 DTO 化（`api/auth.uts`）与三个消费方是同一 PR 的连带改动，
> 单独回填页面会编译不过 —— 这一点正是术前树必须整体回填的判据。

## 判据与结果（全部机器可复核）

| # | 判据 | 术前 | 术后 | 结论 |
| --- | --- | --- | --- | --- |
| ① | 页身份（launch 日志） | `进入页面:pages/forgot-password/forgot-password` | 同左 | 两轮都命中**目标页** |
| ② | a11y 文本集合 | 16 条 / `text_sha=daf28a6d7f7956f0` | 16 条 / `text_sha=daf28a6d7f7956f0` | **逐条一致**（`texts_only_in_a=[]`、`texts_only_in_b=[]`） |
| ③ | a11y 节点三元组（值 + class + x 边界） | 17 节点 / `node_sha=933b47a135948de1` | 17 节点 / `node_sha=933b47a135948de1` | **差异 0** ⇒ 内容与**几何**均零漂移 |
| ④ | a11y 原始 XML | `xml_sha=31599A3C51DBE86C…`（6480 B） | 同左（6480 B） | 6 次采样的 XML **逐字节相同** |
| ⑤ | 逐行像素（全帧） | — | — | `different=34039 ratio=0.013132 bbox=x 67→1007 y 28→1237`；行带 `y0-99:6949 / y1100-1199:17286 / y1200-1299:9804` |
| ⑥ | 逐行像素（忽略顶部 90 行状态栏） | — | — | `different=27090 ratio=0.010859 changed=true`（阈值 0.005）—— 差异**全部落在 `y1100–1299`** |
| ⑦ | logcat 崩溃断言 | — | — | `adb logcat -d` 253,901 行：`FATAL EXCEPTION=0`、`ANR in=0`、`ANR in io.dcloud.uniappx=0` |

### ⑤⑥ 的归因：那 27,090 像素**就是图形验证码**

图形验证码图**每次进页由后端随机生成**，故术前/术后必然不同 —— 这一点用两组对照机检（同一台设备、同一构建）：

| 对照 | 手法 | 机检结论 | 行带分布 |
| --- | --- | --- | --- |
| **A 稳定性** | 术前构建连拍两张（无任何交互） | `different=395 ratio=0.000152 changed=false` | —— |
| **B 归因（术前）** | 点图形验证码图 (869,1186) 触发 `refreshCaptcha` 后拍 | `different=27560 ratio=0.010633 bbox=x 741→998 y 28→1237` | `y0-99:470 / y1100-1199:17286 / y1200-1299:9804` |
| **B' 归因（术后）** | 同上 | `different=28436 ratio=0.010971 bbox=x 143→998 y 32→1237` | `y0-99:1346 / y1100-1199:17286 / y1200-1299:9804` |

**B / B' 的正文两条行带（`y1100-1199:17286` 与 `y1200-1299:9804`）与 ⑤ 的**逐数字相同**；对 a11y 树做交叉核对：
`图形验证码` 输入框节点的 bounds 是 `[121,1117][671,1255]`，验证码图就在它右侧同一行（本页 `.captcha-img` 180rpx×96rpx）
⇒ **`y1100–1299` 正是图形验证码行**。状态栏行带（`y0-99`）是时钟/电量读数，属系统噪声。

**⇒ 结论：正文区域（含全部文字、输入框、按钮、页脚与几何）像素零漂移；唯一的差异是每次进页随机生成的图形验证码图。**

## 与 #650 手术相关的两条实测量（写实）

- **图形验证码确实是 DTO 之后的值**：两轮日志都出现 `[request] >>> GET https://www.gccsmile.com/api/captcha` → `<<< 200`，
  且术后页面正常渲染出验证码图（对照 B' 能刷新出**不同**的图）⇒ `getCaptchaApi` 走 `getMapped` + `CaptchaResult`
  在真机上取到了字段（`data.id` / `data.image`），不是「编译过但运行时不显示」。
- **两轮渲染层报错同形**（`<view class="mode-tab">` 的 font-size/color/font-weight 被忽略）：术前/术后日志逐字相同，
  与该页 `<style>` 块**逐字节未动**互为印证 ⇒ 术前既有，按票面「发现的 UI 问题记 issue，不顺手改」另立 **#1269**；原文见 `machine-lines.txt` 第 3 节。

## 产物

| 文件 | 说明 | 大小 / sha256(前 16) |
| --- | --- | --- |
| `01-forgot-password-before.jpg` | 术前首屏（720w · JPEG q75） | 64,441 B / `ADAFD0987E03237A` |
| `02-forgot-password-after.jpg` | 术后首屏（同帧同参） | 63,933 B / `C1723B009AF58970` |
| `03-before-captcha-refresh-control.jpg` | 对照 B（术前点图刷新后） | 64,234 B / `3100751A1FBC394E` |
| `04-after-captcha-refresh-control.jpg` | 对照 B'（术后点图刷新后） | 64,179 B / `B35ED94A52F26661` |
| `machine-lines.txt` | 设备侧机检行**原样**（部署 / 页身份 / 渲染层报错 / logcat） | 3,105 B |

> 入库纪律照 ADR-0008：宽 ≤720（等比不放大）、单张 ≤150 KB、每 PR ≤10 张 —— 本目录 4 张、合计 ≈257 KB。
> ⚠️ **判据数字（⑤⑥/对照 A/B/B'）由原始 PNG（1080×2400，≈173 KB/张）算出**，原始 PNG 因**超 150 KB/张**未入库，
> 留在 `.ci-verify/t12-*.png`；入库的 JPEG 是**同帧**的 720w 压缩件，供人眼核对，**不是判据输入**。

## 诚实声明（agent 看不到截图内容）

本会话**无视觉通道**（模型不接受图片输入），因此不把「截图已入仓」当作「内容已核对」；上面每一条都是**机器判据**：
a11y 文本集合（内容）、a11y 节点三元组含 bounds（几何）、逐行像素 + 行带分布（渲染）、
对照 A/B/B'（把「图形验证码」这一个必然变化的元素从「漂移」里**归因分离**出来）、logcat（崩溃）。

## 复现命令

```powershell
# 0) 设备侧前提：解锁 + 常亮（10 分钟超时 + 人脸解锁会让长编译期间灭屏 ⇒ a11y 空、截屏全黑）
adb -s 192.168.0.212:39183 shell settings put system screen_off_timeout 1800000   # 本次临时改动，收尾已还原为 600000
adb -s 192.168.0.212:39183 shell settings put global stay_on_while_plugged_in 3

# 1) 术后：部署 + 深链 + 取证
npm run hx:run -- -Device 192.168.0.212:39183
& "D:\软件\HBuilderX.5.23.2026080626\HBuilderX\cli.exe" launch app-android --project <项目> --deviceId 192.168.0.212:39183 --pagePath pages/forgot-password/forgot-password
pwsh -NoProfile -File .scratch/phase-650.ps1 -Phase after                    # a11y dump + screencap（只读）
pwsh -NoProfile -File .scratch/phase-650.ps1 -Phase after2                   # 对照 A
pwsh -NoProfile -File .scratch/phase-650.ps1 -Phase after-refresh -TapX 869 -TapY 1186   # 对照 B'

# 2) 术前：临时回填四文件 → 重复第 1 步（Phase=before / before2 / before-refresh）→ 还原
git -C D:\FL\wt-650 checkout 40cb8c12 -- <page> <api/auth.uts> <useRegisterForm.uts> <login.uvue>
git -C D:\FL\wt-650 checkout HEAD --   <同上>                                 # 收尾还原，git status 复验为空

# 3) 对账
node .scratch/a11y-650.mjs   .ci-verify/t12-before-a11y.xml .ci-verify/t12-after-a11y.xml
node scripts/lib/png-diff.mjs --a .ci-verify/t12-before.png --b .ci-verify/t12-after.png --threshold 0.005 --ignore-top-rows 90
node .scratch/diff-bands-650.mjs .ci-verify/t12-before.png .ci-verify/t12-after.png   # 包围盒 + 行带分布
```

## 设备侧前提与本次的写操作声明（写实）

- **端口会轮换**：T09/T11 记录的 `:43057` 本次已失效（`cannot connect … 10061`），可靠发现方式是
  `adb mdns services`（本次得到 `192.168.0.212:39183`）；**不要照抄历史端口**。
- **本机曾不在同一网段**：取证前一度 `Test-Connection 192.168.0.212` = False（本机 WLAN 在 `172.17.13.11/23`，
  手机在 `192.168.0.x`）⇒ 设备不可达与「HBuilderX 忙」是两件事，先判网络。
- **本次做了三处写操作，逐条声明**（`scripts/device-capture.ps1` 的只读红线是「可能跑在维护者正在调试的真机上」，
  本次该设备**专供本票取证**，故按需放宽并留痕）：
  1. `settings put system screen_off_timeout 1800000`（临时，**收尾已还原为 600000**）；
  2. `settings put global stay_on_while_plugged_in 3`（T09 同款，未还原：手机无充电时该设置本就不生效）；
  3. `input tap 869 1186` 一次 —— **仅对照 B/B'**（点图形验证码图触发 `refreshCaptcha`），
     非「切页/点击导航」，且坐标取自 a11y dump 的实测 bounds（不是猜的）。
  其余全部只读：`exec-out screencap -p` / `exec-out uiautomator dump` / `shell dumpsys` / `shell logcat -d`。
