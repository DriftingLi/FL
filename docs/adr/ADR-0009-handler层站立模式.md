# ADR-0009: handler 层站立模式——struct 方法 + typed DTO 契约

- 状态：已接受（2026-08-10，spec #110 架构深化第二波 / tickets B1~B10）
- 领域：后端主体系 —— HTTP handler 层与 service 返回值形状

## 背景

主体系 handler 层长期以「内联闭包注册」为主形态：`RegisterXxxRoutes(rg, deps)` 内部逐个 `g.GET("/path", func(c *gin.Context) { ... })`。业务增长后暴露三类问题：

- **不可单测**：闭包无法独立构造与调用，行为只能透过整条路由链路覆盖；单个 handler 的输入输出无法直接断言；
- **装配根泄密**：注册函数一律吞整个 `*Deps`（26 字段），蓝图与无关依赖耦合，测试装配必须构造完整 Deps；
- **契约无锚点**：service 层大量 `map[string]any` 返回，key 拼写散落在构造点，前端契约只能靠运行时观测，无法静态冻结。

同时 service 层存在一批历史遗留的 `xxxToDict` 手写 mapper（session/participant/course/chapter/student 等），与 DTO 构造逻辑分置两处，改造时容易漂移。

## 决策

后端 handler 层收敛为**站立模式**：

### 1. handler：struct 方法 + 注册函数只做装配

- 每个蓝图一个 `XxxHandler` struct（持有所需 service 引用），路由方法挂在 struct 上（`func (h *XxxHandler) List(c *gin.Context)`），`RegisterXxxRoutes` 只做路由注册，不再内联业务逻辑；
- 蓝图注册函数**按需注入依赖**（`RegisterXxxRoutes(rg, sess, svc, ...)`），不再吞整个 `*Deps`；装配根仍是 `NewDeps` 单一组合根，`router.go` 只负责把 Deps 字段分发到各注册函数。

### 2. service：typed DTO 取代 map 返回

- service 层返回 `map[string]any` 的方法收敛为导出 typed DTO struct（如 `LevelExamSessionDTO`、`CourseDTO`、`MockExamSubmitDTO`）；
- 手写 mapper（`sessionToDict`/`courseToDict`/`chapterFileToDict` 等）折叠进 DTO 构造方法；
- **JSON 字段名与既有 map key 逐字一致（最高优先级约束）**：可选字段用指针（如 `*int64`、`*[]int`）区分「未填充（省略）」与「零值/空数组（存在）」两种状态，保证 null/[]/absent 三态与旧 map 行为完全一致；
- 每个蓝图落地时补 shape-lock 测试，断言顶层 key 集合与旧契约完全一致，字段声明按 key 字母序保持与旧 map 序列化的字节序一致（byte-level 契约可选断言）。

## Trade-off

- **闭包注册**：代码量小、上下文就近，路由与实现同屏；但不可单测、依赖面宽、逻辑随业务膨胀后难以拆分。适用于真正一次性的薄适配（如 `serveReportGenerate` 这类纯翻译壳仍以函数形式保留）。
- **struct 方法 + typed DTO**：每个 handler 可独立构造与单测；service 返回值静态定型，IDE 与编译期即可发现契约破坏；但样板变多（struct 定义 + 构造器 + 字段注解），且 DTO 与 map 行为存在细微三态差异需要指针字段显式建模——这是「类型安全」换「简洁」的取舍，以 shape-lock 测试兜底契约不漂移。

## 与既有决策的关系

本决策是 ADR-0005（统一响应信封）与 ADR-0008（描述符驱动管理面）的契约类思路一脉相承的收尾：信封统一了响应外壳，描述符冻结了字典面契约，本 ADR 把同一「契约显式化、可静态断言」的原则推广到全部业务蓝图的 handler/service 层。过程中产生的支撑性收敛：Session 单例装配（B2）、报告协调器 gin 脱耦下沉 `internal/valuation/report`（B3）、helper 间接层折叠（B1）均为本模式的前置清理。

## 相关

- `backend/internal/api/*.go`（handler struct 化）、`backend/internal/api/deps.go`（按蓝图收窄的装配根）
- `backend/internal/service/*_dto_test.go`（shape-lock 测试）
- ADR-0005（响应信封统一）、ADR-0008（字典描述符驱动的管理面）
