<script setup lang="ts">
import { computed, useSlots } from 'vue'
import UiErrorState from './UiErrorState.vue'
import UiSkeleton from './UiSkeleton.vue'

/**
 * 列表四段式合成组件（#1054）：错误态 → 骨架 → 空态 → 内容 的**渲染优先级只有这一份实现**。
 *
 * 收敛前这条四分支链在 39 个页面里各手写一遍（且顺序不一：有的错误优先于骨架、
 * 有的骨架优先于错误——后者在「失败且正在重试」时会先闪骨架），迁移后页面只提供
 * 四个状态 + 四个插槽（`default` / `empty` / `error` / `skeleton`），先后与互斥由本组件负责：
 *
 * 1. `empty` 为真（且非加载中）→ `#empty` 插槽 —— **404 归空态**（#1101）：
 *    `useAsyncPage` 的 `isEmpty` 在「资源不存在」时同样为真，页面不必自建 404 分支；
 * 2. 否则 `error` 为真 → `#error` 插槽（未提供时用 UiErrorState，`retry` 事件 +
 *    `retrying` 禁用防连点）；
 * 3. 否则 `loading` 且非表格档 → `#skeleton` 插槽（未提供时给一个默认列表骨架）；
 * 4. 否则渲染默认插槽（内容）。
 *
 * - 表格档（`skeleton: false`）：加载态交给表格自带的加载遮罩（内容保持渲染，
 *   管理端十多个表格页的既有观感不变），错误态与空态仍走本组件——见 #1054 的
 *   Implementation Decisions：不把管理页的加载观感改成骨架。
 * - `empty` 由调用方声明（`useAsyncPage` 返回的 `isEmpty` 是默认路径，页面不再手写判据），
 *   本组件不自己数数据——空态判定只在「加载完成且确实没有条目」时成立，防御性要求
 *   `empty && loading` 时按加载处理。两个状态同真时**空态优先**（404 归空态的落点）。
 * - 空态与骨架组件复用现有封装（UiEmptyState / UiSkeleton），页面把原有的那份原样搬进
 *   对应插槽即可（零视觉变更的迁移姿势）。
 * - 容器样式（min-h、背景卡等）**放在页面自己的容器元素上**，不要挂到本组件——
 *   本组件是多分支模板，不继承 attrs（`inheritAttrs: false`）。
 */
defineOptions({ inheritAttrs: false })

const props = withDefaults(
  defineProps<{
    loading?: boolean
    error?: boolean
    empty?: boolean
    retrying?: boolean
    /** false = 表格档：加载态交给表格自带遮罩，内容保持渲染（默认 true 走骨架） */
    skeleton?: boolean
    errorTitle?: string
    errorDescription?: string
    retryText?: string
  }>(),
  {
    loading: false,
    error: false,
    empty: false,
    retrying: false,
    skeleton: true
  }
)

const emit = defineEmits<{ retry: [] }>()

const slots = useSlots()
/** 页面提供 `#error` 时不渲染内置错误态（自定义错误态的出口）。 */
const hasErrorSlot = computed(() => Boolean(slots.error))
</script>

<template>
  <slot v-if="props.empty && !props.loading" name="empty" />

  <template v-else-if="props.error">
    <slot name="error" />
    <UiErrorState
      v-if="!hasErrorSlot"
      :title="props.errorTitle"
      :description="props.errorDescription"
      :retrying="props.retrying"
      @retry="emit('retry')"
    />
  </template>

  <template v-else-if="props.loading && props.skeleton">
    <slot name="skeleton">
      <UiSkeleton variant="list" />
    </slot>
  </template>

  <slot v-else />
</template>
