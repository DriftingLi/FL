import { computed, ref, watch, type Ref } from 'vue'
import { useCredentialStore } from '@/stores/credential'

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
   * 筛选轴（#1054）：任一 ref 变化 → 回第一页重装一次。声明式入口取代每页手写的
   * 「筛选条件变了要回到第一页」（此前 26 处手写、漏写某条筛选轴用户就会停在
   * 越界页码上看到空列表）。同一同步块内多轴齐变只触发一次回调（Vue watch 批处理）；
   * 与证件切换（credentialScoped）是两条独立 watch，同时发生时各触发一次
   * ——与既有「证件切换即重装」行为一致，不在本入口里合并去重。
   */
  filterDeps?: Array<Ref<unknown>>
  /**
   * 列表条目 ref（loader 装载后写入的那个数组；#1054）：提供即生成 `isEmpty` 空态判据
   * ——「加载成功且无数据」才为真，错误/装载中恒为 false（四段式里空态的合法位置）。
   * 判据形状特殊的页面（如以 total 为准）可不传，自行声明后直接喂给
   * UiAsyncSection 的 `empty` prop——组件的空态判定本来就是外部声明。
   */
  itemsRef?: Ref<{ length: number } | undefined | null>
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
 * - 筛选轴失效刷新（#1054）：`filterDeps` 声明筛选轴 refs，任一变化回第一页重装；
 *   `itemsRef` 提供后生成 `isEmpty` 空态判据（加载成功且无数据），直接喂给
 *   UiAsyncSection 的 `empty` prop。
 *
 * 不分页的页面（详情/聚合页）只解构三态部分即可，分页字段闲置无害。
 */
export function useAsyncPage(load: () => Promise<unknown>, options: UseAsyncPageOptions = {}) {
  const loading = ref(false)
  const loadError = ref(false)
  const retrying = ref(false)
  const page = options.pageRef ?? ref(1)
  const pageSize = ref(options.defaultPageSize ?? 20)
  const total = ref(0)

  /** 装载（首屏/翻页/筛选变化共用）：错误收敛为 loadError，绝不 reject */
  async function run(): Promise<void> {
    loading.value = true
    loadError.value = false
    try {
      await load()
    } catch {
      loadError.value = true
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

  // #1054 筛选轴：任一变化 → 回第一页重装（声明式，单点取代每页手写）。
  if (options.filterDeps?.length) {
    watch(options.filterDeps, () => {
      page.value = 1
      void run()
    })
  }

  // #1054 空态判据：加载成功且无数据。错误态与装载中恒为 false——
  // 空态在四段式里的合法位置只在「装载完成且确实没有条目」时。
  // 判据 opt-in：未提供 itemsRef 时恒 false（判据形状特殊的页面自行声明后
  // 直接喂给 UiAsyncSection 的 empty prop），不替调用方猜「没有数据=空」。
  const isEmpty = computed(() => {
    const items = options.itemsRef?.value
    if (!options.itemsRef || loading.value || loadError.value) return false
    return !items || items.length === 0
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
    loadError,
    retrying,
    isEmpty,
    run,
    retry,
    page,
    pageSize,
    total,
    handlePageChange,
    handleSizeChange
  }
}
