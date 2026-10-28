# #2044 预检 CLI fetchCompare 参数契约修复 · 红绿对照取证

日期：2026-10-28 · 执行人：agent（zhengcookie 会话）
判据：票面验收 3 ——「在一个『门评论绑祖先 sha』的真实分支上跑 preflight 拿绿（#2035 分支可当回归样本）」。

## 样本构造

- 载体：PR #2046（Closes #2035）的 head `ffba63fd`（`git fetch origin refs/pull/2046/head`），worktree 检出后正文用该 PR 原正文（导出自 `gh pr view 2046 --json body`）。
- 该样本天然命中漂移路径：#2035 票上的门评论绑 **祖先 sha** —— ② 绑 `522bde8c`、④ 绑 `89de5828`，均非 head（#1841 降级常态落点）。

## 对照组（修复前 CLI，d4baac48~1 版脚本）

```
EXIT=1
compare [object Object]...undefined 失败，按 fail-closed 处理：from.replace is not a function
  ② ❌ 未绑   ④ ❌ 未绑
结论：红 —— 先补齐上面的缺项，再重跑预检
```

即票面症状逐字复现：本地预检恒红，而同一份证据当时由 CI（对象契约注入）判绿、PR #2046 已合并。

## 实验组（修复后 CLI，d4baac48 版脚本）

```
EXIT=0
  ② ✅ 已绑   ④ ✅ 已绑
· ② 门证据的绑定口径：commit `522bde8` 是 head 的祖先且其间运行时面（.uvue / .uts / 三份 json）未变（放宽判据见 ADR-0008）。
· ④ 门证据的绑定口径：commit `89de582` 是 head 的祖先且其间运行时面未变。
结论：绿 —— 可以开 PR
```

无 `[object Object]` 残迹。受控变量 = 仅脚本版本（同一 worktree、同一正文、同一 head）。

## 测试面契约钉（防再漂移）

`.github/scripts/preflight-pr.test.mjs` 尾部三条 #2044 用例：真 CLI 工厂 × 真校验器祖先绑定判绿（旧形态必红）、工厂对象契约（剥 origin/ 前缀 / 按 base 缓存 / 异常收敛 null）、三方（校验器 / pr-evidence.yml / mobile-gates.yml / CLI）形态文本钉。本地 9/9 绿、校验器套件 104/104 绿。

## ps1 外壳层复核（票面字面判据补全）

评审指出对照组只跑了裸 node CLI；票面写的是跑 `preflight-pr.ps1`。复核：同一样本树上仅换回修复后的 `.github/scripts/preflight-pr.mjs`，走 `pwsh scripts/preflight-pr.ps1 -BodyFile body.md` ⇒ **EXIT=0**，②④ ✅ 已绑、`[object Object]` 零出现（2026-10-28 09:3x 实测）。外壳层与裸 CLI 同结论。
