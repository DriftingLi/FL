# PR #624 真机取证截图（2026-09-12）

> 工具：`scripts/device-capture.ps1`（①a 预置件，**非门**，不替代 ①b 人工关键交互——指纹由人按）。
> 全程只读：只用 `adb devices` / `exec-out screencap -p` / `shell dumpsys` / `shell logcat -d` / UI dump 到 /dev/tty；未清 logcat、未 kill、未 install。

- 设备：serial `192.168.1.26:41253`（model `2510DRK44C`，serialno `f0bae674`）
- 被测构建：本文件所在提交（HBuilderX CLI 增量运行到手机，基座 `io.dcloud.uniappx` 5.24.15006，`--pagePath pages/login/login`）
- 原始 PNG 留在 `.ci-verify/`（不入库）；此处为压缩副本（宽 720，JPEG q75，均 ≤150 KB）

| 文件 | 尺寸 | 字节 | 采集时刻 | 实际页面 | 说明 |
| --- | --- | --- | --- | --- | --- |
| `dashboard-valid-session-20260912-1644.jpg` | 720x1563 | 103071 | 16:44:20 | `/pages/dashboard/dashboard` | 会话仍有效时的 dashboard |
| `after-quicklogin-20260912-1645.jpg` | 720x1563 | 103341 | 16:45:26 | `/pages/dashboard/dashboard` | 维护者登出 → 登录页 → 点生物识别快捷登录 → 按指纹 之后的 dashboard |
| `login-after-logout-20260912-1651.jpg` | 720x1563 | 69870 | 16:50:52 | `/pages/login/login` | 再次登出后的登录页；页面上**未出现**生物识别快捷登录入口（见下） |
| `profile-1632-NOT-loginpage.jpg` | 720x1563 | 65740 | 16:32:34 | `/pages/profile/profile` | 采集时文件名按 `pages/login/login` 生成，实际是「我的」页（App 未停在登录页）；如实入库存证 |

页面身份判定依据：App 自身 console 日志 `进入页面:<path>`（截图时刻落在两次跳转之间）＋ 两张 dashboard 期截图的本地像素比对（整体差异 0.36%）。截图内容未用视觉模型核验。

## 该次快捷登录取证的关键 logcat 行（逐字，设备时钟）

```
16:44:05.766 POST /api/auth/logout  -> 200       （登出）
16:44:05.845 [useBiometric] 设备支持的生物认证模式: ["fingerPrint","facial"]   at composables/useBiometric.uts:59
16:44:05.845 [useBiometric] 含面容: true       at composables/useBiometric.uts:60
16:44:05.845 [useBiometric] 含指纹: true       at composables/useBiometric.uts:61
16:44:05.911 进入页面: /pages/login/login
16:44:15.881 [useBiometric] 认证成功，使用模式: fingerPrint   at composables/useBiometric.uts:131
16:44:15.894 POST /api/auth/refresh -> 200     （payload 来源 api/auth.uts:256）
16:44:15.964 GET  /api/auth/me      -> 200     （quickLogin 内 getUserInfoApi，仅 user==null 即登出后才走）
16:44:16.216 进入页面: /pages/dashboard/dashboard
```

同刻系统侧：`FingerprintAuthenticationClient ... owner=io.dcloud.uniappx`、`CMD_VENDOR_AUTHENTICATED`、`BiometricScheduler: [Finishing] ... success: true`。

排除项：该 logcat 缓冲（155635 行）内 `api/auth/login` 出现 **0** 次；captcha 仅 2 次页面加载 GET、无提交。
计数：FATAL EXCEPTION 0；`ANR in io.dcloud.uniappx` 0；E AndroidRuntime 0。
证明方式：**调用链（onQuickLogin → quickLogin）＋ 排除法**；源码在成功路径无正向日志（`[auth] quickLogin` 只在失败时 warn），故无正向成功日志可直接引用。

## 发现：再次登出后的登录页上没有快捷登录入口

16:50:48 用只读 UI dump（`adb exec-out uiautomator dump --compressed /dev/tty`，不落盘到设备）核对登录页：`快捷登录` / `指纹` / `面容` / `生物识别` 命中数均为 0；可见文本为：叉车维修培训系统、+86、请输入手机号、请输入验证码、获取验证码、图形验证码、学习者、招聘者、《用户协议》、《用户隐私》、立即注册、登 录、手机验证码、邮箱验证码、账号密码登录。

代码前提（`pages/login/login.uvue:30`）：入口 `v-if="showedStored && biometric.isSupported.value"`；`showedStored` 仅当 `hasStoredCredentials()` 为真才置 true（`login.uvue:276`），而 `hasStoredCredentials()` 要求凭据包络 `has==true` 且账号、密码字段均非空（`utils/secureStorage.uts:129`）。本次 dump 里手机号输入框为空（占位符可见），说明连降级包络（`loadSavedAccount()`）也读不到 ⇒ 当时**整个凭据载体读不到**。为什么读不到，logcat 无相关日志可定论（`secureStorage.uts` 不打印），需另一次复现或维护者口述。

（原始 UI dump 与原始 PNG 均未入库，留在本次取证的暂存目录。）
