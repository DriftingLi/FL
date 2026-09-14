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
- **已知限制（留后续片）**：**注解层还没有枚举词汇** —— `status` 一类封闭值集在 swag 侧可用 `enums:"..."` 标注，但生成器尚未把它渲染成 TS 联合类型，故 `ContributionItemDTO.status` 生成 `string`，前端的窄化联合（`ContributionStatus`）只能保留并在消费处断言（`ContributionTab.vue`）。这是注解表达力缺口，不是「注解写错」；补 enum 渲染属生成器能力，另立片；题目类型 `QuestionDTO` 同时是跨域 UI 模型（`frontend/src/types/question.ts`），其删除归 `questionBank` 片，本片在 api 模块用 `WithUIQuestions` 显式标注该边界；生成物按域重复包含共享类型（`QuestionDTO` 同时在 `practiceMode.ts` 与 `mockExam.ts`），暂不引入跨域共享文件。

## 实施修订（2026-09-13，片二 #954）

片二（handler 内联响应 map 收口）落地时的实测与口径修正：

- **计数修订**：决策 6 记的「响应面 9 处 / 6 文件」，实测为 **12 处 / 6 文件**（admin.go 4、admin_recruiter.go 4、admin_points.go 1、recruit.go 1、training_catalog.go 1、practice_mode.go 1）。判据：`backend/internal/api/` 里以 `map[string]any{...}` 作为**响应体**手工拼装的点（Endpoint 骨架的响应类型参数，或裸 handler 的响应体）。
- **`gin.H{}` 形态不在这一片**：另有 35 处 / 12 文件（contact / material / featured / notification / training / job / questionBank / aiAssistant / admin_inspection …），分布在各域自己的消费面里，按决策 7 由**各自域的片**在补注解时一并收口。
- **这 12 个端点当时都不在 swagger 里**（admin / recruit / training 域零注解），故收口本身不改注解；除 `POST /practice-mode/progress` 外，其余 11 处的注解仍归各自域片补。
- **字节契约机制**沿用 ADR-0009 的字节序纪律：`TestInlineResponseDTOBytes` 用「改造前的 map 形态 ↔ 新 DTO」表驱动逐字节比对（参照物是旧 map 本身，不是手抄字面量）；DTO 投影折叠进构造器（ADR-0009 §2）。
- **保留的现状差异**：招聘者创建（10 字段，含 `status`）与编辑（9 字段）形状不同，按「字节不变」保留，是否统一交 admin 片；`POST /practice-mode/progress` 的 `data` 从「注解写作无、实际有」纠正为 `service.ProgressSaveResultDTO`（该端点此前被片一登记为 NoData，本片一并改正）。

## 实施修订（2026-09-13，片三 #959）

auth 域片（注解缺口最深的域）落地时的口径补充：

- **计数**：Web 消费面 **29 个端点**（`api/auth.ts` 26 + `client.ts` 的 `/auth/refresh`）→ **17 个有 data、12 个有意无 data**（logout、DELETE account、email/phone send-code、email/phone reset-password、profile/send-code、profile/email、profile/phone、profile/password、profile/password/send-code、account/send-code）；3 个登录端点（admin / tutor / recruiter）此前**不在 swagger**，本片补注解。
- **`DataRef` 认得 201**：注册类端点用 `response.Created`，声明表锁此前只认 200，会把它们误判成「缺注解」——已修（`internal/apitypes/codegen.go`）。
- **内联响应 map 的判据**：`/auth/refresh` 的响应体是 `map[string]string`（不是 `map[string]any` / `gin.H`，片二的口径没圈到它）——判据应读作「handler 手工拼的响应体」，与容器类型无关。
- **共享 client 也是消费面**：`api/client.ts` 的静默刷新直接裸读 axios 的 `res.data.data`；本片给它补上生成的 `RefreshResultDTO`（信封形状显式声明）。「唯一事实源」覆盖**所有**消费点，不只是域模块。
- **会话态 UI 模型保留**：`UserProfile`（token ∪ 登录基础字段 ∪ `/auth/me` 全量资料）继续手写并注明边界；`PendingProfileChange` 改为从生成类型派生（删掉手写副本）。

## 实施修订（2026-09-14，片四 #962）

互动面五域（forum / notification / favorite / wrongQuestion / questionInteraction）落地时的实测与口径补充：

- **计数**：Web 消费面 **51 个端点**（forum 30 / notification 4 / favorite 4 / wrongQuestion 6 / questionInteraction 7）→ **26 个指认 data、25 个有意无 data**；其中 **2 个端点此前完全不在 swagger**（`GET /admin/forum/topics`、`GET /admin/forum/reports`）；issue 记的「2 个缺席」与实测一致，但 `POST /wrong-questions/{question_id}/redo` 实为「在 swagger、只是路径参数名不同」，不在缺席之列。
- **共享 handler 的第二条路由**：`GET /admin/forum/topics` 与 `GET /forum/topics` 共用 `ListTopics`。处置沿用 `AdminGetTopic` 先例 —— 薄包装方法承载该路由的注解块，路由注册改指包装方法；否则管理端路由会从 swagger 消失（文档面缩水）。
- **内联响应 map 收口 6 处**（决策 6 的 `gin.H{}` 形态）：forum 图片上传（`{url}`）、主题/回复的点赞与取消点赞 ×4（`{liked,likes_count}`）、通知未读数（`{count}`）、题目评论列表（`{items,page,page_size,total}`）—— 各自定型为 service DTO 并进 `TestEnvelopeDTOShapeLock`。**字段按 map 的 key 字母序声明**（`liked` 在 `likes_count` 前）才保证序列化字节序不变。
- **JSONB payload 的表达**：`NotificationDTO.Payload`（站内信 JSONB 落库 payload）此前让 swag 解析失败（`json.RawMessage` 无类型定义），`/notifications` 因此无法指认 `data`。处置：注解层用 `swaggertype:"object" extensions:"x-optional"` 钉成**不透明 object**（决策 6「非响应面不动」），生成物渲染 `Record<string, unknown>`；前端保留唯一的 UI 收窄类型 `NotificationPayload`，`NotificationItem` 由生成类型 `Omit` 掉 payload 后挂上它。
- **可空性进注解层 12 处**：按「omitempty → `x-optional`（键可能不存在）；无 omitempty 的指针 → `x-nullable`（键在、值可 null）」逐字段标注，覆盖 `ForumTopicDTO` / `ForumReplyDTO` / `ForumReportDTO` / `MyReplyDTO` / `NotificationDTO` / `WrongQuestionDTO`。
- **顶层 data 可空是注解层表达力缺口**：`GET /questions/{question_id}/note` 未写笔记时 `data` 为 `null`，而 `response.R{data=model.QuestionNote}` 只能指认 `$ref`。处置：前端 adapter 显式写 `QuestionNote | null` 并注明 —— 与枚举词汇缺口同类的已知限制，不是「注解写错」。
- **枚举词汇缺口**（片一已知限制）在本片的三处消费点：`ForumTopicDTO.category` / `content_format`、`FavoriteDTO.target_type` 在注解层是 `string`。处置：UI 联合保留；`content_format` 新增 `toForumContentFormat()` 在渲染前收窄（未知值回落到 `ForumContent` 的 text 缺省，与后端 `normalizeContentFormat` 同口径）。
- **`model.*` 类型可直接作根**：`model.QuestionNote` / `model.QuestionTag` 是真实返回类型（无 service 包装），登记为根后生成物以去包名形式出现（`QuestionNote` / `QuestionTag`），本片不新增包装 DTO。
- **字段级差异 12 条**全部落在 ② 手写类型过时：`NotificationItem.payload` 的 `?`、`parent_id` / `chapter_id` 把 omitempty（键缺失）当成了 null、`/wrong-questions/{id}/remove|batch-remove` 手写 `<null>` 实为 `{removed}`、`FavoriteDTO` 的 `title?/cover?/created_at?`、测试桩里的 `QuestionTag` 局部形状、错题页本地 `WrongItem` 副本。① 注解写错 0 条、③ 后端第三种形状 0 条。
## 实施修订（2026-09-13，片八 #966）

aiAssistant 域片（第一个「信封 + 流式」混合模块）落地时的口径补充：

- **计数修订**：Web 消费面 **15 个端点**（`api/aiAssistant.ts`）—— **11 个在 swagger 但 `@Success` 一律不指认 data**（本片逐条补/纠），**4 个诊断端点**（`/ai-assistant/diagnosis/*`，含手册字节流）此前**零注解**（本片补完整注解块）→ **11 个有 data、4 个有意无 data**（user-models POST / DELETE、sessions DELETE、diagnosis/manual 字节流）。issue 正文记的「9 个端点不在 swagger」实测不成立：真正缺席的是 4 个诊断端点，另 11 个是「在 swagger 但不指认 data」。
- **SSE 端点的切片样板（决策 6 的落地口径）**：`POST /ai-assistant/chat` **不登记进域声明表**（没有 data 类型可指认，登记只能靠 NoData 撒谎），排除口径写进域 `Title`（随生成物头部渲染给读者），注解维持 `@Success 200 {string} string` 并在 `@Description` 注明「不走统一信封、不在契约生成面」；SSE 事件 payload（message / sources / usage / error / done）在 `api/aiAssistant.ts` 手写并逐条注明「非生成面」。唯一例外：`sources` 事件复用生成类型 `DiagnosisSource`（与历史回放同一形状，不是第二份事实源）。
- **裸 fetch 逐个判定**：模块里 2 处裸 fetch —— `upload-image` **走统一信封**（读 `body.data.url`），纳入生成面并接上 `AIImageUploadResultDTO`；`chat` 是 SSE，排除。
- **非指针容器的可空性**：`AIChatMessageDTO.images` / `.sources` 是无 `omitempty` 的切片，空值出站是 `null`（**键在、值可 null**）→ 标 `extensions:"x-nullable"`。本片把「只动指针字段」的口径补成：**容器字段的 nil 同样是可空态**（判据仍是真实构造处是否总是赋值；先例 `ProgressResultDTO.answers_state map[string]any`）。
- **嵌套匿名结构体定型**：`DiagnosisSource.Metadata` 原是匿名 struct —— swag 只把它吐成内联 object，而渲染规则对「带 properties 的 object」只给 `{ [key: string]: unknown }`，前端 `metadata.source_url` 会退化成 `unknown`。定型为命名类型 `service.DiagnosisSourceMetadata`（同字段序、同 tag，序列化字节不变）后进 definitions 传递闭包。
- **标量数组 data**：`GET /ai-assistant/diagnosis/models` 的 data 是 `[]string`（`data=[]string`，无根类型、不进生成物）——声明表锁只要求「有 data 指认」，不要求 `$ref`。
## 实施修订（2026-09-13，片五 #963）

recruit + job + resume 三域（招聘端三模块）落地时的口径补充：

- **计数**：三个 api 模块合计 **30 个调用点**（recruit.ts 6 / job.ts 14 / resume.ts 10）→ 去重后 **33 个唯一 method+path**（`/recruit/resumes/{id}` 与 `/recruit/contact-requests` 被多模块共用）→ **30 个有 data、3 个有意无 data**（两个 inline `application/pdf` 字节流 + `DELETE /resume/pdf`，后者 data 是空对象且前端不消费）；3 个端点（`/recruit/me`、`/recruit/resumes`、`/recruit/resumes/{id}`）此前**不在 swagger**，本片补完整注解块。
- **`json.RawMessage` 是注解层的死路**：`swag` 解析不了它（`cannot find type definition: json.RawMessage`），会让**整个** swagger 生成失败；直接写 `[]byte` 又会被 `encoding/json` 编成 base64（响应字节会变）。简历卡的 4 个 JSONB 数组字段因此改用新增的 `service.JSONArray`：底层 `[]byte` + 与 `json.RawMessage` 同语义的 `MarshalJSON`/`UnmarshalJSON`，注解层配 `swaggertype:"array,string"` / `"array,object"` 渲染成真数组。**序列化字节不变**由 `TestJobCardContract` / `TestStudentSeesCompanyContactContract` 守住。
- **注解层可以声明形状而不改响应构造**：`ContactRequestListResult` / `ContactPlainDTO` 是**只为 data 指认**新增的类型，handler 里的 `gin.H{...}` 一行未动（片二的口径是「改构造 + 字节锁」，本片的选择是「不动构造 + 只声明形状」；`gin.H` 的收口仍留给需要它的域片）。inline object（`data=object{url=string}` / `object{count=integer}`）是同一口径的轻量形态。
- **枚举词汇缺口的直接后果**：招聘域的 `status` / `apply_state` / `contact_state` 等都是封闭值集，生成物只能到 `string`，前端手写的窄化联合随之删除、消费处按字符串比较。这是注解表达力缺口（片一已记录），不是「手写正确、注解写错」。
- **本片实测差异 13 条**：① 注解写错 **1** 条——`/resume/view-stats` 的 data 从「标量 integer」自我纠正为 `object{count=integer}`（handler 是 `gin.H{"count": cnt}`；新契约测试的顶层 key 断言当场抓红）；② 手写类型过时 **11** 条（最典型：`RecruitResumeItem.expected_specialty_*` 是死字段，后端从 #492 起就叫 `expected_position_*`；`getContact` 的手写字面量漏了后端一直返回的 `photos`/`resume_certifications`）；③ 后端第三种形状 **2** 条（均只留证、无文件改动）。

## 实施修订（2026-09-14，片九 #967）

估值域（`internal/valuation/handler`，Web 消费面唯一「整块零注解」的区域）落地时的实测与口径：

- **计数**：估值前端模块的**显式调用点 32 个**，但本片登记 **67 个端点** —— `valuation/admin.ts` 的字典 CRUD 是 `createCrud × 12 实体`（每个实体 list/create/update/remove 四态），按决策 2「域一旦开做就补全该域」登记该域**全部已注册路由**（69 op − 2 条 PDF 字节流 − `/valuation/health`）；显式调用点 32/32 全部命中 swagger。issue 记的「28 调用点 / 全部不在 swagger」是按模块字面调用点估的。
- **注解块 69 个**：`config.go` 19 / `evaluation.go` 4 / `battery.go` 5 / `report.go` 2 / `auth.go` 2 / 新增 `dictcrud_docs.go` 37 —— 描述符工厂的闭包承载不了 swag 注解，管理端注解写在**薄包装方法**上（先例 `api/forum.go` 的 `AdminGetTopic`），方法体恒一行转发、零行为变更。
- **@Tags 用估值自己的体系**（估值-字典 / 评估 / 电池 / 报告 / 认证 / 管理端），不复用主 `/api` 的「学员端-*」—— 这正是决策 5 把它独立成片的理由之一。
- **平行 auth 显性化**：逐个标注 `@Security` —— public 留空、optional 2 个（匿名可提交，注解里注明）、valAuth 5 个 BearerAuth、admin 37 个 BearerAuth；安全边界第一次在契约面可见。
- **只补注解、不改形状**：7 处非统一信封（`gin.H` / 裸 `c.JSON`）**只登记不改造**，注解用 swag 内联 `object{}` 如实描述形状（故不误标 NoData）：`config.go` 的 ListOriginalPrices / GetEarliestFactoryYear、`evaluation.go` 的 List / Stats、`reportflow.go` 的 serveReportGenerate、`dictcrud_routes.go` 的 deleteDict、`auth.go` 的 Me。
- **可空性 5 处** `x-optional`；本域无指针字段，故无 `x-nullable`。
- **生成物的已知限制**：内联 `object{}` 形状不进生成物（渲染器只渲染具名类型），`generated/valuation.ts` 覆盖具名根类型；`EvaluationStats` / `GenerateReportResponse` 两处前端保留与注解逐字段对齐的窄手写类型 —— 彻底收敛需生成器支持匿名对象，属生成器能力片。
- **字段级差异 8 条**全部落在 ② 手写类型过时：`/evaluations/{id}/report` 与电池报告的 `pdf_path` / `file_name` / `report_path` / `generated_at` 是**凭空字段**（实际是 `{evaluation_id,pdf_url,file_size}`），`EvaluationDetail` / `EvaluationResult` / `BatteryEvaluationDetail` / 12 个字典条目 / `CoefficientConfig` 的字段增减与可空性落后。① 0 条、③ 0 条。
- **swagger 产物可复现（本 PR head 实测）**：实施过程中一度观察到「恢复 backend/docs 后重跑产物大面积不同」，复核后确认那是把「含本片新注解的树」与「HEAD 的旧产物」相比所致，不是工具链不确定性 —— 在本 PR head 上重跑 `swag init` 后 `git status backend/docs` 干净，CI 的「Swagger 产物新鲜度」锁会通过。
## 实施修订（2026-09-14，片六 #964）

questionBank + tutor + training + search 四域，外带 course / material / realExam / student / credential 五个学习面模块（合计 **66 个 Web 消费端点**）落地时的实测与口径补充：

- **计数（实测）**：66 个端点 → **57 个有 data、9 个有意无 data**（题目删除 + 目录四个删除/排序 + 证件删除/排序）；**42 个此前根本不在 swagger**（questionBank 12 / tutor 7 / training 18 / credential 5），逐条补了完整注解块。issue 的 66/42 计数与实测一致。
- **「已注解但不指认 data」的 9 个端点实际几乎全有载荷**，其中 8 个的响应体是 handler 里的 `gin.H{...}`（levels / tags / admin-certificate-templates / admin-question-tags / credentials ×2 / me-credential ×2）——注解要指认类型就必须先有具名类型，故按决策 6 与片二修订在本片一并收口为 DTO（`LevelListDTO` / `QuestionTagListDTO` / `CertificateTemplateListDTO` / `CredentialListDTO` / `CurrentCredentialDTO` / `QuestionImageUploadDTO`，外加 map 形态的 `GroupedCredentialsDTO`）。**issue 正文的 Out of Scope 把 `gin.H{}` 列为不做，与片二修订「分布在各域消费面里、由各自域的片在补注解时一并收口」冲突**；实施按 ADR 与实施简报执行，且只收口本片消费面（`/admin/specialties`、`/admin/levels`、`/positions`、`/materials/{id}/download` 等非本片消费点未动）。
- **swag 没有联合类型表达力**：`GET /api/search` 同一状态码两种形状（type 缺省 = `SearchAllDTO` 分区聚合，指定 type = `SearchPageDTO` 分页）。注解写两条 @Success，swag 同名状态码取最后一条，故 data 指认为聚合形状；分页形状同域生成，前端以联合类型消费。属注解表达力缺口，不是注解写错。
- **具名 map 类型在生成物里会渲染成空 interface**：`GroupedCredentialsDTO` 故改用结构体（service 侧两个 key 恒初始化、字段序按 key 字母序，序列化字节不变，shape-lock 冻结），而不是具名 map。
- **`json:",string"` 字段要显式钉住**：`StudentDTO.UID`（Go 侧 int64 + `json:"uid,string"`）若不写 `swaggertype:"string"`，生成物会渲染 `number` —— 与真实线上类型（字符串）撒谎。这是片一「注解是唯一事实源」的又一处输入面修正。
- **非指针 `omitempty` 字段（集成时已补齐）**：`CourseDTO.certificate_name` / `RealExamPaperDTO.source` / `ChapterDetailDTO.study_status` 一类「非指针 + omitempty」字段，按片一修订的口径应标 `x-optional`（键可能不存在）——片六实施时误读为「只覆盖指针」，集成时按**生成闭包**统一补标（三处），生成物随之渲染 `?`。判据：**omitempty 在不在，与是不是指针无关**；唯一例外是结构体取值字段（`ContributionItemDTO.Author`）——encoding/json 不省略零值结构体，`omitempty` 对其无效，故不标。
- **枚举窄化**（`QuestionType` / `QuestionStatus` / `SearchType` / `CredentialDict.category`）仍依赖尚未实现的 enum 渲染能力，UI 值集保留、消费处按域收窄。
- **前端九个模块退化为薄 adapter**：旧名与生成形状对应者保留为别名（`CourseSummary` / `CourseChapter` / `ChapterDetail` / `ChapterFile` / `TutorCourse` / `TutorChapter` / `CatalogLevel` / `CatalogTree` / `QuestionTag` / `StudyStats` / `StudyRecordItem` / `StudentCourseDetail` / `MaterialItem` / `RealExamPaper` / `SearchItem` …），形状确实不同者删旧名（`QuestionBankStats` 凭空 published/pending/total_count、`StudentProfile` 扁平形状、`TutorChapterDetail` 的幻影 course/chapters、`RealExamPracticeStart` 等）。题目 UI 模型 `Question`（@/types/question）**保留**并由 `WithUIQuestions` 显式标注边界 —— 其删除仍等 enum 渲染能力。可空性收紧使一批测试夹具（课程/等级/标签/章节）补全必填字段。

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