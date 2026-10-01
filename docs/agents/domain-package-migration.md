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
   - **别域的事实常量**改成**调用方传参**：站内信事件构造器收 `reason string`（`NewContributionApprovedEvent(userID int, title string, contributionID int64, points int, reason string)` 等 7 个）—— 积分流水原因是积分域的事实，通知域只把它记进 payload，于是不必 import 别域或服务层的常量；
   - **跨域共享词汇**贴着实体放 `internal/model`（第 1 节）。
3. **搬服务**：`git mv backend/internal/service/<域>_service.go backend/internal/<域>/service.go`；改 `package`；`XxxService`→`Service`、`NewXxxService`→`NewService`；共享助手写限定名（先例 `service.BeijingNow`，P2 波 0a 收编后写 `clock.Now()`）。DTO、哨兵、校验函数名原样不动。
4. **搬 HTTP 出口**：`git mv backend/internal/api/<域>.go backend/internal/<域>/handler.go`；改 `package`；`XxxHandler`→`handler`、`NewXxxHandler`→`newHandler`、`RegisterXxxRoutes(rg, rd RouterDeps, svc)`→`RegisterRoutes(rg *gin.RouterGroup, <只收它真正需要的依赖>, svc *Service)`。faq 只用了 `rd.Session`，于是签名就是 `RegisterRoutes(rg *gin.RouterGroup, session *security.Session, svc *Service)`，由 `routes_registry.go` 的闭包注入 `rd.Session`。
   - **不要把 `RouterDeps` 搬进域包**：它住在 `internal/api/deps.go`。
   - swagger 注解的 `data=service.X` 改成 `data=<域>.X`。
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
| `backend/internal/service/nonnil_outlets_*_test.go` | 属于该域的键与 outlet 函数**删掉**（不是复制），头部注释里的域名列表同步 | `testutil.AssertNonNilOutlets` 报「同名键出现在两张表」 |
| `scripts/check-catalog-sort.mjs` 的 5 条 `declaredIn`、`backend/internal/deploy/nginx_delivery_gen.go:230` 的硬编码路径 | 只在被点名的文件真搬家时改 | 缺文件即 Fatal（不静默），会在 CI 点名 |

方法论：**改名会让按名字/路径判定的锁静默失配**。删或重构 Go 符号后，除了 grep 残留引用（本机 golangci-lint 跑不了，见 `checks.md` 环境 A），还要 grep 一遍**按名字判定的锁**（清单名、白名单键、前缀计数）。

## 4. 契约测试的归属

**契约测试留在 `backend/internal/api`**，不随域搬。理由：它走 `newContractDeps` + `NewRouter` 的全量装配链，而 `backend/internal/api/router_test_helper_test.go` 已明文反对另起测试装配链（「装配链分叉 = 测试装配与生产装配各测各的」）。faq 批实测：`internal/api/faq_contract_test.go` 只引用包内脚手架与 `NewRouter`，不引用任何 `service.Faq*` 类型 ⇒ 迁移后只改一处注释。

**域内单元测试与证据表随域搬**（`service.go` 的单测、`nonnilOutlets*` 表）。域包测试**不得**引用 `internal/service` 的测试脚手架——Go 的 in-package test 会成环；共享 runner 要提成 `internal/testutil` 的普通 `.go`（先例：`testutil/nonnil.go`、`testutil/codescan.go`）。

## 5. 响应 DTO、swagger 键与前端生成物

swagger 的 definitions 键**带 Go 包前缀**（`"service.FaqResult"`，`$ref` 写作 `#/definitions/service.FaqResult`）⇒ DTO 搬进域包后键变成 `faq.FaqResult`。这是**机械改名**，可验证：

1. `cd backend && swag init -g cmd/server/main.go -o docs`（本机 `swag.exe` v1.16.4，与 CI 钉的版本一致）；
2. `go run ./cmd/gen-apitypes`；
3. **验证 diff**：把 `HEAD` 版 `backend/docs/swagger.json` / `.yaml` 落到临时文件，对旧文本做一次纯文本替换（正则 `/service\.((?:Admin)?Faq[A-Za-z]*)/g`，替换成 `faq.<捕获组>`）再与新文件比：json 应**深度相等**（键序无关）、yaml 应**行多重集相等**；`docs.go` 里不应有 `service.Faq*` 残留。
4. **前端生成物应零 diff**：`tsName()`（`internal/apitypes/codegen.go`）只取最后一个点之后 ⇒ 包名不进 TS 类型名。`git diff --stat -- frontend` 为空即证。faq 批实测为空。

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
3. **留在 `internal/service` 并导出**：一时找不到更轻落点的 DB / 业务助手 —— 但要先试完前两条与「叶子包」。**三类曾经留在这里的共享件已在 P1 / P2 波 0a 各自归位**：时钟（`clock.Now` / `clock.DayStart` / `clock.DayKey` → `internal/clock`）、唯一冲突谓词（`dberr.IsDuplicateError` → `internal/dberr`）、时间格式化（`timefmt.FormatISO` / `timefmt.FormatTimePtr` → `internal/timefmt`，见第 2 节）。三者的共性是**无状态、不碰 DB、只吃参数**，所以本出口只该剩「真的要 `*gorm.DB` 或服务内部状态」的件，P3 随 `internal/core` 收编。数值 / 指针助手与 `json_helpers.go` 这类的落点见第 10.1 节（波 0b）。

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
- [ ] PR 正文一行披露「合并到 master 将触发 production 部署」

## 10. 波次与 PR 纪律

- 一波 1 个 PR、**≤3 个域**、diff ≤1500 行；域的顺序按 P1 计划公布的四波（faq / notification 是试点批）。
- 生成链顺序是硬的：gofmt/vet → `make swagger` → `go run ./cmd/gen-apitypes` → `go test ./...` → 前端检查（`internal/apitypes/codegen_test.go` 把前端生成物与注解渲染结果全等比对，顺序弄反会先绿后红）。
- 每个 PR 合并即触发 production 部署（master push ⇒ `cd.yml`）⇒ PR 正文必须留一行披露。

### 10.1 共享叶子与跨文件共享件的落点（P2 判据：出边为零才能单独搬）

域包能不能单独搬，只看出边 —— 域实现引用的每个残留符号都会逼它 import `internal/service`（只要 service 里还有一处引用该域，就是 import cycle）。入边只决定 diff 大小与合批。拆波前先扫一遍**跨文件私有引用**（同包内 A 文件用 B 文件的私有符号；共享扫描器的正式出处是 `internal/testutil/codescan.go`，别在调用点手抄判据）。

| 件 | 落点 | 状态与说明 |
| --- | --- | --- |
| 时钟（`Now` / `DayStart` / `DayKey` / `Location`） | `internal/clock`（叶子） | 波 0a 已收编；`internal/service` 里的一行委托（`BeijingNow` / `startOfShanghaiDay` / `shanghaiDayStr`）全删，调用点写限定名 |
| DB 错误判定（`IsDuplicateError`） | `internal/dberr`（叶子，只 import `strings`） | 波 0a 已收编；双方言谓词测试随行 |
| 时间格式化（`FormatISO` / `FormatTimePtr`） | `internal/timefmt`（叶子） | notification 批收编 |
| 解析出口：查询侧 `QueryIntPtr` / `QueryIntDefault` / `QueryIDPtr` / `PositiveID`，路径侧 `PathInt` / `PathInt64`，以及 `ParseError` / `BadRequest` / `Endpoint` | `pkg/httpx` | 波 0a 尾款收编；`internal/api/helpers.go` 已删；漂移锁 `path_parse_point_drift_lock_test.go` 钉住两侧计数（宿主四枚、本包零枚），新增出口必须同步 `queryParseHelperNames` |
| credential 谓词（`RecordPartitionOf` / `PartitionBucket` / `EntityOwnedBy`） | `internal/scope`（波 0b） | 三族 nil 语义**相反**（记录冻结分区 nil 看全部 / NULL 桶 nil 只看 `credential_id IS NULL` / 归属分区 nil 看全部）⇒ 搬包时同步静态扫描锁的白名单路径（白名单就是谓词实现处那个文件） |
| 数值/指针助手（`toFloat` / `clampFloat` / `parseFloat` / `parseInt` / `ptrInt` / `floatPtr` / `containsString` / `withTimeout`） | `internal/coerce`（波 0b） | 同批删掉 `internal/service/json_helpers.go`（8 行纯转发 `jsonMarshal` / `jsonUnmarshal`，调用点直接用 `encoding/json`） |
| 文件存取 | `internal/filestore`（波 0c） | 未开工 |
| 跨文件的请求 DTO 类型（`idParam` / `taskIDParam` / `courseIDInput` / `chapterIDInput` / `swapCourseSortReq` / `generateContentReq`，声明在 `internal/api/admin.go:873-905`，被 `admin_recruiter.go` / `settings.go` / `tutor.go` 跨文件引用） | **各域包自己声明** | 它们本来就是各域的请求面（`struct{ ID int }` + 用 `httpx.PathInt` 的 Parse 闭包），不是解析出口 ⇒ 不进 `pkg/httpx`；拆包时随 handler 搬、同名保留 |
| 既要用 gin 又要用 middleware + service 的域内解析助手（`studentQuestionScope`，`internal/api/question_bank.go:105`） | **题库域包**（3c 波随 `internal/questionbank` 导出） | **不能进 `pkg/httpx`**：`pkg/` 不得 import 任何 `internal/...`（`internal/layers` 判据 ①），而它同时需要 `internal/middleware`（`CredentialIDPtr`）与 `internal/service`（`NewQuestionReadScope`）；调用它的 `favorite.go` / `note.go` / `question_interaction.go` 都在 3c/4b/4c 波之后 ⇒ 不阻塞波 1、2 |

**P2 波次（issue #1445 公布，2026-10-01）**：波 0 共享叶子 3 个 PR —— 0a 时钟 + DB 助手 + 解析出口（✅ 已交付）；0b `internal/scope` + `internal/coerce` + 删 `json_helpers.go`；0c `internal/filestore`。波 1：1a material / 1b points / 1c inspection。波 2：2a featured + checkin / 2b contribution + forum / 2c aiAssistant。波 3：3a auth / 3b course + training / 3c questionBank + practiceMode。波 4：4a mockExam + realExam / 4b student + favorite + search / 4c note + questionInteraction + wrongQuestion / 4d tutor + admin / 4e recruit + resume + job / 4f 收尾 audit + export。

**三个强环与破环手法**：course↔training、course↔tutor/points、resume↔recruit。五种破法 —— 跨域共享词汇贴实体进 `internal/model`、别域的事实常量改调用方传参、无状态纯函数进叶子包、下游要业务层类型走消费方接口反转（第 7.1 节）、助手搬回自己的域。

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
