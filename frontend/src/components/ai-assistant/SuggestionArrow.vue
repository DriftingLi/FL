<template>
  <!-- 翻页箭头（方案 B 预设提示词翻页；禁用态走原子类 disabled: 变体） -->
  <button
    class="flex h-7 w-7 shrink-0 cursor-pointer items-center justify-center rounded-pill border border-line bg-panel text-base leading-none text-ink-3 transition-colors duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:border-ui-400 hover:text-ui-600 disabled:cursor-default disabled:opacity-35"
    :disabled="disabled"
    :title="dir === 'prev' ? '上一页' : '下一页'"
    @click="emit('page')"
  >
    {{ dir === 'prev' ? '‹' : '›' }}
  </button>
</template>

<script setup lang="ts">
// 预设提示词翻页箭头：两个实例曾是 twin 按钮，抽成组件消重。
withDefaults(
  defineProps<{
    dir: 'prev' | 'next'
    disabled?: boolean
  }>(),
  { disabled: false }
)

const emit = defineEmits<{
  (e: 'page'): void
}>()
</script>
