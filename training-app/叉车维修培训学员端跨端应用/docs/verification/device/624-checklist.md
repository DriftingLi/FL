# PR #624 生物识别快捷登录 · 真机验证人工步骤清单（①b）

> **状态：预置，本次未实测。** 本文件由 agent 于 2026-09-12 为 PR #624 备料；下面的 3 步**还没有被走过一遍**，
> 结论栏也**不得**由 agent 代填。口径见 `docs/adr/0008-移动端验收门与证据.md`「①a 真机自动取证」。
>
> - **①a**（agent 自动）= 逐页截图 + logcat 断言 + 前台 Activity —— 由 `scripts/device-capture.ps1` 采
> - **①b**（**人**，就是本清单）= 关键交互，主要是**按一次指纹**
>
> **机械上替代不了的原因**：#883 实测开发者工具的 console 报
> `checkIsSupportSoterAuthentication:fail … 请使用真机进行开发`；而指纹要人把手指按上去才产生一次成功/失败事件，
> adb 里**没有任何只读命令**能代替这次按压（`adb shell input` 类事件注入也不触发指纹传感器）。

## 你只需做的 3 步

| # | 你要做的 | 具体动作 | 期望看到 |
| --- | --- | --- | --- |
| 1 | 把 **#624 分支**跑到这台真机 | 用 HBuilderX 打开项目 → 运行到手机或模拟器 → Android；目标设备选已无线连接的这台（`192.168.1.26:46701`，model `2510DRK44C`）。分支 = `feat/biometric-quick-login`（head 已同步 master） | 应用装上并进到登录页 |
| 2 | 打开登录页、触发快捷登录入口 | 停在登录页；点 #624 新增的「快捷登录 / 指纹登录」入口 | 弹出系统指纹验证框（或直接进入识别流程） |
| 3 | **按一次指纹** | 按一次已录入的指纹 | **不弹密码、不报错，静默进 dashboard** |

看完把结果（成功/失败 + 现象 + 截图）填进 PR #624 正文 `## 验收证据` 的 ① 行；**「执行人」栏由你填，agent 不代填**。

## agent 会自动采的项（不用你动手）

一条命令即可（**只读**，不会打断你正在做的 #781 调试）：

```powershell
pwsh -NoProfile -File scripts/device-capture.ps1 -Device 192.168.1.26:46701 -Pages "pages/login/login"
```

| 自动采的项 | 说明 |
| --- | --- |
| 当前前台页截图 | `adb -s <dev> exec-out screencap -p`（只读）；压缩后入库（WebP q75，无编码器退 JPEG q75，宽 ≤720、单张 ≤150 KB、每 PR ≤10 张、合计 ≤1.5 MB） |
| logcat 断言 | 窗口内 `FATAL EXCEPTION` / `ANR in <前台包名>` 计数。窗口起点取自**设备时钟**（`logcat -d -v epoch -t 1`）；**不清** logcat 缓冲——清缓冲会抹掉你正在看的输出 |
| 前台 Activity | `dumpsys activity activities` 里 `mResumedActivity` / `topResumedActivity` 的**原始行** |
| 逐页截图 | **仅当**显式加 `-AllowAppStart` 才切页；**默认关闭**（切页会抢前台、打断调试会话） |

## 三条注意

1. **必须显式指定 `-Device`**：实测这台真机同时以 `192.168.1.26:46701` 与 `adb-f0bae674-KYrkiL._adb-tls-connect._tcp` **两条 transport** 出现（无线调试 + mDNS 自动发现）；脚本在「多设备」时 `exit 2`，**不自动猜**。
2. **脚本只用只读命令**：`adb devices` / `exec-out screencap -p` / `shell dumpsys …` / `shell logcat -d`。**不做** `kill-server`（会顶掉你共享的 adb server）、`install`、`force-stop`、`logcat -c`、`push`、`input`。只用 `D:\android-sdk\platform-tools\adb.exe`（与 HBuilderX 共用同一个 server）。
3. **脚本与 #624 的分支无关**：它是独立的只读取证工具，可在任意检出里跑（本次已放在 `E:\_g624`）。若要让截图**入库到 #624**，当前检出必须在 #624 的 head 分支上——脚本的入库有「当前分支 = PR head 且工作树干净」硬前提，不满足只打警告、不影响结论。

## 本次未实测的部分（照实说）

- `-AllowAppStart` **切页分支：未在真机跑过**（跑了就会抢前台、打断维护者的 #781 调试）。已用 stub adb 验证：它构造的 uniapp 深链意图正确，且「默认关闭」与 `-NoStartApp` 的 fail-safe 都成立、只读模式下 adb 调用面只有 5 个只读 verb。
- **截图归档入库**（压缩 → commit → push 到 PR 分支）与 `-PostToPr` 贴评论：**未在真机/真 PR 上跑过**。压缩管线已本地实测（179523 B PNG → 58178 B JPEG q75 @720px，≤150 KB、宽 ≤720）。
- **#624 本身的真机结论：没有。** 指纹那一步只能人按。