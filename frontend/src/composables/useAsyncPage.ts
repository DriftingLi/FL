import { ref, type Ref } from 'vue'

export interface UseAsyncPageOptions {
  /** 默认页大小（分页场景；缺省 20） */
  defaultPageSize?: number
  /** 外部持有的页码 ref（如论坛页按类别分片存储页码）；缺省内部自建 */
  pageRef?: Ref<number>
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

  return { loading, loadError, retrying, run, retry, page, pageSize, total, handlePageChange, handleSizeChange }
}
