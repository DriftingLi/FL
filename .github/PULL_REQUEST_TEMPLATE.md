<!--
合并方式固定 Squash and merge；目标分支固定 master（本模板此前写在移动端目录下且写着 main，GitHub 读不到、分支名也是错的）。
master 有 ruleset「protect master」，必检只有 ci-summary；pr-evidence 在本仓是**可见检查**（本仓有可用 admin 通道，但裁定不装必检——逐 PR 审批成本高于约束收益）。
真正的阻塞是行为约束：**agent 不得自行合并触及运行时面的 PR** —— 必须把证据填齐后停在「待人工签收」，由人执行合并。口径见 `training-app/叉车维修培训学员端跨端应用/docs/adr/0008-移动端验收门与证据.md`（**必须写全路径**：该 ADR 属**移动端编号体系**，根仓库另有一个同名的 `docs/adr/ADR-0008-字典描述符驱动的管理面.md`，只写 `docs/adr/0008-…` 在根目录解析不到）。
-->

## 改了什么 / 为什么改
<!-- 一句话说清意图，并关联 issue：Closes #编号 -->

## 改动类型
- [ ] feat（新功能）
- [ ] fix（缺陷修复）
- [ ] refactor（重构）
- [ ] style（样式/格式）
- [ ] docs（文档）
- [ ] test（测试）
- [ ] chore/build/ci（构建或依赖）

## 影响范围 / 风险点
<!-- 涉及哪些模块、是否影响线上、有无数据库/接口变更；跨模块夹带必须显式写明 -->

## 验收证据

<!--
口径见 ADR-0008。分级判据：改动集命中 *.uvue / *.uts，或 training-app 下的
manifest.json / pages.json / platformConfig.json 时，本段必须逐门填写；未命中的 PR
只需保留本段并写「免（未命中运行时面）」。

字段格式固定：执行人（真的做这门验证的人；agent 不得代填） · 日期（YYYY-MM-DD） · 复测对象 · 结论（含产物）
「待人工」「⏳」或只有勾选 = 缺证据，pr-evidence 会直接红；结论必须引用可核验产物：
截图及其链接、CI run 链接、.ci-verify/kotlin-all.log（④c）/ .ci-verify/build.log（④a）（只写「已验证」不算）。
④ 的证据可由脚本贴的「sha 绑定评论」承载：本仓 `npm run build:kotlin-all` 不转发参数，
要贴评论请直接跑 `pwsh -NoProfile -File scripts/kotlin-all-check.ps1 -PostToPr <PR号>`
（dev 面同理 `scripts/compile-check.ps1 -PostToPr <PR号>`，仅门通过时贴）；正文该行写「见评论 <链接>」即可。
② 自 2026-09-12 起是**半自动门**（#883 实测全链路无人值守，≈4.5 分钟/次）：跑
`pwsh -NoProfile -File scripts/mp-weixin-check.ps1 -PostToPr <PR号>`（或 `npm run build:mp-weixin-check`），
门通过时会贴 `<!-- gate-evidence:② -->` + `commit: <head sha>` 的评论，正文该行同样写「见评论 <链接>」。
**但「执行人」栏仍须由人签收**：agent 只产出证据（`MP_WEIXIN_RESULT` + 截图），不得代填执行人、不得写「已通过」。
校验器只认「带 gate-evidence:④ / gate-evidence:② 注释标记 + commit 与 head 绑定」的评论，不校真伪：
绑定 = `commit` **等于** head sha，**或**它是 head 的**祖先**且二者之间没有运行时面（`*.uvue` / `*.uts` / 三份 json）改动
（2026-09-12 放宽，免去 master 前进一次就重跑重贴的 churn）；其间运行时面动过、或不是祖先，仍须重跑并重贴。
带 -PostToPr 时 ② 还会把截图**压缩入库**到本 PR 分支的 docs/verification/<模块>/<PR号>/<页名>-after.<ext>
（WebP q75，本机无编码器则 JPEG q75；宽 ≤720；每 PR ≤10 张 / 单张 ≤150KB / 合计 ≤1.5MB；`-NoArchive` 可跳过），
并在评论里用仓库内相对路径列出——**入库失败只警告、不影响门结论**。
⚠️ HBuilderX 是**单实例串行资源**：需要 HBuilderX 的门（② / ④a / ④c 的 publish 段）会先走
`scripts/lib/hx-busy.ps1` 的「锁 + 忙探测 + 等待上限」；维护者正在用 GUI 时脚本会等到上限后 `exit 2`
（环境不可用，绝不抢占主程序），日志里打 `HX_BUSY wait=<秒> result=...`。**跑完任何 HBuilderX 门后先
`git status` 看 `manifest.json` 是否被 HBuilderX 回写改脏**（它会把 mp-weixin.appid 置 null），脏了就还原再继续。
低风险运行时面：改动集**只有** .uts 逻辑（无 .uvue、无三份 json、无 uni_modules）时，①② 两行**可以整行不写**；
若写了就写「免（低风险运行时面：仅 .uts 逻辑改动）」——代价是真机/渲染类问题推迟到发版前 ① 全量冒烟兜底。
例外通道：正文写明「已接受未验证风险」+ 理由 + 事后验证计划，检查会打警告放行，但**这类 PR 必须由人执行合并**。
-->

- ① Android 真机逐页截图对比 — 执行人： · 日期： · 复测对象： · 结论（含产物）：
- ② 微信开发者工具无报错（半自动：可跑 `scripts/mp-weixin-check.ps1` 产出证据，执行人栏仍由人签收） — 执行人： · 日期： · 复测对象： · 结论（含产物，引用 `MP_WEIXIN_RESULT` + 截图 `.ci-verify/*.png`；也可写「见评论 <链接>」）：
- ③ `npm run test:unit` 全绿 — 结论（含产物，贴 CI run 链接）：
- ④ 本地编译门（默认 ④c `npm run build:kotlin-all`；dev 面追加 ④a `npm run build:compile`） — 执行人： · 日期： · 复测对象： · 结论（含产物，引用 `.ci-verify/kotlin-all.log` 或 `.ci-verify/build.log`；也可写「见评论 <链接>」）：
- ④b release 云打包（触及打包面时必填；正式发版前必跑） — 执行人： · 日期： · 复测对象： · 结论（含产物）：

## 自检清单
- [ ] 提交信息遵循 Conventional Commits
- [ ] 一个分支只做一件事，可独立 revert
- [ ] 未把其他会话/模块的改动夹带进来
- [ ] 已关联对应 Issue
