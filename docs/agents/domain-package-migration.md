# 域包迁移手册

> 读者：把后端一个功能域从 `internal/service` + `internal/api` 搬进 `internal/<域>/` 的实施者。
> 每次搬家前读第 2、3 节；搬完逐条走第 9 节的验收清单。
> 定案：[ADR-0070](../adr/ADR-0070-域包形态与目录即射程的收口.md)。首个样板：faq（2026-10-01，P1 试点批）。

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
- 域包允许依赖：`internal/model`、`internal/service`（共享助手，P3 收进 `internal/core`）、`internal/security`、`internal/middleware`、`pkg/httpx`、`pkg/response`、第三方。
- 域包**不得** import `internal/api`（装配根在那里，反向依赖成环），也不得 import gin（HTTP 形状归 `handler.go`，见第 3 节）。
- 不新建 `test/`、`tests/` 目录（`internal/layers` 的同居规矩）。

## 2. 迁移 recipe（以 faq 为样板）

1. **定域边界**：在 `backend/internal/apitypes/domains.go` 找到该域的 `Roots`（带 Go 包前缀的 swagger 键，如 `service.FaqResult`）。
2. **共享件先出包**（否则会造出没有消费者的导出 API）：
   - 域实现用到的**同包非导出助手**，先在该包里导出、调用点机械改名、原包不退场（faq 批：`BeijingNow`（`internal/service/auth_service.go:966`）、`IsDuplicateError`（`internal/service/forum_counter.go:67`），119 处 / 37 文件）；
   - **解析类**助手升级到 `pkg/httpx`（先例：`httpx.QueryIntPtr`，票 #1452）。
3. **搬服务**：`git mv backend/internal/service/<域>_service.go backend/internal/<域>/service.go`；改 `package`；`XxxService`→`Service`、`NewXxxService`→`NewService`；共享助手写限定名（`service.BeijingNow`）。DTO、哨兵、校验函数名原样不动。
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
| `backend/internal/apitypes/nullability_lock_test.go` | `nonNilEvidenceSources` 加 `{"internal/<域>","nonnilOutlets"}`；`sweptDirs` 加 `"<域>": "../<域>"` | 域包里声明的 `nonnil` 找不到证据（判据 5 零容忍）／旧键变幽灵键 |
| `backend/internal/api/authz_coverage_lock_test.go` | `allow` 表**键是裸函数名**（12 条）；域包把注册函数改名 `RegisterRoutes` 后裸名会**静默失配** ⇒ 键改限定名（`notification.RegisterRoutes`） | 红（该表只在被点名函数上生效，改错名字即从判据里消失） |
| `backend/internal/api/path_parse_point_drift_lock_test.go` | `queryHelpers` / `allowedPathParseFuncs` 等按名字圈射程的清单（#1452 已允许限定名 `httpx.QueryIntPtr`） | 域包自带解析助手时判据看不见（**静默**）：解析类一律走 `pkg/httpx` |
| `scripts/check-render-error-face.mjs` | 无需改：`GUARDED_FILE_PREFIXES=['handler']` + `GUARDED_FILE_ROOT='backend/internal/'` 自动纳入域包 `handler*.go` | 若 HTTP 出口文件**不以 handler 开头**（如 `admin.go`），判据看不见（**静默**）⇒ 命名必须带 handler |
| `backend/internal/layers/layer_guard.go` | 无需改：gin 只许出现在 HTTPSurface（含域包 `handler*.go`）与四个登记目录（`internal/middleware`、`internal/logger`、`pkg/httpx`、`pkg/response`） | 域实现（`service.go`/`dto.go`）里 import gin 即红 |
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
2. **升级到 `pkg/httpx`**：HTTP 形状与请求解析（`ParseError` / `BadRequest` / `PathInt` / `PathInt64` / `QueryIntPtr` / `Endpoint` / 错误状态表）。`pkg/httpx` 不得 import 任何 `internal/...`（`internal/layers` 判据 ①）。
3. **留在 `internal/service` 并导出**：域实现共用的时间/DB/格式化助手（`BeijingNow` / `IsDuplicateError` / `FormatISO` / `FormatTimePtr`），P3 随 `internal/core` 一并收编。

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
- [ ] `make swagger` + `go run ./cmd/gen-apitypes` 后：swagger diff 只有机械改名、`git diff --stat -- frontend` 为空
- [ ] 旧符号（`XxxService` / `RegisterXxxRoutes` / 旧包前缀键）全仓残留 = 0
- [ ] PR 正文一行披露「合并到 master 将触发 production 部署」

## 10. 波次与 PR 纪律

- 一波 1 个 PR、**≤3 个域**、diff ≤1500 行；域的顺序按 P1 计划公布的四波（faq / notification 是试点批）。
- 生成链顺序是硬的：gofmt/vet → `make swagger` → `go run ./cmd/gen-apitypes` → `go test ./...` → 前端检查（`internal/apitypes/codegen_test.go` 把前端生成物与注解渲染结果全等比对，顺序弄反会先绿后红）。
- 每个 PR 合并即触发 production 部署（master push ⇒ `cd.yml`）⇒ PR 正文必须留一行披露。
