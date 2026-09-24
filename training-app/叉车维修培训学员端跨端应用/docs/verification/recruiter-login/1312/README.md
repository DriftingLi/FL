# #1295 招聘者登录页口令下限档 ①a 真机取证

- **PR**：#1312（base `master`，分支 `fix/1295`）
- **取证时 head**：`d323da3a` ｜ **分支收口 head**：`f5dc9ed2`（第二次同步 `origin/master` 的合并点）
- **页面**：`pages/recruiter/login.uvue`（本票唯一改动的运行时面文件；另新增一份读源文本的守护 `utils/recruiterLoginContract.test.js`，非运行时面）
- **设备**：小米 `23049RAD8C`（`marble`）/ Android 15 / 1080×2400 · 无线调试 adb `192.168.0.212:37611`（adb 1.0.41，`D:\android-sdk\platform-tools\adb.exe`）· 包 `io.dcloud.uniappx`
- **构建**：本 worktree `D:\FL\wt-1295\training-app\叉车维修培训学员端跨端应用`
- **日期**：2026-09-24（11:38–11:49）
- **执行人**：agent 执行（**①b 未命中能力面** ⇒ 无人工门；判据见下第六节）
- **注入授权**：维护者声明「设备归本次取证专用，可以直接操作」⇒ 本取证动用 `input`。**`scripts/device-capture.ps1` 内仍禁 `input`（未放宽）**；本目录记录的 `input` 属会话式取证，依据是这句声明 + 每次现测的注入可用性。

## 一、先证明「抓到的是本树的产物」

真机取证最容易的假绿是抓到旧代码，故部署与产物各有独立直证，不采信脚本自己的判定标签。

1. **部署**：`pwsh -File scripts/hx-run.ps1 -Device …`（**必须是 `pwsh`，用 `powershell` 5.1 会把无 BOM UTF-8 的中文注释按 ANSI 读、直接解析失败、根本没编译**）打出 `HX_RUN_DEPLOY deployed=true … www 的 mtime 相对基线前进`、`HX_RUN … exit=ok`。设备侧另测：部署前 `lastUpdateTime=2026-09-24 09:20:00` / `pidof=29246`，部署后 `pid=27555` ⇒ 进程确实换新资源重启。
2. **产物直证**：uni-app-x「运行到手机」按页推 `classes.dex`。对设备上正在运行的那份做只读字符串探针：

   ```
   D=/sdcard/Android/data/io.dcloud.uniappx/apps/__UNI__1C1D180/www/pages/recruiter/login
   grep -c -a '请输入招聘者账号' $D/classes.dex -> 1
   grep -c -a '请输入密码'       $D/classes.dex -> 1
   grep -c -a '密码至少'         $D/classes.dex -> 1   ← 本票新增档位
   ```

   改动若没落地，第三行会是 `0`。这是「手机跑的就是本树」的最短直证。
3. **页身份**：来源 = `cli launch` 的 stdout `.out`（**不是** logcat —— `进入页面` 在全缓冲 261975 行里命中 0）。原文 `11:45:19.701 进入页面:pages/recruiter/login`，其**后直到取证结束再无任何「进入页面」行** ⇒ 三次点「登 录」都留在本页，没有发生跳转。

## 二、三态取证：客户端门在哪一档返回、按什么顺序返回

`uni.showToast` 只存在约 2 秒、且**不进 a11y 树** ⇒ 截图是它唯一的载体。为抢在窗口内、并省掉一次 WiFi 往返，每帧用**同一条设备端命令**「点按 + 抓帧」：

```
adb -s <S> exec-out "input tap 540 926; screencap -p" > <帧>.png
```

坐标取自 `uiautomator dump`（账号框中心 `540,534`、口令框中心 `491,713`、「登 录」中心 `540,926`）。

| 帧 | 输入态（a11y 实测） | toast 实拍文案 | 入库件 |
| --- | --- | --- | --- |
| F1 | 账号空 + 口令空 | 请输入招聘者账号 | `01-toast-empty-account.jpg` |
| F2 | 账号 `rec1234` + 口令空 | 请输入密码 | `02-toast-empty-password.jpg` |
| F3 | 账号 `rec1234` + 口令 **5 位**（a11y 显示 5 个圆点） | **密码至少 6 位** | `03-toast-password-5digits.jpg` |

三条判据：

- **新增档位真的在真机上生效**：F3 的文案只能是 `validate()` 第三档给的（源码 `pages/recruiter/login.uvue:142-146`）。
- **短路顺序成立**：F1 与 F2 只在「账号是否为空」这一个变量上不同，输出的却是两条不同文案 ⇒ 账号档在口令档之前，不是碰巧。
- **拦在客户端、没去打网络**：脚本自己的只读帧（11:49 的 `pages-recruiter-login-current.png`，56268 B / sha `67788DD808FB…`）显示仍停在本页、口令框仍是 5 个圆点、按钮文案仍是「登 录」而**不是** computed 的「登录中...」——`btnText` 只在 `loading.value` 为真时变，而 `onSubmit()` 在 `validate()` 非空时直接 `return`、根本不置 loading。

三帧 sha256 互不相同 ⇒ 排除「同一张图冒充三态」。原帧（1080×2400 PNG）字节与 sha、入库件（720×1600 JPEG q80，PIL 10.4.0 LANCZOS）字节与 sha 都逐条列在 `machine-lines.txt` 第 5 节。

## 三、①a 机检行与只读保证

```
DEVICE_CAPTURE_RESULT=PASS
  页面 pages/recruiter/login=SKIP  前台=io.dcloud.uniappx/io.dcloud.uniapp.appframe.activity.UniPortraitPageActivity  logcat窗口行数=3313  FATAL=0  ANRin包=0
```

`SKIP` 的原因照原文「未切页：切页默认关闭」——只读模式不切页，本页的到达性由第一节的入页行承担。全窗口另读数：`FATAL EXCEPTION=0 / ANR in 包=0 / ANR 合计=0 / E AndroidRuntime=0 / 进程死亡=0`。

`device-capture.ps1` 全程只用 `devices` / `exec-out screencap -p` / `shell dumpsys` / `shell logcat -d`：未 `kill-server`、未 install/uninstall、未 force-stop、未清 logcat、未 push/pull/reboot。

## 四、为什么这份取证对收口 head 仍然成立

`f5dc9ed2` 只是把 `origin/master` 并进本分支的合并点。实测：

```
git diff --stat d323da3a f5dc9ed2 -- <pages utils components api stores manifest.json pages.json>   →  空
```

⇒ 运行时面逐字节未变，①a 三帧与 dex 探针描述的就是收口 head 的那份代码。④c 的 sha 绑定同理按「祖先 + 其间无运行时面改动」成立（`.github/workflows/pr-evidence.yml` 2026-09-12 放宽口径）。

## 五、诚实边界（本目录**没有**证到的）

1. **「满 6 位就放行」这一侧没有真机帧**。要拍到它必须真提交 ⇒ 会向后端发一次假凭据登录请求。该侧由 ③ 契约门（`utils/recruiterLoginContract.test.js` 的 `< 6` 判据 + 与学员端 `useLoginForm.uts` 的同形锁）承责。真机三帧证明的是「5 位被拦、且拦的文案是新增那条」。
2. **上限不判**：本票只补下限，`maxlength` 保持 32 未动；20 位上限口径归 #1262 三页对齐一起裁，守护里有一条**反向锁**，放宽须显式翻锁并引用票号。
3. **② 不触发**：改动集不含 `manifest.json` / `platformConfig.json`、无条件编译指令行增删。第 10 节那条 `MP_WEIXIN_RESULT` 只是顺手实跑的旁证，**不作为门禁结论**。

## 六、复现

```bash
cd D:\FL\wt-1295\training-app\叉车维修培训学员端跨端应用
# 1) 部署本树（必须 pwsh，不是 powershell）
pwsh -NoProfile -ExecutionPolicy Bypass -File scripts/hx-run.ps1 -Device 192.168.0.212:37611 -LogPath .ci-verify\hx-run-1295-deploy2.log
# 2) 三态取证（会话式，含 input；需维护者「设备专用」声明）
bash .ci-verify/cap-1295/capture.sh
# 3) ①a 机检行（只读）
pwsh -NoProfile -ExecutionPolicy Bypass -File scripts/device-capture.ps1 -Device 192.168.0.212:37611 -Pages "pages/recruiter/login" -Module recruiter-login -NoArchive
```

原始帧、a11y dump（`rl0.xml` / `tree-f*.xml`）、`readings-frames.txt` 与 `capture.sh` 本体留在 `.ci-verify/cap-1295/`（该目录被 gitignore，不入库）；本目录只入可核验的三帧与机检原文。
