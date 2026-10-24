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
- 端点错误面守卫（第十三波 ADR-0060 决策 1 / 票 1b）：`node scripts/check-render-error-face.mjs --all`（本地可 `--diff origin/master` 只看新增行）—— `RenderFunc` 已不含 `err` 参数，错误面归端点骨架无条件渲染，所以 HTTP 面文件（`internal/api` 整目录，**加**任意域包的 `handler*.go`，ADR-0070 决策 5——估值域出口自 #1514 波 9 起也叫 `handler*.go`，那条曾为它单独开的整目录判据已作为死条目删除）的 `Render` 闭包内不得再出现 `response.ServerError/BadRequest/NotFound/Unauthorized/Forbidden`、`renderStatus(` 或 `xxxErrStatus.renderError(`；状态码与固定文案写进本端点的 `ErrStatus`（单码用 `httpx.ErrStatusAll` / `httpx.ErrStatusAllMsg`，哨兵用 `httpx.ErrStatusEntry`，`Sentinel == nil` 表示无条件命中且抢在 `*ParseError` 规则之前；骨架与这些表类型自 #1445 P0-B 起住在 `backend/pkg/httpx`）。需要非信封错误形状的面走 raw handler（不留逃生槽）。域包的 HTTP 出口**必须**命名为 `handler*.go`（`GUARDED_FILE_PREFIXES=['handler']` + `GUARDED_FILE_ROOT='backend/internal/'`）——换个名字守卫就看不见（静默）。自检：`node --test scripts/check-render-error-face.test.mjs`；CI 在 backend-lint 跑 `--all`、判定逻辑自检在 el-controls-selftest
- 改了 handler 的 swagger 注解（`@Success` / `@Param` / `@Router` 等）或**任何进出响应 DTO 的字段**（含可空性）后：`cd backend && make swagger` 再生成 `backend/docs/{docs.go,swagger.json,swagger.yaml}`，**再** `go run ./cmd/gen-apitypes` 再生成 `frontend/src/api/generated/*.ts`，两者与改动一并提交 —— CI 的 backend-lint 有**新鲜度锁**（按钉住的 swag 版本再生成后要求工作树干净），生成物过期直接红。
  ⚠️ **顺序是硬的**：`internal/apitypes/codegen_test.go` 把前端生成文件与 Go 注解渲染结果全等比对，所以「先跑 `go test ./...` 再 `make swagger`」会先给你一个绿、几秒后变红（2026-09-20 #1197 实测踩到：全量套件通过后再生成 swagger，`apitypes` 随即判红）。收口顺序固定为 gofmt/vet → 再生成（swagger → gen-apitypes）→ `go test ./...` → 前端检查。
- 改过**响应字段的两个 struct tag** 时（`nullability:` 可空性表态 / `fact:` 消费点对齐，词汇见 `CONTEXT.md`「接口契约」）：这两把锁都按**生成物**读射程，所以顺序同上（先 `make swagger` 再跑测试）。它们管的东西不同，别混成一条：
  - `nullability:"nullable"` 要在 `internal/core/nullable_declaration_test.go` 的 `nullableOutlets*` 表里举出一个真 marshal 出 `null` 的出口；`nonnil` 要在 `nonnilOutlets*`（`internal/core`、`internal/api` 或**域包**那张——按**组装发生在哪一层**选，ADR-0070 决策 9）里举出一个真发 `[]` / `{}` 的出口；同一把钥匙跨包只许有一个举证地（锁里有 owner 表逐键认领）。锁在 `internal/apitypes/nullability_lock_test.go`。**这一张是欠账表**：未举证的数量钉成常量（判据 3/4），只准减，涨与降都要人改常量并同步注释里的算式。
  - `fact:"<key>"` 要登记进 `internal/api/consumption_fact_registry.go`，且必须**同时**有 ≥1 个明文面载体与 ≥1 个投影位（单侧谈不上对齐）。锁分两处：`internal/api/consumption_fact_lock_test.go`（表自洽 + 载体必须是 `internal/core` 里一枚具名 `errors.New`/`fmt.Errorf` 哨兵并**接进某个端点的错误面** + 表⇔tag 双向逐字相等且各投影位同名 + 契约描述含那句错误）与 `internal/apitypes/fact_reachability_lock_test.go`（投影位所在类型必须在 2xx 闭包里）。**这一张不是欠账表**——加行是目的，不是债务，所以没有常量可减。两处各有一条「扫描面为空即 Fatal」与「论域前缀没扫到即红」的防空转半边，别把它们当装饰删掉。
  ⚠️ `fact:` 还剩一条机器管不到，必须靠评审补：**登记表认领某个 error 作「明文面载体」时，要另写一条 HTTP 级断言钉那句话真发得出去**。锁 today 会推到「这枚哨兵确实被接进某个端点的错误面」（出现在某处 `errors.Is` 实参或某张 `errStatusTable` 里），但**接线不等于外发**——`response.Forbidden(c, "固定文案")` 这种写法下哨兵仍被引用、句子却不出门。那一句归 HTTP 断言（先例：`contact_company_disable_contract_test.go` 里那句 `403 的正文要带那件事实的具名句子`）。
- 注释三态守卫（issue #1445 的 P4 / 标准见 [`docs/agents/comments.md`](comments.md)）：`node scripts/check-comment-cleanliness.mjs --all`（本地可 `--diff origin/master` 只看新增行）—— 锁三类**机检得出、且与代码事实矛盾**的注释欠账：① 失真包文档（文件头 `// Package <名>` 与文件真实 `package` 子句不符；#1445 P3 更名后曾有 42 处）；② 装饰分隔线（整行 8 个以上同一符号）；③ 相邻重复注释行。基线 0、**只减不增**；「注释率」与「注释掉的代码」两类**刻意不锁**（理由见标准第 5 节）。判定面自检：`node --test scripts/check-comment-cleanliness.test.mjs`
- 目录排序串第二源守卫（第十三波 ADR-0060 决策 10 / 票 10）：`node scripts/check-catalog-sort.mjs --all`（本地可 `--diff origin/master` 只看新增行）—— 目录读面（文件名含 `catalog` 的非测试 `.go`）不得再出现与 catalog descriptor `OrderBy` 声明**逐字同串**的裸排序串，排序串的唯一宿主是 spec 表（`catalog_specs.go` / `position_catalog.go`），读面写 `q.Order(specialtyCatalogSpec().OrderBy)` 这类引用。判据窄是刻意的：带表别名的课程行 `course.sort_order ASC, course.course_id ASC` 与章节行 ADR 明记不动（收进射程就得给整个读面文件开 ALLOWLIST，`--all` 的例外是整文件放行，会连带盲掉真危险行）。自检：`node --test scripts/check-catalog-sort.test.mjs`（含「声明表与 spec 表互等」一致性锁与合成违规目录必须判红的防恒绿用例）；CI 在 backend-lint 跑 `--all`、判定逻辑自检在 el-controls-selftest
- 后端三条结构规矩（ADR-0070 决策 1 / 5 / 6，住 `backend/internal/layers`，随 `go test` 跑）：
  1. **单向依赖**：`pkg/httpx` 不得 import 任何 `internal/...`；`internal/api`（装配根）只允许 `cmd/...` 依赖它。
  2. **测试同居**：每个 `_test.go` 必须与同目录的非测试 `.go` 同住；不得有 `test/` / `tests/` 目录。
  3. **gin 只许出现在 HTTP 面与登记的 HTTP 基建里**：HTTP 面 = `internal/api` 整目录**或任意域包的 `handler*.go`**（判定面的唯一出处是 `testutil.HTTPSurface`，`internal/layers` 按参数注入同一判据）；基建白名单 = `internal/middleware` / `internal/logger` / `pkg/httpx` / `pkg/response`。域实现（`service.go` / `dto.go`）要的是解析好的入参，不是 `*gin.Context`。
  判据只吃路径与源码，各带防空转（import 边 < 500 或 gin 宿主 < 5 即 Fatal）与合成违规自测（`TestLayerGuardDetectsPlantedViolations`）；gin 白名单每条目录必须真有宿主，否则算死条目报红。**搬家前先读 `docs/agents/domain-package-migration.md` 的「必须同步的锁与清单」**——改名会让按名字/路径判定的锁**静默失配**（清单名、白名单键、前缀计数三处都要 grep 一遍）

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
- `npm test`（vitest 5；CLI 的两个口径变化见本段末「vitest 4 → 5」）
- 新建 spec 一律用 `epLite()`（`src/test/element-lite.ts`）按需注册 EP 组件，**禁止全量挂载 `plugins: [ElementPlus]`** —— 全量挂载是 CourseCatalog flaky（CI 2 核下 import 争抢超时）的根因；组件清单可用 `node scripts/scan-el-components.mjs` 扫描
- 已收敛控件守卫：`node scripts/check-el-controls.mjs --all`（CI 在 frontend-check 里跑全量；本地也可 `--diff origin/master` 只看新增行）。守卫判定逻辑的自检：`node --test scripts/check-el-controls.test.mjs`
- api seam 守卫（ADR-0053 §7）：`node scripts/check-api-seam.mjs --all`（页面与业务组件不得直接引用 `@/api/request` / `@/api/client`；同样在 frontend-check 跑全量，本地可 `--diff origin/master`）。自检：`node --test scripts/check-api-seam.test.mjs`。逐条登记的例外写在脚本的 `ALLOWLIST`（每条带理由）
- **守卫 runner 单点（第十一波）**：**九个**守卫（api-seam / el-controls / async-section / 契约消费面覆盖锁 / AI 助手发送编排 / 无障碍档位 / 端点错误面 / 目录排序第二源 / 注释三态）的 runner 面（argv 解析 / 走查 / `--diff` 取 diff 文本 / allowlist 放行 / 报告与退出码）收在 `scripts/lib/guard.mjs`，守卫文件只留判定面与措辞（`GUARDED_*` 常量 + `scanSource` + `GUARD_SPEC`；新增守卫只需实现 `scanSource`）。`--diff` 是 **fail-closed** 的：base 解析不了、`git diff` 失败、新增文件读不出来都报错并非零退出，不会静默判绿（历史上 `--diff` 曾因实参形态错而恒 0 违规、CI 只跑 `--all` 无人接住）。runner 自带表驱动自测：`node --test scripts/guard.test.mjs`（合成 diff 正/负样本 + 纯删除 diff 的「跳过」判据 + ALLOWLIST 行号级放行的正/负样本与缺基线 fail-closed + 真实 git 的 `--diff` 回归锁 + fail-closed 面 + 三个守卫的 CLI 面）；CI 在 frontend-check 里除 `--all` 全量外，另有增量门按 `--diff origin/<默认分支>` 跑**前端侧那六个**守卫（含下面的消费面覆盖锁与 AI 助手发送编排守卫）——第十三波新增的两条（端点错误面 / 目录排序第二源）判的是后端文件，由 `backend-lint` 跑 `--all`，不进这条增量门
  - **ALLOWLIST 在 `--diff` 下是行号级的（#1123 收掉上一版的已知洞）**：例外文件先取**基线内容**（默认 `git show <base>:<path>`，经注入面 `io.readBaseline`）跑一次 `scanSource` 得「基线违规行号集合」，只放行落在集合里的行 —— 往存量例外文件里新增违规、或把违规挪到别的行号，增量门照报（此前整文件放行，等于给例外文件开了永久后门）；**基线取不到即非零退出**，不静默退回整文件放行。例外文件必须仍在判定面内（`isGuardedPath` 不再吞 allowlist，豁免的唯一宿主是 runner；`guard.test.mjs` 有契约锁）。`--all` 仍是整体放行，逐字不变。
- **契约消费面覆盖锁（第十一波 / ADR-0056 §11 / #1100）**：`node scripts/check-api-consumers.mjs --all`（本地可 `--diff origin/master` 只看新增行）—— `frontend/src/api/**` 里请求调用（`.get/.post/.put/.delete/.patch`，含嵌套泛型 `get<PagedResult<T>>`）的第一个实参路径字面量，归一后必须落在 `backend/internal/apitypes/domains.go` 的域端点集内（消费了未登记端点即红；路径由变量给出的**不判绿**，判「无法静态判定」）。自检：`node --test scripts/check-api-consumers.test.mjs`
  - 存量欠条逐条登记在脚本 `ALLOWLIST`（键 = `<仓库相对文件>::<METHOD> <归一模式>`，现状 **0 条** —— #1120 把 `valuation/admin.ts` 的 `createCrud` 动态拼装改成「显式资源 → 显式路径」的具名方法表后销清；历史留痕见下一行），**补一个销一个**：条目对应的端点一旦登记进域声明表，`--all` 会把它判红（销账信号），照脚本头部的「销账操作单」删行；条目挂在文件里已经没有的消费上，自测判红（不许有死条目）
  - **已销账 8 条（历史留痕）**：#1097 销 `inspection.ts` 的 `GET /admin/points/ledger`、`/admin/inspection/deleted-after-accepted`、`/admin/recruit/views`、`/admin/recruit/requests`（补齐 swagger 注解并在 `apitypes.Domains` 新建 inspection 域登记）；#1120 销 `valuation/admin.ts` 的 `GET {}` / `POST {}` / `PUT {}/{}` / `DELETE {}/{}`（`createCrud` 改具名方法表后路径全是字面量）—— 脚本头部保留同一段留痕。
  - CI：frontend-check 的 `--all` 全量 + 增量门（六个守卫按 `--diff origin/<默认分支>` 跑），自检在 el-controls-selftest job
- **无障碍档位守卫（#1892）**：`node scripts/check-a11y.mjs --all`（本地可 `--diff origin/master`）—— `.vue` 里不得再出现 `text-ink-muted` / `var(--color-text-muted)`（浅色 2.45:1、深色 3.07:1，当正文用读不清）与「`bg-ui-500` 同行配 `text-white` / `text-panel`」（白字 3.74:1，填充抬到 `bg-ui-600` = 5.47）。**token 对比度矩阵锁在 `src/assets/styles/__tests__/contrast.spec.ts`**（随 `npm test` 跑：正文/次要/三级字 × 页面底/卡片底、主按钮白字、品牌浅底深字、三组语义深字配浅底，外加 `--gradient-brand` 两端白字 ≥4.5）。防空转下界 `minChecked: 120`（当前 169 个 `.vue`）：判定面前缀写错时判红，不会静默判绿。自检：`node --test scripts/check-a11y.test.mjs`（13 例，含变体前缀、注释负例与 CLI 全量防假红）
- **AI 助手发送编排守卫（第十一波 / ADR-0056 §10 / #1104）**：`node scripts/check-ai-assistant-send.mjs --all`（本地可 `--diff origin/master`）—— `pages/ai-assistant/**` 不得再出现发送编排（`sendMessage` / `streamChat` 两个入口名字），一轮发送的编排与三种终态（done / error / aborted）只许在 `stores/aiAssistant.ts` 的 `send` / `finalizeTurn` 里；页面只提交「正文 + 差异参数」，失败/中断读 `lastTurnError` 渲染重试。自检：`node --test scripts/check-ai-assistant-send.test.mjs`（11 例，含 `--diff` fail-closed 与端到端判绿）
- **分页容器归位锁（第十三波 ADR-0060 决策 6 / 票 6）**：`src/api/__tests__/page.spec.ts`（随 `npm test` 跑，无需单独命令）。口径是 **vitest 内的源码扫描而不是 shell 脚本**，形态照 `src/utils/__tests__/statusWordsTemplate.spec.ts`：`pages/` 与 `components/` 的每个 `.vue` 取 `<script>` 段用 `typescript` 真解析成 AST，判 `useAdminTable` 的 `fetch` adapter **有没有返回带 `list:` / `items:` 键的对象字面量**——分页容器属于 api 侧（`api/page.ts` 的 `Page<T>` + 唯一构造器 `toPage`），页面 adapter 的合法形态只有 `return someApi.listX(...)`。按**形状**判红不按文件名登记，新页面无从绕开；存量断言是「当前命中集合为空」，没有白名单。为什么必须走 AST：页面里 `const { list: items } = useAdminTable(…)` 的解构改名长得一模一样，正则会把这十几处合法写法全部误判。防恒绿：同文件断言「AST 数到的 useAdminTable 实例数 == 正则数到的 == 带可解析 fetch 的实例数」（扫描面静默失灵即红）+ 合成违规片段必须判红（`list:` 老形态、换成 `items:` 的绕行形态、concise body、三元分支四种）与合法形态不得判红（直接 return api 容器 / 解构改名 / adapter 内拼参数没 return / `toPage(rows, rows.length)`）。
- 覆盖率：`npx vitest run --coverage`（istanbul provider，text + html 报告落 `coverage/`，不设阈值不挂门禁）；baseline（2026-10-23）：**279 个文件**、全站 lines **52.87%**，`src/pages/admin/` **31.3%** 仍是最大盲区（2026-09-10 那次是 33.3% / 10.4%，涨的是测试量不是口径）
- **vitest 4 → 5（#1788）三条会影响写测试的口径**：
  - **`clearMocks` 默认变 true**：每个用例前 `vi.clearAllMocks()`（实现留着，只清调用记录）⇒ 「跨用例累计调用次数」的写法不再成立。本仓 149 个文件按新默认全绿，故未显式回填 `clearMocks: false`；新写用例若需要跨用例累计，自己显式声明。
  - **`-t` 匹配的是 ` > ` 拼起来的全名**（此前用空格拼）：只跨单个分段边界的片段会失配——`-t "登录 守卫"` 变 `-t "登录 > 守卫"`，或干脆只写单段 `-t 守卫`。
  - **`vi.mock` / `vi.unmock` / `vi.hoisted` 写在函数或 `describe` 里由「告警」升为「抛错」**（它们本来就会被提升到模块顶层）。本仓现状 0 处嵌套，守住即可。
  - 覆盖率插件把第三方 istanbul 包换成了自研 fork（`istanbul-lib-*` / `istanbul-reports` / `@istanbuljs/schema` → `@vitest/istanbul-lib-*`，少 21 个包）；**`@babel/core` 仍在**，只是改由 fork 依赖并随之从 7.x 升到 8.0.7（插桩器，与业务产物无关）。另一处口径变化：`coverage.include/exclude` 改为按「相对项目根的路径」匹配（去掉 picomatch 的 `contains`）—— 同一份源码两版实测报出**同样的 279 个文件**，口径未漂。

## 部署配置

改 `docker-compose*.yml` / `deploy.sh` 后可用 `docker compose -f docker-compose.prod.yml config -q` 做语法校验。

**迁移失败即中止部署**（第十一波 / #1099）：`scripts/deploy-remote.sh` 的 `run_migration` 失败不再降级为 `log_warn` 后继续，而是 `return 1`、调用点 `exit 1`（默认路径必须中止）。线上事故确需带「新代码跑在旧 schema」这个已知风险继续时，显式声明 `ALLOW_MIGRATION_FAILURE=1`（变量名与理由写在脚本「迁移」配置段，判定点只有一处：`migration_failure_action`）；未显式声明一律中止，不得靠改日志绕过。

两条门的语义不同，别混用（2026-09-17 澄清；**没有**「唯一逃生开关」这回事）：

- `SKIP_MIGRATION=true`（CD 的 workflow_dispatch 输入）：**迁移前一整段跳过** `run_migration` —— 迁移根本不跑，也就不存在失败与否；用于已知无需迁移的场景（如仅回滚版本）。
- `ALLOW_MIGRATION_FAILURE=1`：迁移**照跑**，失败后带「新代码跑在旧 schema」的已知风险继续部署；默认（未声明）= 失败即中止。

两者的判定都是显式声明，默认路径一律 fail-closed。常驻判据物：`node --test scripts/deploy-migration-gate.test.mjs`（dry-run 入口 `bash scripts/deploy-remote.sh --migration-gate`，断言「未声明 ⇒ abort / =1 ⇒ continue」，不跑 docker）。

CI 的 `migration-check`（`ci.yml` Stage 6）在服务容器 `postgres:15-alpine` 上空库真跑三步，共用 job 级 `DATABASE_URL`：

1. `go run ./cmd/migrate up` —— 空库重建 baseline（全部 up 迁移真执行一次）。
2. `go run ./cmd/migrate check-columns` —— 自 #1362 起是**三段**判据（实现：`backend/internal/migrate/columns.go`；子命令复用 `cmd/migrate` 既有的 direction 分派，不改 CLI 入口）：
   - **列对账（语义逐字未变，仍是单向）**：GORM 模型期望的列集合必须 ⊆ `information_schema` 实际列集合，**缺列/缺表即非零退出并逐条打印「表.列」**；实际库多出来的列（迁移里有、模型刻意不映射）不算错（**不做反向列对账**）。
   - **唯一索引登记表 ⇔ migrations 逐字相等**（#1362 新增，**不连库**，本机 `go test ./internal/migrate/` 就能红）：`internal/migrate` 那张关键偏唯一索引登记表的每条 DDL，必须与 migrations 里的原句逐字一致、且谁也不多谁也不少。
   - **唯一索引存在性（单向）**：登记表里的索引必须真在 `pg_indexes` 里（按 `current_schema()` 收窄）；库里多出来的索引不算错。
3. `go run ./cmd/migrate down` —— 真跑回滚到空库，随后用 `check-columns` 做**反向断言**：它必须报红且点名 `hrwai_users` / `question` / `credential`（回滚不干净即红）。

口径澄清：`migrate up` 此前并非从没在 CI 跑过——`backend-test` 已注入 `DATABASE_URL`，`internal/testutil/pg.go` 的 `NewPostgresDB` 会为每个 Postgres 契约测试在独立 schema 上真跑迁移（各用例 `DROP SCHEMA CASCADE` 清理）。`migration-check` 补的是**空库 baseline + 列与唯一索引对账 + down 回滚**这三件此前没有的事。本地无 Postgres/Docker 时，对账口径的回归跑 `go test ./internal/migrate/`（含缺列/多列正负样本、登记表 ⇔ migrations 逐字相等、以及「删掉任一索引必红」的合成破坏样本 —— 这三段都**不连库**，是本机唯一能证伪那半张票的地方）；真实迁移链路只能在 CI 上验。

**反代到后端的每个 `location` 必须显式设置 `X-Forwarded-For`**（`frontend/nginx-host.conf`、`frontend/nginx.default.conf`）：nginx 只在设置时才覆写/追加该头，没设置的 location 会把客户端自带的同名头原样透传；后端信任本机对端（`TRUSTED_PROXIES`）之后会采信那个伪造值——限流键可被轮换、访问日志与审计日志写入假 IP。新增或改动反代 location 时逐条核对（#888 的 `/static/` 就是漏网的那条）。

## 安全检测

改动触及认证/授权/密钥/DB 连接/AI 生成代码时，跑 `python -m deepsec shield scan backend frontend/src`，确认无新增 critical/high（已知误报见 `docs/agents/security-scan.md`）。

### 依赖告警面：Dependabot alerts ≠ 可触达漏洞（2026-10-24 清理，#1873）

- **「有没有漏洞」以 `govulncheck ./...` 为准**（CI 的 `security-scan` 每次跑同一条命令），不以安全页上的告警数。
- 2026-10-24 清过一批：21 条 open 告警（17 × `x/crypto` / 3 × `x/net` / 1 × `jwt/v5`）的 `dependency.manifest_path` **全是 `backend-go/go.mod`** —— 该路径在 monorepo 合并后已不存在（现为 `backend/go.mod`），是 GitHub 依赖图里留着的**已删除清单快照**（`/dependency-graph/sbom` 对同样三个包各返回新旧两条版本）。实际版本 `x/crypto v0.58.0` > 0.52.0、`x/net v0.60.0` > 0.55.0、`jwt/v5 v5.3.1` > 5.2.2，**告警范围早就越过了**；21 条已按 `inaccurate` 逐条 dismiss（评论里写了同样的话）。
- **再冒出来先看 `manifest_path`**：`backend-go/go.mod` ⇒ 照上面 dismiss，别去升 `go.mod`（升了也不会关）；若是 `backend/go.mod` ⇒ 那是真问题，按常规升级处理。
- 依赖图快照没有公开接口能清，只能等 GitHub 侧重算 —— 这不是可修的代码问题。

## CI 触发模型与 `backend-test` 分片（2026-10-24，#1856 结论 / #1862）

- **`changes` 的 paths-filter 默认基准就是 `default_branch` 的 merge-base**，不是 `github.event.before`。`dorny/paths-filter@v4` 在 push 事件里只有当 `base` 传的是 **commit SHA** 或**与被推分支同名**时才会退到 `before`（`src/main.ts` 的 `isBaseSha || isBaseSameAsHead` 分支），否则一律 `git diff master...HEAD`（三点比较）。⇒ 「把 master 合进功能分支」**本来就不会**把 master 的改动算进差异；**别**给这个 step 传 `base:` —— 传分支名是 no-op（那正是默认值），传 `origin/master` 还会硬失败 `couldn't find remote ref`（它把 `base` 当远端分支名去 `git fetch origin <base>`）。
- **改 `.github/workflows/**` 必然三条腿全跑**（`backend` / `frontend` / `mobile` 三个 filter 都含该模式；有意的，工作流坏了影响所有 job）。判断 filter 是否正常，看 `changes` job 日志里的 `Detected N changed files` —— 它应当等于**本分支自己的**改动数，而不是 master 合并进来的那些。
- **`backend-test` 按包分两片**（LPT 装箱）：权重表 `scripts/go-package-timings.json`（表里没有的包按 `defaultSeconds` 计权，照样会被排进某一片，不会丢）、切片器 `scripts/go-shard.mjs`（本片为空 ⇒ **退出码 3**，job 直接红）。分片下限由**最慢的包**决定（`internal/api` 400.2s / 全量 948.1s）⇒ **k=2 已到顶**，再多切只多烧 runner；要再快只能拆这个包。改权重表不是「调参」而是**门禁参数**：表坏了只让负载不均、不会漏测（两片并集恒等于 `go list ./...`，`scripts/go-shard.test.mjs` 钉死）。
- `backend-test` 与 `backend-lint` **并行**（两者无产物依赖）：省 140s 墙钟，代价是 lint 红时不再抢在 10 分钟测试之前报错。

## 运行时面判定与证据校验的机器语义（2026-10-15 从根 `AGENTS.md` 迁入，判据源 `.github/scripts/lib/gate-validator.mjs`）

**执行面是仓库级**：`.github/workflows/pr-evidence.yml` 对**每一个 PR** 运行，但**只对命中运行时面**（`*.uvue` / `*.uts` / training-app 下的三份 json）**的 PR 校验正文的 `## 验收证据` 段** —— 未命中时校验器在校验段**之前**就放行（`runtime.length === 0` 早退，用例见 `.github/scripts/pr-evidence-check.test.mjs`「非运行时面 PR：即使正文为空也通过」），命中而缺段则直接判红；未命中运行时面时仍**约定**写 `免（未命中运行时面）`（先例 #908）。适用范围不限于移动端。

**候选来源（2026-10-23，#1841 起）**：PR 评论 + commit 评论 + **PR 正文关联的票评论**，三来源**同构**——marker（`<!-- gate-evidence:<门号> -->`）、`commit: <sha>` 绑定、祖先判定与「其间运行时面未改动」、fail-closed（compare 拿不到即不满足）**全部沿用，一条不放宽**。哪些票算候选由判据源 `issueNumbersInBody` 定：**先剥 HTML 注释**（模板注释里堆着历史票号，不剥会被全拉成候选），再取 `Closes` / `Refs` / `Fixes` / `Resolves` / `Related to #N` 与 `https://github.com/<o>/<r>/issues/<n>`，去重后**最多 5 张**；**未在正文关联的票不进判据**。多出的这一处来源是**降级**不是放宽：GitHub 对本仓 `POST /repos/<owner>/<repo>/commits/<sha>/comments` 长期回 HTTP 404（同 token 在别的仓正常，复盘见 [#1838](https://github.com/DriftingLi/FL/issues/1838)），门脚本遇 404 把**同一份**评论（marker 与 `commit: <sha>` 一字不改）改贴到票当正式落点，并打印机检行 `GATE_COMMENT_DEGRADED tier=issue url=<URL>`；**触发条件只有 404**，其余失败仍是硬失败（`GATE_COMMENT_FAILED`），不得静默 skip。

**开 PR 的时机（2026-09-15 裁定）**：一个 PR = **一件做完且已在目标环境验证过的事** —— 先把事做完、验完，**再**开 PR；不得边做边开、不得把 PR 当进度容器，也不得把「未命中运行时面 ⇒ 免四门」当成「不需要验证」（工具链 / 脚本类改动的验收判据是「**它要达成的那个行为在真实链路上成立**」，例如改截图判据就得跑一次真机 🟡 并留下产物）。完成定义（DoD）、例外通道与反例见 `docs/agents/release.md`。

**① 行免行白名单（2026-10-24 起，ADR-0043（移动端）决策 1/8，判据票 #1859）**：运行时面 PR 的 ① 行固定写 `免（①a 降级发版前一次，ADR-0043）`——①a 真机逐页取证降级为**发版前一次**（发版冒烟）。判据**只认字样不认文件**（`免` 开头 + 含「①a 降级发版前一次」+ 含「ADR-0043」，判据包与文档包谁先合入都不悬空）；跳过的是「取证」不是「签收」——执行人非占位、日期 YYYY-MM-DD 仍是硬判据，复测对象可写「无」。旧窗口写法 `SKIP·待批收口` **过渡期双白名单暂认**，退场判据 = #1859 与文档包 #1858 都合入后，**第一个用新写法完整过门的运行时面 PR** 出现 ⇒ 立小票删旧通道。

**「低风险运行时面」豁免已整体退役**（2026-10-24 起，ADR-0043（移动端）决策 8，判据票 #1859）：判据源已删 `isLowRiskUtChange` / `LOW_RISK_NON_RUNTIME` 与输出字段 `lowRiskRuntime`——纯 `.uts` 不再免 ①（改走上面的 ①a 免行），也不再因「低风险」免 ②（② 触发面命中即必跑；纯 `.uts` 增删 MP-WEIXIN 条件编译指令行同样点亮 ②）。当年的历史经验仍有效：它曾是**逐文件白名单**（改动集里每一个文件都得落在 `*.uts` / `*test.js` / `*.md` / `jest.config*.js` 内），且「拿不到豁免」与「①② 变必过」**从来是两个不同源的过滤器**——`.ps1` 等文件不命中运行时面判定（`runtime.length === 0` ⇒ 早退放行），①② 不会因此变必过；真正会多跑的是 **CI 的 `mobile-test`（＝ ③）**（`ci.yml` 的 `changes.mobile = 'training-app/**'`，该目录下任何文件都命中）。⇒ 把工具改动**拆成独立 PR** 仍是正解，代价是「多跑一次 ③」，**不是**「①② 双双变必过」。

**② 的「机器必检面」与「执行纪律面」是两件事**（2026-10-20 补；2026-10-24 随 ADR-0043 改写）：`pr-evidence` 的 `needs2` 只认 **MP-WEIXIN 面**（条件编译指令行 / `manifest.json` / `platformConfig.json`）——纯 `.uvue` 改动在校验器里判 `②` **不必强制出现**；而 0043 决策 3 把「每个运行时面 PR 都执行 ②」从窗口口径升为**常态主门**（它是唯一能看见小程序渲染差异的门）。两者各判各的：`needs2` 决定**证据段里 ② 行是否强制**，常态纪律决定**要不要真跑**。⇒ 纯 `.uvue` 的 PR 仍要跑 ②，只是正文可以按模板写 `免`；别把校验器的「免」当成不必执行。触发判据只认三份 json 与 MP-WEIXIN 条件编译、纯 `.uvue` / `.uts` 无 ② 机检兜底——该脆弱性随 0043 永久化（纪律兜底）。

## 成本量尺：实付 token（2026-10-17 立，#1705）

**为什么换尺子**：上下文 / 请求数会奖励「多塞 + 常重启」，而真金白银是**未命中缓存的输入**。旧叙事「token/次 −35%」既没测到、方向上还会被这套激励反向拉扯（#1694 如实记录未达）。故本仓成本口径一律改用实付。

**定义与复算**（源 `~/.local/state/billion-context/bili.log` 的 `[acp-usage] input=… cached=…` 行，**单位是 token**）：

- 单次请求 **实付 = input − cached**；run 的 **实付 = Σ(input − cached)**，**实付/请求 = 实付 ÷ 请求数**。
- 双指标**必须成对报**：`实付/请求` + `命中占比 = Σcached ÷ Σinput`（另附 `Σinput` 总量）。只报 input 会掩盖缓存水位，只报实付会掩盖总量放大——两种都是单指标骗人。
- 台账每个 run 一行，字段固定为：`run`（session 前 8 位）/ `cwd` / `title`（含票号）/ 首次～末次时间 / `请求数` / `Σinput` / `Σcached` / `实付` / `实付/请求` / `首帧 input` / `模型` / `上游` / `折叠次数`。

**窗口约定**：run 长度实测 1～607 请求，整跑数字跨 run 不可比 ⇒ 比较**一律取每个 run 的前 37 次请求**（与基线同窗）；不足 37 的短 run 单列、不参与跨 run 比较。run 身份不靠猜：`bili.log` 的 `sess=session-<uuid>` ↔ `sessions/<uuid>.json` 的 `record.rows.cwd.val` 与 `title.val`（票号在 title 里）。

**基线**：**8,773 实付/请求**（#1694 那次 run 的前 37 请求窗口，`Σinput 2,164,469` / `Σcached 1,839,872`，命中 85.00%）。⚠️ 现 log 最早一条是 2026-10-16T06:44Z，log 已被轮转 / 截断 ⇒ 该基线**无法在现 log 上重算**，只能引用票面原值并复算其算式（324,597 ÷ 37 = 8,773）；引用时写明「基线来自 #1694 票面，不可复算」。

**窗口复算示例**（`bb3134bc`，#1666）：前 37 次 `Σinput 2,667,271 − Σcached 2,570,240 = 97,031`，`97,031 ÷ 37 = 2,622` 实付/请求。

**一对前后 run 的对比**（取数时刻 2026-10-17T14:10Z，log 35,244 行：30 个有 usage 的 run / 2,527 请求 / `Σinput 243,163,132` / 实付 26,152,316 / 命中 89.24%；**数字随在跑 run 继续变**，引用须同时写取数时刻）：

| 对照 | before → after | 前 37 实付/请求 | 首帧 input | 模型 / 上游 |
| --- | --- | --- | --- | --- |
| 同票系（降本系列）#1694 → #1705 | `4db99585` → `d3ae9530` | 19,620 → 3,829（−80.5%） | 116,519 → 41,570 | 2/4 → 1/1 |
| 同重构系列 #1664 → #1666 | `ee275a67` → `bb3134bc` | 38,207 → 2,622（−93.1%） | 40,980 → 41,577 | 4 档·5 处 → 1/2 |

判读纪律：**方向可判读、数字可复算，但不构成因果证明**（任务体量、模型档位、票类都不同）；纪律是否生效，看合并后新 run 的同一窗口。取数纪律：报数必须带「取数时刻 + log 行数或 mtime」；窗口里有 run 还在跑时全窗口汇总会继续变（本票写这一节期间两轮取数 2,517 → 2,527，差值 10 全部来自本票 run 自己：46 → 56 请求）；跨 log 版本不可比。四条纪律本身在根 `AGENTS.md`「会话纪律」。

