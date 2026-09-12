<script setup lang="ts">
/**
 * 分页器：包 el-pagination，统一默认 layout、间距与观感（ADR-0035 控件收敛）。
 *
 * 默认值 = 现状 33 处裸用中最常见的写法（R2 纯增量）：
 * - layout = total + prev/pager/next（15 处）；showSizes 默认关（仅 9 处需要）
 * - pageSize 默认 20（写死值的众数；25 处绑页面自己的 pageSize 变量、不显示 sizes 即无同步问题）
 * - 对齐默认左（现状 31 处无包裹直接放；原居中的 2 处传 align="center"）
 * 刻意的视觉变更：background 统一开启（现状仅 2 处开），让选中页背景观感全站一致。
 *
 * 状态不归这里管：继续由页面或 useAsyncPage（39 处）/ useAdminTable 持有，
 * 契约对齐 useAsyncPage 的 page/pageSize/total 三件套 —— 调用方只换标签、不改状态代码。
 */
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    /** 总条数（必填） */
    total: number
    /** 是否显示「共 X 条」 */
    showTotal?: boolean
    /** 是否显示每页条数选择器 */
    showSizes?: boolean
    pageSizes?: number[]
    /** 是否显示跳页输入框 */
    showJumper?: boolean
    /** 选中页背景块（全站统一开启） */
    background?: boolean
    disabled?: boolean
    /** 小尺寸（EP small，用于紧凑场景如打卡页） */
    small?: boolean
    align?: 'left' | 'center' | 'right'
  }>(),
  {
    showTotal: true,
    showSizes: false,
    pageSizes: () => [10, 20, 50, 100],
    showJumper: false,
    background: true,
    disabled: false,
    small: false,
    align: 'left'
  }
)

const page = defineModel<number>('currentPage', { default: 1 })
const size = defineModel<number>('pageSize', { default: 20 })

const emit = defineEmits<{
  /** 与 el-pagination 同名透传，调用方沿用 @current-change 习惯 */
  (e: 'current-change', v: number): void
  (e: 'size-change', v: number): void
}>()

const layout = computed(() => {
  const parts: string[] = []
  if (props.showTotal) parts.push('total')
  if (props.showSizes) parts.push('sizes')
  if (props.showJumper) parts.push('jumper')
  parts.push('prev', 'pager', 'next')
  return parts.join(', ')
})
</script>

<template>
  <div
    class="flex items-center"
    :class="{ 'justify-center': props.align === 'center', 'justify-end': props.align === 'right' }"
  >
    <el-pagination
      v-model:current-page="page"
      v-model:page-size="size"
      :total="props.total"
      :layout="layout"
      :page-sizes="props.pageSizes"
      :background="props.background"
      :disabled="props.disabled"
      :small="props.small"
      @current-change="emit('current-change', $event)"
      @size-change="emit('size-change', $event)"
    />
  </div>
</template>
