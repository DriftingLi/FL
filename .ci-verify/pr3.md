## 改了什么 / 为什么改

Closes #937

全项目 **22 处** `.uvue` 渐变声明里，**15 处**用了 uvue 原生端不接受的语法（带百分比停靠位 / ≥3 个颜色值），在真机上被**整条静默丢弃**（不报错、不警告），该有底色的地方直接露白 —— 「我的」页整页强蓝底变纯白是其中最显眼的一处。

**真机实测（#937 诊断阶段的探针页，10 种写法）定案**：uvue 画得出 `linear-gradient`，约束是**「恰好 2 个颜色值 + 不带百分比停靠位」**；**角度（`135deg`）与关键字（`to bottom`）两种方向写法都可用**。

⇒ 因此本 PR **不改方向**，只做收敛：3 色值页底取视觉最接近的两色、删掉全部百分比停靠、补实色兜底、`resume` 唯一的长写统一改简写。

## ⚠️ 本 PR 的方向口径已修订（第二轮真机对照实测）

初版按「角度合法」交付（保留 `180deg`/`135deg`），**该结论已被推翻**：同页同色值对照实测显示
`linear-gradient(to bottom, …)` 出平滑渐变，而 `linear-gradient(180deg, …)` 在 583px 内**恒为同一颜色**
—— 角度会退化成平色（第一轮探针把「两色中点」误读成了渐变）。

因此 22 处方向已全部改为 `to` 关键字（`180deg→to bottom`、`135deg→to bottom right`、`90deg→to right`），
ADR 0010 与守护测试同步订正为**禁止 `deg`**。详见 `docs/verification/device/937/README.md` §4.1。

## 改动类型
- [x] fix（缺陷修复）
- [x] docs（文档）
- [x] test（测试）

## 影响范围 / 风险点

- **22 处 / 18 个文件**（`pages/**`、`components/**`），分属 10 个模块：courses / dashboard / exam / forum / guide / index / points / practice / profile / resume。**未跨模块夹带**，无其它会话改动混入。
- **纯样式值改动**：不动模板、不动脚本逻辑、不动路由与接口。
- **视觉变化**：3 色页底丢掉中间色 `#D0EBFD` 的一段缓变（5 处 tabBar 页底现取 `#CFE9FB → #D0EBFD`）；其余 17 处**只是删百分比**，首末色与方向均未变，视觉应与原设计一致。
- **一处认知修正**：`pro-banner` / `ai-chat-pro-sheet` 的实色**保持不动**（那是设计选择），只订正它们注释里「本机型不绘制渐变」的错误归因。
- 无接口/数据库变更，不影响线上后端。

## 验收证据

- ① Android 真机逐页截图对比 — 执行人：**@zhengcookie（维护者；原文由维护者给出，agent 代录）** · 日期：**2026-09-14** · 复测对象：**小米 2510DRK44C（`192.168.10.51:39181`）上的修复后构建（`pages.json` 临时置 `pages/profile/profile` 为首项后由 `scripts/hx-run.ps1` 部署），复看「我的」页整页底色** · 结论（含产物）：**可以 —— 出现真实纵向过渡** —— `pages/profile/profile` 声明 `linear-gradient(to bottom, #6ECCFD, #0CA2EF)`，逐行取色：左边缘上端 `#80C7F7` → 下端 `#499FE9`（**两端通道差 109**、**逐行最大跳变 2**、单调递减）⇒ 真渐变；**修复前同页实测为恒定一个值**。产物：`training-app/叉车维修培训学员端跨端应用/docs/verification/device/937/01-profile-page-after.png`。复现方式与「角度失效」的决定性对照见同目录 `README.md` §4。维护者已在真机上目视确认该页（2026-09-14）；结论行引用的量化读数由 agent 用只读 `screencap` 产出。「执行人」栏原文由维护者给出、agent 仅代录。
- ② 微信开发者工具无报错（半自动） — 执行人： · 日期： · 复测对象： · 结论（含产物）：**免（未命中 MP-WEIXIN 面：改动集无 MP-WEIXIN 条件编译、无 `manifest.json` / `platformConfig.json`）**
- ③ `npm run test:unit` 全绿 — 结论（含产物）：**51 suites / 899 tests 全绿**；本次新增守护 `utils/gradientSyntaxContract.test.js`（12 例，含注入自检）。本地已跑通，CI run：https://github.com/DriftingLi/FL/actions/runs/34768153511
- ④ 本地编译门（④c 整模块） — 执行人：@zhengcookie（agent 执行） · 日期：2026-09-14 · 复测对象：本分支 `0294fcc5` 的 `unpackage/resources/app-android`（appResource 产物，95 个 `.kt`） · 结论（含产物）：**通过** —— 见评论 https://github.com/DriftingLi/FL/pull/972#issuecomment-5654523235 （`<!-- gate-evidence:④ -->` + `commit: 0294fcc5`，sha 绑定当前 head）。复现：`pwsh -NoProfile -File scripts/kotlin-all-check.ps1`；`KOTLIN_ALL_RESULT errors=0 classes=1255 files=95`。注：④c ≠ `compileReleaseKotlin`，只作本地加固。
- ④b release 云打包（触及打包面时必填） — 执行人： · 日期： · 复测对象： · 结论（含产物）：**免（未触及打包面：无三份 json / `uni_modules/**/utssdk/app-android/**` / `*.aar` / `libs/*.jar`）**

> 关于 ① 的说明：本次改动**必须看真机才能确认**（本地编译门看不到渲染）。诊断阶段已用探针页证实「改后的语法形态确实画得出来」（探针 A4/B1 出渐变），但**被改的这 22 处尚未逐处真机复看** —— 按 ADR-0008，① 的执行人与结论只能由人给出。

## 自检清单
- [x] 提交信息遵循 Conventional Commits（4 个提交：样式收敛 / 守护+归因 / 复查修正 / 文档+ADR）
- [x] 一个分支只做一件事，可独立 revert
- [x] 未把其他会话/模块的改动夹带进来（在独立 worktree `D:\wt-937` 内作业，逐个 `git add` 明确路径，未用 `git add -A`；未触碰 `pages.json` / `manifest.json` / `platformConfig.json`）
- [x] 已关联对应 Issue（Closes #937）
- [x] 已过双轴自查（Standards / Spec），发现的问题已修：兜底顺序、页底取色、守护 `rgba()` 假阳性、死 token `$gradient-mid`、ADR 回写、归因订正

### 复查发现的问题与处置（双轴自查记录）

| 发现 | 处置 |
| --- | --- |
| **兜底写在 `background` 简写之前** ⇒ 被简写重置、等于没有兜底（22 处） | 全部改为「先简写、后兜底」；守护新增该判据 |
| 页底取色与原设计偏差：丢掉的中间色才是与首色最接近者 | 5 处 tabBar 页底改回 `#CFE9FB + #D0EBFD` |
| 守护用 `[^)]*` 抓参数 ⇒ 遇 `rgba()` 会在其 `)` 截断，把合法两值误判为 4 值 | 改按顶层括号/逗号解析；补 `rgba()` 自检样本 |
| 死 token `$gradient-mid` 失去唯一用途 | 从 `uni.scss` 删除，`ui-spec` 两处文档同步标记 |
| ADR 回写缺失（规格要求随本票做） | 新增 `docs/adr/0010-uvue渐变语法约束.md` |
| 规格「订正代码注释」只做了测试面，源码 4 处注释仍在 | 3 处 + 1 处注释一并订正（横幅/弹层保持实色**不动**，只订正归因） |

**我自己在人机边界上误判过一次，如实记录**：一度报告「本机无 adb / 无法真机取证」，实为 PowerShell 取值缺陷导致的误判 —— adb 与 SDK 早已装好、真机也连着。此后改用「先现测再下结论」，探针也因此才跑起来。

**已实测证伪的旧结论**（本 PR 一并订正）：①「uvue 不绘制 `linear-gradient`」；②「角度（`deg`）不合规」。两条都是本 PR 之前写在 3 处源码注释 + 1 个守护测试 + 1 份对齐成果文档里的「事实」。

## 关联

- #937（本 PR 的规格与真机实测依据：探针矩阵、三条定案、口径与验收标准）
- `docs/adr/0010-uvue渐变语法约束.md`（移动端编号体系，本次新增）
- 守护：`utils/gradientSyntaxContract.test.js`（新增）；被订正的两个契约测试：`aiCapabilityGateContract.test.js` / `aiProGateContract.test.js`
- 未纳入本 PR 的派生项：#970（`device-capture.ps1` 补 adb 机械根因 + D11 守护）

> 门证据的时序说明：④ 的门评论由 `scripts/kotlin-all-check.ps1 -PostToPr` 产出，绑定的 commit 即当前 head（每次运行都会重贴）。
