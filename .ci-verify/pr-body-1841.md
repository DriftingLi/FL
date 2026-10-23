## 改了什么 / 为什么改

本仓 `POST /repos/DriftingLi/FL/commits/{sha}/comments` 长期回 HTTP 404（同 token 在别的仓正常，复盘见 #1838），
门脚本 `-PostToIssue` 的「判据评论贴 commit」因此落不了地，#1838 把**症状**查清了，处置留到本票（#1841 选定 A 路：降级 + 判据面纳入票评论）。

1. **门脚本显式降级（不静默）**：`lib/gate-commit.ps1` 新增纯函数 `Get-CommitCommentKind`，`Send-GateComment` 返回
   `CommitKind` / `CommitUnavailable` / `EvidenceUrl` / `Notes`。只有「已判定可贴 + 真发了请求 + 端点回 **404**」才降级：
   同一份正文（`<!-- gate-evidence:… -->` 与 `- commit: <sha>` 一字不改）贴到票（`-PostToIssue <票号>` 那个落点）当
   **正式证据落点**；401/403/5xx/网络错误仍是硬失败；**exit code 一律不变**（#1209：报告层崩了不许把门通过改成失败）。
2. **六个调用方打印结论**：`GATE_COMMENT_DEGRADED tier=issue url=<URL>` + 人读说明（`Notes` 逐行）；硬失败打
   `GATE_COMMENT_FAILED reason=…`。不再有「静默 warn」形态。
3. **判据面纳入票评论（同构，不放宽）**：`gate-validator.mjs` 候选集 = PR 评论 + commit 评论 + **PR 正文关联的票评论**；
   新增 `issueNumbersInBody`（剥 HTML 注释 → `Closes` / `Refs` / `Fixes` / `Resolves` / `Related to #N` 与 issue 直链，
   去重 ≤5 张）。marker、`commit: <sha>` 绑定、祖先判定、其间运行时面未变、fail-closed **全部沿用**；**未关联的票不进判据**。
4. **三消费者同源**：`pr-evidence.yml`（判据仍读 base 分支，PR 改不了自己的判据）、`mobile-gates.yml`（管家）、
   `preflight-pr.mjs`（本地预检）都按同一个 `issueNumbersInBody` 取数，避免「管家说没绑、真门说绿」的假分歧。
5. **文档同步降级口径**：根 / 移动端 `AGENTS.md`、`docs/agents/release.md`、`docs/agents/checks.md`、`README.md`、PR 模板。

Closes #1841

## 改动类型
- [ ] feat（新功能）
- [x] fix（缺陷修复）
- [ ] refactor（重构）
- [ ] style（样式/格式）
- [ ] docs（文档）
- [ ] test（测试）
- [ ] chore/build/ci（构建或依赖）

## 影响范围 / 风险点

- **影响**：门证据的**落点**（commit 评论 → 404 时票评论）与**候选来源**（多一处票评论）。不改变四门判据强度、不改 exit code。
- **风险**：候选多一处 ⇒ 「别的票上的证据被误认」的面积变大。收口三件：① 只认 **PR 正文显式关联**的票（`Closes`/`Refs`，
  ≤5 张）；② 三来源走**同一条**判定路径（marker / sha 绑定 / 祖先 / 运行时面 / fail-closed 一条不少）；③ 模板注释里的历史
  票号先剥掉再解析（否则 `<!-- Closes #883 -->` 会被拉成候选）。
- **兼容窗口**：`pr-evidence.yml` 读 base 分支的判据源，本 PR 合并前 base 上还没有 `issueNumbersInBody` ⇒ 票评论来源
  **不启用**（fail-closed，不报错）；合并后自动生效。
- **已知边界**（不在本票验收项内，如实登记）：`CommitKind=skipped`（无 sha / sha 远端不可达，口径 C 不贴）时调用方
  仍会打 `GATE_COMMENT_FAILED` 机检行——语义偏噪（投递失败≠门失败），门结论不变；后续若要区分，在调用方分支加
  `-and $sent.CommitKind -ne 'skipped'` 即可。
- [ ] **本次改动触及「agent 观测不到的能力面」** —— 未勾：改动只有 `.github/**` 脚本、根 / 移动端文档与移动端 `scripts/**`
  工具链（`.ps1` / `*.test.js`），不命中运行时面判定（`*.uvue` / `*.uts` / training-app 三份 json）。

## 验收证据

- ① Android 真机逐页截图对比 — 免（未命中运行时面）
- ② 微信开发者工具无报错 — 免（未命中运行时面）
- ③ `npm run test:unit`（接线腿）+ `npm run test:behavior`（行为腿）两腿全绿 — 结论（含产物）：CI run（本 PR 由 `submit-pr.ps1` 从该 commit 的 `ci-summary` check 自动补入）
- ④ 本地编译门 — 免（未命中运行时面）
- ④b release 云打包 — 免（未命中运行时面）

**本票目标行为的真链路验证（DoD ①，工具链类改动的验收判据是「它要达成的那个行为在真实链路上成立」）**：

- **真实仓实跑降级**（`Send-GateComment` 直连 GitHub，非注桩）：`KIND=commit-unavailable UNAVAIL=True COMMITOK=False
  ISSUEOK=True` / `ERRORS=`（空——降级不算失败）/ 落点
  https://github.com/DriftingLi/FL/issues/1841#issuecomment-6366005633（该票评论**真实可访问**，正文即被 404 拒掉的那一份）。
  ⇒ 「404 被识别为可降级、降级落点真实可访问」已在真实链路上成立。
- **判据面**：`node --test .github/scripts/pr-evidence-check.test.mjs` ⇒ **92/92 绿**（新增 6 例：票评论承载门证据、与
  commit 评论**同构**、四类判红（缺 marker / 缺 commit 行 / 非祖先 / 绑定后运行时面变）、`issueNumbersInBody` 取票规则、
  三消费者静态锁）；`node --test .github/scripts/preflight-pr.test.mjs` ⇒ **6/6 绿**。
- **消费者求值路径真跑**（照 workflow 的做法剥 `export` 再 `new Function`）：`keys= validatePrEvidence,issueNumbersInBody`；
  两个 workflow YAML 解析通过（`yaml.safe_load` 无异常）。
- **移动端两腿真执行**：`gateCommitBindingBehavior.test.js`（行为腿）⇒ 1 suite / **11 tests** 通过（含 GCC0–GCC4：
  `Send-GateComment` 的 200 / 404 / 500 / 不可达四条腿**真执行**，桩只替换 `gh` 可执行文件）；`gateCommitBindingContract.test.js`
  （接线腿）⇒ 1 suite / **2 tests** 通过（K8–K11：404 判据唯一性、六调用方消费、返回字段锁定、注入口径）。

## 自检清单
- [x] 提交信息遵循 Conventional Commits
- [x] 一个分支只做一件事，可独立 revert
- [x] 未把其他会话/模块的改动夹带进来
- [x] 已关联对应 Issue
