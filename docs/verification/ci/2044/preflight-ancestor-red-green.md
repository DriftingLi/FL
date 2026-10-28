# #2044 预检 CLI fetchCompare 参数契约修复 · 红绿对照取证

日期：2026-10-28 · 执行人：agent（zhengcookie 会话）
判据：票面验收 3 ——「在一个『门评论绑祖先 sha』的真实分支上跑 preflight-pr.ps1 拿绿（#2035 分支可当回归样本）」。

## 样本构造

- 载体：PR #2046（Closes #2035）的 head `ffba63fd`（`git fetch origin refs/pull/2046/head`），worktree 检出后正文用该 PR 原正文（导出自 `gh pr view 2046 --json body`）。
- 该样本天然命中漂移路径：#2035 票上的门评论绑 **祖先 sha** —— ② 绑 `522bde8c`、④ 绑 `89de5828`，均非 head（#1841 降级常态落点）。
- 受控变量 = 仅脚本版本（同一 worktree、同一正文、同一 head）。以下三段围栏为**逐字捕获**（`Out-String` 原文，首行为退出码）。

## 对照组：修复前 CLI（ffba63fd 自带脚本，位置参数形态）

```
EXIT=1
compare [object Object]...undefined 失败，按 fail-closed 处理：from.replace is not a function
#1770 预检（与 CI 同一份判据：.github/scripts/lib/gate-validator.mjs）
  commit   ffba63fde910
  base     origin/master（compare diverged，7 个文件）
  author   @zhengcookie
  证据     正文 有 · PR 评论 0 条 · commit 评论 0 条（3 个 commit） · 票评论 3 条（正文关联的票）
  ci-summary  success https://github.com/DriftingLi/FL/actions/runs/40522004923/job/122253914408

  ① ❌ 未绑
  ② ❌ 未绑
  ④ ❌ 未绑

  ❌ ④ 本地编译门（默认 ④c 整模块；dev 面追加 ④a）：「结论」须引用 `.ci-verify/build.log`（④a）或 `.ci-verify/kotlin-all.log`（④c）。

  · ① Android 真机逐页截图对比：跳过（①a 降级常态，ADR-0043）——「结论」写的是「免（①a 降级发版前一次，ADR-0043）」；①a 已降级为发版前一次（发版冒烟），本 PR 不做逐页真机取证。
  · ② 触发面命中（MP-WEIXIN 面，ADR-0043 起为常态主门、无低风险减免）：training-app/叉车维修培训学员端跨端应用/pages/forum/components/forum-markdown-blocks.uvue、training-app/叉车维修培训学员端跨端应用/utils/externalLinkDispatch.uts。
  · ④ 门证据未绑上：绑定 `89de582`：取不到 compare(`89de582`…head) 结果（未知 sha / API 异常 / 超时）⇒ fail-closed 不满足
  · ② 门证据未绑上：绑定 `522bde8`：取不到 compare(`522bde8`…head) 结果（未知 sha / API 异常 / 超时）⇒ fail-closed 不满足

结论：红 —— 先补齐上面的缺项，再重跑预检（绿了才开 PR）
```

即票面症状逐字复现：本地预检恒红，而同一份证据当时由 CI（对象契约注入）判绿、PR #2046 已合并。

## 实验组 1：修复后 CLI（origin/master 版 preflight-pr.mjs）

```
EXIT=0
#1770 预检（与 CI 同一份判据：.github/scripts/lib/gate-validator.mjs）
  commit   ffba63fde910
  base     origin/master（compare diverged，7 个文件）
  author   @zhengcookie
  证据     正文 有 · PR 评论 0 条 · commit 评论 0 条（3 个 commit） · 票评论 3 条（正文关联的票）
  ci-summary  success https://github.com/DriftingLi/FL/actions/runs/40522004923/job/122253914408

  ① ❌ 未绑
  ② ✅ 已绑
  ④ ✅ 已绑

  · ① Android 真机逐页截图对比：跳过（①a 降级常态，ADR-0043）——「结论」写的是「免（①a 降级发版前一次，ADR-0043）」；①a 已降级为发版前一次（发版冒烟），本 PR 不做逐页真机取证。
  · ② 微信开发者工具门（半自动）：证据来自 PR 评论（sha 绑定 522bde8 —— 祖先口径，当前 head 为 ffba63f）；执行人栏仍须由人签收。
  · ④ 本地编译门：证据来自 PR 评论（sha 绑定 89de582 —— 祖先口径，当前 head 为 ffba63f）。
  · ② 触发面命中（MP-WEIXIN 面，ADR-0043 起为常态主门、无低风险减免）：training-app/叉车维修培训学员端跨端应用/pages/forum/components/forum-markdown-blocks.uvue、training-app/叉车维修培训学员端跨端应用/utils/externalLinkDispatch.uts。
  · 证据结构完整（pr-evidence 只校结构，真伪由维护者判定）。
  · ④ 门证据的绑定口径：commit `89de582` 是 head 的祖先且其间运行时面（.uvue / .uts / 三份 json）未变（放宽判据见 ADR-0008）。
  · ② 门证据的绑定口径：commit `522bde8` 是 head 的祖先且其间运行时面（.uvue / .uts / 三份 json）未变（放宽判据见 ADR-0008）。

结论：绿 —— 可以开 PR（提交前再确认 HEAD 已推到 origin）
```

## 实验组 2：票面字面判据 —— preflight-pr.ps1 外壳层（origin/master 版 mjs + ps1）

```
EXIT=0
#1770 预检（与 CI 同一份判据：.github/scripts/lib/gate-validator.mjs）
  commit   ffba63fde910
  base     origin/master（compare diverged，7 个文件）
  author   @zhengcookie
  证据     正文 有 · PR 评论 0 条 · commit 评论 0 条（3 个 commit） · 票评论 3 条（正文关联的票）
  ci-summary  success https://github.com/DriftingLi/FL/actions/runs/40522004923/job/122253914408

  ① ❌ 未绑
  ② ✅ 已绑
  ④ ✅ 已绑

  · ① Android 真机逐页截图对比：跳过（①a 降级常态，ADR-0043）——「结论」写的是「免（①a 降级发版前一次，ADR-0043）」；①a 已降级为发版前一次（发版冒烟），本 PR 不做逐页真机取证。
  · ② 微信开发者工具门（半自动）：证据来自 PR 评论（sha 绑定 522bde8 —— 祖先口径，当前 head 为 ffba63f）；执行人栏仍须由人签收。
  · ④ 本地编译门：证据来自 PR 评论（sha 绑定 89de582 —— 祖先口径，当前 head 为 ffba63f）。
  · ② 触发面命中（MP-WEIXIN 面，ADR-0043 起为常态主门、无低风险减免）：training-app/叉车维修培训学员端跨端应用/pages/forum/components/forum-markdown-blocks.uvue、training-app/叉车维修培训学员端跨端应用/utils/externalLinkDispatch.uts。
  · 证据结构完整（pr-evidence 只校结构，真伪由维护者判定）。
  · ④ 门证据的绑定口径：commit `89de582` 是 head 的祖先且其间运行时面（.uvue / .uts / 三份 json）未变（放宽判据见 ADR-0008）。
  · ② 门证据的绑定口径：commit `522bde8` 是 head 的祖先且其间运行时面（.uvue / .uts / 三份 json）未变（放宽判据见 ADR-0008）。

结论：绿 —— 可以开 PR（提交前再确认 HEAD 已推到 origin）
```

无 `[object Object]` 残迹；②④ 由「❌ 未绑 ⇒ fail-closed」翻为「✅ 已绑 ⇒ 祖先且其间运行时面未变」。

## 测试面契约钉（防再漂移）

`.github/scripts/preflight-pr.test.mjs` 尾部三条 #2044 用例：真校验器 × 真 CLI 工厂祖先绑定判绿（旧形态必红）、工厂对象契约（剥 origin/ 前缀 / 按 key 缓存 / 异常收敛 null）、四方（校验器 / pr-evidence.yml / mobile-gates.yml / CLI）形态文本钉。本地 9/9 绿、校验器套件 104/104 绿，CI `node --test` 同判绿。
