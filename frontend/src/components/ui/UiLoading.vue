<script setup lang="ts">
/**
 * 加载指示：自写的块级/行内占位，用来替代一部分全屏 `v-loading`
 * （全屏遮罩会打断阅读节奏，局部占位能让页面骨架先立起来）。
 */
const props = withDefaults(
  defineProps<{
    size?: 'sm' | 'md' | 'lg'
    tip?: string
    /** true 撑满父容器并带最小高度；false 为行内指示 */
    block?: boolean
  }>(),
  { size: 'md', block: false }
)

const ICON: Record<string, string> = { sm: 'text-base', md: 'text-xl', lg: 'text-3xl' }
const MIN_H: Record<string, string> = { sm: 'min-h-20', md: 'min-h-32', lg: 'min-h-52' }
</script>

<template>
  <div
    :class="
      props.block
        ? `flex flex-col items-center justify-center gap-2 ${MIN_H[props.size]}`
        : 'inline-flex items-center gap-2'
    "
    role="status"
    aria-live="polite"
  >
    <el-icon class="animate-spin text-ui-500" :class="ICON[props.size]">
      <Loading />
    </el-icon>
    <span v-if="props.tip" class="text-xs text-ink-3">{{ props.tip }}</span>
  </div>
</template>
