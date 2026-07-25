# Git 命令速查（FL 团队）

> 配合 `docs/GIT_WORKFLOW.md` 使用。默认远程为 `origin`（DriftingLi/FL），主分支 `main`。

## 开工
```bash
git fetch origin
git checkout main && git pull --ff-only origin main
git checkout -b feat/我的任务 origin/main
```

## 日常开发（原子提交）
```bash
git add <具体文件>                       # 不要 git add -A
git commit -m "feat(module): 做了什么"
git status                               # 提交前看清楚改了啥
```

## 同步最新 main（保持线性，❌ 禁止 merge）
```bash
git fetch origin
git rebase origin/main                  # 冲突→解决→git add→git rebase --continue
git push --force-with-lease origin feat/我的任务   # 已推过才需要
```

## 提 PR
```bash
git push -u origin feat/我的任务
# GitHub 上 New Pull Request → base: main → Squash and merge
```

## 合并后清理
```bash
git checkout main && git pull --ff-only origin main
git branch -d feat/我的任务
git push origin --delete feat/我的任务
```

## 整理提交（推送前）
```bash
git rebase -i HEAD~3        # 后几条 pick 改 squash / fixup 合并
git commit --amend          # 改最近一条的信息（未推送）
```

## 救火
```bash
git rebase --abort          # rebase 搞砸，立刻退出
git reflog                  # 找回误 reset 的提交
git reset --hard <sha>      # 谨慎！回退到 reflog 里的某个点
```

## 多任务并行（worktree）
```bash
git worktree add ../fl-hotfix fix/login-redirect   # 独立目录互不污染
git worktree list
git worktree remove ../fl-hotfix
```

## 发布打 tag
```bash
git tag -a v1.2.0 -m "release: v1.2.0"
git push origin v1.2.0
```

## 绝对禁止
- ❌ 对 `main` 强推（`git push --force` on main）
- ❌ `git merge origin/main` 制造同步合并提交
- ❌ `git commit -a -m "update"` 之类模糊提交
- ❌ 用个人名分支（`zhengcookie` 等）当工作分支
- ❌ `git add -A` 一把梭，把无关文件塞进提交
