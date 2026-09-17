# 测试与检查流程

> 每次提交前必读。从 AGENTS.md 拆出（2026-09-09），内容为权威版本。

改动后**必须**跑完对应栈的检查，全绿才能提交：

## 后端（`backend/`）

检查项四件套（两个环境任选其一，结果一致）：

- `gofmt -l .`（应无输出）
- `go vet ./...`
- `golangci-lint run ./...`（errcheck 等静态检查）
- `go test ./...`
- 改了 handler 的 swagger 注解（`@Success` / `@Param` / `@Router` 等）后：`cd backend && make swagger` 再生成 `backend/docs/{docs.go,swagger.json,swagger.yaml}` 并一并提交 —— CI 的 backend-lint 有**新鲜度锁**（按钉住的 swag 版本再生成后要求工作树干净），生成物过期直接红

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
- 新建 spec 一律用 `epLite()`（`src/test/element-lite.ts`）按需注册 EP 组件，**禁止全量挂载 `plugins: [ElementPlus]`** —— 全量挂载是 CourseCatalog flaky（CI 2 核下 import 争抢超时）的根因；组件清单可用 `node scripts/scan-el-components.mjs` 扫描
- 已收敛控件守卫：`node scripts/check-el-controls.mjs --all`（CI 在 frontend-check 里跑全量；本地也可 `--diff origin/master` 只看新增行）。守卫判定逻辑的自检：`node --test scripts/check-el-controls.test.mjs`
- api seam 守卫（ADR-0053 §7）：`node scripts/check-api-seam.mjs --all`（页面与业务组件不得直接引用 `@/api/request` / `@/api/client`；同样在 frontend-check 跑全量，本地可 `--diff origin/master`）。自检：`node --test scripts/check-api-seam.test.mjs`。逐条登记的例外写在脚本的 `ALLOWLIST`（每条带理由）
- **守卫 runner 单点（第十一波）**：三个守卫（api-seam / el-controls / async-section）的 runner 面（argv 解析 / 走查 / `--diff` 取 diff 文本 / allowlist 放行 / 报告与退出码）收在 `scripts/lib/guard.mjs`，守卫文件只留判定面与措辞（`GUARDED_*` 常量 + `scanSource` + `GUARD_SPEC`；新增守卫只需实现 `scanSource`）。`--diff` 是 **fail-closed** 的：base 解析不了、`git diff` 失败、新增文件读不出来都报错并非零退出，不会静默判绿（历史上 `--diff` 曾因实参形态错而恒 0 违规、CI 只跑 `--all` 无人接住）。runner 自带表驱动自测：`node --test scripts/guard.test.mjs`（合成 diff 正/负样本 + fail-closed 面 + 三个守卫的 CLI 面）；CI 在 frontend-check 里除 `--all` 全量外，另有增量门按 `--diff origin/<默认分支>` 跑三个守卫
- **契约消费面覆盖锁（第十一波）**：`frontend/src/api/**` 里出现的 method+path 必须落在 `backend/internal/apitypes` 的域声明表内（消费了未登记端点即红）；存量欠条登记在脚本 ALLOWLIST，补注解后销号
- 覆盖率：`npx vitest run --coverage`（istanbul provider，text + html 报告落 `coverage/`，不设阈值不挂门禁）；baseline（2026-09-10）：全站 lines 33.3%，admin 10.4% 为最大盲区

## 部署配置

改 `docker-compose*.yml` / `deploy.sh` 后可用 `docker compose -f docker-compose.prod.yml config -q` 做语法校验。

**迁移失败即中止部署**（第十一波）：`scripts/deploy-remote.sh` 不再把迁移失败降级为 `log_warn` 后继续；线上事故确需继续时走显式逃生开关（变量名写在脚本内并登记理由），不得靠改日志绕过。CI 侧 `migration-check` 在现成的 Postgres 上真跑 `go run ./cmd/migrate up`（空库 baseline），并做**单向列对账**：GORM 模型期望的列集合必须 ⊆ `information_schema` 实际列集合，缺列即红（防「模型加字段、忘写迁移」）。

**反代到后端的每个 `location` 必须显式设置 `X-Forwarded-For`**（`frontend/nginx-host.conf`、`frontend/nginx.default.conf`）：nginx 只在设置时才覆写/追加该头，没设置的 location 会把客户端自带的同名头原样透传；后端信任本机对端（`TRUSTED_PROXIES`）之后会采信那个伪造值——限流键可被轮换、访问日志与审计日志写入假 IP。新增或改动反代 location 时逐条核对（#888 的 `/static/` 就是漏网的那条）。

## 安全检测

改动触及认证/授权/密钥/DB 连接/AI 生成代码时，跑 `python -m deepsec shield scan backend frontend/src`，确认无新增 critical/high（已知误报见 `docs/agents/security-scan.md`）。
