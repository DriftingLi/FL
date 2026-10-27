# 前端 UI 约定

> 改前端模板/样式前必读。从 AGENTS.md 拆出（2026-09-09），内容为权威版本。

## 视觉基调

> 本节回答「我们看起来像什么、为什么」，**不是规则** —— 规则在下面各节，禁令在文末「不得触碰的边界」。加这一节的原因：2026-10-26 实测全库 `grep -r "设计语言\|视觉方向" docs/` **零命中**，而本文件此前全是「必须 / 不得」。只有禁令没有主张的后果是：界面**合规，但没人判断它该不该存在** —— 守卫拦得住「用错了色」，拦不住「这块卡片本来就是多余的」。

- **一色一意**：培训域唯一品牌色是石墨青（`--color-primary-500 #0D9488`）；晴空蓝 `--color-accent-*` 是强调档，不与主色抢主按钮；残值域的 `--color-brand-*` 是另一套，只在 `.valuation-root` 内活着（禁止提升为全局，见文末）。
- **渐变是品牌动作，不是待优化的瑕疵**：主按钮 `--gradient-brand`（`#0F766E → #0369A1`，135°），两端白字实测 5.47 / 5.93（#1892），判据锁在 `contrast.spec.ts`，改色标会红。培训域里它出现的面只有两类：主 CTA（`element-overrides.css`）与头像/首字母块（`AppTopBar` / `Profile` / `ProfileEditDialog`）。外部工具会把青蓝渐变判成「AI 味」—— 那是工具的口径，**不改实底、不扩散到第三类面**（不用于正文、边框、大面积背景）。残值域另有独立用法（含 64px 指标数字的渐变文字，`valuation-tokens.css:225`），属该域既有形态，不在本条射程。
- **表面靠同色消融，边界靠一条缝**：外壳三面同用 `--color-bg-page`，层次由卡片承担；边界只有十字细线 `--color-border-seam`（ADR-0072 / #1629）。不叠第二档底色、不加外壳阴影。
- **字体三分工**：正文 Noto Sans SC、西文与数字 DM Sans、等宽 JetBrains Mono 只给代码与需要对齐的数字。字号只有七档（ADR-0080），展示级 28 / 40 / 44 / 64 是单独登记的例外。
- **动效只报状态，不做编排**：`--duration-tap 120ms` 给点击反馈，过渡走 150 / 200 / 250ms；弹簧与 `reducedMotion` 在 `config/motion.ts`。不加入场编排、不做装饰性循环动画。
- **密度中偏高**：B 端后台，信息密度优先于留白（先例：侧栏密度统一到单档，#1932）。给某个页面加「呼吸感」之前，先确认它不是在偷走一屏信息。
- **中文优先**：界面文案、空态、错误、提示全中文，英文只出现在专有名词与代码里。

## UI 词汇

| 术语 | 含义 |
| --- | --- |
| **意图** | 用户要做的那件事。判「同一意图」用**三轴**：① **目标**（选一个看下去 / 选一个存下来 / 确认一个动作 / 把一个对象当入口…）；② **后果**（是否产生持久化、是否改变你正在操作的对象）；③ **可逆性 / 影响半径**（能否撤销、影响是当前一行还是整页 —— 「交卷」与「仅解除关联」不在这根轴的同一处）。三轴都相同才是同一意图。**层级（页面级 / 卡片级）不是轴** —— 同一意图只有一个视觉语法，层级只派生**尺寸档**（高度 / 字重 / 间距），不派生第二形态。**容器（卡片等）不受意图约束，受档位表约束**；只有可点击的入口才受它约束（**ADR-0083**，2026-10-26；票 #1957）。 |
| **封装层** | `components/ui/` 下的自建组件集合。业务页面**不得直接使用 Element Plus 控件**，一律经此层（ADR-0035）。例外：`el-table`（197 处，走全局样式覆盖，ADR-0037）与 `el-radio` / `el-radio-button`（组内内容项，ADR-0038）。 |
| **分段控件** | `UiSegmentTabs`，带滑动指示条的选项卡。激活态必须用品牌语义色，**不可用 `bg-panel`** —— 与卡片同值会让滑块在卡片内彻底隐身（浅色 `#FFFFFF` on `#FFFFFF`、深色 `#1E293B` on `#1E293B`，两套主题都失效）。 |
| **下划线 tab** | `UiUnderlineTabs`，**视图切换**用（临时切换看什么，如 编写/预览）。**参数选择**（列表筛选 + 会被保存的取值，如 纯文本/Markdown、最新/最热）仍用 `UiSegmentTabs`（胶囊 + 实心品牌色滑块）。两者**不可互换**：同一卡片里并排两组同款胶囊，用户分不清哪组会永久保存。判据与落点见下「意图登记表」（2026-10-26 修正：原词条把胶囊说成「数据档 = 会被保存」，但实测 20 处胶囊里 17 处是列表筛选，见 #1957）。激活下划线必须用品牌语义色 `--color-ui-*`，**不可 `bg-panel`**（与卡片同值会隐身，同分段控件的硬约束）；调用方给顶栏 1px 底边，组件用 `-mb-px` 压住它。 |
| **图标工具栏** | `MarkdownToolbar`，一排图标按钮：**每个按钮都要有 `UiTooltip` 中文提示 + `aria-label`**（文案与图标名的唯一来源是命令表 `utils/markdownToolbar.ts`，不许在组件里另抄）。置灰用 `aria-disabled` + 视觉降透明度，**不用原生 `disabled`** —— 原生 disabled 的按钮不派发鼠标事件，提示气泡会整排消失。**按钮承诺的判据**：按钮插进去的语法必须**在本端预览与发布所用的同一渲染单点上真能渲染**（不是「在某个 markdown 实例上能解析」——裸 `new MarkdownIt()` 与生产实例的插件集不同，结论会相反）；新增按钮逐条过**准入三判据**（频率 / 无对等轻量替代 / 渲染面无歧义降级），见 ADR-0052 的二次 grilling 补记。 |
| **空态两级** | 独立占据内容区的空态（整页 / 列表 / 面板）用 `UiEmptyState`；**卡片正文内嵌**的一行提示保留纯文案（统一 `text-ink-3`），不塞组件 —— 后者换成组件会多出图标与整块留白，比问题本身更重。 |
| **列表四段式** | `UiAsyncSection`：**空态 → 错误态（+ retry）→ 骨架 → 内容**的唯一编排实现（props `loading` / `error` / `empty` / `retrying`，slots `default` / `empty` / `error` / `skeleton`，`@retry`）。页面只提供 loader 与 slot，**不得再手写四分支链**。空态判据与「筛选变化回第一页」在 `useAsyncPage` 内，不再各页手写（ADR-0053）。**判据只认 `isEmpty`**（第十一波 #1101）：新增 `:empty=` 只允许 `isEmpty`（composable 返回的，或页面里 `const isEmpty = …` 具名派生的）或登记例外，不得再写 `x.length === 0` / `!data` / 写死 `false`——守卫 `check-async-section` 机械拦截。**404 = 空态、其余 = 错误态**：`useAsyncPage` 提供 `loadErrorKind`（复用 `api/client.ts` 的 `ApiErrorKind`），资源不存在时 `isEmpty` 同样为真，**详情页不再自建「未找到」布尔**；判据形状特殊（一个 loader 写两个列表、以 total 为准）的页面复用 `utils/listState.ts` 的 `isEmptyValue` / `isEmptyList`，不要再抄一份表达式。**追加式分页（「加载更多」）只有 `useAsyncPage({ mode: 'append' })` 一个入口**（`itemsRef` + `loadMore` / `hasMore` / `loadingMore` / `reset`），页面不得再手算页码与累积。**表格页是它的一档**（`skeleton="none"`）：`el-table` 保留自带 loading 遮罩，空态仍按本表「表格」行（ADR-0037）——不为统一观感把 10+ 个管理页的加载态换掉。展示型第 5 态（如章节不存在）用 `UiEmptyState` + action 表达，**不自造状态**。 |
| **筛选栏** | `UiFilterBar`，只提供容器与 `#filters` / `#actions` 两个插槽，字段由各页自写；**不做 prop 化** —— 各页字段数与按钮语义不一致，prop 化会让组件持续膨胀。 |
| **表格** | `<el-table>` 走 `element-overrides.css` 的 `--el-table-*` 全局变量（ADR-0037），**不封装 UiTable**。两条硬边界：**不动行高与单元格内边距**；**禁用 `primary-*`/`accent-*` 做表格底色**（深色块未重定义这两个色阶，会出「暗底亮块」）。表格显式空态用 `UiEmptyState`，默认空态走全局文字色。 |
| **剩余控件** | 标签一律 `UiTag` 的 `tone`（**不写 `type`**；新值 `brand/primary/success/info/warning/danger/neutral`，映射表用导出的 `UiTagTone` 类型收窄）；开关/多选/单选组/上传/提示气泡对应 `UiSwitch` / `UiCheckbox`（+`UiCheckboxGroup`）/ `UiRadioGroup` / `UiUpload` / `UiTooltip`。全部薄封装（attrs/事件/slot 全透传、**不设默认值**，ADR-0038）；`el-radio` / `el-radio-button` 保持原生（组内内容项）。 |
| **溢出菜单** | `UiMoreMenu`，**治理动作收纳**（举报 / 删除等低频、破坏性、非互动类操作）。**互动动作不进菜单**——回复 / 点赞这类高频社交动作留在卡片底部主操作行，两者不可混放（ADR-0042 的回复区形态）。菜单项由调用方提供，**可见性判定也在调用方**（如自己的回复不出现「举报」；`items` 为空时连触发按钮都不渲染）。**两档触发**（#1978 裁定 B）：`trigger="icon"`（默认，卡片/列表项右上角的 `⋯`，靠 `label` 给读屏名）/ `trigger="text"`（表格「操作」列的「操作 ▾」，管理端 10 个列表页的既有形态）。**档不是新组件**（ADR-0083）；两档的菜单内容规则完全一致，`divided` 只承接删除项前的分隔线。 |
| **管理端列表状态机** | admin **列表页**一律 `useAdminTable`（页面只声明 `fetch` adapter 与 `actions` adapter，内置三态 / 分页 / 搜索 / 行操作分发 / 删除确认；ADR-0015 + ADR-0039）。**`fetch` 的出口形状是 api 侧的 `Page<T>`（`api/page.ts`，ADR-0060 决策 6 / 票 6），页面不自搭分页容器**——见下「分页容器归 api 侧」。**非列表页不套**（详情/仪表盘/配置页用 `useAsyncPage` 的三态即可）。`useAsyncPage` 是服务全站 34 处的通用三态件，**不要为 admin 改它**。**同一页面里的第二档要写明归属**（第十一波）：分页列表 → `useAdminTable`；只读/计数 section（巡检计数、汇总卡）→ `useAsyncPage` + `UiAsyncSection`。两档都不得 `catch {}` 静默吞错，档位在文件顶部注释里登记——判定口径与登记格式见下「管理端列表两档归属」。 |
| **状态词表** | 同一业务状态（联络授权 / 投递 / 题目状态…）的 **label 与 tone 各只有一个 descriptor**：输入 status，输出 `{ label, tone }`，列表、抽屉、角标、admin 留痕只消费它；status 收成 union（取值集合与后端常量表对齐），**模板里不得内联状态文案裸串**（第十一波）。已落地的两个域见下「状态词表（联络授权 / 投递）」——**状态词按「描述状态事实」取词**，同一个取值不得有两种文案。 |
| **外壳（十字细线）** | 顶栏 / 侧栏 / 内容区底**同用** `--color-bg-page`（背景靠同色融为一体，层次由卡片承担），边界由**十字细线**明示（ADR-0072 的 #1629 改判）：横线 = 顶栏 `border-bottom`，竖线 = `.topbar-left` 与 `.app-sidebar` 各自的 `border-right`（**贯通顶栏**），二者在顶栏下沿 × 侧栏右缘交叉。**不得**再加第二档底色或阴影来表达边界；侧栏的「自带深色」体系（`sidebarTheme` / `.is-dark` / `--color-bg-sidebar` 三件套）已整体退役，不要再复活。**外壳内部不得出现可见滚动条**（#1627 补漏）：侧栏导航列一溢出，原生滑块就紧贴侧栏右缘、成了接缝上最显眼的一条竖线（线上逐像素实测：x=293..299、6px、`--color-border-dark`）——该列 `scrollbar-width: none` + `::-webkit-scrollbar { width: 0 }`，滚动本身照常。**线色走 `--color-border-seam`**（#1646）：接缝是外壳边界的唯一表达，比「卡片内部分隔线」清晰一档（浅色 `#CBD5E1`），**不得**改回通用边框族 `--color-border-light`（它与 `--color-bg-page` 几乎同值，缝在屏幕上等于没画）；三处消费点（顶栏横线 / 左格右边线 / 侧栏右边界）是同一条缝，值必须同源。**底边也是同一条缝**（#1811）：侧栏「收起侧栏」上方那条与 `SiteFooter` 的上边框满幅、同色，页脚挂在 `.main-container` 里（`.main-content` 的兄弟，不是它的子节点）才吃得到 `flex: 1` 的剩余高度；两块高度同为 53px，内容短时两条线在视口底部接成一条，内容长时页脚随文档滚走、侧栏那条线留在视口底。**页脚不得改成 `position: fixed`**。 |
| **顶栏** | `components/layout/AppTopBar.vue`，棋盘格第一行：**左格** = 身份（头像 + 昵称，纯展示，宽度与侧栏同源、随折叠联动）、**中区** = 工作区控件插槽（学员端放 `CredentialSwitcher`）、**右区** = 工具位（通知入口 + 单一用户菜单：外观三态 + 退出登录）。**主题入口与退出登录只此一处**（浮动主题按钮已退役）；角色徽标收在用户菜单顶部的只读行里。 |
| **标签栏** | `components/layout/AdminTabBar.vue`，**只在管理端**出现在内容列顶部。**承载位置是 `SidebarLayout` 的 `.content-header`（内容区之外、与页面上下列）**：挂在 `.main-content` 里会吃到 `--space-6` 内边距、离顶栏下沿差 24px（#1633 线上反馈「标签页没有贴近顶栏」），现在 `position: sticky; top: var(--topbar-height)`、底色与内容区同底、**不另加分隔线**（十字细线仍只有顶栏那一条横线）。标签标题取描述符的 `nav.label`（`navigation.pageTitleOf`）——**不读 `route.meta.title`**（`routeMeta()` 从来没注入过它，读了就是印英文路由名，`仪表盘 \| AuditLogs`，同 #1630）。固定页（仪表盘）不可关；右键出菜单（关闭当前/其他/全部）；详情页按描述符的 `nav.activeRouteNames` 并入所属列表标签。与其配套的页面缓存是**逐页 opt-in**（`PageDescriptor.keepAlive` + 页面 `defineOptions({ name })` 与路由名对齐），锁在 `config/__tests__/keepAliveLock.spec.ts`。 |
| **页面过渡与页面根** | 内容列的页面切换过渡**不得用 `mode="out-in"`**（#1631）：它的放行条件是「离场动画完成」的回调，而**根节点不是单个元素**的页面在渲染器里走 Fragment 分支、那个回调永不触发 ⇒ `state.isLeaving` 永久为真、**内容区从切走那一刻起永久空白**（刷新才恢复）。管理端「管理员管理」的根恰好是多分支组件 `UiAsyncSection`（Vue 警告 `renders non-element root node that cannot be animated`）。现状：默认模式 + **离场即时隐藏**（`.inner-fade-leave-active { display: none }`）+ 只保留进入动画；回归锁 `layouts/__tests__/SidebarLayoutCache.spec.ts`（**必须显式 `stubs: { transition: false }`** —— @vue/test-utils 默认把 `<transition>` 换成 stub，stub 掉就等于把被测行为删掉，用例会假绿）。**页面组件模板尽量单根**（多根仍可用，只是没有进入动画）；直接以多分支组件为根时自包一层容器。 |
| **能力的中文名** | 能力键（`admin_role.manage` 这类）与资源域**不得原样印给用户看**：展示名走 `utils/capabilityWords.ts`（`describeCapability` / `describeCapabilityDomain`），词表是 `Record<AuthzCapability, string>` 与 `Record<CapabilityDomain, string>` 的**穷尽**映射 —— 后端加能力而前端产物没再生成时 `npm run type-check` 直接红（键集合的事实源仍是 `config/authz.ts`，域由键的模板字面量推断，不另立清单）；未登记的键回落为原键（降级可见，不静默留空）。角色权限页（`pages/admin/RoleManage.vue`）是当前唯一消费面，原始键留在勾选框的 `title` 上供对照后端日志。 |
| **侧栏导航行** | 侧栏每一项由 `components/layout/AppSidebarItem.vue` **这一个**项级渲染 module 出行（行种类外链 `<a>` / `<router-link>` / 目标未就绪的不可点行，只在它的 `kind` 判据上分档），`AppSidebar.vue` 只管分组编排（分组标题、展开态、折叠态 tooltip）。**不得在任一层级里另抄一份 `<a>`/`<router-link>` markup**（第十三波 票9 前：三层嵌套各写两遍、共 9 份）。高亮与展开判定一律走 `config/navigation.ts` 的纯函数（`isNavRouteActive` / `isGroupExpanded` / `toggleGroupExpanded` / `flattenLeaves` / `isGroupActive`，测试面 `config/__tests__/navigation.spec.ts`），**组件里不自建匹配逻辑**。项级样式（`.nav-item` 一族）随组件下沉到 `AppSidebarItem.vue` 的 scoped 块——`scoped` 不穿子组件，祖先选择器（`.app-sidebar.collapsed`）仍照常命中。**密度只有单档**：`density` / `sidebarDensity` 与 `is-compact` 一档已随 ADR-0081 退役（四端统一到管理端那档），不要再为某一端加回来。 |

### 意图登记表（ADR-0083 / #1957）

> **判重与判「新意图」只用三轴**；层级（页面级 / 卡片级）不是轴，只派生尺寸档。三轴全同 = 同一意图，必须收敛到一个形态。
> 「唯一形态」= **迁移完成后**应当出现的那个；存量里的第二种写法写在括号里（待迁），那是「要消失的」，**不是备选**。
> 取证与逐条清单：[1957-UI形态盘点](../design/1957-UI形态盘点.md)；截图：`docs/verification/1957/`。

| 意图 | 三轴：目标 / 后果 / 可逆性·影响半径 | 唯一形态 | 存量 |
| --- | --- | --- | --- |
| **命令动作** | 执行一个会改变数据的动作 / 提交或写入 / 用 variant 表达危险度 | `UiButton` | 326 |
| **图标动作** | 同屏内快速操作 / 局部、单对象 / 多为视图级、可逆 | 原生 `<button>` + `UiIcon`（命中区按 ADR-0078：桌面 28 / 移动 40） | 39（封装层内 6 / 组件 26 / 页面 7） |
| **互动计数动作** | 表达或撤销一次社交动作 / 写一次计数 / 可撤销、单条 | `UiActionChip` | 7 |
| **快捷入口** | 一次点击填好查询或输入 / 只读查询 / 当前面板 | `UiCapsule` | 2 |
| **菜单收纳** | 收低频、破坏性动作 / 打开菜单，不直接改数据 / 可取消 | `UiMoreMenu`（两档触发：卡片/列表项 `⋯` / 表格「操作 ▾」）；**触发方式不是「⋯ 按钮」的三处保留原生 `el-dropdown`**：顶栏用户菜单 / 页签右键菜单 / 播放器倍速 | 16（含表格行内动作；3 处非收纳不迁，见左）。形态判据见下「行内动作的形态规则」 |
| **主题切换** | 换外观 / 只改本地偏好 / 随时切回 | `ThemeToggle` | 3 |
| **参数选择** | 换一组筛选条件，或选一个会被保存的取值 / 重新查询或写入设置 / 当前列表或字段 | `UiSegmentTabs` | 18（+ `el-tabs` 2 待迁：`ProfileReview`、`ForumManage` 内层） |
| **视图切换** | 换呈现方式（编写/预览）/ 不改数据 / 当前输入区 | `UiUnderlineTabs` | 2 |
| **面板切换** | 换一整块工作区 / 整块替换，可能丢未保存编辑 / **整个内容区** | `el-tabs`（原生，登记） | 4（+ `UiSegmentTabs` 2 待迁：`FaqManage` 分类/条目、`Materials` 课程资料/学员投稿） |
| **布尔开关** | 立即生效 / 立即写入 / 单字段 | `UiSwitch` | 9 |
| **多选** | 选若干项再提交 / 提交时写入 / 单表单 | `UiCheckboxGroup`（组内 `el-checkbox` 登记） | 3 |
| **单选组** | 互斥选一个再提交 / 提交时写入 / 单表单 | `UiRadioGroup`（组内 `el-radio` / `el-radio-button` 登记） | 8 |
| **下拉选择** | 从数据源选一个 / 提交时写入 / 单字段 | `UiSelect` | 3（迁移完成，页面侧 0 处；同上） |
| **文本输入** | 输入文本 / 提交时写入 / 单字段 | `UiInput` | 15（迁移完成，页面侧 0 处；口径见 [1981 盘点](../design/1981-表单域存量盘点.md)） |
| **结构化值输入** | 数值 / 日期 / 级联 / 提交时写入 / 单字段 | 原生 `el-input-number` / `el-date-picker` / `el-cascader`（登记） | 25 |
| **文件上传** | 选或拖文件 / 提交时写入 / 可重选 | `UiUpload` | 5 |
| **分面筛选** | 按维度收窄结果并显示计数 / 重新查询 / 当前列表 | `FacetCard` + `FacetItem`（**成对唯一**，不拆用） | 卡片 6 + 项 13 / 3 页 |
| **行内确认** | 确认一次半径 = 一行的删除 / 立即执行 / 一行 | `el-popconfirm`（原生，登记；参考元素必须是 `UiButton` 的 `link` 档） | 5 |
| **模态确认** | 确认一次跨行或不可逆动作 / 打断后执行 / 一次操作或一批 | `useConfirm().confirm` / `.confirmDanger` | 45 实例 / 46 调用 |
| **输入式确认** | 带输入再执行（如驳回填原因）/ 写入后执行 / 一条记录 | `useConfirm().prompt` | 1 |
| **模态对话** | 收集一次输入、阻断 / 提交后写入 / 当前任务 | `UiDialog` | 56 |
| **侧边抽屉** | 看或改长内容、保留列表上下文 / 提交后写入 / 当前对象 | `el-drawer`（原生，登记） | 4 |
| **气泡提示** | 补充说明、不承载动作 / 无 / — | `UiTooltip` | 18 |
| **富内容气泡** | 气泡里承载小段富内容 / 无 / — | `el-popover`（原生，登记） | 1 |
| **卡片容器** | **不受意图约束** —— 受下「容器档位表」约束 | `UiCard` 或页面本地容器类 | UiCard 24 / 本地容器 29 / `el-card` 27（待退场，ADR-0087） |
| **指标卡** | 一眼读一个数 / 只读 / 一屏 | `UiStatCard` | 6（+ 管理端 8 待迁，ADR-0087） |
| **空态** | 告诉这里没内容 / 只读 / 一块区域 | `UiEmptyState` | 59 |
| **错误态** | 失败并可重试 / 只读 / 一块区域 | `UiErrorState`（或被 `UiAsyncSection` 的四态覆盖） | 6 |
| **异步四态编排** | 空 / 错 / 骨架 / 内容的唯一编排 / — / 一块区域 | `UiAsyncSection` | 50 |
| **提示条** | 常驻提示 / 只读 / 一块区域 | `UiAlert` | 2（+ `el-alert` 6 待迁） |
| **分页** | 翻页 / 只读 / 当前列表 | `UiPagination` | 34 |
| **骨架** | 加载占位 / 无 / 一块区域 | `UiSkeleton` | 38（+ `el-skeleton` 2 待迁） |
| **行内加载** | 加载指示 / 无 / 行内 | `UiLoading` | 1 |
| **进度** | 百分比 / 无 / 一块区域 | `UiProgress` | 2（+ `el-progress` 3 待迁） |
| **计数角标** | 一个图标上的数字 / 只读 / 一个图标 | `UiBadge` | 0（+ `el-badge` 1 待迁） |

**结构件**（无判重问题，单点形态；不在意图约束内，但也不得另起第二种写法）：

| 件 | 唯一形态 | 存量 |
| --- | --- | --- |
| 数据表 | `<el-table>`（ADR-0037，**不封装**） | 32 / 173 列 |
| 导航行 | `AppSidebarItem` | — |
| 页签 | `AdminTabBar` | — |
| 页面标题 | `UiPageHeader` | 5 |
| 区块标题 | `UiSectionHeader` | 2 |
| 表单字段壳 | `UiFormField` | 5 |
| 筛选栏容器 | `UiFilterBar` | 13 |
| 图标 | `UiIcon`（唯一入口，`check-icons` 守卫） | 194 |
| 图标形变 | `UiIconMorph`（`UiIcon morph` 的底层，不直接落点） | 0 |
| 计数动画 | `UiCountTo`（只经 `UiStatCard` 使用，不当独立件落点） | 1（在 `UiStatCard` 内） |
| 分割线 | `UiDivider` | 0（+ `el-divider` 4 待迁） |
| 头像 | `el-avatar`（原生，登记） | 15 |
| 图片 | `el-image` / `el-image-viewer`（EP 自带全屏查看器，`el-image` 配套）+ 域内查看器（`ImageViewer` / `PptViewer` / `SpreadsheetViewer` / `DocumentViewer` / `VideoPlayer`） | 4 + viewer 1 |
| 折叠面板 | `el-collapse`（原生，登记） | 2 / 6 |
| 描述列表 | `el-descriptions`（原生，登记） | 1 / 6 |
| 布局栅格 | `el-row` / `el-col`（EP 布局，登记） | 4 / 10 |
| 表单容器 | `el-form`（原生，登记；字段壳 `el-form-item` 见下「子件」） | 38 |

**子件**（随父件：只在某个父件内部合法，父件被登记即子件被登记。它们没有独立意图，不参与判重，也不单独落点）：

| 子件 | 父件 |
| --- | --- |
| `el-form-item` | `el-form` |
| `el-option` / `el-option-group` | `el-select` |
| `el-table-column` | `el-table` |
| `el-tab-pane` | `el-tabs` |
| `el-collapse-item` | `el-collapse` |
| `el-descriptions-item` | `el-descriptions` |
| `el-dropdown-menu` / `el-dropdown-item` | `el-dropdown` |

**登记表怎么用**（三条，回答「拿不准怎么办」）：

1. **先问三轴**：新落点想用第二种写法时，先指出**哪一轴**与现有行不同。指不出来 = 同一意图，必须用那一行的形态。
2. **容器查档位表，入口查登记表**：卡片、面板、抽屉壳这类容器不判意图（ADR-0083 决策 3），只管四项档位；只有**可点击的入口**才受「一意图一形态」约束。
3. **原生 + 登记不是欠账**：`el-tabs`（面板切换）、`el-drawer`、`el-popconfirm`、`el-avatar`、`el-collapse`、`el-descriptions` 是**判过之后决定不封装**的（落点少、结构样式差异大、薄封装挡不住结构），不是「还没迁」。哪天落点数或结构需求变了，先改这张表再动代码。

### 行内动作的形态规则（#2005）

表格「操作」列只有两种形态，按**列内可能出现的最大动作数**判（不按行动态切换 —— 否则同一页会出现「同行不同形」）：

| 条件 | 形态 |
| --- | --- |
| 恒为 **1** 个动作 | 平铺单个 `UiButton link` |
| **≥2** 个动作 | `UiMoreMenu trigger="text"`（「操作 ▾」），列宽固定 **90** |
| **例外**：审核 / 处置队列的**结论动作**（通过 / 驳回 / 下架 / 标记已处理） | 平铺 `link`，保一次点击 |

- **为什么留这个例外**：审核队列的工作循环就是「逐行判定」，每行多一次点击 = 队列处理速度下降。判据是动作的性质（对一条待处理记录下结论），不是页面名 —— 别拿它当「不想进菜单」的通用逃生口。
- **破坏性动作只进菜单**，`divided: true` 与上一项隔开；确认走 `useConfirm`（模态），行内 `el-popconfirm` 不再用于删除。
- **平铺一律 `link` 文字档**：行内一排描边 / 实心小按钮在密集表格里最抢眼，与「密度中偏高」的基调相反。
- **暂时不允许的菜单项**用 `disabled` + `hint`（原生 title 说明原因），**不要把它藏掉** —— 藏掉会让人以为没这个操作。
- 判据的自动化：未来守卫可数 `items` 或按钮数核对本表（#1982 的登记制守卫预留了这个面）。

### 容器档位表（ADR-0083 / #1957）

> 容器（卡片、面板、抽屉壳、区块框）的外观**只能取档内值**。页面本地容器类允许存在，但**只许取值，不许自造**。值只有两个出处：scoped 写 `var(--radius-*)` / `var(--space-*)`，模板写 `rounded-card` / `rounded-ctl` / `rounded-pill`（`tailwind.css` 的语义别名，解析到 `design-tokens.css`）。
> 早于本表的写法怎么收敛、现在还剩什么，逐条清单在 [1957 附录](../design/1957-附录-scoped取值清单.md)（#1979 / #1980 收敛后用同一脚本重新生成）。

**圆角（4 档）**

| 档 | 别名 / token | 值 | 用于 |
| --- | --- | --- | --- |
| 卡片 | `rounded-card` / `--radius-lg` | 12px | 卡片、面板、弹层、抽屉 |
| 控件 | `rounded-ctl` / `--radius-md` | 8px | 输入、按钮、图标钮、小容器、内嵌块 |
| 胶囊 | `rounded-pill` / `--radius-full` | 9999px | 胶囊、标签、头像、圆点（`50%` 真圆等价） |
| 零 | `rounded-none` | 0 | 贴边、表格内、无缝拼接 |

**收敛完成（#1979）**，越档 0 处：16px / 10px / 6px / 4px / 3px / 2px 与裸 `rounded` 已全部归位（scoped 侧写 `var(--radius-*)`，模板侧写 `rounded-card` / `rounded-ctl` / `rounded-pill`，含方向变体 `rounded-bl-ctl`）。

三种写法**不算越档**、保留原样：`border-radius: 50%`（真圆，与胶囊档等价）、`border-radius: 0 !important`（零档值，`!important` 是对抗 vditor 内联样式的）、以及模板里的 `rounded-none`。

**唯一登记例外**：`0 var(--radius-full) var(--radius-full) 0`（`AppSidebarItem` 的侧栏项左贴边）。

**内边距（4 档，仅容器卡片）**

| 档 | 值 | 模板类 |
| --- | --- | --- |
| snug | 12 | `p-3` |
| base | 16 | `p-4` |
| roomy | 20 | `p-5` |
| loose | 24 | `p-6` |

复合值（如 `20px 16px`）**不是越档**：横竖各取一档即可。越档的只有 `8px` / `10px`（→ snug）、`14px`（→ base）、`32px` / `40px`（→ loose）。

**描边（1 档）**：`1px solid var(--color-border-light)`（模板 `border-line`）。更强的 `--color-border`（`border-line-strong`）只允许用在「结果卡 / 选中态」这类**要压过同屏其它卡片**的地方，并且必须逐条登记（现存 1 处：`.result-card`）。

**阴影（4 档）**

| 档 | 别名 / token | 用于 |
| --- | --- | --- |
| 无 | `none` | 贴边容器、页内 section |
| 卡片 | `shadow-card` / `--shadow-xs` | 默认卡片 |
| 悬起 | `shadow-raised` / `--shadow-md` | 可点卡片 hover |
| 弹层 | `--shadow-lg` | 对话框 / 抽屉 / popper（EP 自带，页面不写） |

**收敛完成（#1980）**：`--shadow-sm` 不再被页面引用。它原本是 `xs` 与 `md` 之间的第五档，而仅剩的 3 处全都落在 hover / elevated 态 —— 归到卡片档会与底座同档（hover 看不出变化），所以按档位表落到**悬起档**（`var(--shadow-md)`），卡片的高亮阶梯因此固定为 `xs`（静置）→ `md`（悬起）。

**不是阴影的别写成阴影**：选中环用 `outline` 表达（`.credential-card.active` 的 `0 0 0 2px` 已改，#1980）。

两类**登记例外**（各自有理由，不是漏改）：

- **着色按下光晕**（2 处，`AuthPageShell` 的备用登录方式）：`group-aria-pressed:shadow-[0_4px_10px_rgba(…)]`。着色光晕用 4 档中性 token 表达不出来，去掉则按下态丢掉品牌色反馈。
- **inset 描边环**（3 处，`AuthPageShell` 的 `.el-input__wrapper` 常态 / hover / 聚焦）：这是 Element Plus 给输入框画边框的固有手法（`border: none` + `0 0 0 1px … inset`），我们的代码只换颜色、不换机制。改成 `border` 要连带覆写 `box-shadow: none`，还会绕开 EP 自己的 `transition: box-shadow`，收益不抵风险。

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
   **能力清单改为由子集真值表派生（口径随 #1241 落档，实现随 #1253）**：今天这段文案把九项能力**手抄**成一句话（「支持 标题、加粗、斜体、引用、代码、链接、列表、任务列表、公式与图表；表格不渲染，图片请用下方粘贴区」），子集一变就成假话——它曾是四处手工同步点里的第四处。裁定后**能力清单由白名单真值表生成**（两条边界仍固定文案），因此落地后它不再是手工同步点，也不再是「第二事实源」；ADR-0052 评审补记里那条「能力提示补上公式与图表」的可发现性要求由此保住（不再靠人记得改）。

按本条约定清理 hint 时**跳过这两条**。

## 图标（ADR-0079）

- **唯一入口** `UiIcon`：**图标是数据不是组件**。注册表在 `icons/registry.ts`，`IconName` 由它推断 —— 名字写错 `npm run type-check` 直接红；运行期未登记的名字回落警示图形并在**开发环境**告警（生产静默降级），不白屏。
- **尺寸**：默认 `1em`（跟随字号，与旧 `el-icon` 同口径，迁移零位移）；要固定值就传数字（px）。图标控件的**命中区**另见 ADR-0078（桌面 28 / 移动 40），与本条不冲突。
- **形变只在形状真的改变时用**：`morph` 是显式开关，白名单 = 主题三态、侧栏折叠、收藏/举报/删除的描边↔实心、上传结果。同形状换方向（箭头旋转）与纯填充开关（心形，同一个 `d`）走 CSS，不用形变。
- **可访问名**：有 `label` 即 `role="img"` + `<title>`；没有就是装饰性图形（`aria-hidden`）。纯图标按钮的 `aria-label` 仍按 #1893 的口径挂在按钮上，不靠图标自证。
- **不得**：业务代码 import `@element-plus/icons-vue`、写 `<el-icon>`、手写 `viewBox="0 0 24 24"` 的 svg —— 守卫 `node scripts/check-icons.mjs --all` 报红；迁移期例外逐条登记在 `scripts/icon-legacy-baseline.json`，**只减不增**（迁干净的文件必须删行，自检会拦）。
- **动效参数单点**：`config/motion.ts`。弹簧档与降级口径（`reducedMotion` 默认 `user`，跟随系统）只在那里改；CSS 过渡仍归 `design-tokens.css`，两套机制各管各的动画面。

## 字号（ADR-0080）

- **只有七档**：`text-xs` 12 / `text-sm` 14 / `text-base` 16 / `text-lg` 18 / `text-xl` 20 / `text-2xl` 24 / `text-3xl` 30（取值源是 `design-tokens.css` 的 `--text-*`）。
- **两条路都走 token**：工具类用阶梯档，禁 `text-[13px]` 这类任意值；scoped 样式写 `font-size: var(--text-*)`（残值域三个文件可用自家的 `var(--fs-sm, 14px)`）。映射是**唯一解、等距向上取**：13→`text-sm`、15→`text-base`、17→`text-lg`、22→`text-2xl`（与 `UiPageHeader` 的页标题同档）。守卫 `node scripts/check-font-scale.mjs --all` 报红（两条规则：`arbitrary-font-size` 管工具类，`literal-font-size` 管 scoped 声明）。
- **两类字面量是规则的一部分**（别当成漏网去"顺手收"）：① **展示级 28 / 40 / 44 / 64px** —— 大号数字与占位字形，不进七档，取值登记在守卫 `DISPLAY_TIER`；② **`html` 的三档 rem 基准**（16 / 14 / 13）—— 它派生整个 rem 尺度，声明上一行必须带 `font-scale-root` 标记，token 化等于「整体缩放页面」。
- **其余放行面**：`text-[var(…)]` 是颜色、`text-[1em]` 是图标跟随父字号、`leading-[…]` 行高没有阶梯（别为整齐去收行高）、`font-size: inherit`。
- **已知接缝**：scoped 的 token 是 **px**，工具类 `text-*` 是 **rem**；`html` 在 ≤768 / ≤480 把基准降到 14 / 13px，同一档字号在窄屏上会差 81–93%（既有事实，统一会动全站节奏，见 ADR-0080 决策 4）。
- **行高不跟字号走**：`text-*` 会带上 Tailwind 的默认行高（`text-sm` 1.43 / `text-base` 1.5）；要 1.6 / 1.8 这类阅读行高就显式写 `leading-[…]`。

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

## 响应式（ADR-0084 / 0085 / 0086）

- **管理端只保桌面**（ADR-0084）：验收宽度是 1280 / 1024；375 / 768 只保证「不崩」——**无页面级横向溢出**即可，不承诺能完成业务动作。
- **管理端 ≤768 的 `@media` 只允许「防溢出 / 换行」类兜底**；字号、间距、内边距这类观感微调**不再新增**（新增即违规）。存量逐条判定见 `docs/design/1965-管理端媒体查询判定.md`。
- **学员端必守 375×667**（ADR-0085）：PR 验收必须含 **375 宽截图**；本地可用 `node scripts/check-student-375.mjs` 复核（需要 dev server + 开了远程调试的 Chromium）。
- **宽表只横滚、不降列**（ADR-0086）：`el-table` 保留表体横滚；固定列**只允许「操作」一列固定在右**。

## 不得触碰的边界

- `--color-brand-*` 属**残值域**专用（`assets/styles/valuation-tokens.css` 在 `.valuation-root` 内定义），**禁止提升为全局变量** —— 会击穿 `pages/admin/ValuationConfigManage.vue` 与 `pages/ai-assistant/*` 两处依赖「变量未定义 → 走 fallback」的写法。培训域品牌色用 `--color-primary-*`。

- 残值模块本轮冻结，批量替换色值等机械操作时记得排除。（2026-10 起公开界面已北迁门户仓，本仓只剩 `pages/admin/ValuationConfigManage.vue` 与 `components/valuation/PageHeader.vue`。）
  **2026-09-04 解除冻结**（见 #554）：为清理深色模式下不跟随主题的硬编码色值，残值域与 `components/catalog/**` 已解冻。解冻后两条约束不变：
  ① `--color-brand-*` 依然禁止提升为全局（见上一条）；
  ② `components/catalog/**`（FacetCard/FacetItem/CourseCard）师生管三端共用，只换色值、不新增 prop。

- **新增样式禁止硬编码色值**：深色模式是靠翻 CSS 变量实现的，只对走 `var()` 的声明生效；裸 hex 不参与变量链，暗色下保持亮色 → 「暗底亮块」崩坏。一律用 `design-tokens.css` 的变量或 Tailwind 原子类。
  CI 会对 PR 的新增行做检查（`scripts/check-bare-hex.sh --diff`），豁免 `var(--token, #fallback)` 防御写法、`#fff`/`#000`、注释行与 `<script>` 块（canvas 色板属合理存在）。存量进度自查：`bash scripts/check-bare-hex.sh --all frontend/src`。

- **主题切换入口要覆盖所有布局**：#1619 起，四个走 `SidebarLayout` 的工作区（学员/讲师/管理/招聘）由**顶栏右区的用户菜单**承载（外观三态；原 `ThemeToggle fixed` 浮动按钮已退役）；`AuthPageShell`、`ChatPageShell` 各自保留自己的入口。
  认证布局 `AuthPageShell` 曾漏装，导致系统深色偏好的用户在登录页既看到崩坏画面、又无法切回浅色（#554）。**新增任何独立布局时，必须一并评估主题入口**——这条已踩两次（#432 补的是残值布局，该布局已随北迁删除；#554 补 AuthPageShell）。

## 已知缺口（Known Gaps）

> 骨架借自 DESIGN.md 的 Known Gaps 一节（`awesome-design-md` 的 84 份文档每份都有）：**把「这里没覆盖、为什么暂缓、动了会怎样」写明白**，比只写禁令便宜，也比只写禁令有效 —— agent 读到缺口就不会「顺手补齐」。**补齐一条就删一行**，不要留成历史；历史在票与 ADR 里。

| 范围 | 现状 | 为什么暂缓 | 动了会怎样 |
| --- | --- | --- | --- |
| 固定色阶当填充（B 类） | 31 处成对 chip：深色下是「暗页上的亮块」，但对比度不变 | `docs/design/1880-token-audit.md` 另票 | 逐个换翻转填充会变「深底深字」，必须先有新的成对 token |
| 打印 | 全库 `@media print` **0 条** | 未立票 | 打印会把顶栏、侧栏、页脚一起印出来；要支持得单独定一版稿 |
| 响应式 | 172 个 `.vue` 里 **39 个**有响应式处理（26 条 `@media` + 1 处 `matchMedia` + 41 处 `sm:`/`md:` 工具类，并集 39）；断点集中在 768px | 口径已定（ADR-0084/0085/0086，见上「响应式」节），**迁移按页面族另开票**，不是没有判据 | 现状不违规：管理端 ≤768 只承诺不崩、学员端必守 375；逐个页面「顺手加断点」会违反 ADR-0084 的「管理端 ≤768 只允许防溢出类兜底」 |
| 卡片容器三写法 | `UiCard` 24 / `el-card` 27 / 本地容器类 29 | **ADR-0087 已签字（2026-10-26）**，迁移按批推进（批一 = 仪表盘 + 统计分析） | 一次性改会动 13 个文件的垂直节奏（el-card 的内边距在 `.el-card__body` 上），必须分批带截图 |
| 指标卡三形态 | `UiStatCard` 只在学员端 6 处；管理端 8 处是两套写法（仪表盘 `.stat-card` 4、统计分析 `el-card.stat-card` 4） | 并入 ADR-0087 批一（仪表盘 + 统计分析同屏可对照） | 直接迁会丢掉版式差异（居中 / 左侧色条 / 图标位），得先补 `layout` / `accent` 两档 |
| Element Plus 内置文案 | 分页 `Total 26`（34 处）、行内确认 `Yes/No`（5 处）等，全站未配 locale | [#1971](https://github.com/DriftingLi/FL/issues/1971) 已立票 | 只给单个组件传中文文案治不了 `el-date-picker` / `el-upload`；要修就 `ElConfigProvider + zhCn` 一处修全站，并回归分页/气泡/日期三处 |
| 标签 `effect="dark"` | 白字压在语义主色填充上：warning **2.15** / success 2.54 / primary 3.74 / danger 3.76 / info 4.83（全站 7 处，如「✓ 已解决」） | #1954 只收口 light / plain 的文字档；改实心填充属观感裁定 | 要修得把填充换成**不翻转**的深色（danger `#B91C1C` 等）；别只换 ink —— 深色主题下 `-strong` 会翻成浅色，白字压上去只剩 1.6:1 |
