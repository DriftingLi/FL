# ADR-0048: 前端契约 codegen —— 按域解冻与漂移处置

- 状态：已接受（2026-09-13；承接 ADR-0019 的推迟结论，落实 spec #940 之后的执行口径）—— 片一 #952 的实施修订见「实施修订」一节
- 领域：前端契约 / 代码生成 / 前后端联调口径

## 背景

ADR-0019 把「由 OpenAPI 生成前端类型」整体推迟，只留下方向：下一次前端契约专项以 gin-swagger 注解为唯一输入。spec #940（ADR 遗留项收口）交付了专项的**第一步**：输入面可信化（swagger 再生成 + CI 新鲜度锁）、窄域生成管道（`internal/apitypes` 声明表 + `cmd/gen-apitypes` + 字节级同步契约）、以及打卡域试点（3 个端点 / 6 个类型，前端删掉手写副本）。

试点把三个「开工前必须定死」的问题摆到台面上（数字均为实测）：

1. **覆盖口径**：注解面 197 个端点里只有 54 个指认了 `data` 类型，另有 172 个端点压根不在 swagger（`admin` 75 / `valuation` 65 / `question-bank` 12 …）；而 **Web 实测只消费 168 个端点**（30 个 api 模块）。全量补注解有一半劳动没有任何消费者。
2. **漂移处置**：生成类型与前端手写类型（合计 164 个 interface）**必然**对不上 —— 那正是 ADR-0019 的动因（字段名颠倒、结构与后端不符）。一刀切「以注解为准」会把「手写正确、注解写错」的情况改错；一刀切「以手写为准」则等于承认注解不可信，漂移被固化。
3. **完成标准**：只产出类型文件、前端继续用手写 interface = 两套事实源并存，等于没收敛。

## 决策

1. **目标：注解是前端类型的唯一事实源**（不是为了补文档质量）。判据是「前端手写响应类型被删掉」。
2. **覆盖口径：Web 消费面 + 域内补完整。** 边界取 `frontend/src/api/**` 实际调用的端点（实测 168 个 / 30 个模块）；**域一旦开做就补全该域** —— 域内「一半生成一半手写」比不做更糟。
3. **完成标准（每域）**：生成响应类型 → 前端 api 模块改为 re-export 生成类型、删掉手写响应 interface。**入参（query / body）与 UI 态类型不生成**（swag 对 body 描述弱；本仓多处 body 直接是 `map[string]any`）。
4. **漂移处置：逐条判定，以实际返回为裁判。** 差异按三类归档：① 注解写错（改注解）② 手写类型过时（改前端）③ 后端实际返回是第三种形状（真 bug：改后端并同步两边）。每片的**字段级差异清单**进 PR 验收证据。
5. **估值模块纳入，但独立成最后一片。** 它是 Web 消费面里唯一「整块零注解 + 平行 auth（public / optional / valAuth / admin）+ 独立 model 包」的区域（`internal/valuation/handler` 4,641 行、零 `@Router`）；混进主 `/api` 各域片会让口径与验收不一致，独立成片才能单独决定「只补注解」还是「连 7 处非统一信封也定 DTO」。
6. **handler 内联响应 map 一并收口（独立成片，排在 recruit / admin 之前）。** 响应面 9 处 / 6 文件定型为 DTO（**序列化字节不变 + shape-lock**），`recruitMe` 顺带迁到 `Endpoint` 骨架；**非响应面不动**：站内信 JSONB 落库 payload（改形状要迁移）与 SSE 事件 payload（不走信封）。
7. **分片：每片一个 PR、独立验收。** 顺序：`contribution + practiceMode + mockExam`（后端注解已就绪，先暴露「删手写副本」的真实摩擦）→ 内联 map 片 → `auth` → 互动面（forum / notifications / favorite / wrongQuestion）→ `recruit + job + resume` → `questionBank + tutor + training + search` → `admin`（最大片）→ `aiAssistant`（SSE 端点排除）→ `valuation`。
8. **不重开 ADR-0019 的「全量推迟」结论，而是按域解冻**：本 ADR 是它的修订与执行口径；ADR-0019 保留原文并指向本文。

## 实施修订（2026-09-13，片一 #952）

片一（`contribution` + `practiceMode` + `mockExam`）落地时暴露了决策 4 没覆盖的一类漂移 —— **可空态与缺省态是两件事**，补齐口径如下：

- **缺省态也要进注解层**：`extensions:"x-nullable"` → `T | null`（键一定在，值为 null；Go 指针且无 omitempty）；新增 `extensions:"x-optional"` → `T?`（键**可能整个不存在**；Go omitempty）。
- **为什么不能只靠 x-nullable**：`omitempty` 字段漏标时，生成物把前端手写的 `?` **静默升级为必填**，而 type-check 只抓「窄 → 宽」（消费处少判空），抓不到「宽 → 窄」——正是本 ADR 想消灭的那类静默漂移。
- **swagger 空 schema（Go `any`）渲染 `unknown`**：此前落到 object 分支渲染 `Record<string, unknown>`，对实际是 string / number / 数组的取值撒谎（`user_answer` / `answers` / `options` 等）。
- **片一实测**：三个域 30 个 Web 消费端点、25 个已指认 data、5 个有意无 data（撤回 / 举报 / 处置举报 / `POST /practice-mode/progress` / `POST /mock-exam/{id}/save`）；字段级差异逐条判定后**全部落在 ② 手写类型过时**（3 个凭空字段 `practice_mode` / `finished_at` / `score`、2 个漏字段、若干过时可空性），无一例 ① 注解写错或 ③ 后端第三种形状。清单与处置见 PR 验收证据。
- **已知限制（留后续片）**：题目类型 `QuestionDTO` 同时是跨域 UI 模型（`frontend/src/types/question.ts`），其删除归 `questionBank` 片，本片在 api 模块用 `WithUIQuestions` 显式标注该边界；生成物按域重复包含共享类型（`QuestionDTO` 同时在 `practiceMode.ts` 与 `mockExam.ts`），暂不引入跨域共享文件。

## 备选

- **继续全量推迟**：拒绝 —— 输入面已可信、管道已跑通（#940），继续等只会让手写副本继续漂移；按域解冻把风险限制在「每片独立验收」内。
- **一次性全量做（315 端点 + 全部前端模块）**：拒绝 —— 一半端点无人消费（注解写错也无人发现），单 PR 也无法评审。
- **只生成类型文件、前端不动**：拒绝 —— 两套事实源并存，ADR-0019 的动因（漂移）原样保留。
- **漂移一刀切以注解为准**：拒绝 —— 会把「手写正确、注解写错」改错，而这类错误只在联调时暴露。
- **估值排除在外**：拒绝 —— 估值页是 Web 一等消费者，排除等于「单一事实源」名不副实。
- **入参类型一起生成**：拒绝（本批）—— 需先大改 handler 签名，长尾成本远高于收益。

## 后果

- 每片交付物固定为：该域生成物（`frontend/src/api/generated/<域>.ts`）+ 后端字节级同步契约 + 前端 api 模块薄 adapter + 字段级差异清单。生成物过期由后端测试直接变红（先例：`authz` / `deploy` / `apitypes` 三条）。
- 前端手写响应类型从 164 个逐域减少；`API.md` 与注解产物的端点差集逐片收敛。
- 漂移从「联调时才发现」变成「片内必须逐条判定并留证」。
- 每片的硬门：前端 `type-check` + 该域相关 vitest + 后端 `go test ./...`；字段名差异会先在 type-check 暴露。

## 明确不做

- **移动端契约**：归移动端自己的 ADR 体系（`training-app/…/docs/adr/`），根仓库不重复提议。
- **入参（query / body）与 UI 态类型生成**。
- **全量端点注解**（只做 Web 消费面）。
- **站内信 JSONB 落库 payload / SSE 事件 payload 的定型**。
- **`API.md` 重写**：它是人类可读叙述面，字段级以注解产物为准（优先级已写在文档顶部）。
- **估值模块 7 处非统一信封**（`gin.H` / 裸 `c.JSON`）：留到估值片单独决定。

## 相关

- ADR-0019（推迟原文 —— 本文是它的按域修订）、ADR-0030（窄域 codegen 试点）、ADR-0047 §1 / §5（能力表与部署拓扑两条同构生成先例）、ADR-0009（handler 站立模式 / typed DTO）
- spec #940（专项第一步：输入面可信化 + 打卡域试点）
- `docs/agents/checks.md`（`make swagger` 与 CI 新鲜度锁）、`backend/internal/apitypes`（域声明表与渲染器）、`frontend/src/api/generated/checkin.ts`（试点生成物）
