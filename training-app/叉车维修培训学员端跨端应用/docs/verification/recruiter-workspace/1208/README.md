# #1195 招聘者工作区骨架与三面 —— ①a 真机只读取证记录（PR #1208）

> 归档时间：2026-09-20 16:20–17:2x ｜ 分支：`feat/mobile-1195-recruiter-workspace`
> 取证时 HEAD：`6702adff`（含 ①a 抓获的缺陷修复）
> 设备：`b32d8398`（Redmi `23049RAD8C`，1080×2400）｜ 工具链：HBuilderX 5.23.2026080626 + adb（`D:\android-sdk\platform-tools\adb.exe`）
> 口径：`training-app/叉车维修培训学员端跨端应用/docs/adr/0008-移动端验收门与证据.md`
> 「①a 取证手法补遗（2026-09-15 实测）」「页身份约束」「选页规则」+ `0016-真机门的人工性收缩与按批取证.md`
> **①a 由 agent 出证；本文件不含也不替代 ①b。** 本票机检 `CAPABILITY_SURFACE touched=False`
> ⇒ ①b **未命中能力面、无需人签**（判据见 PR 正文该行）。

## 一、场景与会话前提（很重要）

本票五个招聘者页面的 `onLoad` 都先过 `ensureRecruiterSession()`（`utils/recruitGuard.uts`）：

```
hasRecruiterSession() = getStorage(STORAGE_KEY_TOKEN).length > 0 && isRecruiterActive()
not → uni.reLaunch({ url: '/pages/recruiter/login' })
```

⇒ 按 ADR-0008「选页规则：目标页必须是**当前登录态下可达**的页」，**深链进工作区必须先有招聘者登录态**。
本轮的登陆态由**人在设备上手工登录一次**建立（ADR-0008 的夹具配方）：
agent **未**注入任何输入、**未**索取或代填任何凭证（`device-capture.ps1` 的只读红线 + 移动端 `AGENTS.md`）。

**会话态机检（只读）**——`run-as io.dcloud.uniappx cat databases/DCStorage` 现测：

```
auth_active_role       recruiter      (2026-09-20 17:01:06)
auth_login_provider    recruiter      (2026-09-20 17:01:06)
auth_token / auth_refresh_token       JWT payload 含 "role":"recruiter"
```

> 这条同时证明**会话在 App 私有 storage 里、可跨 HBuilderX 重启存活**（17:01 那次 `--pagePath` launch 之后仍在）。
> ⚠️ 现场曾出现「实拍停在学员首页」的**陈旧画面**：那是 HBuilderX 重启后停在了学员页，
> **不是**身份槽状态 —— 判断有没有招聘者会话要看 storage / 页面可达性，不看当前那一屏。

## 二、①a 逐页结果

页面集合取自本分支 `pages.json` 的**实际注册**：`login / jobs / resumes / contacts / applications / me`。

| # | 目标页 | 落定 | 页身份判据（设备侧原文） | 截图入仓 | 实拍内容 |
| --- | --- | --- | --- | --- | --- |
| 1 | `pages/recruiter/jobs`（一级面 · 职位段） | 282s | `进入页面:pages/recruiter/jobs` | `01-jobs.jpg` | 工作台 › 职位：真实职位「专业叉车维修员 / 招聘中 / 广州 · 无」 |
| 2 | `pages/recruiter/resumes`（一级面 · 简历库段） | 223s | `进入页面:pages/recruiter/resumes` | `02-resumes.jpg` | 工作台 › 简历库：本票只落骨架（正文属 P3）⇒ 空态；分段件在、徽标在 |
| 3 | `pages/recruiter/contacts`（一级面 · 交换段） | 215s | `进入页面:pages/recruiter/contacts` | `03-contacts.jpg` | 工作台 › 交换：3 条授权卡（「剩 7 天」/「有效期至 2026-09-16」）；**行内零操作入口** |
| 4 | `pages/recruiter/applications`（二级面 · 投递列表） | 219s | `进入页面:pages/recruiter/applications` | `04-applications.jpg` | 「投递」标题 + 返回；空态「该职位暂无投递」；**无分段件**（二级面判据） |
| 5 | `pages/recruiter/me`（二级面 · 我的） | 231s | `进入页面:pages/recruiter/me` | `05-me.jpg` | 招聘账号 HRtester / 当前身份 企业招聘者 / 记住密码·生物识别「不提供」/ 退出登录 |

**页身份证据**（`logcat -d` 原文，`进入页面:` 是 uniapp 全页面同 Activity 下唯一可得的设备侧页身份证据）：

```
17:01:06.392 进入页面:pages/recruiter/jobs 。[{"创建dom元素个数":"17个",…,"跳转页面到onReady总耗时":"245ms"}]
17:05:4x     进入页面:pages/recruiter/resumes 。[{"创建dom元素个数":"15个",…,"跳转页面到onReady总耗时":"135ms"}]
17:09:15.583 进入页面:pages/recruiter/contacts 。[{"创建dom元素个数":"17个",…,"跳转页面到onReady总耗时":"210ms"}]
17:12:56.046 进入页面:pages/recruiter/applications 。[{"创建dom元素个数":"10个",…,"跳转页面到onReady总耗时":"183ms"}]
17:16:54.535 进入页面:pages/recruiter/me 。[{"创建dom元素个数":"24个",…,"跳转页面到onReady总耗时":"232ms"}]
```

> 行首时间戳是**设备时钟**的 logcat 行时间；`resumes` 那条因缓冲轮转只保住了秒级前缀（`17:05:4x`），
> 其落定耗时 223s 与截图落盘时间（本机 17:05:48）一致。时间戳截断**照实记**，不做补写。

**后端请求证据**（同一轮 `logcat -d`，全部 `<<< 200`）：

```
17:08:20.937 >>> GET /api/recruit/jobs?page=1&page_size=20            ← 17:08:21.087 <<< 200
17:08:20.941 >>> GET /api/recruit/contact-requests?page=1&page_size=20 ← 17:08:21.092 <<< 200
17:09:15.385 >>> GET /api/recruit/contact-requests?page=1&page_size=20 ← 17:09:15.565 <<< 200
17:16:54.333 >>> GET /api/recruit/me                                  ← 17:16:54.445 <<< 200
```

> 17:08 那一对 `jobs + contact-requests` 恰好 2 个批量请求，与 ADR-0022 ②「首屏批量请求恰好 2 个」逐条对齐（真机侧佐证）。

### 机检行（`scripts/device-capture.ps1`，**只读**，`DEVICE_CAPTURE_RESULT=` 是唯一判据）

```
DEVICE_CAPTURE_RESULT=PASS
  前台=io.dcloud.uniappx/io.dcloud.uniapp.UniAppActivity
  窗口行数=292  FATAL EXCEPTION=0  ANR in 包=0  ANR 合计=0  E AndroidRuntime=0  进程死亡=0
  页面 pages/recruiter/me=SKIP（未切页：切页默认关闭，只读模式只对当前前台采 1 张）
  收尾完成：全程只读，未对设备做任何写操作
```

- 运行时间 2026-09-20 17:18:29；命令
  `npm run capture:device -- -Device b32d8398 -Pages pages/recruiter/me -Module recruiter-workspace -PostToPr 1208`。
- ⚠️ 机检行的 `页面 …=SKIP（未切页）` 是 `device-capture.ps1` 的**既有默认语义**（切页默认关闭、只读模式只采当前前台，
  ADR-0008「切页默认关闭」段），**不是**本轮逐页取证失败的标记 —— 逐页截图由上面的
  `auto-screenshot.ps1`（`--pagePath`）承担，两者分工见第四节。
- 该脚本按 `-PostToPr 1208` 贴了一条 `<!-- prefilter:device-capture -->` 评论（**刻意不用门证据前缀**，
  见 ADR-0008），它**不是**门证据、也不影响 `pr-evidence`；同期它因「工作树有未提交改动」跳过了自动入库，
  截图与日志留在本机 `.ci-verify/`（结论不受影响）。

## 三、①a 抓获并修复的运行时缺陷（本轮的额外产出）

### 现象
职位段**恒显示「暂无职位」**，而后端其实回了 `<<< 200` + 正常列表 —— 取数异常被页面 `catch` 后
当成空列表吞掉（**假空态**），用户看到的「没有职位」与事实相反。

### 根因（生成 Kotlin 层确证）
- 真机 `logcat`：`GET /api/recruit/jobs?page=1&page_size=20` → `<<< 200`，
  紧接着 `[recruiter-jobs] load jobs failed: java.lang.NullPointerException: null cannot be cast to
  non-null type kotlin.String`，栈顶 `IndexKt.getRecruitJobsApi$lambda$79(index.kt:10674)`。
- 修复前生成代码（`unpackage/cache/.app-android/src/index.kt:10674`，原文）：
  ```kotlin
  items.push(RecruitJob(..., offline_reason = (obj["offline_reason"] as String) ?: "", ...))
  ```
  `as String` 在 Kotlin 是**非空断言** ⇒ 缺键**先抛**，`?:` / `?? ''` **兜不住**（抛的是 Exception，不是 null）。
- 后端 `JobPostingDTO.OfflineReason` 带 `json:"offline_reason,omitempty"`
  （`backend/internal/service/job_posting_service.go:74`）⇒ **未强制下架时该键根本不出现**。
- 同族核查：`offline_reason` 是 `RecruitJob` 读到的**唯一「既 omitempty 又真会缺」**字段；
  `CompanyName` / `ContactRequestDTO.Source` / `ApplicationDTO.JobTitle` 同样带 `omitempty`
  但 `toDTO` 恒赋值 ⇒ 现状不会缺键（结论：不做无依据的连带改动）。

### 修复（提交 `6702adff`）
改为「先判空再强转」，写法对齐仓内既有先例 `api/auth.uts:20-21`：
```ts
const offlineReasonRaw = obj['offline_reason']
… offline_reason: offlineReasonRaw == null ? '' : offlineReasonRaw as string,
```
修复后生成代码（同文件 `:10675-10680`，原文）：
```kotlin
val offlineReasonRaw = obj["offline_reason"]
… offline_reason = if (offlineReasonRaw == null) { "" } else { offlineReasonRaw as String }
```
⇒ 缺失键走空串分支，**不再存在非空强转**。

### 真机复验（判据：logcat 无 `load jobs failed` + 渲染真列表）
```
17:01:06.168 [request] >>> GET https://www.gccsmile.com/api/recruit/jobs?page=1&page_size=20  (api/request.uts:262)
17:01:06.387 [request] <<< 200   https://www.gccsmile.com/api/recruit/jobs?page=1&page_size=20  (api/request.uts:341)
17:01:06.392 进入页面:pages/recruiter/jobs 。[{"创建dom元素个数":"17个","耗时":"26ms"},…]
```
- `load jobs failed` **零命中**；应用日志 `NullPointerException` **零命中**
  （唯一一条 NPE 来自 MIUI 系统组件 `HyperOSCustFeatureResolve`，与 App 无关）。
- 实拍 `01-jobs.jpg`：**工作台 › 职位**渲染出真实职位「专业叉车维修员 · 招聘中 · 广州 · 无」，
  不再是「暂无职位」。
- ⇒ 「后端 200」+「映射不再抛」+「页面渲染真列表」三条判据齐。

### 对照帧（**修复前**，非 ①a 证据）
`00-jobs-BEFORE-FIX-false-empty-state.jpg` —— 修复前 16:20 的职位段，恒「暂无职位」的假空态现场。
`00-resumes-BEFORE-FIX.jpg` —— 同一轮修复前的简历库段（空态）。
这两张是**缺陷对照**，用于验证「修复前确实不复现真列表」，**不得**当作 ①a 的页面证据引用。

## 四、只读保证与工具链

- 全程只用 `adb devices` / `exec-out screencap -p` / `shell dumpsys …` / `shell logcat -d`（`-d` 取完即退、**不清**缓冲）/
  `run-as … cat`（读 App 私有 storage 判会话态）；
  **未** `kill-server`、**未** `install` / `uninstall`、**未** `force-stop`、**未** `logcat -c`、
  **未** `push` / `pull`、**未**注入 `input`、**未**退出登录、**未**清 App 数据。
- 逐页导航用 ADR-0008 明文规定的机制：HBuilderX `cli launch app-android --pagePath <页>`，
  并走 `scripts/lib/auto-screenshot.ps1` 的 `Start-NavLaunchDetached`（分离派发，避免永久挂住）+
  `Wait-NavSettled`（页身份 `进入页面:<页>` + 非全黑 + 等满最短时长且画面稳定，**fail-closed 不产截图**）。
  ⚠️ 该机制**会重启 App**（HBuilderX 真运行语义），但**不改身份槽**（见第一节 storage 机检）；
  它**不是** `hx:run` 门脚本，仅作取证载体。
- 截图入库纪律：正文引用仓库内路径 `docs/verification/recruiter-workspace/1208/<页>.jpg`
  （压缩后 ≤720 宽 / 单张 ≤150 KB / 合计 ≤1.5 MB / 每 PR ≤10 张）。

## 五、未做项 / 谁来做

| 项 | 状态 | 谁 / 何时 |
| --- | --- | --- |
| ①a `jobs / resumes / contacts / applications / me` | 见第二节结果表 | agent（2026-09-20） |
| ①a `pages/recruiter/login`（招聘者登录页） | ❌ 未在本轮取（见下注） | agent；需一次**未登录态**批次，或由人退出招聘者会话后补 |
| **①b 能力面** | **未命中能力面 ⇒ 无需人签** | —（判据：`CAPABILITY_SURFACE touched=False categories=[]`） |
| ④c 整模块 Kotlin 编译 | 待按最终 HEAD 跑 | agent |
| ④a dev 全量编译 | 待按最终 HEAD 跑 | agent |
| ④b release 云打包 + 装机自测 | 未跑 | **人**，正式发版前（发布前置条款） |

> **`pages/recruiter/login` 未取的原因**：①a 的选页规则要求目标页在当前登录态下可达，
> 而持有招聘者会话时该页 `onLoad` 会按既有逻辑跳走（本票 P1 同类记录见
> `docs/verification/recruiter-login/1206/README.md`：深链进 `pages/login/login` 后 632 ms 自跳）。
> 该页的形态与身份互斥模态已在 P1（#1206）的 ①a 中实拍入仓，本轮**不重复冒充**。
