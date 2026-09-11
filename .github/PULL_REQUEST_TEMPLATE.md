<!--
合并方式固定 Squash and merge；目标分支固定 master（本模板此前写在移动端目录下且写着 main，GitHub 读不到、分支名也是错的）。
master 有 ruleset「protect master」：必检 ci-summary 与 pr-evidence，且要求至少 1 个非作者 approve ——
「人工门只能人签」是结构性阻塞，不是文字规定。口径见 docs/adr/0008-移动端验收门与证据.md。
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

字段格式固定：执行人（GitHub handle，不得是 PR 作者/agent 账号） · 日期（YYYY-MM-DD） · 复测对象 · 结论（含产物）
「待人工」「⏳」或只有勾选 = 缺证据，pr-evidence 会直接红；结论必须引用可核验产物：
截图及其链接、CI run 链接、.ci-verify/build.log（只写「已验证」不算）。
例外通道：正文写明「已接受未验证风险」+ 理由 + 事后验证计划，且 PR 已获非作者 approve 时放行（会打警告）。
-->

- ① Android 真机逐页截图对比 — 执行人： · 日期： · 复测对象： · 结论（含产物）：
- ② 微信开发者工具无报错 — 执行人： · 日期： · 复测对象： · 结论（含产物）：
- ③ `npm run test:unit` 全绿 — 结论（含产物，贴 CI run 链接）：
- ④a dev 全量编译（`npm run build:compile`） — 执行人： · 日期： · 复测对象： · 结论（含产物，引用 `.ci-verify/build.log`）：
- ④b release 云打包（收口 PR / 新增 async / 新增 composable 时必填） — 执行人： · 日期： · 复测对象： · 结论（含产物）：

## 自检清单
- [ ] 提交信息遵循 Conventional Commits
- [ ] 一个分支只做一件事，可独立 revert
- [ ] 未把其他会话/模块的改动夹带进来
- [ ] 已关联对应 Issue
