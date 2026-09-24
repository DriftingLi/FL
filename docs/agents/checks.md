# 测试与检查流程

> 每次提交前必读。从 AGENTS.md 拆出（2026-09-09），内容为权威版本。

改动后**必须**跑完对应栈的检查，全绿才能提交：

## 后端（`backend/`）

检查项四件套（两个环境任选其一；⚠️ **本机现在只能跑三条**——golangci-lint 与 go 1.27 不匹配，见「环境 A」，第四条由 CI `backend-lint` 兜）：

- `gofmt -l .`（应无输出）
- `go vet ./...`
- `golangci-lint run ./...`（errcheck 等静态检查）
- `go test ./...`
- **Postgres 契约测试的两条纪律**（`testutil.NewPostgresDB`，先例见 `#1197` 的 `contact_window_postgres_contract_test.go`）：
  1. 它为每个测试建**随机 schema** 并跑真实 `migrations/`，`DATABASE_URL` 未设时干净 `t.Skip` ⇒ 本机（无 PG）看到 `ok` **不等于测过**；这类测试的首跑在 CI（`backend-test` 带 PG 15 + `-race`，`migration-check` 真跑 up→列对账→down 到底），写的时候就按「CI 才第一次真跑」准备，别把 skip 当验证。
  2. 目录类断言**必须按 schema 收窄**：`go test ./...` 并发跑多个包、共用同一个测试库，而 `pg_indexes` / `pg_constraint` / `information_schema.columns` 是**全库视图** ⇒ 不加 `schemaname = current_schema()`（或 join `pg_namespace` 后按 `nspname` 收窄）就会数到别的包那份同名对象（#1197 首跑正是「CHECK 数到 2 行」）。存量同类隐患：`internal/api/forum_experience_migration_contract_test.go` 的 `information_schema.columns` 查询未带 schema 条件。
- 端点错误面守卫（第十三波 ADR-0060 决策 1 / 票 1b）：`node scripts/check-render-error-face.mjs --all`（本地可 `--diff origin/master` 只看新增行）—— `RenderFunc` 已不含 `err` 参数，错误面归端点骨架无条件渲染，所以 `backend/internal/api/` 的 `Render` 闭包内不得再出现 `response.ServerError/BadRequest/NotFound/Unauthorized/Forbidden`、`renderStatus(` 或 `xxxErrStatus.renderError(`；状态码与固定文案写进本端点的 `ErrStatus`（单码用 `errStatusAll` / `errStatusAllMsg`，哨兵用 `errStatusEntry`，`sentinel == nil` 表示无条件命中且抢在 `*ParseError` 规则之前）。需要非信封错误形状的面走 raw handler（不留逃生槽）。自检：`node --test scripts/check-render-error-face.test.mjs`；CI 在 backend-lint 跑 `--all`、判定逻辑自检在 el-controls-selftest
- 改了 handler 的 swagger 注解（`@Success` / `@Param` / `@Router` 等）或**任何进出响应 DTO 的字段**（含可空性）后：`cd backend && make swagger` 再生成 `backend/docs/{docs.go,swagger.json,swagger.yaml}`，**再** `go run ./cmd/gen-apitypes` 再生成 `frontend/src/api/generated/*.ts`，两者与改动一并提交 —— CI 的 backend-lint 有**新鲜度锁**（按钉住的 swag 版本再生成后要求工作树干净），生成物过期直接红。
  ⚠️ **顺序是硬的**：`internal/apitypes/codegen_test.go` 把前端生成文件与 Go 注解渲染结果全等比对，所以「先跑 `go test ./...` 再 `make swagger`」会先给你一个绿、几秒后变红（2026-09-20 #1197 实测踩到：全量套件通过后再生成 swagger，`apitypes` 随即判红）。收口顺序固定为 gofmt/vet → 再生成（swagger → gen-apitypes）→ `go test ./...` → 前端检查。
- 改过**响应字段的两个 struct tag** 时（`nullability:` 可空性表态 / `fact:` 消费点对齐，词汇见 `CONTEXT.md`「接口契约」）：这两把锁都按**生成物**读射程，所以顺序同上（先 `make swagger` 再跑测试）。它们管的东西不同，别混成一条：
  - `nullability:"nullable"` 要在 `internal/service/nullable_declaration_test.go` 的 `nullableOutlets*` 表里举出一个真 marshal 出 `null` 的出口；`nonnil` 要在 `nonnilOutlets*`（service 或 api 那张，按**组装发生在哪一层**选）里举出一个真发 `[]` / `{}` 的出口。锁在 `internal/apitypes/nullability_lock_test.go`。**这一张是欠账表**：未举证的数量钉成常量（判据 3/4），只准减，涨与降都要人改常量并同步注释里的算式。
  - `fact:"<key>"` 要登记进 `internal/api/consumption_fact_registry.go`，且必须**同时**有 ≥1 个明文面载体与 ≥1 个投影位（单侧谈不上对齐）。锁分两处：`internal/api/consumption_fact_lock_test.go`（表自洽 + 表⇔tag 双向 + 契约描述含那句错误）与 `internal/apitypes/fact_reachability_lock_test.go`（投影位所在类型必须在 2xx 闭包里）。**这一张不是欠账表**——加行是目的，不是债务，所以没有常量可减。
  ⚠️ `fact:` 有一条机器管不到，必须靠评审补：**登记表认领某个 error 作「明文面载体」时，要另写一条 HTTP 级断言钉那句话真发得出去**。对齐锁读的是生成物里的静态描述，把 handler 换成另一枚哨兵它不会红（先例见 `contact_company_disable_contract_test.go` 里那句 `403 的正文要带那件事实的具名句子`）。
- 目录排序串第二源守卫（第十三波 ADR-0060 决策 10 / 票 10）：`node scripts/check-catalog-sort.mjs --all`（本地可 `--diff origin/master` 只看新增行）—— 目录读面（文件名含 `catalog` 的非测试 `.go`）不得再出现与 catalog descriptor `OrderBy` 声明**逐字同串**的裸排序串，排序串的唯一宿主是 spec 表（`catalog_specs.go` / `position_catalog.go`），读面写 `q.Order(specialtyCatalogSpec().OrderBy)` 这类引用。判据窄是刻意的：带表别名的课程行 `course.sort_order ASC, course.course_id ASC` 与章节行 ADR 明记不动（收进射程就得给整个读面文件开 ALLOWLIST，`--all` 的例外是整文件放行，会连带盲掉真危险行）。自检：`node --test scripts/check-catalog-sort.test.mjs`（含「声明表与 spec 表互等」一致性锁与合成违规目录必须判红的防恒绿用例）；CI 在 backend-lint 跑 `--all`、判定逻辑自检在 el-controls-selftest

**环境 A：Windows 本机（Git Bash）—— 2026-09-08 起实测可用，优先使用**

- 工具链装在 Windows 本机并在 PATH：go **1.27.1** + golangci-lint v1.64.8；gofmt / go vet / go test 直接跑，**不依赖 WSL**。
- ⚠️ **`golangci-lint` 本机目前跑不了**（2026-09-20 实测）：v1.64.8 是用 go 1.26.4 构建的，面对 1.27.1 的导出数据直接失败——`cannot decode "internal/goarch": export data version 4 is greater than maximum supported version 2`，且在**未改动的包**上报错，属环境性。四条检查本机实为三条 + CI 兜底（`backend-lint` job 跑 `golangci-lint run ./...`）。**补 CI 那条腿人工能做的**：删掉或重构 Go 代码后 grep 一遍被删符号的残留引用（历史上票6 残留的一个 `toIntDefault` 就是这么在 CI 才红的）。升级匹配本机 go 的 golangci-lint 前，别把「本地四件套全绿」当成事实。
- 已知存量失败（与改动无关，可忽略；CI 为 Linux 不受影响）：`internal/logger` 的 `TestNew_FileOutput` / `TestRedactHook_AppliedByFactory`（Windows TempDir 文件锁）；`internal/deploy` 的 `TestDeployEnvChainPreservesProvidedValues`。
- ⚠️ 全量 `go test ./...` 会在**仓库根**落下垃圾文件（形如 `C：Users...TestDeployEnvChain...deploy-env.sh`，其中的「：」是 U+F03A 私有区字符，按字面名 `rm` 删不掉）：每次跑完全量后用 `find . -maxdepth 1 -type f -name '*TestDeployEnvChain*' -delete` 清掉，别让它进 `git status` 或提交。

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
- **守卫 runner 单点（第十一波）**：**七个**守卫（api-seam / el-controls / async-section / 契约消费面覆盖锁 / AI 助手发送编排 / 端点错误面 / 目录排序第二源）的 runner 面（argv 解析 / 走查 / `--diff` 取 diff 文本 / allowlist 放行 / 报告与退出码）收在 `scripts/lib/guard.mjs`，守卫文件只留判定面与措辞（`GUARDED_*` 常量 + `scanSource` + `GUARD_SPEC`；新增守卫只需实现 `scanSource`）。`--diff` 是 **fail-closed** 的：base 解析不了、`git diff` 失败、新增文件读不出来都报错并非零退出，不会静默判绿（历史上 `--diff` 曾因实参形态错而恒 0 违规、CI 只跑 `--all` 无人接住）。runner 自带表驱动自测：`node --test scripts/guard.test.mjs`（合成 diff 正/负样本 + 纯删除 diff 的「跳过」判据 + ALLOWLIST 行号级放行的正/负样本与缺基线 fail-closed + 真实 git 的 `--diff` 回归锁 + fail-closed 面 + 三个守卫的 CLI 面）；CI 在 frontend-check 里除 `--all` 全量外，另有增量门按 `--diff origin/<默认分支>` 跑**前端侧那五个**守卫（含下面的消费面覆盖锁与 AI 助手发送编排守卫）——第十三波新增的两条（端点错误面 / 目录排序第二源）判的是后端文件，由 `backend-lint` 跑 `--all`，不进这条增量门
  - **ALLOWLIST 在 `--diff` 下是行号级的（#1123 收掉上一版的已知洞）**：例外文件先取**基线内容**（默认 `git show <base>:<path>`，经注入面 `io.readBaseline`）跑一次 `scanSource` 得「基线违规行号集合」，只放行落在集合里的行 —— 往存量例外文件里新增违规、或把违规挪到别的行号，增量门照报（此前整文件放行，等于给例外文件开了永久后门）；**基线取不到即非零退出**，不静默退回整文件放行。例外文件必须仍在判定面内（`isGuardedPath` 不再吞 allowlist，豁免的唯一宿主是 runner；`guard.test.mjs` 有契约锁）。`--all` 仍是整体放行，逐字不变。
- **契约消费面覆盖锁（第十一波 / ADR-0056 §11 / #1100）**：`node scripts/check-api-consumers.mjs --all`（本地可 `--diff origin/master` 只看新增行）—— `frontend/src/api/**` 里请求调用（`.get/.post/.put/.delete/.patch`，含嵌套泛型 `get<PagedResult<T>>`）的第一个实参路径字面量，归一后必须落在 `backend/internal/apitypes/domains.go` 的域端点集内（消费了未登记端点即红；路径由变量给出的**不判绿**，判「无法静态判定」）。自检：`node --test scripts/check-api-consumers.test.mjs`
  - 存量欠条逐条登记在脚本 `ALLOWLIST`（键 = `<仓库相对文件>::<METHOD> <归一模式>`，现状 **0 条** —— #1120 把 `valuation/admin.ts` 的 `createCrud` 动态拼装改成「显式资源 → 显式路径」的具名方法表后销清；历史留痕见下一行），**补一个销一个**：条目对应的端点一旦登记进域声明表，`--all` 会把它判红（销账信号），照脚本头部的「销账操作单」删行；条目挂在文件里已经没有的消费上，自测判红（不许有死条目）
  - **已销账 8 条（历史留痕）**：#1097 销 `inspection.ts` 的 `GET /admin/points/ledger`、`/admin/inspection/deleted-after-accepted`、`/admin/recruit/views`、`/admin/recruit/requests`（补齐 swagger 注解并在 `apitypes.Domains` 新建 inspection 域登记）；#1120 销 `valuation/admin.ts` 的 `GET {}` / `POST {}` / `PUT {}/{}` / `DELETE {}/{}`（`createCrud` 改具名方法表后路径全是字面量）—— 脚本头部保留同一段留痕。
  - CI：frontend-check 的 `--all` 全量 + 增量门（五个守卫按 `--diff origin/<默认分支>` 跑），自检在 el-controls-selftest job
- **AI 助手发送编排守卫（第十一波 / ADR-0056 §10 / #1104）**：`node scripts/check-ai-assistant-send.mjs --all`（本地可 `--diff origin/master`）—— `pages/ai-assistant/**` 不得再出现发送编排（`sendMessage` / `streamChat` 两个入口名字），一轮发送的编排与三种终态（done / error / aborted）只许在 `stores/aiAssistant.ts` 的 `send` / `finalizeTurn` 里；页面只提交「正文 + 差异参数」，失败/中断读 `lastTurnError` 渲染重试。自检：`node --test scripts/check-ai-assistant-send.test.mjs`（11 例，含 `--diff` fail-closed 与端到端判绿）
- **分页容器归位锁（第十三波 ADR-0060 决策 6 / 票 6）**：`src/api/__tests__/page.spec.ts`（随 `npm test` 跑，无需单独命令）。口径是 **vitest 内的源码扫描而不是 shell 脚本**，形态照 `src/utils/__tests__/statusWordsTemplate.spec.ts`：`pages/` 与 `components/` 的每个 `.vue` 取 `<script>` 段用 `typescript` 真解析成 AST，判 `useAdminTable` 的 `fetch` adapter **有没有返回带 `list:` / `items:` 键的对象字面量**——分页容器属于 api 侧（`api/page.ts` 的 `Page<T>` + 唯一构造器 `toPage`），页面 adapter 的合法形态只有 `return someApi.listX(...)`。按**形状**判红不按文件名登记，新页面无从绕开；存量断言是「当前命中集合为空」，没有白名单。为什么必须走 AST：页面里 `const { list: items } = useAdminTable(…)` 的解构改名长得一模一样，正则会把这十几处合法写法全部误判。防恒绿：同文件断言「AST 数到的 useAdminTable 实例数 == 正则数到的 == 带可解析 fetch 的实例数」（扫描面静默失灵即红）+ 合成违规片段必须判红（`list:` 老形态、换成 `items:` 的绕行形态、concise body、三元分支四种）与合法形态不得判红（直接 return api 容器 / 解构改名 / adapter 内拼参数没 return / `toPage(rows, rows.length)`）。
- 覆盖率：`npx vitest run --coverage`（istanbul provider，text + html 报告落 `coverage/`，不设阈值不挂门禁）；baseline（2026-09-10）：全站 lines 33.3%，admin 10.4% 为最大盲区

## 部署配置

改 `docker-compose*.yml` / `deploy.sh` 后可用 `docker compose -f docker-compose.prod.yml config -q` 做语法校验。

**迁移失败即中止部署**（第十一波 / #1099）：`scripts/deploy-remote.sh` 的 `run_migration` 失败不再降级为 `log_warn` 后继续，而是 `return 1`、调用点 `exit 1`（默认路径必须中止）。线上事故确需带「新代码跑在旧 schema」这个已知风险继续时，显式声明 `ALLOW_MIGRATION_FAILURE=1`（变量名与理由写在脚本「迁移」配置段，判定点只有一处：`migration_failure_action`）；未显式声明一律中止，不得靠改日志绕过。

两条门的语义不同，别混用（2026-09-17 澄清；**没有**「唯一逃生开关」这回事）：

- `SKIP_MIGRATION=true`（CD 的 workflow_dispatch 输入）：**迁移前一整段跳过** `run_migration` —— 迁移根本不跑，也就不存在失败与否；用于已知无需迁移的场景（如仅回滚版本）。
- `ALLOW_MIGRATION_FAILURE=1`：迁移**照跑**，失败后带「新代码跑在旧 schema」的已知风险继续部署；默认（未声明）= 失败即中止。

两者的判定都是显式声明，默认路径一律 fail-closed。常驻判据物：`node --test scripts/deploy-migration-gate.test.mjs`（dry-run 入口 `bash scripts/deploy-remote.sh --migration-gate`，断言「未声明 ⇒ abort / =1 ⇒ continue」，不跑 docker）。

CI 的 `migration-check`（`ci.yml` Stage 6）在服务容器 `postgres:15-alpine` 上空库真跑三步，共用 job 级 `DATABASE_URL`：

1. `go run ./cmd/migrate up` —— 空库重建 baseline（全部 up 迁移真执行一次）。
2. `go run ./cmd/migrate check-columns` —— **单向列对账**：GORM 模型期望的列集合必须 ⊆ `information_schema` 实际列集合，**缺列/缺表即非零退出并逐条打印「表.列」**；实际库多出来的列（迁移里有、模型刻意不映射）不算错（不做反向对账）。实现：`backend/internal/migrate/columns.go`（子命令复用 `cmd/migrate` 既有的 direction 分派，不改 CLI 入口）。
3. `go run ./cmd/migrate down` —— 真跑回滚到空库，随后用 `check-columns` 做**反向断言**：它必须报红且点名 `hrwai_users` / `question` / `credential`（回滚不干净即红）。

口径澄清：`migrate up` 此前并非从没在 CI 跑过——`backend-test` 已注入 `DATABASE_URL`，`internal/testutil/pg.go` 的 `NewPostgresDB` 会为每个 Postgres 契约测试在独立 schema 上真跑迁移（各用例 `DROP SCHEMA CASCADE` 清理）。`migration-check` 补的是**空库 baseline + 列对账 + down 回滚**这三件此前没有的事。本地无 Postgres/Docker 时，对账口径的回归跑 `go test ./internal/migrate/`（含缺列/多列正负样本）；真实迁移链路只能在 CI 上验。

**反代到后端的每个 `location` 必须显式设置 `X-Forwarded-For`**（`frontend/nginx-host.conf`、`frontend/nginx.default.conf`）：nginx 只在设置时才覆写/追加该头，没设置的 location 会把客户端自带的同名头原样透传；后端信任本机对端（`TRUSTED_PROXIES`）之后会采信那个伪造值——限流键可被轮换、访问日志与审计日志写入假 IP。新增或改动反代 location 时逐条核对（#888 的 `/static/` 就是漏网的那条）。

## 安全检测

改动触及认证/授权/密钥/DB 连接/AI 生成代码时，跑 `python -m deepsec shield scan backend frontend/src`，确认无新增 critical/high（已知误报见 `docs/agents/security-scan.md`）。
