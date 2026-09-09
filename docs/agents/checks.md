# 测试与检查流程

> 每次提交前必读。从 AGENTS.md 拆出（2026-09-09），内容为权威版本。

改动后**必须**跑完对应栈的检查，全绿才能提交：

## 后端（`backend/`）

检查项四件套（两个环境任选其一，结果一致）：

- `gofmt -l .`（应无输出）
- `go vet ./...`
- `golangci-lint run ./...`（errcheck 等静态检查）
- `go test ./...`

**环境 A：Windows 本机（Git Bash）—— 2026-09-08 起实测可用，优先使用**

- 工具链已装在 Windows 本机并在 PATH：go 1.26.4 + golangci-lint v1.64.8，四项检查直接跑，**不依赖 WSL**。
- 已知例外：`internal/logger` 的 `TestNew_FileOutput` / `TestRedactHook_AppliedByFactory` 在 Windows 下因 TempDir 文件锁失败（`unlinkat ... The process cannot access the file because it is being used by another process`），与改动无关，可忽略；CI（Linux）不受影响。

**环境 B：WSL**

- Go 工具链在 `~/go/bin`（`export PATH=/home/root86155/go/bin:$PATH`）。
- 已知例外：`internal/api` 的 `TestStaticOtherResource` 在 WSL 下因 `static/favicon.ico` 权限问题失败，与改动无关，可忽略。

## 前端（`frontend/`）

`cd frontend` 后：

- `npm run type-check`（vue-tsc）
- `npm test`（vitest）

## 部署配置

改 `docker-compose*.yml` / `deploy.sh` 后可用 `docker compose -f docker-compose.prod.yml config -q` 做语法校验。

## 安全检测

改动触及认证/授权/密钥/DB 连接/AI 生成代码时，跑 `python -m deepsec shield scan backend frontend/src`，确认无新增 critical/high（已知误报见 `docs/agents/security-scan.md`）。
