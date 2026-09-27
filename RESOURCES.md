# 技能与验收机制 Resources

## Knowledge

- [Repo: mattpocock/skills（GitHub 上游真源）](https://github.com/mattpocock/skills)
  25 个技能的源头与设计动机：「Why Four Failure Modes」四节 + 文末 Reference 的 user-invoked / model-invoked 划分。用 for：技能是什么、分类、设计哲学。
- 本地 bundled deck：`C:\Users\ZHENG\.dsh\profiles\web\node_modules\dsh-mattpocock-skills-deck\bundled-skills\`（pin = `v1.2.3`，25 个技能目录 + `README.md` + `VERSION`）
  harness 实际注入的那份。用 for：查某个技能的 SKILL.md 全文与配套文件。
- [Docs: 技能供给与管线归属（仅移动端线）](training-app/叉车维修培训学员端跨端应用/docs/agents/skills.md)
  技能扫描根 rank 表、四条约定、每次现测的判据命令。用 for：「技能从哪加载 / 副本为什么不生效」。
- [ADR-0024: 技能供给与管线归属（移动端编号）](training-app/叉车维修培训学员端跨端应用/docs/adr/0024-技能供给与管线归属.md)
  上一条的决策与逐项实测出处（死副本清退、12 个独有技能、生效面）。用 for：核对「为什么不往仓里放技能」。
- [Docs: 测试与检查流程（根仓权威命令表）](docs/agents/checks.md)
  后端四件套 / 前端 type-check+vitest / 部署与安全检查的完整判据。用 for：第 2 课「验证层有哪些」的唯一真源，引用时不抄表。
- [ADR-0008: 移动端验收门与证据（移动端编号）](training-app/叉车维修培训学员端跨端应用/docs/adr/0008-移动端验收门与证据.md)
  四门（①真机 ②微信工具 ③单测 ④编译）的判据与原因。用 for：回答「每道门到底在验什么」。
- [ADR-0016: 真机门的人工性收缩与按批取证（移动端编号）](training-app/叉车维修培训学员端跨端应用/docs/adr/0016-真机门的人工性收缩与按批取证.md)
  人工门如何从「都要人看」收缩到只剩 ①b 能力面。用 for：回答「是否真需要人工验收门」。
- [AGENTS.md: 验收门（合并前置）](AGENTS.md)
  硬口径：低风险白名单逐文件、结论行行内产物、签收在人合并不限人。用 for：第 3 课讲「门为什么会形同虚设 / 怎么不形同虚设」。

## Wisdom (Communities)

- 本仓 GitHub Issues（`gh` CLI）—— 真实争议的裁决记录（#1030 触发判据教训、#1156 恒绿证伪、#883 半自动门）。用 for：看「一条门为什么长这样」的一手讨论，比任何二手总结可信。
- 上游仓库 Issues（[mattpocock/skills](https://github.com/mattpocock/skills/issues)）—— 技能设计问题的原作者讨论。

## Gaps

- 没有找到「AI 生成代码的自动验证到底拦得住多少」的高质量外部实证（业界研究多为厂商自报）。第 3 课先用本仓血账与 ADR 讲，外部证据留待后续检索。
