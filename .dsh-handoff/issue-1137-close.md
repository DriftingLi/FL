## 已收口（#1174 合并）

补钉已实现并进主干：根 `.gitattributes` 追加

```
*.uts text eol=lf
*.uvue text eol=lf
```

- **PR**：#1174（squash `83645c80`）
- **门禁**：`ci-summary` pass · `pr-evidence` pass · `mobile-test` skipping（改动集只有根 `.gitattributes`，`changes` 判定 mobile=false）

### 收口前复核的三条事实

1. **无 renormalize 风险仍成立**：全仓 `git ls-files --eol` 里 0 个 `i/crlf` / `i/mixed`，且无 `* text=auto` ⇒ 本钉只改检出行为，不改动任何已提交内容。
2. **真实链路上生效**：把 worktree 的 `core.autocrlf` 置真并强制重新检出后，**203** 个 `*.uts` / `*.uvue` 由 `i/lf w/crlf` 变为 `i/lf w/lf`，`utils/format.uts` 实测 `CR=0 / LF=135`，且 `git status` **零幻影 diff**；同步主干后再检出仍是 `w/lf`。
3. **前后对照**（最小隔离仓库，排除大仓噪声）：无规则时 `format.uts` / `thing.uvue` 检出为 CRLF；加规则后二者与本已钉过的 `README.md` 一并变 LF ⇒ 机制确实在跑、对照组有效。

### 与本票 triage 结论的两处出入（照实记）

- 票面称「约 62 个契约测试在读 `*.uts` / `*.uvue` 源文本」—— 实测读源文本的是 **72** 个，但其中真正使用**跨行匹配锚点**的只有 **1** 个，且正是已被 `#1151`（`normalizeEol`）修好的那个 ⇒ **当前实际暴露面为 0**，本票性质是「拆掉一颗埋着的雷」，不是「修一条正在红的测试」。
- 票面第 3 条建议（改 `:229` 的正则为 `[^\n]*\r?\n`）**未采用** —— 它只是绕过症状，且 `#1151` 已在 reader 层统一归一化，无需再改正则。

行号也有漂移：票面 `:231` 在当前主干是 `:242`（文件长了约 11 行，断言本身未变）。
