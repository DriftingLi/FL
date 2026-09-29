# 1394 · 快捷登录改凭据登录 · ①a/①b 真机取证

- 票：#1391（快捷登录改用凭据登录，解开 ADR-0004 与登出吊销的冲突）
- PR：#1394 · commit `9fa6cf03` · 分支 `feat/1391`
- 日期：2026-09-29 · 设备：Redmi 23049RAD8C（marble）/ Android 15 / 无线 ADB `192.168.0.212:42377`
- 运行时：HBuilderX 主程序 + 标准基座 `io.dcloud.uniappx`（`npm run hx:run` 增量部署，
  `HX_RUN_DEPLOY deployed=true`，资源 mtime 相对基线前进，pid 3611→13592）

## 判据（票面 + #1391 评论第 3 条）

1. 指纹验证通过后，**发出的是 `POST /auth/login`（body 为账号密码），不是 `/auth/refresh`**；
   成功后直接进 dashboard，全程零输入。
2. 登出 ⇒ 旧 `refresh_token` 被吊销（#1387 语义不回退）。
3. 包络不完整 ⇒ 不显示快捷登录入口（`hasStoredCredentials()` 同口径）。

## 操作序列（①b 人工段 = 登录与按指纹；①a agent 段 = 导航、截图、logcat）

1. 人在登录页手输账号密码 + 勾选「记住密码」→ 登录成功进 dashboard（`04-dashboard-first-login.png`）。
2. agent 经「我的 → 设置 → 账号与安全 → 退出当前账号」点「退出」（logcat：
   `POST /api/auth/logout` ⇒ 200，吊销发生，判据 2 不回退）→ 回登录页。
3. 登录页切到「账号密码登录」tab：**「🔐 指纹快捷登录」入口在位**（`02-login-quick-entry.png`）。
4. agent 点入口 → **人按指纹** → logcat（`01-logcat-request-lines.txt`，epoch 原样行）：
   - `1790673125.164 [useBiometric] 认证成功，使用模式: fingerPrint`
   - `1790673125.194 [request] >>> POST https://www.gccsmile.com/api/auth/login`（body 含 username/password）
   - `1790673125.322 [request] <<< 200 /api/auth/login`
   - `1790673125.886 进入页面: /pages/dashboard/dashboard`（零输入直达，判据 1 成立）
   - **全窗口 121 条 console 行中 `auth/refresh` 命中 0 条**（机检行：`auth/refresh lines: 0`）。

## 结论

- 判据 1 ✅（logcat 三条链 + 页面跳转行）；判据 2 ✅（登出带 body 的 POST /auth/logout 200；
  旧 rt 401 的吊销判据由 #1387 出证（`docs/verification/auth-session/1387/revoke-check-prod.md`）
  且本 PR 未触碰 `logout()` 面，机械锁 `utils/logoutRevokeBehavior.test.js` L1/L2 在位）；
  判据 3 ✅（入口显隐判定未改动，本轮登出后入口按包络在位、无包络 tab 页不显示）。
- 「被吊销的旧 rt 不再有任何消费方」：快捷登录路径实测不发 `/auth/refresh`（上表 0 命中）。

## 声明

- `01-logcat-request-lines.txt` 为 `adb logcat -d -v epoch` 中 `I console` 行的原样摘录，
  **仅对 `password` 字段值做了 `***MASKED***` 替换**（request.uts:332 会打印请求体，密码明文行不得入库）。
- 截图为 `screencap -p` 原图。`03-settings-after-quicklogin.png` 拍摄时用户已在会话内浏览设置页
  （快捷登录成功后 1 分钟），dashboard 在位证据以 `04` + logcat 跳转行为准。
