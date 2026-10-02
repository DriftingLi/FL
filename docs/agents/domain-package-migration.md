# 域包迁移手册

> 读者：把后端一个功能域从 `internal/service` + `internal/api` 搬进 `internal/<域>/` 的实施者。
> 每次搬家前读第 2、3 节；搬完逐条走第 9 节的验收清单。
> 定案：[ADR-0070](../adr/ADR-0070-域包形态与目录即射程的收口.md)。样板：faq（2026-10-01，P1 试点批）、notification（同日，试点批第二域 —— 跨域词汇、事件构造器与依赖倒置的完整样本，见第 7.1 与第 11 节）。

## 1. 目标形态

一个功能域 = 一个包：

    backend/internal/<域>/
      handler.go      HTTP 出口：RegisterRoutes + 端点定义 + 错误状态表
      service.go      应用服务：用例、事务、DTO 组装
      dto.go          响应 DTO（swagger definitions 键随包改名）
      errors.go       哨兵（少时可并入 service.go）
      *_test.go       域内单元测试（与实现同目录同包）

- **包名 = 域名**，域名取自 `backend/internal/apitypes/domains.go` 的域声明表（30 个域）。那张表同时是 swagger definitions 前缀与前端生成物的域边界，别另立一份域清单。
- **分层靠文件名约定，不靠子目录**：先例是 `scripts/check-catalog-sort.mjs` 的 `GUARDED_PATH_SEGMENT`（按文件名判定）。子目录 = 多包 = 导出面被迫扩大，且会让「目录即射程」的守卫再次失明。
- 域包允许依赖：`internal/model`、`internal/service`（共享助手，P3 收进 `internal/core`）、`internal/timefmt`（时间格式化叶子包）、`internal/security`、`internal/middleware`、`pkg/httpx`、`pkg/response`、第三方。
- 域包**不得** import `internal/api`（装配根在那里，反向依赖成环），也不得 import gin（HTTP 形状归 `handler.go`，见第 3 节）。
- **跨域共享的词汇贴着实体放 `internal/model`**（notification 批的裁决）：两个域都要读的常量塞进任一个域包，就会让两个域互相 import ⇒ 贴着实体定义。`ProfileFieldNickname` / `ProfileStatusPending` 等因此从 `internal/service/profile_review_service.go` 挪到 `internal/model/account.go`（紧挨 `ProfileChangeRequest`）。**只被一个域读**的常量仍留在域包。
- 不新建 `test/`、`tests/` 目录（`internal/layers` 的同居规矩）。

## 2. 迁移 recipe（以 faq 为样板）

1. **定域边界**：在 `backend/internal/apitypes/domains.go` 找到该域的 `Roots`（带 Go 包前缀的 swagger 键，如 `service.FaqResult`）。
2. **共享件先出包**（否则会造出没有消费者的导出 API；notification 批把五种情形都走了一遍）：
   - 域实现用到的**同包非导出助手**，先在该包里导出、调用点机械改名、原包不退场（faq 批：`BeijingNow`（`internal/service/auth_service.go:966`）、`IsDuplicateError`（`internal/service/forum_counter.go:67`），119 处 / 37 文件）。**P2 波 0a 已把这两者分别收编进叶子包 `internal/clock`（`clock.Now` / `clock.DayStart` / `clock.DayKey`）与 `internal/dberr`（`dberr.IsDuplicateError`），原一行委托与包装函数全部删除** —— 「原包不退场」只是过渡态，找到稳定落点后要收编干净，别让委托长期挂着；
   - **解析类**助手升级到 `pkg/httpx`（先例：`httpx.QueryIntPtr`，票 #1452；notification 批新增 `httpx.QueryIntDefault` —— 域包 import 不到 `internal/api` 的私有 `atoiDefault`，各域再抄一份就是第二处实现）；
   - **无状态纯函数**进叶子包：`internal/timefmt`（`FormatISO` / `FormatTimePtr`，原 `internal/service/helpers.go` 的私有函数）—— 域包与 `internal/service` 都取它，比塞回服务层轻（P3 收 `internal/core` 时再议去留）；
   - **数值 / 指针助手**进叶子包：`internal/coerce`（`ToFloat` / `ClampFloat` / `ParseFloat` / `ParseInt` / `IntPtr` / `FloatPtr`，原 `internal/service/helpers.go`）—— 只 import `strconv`；同批删掉两枚只转发标准库的助手（`withTimeout` → `context.WithTimeout`、`containsString` → `slices.Contains`）与 `internal/service/json_helpers.go`（8 行纯转发 ⇒ 调用点直接 `encoding/json`）（P2 波 0b）；
   - **吃 `*gorm.DB` 的分区谓词**也能进叶子包：`internal/scope`（`RecordPartitionOf` / `PartitionBucket` / `EntityOwnedBy` 三谓词 + 三枚导出片段 `RecordPartitionClause` / `PartitionBucketClause` / `EntityOwnedByClause`，原 `internal/service/credential_scope.go`，11 个 service 文件改限定名）—— 只 import `gorm.io/gorm`；判别句是「**它读参数还是读服务状态**」，不是「它碰没碰 DB」（P2 波 0b；**搬它必须同步静态扫描锁的白名单**，见第 3 节）；
   - **一族文件存取件整包搬**：`internal/filestore`（`FileStore` 的上传/删除/列表/读取 + 类型表 `fileTypeTable` + 附件归属判定 + 悬空回收单点，原 `internal/service/{file_store,attachment,file_type_table,image_cleanup_helpers,orphan_sweep}.go`，只 import `internal/storage` / zap / stdlib）。三条子规矩：**跨文件被用的私有闸门件升导出**（`FileExtension` / `FileContentType` / `AllowedFile` / `ValidateFileSize`，大小表不裸导出、改出纯函数 `MaxFileSize(filename)`）；**伸手进结构体私有字段的调用点换成导出方法**（新增 `FileStore.Exists(ctx, url)` + 哨兵 `ErrStorageUnconfigured`，调用方保住「没配存储 ⇒ 本域 500 哨兵，不读成文件不存在」）；**只转发标准库的私有助手保持私有**（`base64Encode`/`base64Decode` ⇒ 调用点直接用 `encoding/base64`）。**生成链里有它的硬编码路径**，见第 3 节（P2 波 0c）；
   - **出边为零的域直接搬**：material（`internal/api/material.go` → `internal/material/handler.go`、`internal/service/material_service.go` → `internal/material/service.go`）的域实现只引 `internal/timefmt` 之类的叶子包，**没有任何 `internal/service` 符号** ⇒ 第 2 步无共享件要提，是 P2 里第一个「只做机械搬家」的完整域（波 1a）。判据仍是出边：先 grep 域实现里的 `service.` 限定名与同包私有助手，两样都空才敢直接搬。
   - **跨域的读面单点进叶子包**：`internal/entitlement`（`CourseSKU` / `RealPaperSKU` / `Holds(db, userID, sku, refID)`，原 `internal/service/entitlement_read.go` 的 `CourseSKU` / 私有 `holdsEntitlement` + `points_service.go:796` 的 `RealPaperSKU`）—— 只 import `gorm.io/gorm` / `internal/model`；它是「这一份是否被这个人兑换过」与 sku 词汇表的唯一出处（ADR-0062 决策 3），points 域先搬 ⇒ 载体不能留在任一域包里。同批把两枚**跨域哨兵**贴实体进 `internal/model`：`ErrCourseNotFound`（`internal/model/training.go`）、`ErrHrwaiUserNotFound`（`internal/model/account.go`）——**model 里第一次出现包级哨兵**，依据是 ADR-0064 的跨域文案锁（同一 wire 文案不得由两个域的哨兵发出）而两者都被 points 域与 course / admin / 讲师 / 学员域同时引用（P2 波 1b-0）；
   - **别域的事实常量**改成**调用方传参**：站内信事件构造器收 `reason string`（`NewContributionApprovedEvent(userID int, title string, contributionID int64, points int, reason string)` 等 7 个）—— 积分流水原因是积分域的事实，通知域只把它记进 payload，于是不必 import 别域或服务层的常量；
   - **跨域共享词汇**贴着实体放 `internal/model`（第 1 节）。
3. **搬服务**：`git mv backend/internal/service/<域>_service.go backend/internal/<域>/service.go`；改 `package`；`XxxService`→`Service`、`NewXxxService`→`NewService`；共享助手写限定名（先例 `service.BeijingNow`，P2 波 0a 收编后写 `clock.Now()`）。DTO、哨兵、校验函数名原样不动。
4. **搬 HTTP 出口**：`git mv backend/internal/api/<域>.go backend/internal/<域>/handler.go`；改 `package`；`XxxHandler`→`handler`、`NewXxxHandler`→`newHandler`、`RegisterXxxRoutes(rg, rd RouterDeps, svc)`→`RegisterRoutes(rg *gin.RouterGroup, <只收它真正需要的依赖>, svc *Service)`。faq 只用了 `rd.Session`，于是签名就是 `RegisterRoutes(rg *gin.RouterGroup, session *security.Session, svc *Service)`，由 `routes_registry.go` 的闭包注入 `rd.Session`。
   - **不要把 `RouterDeps` 搬进域包**：它住在 `internal/api/deps.go`。
   - swagger 注解的 `data=service.X` 改成 `data=<域>.X`。
   - **一个域两条蓝图时分居两文件、名字带 Admin 后缀**：`/api/points` 与 `/api/admin/points` 同属积分域 ⇒ `handler.go` 的 `RegisterRoutes(rg *gin.RouterGroup, session *security.Session, svc *Service)`（6 条路由）+ `handler_admin.go` 的 `RegisterAdminRoutes(rg, session, pointsSvc *Service)`（POST `/admin/points/penalty`），两个 handler 类型 `handler` / `adminHandler` 都保持私有。同包不能有两个 `RegisterRoutes`，两个 HTTP 出口文件也都必须以 `handler` 开头（判据见第 3 节）。
   - 文件里那两个私有请求体结构（`faqCategoryBody` / `faqEntryBody`）原样留下：它们走 `c.ShouldBindJSON`，本来就不进 swagger definitions。**新增**端点要用 `@Param` 才可见。
5. **装配三处**：`internal/api/deps.go` 的字段类型（`FaqSvc *faq.Service`）、`internal/api/providers_*.go` 的构造（`d.FaqSvc = faq.NewService(c.db, c.logger)`，新增 import）、`internal/api/routes_registry.go` 的一行（`faq.RegisterRoutes(api, rd.Session, deps.FaqSvc)`）。
6. **域声明表**：`internal/apitypes/domains.go` 该域 `Roots` 的 `service.*` 改 `<域>.*`。
7. **证据表随域搬**：见第 6 节。
8. **同步锁与清单**：见第 3 节（尤其 `ResponsePackages()` 与两把 fact 计数锁）。
9. **重生成与验证**：见第 5 节。顺序是硬的：gofmt/vet → `make swagger` → `go run ./cmd/gen-apitypes` → `go test ./...` → 前端检查。

## 3. 必须同步的锁与清单

| 文件 | 改什么 | 不改的后果 |
| --- | --- | --- |
| `backend/internal/testutil/codescan.go` | `ResponsePackages()` 加一项 `{"internal/<域>","<域>"}` | fact 两把锁不扫新域包（**漏扫**，静默）；`HTTPSurface` 无需改（文件名规则自动纳入） |
| `backend/internal/api/consumption_fact_lock_test.go`、`backend/internal/apitypes/fact_reachability_lock_test.go` | 各自的 `n != <计数>` 与 Fatal 文案里的数字（迁一域 +1；faq 批 6→7） | 红——这是刻意的：数清单本身就是断言（ADR-0065 批⑤ 的规矩） |
| `backend/internal/apitypes/nullability_lock_test.go` | `nonNilEvidenceSources` 加 `{"internal/<域>","nonnilOutlets"}`；`sweptDirs` 加 `"<域>": "../<域>"` | 域包里声明的 `nonnil` 找不到证据（判据 5 零容忍）／旧键变幽灵键。**漏加 `sweptDirs` 的症状会骗人**：报的是「证据表里的 `notification.X.items` 并不是一条『射程内声明 nonnil』的字段」，看起来像字段改名，其实是被扫目录没进去 |
| `backend/internal/api/authz_coverage_lock_test.go` | `allow` 表**键是裸函数名**（12 条）；域包把注册函数改名 `RegisterRoutes` 后裸名会**静默失配** ⇒ 键改限定名（`notification.RegisterRoutes`） | 红（该表只在被点名函数上生效，改错名字即从判据里消失） |
| `backend/internal/api/path_parse_point_drift_lock_test.go` | `queryHelpers` / `allowedPathParseFuncs` 等按名字圈射程的清单（#1452 已允许限定名 `httpx.QueryIntPtr`）；**在 `pkg/httpx` 新增解析出口**（如 notification 批的 `QueryIntDefault`）要同时加进 `queryParseHelperNames` —— 那张表断言「宿主 `pkg/httpx` 里恰好这么多枚」 | 域包自带解析助手时判据看不见（**静默**）：解析类一律走 `pkg/httpx` |
| `scripts/check-render-error-face.mjs` | 无需改：`GUARDED_FILE_PREFIXES=['handler']` + `GUARDED_FILE_ROOT='backend/internal/'` 自动纳入域包 `handler*.go` | 若 HTTP 出口文件**不以 handler 开头**（如 `admin.go`），判据看不见（**静默**）⇒ 命名必须带 handler |
| `backend/internal/layers/layer_guard.go` | gin 面**无需改**（自动纳入域包 `handler*.go`）；但**依赖方向的三条硬规矩**要过一眼：① `pkg/httpx` 不 import `internal/...`；② `internal/api` 只许 `cmd/...` 依赖；③ **`internal/middleware` 不得 import `internal/service`**（notification 批新增，见第 7.1 节） | 域包 `handler.go` 一引 middleware，三角 `service → 域包 → middleware` 就成环：`go build` 直接报 `import cycle not allowed`（不是红一条判据） |
| `backend/internal/service/credential_scope_guard_test.go` | 白名单常量 `implementation` 改指实现处（波 0b 起 `internal/scope/scope.go`）、负向探针与失败文案同步；实现处每族谓词要留足命中（`implementationHits < 3` 判「扫描规则失灵」） | 锁的射程 = 整条 `internal/service` 生产代码：实现搬走后不指新家，判据会对旧路径零命中而报「规则失灵」—— **会红，但文案说的是扫描器坏，容易误诊** |
| `backend/internal/service/nonnil_outlets_*_test.go` | 属于该域的键与 outlet 函数**删掉**（不是复制），头部注释里的域名列表同步 | `testutil.AssertNonNilOutlets` 报「同名键出现在两张表」 |
| `scripts/check-catalog-sort.mjs` 的 5 条 `declaredIn`、`backend/internal/deploy/nginx_delivery_gen.go` 的硬编码路径（:154 生成注释文案、:231 `filepath.Join(root, "backend", "internal", "<包>", "file_type_table.go")`）与 AST 判据（按变量名 `fileTypeTable` 找表 ⇒ 变量名不许改） | 只在被点名的文件真搬家时改 | 缺文件即 Fatal（不静默），会在 CI 点名 |
| `frontend/nginx-host.conf`（生成物，两处提到路径：:15 手写表头 + :22 生成区） | 搬 `file_type_table.go` 后 `cd backend && go run ./cmd/gen-deploy -only nginx` 重生成（只动生成区那行）；**:15 在手写区，生成器不管**，要手改 | 两把锁 `TestNginxDeliveryMapInSync`（`codegen.AssertInSync`）与 `TestNginxDeliveryMapCoversFileTypeTable` 直接红（不静默）|
| `training-app/叉车维修培训学员端跨端应用/utils/*Contract.test.js` 里**硬编码的后端文件路径**（`resourcesContract.test.js:347` 直读 `backend/internal/api/material.go` 做幻影路由对照） | 搬 handler / service 时全仓 grep 这类路径（**含 `training-app/**`**；`frontend/` 与 `scripts/` 目前没有直读后端源码的写法），改指新家 | 该 ③ 门套件读不到文件即 fail-closed 判红（不静默），但只在 CI `mobile-test` 里红 |
| `training-app/叉车维修培训学员端跨端应用/**` 里**只提旧后端路径的注释**（`composables/useAiPro.uts`、`types/points.uts`、`api/points.uts`、`utils/*.test.js`） | **不改**（改注释就动到了 `.uts` 源文件）——PR 正文披露一行即可 | 无事发生（它们是史述）；但**改 `.uts` 会把 PR 降级为运行时面**：低风险白名单是**逐文件**口径（`*.uts` / `*test.js` / `*.md` / `jest.config*.js`，判据源 `.github/workflows/pr-evidence.yml`），本波只改了恰在白名单里的 `utils/pointsBalance.test.js:5` ⇒ ①真机与 ②微信开发者工具双双必过的口子没被打开 |
| `backend/internal/api/errstatus_test.go`（`TestErrStatusTable_Snapshot_Points`）与 `backend/internal/api/course_fact_distinctness_test.go` 的**哨兵清单表** | 哨兵搬家（波 1b-0 把 `ErrCourseNotFound` / `ErrHrwaiUserNotFound` 贴进 `internal/model`）时，表里的 `service.X` 改 `model.X`（快照表按哨兵**值指针**与文案比对） | 红（不静默），但报的是「哨兵值不符」，容易被读成文案被改坏 |
| 域包内**结构体字段**改名（波 0c：`OrphanSweepConfig` 的 `domain/ttl/list/referenced/keyOf/deleteFile/logger` → 导出名） | 包内**测试**里的复合字面量也用小写键（脚本的区间替换只覆盖包外 `<包>.OrphanSweepConfig{…}`）| `go vet` 报 `unknown field domain in struct literal`（不静默，但只在 vet 阶段）|

方法论：**改名会让按名字/路径判定的锁静默失配**。删或重构 Go 符号后，除了 grep 残留引用（本机 golangci-lint 跑不了，见 `checks.md` 环境 A），还要 grep 一遍**按名字判定的锁**（清单名、白名单键、前缀计数）。

## 4. 契约测试的归属

**契约测试留在 `backend/internal/api`**，不随域搬。理由：它走 `newContractDeps` + `NewRouter` 的全量装配链，而 `backend/internal/api/router_test_helper_test.go` 已明文反对另起测试装配链（「装配链分叉 = 测试装配与生产装配各测各的」）。faq 批实测：`internal/api/faq_contract_test.go` 只引用包内脚手架与 `NewRouter`，不引用任何 `service.Faq*` 类型 ⇒ 迁移后只改一处注释。

**域内单元测试与证据表随域搬**（`service.go` 的单测、`nonnilOutlets*` 表）。域包测试**不得**引用 `internal/service` 的测试脚手架——Go 的 in-package test 会成环；共享 runner 要提成 `internal/testutil` 的普通 `.go`（先例：`testutil/nonnil.go`、`testutil/codescan.go`）。

## 5. 响应 DTO、swagger 键与前端生成物

swagger 的 definitions 键**带 Go 包前缀**（`"service.FaqResult"`，`$ref` 写作 `#/definitions/service.FaqResult`）⇒ DTO 搬进域包后键变成 `faq.FaqResult`。这是**机械改名**，可验证：

1. `cd backend && swag init -g cmd/server/main.go -o docs`（本机 `swag.exe` v1.16.4，与 CI 钉的版本一致）；
2. `go run ./cmd/gen-apitypes`；
3. **验证 diff**：把 `HEAD` 版 `backend/docs/swagger.json` / `.yaml` 落到临时文件，对旧文本做一次纯文本替换（正则 `/service\.((?:Admin)?Faq[A-Za-z]*)/g`，替换成 `faq.<捕获组>`）再与新文件比：json 应**深度相等**（键序无关）、yaml 应**行多重集相等**；`docs.go` 里不应有 `service.Faq*` 残留。
4. **前端生成物「除声明顺序外」零 diff**：`tsName()`（`internal/apitypes/codegen.go`）只取最后一个点之后 ⇒ 包名不进 TS 类型名（faq / material 批 `git diff --stat -- frontend` 为空）。但 `collect()`（同文件）对根类型的传递闭包做 `sort.Strings`，排序键是**带包前缀的定义键** ⇒ 当一个域文件里混进了**别的域**的根类型（跨域复用的 DTO），换包会把 `points.*` 整段排到 `service.*` 之前，接口声明顺序与头部「覆盖的 Go 类型」清单一起前后移动（points 批：`generated/inspection.ts`、`generated/realExam.ts`）。判据因此是「**diff 只有顺序、类型内容逐字不变**」，不是「diff 为空」。

⚠️ 用 pwsh 读 `git show` 的输出会把 `warning: ... LF will be replaced by CRLF` 混进 stdout ⇒ 直接 `JSON.parse` 会炸：落盘再读，读时去掉 BOM。

## 6. 证据表随域走

`nullability:` / `nonnil` 的举证表按**组装发生在哪一层**选落点，可以住在域包：

```go
// backend/internal/<域>/nonnil_outlets_test.go
var nonnilOutletsFaq = map[string]func(t *testing.T) any{
    "faq.FaqResult.categories": func(t *testing.T) any { /* 真发 [] 的出口 */ },
}
func TestNonNilDeclaredOutletsNeverEmitNull(t *testing.T) {
    testutil.AssertNonNilOutlets(t, []map[string]func(t *testing.T) any{nonnilOutletsFaq})
}
```

- runner 住在 `backend/internal/testutil/nonnil.go`（普通 `.go`）：`MarshalKey` / `AssertNonNilOutlets`（跨表同键 Fatal、空表 Fatal、逐键断言非 `null`）。
- 键格式 `包名.类型名.json键` ⇒ DTO 换包后**键也要换**（`service.FaqResult.categories` → `faq.FaqResult.categories`）。
- `internal/apitypes/nullability_lock_test.go` 的 `owner` 表管**跨包唯一举证**：同一把钥匙出现在两个举证地即红（`谁欠证据` 必须只有一个答案）。同包内重复由运行时的 `AssertNonNilOutlets` 抓。

## 7. 通用件的三个出口

域实现需要别处的东西时，只有三条路（别开第四条）：

1. **注入**：`RegisterRoutes` 的参数（faq：`session *security.Session`），由装配根闭包注入。
2. **升级到 `pkg/httpx`**：HTTP 形状与请求解析（`ParseError` / `BadRequest` / `PathInt` / `PathInt64` / `QueryIntPtr` / `QueryIntDefault` / `QueryIDPtr` / `PositiveID` / `Endpoint` / 错误状态表）。`pkg/httpx` 不得 import 任何 `internal/...`（`internal/layers` 判据 ①）。解析类助手**一律**走这条（`internal/api/helpers.go` 最后三枚包私有助手已随波 0a 尾款搬完并删除该文件）；`PositiveID` 只吃字符串、不带 HTTP 语义，专给「已取到原文、要自己分流缺失与非法」的调用方。
3. **留在 `internal/service` 并导出**：一时找不到更轻落点的 DB / 业务助手 —— 但要先试完前两条与「叶子包」。**三类曾经留在这里的共享件已在 P1 / P2 波 0a 各自归位**：时钟（`clock.Now` / `clock.DayStart` / `clock.DayKey` → `internal/clock`）、唯一冲突谓词（`dberr.IsDuplicateError` → `internal/dberr`）、时间格式化（`timefmt.FormatISO` / `timefmt.FormatTimePtr` → `internal/timefmt`，见第 2 节）。三者的共性是**无状态、不碰 DB、只吃参数**，所以本出口只该剩「真的要 `*gorm.DB` 或服务内部状态」的件，P3 随 `internal/core` 收编。数值 / 指针助手与 `json_helpers.go` 这类的落点见第 10.1 节（波 0b）。波 0b 又证明**吃 `*gorm.DB` 不构成「必须留在服务层」的理由**：`internal/scope` 的三族谓词只吃 `*gorm.DB`、列名与证件 ID 三个参数，不碰任何服务实例状态 ⇒ 同样落进叶子包。判别句是「**它读参数还是读服务状态**」，不是「它碰没碰 DB」。

### 7.1 反向依赖（下游要业务层类型）：消费方接口反转

上面三条讲的是「域包要别处的东西」。反过来 —— **HTTP 基建要业务层类型**时，绝不能在基建包里 import 业务包，否则三角成立（notification 批实测）：

    cmd/*  →  internal/logger  →  internal/middleware  →  internal/service  →  internal/notification  →  internal/middleware
                                                                                    ↑ import cycle not allowed

做法是**消费方定义接口**（先例：`internal/middleware/audit.go`）：

- `type AuditWriter interface { Write(record model.AuditLog) error; DescribeAction(method, path string) string }`，`AuditLog(svc AuditWriter, logger *zap.Logger)` —— 基建只认自己需要的那两个方法，实现仍是单点 `*service.AuditService`（`internal/service/audit_service.go:28` / `:53` 原样满足），由装配点注入。
- **typed nil 陷阱**：`(*service.AuditService)(nil)` 装进接口**不等于** nil 接口，`svc == nil` 拦不住 ⇒ 判空上移到装配点（`internal/api/router.go:70-75`、`internal/valuation/handler/router.go:151` 都写成 `if auditSvc != nil { ... }`），基建里只留「真传进来的 nil 接口」兜底。
- 这条因此被钉成方向规矩 ③（`internal/layers/layer_guard.go` 的 `directionViolations`）：`internal/middleware` 不得 import `internal/service`；合成违规自测在 `layer_guard_test.go`。

**外部测试包例外**：`package foo_test` 住在被测包之外，两条边都能拿（import cycle 只发生在同一构建里）⇒ 它**不进依赖图**：`layer_guard.go` 的 `importEdges` 见到包名以 `_test` 结尾就跳过。这条例外是必要的 —— `internal/middleware/audit_ip_test.go` 就得拿**真实**的 `service.AuditService` 落库举证（本次成为全仓唯一一处外部测试包）。内部测试包（`package foo`）仍判：反向 import 在 test 构建里就是 import cycle。

## 8. 不许做的事

- 不改对外行为：wire 文案、状态码、字段名、分页形状**逐字不动**（本专项是搬家，不是改协议）。
- 不搬契约测试（第 4 节）、不新建子目录分层（第 1 节）。
- 不在域包 import `internal/api`；不在 `service.go` / `dto.go` import gin。
- 不为了过守卫放宽射程或删判据：要改就改**计数常量**并同步注释里的算式（欠账表只准按人改的数字走）。
- 不把「本机 golangci-lint 跑不了」当借口：那一条由 CI `backend-lint` 兜，提交前 grep 残留引用与按名字判定的锁。

## 9. 单域验收清单

- [ ] `gofmt -l .` 空；`go build ./...`；`go vet ./...`
- [ ] `go test ./internal/<域>/... ./internal/layers/... ./internal/apitypes/... ./internal/service/... ./internal/api/...`
- [ ] `node scripts/check-render-error-face.mjs --all`；涉目录读面时另跑 `node scripts/check-catalog-sort.mjs --all`
- [ ] `ResponsePackages()` 已加域包；两把 fact 计数锁与文案已同步
- [ ] `nullability_lock_test.go` 的 `nonNilEvidenceSources` / `sweptDirs` 已加域包；域内证据表键已换包前缀；旧表里的键已删
- [ ] 没引入新的反向边：`go test ./internal/layers/` 过（三条方向规矩）；下游要业务层类型时走消费方接口反转（第 7.1 节）；在 `pkg/httpx` 新增解析出口时同步了 `queryParseHelperNames`
- [ ] `make swagger` + `go run ./cmd/gen-apitypes` 后：swagger diff 只有机械改名、`git diff --stat -- frontend` 为空
- [ ] 旧符号（`XxxService` / `RegisterXxxRoutes` / 旧包前缀键）全仓残留 = 0
- [ ] 域有两条蓝图时（`/x` 与 `/admin/x`）：`RegisterRoutes` / `RegisterAdminRoutes` 与 `handler` / `adminHandler` 分居 `handler.go` / `handler_admin.go`，两个注册函数都只收自己需要的依赖
- [ ] 搬走的**私有**助手在包内还有调用点，或者被删掉：本地 `go build` / `go vet` 报不出「未使用」（那是 `unused` 的活，只在 CI `backend-lint` 兜）；自查方式是对每个进新包的私有函数数一遍**剥掉注释**后的包内引用
- [ ] 搬动的后端文件若被移动端契约测试直读（第 3 节那一行），硬编码路径已同步；**点段路径的工作树里跑不了 ③**（jest 对 `.scratch` 这类父段静默 0 套件、还 exit 0）⇒ 本地跑不了就明确交 CI `mobile-test`
- [ ] PR 正文一行披露「合并到 master 将触发 production 部署」

## 10. 波次与 PR 纪律

- 一波 1 个 PR、**≤3 个域**、diff ≤1500 行；域的顺序按 P1 计划公布的四波（faq / notification 是试点批）。
- 生成链顺序是硬的：gofmt/vet → `make swagger` → `go run ./cmd/gen-apitypes` → `go test ./...` → 前端检查（`internal/apitypes/codegen_test.go` 把前端生成物与注解渲染结果全等比对，顺序弄反会先绿后红）。
- 每个 PR 合并即触发 production 部署（master push ⇒ `cd.yml`）⇒ PR 正文必须留一行披露。

### 10.1 共享叶子与跨文件共享件的落点（P2 判据：出边为零，或单向无环）

域包能不能单独搬，看出边 —— 域实现引用的每个残留符号都会逼它 import `internal/service`（只要 service 里还有一处引用该域，就是 import cycle）。**判据（3a 修订）**：出边为零，**或**出边单向无环（`internal/service` 对该域的引用已清零、不再回头），并把这批出边逐枚写进 PR 正文与 ADR —— forum（2b-2）是第一个实例（1 枚 `ForumCounter`），auth（3a）是第二个（20 枚，见下表）。入边只决定 diff 大小与合批。拆波前先扫一遍**跨文件私有引用**（同包内 A 文件用 B 文件的私有符号；共享扫描器的正式出处是 `internal/testutil/codescan.go`，别在调用点手抄判据）。

| 件 | 落点 | 状态与说明 |
| --- | --- | --- |
| 时钟（`Now` / `DayStart` / `DayKey` / `Location`） | `internal/clock`（叶子） | 波 0a 已收编；`internal/service` 里的一行委托（`BeijingNow` / `startOfShanghaiDay` / `shanghaiDayStr`）全删，调用点写限定名 |
| DB 错误判定（`IsDuplicateError`） | `internal/dberr`（叶子，只 import `strings`） | 波 0a 已收编；双方言谓词测试随行 |
| 时间格式化（`FormatISO` / `FormatTimePtr`） | `internal/timefmt`（叶子） | notification 批收编 |
| 解析出口：查询侧 `QueryIntPtr` / `QueryIntDefault` / `QueryIDPtr` / `PositiveID`，路径侧 `PathInt` / `PathInt64`，以及 `ParseError` / `BadRequest` / `Endpoint` | `pkg/httpx` | 波 0a 尾款收编；`internal/api/helpers.go` 已删；漂移锁 `path_parse_point_drift_lock_test.go` 钉住两侧计数（宿主四枚、本包零枚），新增出口必须同步 `queryParseHelperNames` |
| credential 谓词（`RecordPartitionOf` / `PartitionBucket` / `EntityOwnedBy`） | `internal/scope`（叶子，只 import `gorm.io/gorm`；波 0b ✅ 已交付） | 三枚包私有片段已升为导出片段（favorite 子查询复用同一份判据）；三族 nil 语义**相反**（记录冻结分区 nil 看全部 / NULL 桶 nil 只看 `credential_id IS NULL` / 归属分区 nil 看全部）⇒ 搬包时同步静态扫描锁的白名单路径（白名单就是谓词实现处那个文件） |
| 数值/指针助手（`toFloat` / `clampFloat` / `parseFloat` / `parseInt` / `ptrInt` / `floatPtr` / `containsString` / `withTimeout`） | `internal/coerce`（叶子，只 import `strconv`；波 0b ✅ 已交付） | 同批删掉 `internal/service/json_helpers.go`（8 行纯转发 `jsonMarshal` / `jsonUnmarshal`，调用点直接用 `encoding/json`）；波 3a 追加泛型 `func Ptr[T any](v T) *T` —— `internal/service/auth_service.go:140` 的包内 `ptr[T]` 定义随域搬包，而留驻的 `internal/service/question_bank_service_test.go:312` 也在用，故定义收编进本叶子包（auth 内 16 处 + 留驻 1 处改 `coerce.Ptr`）；波 3b-1 追加 `StrPtr` / `RoundFloat1` / `RoundFloat2` —— 课程域的 `ptrStr` 与 `roundFloat1/2` 随域搬，而留驻 `internal/service` 侧同一批助手仍在用（`internal/service/credential_test.go:24` 等）⇒ 与 `Ptr` 同理由收编进本叶子包 |
| 组内排序位置三助手（`NextValue` / `Renumber` / `SwapPositions` + `ErrSwapItemNotFound`） | `internal/sortorder`（叶子，只 import `errors` / `gorm.io/gorm`；波 3b-1 新建 ✅） | 原 `internal/service/course_service.go` 的 `nextSortOrderValue` / `renumberSortGroup` / `swapGroupPositions`（`toInt` 保持私有）。落点理由写在包注释里：课程域与培训域互为强环（15 条边），载体留任一域包都会让另一域反向依赖它，留 `internal/service` 导出又成环 ⇒ 第三种破法（无状态纯函数进叶子包）|
| 切片去重（`Ints`） | `internal/slicesx`（叶子，无 import；波 3b-1 新建 ✅） | 原 `internal/service/course_service.go` 的 `dedupeInts`，课程域（前置课程去重）与培训域（题目标签去重）都在用。**不进 `internal/coerce`**：该包自述是「宽松数值 / 指针转换」，切片去重会稀释它的命名族（`<Type>Ptr` / `Parse*` / `To*` / `Clamp*`）。豁免：`backend/cmd/import-reference-content/papers.go:136` 有一份同名独立副本（不 import 叶子、随该 cmd 自立，§11 残留 grep 已登记）|
| 题目池 raw SQL 片段（`PublishedSQL` / `ExcludeSourceTagsSQL` / `CredentialColumn`） | `internal/questionpool`（叶子，零 import；波 3b-2 新建 ✅） | 原 `internal/service/question_pool_scope.go:28-37` 的 `QuestionPoolPublishedSQL` / `QuestionPoolExcludeSourceTagsSQL` / `QuestionPoolCredentialColumn`（常量名去掉包名已承载的前缀）；文件自述「raw SQL 计数必须引用它们、不得就地重写」是本包存在的理由。消费者是留驻 service 的池 scope 与 `internal/training` 的标签计数（training 不得 import service）。池的另两形态（gorm 链式 `QuestionPoolScope` / `QuestionReadScope` / `QuestionEditScope` 值对象）暂留 service，待题目域收口时同迁 |
| 文件存取一族（`FileStore` 上传/删除/列表/读取、类型表 `fileTypeTable`、附件归属 `IsSiteAttachmentURL`/`AttachmentKey`/`ReadMultipartFile`、悬空回收 `RunOrphanSweep`） | `internal/filestore`（叶子，只 import `internal/storage` / zap / stdlib；波 0c ✅ 已交付） | 五个源文件 + 四个测试文件整包搬（9 个 `git mv`，39 文件机械改名）。**跨文件私有引用 11 处**是搬包的主要暗礁，三种破法：闸门四件升导出（`FileExtension` / `FileContentType` / `AllowedFile` / `ValidateFileSize`）+ 大小表改纯函数 `MaxFileSize(filename)`；结构体私有字段改导出方法（`FileStore.Exists` + `ErrStorageUnconfigured`）；纯转发标准库的 `base64Encode`/`base64Decode` 保持私有、调用点直接用 `encoding/base64`。`image_cleanup_helpers.go` 只导出 `MarkdownImageURLs`，`orphan_sweep.go` 的 `OrphanSweepConfig`/`RunOrphanSweep` 升导出 |
| 权益读面（`CourseSKU` / `RealPaperSKU` / 「这一份是否被这个人兑换过」）与两枚跨域哨兵（`ErrCourseNotFound` / `ErrHrwaiUserNotFound`） | 读面进 `internal/entitlement`（叶子，只 import `gorm.io/gorm` / `internal/model`）；两枚哨兵贴实体进 `internal/model`（波 1b-0 ✅ 已交付） | 读面源 = `internal/service/entitlement_read.go` + `points_service.go` 的 `RealPaperSKU`；`realPaperPrice` **不搬** —— 它读 `points_shop_item`，是积分域自己的价格事实（波 1b 已升导出 `RealPaperPrice()`）。哨兵是 model 里第一次出现包级哨兵：`Holds` 仍回 error 而不吞（「查不动」与「真不存在」由各自调用方分开） |
| 跨文件的请求 DTO 类型（`idParam` / `taskIDParam` / `courseIDInput` / `chapterIDInput` / `swapCourseSortReq` / `generateContentReq`，声明在 `internal/api/admin.go:873-905`，被 `admin_recruiter.go` / `settings.go` / `tutor.go` 跨文件引用） | **各域包自己声明** | 它们本来就是各域的请求面（`struct{ ID int }` + 用 `httpx.PathInt` 的 Parse 闭包），不是解析出口 ⇒ 不进 `pkg/httpx`；拆包时随 handler 搬、同名保留 |
| 既要用 gin 又要用 middleware + service 的域内解析助手（`studentQuestionScope`，`internal/api/question_bank.go:105`） | **题库域包**（3c 波随 `internal/questionbank` 导出） | **不能进 `pkg/httpx`**：`pkg/` 不得 import 任何 `internal/...`（`internal/layers` 判据 ①），而它同时需要 `internal/middleware`（`CredentialIDPtr`）与 `internal/service`（`NewQuestionReadScope`）；调用它的 `favorite.go` / `note.go` / `question_interaction.go` 都在 3c/4b/4c 波之后 ⇒ 不阻塞波 1、2 |
| 邮箱发送一族（`MailSender` / `SMTPMailSender` / `LogMailSender` / `NewMailSender`） | `internal/service/mailer.go`（留驻面，波 3a ✅） | 被**留驻 service 侧**的 `ContactService`（`internal/service/contact_service.go:59/64`）、`JobReportService`（`job_report_service.go:35/44`）、`JobApplicationService`（`job_application_service.go:57/70`）与搬进 `internal/auth` 的验证码通道（`internal/auth/code_service.go:178/185`）**同时**使用 ⇒ 进任一侧都成环，只能留 `internal/service`；装配点 `internal/api/providers_core.go:39/69`。auth 侧一律写 `service.MailSender` / `service.NewMailSender` |
| 身份词汇与口令原语（`HrwaiRole` / `RecruiterRole` / `TutorRole` / `ErrRecruiterNotFound` / `MaskedPhone` / `HashPassword` / `VerifyPassword`） | `internal/service/identity_vocab.go`（留驻面，波 3a ✅） | 同一批符号同时被 `internal/auth`（写 `service.HrwaiRole` 这类限定名）与留驻 `internal/service` 消费。`TutorRole` 在此处的理由：吊销命名空间键与 JWT 角色 claim 是同一个字符串，此前只以字面量散在登录分派里 |
| 随机账号生成（`GenerateRandomAccount`） | `internal/service/account_gen.go`（留驻面，波 3a ✅） | 原 `AuthService.generateRandomAccount` 升导出留驻：消费者是 `internal/auth/code_service.go` 与留驻的 `internal/service/admin_service.go:152` 两处，进 auth 必成环 |
| 口令写动作（`ApplyHrwaiPassword` / `ApplyRecruiterPassword` / `ValidatePasswordLength` / `PasswordWriteResult`） | `internal/service/password_write.go`（留驻面 + 两个导出包装，波 3a ✅） | 方法形态 `func (s *AuthService) SetNewPassword` 搬包后留不下来 ⇒ **动作留本包、声明权归调用方**：五条入口里学员两条在 `internal/auth`、管理员三条在留驻 `AdminService`，故改成两个包级函数，`auth.AuthService.SetNewPassword` 与 `auth.VerifyCodeService.ResetPasswordWithCode` 成为包装调用点；`validatePasswordLength` 升导出（auth 侧必须**在消费验证码之前**做前置校验） |

**P2 波次（issue #1445 公布，2026-10-01）**：波 0 共享叶子 3 个 PR —— 0a 时钟 + DB 助手 + 解析出口（✅ 已交付）；0b `internal/scope` + `internal/coerce` + 删 `json_helpers.go`（✅ 已交付）；0c `internal/filestore`（✅ 已交付）。波 1：1a material（✅ 已交付，唯一不需要提共享件的域）/ 1b points（✅ 已交付：共享件 1b-0 先出包 —— `internal/entitlement` + 两枚跨域哨兵贴 `internal/model`；域搬包 `internal/points` 随后同波完成后半）/ 1c inspection（✅ 已交付：出边为零、不需要先出共享件；三条管理端 GET 与三个 typed DTO 出包，2026-10-01）。波 2：2a featured + checkin（✅ 已交付：两个域同波搬包、不需要先出共享件；唯一跨域共享件 `ForumAuthor` 按「跨域词汇贴实体」移进 `internal/model`，2026-10-01）/ 2b-1 contribution（✅ 已交付：出边天然干净、不需要先出共享件；6 个 `git mv` + handler 一拆二（公开 8 条 / 管理端 6 条）+ DTO 抽 `dto.go`；留驻侧唯一跨域符号 `ContributionStatus*` 改限定名，2026-10-01）/ 2b-2 forum（✅ 已交付：五个生产文件 2733 行 + HTTP 面一拆二（公开 21 条 / 管理端 10 条）+ 五个随域测试 1126 行，合计 5132 行；ForumCounter 留 internal/service 导出、域包 import 取用（首个依赖 internal/service 的域包实例）；ErrChapterNotFound 留 service；留驻侧唯一编译断点是 forum_counter_test.go 的注销回扣用例，改为直插赞行 + NewForumCounter 造数，2026-10-01） / 2c aiAssistant（✅ 已交付：11 个生产文件 3 633 行 + HTTP 面三份 1 018 行（学员端 handler.go / 诊断代理 handler_diagnosis.go / 管理端 handler_admin.go —— **文件级三拆，没有行号切点**）+ 12 个随域测试 3 154 行；出边为零（11 个生产文件对 internal/service 顶层符号零引用）；反向边 5 个生产文件（service → 域包，非首例）；orDefault 在 internal/service 留 4 行副本、diagnosisSourceID 升导出 DiagnosisSourceID、新增 NewQuestionExplanationWith；nullable 证据表域包自带 runner；authz 覆盖锁新增 aiassistant.RegisterAdminRoutes 豁免（守卫在 internal/api/admin.go:39 的 /admin 组上），2026-10-02）。波 3：3a auth（✅ 已交付：**最大域** —— 14 个生产文件 3800 行 + 10 个随域测试 2229 行整包 `git mv` 进 `internal/auth`，机械面合计 6029 行 / 24 个 `git mv`；**出边不为零而单向无环**：auth → `internal/service` 20 枚符号（邮箱族、身份词汇、uid 族、口令写动作、`ForumCounter`），反向引用 0 命中；留驻面新增 `internal/service/{mailer,identity_vocab,account_gen}.go` 并把 `password_write.go` 的 `SetNewPassword` 方法改成两个包级包装；`deps.AuthH` 字段与 `NewAuthHandler` 装配行删除（handler 私有化，会话由 `RegisterRoutes` 内同步 —— 吊销类用例做了变异检验，11 条判红）；跨端 `authLoginOutletsContract.test.js` 7 处锚点重指向，2026-10-02）/ 3b-1 course（✅ 已交付：6 个生产文件 2 363 行 + 1 个 HTTP 面 357 行 + 7 个随域测试 1 520 行整包 `git mv` 进 `internal/course`；域包对 `internal/service` **0 命中**，反向 `internal/service` → `internal/course` 是允许的单向边（tutor 波 4d 会继承）；破环四件套拆出两个新叶子 `internal/sortorder` / `internal/slicesx`、给 `internal/coerce` 追加 `StrPtr` / `RoundFloat1` / `RoundFloat2`、三枚跨域哨兵贴 `internal/model/training.go`；导师侧 4 条用例拆回 `internal/service/tutor_chapter_detail_test.go`、课程 CRUD 3 条搬进 `internal/course/admin_service_test.go`，2026-10-02）/ 3b-2 training（✅ 已交付：5 个生产文件 1 461 行 + 1 个 HTTP 面 1 131 行按职责**文件级三分**（学员读面 `handler.go` / 管理面 `handler_admin.go` / 证件面 `handler_credential.go`）+ 5 个随域测试 1 334 行整包搬进 `internal/training`；域包对 `internal/service` **0 命中**，反向 `internal/service` → `internal/training` 是允许的单向边（`question_service.go:413`/`:488` 改 `training.ReplaceQuestionTags`）；破环：新建叶子 `internal/questionpool` 承载三枚 raw SQL 片段（计划外，见 §11 第十四批）、私有表 `sortFacts400` 升导出 `training.SortFacts400` 供 `internal/api` 的课程交换表复用、留驻 `internal/service` 侧三枚搬走 helper 就地内联；证据表按「生产者是谁」再平衡（`course.CourseDTO.chapters` 与 `training.CatalogLevelNode.courses` 两格从留驻 service 搬回域包）；跨端三锚点重指向（`dashboardContract` / `coursesContract` / `resumeContract`，三套件 193 passed）；前端生成物 **0 diff**，2026-10-02）/ 3c questionBank + practiceMode。波 4：4a mockExam + realExam / 4b student + favorite + search / 4c note + questionInteraction + wrongQuestion / 4d tutor + admin / 4e recruit + resume + job / 4f 收尾 audit + export。

**三个强环与破环手法**：course↔training、course↔tutor/points、resume↔recruit。五种破法 —— 跨域共享词汇贴实体进 `internal/model`、别域的事实常量改调用方传参、无状态纯函数进叶子包、下游要业务层类型走消费方接口反转（第 7.1 节）、助手搬回自己的域（3b-1 实例：三枚批量加载器从留驻 `internal/service` 搬进 `internal/course/batch_backfill.go`，留驻侧写限定名 —— 方向是 `service → course` 的单向边；3b-2 实例：course↔training 强环上 training 侧要用的三枚 raw SQL 片段进 `internal/questionpool`）。

## 11. 批量机械改名的纪律（血账）

一个域几百处引用只能靠脚本改名 —— 脚本本身就是风险源。notification 批在这里踩过两次，四条硬规矩：

- **pwsh 的 `-replace` 大小写不敏感**（要区分大小写必须 `-creplace`）。用它给 `internal/service` 加 `notification.` 限定名时，替换词表里的 `Service` / `NewService` 把 `package service`、字符串 `"forklift-training/internal/service"` 一起改掉，**一次性改坏 212 个文件**（import 路径变成 `"forklift-training/internal/notification.Service"`）。复原手法：`git checkout -- backend/internal/service`（从 **index** 恢复；`git mv` 的改名已 staged，不会被撤销）。
- **替换词表里剔除过于通用的名字**：`Service` / `NewService` / `Handler` 这类词靠词边界拦不住组合形态，必须用显式模式逐条写（`-creplace 'NewNotificationService\(', 'notification.NewService('`、`-creplace '\*NotificationService\b', '*notification.Service'`）。
- **闸门先试跑**：脚本先只对 2-3 个文件跑一遍看 diff；跑完立刻反查「**不该变的文件为什么出现在变更列表里**」（`uid.go` / `nickname.go` 这种域外文件一出现就停手回滚）。
- **自检判据要写对**：`(?m)^package service\r?$` 计数 == 1、不得出现 `"forklift-training/internal/notification.``（字符串里带点的路径）、不得出现双重包前缀；**别在 CRLF 文件上用 `$` 收尾判行**（`$` 匹配不到 `\r` 前的位置，会给出假阳性）；改完 `gofmt -l` 必须为空。

第二批（P2 波 0a：`BeijingNow`→`clock.Now`、`IsDuplicateError`→`dberr.IsDuplicateError`、`startOfShanghaiDay`→`clock.DayStart`、`shanghaiDayStr`→`clock.DayKey`）又添三条：

- **先删定义，再改名**：改名规则只要会命中定义行（`func IsDuplicateError(`、`func (s *X) startOfShanghaiDay(`），就必须先把定义整块删掉或搬走再跑脚本 —— 否则产出 `func dberr.IsDuplicateError(` 这种语法垃圾。**方法形态尤其危险**：裸模式会匹配 `s.startOfShanghaiDay(`（`s.` 不是词边界），要么用后置断言 `(?<![\w.])`，要么先显式替换限定形态。
- **别「内存改一遍 + 磁盘另跑一遍」**：把文件读进内存做区间删除、又用另一遍全仓扫描直接写盘，最后 `save()` 内存版会**把磁盘上的改名结果覆盖回去**（波 0a 实测 `contribution_service.go` 的 6 处改名被覆盖，方法名成了 `func (s *ContributionService) clock.DayStart(...)`，靠 `go build` 才发现）。每步改完立刻 `git status` + `go build`。
- **正则不剥注释、也不防同名局部变量**：`clock := &stepClock{}` 这种局部变量会让「该文件用了 `clock` 包」的判据误判，给不需要的文件加 import（`go vet` 报 imported and not used 才抓到）；注释里的引用会被一起改名 —— 改完注释是对的，但 import 是多余的，加 import 的判据要么剥注释、要么以编译器为准。

第三批（P2 波 0c：五文件搬 `internal/filestore`，39 文件机械改名）再添七条：

- **搬包前先扫「跨文件私有引用」，不要只扫导出面**：被搬的文件里，11 处私有符号是**留在原包的其他文件**在用的（`fileExtension` ×6、`allowedFile` ×2、`validateFileSize` ×2、`fileContentType`、`maxFileSizes`、`base64Encode`/`base64Decode`，外加一次直接读结构体私有字段 `s.fileSvc.storage`）。这类引用在 `git mv` 之前 grep 不到（同包内不需要限定名），`go build` 才一次报全 ⇒ 先做一遍「新包导出面 vs 旧包剩余文件引用」的比对，再逐个决定升导出 / 换导出方法 / 调用点直接用标准库（本波三种破法各用了一次）。
- **别用「导出字段」破封装**：`stagedFileExists` 读的是 `s.fileSvc.storage`，把 `storage` 升导出等于把存储适配器漏给所有调用方；正解是在 `FileStore` 上加**语义完整的导出方法**（`Exists(ctx, url)`）并把它自己的失败语义带出来（`ErrStorageUnconfigured` ⇒ 调用方仍回本域 500 哨兵，不静默读成「文件不存在」）。
- **成对命名（大驼峰 / 小驼峰并存）要求全脚本一律 `-creplace`**：`FileStore` 与局部变量 `fileStore`、`markdownImageURLs` 与 `MarkdownImageURLs` 必须分开处理。反过来，**否定后顾 `(?<![\w.])` 会漏掉带点的旧包前缀**（`service.NewFileStore` / `service.ReadMultipartFile` 不是裸名）⇒ 改名后要额外 grep 一遍 `<旧包>.<符号>` 的组合形态（本波漏 9 处，靠 `go build` / `go vet` 抓到）。
- **改结构体字段名要连包内测试的复合字面量一起改**：脚本的区间替换只覆盖包外 `<包>.OrphanSweepConfig{…}` 形态，包内 `orphan_sweep_test.go` 两处字面量仍是小写键 ⇒ `go vet` 报 `unknown field domain in struct literal`。字段导出后先 grep 全仓 `\b<小写字段>:` 再收工。
- **生成链上的硬编码路径要一起改，且分「生成区 / 手写区」**：`internal/deploy/nginx_delivery_gen.go` 的 :154 文案与 :231 路径（AST 判据按变量名 `fileTypeTable` 找表 ⇒ **变量名不许改**），生成物 `frontend/nginx-host.conf` 的生成区一行由 `cd backend && go run ./cmd/gen-deploy -only nginx` 覆写、**手写表头 :15 那行生成器不管**。两把锁 `TestNginxDeliveryMapInSync` / `TestNginxDeliveryMapCoversFileTypeTable` 会在 CI 点名。
- **`gofmt -w` 会改写文件**：脚本跑完执行 `gofmt -w` 之后再用编辑工具改同一文件，会报 `file changed since it was read` ⇒ 重新 read 再改。
- **搬走的私有助手要回头看它在新家还有没有调用点**：`base64Encode` 的唯一消费者在包外（`internal/service/slide_renderer.go:245`），那条调用点改成 `base64.StdEncoding` 之后它在包内零引用 —— 而「未使用函数」**不由 `go build` / `go vet` / `go test` 报**（本机三样全绿），只有 CI `backend-lint`（`unused`）报 `internal/filestore/file_store.go:309:6: func base64Encode is unused (unused)`。本波第一个 PR 就是被这条拦下的：**本地验证全绿 ≠ CI 绿**，搬包后除「跨文件私有引用」（上一条）之外，还要反向数一遍「私有件在新包内的引用数」。数的时候**先剥注释** —— 文档注释里常把函数名再写一遍，会让零引用看起来非零。

第四批（P2 波 1a：完整域 `material`，两个 `git mv` + 14 处调用点）再添三条：

- **批量脚本只做「行级操作」，Go 源码一律用手工编辑逐处改**：`move-material.ps1` 第一版把 `$TAB` / `$DQ` 占位符写在 PS **单引号**字符串里（单引号内不插值 ⇒ 替换全不命中），第二版改用模板插值出真实制表符与引号、又拼 `[char]10` 换行 —— 结果把 7 个文件（`internal/api/{deps,providers_training,routes_registry,envelope_registry,mobile_p1_contract_test}.go`、`internal/apitypes/nullability_lock_test.go`、`internal/testutil/codescan.go`）**写成整文件一行**（判据：`[regex]::Matches($t, [string][char]10).Count -eq 0`；被脚本碰过的文件都要数一遍）。复原靠 `git checkout -- <带 `backend/` 前缀的路径>`，随后逐处手改。
- **搬 handler 要全仓 grep 消费者的硬编码路径，别只 grep Go**：移动端 `utils/resourcesContract.test.js:347` 直读后端源码做幻影路由对照（`read('../../backend/internal/api/material.go')`），文件一搬它就 fail-closed 判红；同文件 :345 注释与 :359 用例名里的文件名文案、以及 `materialsPageContract.test.js:5` 的 `material.go:25-37` 行号引用（已改指 `internal/material/handler.go:31-38`）都要一起同步。
- **点段路径的工作树里跑不了移动端 ③**：`D:\FL\.scratch\…` 这种含点父段的落点，jest `--listTests` **静默 0 套件且 exit 0**（#1144 的坑）；建无点 junction 也没用 —— jest 会把 rootDir realpath 回点段。本波取证办法：把移动端目录 robocopy 到无点路径（`D:\flmob\…`）+ `node_modules` junction + 一份新家的 `handler.go`，再用主树的 jest 跑该套件（`utils/resourcesContract.test.js` 41 passed）。

第五批（P2 波 1b-0：两枚跨域哨兵贴 `internal/model` + 权益读面进 `internal/entitlement`）再添三条：

- **删掉定义后，同一文件里的其余裸引用要立刻 `replace_all`**：`internal/service/course_service.go` 的 `ErrCourseNotFound` 定义删掉后，函数体里还有三处裸名（:232/:330/:362），而 `go build` **一次只报前 10 条** `undefined`（先报 course_service，`points_service.go` / `real_exam_service.go` 的十几处要再跑一遍才看到）⇒ 别逐处点，删定义后就在该文件里对同名标识符做一次 `replace_all`。
- **批量补 import 的命令要有「非空」校验**：`goimports -local forklift-training -w <文件列表>` 在列表**为空**时静默 `exit 0`（本波第一版把 `git status --porcelain` 的 `backend/…` 路径拿去给 cwd 已是 `backend/` 的 `Test-Path` 过滤，全被滤掉 ⇒ 看着「补过了」，`go build` 却全是 `undefined: entitlement`）。路径口径要对齐：`goimports` 收的是**相对模块根**（`internal/...`），`porcelain` 给的是**相对仓库根**（`backend/internal/...`）。
- **本机 `golangci-lint` 已不可用，别信它报的错**：`%USERPROFILE%\go\bin\golangci-lint.exe` 配 `go1.27.1` 时刷一片 `typechecking error: … export data version 4 is greater than maximum supported version 2`（连 `unicode/utf8`、`internal/goarch` 都导不进来，还捎带一批与本次改动无关的假错）—— 它自带的 Go 比仓库工具链旧。波 0c 那条「本地三样全绿 ≠ CI 绿」的兜底只能靠**反向数私有件在新包内的引用数**（先剥注释），不能靠本机 lint。

第六批（P2 波 1b：完整域 `points`，13 个 `git mv` + 172 处入边）再添四条：

- **`goimports` 不会为新搬出的包补 import**：`internal/service` 里的 `points.X` 全是 `undefined: points`，`goimports -local forklift-training -w` **一条都没加**（它只在能推断出包路径时才动手）⇒ 搬包后要自己按「出现 `<域>.[A-Z]` 但没有该 import」插一行，插在 `forklift-training` 组的字母序位置上；补完再跑一次 `goimports` 收格式。
- **包内改名会改变导出名 ⇒ 包外的替换规则要跑第二遍**：第一遍规则把 `PointsService` 一起写成了不存在的 `points.PointsService`（16 个文件），因为包内规则已把它改名成 `Service`。判据：`go build` 绿之后再 grep 一遍 `<域>.<旧类型名>` = 0 才收工。
- **先探「局部变量与包同名」再插 import**：`points := NewPointsService(...)` 这类绑定在 import 进来之后变成**遮蔽**（报错会指向别处，不指向这行）⇒ 脚本先用 `^s*<包名>s*(,|:=|=)` 扫一遍，命中就改名 `pointsSvc`（本波 6 个文件）。
- **注册函数收窄签名会把 in-package 契约测试的调用点一起改**：`RegisterRoutes(rg, rd RouterDeps, svc)` → `RegisterRoutes(rg, rd.Session, svc)` 之后，`internal/api` 里 6 个契约测试的 `deps.RouterDeps()` 要改 `deps.RouterDeps().Session`（8 处）。「契约测试留在 `internal/api`」（第 4 节）不等于搬注册函数时可以不看它们 —— 它们测的正是注册入口。
- **顺带一条（非脚本）**：改注释/加包文档也要走一遍 `gofmt`；包文档与文件注释的逐字稿先写进计划文件再落盘，比在实现期现编省一轮核对。

第七批（P2 波 1c：完整域 `inspection`，3 个 `git mv` + 零跨包共享件）再添三条：

- **改写脚本改用 Node/TypeScript 的 `fs` 写，比 pwsh 稳**：一支脚本只需两个原语 —— `edit(rel, fn)`（读 → 改 → **有差异才写**）与 `sub(s, from, to, tag)`（片段没命中即 `throw`）。`throw` 发生在 `writeFileSync` **之前** ⇒ 任一片段没命中就整文件不落盘，不会留半成品（本波 `domains.go` 因搜索串多带一个引号被拦下，改对搜索串重跑即好）。pwsh 的单引号不插值、反引号截断、`[char]10` 拼行那几类坑一次性绕开。
- **搜索串必须逐字核原文，不能凭「应该长这样」写**：本波三处坑全是空白与标点 —— `internal/api/deps.go` 的字段行是 `InspectionSvc        *inspection.Service`（名字后有对齐空格）、`internal/apitypes/domains.go` 的 Roots 条目缩进是 **3 个 tab**（不是 2 个）、同文件注释行尾**没有**引号。⇒ 先 `read` 或 `grep -n` 把原文打出来（只锚定不含空白的片段）再拼替换串。
- **别整体跑 `goimports -w`：它列出的噪声文件与本次改动无关**：`goimports -l internal pkg cmd` 在本波恒列出 15 个文件（`cmd/server/main.go`、`internal/api/job_card_contract_test.go`、`internal/api/routes_registry_test.go`、`internal/cache/cache_test.go`、`internal/aiassistant/*.go` 等），是既有的三方 import 分组偏好，与搬包无关（波 1b 曾为此回退 16 个文件的纯 import 改动）。**判据反过来用**：自己按字母序插的 import 只要**不出现**在那份 `-l` 列表里，就说明分组与排序都对 —— 本波 7 个改动文件全不在列表里。

第八批（P2 波 2a：两个完整域 `featured` + `checkin` 同波搬包，8 个 `git mv` + `ForumAuthor` 跨域共享件迁 `internal/model`）再添四条：

- **`internal/apitypes` 的测试读的是生成产物 ⇒ 顺序必须是「先生成链、再跑测试」**：改完 DTO 的包前缀后直接跑 `go test ./internal/apitypes/` 会报两条**骗人的红** —— `codegen_test.go` 说「swagger definitions 里没有 `checkin.CheckInResult`」（生成物还是旧的），`nullability_lock_test.go` 说「nonnil 证据表里的 X 并不是一条射程内声明 nonnil 的字段」（证据是新的、被扫的 swagger 是旧的，看着像字段改名）。正解：`swag init -g cmd/server/main.go -o docs` → `go run ./cmd/gen-apitypes` → 再跑测试（本波两把红随即全绿）。
- **域包 HTTP 出口要注入「不属于本域的信封单点」时，用公共函数类型当形参，别把助手搬进域包、更别复制**：featured 的 Vditor 上传（`backend/internal/api/vditor_upload.go`，与 tutor 域共用）不搬包 —— 域包声明 `type VditorUploader func(c *gin.Context, fileSvc *filestore.FileStore, saver func(content []byte, filename string) (string, error))`，`RegisterRoutes(rg, session, svc, fileSvc, uploader)` 收第 5 形参，装配根 `routes_registry.go` 直接传函数值（零包装、零新机制，属第 7 节「注入」出口）。注意 `internal/api/vditor_upload.go` 里那个具名类型 `vditorUploadSaver` 要**整块删掉**（形参改无名后它会变成未使用符号，`unused` 只在 CI backend-lint 报）。
- **跨域共享的响应 DTO 贴实体进 `internal/model`**（本波 `ForumAuthor`：论坛域与打卡域都发它）：`model` 早已在两把 fact 锁的射程内（`ResponsePackages()` 含 `{"internal/model","model"}`、`nullability_lock_test.go` 的 `sweptDirs` 含 `"model": "../model"`）⇒ 不需要任何新机制，swagger 键从 `service.ForumAuthor` 变 `model.ForumAuthor`；代价是 model 里首次出现「带 json tag 的响应 DTO + 展示名方法」（Go 的方法不能与类型分居，故 `DisplayName()` 随之搬）。前端仍零 diff（`tsName` 剥包前缀）。
- **证据表键随域换**：域包新建 `nonnil_outlets_test.go`（键 `featured.FeaturedContentPageResult.items` / `checkin.CheckInCalendarResult.days` …）的同时，`internal/service/nonnil_outlets_{catalog,people}_test.go` 里 `service.<域>…` 的键与对应 outlet 函数要**删掉**（不是复制），段头留一行「本域的举证已随域包搬去 `internal/<域>/nonnil_outlets_test.go`（ADR-0070）」。漏删的症状：旧键指向已搬走的类型 ⇒ 报「并不是一条射程内声明 nonnil 的字段」。同理，随域搬走的测试文件第 1 行往往**就是** `package` 子句（无前置换行）⇒ 搜索串不能带 `\n`。

第九批（P2 波 2b-1：第六个完整域 `contribution`，6 个 `git mv` + 出边天然干净）再添五条：

- **`git mv` 不会创建目标目录**：`git mv backend/internal/service/contribution_service.go backend/internal/contribution/service.go` 在目标目录不存在时报 `fatal: renaming 'backend/internal/service/contribution_service.go' failed: No such file or directory` ——**报的是源路径**，看着像源文件不存在。先 `New-Item -ItemType Directory -Force backend/internal/<域>`（计划里写的「目录由 `git mv` 自动创建」是错的）。
- **一域两蓝图 ⇒ 注册函数一拆二之后，契约测试的测试路由器要把两个注册函数都调上**：只调 `RegisterRoutes` 时管理端端点**静默 404**，症状是「学员访问审核端点应 403, got 404」（`internal/api/contribution_contract_test.go:155`）——不是权限回归，是路由没注册。
- **抽 helper 的 `extractFunc` 必须做花括号配对**：只按「遇到独立一行 `}` 停」会把单行函数（`func itoa(n int) string { return strconv.Itoa(n) }`）连同**下一个函数**一起吞进来，症状是 `go vet` 报 `undefined: QuestionBankService`（被吞进来的函数引用了别处的符号）。helper 依赖还是**传递的**（`assertShapeLock` 调用 `topLevelKeys`）⇒ 追加要迭代到不动点。
- **拆出去的 handler 文件要按实际引用算 import**：`handler_admin.go` 里没有 `errors` 引用了，留着就是未使用符号（`unused` 只在 CI 的 backend-lint 报，本机 `golangci-lint` 不可用）⇒ 抽完文件先 `go build`/`go vet` 再逐组核 import。
- **`git grep` 的路径口径是仓库根**：worktree 里后端路径要写 `backend/internal/...`；只写 `internal/...` **静默零命中**，会被误读成「无残留」。本波差点据此提前收工。
- **本机 `gofmt` 可能比 CI 宽松：判据要用 CI 口径的工具链**。本机 go1.27.1 的 `gofmt` **不再折叠顶层连续空行**，而 CI `backend-lint` 用 Go 1.26 的 `gofmt -l .`（`GO_VERSION: 1.26`）⇒ 本波 `internal/contribution/dto_shape_test.go` 手拼 helper 段留了两个空行，本机 `gofmt -l` 报干净、CI 红在 9 秒。本机已另装 CI 口径工具链：`%USERPROFILE%\sdk\go1.26.6\bin\gofmt.exe`（同目录还有 `go1.27.0`）⇒ 提交前跑 `& "$env:USERPROFILE\sdk\go1.26.6\bin\gofmt.exe" -l .`（workdir `backend/`）**并且**本机那份也要空，两手都空才算过。

第十批（P2 波 2b-2：forum 域搬包，5 个 git mv + HTTP 面一拆二 + 证据表随域走）再添七条：

- **19 处契约调用点要拆成两行注册，漏 RegisterAdminRoutes 时管理端 10 条端点静默 404**：产线路由与契约测试自建引擎是两套注册，routes_registry_test 的 expectedRouteCount 只护前者 —— 管理端漏注册时它照样绿。判据是 git grep -n "RegisterAdminRoutes" -- backend 的命中数（1 装配 + 1 定义 + 19 调用 = 21），不是测试颜色。
- **留驻 internal/service 的测试文件不得 import 域包**：内部/论坛 → 内部/服务 是生产包依赖，测试文件再加一条反向边就是 import cycle not allowed in test。本次的命中点是注销回扣用例（它直接构造 NewForumService），修法是**脱开域包构造**（直插赞行 + NewForumCounter 造数），不是把用例搬回。
- **两把 fact 锁的计数与 nullability 的三处登记必须同批**：ResponsePackages 加一项 ⇒ fact_reachability_lock_test 与 consumption_fact_lock_test 的 14→15 ＋ nullability_lock 的 nonNilEvidenceSources/sweptDirs 各加一条；漏 sweptDirs 时症状骗人（报「证据表里的字段不在射程内」，指向的却是刚建的域包证据表）。
- **拆出的 handler_admin.go 的 import 要按实际引用重算**：拆文件不是剪切粘贴字符串，新文件的 import 集合必须重新推（否则要么未使用 import 报错，要么漏 gin/middleware 直接编译不过）。
- **域包 HTTP 出口文件名必须 handler 前缀**（handler.go / handler_admin.go）：根 scripts/check-render-error-face.mjs 只看该前缀，起名 forum_routes.go 会被**静默漏扫**。
- **纯委托 handler 会躲过「管理端有没有用 svc」的静态自查**：`func (h *handler) AdminListTopics(c *gin.Context) { h.ListTopics(c) }` 这种一行委托里**不出现 `h.svc`**，只 grep `h.svc` 会得出「管理端只依赖 modSvc」的错误结论；症状是管理端 10 条里的读端点 panic 被 recovery 成 500（`{"code":500,"message":"服务器内部错误"}`），而公开端点全绿。判据不是 grep，是**逐条跑管理端契约**或按「委托链的终点用了谁」倒推形参。
- **生成链之前先留三产物快照，比对前先把旧产物 CRLF 归一**：否则 docs.go/swagger 的假 diff 会淹掉真正的 10 个键改动；判据用行多重集/键序无关深比（ADR-0070 :152），不用裸 diff 的目测。

第十一批（P2 波 2c：AI 域 `aiAssistant` 整包搬进 `internal/aiassistant` —— 11 个生产文件 + 3 个 HTTP 面文件级三拆 + 12 个随域测试 + 证据表随域走）再添四条：

- **2c aiAssistant**：① 三份 HTTP 文件是**整文件**归位（`internal/api/settings.go` 通篇是 /admin/ai-configs*），所以「一拆二切点」这一步没有行号可抄 —— 真正的坑在**注册函数的守卫归属**：管理端注册函数体内没有 CapabilityRequired（守卫在调用方 `internal/api/admin.go:39` 的组上），若不显式登记 authz 覆盖锁的 allow 表就当场报红（`aiassistant.RegisterAdminRoutes`）；② `git grep -c "data=service\."` 匹配不到 `data=[]service.` 的形状，本波真实是 **12 处**而不是 5 处 —— 判据要写 `-E "data=\[?\]?service\."`。
- **「移动端 0 命中」是猜出来的判据，跨端其实有活引用**：`training-app/叉车维修培训学员端跨端应用/utils/aiFeatureRegistryParityContract.test.js:35` 的 `REGISTRY_PATH` 直读后端注册表源码（搬迁后必红），另有 3 处注释路径（`pages/ai-assistant/ai-assistant-constants.uts:13`、`utils/aiSourcesDisplay.test.js:22` 与 `:219`）同批同步。`.test.js` / `.uts` 在低风险白名单内可以改；**`*.uvue` 不在白名单**（`pages/ai-assistant/ai-feature.uvue:185/191` 的旧路径注释故意不改，否则整个 PR 降级为常规运行时面 —— 真机 + 微信开发者工具双双必过）⇒ 判据要实跑 `git grep -n "internal/service/ai_" -- training-app/`（期望 0）。
- **`git grep "internal/service/ai_"` 抓不到裸文件名残留**：域包内与留驻文件注释里写的是兄弟文件名（`ai_feature_registry.go`、`ai_prompt_chars.go`、`ai_features_codegen_test.go`、`api/diagnosis.go` …），只能按「旧名逐个人工替换」再扫一遍；本波另扫出 **14 处**，且 `backend/internal/deploy/topology.go:5` 这类不在 docs 排除面内的代码注释会被残留判据命中 ⇒ 计划里判「不改」的条目要以判据为准、当场改掉。
- **「零改动」的结论要用判据复核**：计划说留驻的 `backend/internal/api/ai_assistant_contract_test.go` 零改动（依据是 `git grep -c "service\." ` = 0），实际它有 **8 行注释**写着当前契约形状（`data=[]service.ModelOption`、`data=service.AIChatSessionDTO` …）⇒ 搬包后注释与注解不一致，只能同批改掉。教训：侦察子代理给出的「零改动」要拿 §11 的 `data=\[?\]?service\.` 判据实跑一遍，别直接采信。

第十二批（P2 波 3a：认证域整包搬成 `internal/auth` —— 14 个生产文件 + 8 个 HTTP 面 + 10 个随域测试；首个「出边不为零但单向无环」的交付）再添五条：

- **`config.Config` 字面量绕过 `Load()` 的默认值，D1 会话同步之后才现形**：`internal/api/account_change_contract_test.go` 与 `internal/api/code_auth_test.go` 自建 `cfg := &config.Config{JWTSecretKey: "test-secret", AuthCookie: …}` 时没给 `JWTExpiresHours`（默认值只在 `internal/config/config.go:362-363` 的 `Load()` 里给 2h/7d）⇒ `config.go:577 func (c *Config) JWTExpiry() time.Duration` 返回 0 ⇒ `internal/security/session.go:216 issue()` 签出的 access 是 `ExpiresAt = now`，域包服务签发全成 401（症状：`/api/auth/account/send-code` 回 `{"code":401,"message":"Token无效或已过期，请重新登录"}`）。修法分两层：① 字面量补 `JWTExpiresHours: 2, JWTRefreshExpiresDays: 7`（与 `Load()` 默认一致）；② 根因侧加固 —— `config.go:577 JWTExpiry()` 与 `:582 JWTRefreshExpiry()` 对 `<= 0` 按 `Load()` 的同一口径回退 2h/7d（环境变量那条路走 `config.go:600 positiveInt`，`<=0` 即回退默认值，到不了 0；能到 0 的只有字面量，所以生产行为不变）。**这条坑的首跑在 CI**：`internal/api/account_deletion_postgres_contract_test.go` 的两条用例走真实路由面 + 真 access，而 `testutil.NewPostgresDB` 在本机缺 `DATABASE_URL` 时干净 skip ⇒ 本地 `go test ./...` 全绿、CI 的带 PG 的 backend-test 判红。故另加一条不需要 PG 的本地钉子 `internal/api/contract_deps_access_guard_test.go`（`TestContractDepsAccessIsNotExpired`：走 `newContractDeps(t, db, nil)` 的同一条装配链 + 真 `middleware.JWTAuth`，断言不是 401）；变异检验：把回退条件改成 `if false` ⇒ 该用例判红，且 body 与 CI 逐字相同（`{"code":401,"message":"Token无效或已过期，请重新登录","data":null}`）。判据是「测试自建 cfg」这一点，不是 token 本身。
- **`middleware.JWTAuth` 不认已有的 `CtxUserID`**：`internal/middleware/middleware.go:113 resolveClaims` 只认 `Authorization` 头或 refresh cookie ⇒ 计划里「把注入 user-id 的中间件上移一层」过不了闸；直打 `/api/auth/account` 的契约测试必须签真 access（`sess.IssuePair(...)` + `codeAuthRequest(..., token)`），而不是注入上下文。
- **swagger 注解的裸名随文件走，只有跨包引用才要前缀**：注解搬进域包后 `data=LoginResult` 由 swag 按本包解析成 `auth.LoginResult`（本波 13 枚键以无前缀形态出现在 `internal/auth/handler*.go` 的 18 处）；留驻 `internal/api` 里原有的跨包注解保持 `data=auth.X`（5 处）；改完还要同步 `internal/apitypes/domains.go` 的 Roots（本波 `api.GenerateCaptchaDTO` → `auth.GenerateCaptchaDTO`），否则 `go run ./cmd/gen-apitypes` 报 `swagger definitions 里没有 "api.GenerateCaptchaDTO"（注解改名？声明表未同步？）`。
- **authz 覆盖锁要给公开注册函数留豁免位**：`internal/auth/handler_admin.go` 的 `RegisterAdminRoutes` 体内有 `middleware.CapabilityRequired(authz.CapProfileReview)` 无需登记，但 `auth.RegisterRoutes`（公开登录/刷新/登出面）体内没有 ⇒ 必须新增 allow 表项（键形如 `auth.RegisterRoutes`；非 `internal/api` 目录取 `path.Base(src.Dir) + "." + fn.Name`，射程 = `testutil.HTTPSurface`）。
- **大域的出边不为零也能交付 —— 红线是无环，不是零出边**：auth 是最大的域（20 枚出边），处置是「留驻面 + 包级包装」：不能进域包的共享件留在 `internal/service` 并导出（`mailer.go` / `identity_vocab.go` / `account_gen.go`），方法形态的共享件改成包级函数让域包包装调用（`password_write.go` 的 `SetNewPassword` → `ApplyHrwaiPassword` / `ApplyRecruiterPassword`）。这类会话一致性改动（D1）的验收手段是**变异检验**：把 `internal/security/session.go:272 RevokeIdentity` 改成首行 `return nil`，`go test ./internal/api/ ./internal/auth/ -run 'Revokes|吊销|旧refresh' -count=1` 应有 11 条判红（api 8 + auth 3），恢复后转绿。
第十三批（P2 波 3b-1：课程域整包搬成 `internal/course` —— 6 个生产文件 + 1 个 HTTP 面 + 7 个随域测试，另拆出两个新叶子包）再添六条：

- **行级盲替换会改坏字符串字面量与注释里的 URL**：为避包名遮蔽（局部变量 `course` → `crs`）而做的词边界全包替换，把 `"/api/course/%d"` 改成了 `"/api/c/"`，注释里 `/api/course/:id` 也一起变 —— 共修回 11 处（`internal/api/course_read_visibility_contract_test.go` 6 处、`internal/api/student_courses_contract_test.go` 5 处）。教训：机械替换要么先剔除字符串与注释，要么改完用 `git grep -n "/api/c/"` 反向验伤；**域包名是通用名词时（`course` / `points` / `auth`）遮蔽冲突必然出现，先规划局部变量名再动手**。
- **域包测试不能 import `internal/service` 的测试助手 ⇒ 夹具逐字复制、用例按「接缝是谁」拆**：`seedVisibleCourse` / `seedChapterWithMeta` / `seedCatalogCourse` 三枚夹具在 `internal/course` 与 `internal/service` 各留一份逐字副本（两包测试互不 import）；`internal/course/chapter_detail_test.go` 里导师侧 4 条用例拆回 `internal/service/tutor_chapter_detail_test.go`（接缝是 `TutorService`），课程 CRUD 3 条（`TestCourseSortOrder` / `TestAdminCourse_TrainingFields` / `TestCourseService_TrainingFields`，接缝是域包的 `AdminService`）反向搬进 `internal/course/admin_service_test.go`。拆散的两侧都是新建文件 ⇒ 规模按**全文**计入手工面口径。
- **非 nil 证据表：证据跟「真实出口」走，不跟类型名前缀走**：`course.ChapterDTO.files` 只由 `TutorService.GetCourseChapters` 填、`course.CourseDTO.chapters` 只由留驻的 `TrainingCatalogService` 的 withChapters 分支填 ⇒ 这两枚键的证据留在 `internal/service/nonnil_outlets_course_test.go`（锁只认键字符串、不认目录归属）；域包表收 6 枚（`CoursePageResult.courses` / `AdminCourseDetailDTO.chapters` / `CourseDetailDTO.chapters` / `ChapterDetailDTO.files` / `CourseDTO.prerequisites` / `CourseDTO.prerequisite_course_ids`，后两枚共用一个 outlet —— 同一 outlet 可给多键举证，但一把钥匙只能有一处证据）。
- **多文件拆散的重命名件相似度会掉到 57%~83%**：`git diff -M` 下这些搬运件仍按改写行计入 diff，不是「纯重命名计 0」⇒ 规模估算别按纯重命名外推。本波实测手工面 **1698 行**（口径 = 总插入 − `backend/docs` 插入），超手册 §10 的 1500 上限 198 行，超出部分全部是两处新建证据表（229 行）、两个新叶子包（123 行）、`internal/course/batch_backfill.go`（79 行）与两处随域/回迁用例（450 行）。
- **计划外的「助手搬回自己的域」要把方向写进 PR**：三枚批量加载器（`BatchChapterCounts` / `BatchPrereqIDs` / `BatchStudentCounts`）从留驻 `internal/service/batch_backfill.go` 搬进 `internal/course/batch_backfill.go`，留驻侧写限定名 —— 这是 `service → course` 的**单向边**（与 auth 波同型）；反向 `course → service` 必须仍是 0 命中，这才是硬判据。
- **工具行号差 1**：`read` 工具报的行号比 Node `fs.readFileSync(...).split('\n')` 的下标**大 1**（read 说 `:649` 的注释，数组下标 649 = 第 650 行；`totalLines=938` 而 split 长度 939）⇒ 用行号做切片前先 print 数组元素核对锚点。
- **跨端直读后端源码的用例要全量 grep，不能照计划点名单**：3b-1 只按计划改了 `training-app/叉车维修培训学员端跨端应用/utils/coursesContract.test.js` 的锚点，漏了同目录 `utils/chapterCompleteContract.test.js:160` 直读 `path.join(ROOT, '..', '..', 'backend', 'internal', 'service', 'course_service.go')` 的用例 ⇒ PR #1470 的 `mobile-test` 首轮红（`Tests: 1 failed, 2821 passed`）。判据：搬包前把 `training-app/**` 与 `frontend/**` 里所有 `path.join(...)` 含 `backend` 的调用解析成绝对路径逐一 `existsSync`（本次 9 处，唯一功能性命中即此处；`.uts` / `.uvue` 里的旧路径注释按「只披露不改」留着 —— 改它们会把 PR 拉进运行时面）。
第十四批（P2 波 3b-2：培训域搬成 `internal/training` —— 5 个生产文件 + 1 个 HTTP 面文件级三分 + 5 个随域测试，另新建一个计划外叶子包）再添九条：

- **搬用例要按「用例集」切，不能按行号连续区间切**：`internal/service/envelope_dto_shape_test.go` 的 `206-375` 是连续区间，却混着 7 条例域用例（`RecruitMeDTO` / `ProgressSaveResultDTO` / `notification.NotificationUnreadCountDTO` / `QuestionCommentPageResult` / `aiassistant.AISessionRenameResultDTO` / `aiassistant.AIImageUploadResultDTO` / `QuestionImageUploadDTO`）⇒ 整段切当场报 `undefined: RecruitMeDTO`。正解是 training 只取 `206-215` + `261-375`（13 例，自带 `TestInlineResponseDTOBytes` / `intPtr` / `int64Ptr`），留驻侧保留 `1-205` + `216-260` + `376-429` 并删掉搬走侧专用的两个 helper。判据：清单点名的**用例集**为准，搬完用 `go vet` 逐条点名缺失符号。
- **计划外的新叶子包要在同一波里当场立，别用「就地复制」绕过**：`internal/training/catalog_service.go` 的标签题目数计数需要原 `internal/service/question_pool_scope.go:28-37` 的三枚片段，而该文件自述「raw SQL 计数必须引用它们、不得就地重写」，training 又不得 import `internal/service`（反向边 service → training）⇒ 新建 `internal/questionpool`（第三种破法），留驻 service 侧改引用限定名。**计划里「本波不新建叶子包」不是判据，出边为零 / 无环才是**；偏差要写进 PR 正文与 ADR。
- **包私有表被留驻侧复用时要升导出，不许抄字面量**：`internal/api/admin.go:242` 的 `courseSortFacts400` 引用随 handler 搬进域包的 `sortFacts400` ⇒ 域包侧改名 `SortFacts400`（导出）＋同包四处调用点同步（`handler_admin.go:576`/`:600`/`:745`、`handler_credential.go:213`），留驻侧写 `append(append([]error{}, training.SortFacts400...), …)` 保住「目录侧两条只有一份出处、第一个 append 落进新 backing array」。就地抄两元素＝第二真源，否决。
- **nullability 锁的报红形态是「键在射程内声明 nonnil 却没有出口例」，而且会在下一波才现形**：3b-1 把 `course.CourseDTO.chapters` 与 `training.CatalogLevelNode.courses` 的证据按「生产者是谁」留在 `internal/service/nonnil_outlets_course_test.go`，本波 `sweptDirs` 加入 training 之后 `internal/apitypes` 的 `TestResponseCollectionsMustDeclareNullability` 判红（`nullability_lock_test.go:577`「training.CatalogLevelNode.courses 声明 nonnil 却没有发 [] 的出口例」＋`:589`「非射程内字段」）⇒ 两格连 outlet（`OutletAdminCatalogCourseNode` / `OutletCatalogLevelNodeNoCourses` ＋本地 `seedVisibleCourse` 夹具）搬进 `internal/training/nonnil_outlets_test.go`，留驻表只留 `course.ChapterDTO.files`。**证据的归属会随波次再平衡：每加一个域的 `sweptDirs`，就把「该域声明的键」全部对一遍出口例。**
- **测试编译会抓到「搬去域包的 helper 留驻侧还在用」，`go vet` 在生产面上抓不到**：三处就地内联 —— `internal/service/real_exam_service_test.go` 补 `mustListQuestionTags`、`internal/service/nonnil_outlets_course_test.go:72` 的 `newCatalogSvc` 换成 `testutil.NewMemoryDB(t)` + `training.NewService(db, zap.NewNop())`、`internal/service/practice_mode_service_test.go` 尾部补 `p16`。**`p16` 必须放在 import 块之后** —— 紧贴 `package service` 写会让 `gofmt` 报 `imports must appear before other declarations`。
- **留驻测试里的裸构造名要整批改限定名**：9 处 `NewTrainingCatalogService(`→`training.NewService(` 与 15 处裸 `QuestionTagInput{`→`training.QuestionTagInput{`；后者用后置断言 `(?<![\w.])QuestionTagInput\{` 一次收完（绕过 `service.QuestionTagInput{` 形态），改完按文件分别计数复核。
- **前端生成物可能是真的 0 diff**：3b-1 是纯声明顺序 diff（`admin.ts` / `training.ts` / `tutor.ts` 共 +117/−117），本波 `go run ./cmd/gen-apitypes` 重写 15 个文件后与 HEAD **逐字相同**（TS 发射器剥包前缀，注解里 `data=service.X`→`data=training.X` 到不了前端）。判据是 `git diff --stat -- frontend` 的结果，不是「生成链跑过必然有 diff」。本波手工面 **1 402 行**（口径 = 总插入 − `backend/docs` 插入；54 文件 +2 862/−2 490，其中 `backend/docs` +1 460/−1 460、前端 0）—— 在手册 §10 的 1500 上限内。
- **「陈旧路径全量扫描」要能区分真引用与守卫自测的合成样本**：判据是「把 `training-app/**` 与 `frontend/**` 里所有含 backend 的字符串字面量解析成绝对路径，对 file-dir / app-dir / worktree / frontend 四个基准逐一 `existsSync`」；实测 63 条 `.go` 引用里 9 条无解，全部是 `scripts/check-catalog-sort.test.mjs` 的 `faq_service.go`、`scripts/check-render-error-face.test.mjs` 的 `api/forum.go` 这类**合成负样本**与文档历史引用；另按「`path.join` 含 `backend` 段」全扫的 7 处（`chapterCompleteContract.test.js:160` 等）逐一 `existsSync` 全部存在 ⇒ 本波无 3b-1 那类漏网。结论要写「按什么判据扫出几条、逐条为什么不是真引用」，不能只写「已全量扫描」。


- **跨包复制测试 helper 要按调用点裁剪，别整块拷**：`internal/training/dto_shape_helpers_test.go` 从 `internal/service/dto_shape_test_helpers_test.go` 整块拷了三件，本包其实只调用 `topLevelKeys` / `assertShapeLock` ⇒ 多出来的 `marshalJSON` 只有 CI `backend-lint` 的 `unused` 抓得到（本地 `go build` / `go vet` 都不报「未使用」）；对称地，envelope 13 例搬走之后留驻 `internal/service/envelope_dto_shape_test.go:14` 的 `intPtr` 成了孤儿，同样只在 CI 现形。修法：复制 helper 后先数一遍「剥掉注释的包内调用点」，零调用的当场删；波次收尾用 §8 自查清单第 1 条把新包与留驻包各过一遍。

```
