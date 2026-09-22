import { computed, ref, watch, type Ref } from 'vue'
import { useCredentialStore } from '@/stores/credential'
// 只取类型：client.ts 会建 axios 实例，不能因这条依赖把网络侧带进 composable 的运行时
import type { ApiErrorKind } from '@/api/client'
import { isEmptyList, type EmptyableValue } from '@/utils/listState'

export type { EmptyableValue }

export interface UseAsyncPageOptions {
  /** 默认页大小（分页场景；缺省 20） */
  defaultPageSize?: number
  /** 外部持有的页码 ref（如论坛页按类别分片存储页码）；缺省内部自建 */
  pageRef?: Ref<number>
  /**
   * 证件切换失效刷新（#604，默认 true）：watch credential store 当前证件 id，
   * 变化即回第一页重装——筛选词/关键词是 loader 闭包读的外部 ref，原样保留。
   * 装载数据不随当前证件变化的调用方显式传 false 关闭，两类：
   * ① 学员工作区内本就不受证件过滤的面（论坛/招聘/积分/打卡，口径同 client.ts
   *    注入豁免与 CONTEXT.md「当前证件」词条）；
   * ② 证件切换联动属另案决策的域（证件引导页目录、课程章节学习位置，#594 Out of Scope）。
   * admin/tutor/recruit 三端无需显式关闭：credential store 只对学员角色初始化
   * （路由守卫 + 学员侧栏切换器），current 恒为 null，watch 永不触发。
   * 切勿在调用方再自行 watch 证件变化重装本 loader——会构成本机制的双触发；
   * 同页其他证件口径数据面（筛选选项、课程目录树、标签计数这类 facet）**声明进 `facets`**，
   * 由本机制在同一时刻一起重装（第十四波票 10；此前它们游离在 seam 之外，只挂在 onMounted 上）。
   */
  credentialScoped?: boolean
  /**
   * 筛选轴（#1054）：任一轴变化 → 回第一页重装一次。声明式入口取代每页手写的
   * 「筛选条件变了要回到第一页」（此前 26 处手写、漏写某条筛选轴用户就会停在
   * 越界页码上看到空列表）。同一同步块内多轴齐变只触发一次回调（Vue watch 批处理）；
   * 与证件切换（credentialScoped）是两条独立 watch，同时发生时各触发一次
   * ——与既有「证件切换即重装」行为一致，不在本入口里合并去重。
   *
   * 元素可以是 ref，也可以是 getter（`() => filters.region`，用于 reactive 对象的
   * 单轴）——watch 源两种形态都收，页面无需自建 computed 数组。
   *
   * 第十四波票 10：`facets` 里声明的旁路装载流随这条轴一起重装（scoped facet），
   * 于是「筛选项变化」既不会漏回第一页、也不会漏刷选项，页面不需要再写第二处判据。
   */
  filterDeps?: Array<Ref<unknown> | (() => unknown)>
  /**
   * 条目容器 ref（#1054 / 第十一波 #1101）：装载后写入的那个数组（或详情页的
   * 单个对象 ref）。提供即生成 `isEmpty` 空态判据——「加载成功且没有条目」或
   * 「后端 404（资源不存在）」才为真；装载中恒为 false（四段式里空态的合法位置）。
   *
   * 判据形状特殊的页面（如以 total 为准）可不传，自行声明具名判据后直接喂给
   * UiAsyncSection 的 `empty` prop——组件的空态判定本来就是外部声明。
   */
  itemsRef?: Ref<EmptyableValue>
  /**
   * 装载形态（第十一波 #1101）：
   * - `'replace'`（默认）：每次 run 重新装载当前页（按钮式分页 el-pagination 口径）。
   * - `'append'`：追加式分页（「加载更多」口径）的**唯一入口**。loader 只取一批
   *   （页码由本 composable 维护与推进，作为入参给出），`pickItems` 从响应里取出
   *   条目数组（缺省读 `res.items`），追加与 `hasMore` 判定由本 composable 负责；
   *   `reset` 清空累积并回第 1 批（筛选变化走 `filterDeps`，语义等价）。
   *   此前 JobPlaza 与 Resumes 各手抄一份（含 loadingMore / hasMore / 页码推算 /
   *   筛选重置），现已收编到本分支。
   *   ⚠️ 本档的「刷新 / 重装载」入口是 `reset` 而不是 `run`——`run` 装载的是**当前批并
   *   追加**（重试语义），拿它刷新会把已累积的第 1 批再叠一遍。
   *
   * **服务端事实前置（ADR-0060 §4）**：本档的 `hasMore` 只认响应自带的分页信封（`pages`，
   * 或只给 `total` 时按 `batchSize` 换算），两头都不给即判「没有下一批」并记一条 error——
   * 旧的「本批满一批就算还有下一批」猜测已废除，且不留在缺省路径里兜底。
   */
  mode?: 'replace' | 'append'
  /**
   * append 形态的批大小（缺省 20）；replace 形态用 defaultPageSize。
   * 本值同时是响应只给 `total` 时的换算除数（总页数 = ceil(total / batchSize)：分子来自
   * 服务端、分母是本页自己发过去的 page_size，两头都不是猜）。
   */
  batchSize?: number
  /** append 形态：从 `fetch` 的响应里取出本批条目（缺省读 `res.items`）。 */
  pickItems?: (res: unknown) => unknown[] | undefined
  /**
   * facet 声明槽（第十四波 B 票 10，ADR-0062 决策 10）：**列表之外的旁路装载流**
   * （筛选选项、目录树、标签计数这类跟着页面走的面）在这里声明，
   * 于是「它什么时候该重装」这件事全仓只有一处判据 —— 与列表共享同一批失效时机：
   * 筛选轴变化 / 切证件 / 显式 `reset`。首装跟随列表首装（第一次 `run`），
   * 页面因此不必再自带一份 `onMounted(loadX)` 与一条手写 `watch(证件)`。
   *
   * 漏掉这一格的实测代价：7 个引用当前证件的文件里只有 2 个真在 watch 里重装，
   * `QuestionBank.loadTags()` / `Materials.loadCourses()` 只挂在 `onMounted` 上
   * ⇒ 切证件后选项仍是上一个证件的，拿旧证件的 tag_id/course_id 去过滤新证件 = 空列表。
   *
   * 本槽只管**何时重装**，不管写回：`load` 自己把结果写进页面的 ref（与 loader 同形态）。
   * 在飞期间的失效按 facet 合流（跑完再补一次），故最终一定停在**最后那次**失效的状态上。
   */
  facets?: AsyncPageFacet[]
}

/**
 * facet 声明条目（`UseAsyncPageOptions.facets`）。
 *
 * `scoped`（默认 `true`）= 本 facet 的数据按列表的失效轴分区（筛选轴变化 / 切证件即重装）；
 * `scoped: false` = 全局字典类 facet，只随首装与显式 `reset` 重装
 * （例：不区分证件、也不随筛选变化的常量表）。
 */
export interface AsyncPageFacet {
  /** 装载并写回页面自己的 ref；抛错由本 composable 记 error，不占列表的 `loadError` 通道。 */
  load: () => unknown
  /** 是否随筛选轴/证件切换重装（默认 true）。 */
  scoped?: boolean
}

/**
 * 学员端列表页统一的三态 + 分页状态机（#388）。
 *
 * 收编各页逐字复制的 loading/loadError/retrying + retryLoad 模板与手写分页：
 * - `run`：装载入口。装载前清错误态；loader 抛错即置 loadError（拦截器已统一 toast），
 *   loader 保持纯装配（拉数据 + 写响应），不再各自 try/catch、juggle loading。
 * - `retry`：错误态重试。retrying 防重入，并作为重试按钮的 loading 态。
 * - `page/pageSize/total`：el-pagination 三件套；`handlePageChange` 翻页即重装，
 *   `handleSizeChange` 改页大小回第一页重装（与存量页行为一致）。
 * - 证件切换即失效刷新（#604）：内部 watch 当前证件 id，变化即回第一页重装
 *   （`credentialScoped: false` 显式关闭）。
 * - 筛选轴失效刷新（#1054）：`filterDeps` 声明筛选轴 refs，任一变化回第一页重装。
 *
 * 第十一波（#1101）补齐编排侧三件：
 * - `isEmpty` 成为喂 UiAsyncSection `empty` prop 的默认路径（提供 `itemsRef` 即得）；
 * - `loadErrorKind` 携带 `ApiErrorKind`（复用 `api/client.ts` 的既有分类，不另造一份）；
 *   **404 = 空态、其余 = 错误态**：notfound 时 `isEmpty` 亦为真，页面无需自建 404 分支；
 * - `mode: 'append'` 提供追加式分页的唯一入口（`loadMore` / `hasMore` / `loadingMore` / `reset`）。
 *
 * 第十三波票 4（ADR-0060 §4，含行为变更）：append 档的 `hasMore` 改由**服务端事实**判定
 * ——读响应的分页信封（`pages`，只有 `total` 时按批大小换算）而不是猜「本批满不满一批」，
 * 信封整体缺失即记 error 并判「到底」。同时 `total` 在 append 档由响应写入（页面的
 * 「剩余 N 条」直接读它，不再自己数累积）。
 *
 * 第十四波票 10（ADR-0062 决策 10）：装载流归位到本 composable，两格一起补上——
 * - **批次代数**：`run` 递增、`loadMore` 发起时捕获、落地前比对，不等即整批作废
 *   且**不回写 `page`/`total`/`hasMore`**（旧批写进新窗口就是把两个对象的批次混进同一面板）；
 *   页面零改动，判据只在这一处。
 * - **facet 声明槽**（`facets`）：列表之外的旁路装载流（筛选选项/目录树/标签计数）与列表
 *   共享同一批失效时机（筛选轴变化 / 切证件 / reset），首装跟随列表首装。
 *   「筛选项变化要回第一页」从此只有 `filterDeps` 一个入口 —— 页面不得再拿
 *   `handlePageChange()` 处理筛选切换（那是假空态的成因，机检见 composables/__tests__/loadFlowLocks.spec.ts）。
 *
 * 不分页的页面（详情/聚合页）只解构三态部分即可，分页字段闲置无害。
 */
export function useAsyncPage(load: (page?: number) => Promise<unknown>, options: UseAsyncPageOptions = {}) {
  const loading = ref(false)
  const loadError = ref(false)
  /** 最近一次装载失败的错误分类（`ApiErrorKind`，成功时清空）。404 归空态见 `isEmpty`。 */
  const loadErrorKind = ref<ApiErrorKind | null>(null)
  const retrying = ref(false)
  const page = options.pageRef ?? ref(1)
  const pageSize = ref(options.defaultPageSize ?? options.batchSize ?? 20)
  const total = ref(0)

  const append = options.mode === 'append'
  const batchSize = options.batchSize ?? pageSize.value
  /**
   * append 形态：还有下一批。判据是**服务端给的总页数**（`serverPageCount`，ADR-0060 §4），
   * 不是「本批满不满一批」。replace 形态恒 false。
   */
  const hasMore = ref(false)
  /** append 形态：`loadMore` 在飞行中（按钮 loading 态 + 防重入）。 */
  const loadingMore = ref(false)

  /**
   * 批次代数（第十四波 B 票 10，ADR-0062 决策 10）：`run` 递增、`loadMore` 发起时捕获、
   * 落地前比对 —— 不等即整批作废（条目不追加、`page`/`total`/`hasMore` 都不回写）。
   * 缺了它，「加载更多」飞行中切帖会把上一帖的第 2 批 push 进新帖已清空的列表
   * （`components/student/ChapterDiscussion.vue` 的实测形态：同一面板混两帖回复）。
   * 判据住在 composable 而不是页面：**否**「页面各自加 request-id 守卫」（那正是本波根因形状），
   * 也**否** loader 收 `AbortSignal`（要改 loader 签名与 `client.ts` 透传，非本波量）。
   */
  let generation = 0

  /** 一轮装载起飞前的作废动作：旧轮（含在飞的 append 批）就此不落地。 */
  function invalidate(): number {
    generation += 1
    // 在飞批次的 loading 态由这一轮接管：作废了还不归零，按钮就悬在一个不会回来的 spinner 上
    loadingMore.value = false
    return generation
  }

  // ===== facet 声明槽（ADR-0062 决策 10）：旁路装载流与列表共享失效时机 =====
  interface FacetRuntime {
    facet: AsyncPageFacet
    running: Promise<void> | null
    queued: boolean
  }
  const facetRuntimes: FacetRuntime[] = (options.facets ?? []).map(facet => ({
    facet,
    running: null,
    queued: false
  }))
  /** facet 首装跟随列表首装（第一次 `run`），页面不必再自带一份 `onMounted` 装载。 */
  let facetsBooted = false

  /**
   * 装一个 facet：在飞时不并发第二份，只把这次失效**排队**成一次补跑 ——
   * 于是「最后一次失效」的那次装载一定读到当时的证件/筛选值（不需要 loader 侧的取消语义）。
   */
  function runFacet(runtime: FacetRuntime): Promise<void> {
    if (runtime.running) {
      // 在飞：不并发第二份，记一次「还欠一趟」，由在飞那趟跑完补上（见下方 while 条件）
      runtime.queued = true
      return runtime.running
    }
    runtime.running = (async () => {
      do {
        runtime.queued = false
        try {
          await runtime.facet.load()
        } catch (error) {
          // facet 是选项/聚合面，不占列表的 loadError 通道（列表本体这一轮是好的）；
          // 但不得静默吞错 —— 口径同本文件 append 信封缺失那条 console.error。
          console.error(
            '[useAsyncPage] facet 装载失败：该面降级为上一次的读数（不连带把列表打成错误态）。' +
              '请给这条 facet 的端点补错误出口，或在页面里显式声明降级判据。',
            error
          )
        }
      } while (runtime.queued)
      runtime.running = null
    })()
    return runtime.running
  }

  /**
   * facet 重装入口：`'all'` = 首装与显式 `reset`（连全局字典一起重装）；
   * `'scoped'` = 筛选轴变化 / 切证件（`scoped: false` 的不动）。
   * 不返回给调用方 await —— 列表与 facet 是并行两条流，失效时机同源但互不阻塞。
   */
  function loadFacets(scope: 'all' | 'scoped'): void {
    for (const runtime of facetRuntimes) {
      if (scope === 'all' || runtime.facet.scoped !== false) void runFacet(runtime)
    }
  }

  /** 拦截器把 ApiErrorKind 挂在错误对象上（client.ts attachKind）；无 kind 视为未分类。 */
  function kindOf(error: unknown): ApiErrorKind | null {
    const kind = (error as { kind?: ApiErrorKind } | null)?.kind
    return typeof kind === 'string' ? kind : null
  }

  /** 本批条目（append 形态用）。 */
  function pickBatch(res: unknown): unknown[] | undefined {
    if (options.pickItems) return options.pickItems(res)
    const items = (res as { items?: unknown } | null)?.items
    return Array.isArray(items) ? items : undefined
  }

  /**
   * 服务端给的**总页数**（append 形态 `hasMore` 的唯一事实来源）：
   * 1. `pages` —— 服务端算好的总页数（论坛 / 收藏 / 通知等域直接下发）；
   * 2. `total` —— 只给条目总数时按批大小换算（职位广场与简历库的 `{items,total}` 属这类）。
   *
   * 两头都没有返回 `undefined`，由调用方按契约违规处理（ADR-0060 §4 明令不得退回
   * 「本批满一批」的旧猜测：那条规则两头都错——论坛首页给被采纳回复留一格而置顶条没落位时
   * 只回 19/20，按钮就此消失、长帖翻不到底；末页恰好满批时它又该消失不消失，
   * 点下去是空的一批）。
   */
  function serverPageCount(res: unknown): number | undefined {
    const envelope = res as { pages?: unknown; total?: unknown } | null
    if (typeof envelope?.pages === 'number') return envelope.pages
    if (typeof envelope?.total === 'number') return Math.ceil(envelope.total / batchSize)
    return undefined
  }

  /** append 形态写入一批：追加进 `itemsRef`，`hasMore` 与 `total` 均取响应的分页信封。 */
  function appendBatch(res: unknown, requestedPage: number): void {
    const batch = pickBatch(res)
    if (!batch) {
      hasMore.value = false
      return
    }
    const target = options.itemsRef?.value
    if (Array.isArray(target)) target.push(...batch)
    // 「剩余 N 条」这类人读文案要的条目总数：响应给了才写（没给就不动，页面无需自己数）
    const totalFromRes = (res as { total?: unknown } | null)?.total
    if (typeof totalFromRes === 'number') total.value = totalFromRes
    const pages = serverPageCount(res)
    if (pages === undefined) {
      console.error(
        `[useAsyncPage] append 形态：第 ${requestedPage} 批的响应既无 pages 也无 total，` +
          '无法向服务端确认是否还有下一批 —— 按「没有下一批」处理（宁可不给「加载更多」入口，' +
          '也不退回已废除的「本批满一批」猜测，ADR-0060 §4）。' +
          '请给该列表端点补分页信封，或让本页改用 replace 形态。'
      )
      hasMore.value = false
      return
    }
    hasMore.value = requestedPage < pages
  }

  /** 装载（首屏/翻页/筛选变化共用）：错误收敛为 loadError + loadErrorKind，绝不 reject */
  async function run(): Promise<void> {
    const gen = invalidate()
    if (!facetsBooted) {
      facetsBooted = true
      loadFacets('all')
    }
    loading.value = true
    loadError.value = false
    loadErrorKind.value = null
    if (append) hasMore.value = false
    try {
      const res = await load(page.value)
      // 更新的一轮已起飞（切证件 / 筛选变化 / 再翻页）：本轮结果整体作废
      if (gen !== generation) return
      if (append) appendBatch(res, page.value)
    } catch (error) {
      if (gen !== generation) return
      loadError.value = true
      loadErrorKind.value = kindOf(error)
      // append 形态：本批失败即停（不回退页码），由 retry 重跑同一批
      if (append) hasMore.value = false
    } finally {
      if (gen === generation) {
        loading.value = false
        retrying.value = false
      }
    }
  }

  /** 错误态重试：retrying 防重入 */
  async function retry(): Promise<void> {
    if (retrying.value) return
    retrying.value = true
    await run()
  }

  /** 翻页重装（v-model:current-page 已先行写入 page） */
  function handlePageChange(): void {
    void run()
  }

  /** 页大小变化：回第一页重装（存量页一致行为） */
  function handleSizeChange(): void {
    page.value = 1
    void run()
  }

  /**
   * 回第 1 批重装（facet 重装范围由调用方给：见 `loadFacets`）。
   * `page.value = 1` + `clearItems()` + `run()` 这条序列就是「失效」的唯一种子。
   */
  function reload(scope: 'all' | 'scoped'): Promise<void> {
    page.value = 1
    clearItems()
    hasMore.value = append
    // 首装判据一并结清：`reload` 作为页面挂载入口时不该让 facet 装两趟
    facetsBooted = true
    loadFacets(scope)
    return run()
  }

  /**
   * 清空累积并回第 1 批（append 形态的「刷新」/ 筛选重置 / 写操作后的重装载）。
   * 返回重装载那条 promise，让写操作（采纳 / 删除 / 发帖）能 await 到新批次落地。
   *
   * facet 声明槽（ADR-0062 决策 10）在这条入口上的语义：`reset` 是「整页重来」，
   * 连 `scoped: false` 的全局字典 facet 也一起重装。
   */
  function reset(): Promise<void> {
    return reload('all')
  }

  /** 就地清空 `itemsRef`（保持 ref 引用不变，页面无需换数组）。 */
  function clearItems(): void {
    const target = options.itemsRef?.value
    if (Array.isArray(target)) target.length = 0
  }

  /**
   * append 形态的「加载更多」：推进页码并追加下一批。`loadingMore` 防重入；
   * 失败即停（拦截器已 toast）且**不推进页码**——已累积的条目原地保持、
   * `hasMore` 维持原值（入口不消失），再点一次就是对同一批的重试
   * （本入口不把整页翻成错误态，避免把已看到的内容换掉）。
   */
  async function loadMore(): Promise<void> {
    if (!append || loadingMore.value || loading.value || !hasMore.value) return
    const gen = generation // 发起时捕获批次代数：落地前比对，不等即整批作废（票 10）
    loadingMore.value = true
    try {
      const next = page.value + 1
      const res = await load(next)
      if (gen !== generation) return
      appendBatch(res, next)
      page.value = next
    } catch {
      // 拦截器已统一 toast；页码与本批判据都不动，再点一次即重试同一批
      // （已被更新的一轮作废时同样什么都不做 —— 那一轮自有它的判据）
    } finally {
      if (gen === generation) loadingMore.value = false
    }
  }

  // #1054 筛选轴：任一变化 → 回第一页重装（声明式，单点取代每页手写）。
  // append 形态下这条就是 `reset` 的语义（清空累积 + 回第 1 批），故直接复用而不是再抄一遍。
  // 票 10：facet 声明槽与列表共用这条判据（scoped facet 随轴一起重装）。
  if (options.filterDeps?.length) {
    watch(options.filterDeps as Array<Ref<unknown> | (() => unknown)>, () => {
      void reload('scoped')
    })
  }

  /**
   * append 形态：只有「装载第 1 批」才是首屏骨架（`loadMore` 追加时旧列表原地保持，
   * 由按钮自身的 loading 表达）。replace 形态等价于 `loading`。
   */
  const initialLoading = computed(() => loading.value && !loadingMore.value)

  // #1054 / #1101 空态判据：没有条目即为空——「加载成功但无数据」与
  // 「后端 404（资源不存在，如题目已下架、章节已删除、简历不存在）」同属空态；
  // 其余错误（网络/服务端/权限）是错误态，装载中恒为 false。
  // 判据 opt-in：未提供 itemsRef 时恒 false（判据形状特殊的页面自行声明具名判据后
  // 直接喂给 UiAsyncSection 的 empty prop）。
  // 判定本身在 utils/listState.ts（同一条不变式的唯一实现，判据形状特殊的页面复用同一份）。
  const isEmpty = computed(() => {
    if (!options.itemsRef || loading.value) return false
    return isEmptyList(options.itemsRef.value, {
      error: loadError.value,
      kind: loadErrorKind.value
    })
  })

  // #604/#605 证件切换即失效刷新：回第一页重装，单点替代已删除的每页 opt-in 失效 composable。
  // watch 在调用方 setup 作用域内建立，随组件卸载自动销毁；
  // Pinia 未激活（无 store 依赖的页面单测直挂）时静默跳过：无证件上下文即无切换信号，
  // 容错口径与 client.ts 注入层一致。
  if (options.credentialScoped ?? true) {
    try {
      const credentialStore = useCredentialStore()
      watch(
        () => credentialStore.current?.id,
        () => {
          page.value = 1
          // append 形态：累积窗口整体作废（不清就是把已看过的第 1 批再叠一遍）；
          // replace 形态不动累积——loader 自己覆盖那个 ref，清了反而会让表格页闪一下空表。
          if (append) clearItems()
          // 票 10：证件这一轴与筛选轴同判据 —— 声明进 facets 的旁路装载流一起重装，
          // 页面不再自带 `watch(证件, loadX)`（此前 7 个引用当前证件的文件里只有 2 个真在 watch 里重装）。
          loadFacets('scoped')
          void run()
        }
      )
    } catch {
      // Pinia 未激活：跳过 watch
    }
  }

  return {
    loading,
    initialLoading,
    loadError,
    loadErrorKind,
    retrying,
    isEmpty,
    run,
    retry,
    reset,
    page,
    pageSize,
    total,
    hasMore,
    loadingMore,
    loadMore,
    handlePageChange,
    handleSizeChange
  }
}
