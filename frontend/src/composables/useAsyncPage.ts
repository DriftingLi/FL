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
   * 同页其他证件口径数据面（如课程目录 facet 由其所属装载流收敛）不在此限。
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
   */
  mode?: 'replace' | 'append'
  /** append 形态的批大小（缺省 20）；replace 形态用 defaultPageSize。 */
  batchSize?: number
  /** append 形态：从 `fetch` 的响应里取出本批条目（缺省读 `res.items`）。 */
  pickItems?: (res: unknown) => unknown[] | undefined
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
  /** append 形态：还有下一批（本批未满一批即到底）。replace 形态恒 false。 */
  const hasMore = ref(false)
  /** append 形态：`loadMore` 在飞行中（按钮 loading 态 + 防重入）。 */
  const loadingMore = ref(false)

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

  /** append 形态写入一批：追加进 `itemsRef`，并按「本批是否满一批」定 `hasMore`。 */
  function appendBatch(res: unknown): void {
    const batch = pickBatch(res)
    if (!batch) {
      hasMore.value = false
      return
    }
    const target = options.itemsRef?.value
    if (Array.isArray(target)) target.push(...batch)
    hasMore.value = batch.length >= batchSize
  }

  /** 装载（首屏/翻页/筛选变化共用）：错误收敛为 loadError + loadErrorKind，绝不 reject */
  async function run(): Promise<void> {
    loading.value = true
    loadError.value = false
    loadErrorKind.value = null
    if (append) hasMore.value = false
    try {
      const res = await load(page.value)
      if (append) appendBatch(res)
    } catch (error) {
      loadError.value = true
      loadErrorKind.value = kindOf(error)
      // append 形态：本批失败即停（不回退页码），由 retry 重跑同一批
      if (append) hasMore.value = false
    } finally {
      loading.value = false
      retrying.value = false
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

  /** 清空累积并回第 1 批（append 形态的「刷新」/筛选重置）。 */
  function reset(): void {
    page.value = 1
    clearItems()
    hasMore.value = append
    void run()
  }

  /** 就地清空 `itemsRef`（保持 ref 引用不变，页面无需换数组）。 */
  function clearItems(): void {
    const target = options.itemsRef?.value
    if (Array.isArray(target)) target.length = 0
  }

  /**
   * append 形态的「加载更多」：推进页码并追加下一批。`loadingMore` 防重入；
   * 失败即停（拦截器已 toast）且**不推进页码**——已累积的条目原地保持，
   * 再点一次就是对同一批的重试（本入口不把整页翻成错误态，避免把已看到的内容换掉）。
   */
  async function loadMore(): Promise<void> {
    if (!append || loadingMore.value || loading.value || !hasMore.value) return
    loadingMore.value = true
    hasMore.value = false
    try {
      const next = page.value + 1
      appendBatch(await load(next))
      page.value = next
    } catch {
      // 拦截器已统一 toast；页码不动，再点一次即重试同一批
    } finally {
      loadingMore.value = false
    }
  }

  // #1054 筛选轴：任一变化 → 回第一页重装（声明式，单点取代每页手写）。
  if (options.filterDeps?.length) {
    watch(options.filterDeps as Array<Ref<unknown> | (() => unknown)>, () => {
      page.value = 1
      clearItems()
      hasMore.value = append
      void run()
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
