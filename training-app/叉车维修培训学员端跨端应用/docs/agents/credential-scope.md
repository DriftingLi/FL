# 移动端证件下发口径表（哪些端点必须显式传 `credential_id`）

**用途**：移动端**唯一**的「证件下发口径」事实源。改任何按证件分区的请求前先读它；新增/删除传参点必须回来登记，否则 `utils/credentialScopeContract.test.js` 判红。

**它登记的是「核对结论」，不是运行期豁免表** —— 代码里没有、也不要加一张「哪些端点要注入」的表（根仓 ADR-0047 §4 的收敛方向就是删掉客户端注入表）。本表只回答「谁该传、为什么」，传参仍由**调用方显式**发出。

**来源与跨端口径**：
- 服务端解析序 = `backend/internal/middleware/credential_scope.go:24-51`：① 显式 query `credential_id > 0` 优先；② 否则用登录学员的 `hrwai_users.current_credential_id`，**但兜底分支要求 `roleOf(c) == student`**（`:42`）；③ 都没有 ⇒ 不分区。
- 根仓把这条例外登记在 **ADR-0056 §2**（第十一波，`docs/adr/ADR-0056-第十一波架构深化.md`），Web 客户端半边在 `frontend/src/api/__tests__/credentialScopeContract.spec.ts` 的冻结清单里（#1106）。
- 公开端点的决策落点 = **ADR-0057「公开端点的证件口径」**（已合并，`docs/adr/ADR-0057-公开端点的证件口径.md`）。它把例外分**两类**，本表沿用这套词：
  - **类别 A**：未挂 `CredentialScoped`、handler 自读 query（`/catalog/tree`、`/tags`）—— 显式传参是唯一分区通道，挂中间件也不改行为；
  - **类别 B**：挂了 `CredentialScoped` 但所在组没有 JWT（`/search`、`/courses`）—— `roleOf(c)` 恒空，兜底永不生效。
  决策 1/2 明确：这两类都**保持现状**（不挂 `OptionalAuth`）、「未传 = 不分区」原样成立；决策 3 把「显式传参」定为唯一分区通道，由客户端冻结清单守。
- 移动端自身的历史先例：`docs/adr/0014-移动端全局搜索落点与投影口径.md` §40「任何挂在无 `OptionalAuth` 的组下的公开端点，客户端都必须显式传 `credential_id`」。

## 一、必须显式传（服务端兜底不生效）

| 端点 | 服务端注册面 | 分区通道 | 移动端传参点 | 依据 |
| --- | --- | --- | --- | --- |
| `GET /search` | 公开 + `CredentialScoped`（**类别 B**，`api/search.go:28`） | query | `api/search.uts` 两出口 ← `pages/search/search.uvue` | 无 JWT ⇒ `roleOf(c)` 恒空 ⇒ 兜底永不生效；不传 = 全证件。**跟随当前** |
| `GET /courses` | 公开 + `CredentialScoped`（**类别 B**，`api/courses.go:31` 挂在 courses 蓝图 group，公开路由 `:34`） | query | `api/course.uts#getCourseListApi` ← `pages/courses/courses.uvue`、`pages/mall/mall.uvue` | 同上（#1107 前移动端漏传，页面显示全证件课程） |
| `GET /tags` | **公开、未挂** `CredentialScoped`，handler 自读 query（**类别 A**，`api/training_catalog.go:143`） | query | `api/course.uts#getTagsApi` ← `pages/practice/composables/usePracticeOverview.uts` | `question_count` 与**抽题池**同口径（`service.ListQuestionTags` 的 #702 注释）：不传是全证件计数，与学员能抽到的题量对不上（#1107 前移动端漏传） |
| `POST /practice-mode/progress` | JWT + `CredentialScoped`，但证件**从 JSON body 解析**（`api/practice_mode.go:244`） | **body** | `api/practice.uts#savePracticeProgressApi`（仅 `sequential`）← `pages/practice/composables/usePracticeSession.uts` | 中间件对 body 无能为力 ⇒ 写路径无兜底。顺序练习是 **NULL 桶**分区，漏传即把游标写进 NULL 桶、续练读空（迁移史见 `000013` / `000019`） |
| `POST /contributions`（创建） | JWT + `CredentialScoped` | **body（归属声明）** | `api/contribution.uts#createContributionApi` ← `pages/forum/components/forum-contribution-form.uvue` | 投稿必挂目标证件（`credential_id` 必填，表单默认当前）；这是**归属声明**，与分区参数同名不同义 |

## 二、依赖服务端兜底（JWT 面，**不传**）

这些端点挂在 `JWTAuth + CredentialScoped` 的 group 上，学员端调用时 `roleOf(c) == student` 成立 ⇒ 服务端用「当前证件」兜底，客户端**不需要也不应该**传（传了是「显式浏览指定」，移动端当前没有这个入口）。

| 端点组 | 移动端消费点 |
| --- | --- |
| `/question-bank/*` | `api/practice.uts`（stats / questions / by-id） |
| `/practice-mode/*`（除上面的 progress POST） | `api/practice.uts`（free / sequential / tag / submit / stats / history / overview） |
| `/wrong-questions/*` | `api/wrongQuestion.uts` |
| `/mock-exam/*` | `api/mockExam.uts` |
| `/favorites*` | `api/favorite.uts` |
| `/contributions`（列表 / mine / upload-file） | `api/contribution.uts` |

> `/real-exam/*` 同样是 JWT + `CredentialScoped`，但**移动端没有消费面**（无 `api/realExam.uts`）。

## 三、不分区（不传，传了也没用）

| 端点 | 移动端消费点 | 为什么不用传 |
| --- | --- | --- |
| `GET /catalog/tree` | `api/course.uts#getCatalogTreeSpecialties`（courses 页筛选、招聘页筛选） | **类别 A**：未挂 `CredentialScoped`、handler 自读 query（`training_catalog.go:105`），但传 `credential_id` **只分区课程节点**；`service.getCatalogTree` 的专业方向 / 等级列表恒为全量启用项（`training_catalog_service.go:456-475`），而移动端**只消费 `specialties`** ⇒ 传了是 no-op。改成消费课程节点时才需要补参数 |
| `GET /course/:id`、`GET /course/:id/chapter/:id`、`POST /course/:id/progress`、`GET /chapter/:id/slides` | `api/course.uts` | 这四条**在** `CredentialScoped` 的 group 里，但 handler 不读上下文（`courses.go` 只有 `ListCourses` 读，`:72`）⇒ 实际不分证件 |
| `GET /levels`、`GET /credentials`、`GET /credentials/grouped`、`GET /positions` | `api/course.uts#getLevelsApi`、`api/credential.uts`、`api/resume.uts` | 公开字典读，无证件参数（专业方向 / 等级 / 证件 / 岗位均全局共享） |
| `GET/PATCH /me/credential` | `api/credential.uts` | 这是**当前证件自身**的读写（切证件），与「按证件过滤」正交 |
| `/materials`、`/student/materials`、`/student/*` | `api/material.uts`、`api/student.uts` | 未挂 `CredentialScoped`；学习资料 / 学员档案不按证件分区 |
| `/resume*`、`/positions` | `api/resume.uts` | 简历域，与证件分区无关（`credential_id` 字段是简历里的**期望证件**，不是分区参数） |
| `/forum/*`、`/featured-contents*`、`/points/*`、`/check-in/*`、`/notifications*`、`/jobs*` | `api/forum.uts`、`api/featured.uts`、`api/points.uts`、`api/checkin.uts`、`api/notification.uts`、`api/job.uts` | 与 `CONTEXT.md`「当前证件」一致：**论坛 / AI 不过滤**，个人流水面（积分 / 打卡 / 站内信 / 任务）不按证件分区 |
| `/questions/:id/comments`、`/questions/:id/knowledge`、`/questions/:id/note` | `api/questionInteraction.uts` | 评论 / 考点 / 笔记：用户私有或题目附属面，不按证件分区 |

## 四、守护与判据

- **冻结清单**：`utils/credentialScopeContract.test.js` 扫 `api/*.uts` 的**字面量请求点**（方法 + 路径），冻结「落在按证件分区路由族上的消费面」集合与逐文件计数，再正锁必须显式传的「形参 + 赋值」成对存在、反锁 JWT 面不得携带分区参数。新增消费面、多写一处调用、删掉传参都会判红 —— 回来登记本表即可复绿。
- **扫描边界（老实写清）**：静态缝只认字面量 URL ⇒ **看不见**「先拼进变量再发请求」（如 `/wrong-questions/:id/redo`）与 `uploadFile(...)`；那部分靠下面的运行期证据兜。
- **页面传参点用自解释名**：页面里承载「浏览指定 / 跟随当前」的变量一律叫 `browseCredentialId`（对齐根仓 #1106 的意图拆名），便于一眼看出这是分区参数而不是业务字段。
- **运行期证据**：契约测试只能证明「源码里有这个传参点」，证明不了「请求真的带上了」。分支收口时用 ①a 真机的 logcat 机检行核对 URL（`ENABLE_DEBUG_LOG` 下 `request.uts` 打印 `[request] >>> GET <url>`，query 可见）—— 这是比静态扫描更硬的判据。

## 五、已知残余（未做，别当成已修）

- **历史 NULL 桶游标不回填**：`POST /practice-mode/progress` 的修复只对之后的保存生效。此前移动端漏传写下的 `credential_id IS NULL` 顺序练习游标仍在 NULL 桶（根仓 `000019` 那次合并只处理了当时存量）；需要时另开数据票，不在本口径表范围。
- **对 ADR-0057「复访条件」的回复（本表定案）**：ADR-0057 写「移动端确认消费口径后，若两端都愿意把『未传』改成『按当前证件』，本决策重开」。移动端的结论是**不申请复访**：`/catalog/tree` 只被消费 `specialties`（全局共享字典，服务端不随证件分区）⇒「未传」在本端没有实际代价；`/search`、`/courses`、`/tags` 三条「跟随当前」的消费面已各自显式传参，不需要服务端替客户端做主。若将来**给这两个公开端点挂 `OptionalAuth`**（口径真的变了），本表「必须显式传」两行随之复访 —— 传参点去掉也要回来登记。

## 相关

- 根仓：`docs/adr/ADR-0047-第八波架构深化.md` §4（作用域事实源移到服务端 / 删客户端豁免表）、`docs/adr/ADR-0056-第十一波架构深化.md` §2（两类例外的登记）、`docs/adr/ADR-0057-公开端点的证件口径.md`（公开端点两类例外的决策 + 复访条件）、`CONTEXT.md`「当前证件」三族语义
- 移动端：`docs/adr/0014-移动端全局搜索落点与投影口径.md`（#979 M6 的搜索侧先例）、`api/search.uts` / `api/course.uts` / `api/practice.uts` 文件头
- Web 客户端半边：#1106（`frontend/src/api/__tests__/credentialScopeContract.spec.ts`）
