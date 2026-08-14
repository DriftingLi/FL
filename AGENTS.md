# 叉车维修培训与残值评估系统

面向叉车维修培训与叉车残值评估的全栈系统。架构、领域词汇与评审记录见下方文件：

- **领域词汇表**：`CONTEXT.md`（repo 根）
- **架构决策记录（ADRs）**：`docs/adr/`
- **AI/agent 工作约定**：`docs/agents/`

## Agent skills

### Issue tracker

Issues 存放在 GitHub Issues（使用 `gh` CLI）。See `docs/agents/issue-tracker.md`.

### Triage labels

五个 canonical triage roles，label 与 role 同名（`needs-triage` 等）。See `docs/agents/triage-labels.md`.

### Domain docs

Single-context：root `CONTEXT.md` + `docs/adr/`。See `docs/agents/domain.md`.

### Security scan

AI 安全审计用 DeepSec（Shield）。See `docs/agents/security-scan.md`.

## 前端 UI 约定

页面保持整洁：不要写冗余的小标题、装饰性提示与说明性 hint 文本，有的话就清理，仅保留必要的功能性提示。删除 hint 时同步删除对应的 CSS class 与 scoped style，避免残留死代码。

## 测试与检查流程

改动后**必须**跑完对应栈的检查，全绿才能提交：

- **后端（`backend/`）**：Go 工具链在 `~/go/bin`（`export PATH=/home/root86155/go/bin:$PATH`）
  - `gofmt -l .`（应无输出）
  - `go vet ./...`
  - `golangci-lint run ./...`（errcheck 等静态检查）
  - `go test ./...`
  - 已知例外：`internal/api` 的 `TestStaticOtherResource` 在 WSL 下因 `static/favicon.ico` 权限问题失败，与改动无关，可忽略
- **前端（`frontend/`）**：`cd frontend`
  - `npm run type-check`（vue-tsc）
  - `npm test`（vitest）
- **部署配置**：改 `docker-compose*.yml` / `deploy.sh` 后可用 `docker compose -f docker-compose.prod.yml config -q` 做语法校验
- **安全检测**：改动触及认证/授权/密钥/DB 连接/AI 生成代码时，跑 `python -m deepsec shield scan backend frontend/src`，确认无新增 critical/high（已知误报见 `docs/agents/security-scan.md`）。

## 发布流程（push / PR / merge）

master 有仓库 ruleset「protect」保护（直接 push 会被拒，`push declined due to repository rule violations`）。发布必须走分支 + PR：

1. **本地提交**（只 add 本次改动的文件，勿 `git add -A`）。
2. **建分支推送**：若提交已在本地 master 上，`git branch feat/xxx` 后 `git reset --hard origin/master` 还原本地 master；然后 `git push -u origin feat/xxx`。
3. **CI 自动跑**：push 事件触发全量 CI；ci-summary 通过后**自动触发 CD 部署 testing**（非 master 分支 → testing 环境）。
4. **创建 PR**：`gh pr create --base master --head feat/xxx --title "..." --body "..."`。
5. **等门禁**：`gh run watch <id> --exit-status` 等 CI 全绿（纯前端改动时 backend job 跳过属正常）；**必须等 CD testing 部署成功**（ruleset 的 `required_deployments: testing` 是 merge 前置条件），用 `gh run list --workflow cd.yml` 找到对应 commit 的 run 并 watch。
6. **Squash merge**：`gh pr merge <n> --squash --delete-branch`。若报 "requirements have not been met"，用 `gh pr view <n> --json statusCheckRollup` 排查，确认 testing 部署完成后重试。
7. **收尾**：`git fetch --prune` → `git checkout master && git pull --ff-only` → 删除本地 feat 分支（若 gh 未自动删）。
