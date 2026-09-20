# #1196 招聘者简历库与简历详情 —— ①a 真机只读取证记录（PR #1207）

> 归档时间：2026-09-20 18:53–19:0x ｜ 分支：`feat/mobile-1196-recruiter-resume-library` ｜ 取证时 HEAD：`c2fff081`（其后仅加本文档所在证据提交）
> 设备：`b32d8398`（Redmi `23049RAD8C`，1080×2400）｜ 工具链：HBuilderX `D:\软件\HBuilderX.5.23.2026080626`（**编译器 5.24**，PID 28860）+ adb `D:\android-sdk\platform-tools\adb.exe`
> 口径：`training-app/叉车维修培训学员端跨端应用/docs/adr/0008-移动端验收门与证据.md`
> 「①a 取证手法补遗」「页身份约束」「选页规则」+ `0016-真机门的人工性收缩与按批取证.md`
>
> **①a 由 agent 出证；本文件不含也不替代 ①b。** 本票**命中能力面**（`api/request.uts` ∈ 真机上传白名单）
> ⇒ **①b 必做、且只能由人给原文**（agent 只可代录、不得自拟、不得写「已通过」）。

## 一、场景与会话前提

招聘者页面 `onLoad` 先过身份守卫（`utils/recruitGuard.uts` 的 `ensureRecruiterSession()` / 页面自持的
`guardRecruiter()`）⇒ 按 ADR-0008「选页规则：目标页必须是**当前登录态下可达**的页」，
必须有招聘者登录态。本轮登录态由**人在设备上手工登录一次**建立（ADR-0008 的夹具配方）；
agent **未**注入输入、**未**索取或代填任何凭证。

**会话态机检（只读 `run-as io.dcloud.uniappx cat databases/DCStorage`）**：
```
auth_active_role     recruiter   (2026-09-20 17:16:54)
auth_login_provider  recruiter   (2026-09-20 18:53:39)   ← P3 部署后仍在
auth_token / auth_refresh_token  JWT payload 含 "role":"recruiter"
```
> 这条同时证明 **`hx:run` 部署不会清掉 App 私有 storage 里的招聘者会话**（P3 部署后写入时间前移到 18:53）。

## 二、①a 逐页结果 + **复用来历表**（复用的页必须过核对 A/B）

页面集合取自本分支 `pages.json` 的实际注册（收口后）：`login / jobs / resumes / contacts / applications / me / resume-detail`。

**核对口径**（两棵树：P2 = `cf537fae`，P3 = `c2fff081`）
- **核对 A（页面本体）**：`git diff cf537fae c2fff081 -- <page.uvue>` 为空；
- **核对 B（该页实际调用的符号）**：把该页从 `api/recruit.uts` import 的每个函数/类型**抽块后逐字节**比对
  （脚本化：按 `export function NAME(` / `export type NAME` 起花括号配平取块 → SHA256 比）；
  其余被 import 的整文件（`utils/recruitDisplay.uts` / `utils/recruitGuard.uts` / `stores/auth.uts` / `utils/authRole.uts` /
  `utils/storage.uts` / `api/helpers.uts` / `constants/app.uts`）两树**整文件字节一致**。

| 页 | 帧文件 | 取自哪棵树/哪次运行 | 核对 A（页面本体） | 核对 B（该页调用面） | 结论 |
| --- | --- | --- | --- | --- | --- |
| `pages/recruiter/resumes` | `p3-02-resumes.jpg` | **P3 树 `c2fff081`，2026-09-20 18:56 真机** | **DIFF**（+370/−19：P3 正文并入） | — | ✅ **自行采集** |
| `pages/recruiter/resume-detail` | `p3-04-resume-detail.jpg` | **P3 树 `c2fff081`，2026-09-20 18:5x 真机** | **DIFF**（+701：P3 新增） | — | ✅ **自行采集** |
| `pages/recruiter/jobs` | `p3-01-jobs.jpg` | **P3 树 `c2fff081`，2026-09-20 19:0x 真机** | IDENTICAL | **DIFF**：`getRecruitContactRequestsApi` / `RecruitContactRequest` 两树不同 | ✅ **自行采集**（判不准就自采） |
| `pages/recruiter/contacts` | `p3-03-contacts.jpg` | **P3 树 `c2fff081`，2026-09-20 19:0x 真机** | IDENTICAL | **DIFF**：同上（该页只调这两个符号） | ✅ **自行采集**（判不准就自采） |
| `pages/recruiter/applications` | `reuse-05-applications-from-1208.jpg` | **P2 树 `cf537fae`（PR #1208 同树帧，2026-09-20 17:12 真机）** | IDENTICAL | **IDENTICAL**：`getRecruitJobApplicationsApi` / `rejectRecruitApplicationApi` / `RecruitApplication` / `RecruitApplicationListResult` 抽块 SHA256 全等 | ♻️ **复用** |
| `pages/recruiter/me` | `reuse-06-me-from-1208.jpg` | **P2 树 `cf537fae`（PR #1208 同树帧，2026-09-20 17:16 真机）** | IDENTICAL | **IDENTICAL**：`getRecruitMeApi` / `RecruitMe` 抽块 SHA256 全等 | ♻️ **复用** |
| `pages/recruiter/login` | — | — | IDENTICAL | 不 import `api/recruit` | ⏭️ **本轮未采**（见末节「未取到」） |

> **复用帧的限定**：`reuse-05-applications-from-1208.jpg` 与 `reuse-06-me-from-1208.jpg` 是
> **在 P2 树上产出的帧**；它们可被本 PR 引用，是因为上表的 **A + B 两条核对同时成立**
> （页面本体字节一致 **且** 该页实际调用的符号抽块字节一致）。**不得**把它们读成「本次在 P3 树上采集」。
> 复用命令（可复现）：
> ```
> git diff --numstat cf537fae c2fff081 -- training-app/叉车维修培训学员端跨端应用/<page.uvue>   # 核对 A
> # 核对 B：把两树 api/recruit.uts 抽出，按 export 块配平取块比 SHA256（见报告里的 symbol-diff 输出）
> ```

**页身份证据**（`logcat -d` 原文；`进入页面:` 是 uniapp 全页面同一 Activity 下唯一可得的设备侧页身份证据）
```
19:02:15.372  进入页面:pages/recruiter/jobs
19:05:09.505  进入页面:pages/recruiter/contacts
```
> ⚠️ **照实记**：`resumes` 与 `resume-detail` 的 `进入页面:` 行**已被 logcat 环形缓冲轮转掉**
> （两条是 18:56 采的，其后 jobs/contacts 两页各约 3 分钟的 launch 输出把它挤出了缓冲区）。
> 这两页的页身份判据**在当时那一轮**由 `Wait-NavSettled` 现场满足并据此产图（其判据要求
> 「设备侧日志出现**目标页**的页面进入行 + 非全黑 + 画面稳定」三条同时成立，否则 fail-closed 不产图），
> 且同一时间窗的 `<<< 200` 请求行仍在（见下）；但**当下**已无法从设备重新取到那两行 —— 不补写、不伪造。

**后端请求证据**（同一轮 `logcat -d`，全部 `<<< 200`）
```
18:56:22.518 >>> GET /api/recruit/resumes?page=1&page_size=20   ← 18:56:22.892 <<< 200
18:56:36.533 >>> GET /api/recruit/resumes?page=1&page_size=20   ← 18:56:36.592 <<< 200
18:56:39.884 >>> GET /api/recruit/resumes/1                     ← 18:56:39.962 <<< 200
18:56:40.025 >>> GET /api/recruit/resumes/1/contact             ← 18:56:40.199 <<< 200
```
> `/recruit/resumes/{id}/contact` 回 `200`（不是 403）⇒ 该简历**已授权**，详情页走的是「授权后 6 键明文」那条分管
> （与票面判据 2 对应；未授权态由 `utils/recruiterResumeBehavior.test.js` 的 B 组在运行期守）。

**机检行**（`scripts/device-capture.ps1`，**只读**）
```
DEVICE_CAPTURE_RESULT=PASS
  前台=io.dcloud.uniappx/io.dcloud.uniapp.UniAppActivity
  窗口行数=593  FATAL EXCEPTION=0  ANR in 包=0  ANR 合计=0  E AndroidRuntime=0  进程死亡=0
  页面 pages/recruiter/contacts=SKIP（未切页：切页默认关闭，只读模式只对当前前台采 1 张）
  收尾完成：全程只读，未对设备做任何写操作
```
- 命令：`npm run capture:device -- -Device b32d8398 -Pages pages/recruiter/contacts -Module recruiter-workspace -PostToPr 1207`。
- ⚠️ `页面 …=SKIP（未切页）` 是 `device-capture.ps1` 的**既有默认语义**（ADR-0008「切页默认关闭」段），
  **不是**逐页取证失败：逐页截图由 `auto-screenshot.ps1`（`--pagePath`）承担。

## 三、④a 编译门抓获并修复的**真缺陷**（4 处，全部「首见」）

**为什么是首见**：P3 分支**从未成功编译过** —— `D:\wt-1196\.ci-verify` 一直是空的；本次是第一次真正跑到
Kotlin 编译段。以下 4 处都由 ④a（`npm run build:compile`）在**编译期**抓获，全部是**编译阻断**。

| # | `文件:行` | 报错（原文译回） | 根因 | 修法 |
| --- | --- | --- | --- | --- |
| 1 | `pages/recruiter/components/recruiter-filter-drawer.uvue:84-86` | `[plugin:uni:app-uvue] Could not resolve "../../api/recruit"` | 相对深度写错一层：本文件在 `pages/recruiter/components/` 下，两层只到 `pages/`（`pages/api/` 不存在）。仓内同构位置先例全为三层 | 改 `'../../../api/recruit'`、`'../../../api/helpers'`（含 `import type` 一行）；加 R6 静态锁 |
| 2 | `api/request.uts:215` | `error18 找不到名称"statusCode"` | `const forbiddenAny = forbiddenErr as any; forbiddenAny.statusCode = 403` —— UTS→Kotlin **不允许给 `any` 动态加属性**；`api/recruit.uts:244` 的读取侧 `(err as any).statusCode` 同样编不过 | 403 改由 `api/request.uts` 的 **`FORBIDDEN_MESSAGE_PREFIX`（`'403:'`）消息前缀**承载，读端 `isContactForbidden` 用 `startsWith` 判（**同一常量单点**）；加 E2/E3/E4 锁 |
| 3 | `api/recruit.uts:627` | `error18 找不到名称"API_BASE_URL"` | `import { get, post, API_BASE_URL } from './request'`，但 `request.uts` **并不 export** `API_BASE_URL`（真源是 `config/env.uts:28`） | 改为 `import { API_BASE_URL } from '../config/env'`（先例 `api/aiAssistant.uts:13`）；加 E6 锁 |
| 4 | `api/recruit.uts:643` | `No parameter with name 'showMenu' found.` | `uni.openDocument({ …, showMenu: true })` —— 该参数在本仓基座/UTS 版本下不存在（仓内两处先例都没有它） | 删除该行（对齐 `api/material.uts:120`）；加 E5 锁 |

**同批修掉的第 5 处（import 修好后才暴露）**：
`pages/recruiter/components/recruiter-filter-drawer.uvue` 把**跨模块对象类型**直接挂进
`defineProps<{ filters : RecruitResumeFilters }>()` ⇒ 组件层逐个报
`error18 找不到名称"region"/"position_id"/"credential_id"/"salary_min"…`
（`import type` 与值 import 两种形态都试过，同样报）。
修法 = 在组件内**就地声明结构等价的类型** `RecruitResumeFiltersProp`；
代价是类型有了第二份 ⇒ 加 **R7 锁**把两处字段清单**逐字对账**（顺序也钉），防「api 加一维、组件漏跟」的静默少一维。

**④a 判据盲区（如实记录，本轮不改工具链）**：`scripts/compile-check.ps1:207-209` 的**真判据**是
`(?i)\berror\b|unresolved reference|cannot infer type|找不到名称|类型不匹配|编译失败`
（另有一条排除行 `(?i)0\s*error|errors?\s*[:=]\s*0|no errors?|error count\s*[:=]\s*0`）。
⚠️ **订正（2026-09-20 现测）**：本节初版把判据写成 `(^\s*e: )|(:\d+:\d+: *error:)`、并把盲区说成
「漏 `Could not resolve` 与 `⛔error:` **两种形态**」——**两处都不对**：
那串正则**不是本脚本的判据**，而是 **④c** 的 `scripts/kotlin-all-check.ps1:64`（`$ErrorLinePattern`）；
而 `⛔error:` / `找不到名称` / `编译失败` 这一类，**④a 的真判据本来就会命中**（不属盲区）。
**准确口径只有一条**：④a 漏掉的是「**行内不含上述任何 token 的整族编译诊断**」，
已实测复现的实例是 `[plugin:uni:app-uvue] Could not resolve "…"`。
**同一批日志行对两条 pattern 的现测对照**（本会话重跑，源文件在 `wt-1196/.ci-verify/`）：

| 日志行（原文） | ④a 判据 | ④c 判据 |
| --- | --- | --- |
| `p3-4a-precheck.log`：`[plugin:uni:app-uvue] Could not resolve "../../api/recruit"` | ❌ 不命中（无任何 token）⇒ **假绿成因** | ❌ 不命中 |
| `p3-4a-precheck.log`：`已停止运行...` | ❌ 不命中（非错误行） | ❌ 不命中 |
| `p3-4a-run2.log`：`[plugin:uni:app-uts] kotlin编译失败` | ✅ 命中（`编译失败`） | ❌ 不命中 |
| `p3-4a-run2.log`：`error: 找不到名称“region”。…` | ✅ 命中（`\berror\b` 与 `找不到名称` 两侧都命中） | ❌ 不命中 |

⇒ 手工 grep 保留 `plugin:uni:` / `error:` / `Could not resolve` / `kotlin编译失败` 对 ④a 是**零成本冗余**
（后三类本就命中），**不是**「④a 脚本漏了它们」；反过来，④c 的那条 pattern 对上面两种插件形态
**两行都不命中** ⇒ ④c 的 `errors=0` 同样必须配手工 grep 才有判别力。
（`⛔error:` 这种「前缀 + `error`」形态由 `\berror\b` 捕获；本机存活的日志里没有它的实例
—— 那一轮的日志已不在磁盘上，故此处以同类实测行替代，**不虚报读数**。）
**假绿实证**：那一轮 `build.log` 明明有 `Could not resolve` + 代码帧、尾部 `已停止运行...`，
却照样打印 `COMPILE_RESULT errors=0 clean=True`（`p3-4a-precheck.log` 尾部还写「✅ 编译门通过」）。
⇒ 本轮 ④a 的结论改为**三条件**（见第四节），已把这条反馈给维护者另行处理。
**P1 / P2 未被该盲区污染**（逐文件扫 `build.log` / `hx-run.log` / `launch-*.out`：`wt-1194` 命中 0、`wt-1195` 命中 0；
且两树都在真机上真的跑起来过）⇒ 那两份 `errors=0` 有独立佐证。

## 四、④a 三条件判据（本轮的实证数字）

| 条件 | 内容 | 实测 |
| --- | --- | --- |
| ① 脚本判据 | `COMPILE_RESULT errors=0` | `errors=0 clean=True` |
| ② 日志形态 | `build.log` / 编译日志里 `plugin:uni:` / `error:` / `Could not resolve` / `kotlin编译失败` 命中数 | **全 0**（`build.log` 与本次运行日志各统计一遍） |
| ③ 正向证据 | 日志出现编译成功行 | `18:49:11.524 项目 叉车维修培训学员端跨端应用 编译成功。` + `ready in 181536ms.` |

## 五、只读保证与工具链

- 全程只用 `adb devices` / `exec-out screencap -p` / `shell dumpsys …` / `shell logcat -d` /
  `run-as … cat`；**未** `kill-server` / `install` / `uninstall` / `force-stop` / `logcat -c` /
  `push` / `pull` / 注入 `input` / 退出登录 / 清 App 数据。
- 逐页导航：`cli launch app-android --pagePath <页>`（`resume-detail` 另带 `--pageQuery id=1`），
  走 `scripts/lib/auto-screenshot.ps1` 的 `Start-NavLaunchDetached` + `Wait-NavSettled`
  （页身份 `进入页面:<页>` + 非全黑 + 等满最短时长且画面稳定；**fail-closed 不产截图**）。
- 截图纪律：≤720 宽 / 单张 ≤150 KB / 合计 ≤1.5 MB / 每 PR ≤10 张 —— 本目录 6 张 / 约 265 KB。

## 六、未做项 / 谁来做

| 项 | 状态 | 谁 / 何时 |
| --- | --- | --- |
| ①a `resumes` / `resume-detail` / `jobs` / `contacts` | ✅ 本轮 P3 树自采（本目录） | agent（2026-09-20） |
| ①a `applications` / `me` | ♻️ 复用 P2 同树帧（A+B 核对通过，见第二节） | agent |
| ①a `pages/recruiter/login` | ⏭️ 未采：①a 选页规则要求当前登录态可达，而持有招聘者会话时该页会自跳（P1 同类记录见 `docs/verification/recruiter-login/1206/README.md`）；该页形态已在 P1 的 ①a 实拍入仓 | agent（一次未登录态批次补） |
| **①b 能力面**（`api/request.uts` = 真机上传白名单） | ⛔ **待维护者签**（原文由人给出；agent 只可代录、不得自拟、不得写「已通过」） | **人** |
| ④c 整模块 Kotlin 编译 | 建议按最终 HEAD 跑（P3 此前从未编过） | agent |
| ④a dev 全量编译 | ✅ 本轮三条件已过（第四节） | agent |
| ④b release 云打包 + 装机自测 | 未跑 | **人**，正式发版前（发布前置条款） |
