# 前端 UI 约定

> 改前端模板/样式前必读。从 AGENTS.md 拆出（2026-09-09），内容为权威版本。

## UI 词汇

| 术语 | 含义 |
| --- | --- |
| **封装层** | `components/ui/` 下的自建组件集合。业务页面**不得直接使用 Element Plus 控件**，一律经此层（ADR-0035）。例外：`el-table`（197 处，走全局样式覆盖，ADR-0037）与 `el-radio` / `el-radio-button`（组内内容项，ADR-0038）。 |
| **分段控件** | `UiSegmentTabs`，带滑动指示条的选项卡。激活态必须用品牌语义色，**不可用 `bg-panel`** —— 与卡片同值会让滑块在卡片内彻底隐身（浅色 `#FFFFFF` on `#FFFFFF`、深色 `#1E293B` on `#1E293B`，两套主题都失效）。 |
| **下划线 tab** | `UiUnderlineTabs`，**视图档位**用（临时切换看什么，如 编写/预览）。**数据档位**（会被保存的选择，如 纯文本/Markdown）仍用 `UiSegmentTabs`（胶囊 + 实心品牌色滑块）。两者**不可互换**：同一卡片里并排两组同款胶囊，用户分不清哪组会永久保存。激活下划线必须用品牌语义色 `--color-ui-*`，**不可 `bg-panel`**（与卡片同值会隐身，同分段控件的硬约束）；调用方给顶栏 1px 底边，组件用 `-mb-px` 压住它。 |
| **图标工具栏** | `MarkdownToolbar`，一排图标按钮：**每个按钮都要有 `UiTooltip` 中文提示 + `aria-label`**（文案与图标名的唯一来源是命令表 `utils/markdownToolbar.ts`，不许在组件里另抄）。置灰用 `aria-disabled` + 视觉降透明度，**不用原生 `disabled`** —— 原生 disabled 的按钮不派发鼠标事件，提示气泡会整排消失。 |
| **空态两级** | 独立占据内容区的空态（整页 / 列表 / 面板）用 `UiEmptyState`；**卡片正文内嵌**的一行提示保留纯文案（统一 `text-ink-3`），不塞组件 —— 后者换成组件会多出图标与整块留白，比问题本身更重。 |
| **列表四段式** | `UiAsyncSection`：**空态 → 错误态（+ retry）→ 骨架 → 内容**的唯一编排实现（props `loading` / `error` / `empty` / `retrying`，slots `default` / `empty` / `error` / `skeleton`，`@retry`）。页面只提供 loader 与 slot，**不得再手写四分支链**。空态判据与「筛选变化回第一页」在 `useAsyncPage` 内，不再各页手写（ADR-0053）。**判据只认 `isEmpty`**（第十一波 #1101）：新增 `:empty=` 只允许 `isEmpty`（composable 返回的，或页面里 `const isEmpty = …` 具名派生的）或登记例外，不得再写 `x.length === 0` / `!data` / 写死 `false`——守卫 `check-async-section` 机械拦截。**404 = 空态、其余 = 错误态**：`useAsyncPage` 提供 `loadErrorKind`（复用 `api/client.ts` 的 `ApiErrorKind`），资源不存在时 `isEmpty` 同样为真，**详情页不再自建「未找到」布尔**；判据形状特殊（一个 loader 写两个列表、以 total 为准）的页面复用 `utils/listState.ts` 的 `isEmptyValue` / `isEmptyList`，不要再抄一份表达式。**追加式分页（「加载更多」）只有 `useAsyncPage({ mode: 'append' })` 一个入口**（`itemsRef` + `loadMore` / `hasMore` / `loadingMore` / `reset`），页面不得再手算页码与累积。**表格页是它的一档**（`skeleton="none"`）：`el-table` 保留自带 loading 遮罩，空态仍按本表「表格」行（ADR-0037）——不为统一观感把 10+ 个管理页的加载态换掉。展示型第 5 态（如章节不存在）用 `UiEmptyState` + action 表达，**不自造状态**。 |
| **筛选栏** | `UiFilterBar`，只提供容器与 `#filters` / `#actions` 两个插槽，字段由各页自写；**不做 prop 化** —— 各页字段数与按钮语义不一致，prop 化会让组件持续膨胀。 |
| **表格** | `<el-table>` 走 `element-overrides.css` 的 `--el-table-*` 全局变量（ADR-0037），**不封装 UiTable**。两条硬边界：**不动行高与单元格内边距**；**禁用 `primary-*`/`accent-*` 做表格底色**（深色块未重定义这两个色阶，会出「暗底亮块」）。表格显式空态用 `UiEmptyState`，默认空态走全局文字色。 |
| **剩余控件** | 标签一律 `UiTag` 的 `tone`（**不写 `type`**；新值 `brand/primary/success/info/warning/danger/neutral`，映射表用导出的 `UiTagTone` 类型收窄）；开关/多选/单选组/上传/提示气泡对应 `UiSwitch` / `UiCheckbox`（+`UiCheckboxGroup`）/ `UiRadioGroup` / `UiUpload` / `UiTooltip`。全部薄封装（attrs/事件/slot 全透传、**不设默认值**，ADR-0038）；`el-radio` / `el-radio-button` 保持原生（组内内容项）。 |
| **溢出菜单** | `UiMoreMenu`，卡片/列表项右上角「⋯」触发的**治理动作收纳**（举报 / 删除等低频、破坏性、非互动类操作）。**互动动作不进菜单**——回复 / 点赞这类高频社交动作留在卡片底部主操作行，两者不可混放（ADR-0042 的回复区形态）。菜单项由调用方提供，**可见性判定也在调用方**（如自己的回复不出现「举报」）。 |
| **管理端列表状态机** | admin **列表页**一律 `useAdminTable`（页面只声明 `fetch` adapter 与 `actions` adapter，内置三态 / 分页 / 搜索 / 行操作分发 / 删除确认；ADR-0015 + ADR-0039）。**`fetch` 的出口形状是 api 侧的 `Page<T>`（`api/page.ts`，ADR-0060 决策 6 / 票 6），页面不自搭分页容器**——见下「分页容器归 api 侧」。**非列表页不套**（详情/仪表盘/配置页用 `useAsyncPage` 的三态即可）。`useAsyncPage` 是服务全站 34 处的通用三态件，**不要为 admin 改它**。**同一页面里的第二档要写明归属**（第十一波）：分页列表 → `useAdminTable`；只读/计数 section（巡检计数、汇总卡）→ `useAsyncPage` + `UiAsyncSection`。两档都不得 `catch {}` 静默吞错，档位在文件顶部注释里登记——判定口径与登记格式见下「管理端列表两档归属」。 |
| **状态词表** | 同一业务状态（联络授权 / 投递 / 题目状态…）的 **label 与 tone 各只有一个 descriptor**：输入 status，输出 `{ label, tone }`，列表、抽屉、角标、admin 留痕只消费它；status 收成 union（取值集合与后端常量表对齐），**模板里不得内联状态文案裸串**（第十一波）。已落地的两个域见下「状态词表（联络授权 / 投递）」——**状态词按「描述状态事实」取词**，同一个取值不得有两种文案。 |
| **侧栏导航行** | 侧栏每一项由 `components/layout/AppSidebarItem.vue` **这一个**项级渲染 module 出行（行种类外链 `<a>` / `<router-link>` / 目标未就绪的不可点行，只在它的 `kind` 判据上分档），`AppSidebar.vue` 只管分组编排（分组标题、展开态、折叠态 tooltip）。**不得在任一层级里另抄一份 `<a>`/`<router-link>` markup**（第十三波 票9 前：三层嵌套各写两遍、共 9 份）。高亮与展开判定一律走 `config/navigation.ts` 的纯函数（`isNavRouteActive` / `isGroupExpanded` / `toggleGroupExpanded` / `flattenLeaves` / `isGroupActive`，测试面 `config/__tests__/navigation.spec.ts`），**组件里不自建匹配逻辑**。项级样式（`.nav-item` 一族）随组件下沉到 `AppSidebarItem.vue` 的 scoped 块——`scoped` 不穿子组件，祖先选择器（`.app-sidebar.collapsed|is-dark|is-compact`）仍照常命中。 |

### 状态词表（联络授权 / 投递）（第十一波 #1103 / ADR-0056 §8）

| 域 | descriptor（唯一判定处） | 取值（union == 后端常量表） | label（状态事实） | tone |
| --- | --- | --- | --- | --- |
| 联络授权 `contact_requests.status` | `utils/contactRequestStatus.ts` → `describeContactRequest(status)` | `pending` / `approved` / `rejected` / `expired` / `revoked`（后端 `service.ContactGrantState`，`contact_authz.go`） | 待同意 / 已同意 / 已拒绝 / 已过期 / 已撤回 | warning / success / danger / info / danger |
| 投递 `job_applications.status` | `utils/applicationStatus.ts` → `describeApplication(status)` | `applied` / `rejected` / `withdrawn`（后端 `service.ApplicationStatus*`，`job_application_service.go`） | 投递中 / 不合适 / 已撤回 | warning / danger / info |

**漂移裁定**（取「描述状态事实」的那个词，旧词不得回流）：

| 取值 | 旧文案 → 新文案 | 消费面 |
| --- | --- | --- |
| `applied` | 「待处理」→「**投递中**」 | 我的投递（`MyApplications`）、企业投递列表与详情抽屉（`ApplicationList`） |
| `pending`（联络授权） | 「待处理」→「**待同意**」 | 我的申请（`MyRequests`）、收到的申请（`ResumePage`）、admin 留痕（`Inspection`） |
| 企业侧角标 | 「已授权」→「**已同意**」、「待学员确认」→「**待同意**」 | 简历库卡面角标（`Resumes`） |

- **消费面只调 descriptor**：列表 / 抽屉 / 角标 / admin 留痕不得自写 `Record<string, string>`、`if (s === …) return '…'` 或模板内联字面量（含插值与属性里的裸串）。扫描：`src/utils/__tests__/statusWordsTemplate.spec.ts`（把 `<template>` 编译成渲染函数，命中状态文案即红）。
- **取值集合与后端常量表对账**：`src/utils/__tests__/statusWords.spec.ts` 直接读 Go 源里的常量表断言集合相等——**新增状态要先加 Go 常量表**（先例 `contact_authz.go` / `contribution_service.go`），再加 TS union 与 `Record<Status, …>`（漏一个取值编译报错）。
- **三值投影不是第二套状态**：企业侧 `contact_state`（`approved` / `pending` / 空 = 未授权，后端 `contactGrant.State`）由 `contactBadge(state)` 决定出不出角标，label / tone 仍取同一张表。
- 相邻域（题目状态、证件审核状态…）按同一形状收编，各建自己的 descriptor module，不共用一张表。

### 管理端列表两档归属（第十一波 #1102 / ADR-0056 §9）

admin 页里「三态 + 分页 + 筛选」**只有两档**，页面按 section 声明归属，不允许第三份手写实现：

| 档位 | 用于 | 错误通道与判据 |
| --- | --- | --- |
| **档位一** `useAdminTable`（分页列表） | 任何会翻页的列表（服务端分页）。页面只声明 `fetch` adapter 与行操作 adapter；筛选轴（`domain` / `reportStatus` / `jobFilterRecruiter` 这类页面 ref）由 adapter 自己读，不塞进 composable | `loadError` / `loadErrorKind` / `retrying` / `retry` / `isEmpty`；可同页多实例 |
| **档位二** `useAsyncPage`（只读计数） | 只读/计数/汇总 section：单值计数（`deleted-after-accepted`）、汇总卡、facet 计数、非分页聚合页（如课程目录） | `loadError` / `loadErrorKind` / `retrying` / `retry` / `isEmpty` + `UiAsyncSection` |
| **档位二的行内形态** `useAsyncPage`（行内追加） | 装不进页面级实例的行内追加式分页：帖子展开面板里的回复分页（按行实例化——setup 之外不能建 `watch`，所以不能直接持有 `useAsyncPage`） | 判据复用同源 `utils/listState.isEmptyList`；错误态与重试入口**按行**（ADR-0042 回复区） |

**判定口径**（按顺序问）：

1. 这段**会翻页**吗？会 → 档位一。同页多段列表就多实例（每段一个 `useAdminTable`），不要合并成一个大状态机。
2. 不会翻页、只是**一个值 / 一组计数 / 一张汇总卡**吗？是 → 档位二。**不硬套列表状态机**（伪造成 items 只为套组件是 ADR-0056 §9 被否的备选）。
3. 都不是（行内追加分页）→ 登记为**档位二的行内形态**：判据必须同源（`isEmptyList`），状态按行维护，页面上不得出现第二份 `length === 0`。

**空态判据同源**：两档的 `isEmpty` 是同一份实现（`utils/listState.isEmptyList`），`loadErrorKind` 同名同义（**404 = 空态、其余 = 错误态**）。同页多实例用解构改名取具名判据 —— `const { isEmpty: isEmptyViews, … } = useAdminTable(…)`，模板里 `:empty="isEmptyViews"`；`:empty=` 不接受表达式与写死值（守卫 `check-async-section` 机械拦截）。

**分页容器归 api 侧**（第十三波 ADR-0060 决策 6 / 票 6）：档位一的 `fetch` 契约吃 `api/page.ts` 的 `Page<T> = { items, total }`，**容器由 api 模块的出口给**——「这一页的行在响应里叫什么键」（`topics` / `questions` / `tutors` / `requests` / `list` / `items` …，后端同一概念 9+ 个键名）是各域 api 模块自己的事，那里本就写着生成物类型。于是页面的 adapter 只有一行合法形态：

```ts
// 页面：挑那条 api 列表函数、透传分页与筛选轴
fetch: (paging) => adminForumApi.listTopics({ page: paging.page, page_size: paging.pageSize, keyword: keyword.value || undefined })

// api 模块（src/api/forum.ts）：键方言在这里消解，只有一个构造器 toPage(items, total)
async listTopics(params: AdminForumListParams): Promise<Page<AdminForumTopic>> {
  const res = await unwrappedRequest.get<ForumTopicPageResult>('/admin/forum/topics', { params })
  return toPage(res?.topics, res?.total)
}
```

- 页面里**不得**再出现 `{ list: res.topics || [], total: res.total || 0 }` 这类手抄（归位前 11 页 17 处）；也不得换成中立键名绕过去（`{ items: … }` 同样判红）。锁：`src/api/__tests__/page.spec.ts`（按形状判、走 AST，解构改名的 `list:` 不误伤）。
- 不读服务端分页的页面（岗位字典、原价表）在自己 adapter 出口 `toPage(rows, rows.length)` 造容器，**不给 `useAdminTable` 开特例**。
- 不在 `client.ts` 嗅探键名、不让 api 层反向 import composable（ADR-0060 被否备选）。

**两档都不得静默吞错**：失败必须有 error 态 + 重试入口，走各自档位的既有通道（`@retry` 接 `retry`），不得 `catch {}` 了事。

**档位登记**（文件顶部注释，脚本在前的页面用 `//`、模板在前的页面用 HTML 注释；守卫可扫）：

```
// 列表档位：useAdminTable（分页列表）—— 待审核队列 / 举报处置队列
<!-- 列表档位：useAsyncPage（只读计数）—— 删除已解决帖计数 -->
<!-- 列表档位：useAsyncPage（行内追加）—— 帖子展开面板的行内回复分页 -->
```

- 可写的只有上表三行（档位 + 形态）；**两档在场的页面必须逐档登记**——单档页面的归属由上表唯一确定，不必登记。
- 登记行**要拿得出实据**（页面里真有 `useAdminTable(` / `useAsyncPage(`，行内形态真有 `isEmptyList(`）：登记了却对不上即报红，登记行不能退化成一句注释。
- 扫描：`node scripts/check-async-section.mjs --all`（规则 ③；`--diff <base>` 只看新增行）。**扫描面自第十二波票 3 起含 `frontend/src/composables/` 的 .ts**（规则 ④：composable 里「catch 把列表 ref 置空」= 吞错的第三实现形态，即 useCrudTable 的退役根因；附属降级清空须进脚本 `FAIL_OPEN_EXCEPTIONS` 逐条登记理由）。

### 已收敛控件的机械守卫（ADR-0035 备选转正 / spec #940）

「业务页面不得直接使用 Element Plus 控件」不再只靠评审人眼守，CI 里有一条可核验的守卫：

```
node scripts/check-el-controls.mjs --all                  # 全量（CI 用这个），有违规则退出 1
node scripts/check-el-controls.mjs --diff origin/master   # 只看新增行（本地/渐进期）
```

- **守卫集 = 已收敛的 10 类**：dialog / empty / pagination / button / tag / switch / checkbox-group / radio-group / upload / tooltip。命中即报红并给出「→ UiXxx」的对应关系。
- **判定面只认根 `<template>` 块**并跳过 HTML 注释 —— `<script>` 里的字符串与 EP 类名字面量不算违规。
- **放行面**（有意不进守卫集，别当成漏封装）：`components/ui/**`（封装层内部本来就是 EP）、`el-radio` / `el-radio-button` / `el-checkbox`（组的「内容项」而非容器）、`el-table` / `el-table-column`（表格边界）、表单域与布局类 EP（未封装例外）。
- **守卫自己有表驱动自检**（`scripts/check-el-controls.test.mjs`，CI job `el-controls-selftest`）：守卫坏了必须报红，否则规则退化为假绿。

页面保持整洁：不要写冗余的小标题、装饰性提示与说明性 hint 文本，有的话就清理，仅保留必要的功能性提示。删除 hint 时同步删除对应的 CSS class 与 scoped style，避免残留死代码。

**明确例外（不要清）**：

1. 发帖 / 回复输入区的**属地披露提示**（「发布内容会显示 IP 属地」，文案单点 `frontend/src/utils/forumDisplay.ts` 的 `FORUM_REGION_NOTICE`）属于「必要的功能性提示」而非装饰——属地在点发布那一刻才产生，事前告知比事后解释便宜（ADR-0045）。
2. Markdown 档的**能力与边界提示**（文案单点 `forumDisplay.FORUM_MARKDOWN_HINT`，由 `ForumMarkdownInput` 渲染在输入框底部）——它告知两条硬边界：**表格不渲染**、**图片要走粘贴区**（正文里的 `![]()` 会被展开成文字）。依据是 ADR-0046 自己的判据「判据放在作者看得见的地方（编辑器提示 + 预览里的越界说明）」，内容精选编辑器已有同款（ADR-0052）。

按本条约定清理 hint 时**跳过这两条**。

## Tailwind 增量共存四条边界规则

项目已引入 Tailwind CSS v4，与既有 `<style scoped>` 长期共存（详细背景见 `.workbuddy/plans/student-ui-redesign.md`）。共存期间遵守：

| 规则          | 内容                                                                                                       |
| ----------- | -------------------------------------------------------------------------------------------------------- |
| **R1 原子类区** | `src/components/ui/**` 与已纳入改造的页面：模板只用原子类；`<style scoped>` 仅保留伪元素、`:deep()` 改 Element Plus、keyframes、媒体查询 |
| **R2 冻结区**  | 未列入当期改造的页面与组件，scoped 样式**一行不动**。改共用件时新特性一律走 prop + 默认值等于现状，让未传值的调用方零 diff                                |
| **R3 禁双写**  | 同一元素同一属性不允许既有 scoped 类又有原子类。需覆盖 Element Plus 外观时二选一：① scoped 内 `:deep()`；② 原子类加 `!` 后缀（`!px-4`）          |
| **R4 迁移动作** | 页面改用原子类后，删除 scoped 块中已被替代的规则（沿用上一段「删 hint 同步删 CSS」的约定）                                                   |

**变体一律追加覆盖，不改写现有规则**：新增外观分支写成 `.xx.is-dark { … }` 这类多一个类的选择器追加在样式块末尾，特异性天然高于原单类规则，无需 `!important`。

**样式入口只有一个**：`src/assets/styles/tailwind.css`，新增全局样式写进它或它 `@import` 的文件，不要在 `main.ts` 里再加 import。

## 不得触碰的边界

- `--color-brand-*` 属**残值域**专用（`assets/styles/valuation-tokens.css` 在 `.valuation-root` 内定义），**禁止提升为全局变量** —— 会击穿 `layouts/ValuationLayout.vue` 与 `pages/ai-assistant/*` 两处依赖「变量未定义 → 走 fallback」的写法。培训域品牌色用 `--color-primary-*`。

- 残值模块（`pages/student/valuation/**`、`components/valuation/**`）本轮冻结，批量替换色值等机械操作时记得排除。
  **2026-09-04 解除冻结**（见 #554）：为清理深色模式下不跟随主题的硬编码色值，残值域与 `components/catalog/**` 已解冻。解冻后两条约束不变：
  ① `--color-brand-*` 依然禁止提升为全局（见上一条）；
  ② `components/catalog/**`（FacetCard/FacetItem/CourseCard）师生管三端共用，只换色值、不新增 prop。

- **新增样式禁止硬编码色值**：深色模式是靠翻 CSS 变量实现的，只对走 `var()` 的声明生效；裸 hex 不参与变量链，暗色下保持亮色 → 「暗底亮块」崩坏。一律用 `design-tokens.css` 的变量或 Tailwind 原子类。
  CI 会对 PR 的新增行做检查（`scripts/check-bare-hex.sh --diff`），豁免 `var(--token, #fallback)` 防御写法、`#fff`/`#000`、注释行与 `<script>` 块（canvas 色板属合理存在）。存量进度自查：`bash scripts/check-bare-hex.sh --all frontend/src`。

- **主题切换按钮要覆盖所有布局**：现装在 `SidebarLayout`（学员/导师/管理/招聘四端继承它）与 `ValuationLayout`。
  认证布局 `AuthPageShell` 曾漏装，导致系统深色偏好的用户在登录页既看到崩坏画面、又无法切回浅色（#554）。**新增任何独立布局时，必须一并评估主题入口**——这条已踩两次（#432 补了 ValuationLayout，#554 补 AuthPageShell）。
