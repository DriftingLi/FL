# 多 Agent 并发与 git 隔离（可选）

> 多个会话/自动化工具同时操作仓库前必读；单人单会话可跳过。从 AGENTS.md 拆出（2026-09-09），内容为权威版本。

多个 agent 会话（含 aider / 自动化监控）若**共用同一份 checkout 操作 git，会互相踩踏**：并发会话可能擅自 `checkout master` 并清掉分支，把另一会话刚 `commit`、尚未 `push` 的提交甩成无引用游离状态；`add`/`commit`/`push` 也会抢 `.git/index.lock`。

**单人单会话开发时不需要 worktree**，直接在主目录切分支即可。只有当多个 AI 会话/自动化工具同时操作同一个仓库时，才需要用 worktree 隔离：

- **一会话一 worktree 一分支**：动代码前先 `git worktree add ../wt-<task> -b feat/<task> origin/master`，全程在 `../wt-<task>` 内改、提交、push、开 PR；HEAD 与 index 天然隔离，避免并发 checkout/reset 互踩。

- **提交前验分支归属**：`git branch --show-current` + `git log --oneline origin/master..HEAD`，确认 HEAD 与待推提交属本会话任务，不在别的会话占用的分支上落 commit。

- **发现游离提交先取证、勿覆盖**：工作区被并发切走、自己的 commit 在 `git branch` 里消失时，用 `git log --oneline <sha>` + `git diff <sha> origin/master -- <paths>` 只读确认内容是否已在 master；已在则不 push、不 `reset`，交 gc 自然回收，绝不 `cherry-pick` 造冲突。

- **绝不** **`git add -A`**：共享工作区常混有其他会话/工具的未提交改动（如 `forum.uts`、`.aider-desk/`、`.monitor/`），只 `git add <本次文件>`。

用完 worktree 后记得清理：`git worktree remove <dir>` + `git branch -D <branch>`。
