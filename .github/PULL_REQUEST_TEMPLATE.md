<!--
合并方式固定 Squash and merge；目标分支固定 master（本模板此前写在移动端目录下且写着 main，GitHub 读不到、分支名也是错的）。
master 有 ruleset「protect master」，必检只有 ci-summary；pr-evidence 在本仓是**可见检查**（本仓有可用 admin 通道，但裁定不装必检——逐 PR 审批成本高于约束收益）。
真正的阻塞是行为约束：**agent 不得自行合并触及运行时面的 PR** —— 必须把证据填齐后停在「待人工签收」，由人执行合并。口径见 docs/adr/0008-移动端验收门与证据.md。
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
校验器只认「带 gate-evidence:④ 注释标记 + commit 与 head sha 相等」的评论，不校真伪。
低风险运行时面：改动集**只有** .uts 逻辑（无 .uvue、无三份 json、无 uni_modules）时，①② 两行**可以整行不写**；
若写了就写「免（低风险运行时面：仅 .uts 逻辑改动）」——代价是真机/渲染类问题推迟到发版前 ① 全量冒烟兜底。
例外通道：正文写明「已接受未验证风险」+ 理由 + 事后验证计划，检查会打警告放行，但**这类 PR 必须由人执行合并**。
-->

- ① Android 真机逐页截图对比 — 执行人： · 日期： · 复测对象： · 结论（含产物）：
- ② 微信开发者工具无报错 — 执行人： · 日期： · 复测对象： · 结论（含产物）：
- ③ `npm run test:unit` 全绿 — 结论（含产物，贴 CI run 链接）：
- ④ 本地编译门（默认 ④c `npm run build:kotlin-all`；dev 面追加 ④a `npm run build:compile`） — 执行人： · 日期： · 复测对象： · 结论（含产物，引用 `.ci-verify/kotlin-all.log` 或 `.ci-verify/build.log`；也可写「见评论 <链接>」）：
- ④b release 云打包（触及打包面时必填；正式发版前必跑） — 执行人： · 日期： · 复测对象： · 结论（含产物）：

## 自检清单
- [ ] 提交信息遵循 Conventional Commits
- [ ] 一个分支只做一件事，可独立 revert
- [ ] 未把其他会话/模块的改动夹带进来
- [ ] 已关联对应 Issue
