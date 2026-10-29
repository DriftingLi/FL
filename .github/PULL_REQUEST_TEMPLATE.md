<!--
合并方式固定 Squash and merge；目标分支固定 master（本模板此前写在移动端目录下且写着 main，GitHub 读不到、分支名也是错的）。
master 有 ruleset「protect master」，必检只有 ci-summary；pr-evidence 在本仓是**可见检查**（本仓有可用 admin 通道，但裁定不装必检——逐 PR 审批成本高于约束收益）。
**证据 = PR 正文一行 `跑于 <sha7>` + 门读数行原样（A1 形态，2026-10-29 #2100 起）**：sha 绑定门评论、三候选集（PR / commit / 票评论）、`-PostToIssue`、marker 与展示副本**全部退役**（来路 #1770 / #1841 / [#1838](https://github.com/DriftingLi/FL/issues/1838) 复盘）→ `preflight-pr.mjs` 预检绿 → 再由 `submit-pr.ps1` 建 PR。**先开 PR 再补证据的老顺序已废弃**。
真正的阻塞只剩行为约束：**证据齐即 agent 直接合并，不必停在「待人工签收」**——**①b 已整体退役**（2026-10-29，#2103）：能力面 PR 合并前**零人签**、不要求设备在线，正文命中才写一行 `能力面：<类别> · 已记入发版冒烟`（**不是「需要人签」**）；**①a** 降为发版前一次的发版冒烟（见 `training-app/叉车维修培训学员端跨端应用/docs/adr/0043-真机门降级常态化与发版冒烟（2026-10-23 裁定）.md`）。证据填齐即可合并；例外通道（「已接受未验证风险」）仍由人执行合并。口径见 `training-app/叉车维修培训学员端跨端应用/docs/adr/0008-移动端验收门与证据.md`（**必须写全路径**：该 ADR 属**移动端编号体系**，根仓库另有一个同名的 `docs/adr/ADR-0008-字典描述符驱动的管理面.md`，只写 `docs/adr/0008-…` 在根目录解析不到）。
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

- [ ] **本次改动触及「agent 观测不到的能力面」**（指纹 / 运行时权限弹窗 / 真机上传 / 厂商 ROM 交互）—— 勾上**不代表需要人签**（①b 已整体退役，#2103），只表示该面记入下一次发版冒烟；机检口径 = `training-app/叉车维修培训学员端跨端应用/scripts/lib/capability-surface.ps1` 的**路径白名单** + 收口脚本提示行（`dev:finish` 打 `CAPABILITY_SURFACE touched=…`），两者**只做提示、不判红**。

## 验收证据

<!--
口径见 ADR-0008（A1 形态，2026-10-29 #2100 起）。**这一段只有一件必做的事**：哪道门触发了，就把它自带的机读行**原样**贴进来，行首写门号、行内写 `跑于 <sha7>`。

- ② 微信小程序门：命中 MP-WEIXIN 面时必填（ADR-0043 起为常态主门）
- ④ 本地编译门：命中运行时面时必填
- ① 不写（已降为发版前一次的发版冒烟）；③ 不写（两腿在 ci-summary 里，正文抄一遍零信息）；④b 不写（打包链路在发版流程里，触到只留 notes 提醒）

分级判据（哪些改动算命中运行时面 / MP-WEIXIN 面）的权威定义在 `.github/scripts/lib/gate-validator.mjs`——本模板**刻意不复述那份文件清单**：复述品会与权威定义漂移，历史上已因此产生过一次假红。未命中运行时面的 PR 写「免（未命中运行时面）」。

`<sha7>` = 门实际跑的那个 commit 的前 7 位。它等于 head、或是 head 的祖先且其间运行时面没动过，都算绑上；
其余一律要重跑并把 sha7 改成当前 head（**fail-closed**：compare 取不到也判不满足，不因 API 抖动放行）。

**图的地位**：截图是**附证**不是判据（#2108）——附在行尾供人看，校验器不读图、也不校验链接可达。
**自检**：`node .github/scripts/preflight-pr.mjs --body-file <正文文件>`（与 CI 同一份判据；红了别开 PR）。
-->

- ② 跑于 `<sha7>` · `MP_WEIXIN_RESULT ...`（`scripts/mp-weixin-check.ps1` 原样输出）· 图
- ④ 跑于 `<sha7>` · `KOTLIN_ALL_RESULT ...`（`scripts/kotlin-all-check.ps1` 原样输出；dev 面为 `COMPILE_RESULT ...`）

## 自检清单
- [ ] 提交信息遵循 Conventional Commits
- [ ] 一个分支只做一件事，可独立 revert
- [ ] 未把其他会话/模块的改动夹带进来
- [ ] 已关联对应 Issue
