import { ref } from 'vue'

export type ForumSortDimension = 'latest' | 'hot'
export type ForumSortOrder = 'asc' | 'desc'

/**
 * 论坛排序双轴（#389 小重复收编）：维度（最新/热门）与方向（正序/逆序）。
 *
 * - flipOrder：方向反转按钮（列表页「正序/逆序」、详情回复「正序/逆序」共用）
 * - resetOrder：切维度时回默认方向（热门/最新优先）
 *
 * 维度→方向的映射口径两页不同（列表固定 desc；详情回复「热门逆序、最新正序」），
 * 属页面语义，由调用方按自身口径写 order.value。
 */
export function useForumSort(defaultOrder: ForumSortOrder = 'desc') {
  const sort = ref<ForumSortDimension>('latest')
  const order = ref<ForumSortOrder>(defaultOrder)

  function flipOrder(): void {
    order.value = order.value === 'asc' ? 'desc' : 'asc'
  }

  function resetOrder(): void {
    order.value = defaultOrder
  }

  return { sort, order, flipOrder, resetOrder }
}
