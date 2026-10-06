# #1484 ①a 真机逐页取证 · 登录页 App 侧提供方入口件

- 票：https://github.com/DriftingLi/FL/issues/1484 · PR：https://github.com/DriftingLi/FL/pull/1558
- 取证时刻：2026-10-06 00:08（轮次 B 即正式取证那一次）
- 复测对象：`feat/1484` @ `53c0393d`（本树 `D:\FL\wt-1484\training-app\叉车维修培训学员端跨端应用`）
- 设备：`b32d8398`（Xiaomi 23049RAD8C / Android，1080×2400，USB）
- 驱动：`Invoke-AutoScreenshot -Pages 'pages/login/login' -NavigateMinSeconds 20 -NavigateTimeoutSeconds 900`
  （点源 `scripts/lib/hx-busy.ps1` 取锁 → `Wait-HxFree` → 取证 → `Release-HxLock`；忙则 exit 2，不抢占）
- 结果行：`Ok=true Skipped=[] HashConflicts=[] StaleShots=[]`

## 产物清单

| 文件 | 是什么 | sha256 |
| --- | --- | --- |
| `login.png` | ①a 脚本产出的登录页整屏（禁用态入口 + 明示文案） | `c6984e908302050f7cd290a46eb07bcdea0508b2ec82f6a4a5bb57306dd395b9` |
| `login-tap-toast.png` | 点一次入口后 0.55 s 的整屏：Toast「微信登录暂未开通」在屏、页面态未变 | `51e95ff586aa257723bcd8389e572f7220d906093ad1c631807b109cf0cb629e` |
| `a11y-login-entry.xml` | `uiautomator dump`：入口件两条文字节点在位（含 bounds） | `ade8969cdf642e4a1ad4397c498104c148ce801440128b9a5f3278e7c83a406f` |
| `a11y-after-5-taps.xml` | 连点 5 次后再 dump —— **与上一份逐字节同 sha** ⇒ 页面层级零变化 | 同上 |
| `launch-log-excerpt.txt` | 两轮 launch 日志摘录（含 DIAG 机检行、缓存/同步判定、ADB 冲突警告） | — |

## 判据逐条读数

| 票面判据 | 设备侧读数 | 结论 |
| --- | --- | --- |
| App 端入口常驻可见 | `content-desc="微信登录"` bounds `[431,1860][649,1948]`；整屏 `login.png` 可见 | ✅ |
| 未接通时为禁用态 | 该节点渲染为灰字（`.lpe-entry-off` → `#C8CDD8`），与同页蓝色可用链（`账号密码登录` `#3B82F6` 系）明显不同档 | ✅ |
| 明示「没开通」而不是骗人 | `content-desc="微信登录暂未开通"` bounds `[404,1954][676,2000]` 常驻；点击后 Toast 同文案（`login-tap-toast.png`） | ✅ |
| 禁用态点击 ⇒ 请求层 0 次 | 连点 5 次：a11y 层级 sha 前后同值（模式未切、`switchMode` 未走）；验证码裁剪区（x680 y1040 w340 h190）sha256 前后同为 `b061fc644a16d57da20664eaac60d06b6f1475a1f17f3a2da80293f7fb94bcd7` ⇒ `loadCaptcha()` 未被触发 | ✅（结构 + 像素双判据） |
| 日志能看到探测结论 | `[login-providers] DIAG platform= app available= [boolean] true ready= [boolean] false isApp= [boolean] true at composables/useLoginProviders.uts:121` | ✅ |
| 小程序端不受影响 | 本门不测（② 门免：`#ifdef` 指令行增删数 = 0）；结构证据在 `utils/loginContract.test.js`（`STYLE_SHA256` 逐字节不变） | 由 ③ 门兜 |

## 「截图里没有入口」那轮的归因（下一票可直接抄）

首轮 `Ok=true` 的截图里**没有**入口，但不是组件坏了：HBuilderX 里同时导入了同名项目
（`project list` 回显两条一模一样的 `叉车维修培训学员端跨端应用(UniApp_VUE)` 且**不回显路径**），
`cli launch --project <绝对路径>` 于是按名字解析，编译落在**主树**。三条文件级读数：

1. 22:30–22:45 那段编译只写主树 `unpackage/`，本树 `unpackage/` 零写入；
2. 设备侧 `www/pages/login/login/classes.dex` = 46,888 B / 14:39（= 主树同名文件尺寸），本树为 47,608 B / 18:09；
3. 关掉其余同名注册项后，同一条命令立刻 `检测到编译缓存部分失效，开始差量编译`，同步后
   `进入页面` 的 **dom 元素数 39 → 42**（多的正是入口件的 `view` + 两个 `text`），DIAG 行随之出现。

⇒ **①a 的 `Ok=true` 只证明「截到了目标页」，不证明「截的是你这棵树」**。判据用两条：运行期 DIAG 行 +
设备侧 dex 尺寸/mtime 与本树 `unpackage/dist/dev/app-android/` 同名产物比对。
`kotlin-all-check.ps1:251` 已为 publish 侧记过这条坑，launch 侧此前没有对应判据。

## 未覆盖 / 诚实声明

- Toast 文案不进 `uiautomator` 层级（它是独立窗口），所以「点击有提示」这条**只有截图证据**，没有 a11y 证据。
- 设备侧 `console.log` 不落 logcat（`logcat --pid` 62 行全是系统噪声），零请求这条的设备侧判据取的是
  「页面层级 + 验证码像素不变」，不是网络层读数；网络层 0 次由 `utils/loginProviderEntry.test.js` A1/A2 真跑断言。
- 本机存在两套 adb（`D:\android-sdk\platform-tools` 37.0.1 与 HBuilderX 内置 35.0.1），launch 日志里
  HBuilderX 自己警告「ADB 冲突 ⇒ 真机运行不刷新」。取证期间的设备侧读操作统一走内置那套。
- ①b（人工能力面门）不触发：本票无指纹 / 权限弹窗 / 真机上传 / 厂商 ROM 交互。
