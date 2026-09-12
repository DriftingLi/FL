<template>
  <!-- 折叠头（当轮来源面板与逐轮来源回放共用）：标题 + 展开态，单行 -->
  <div
    class="flex cursor-pointer items-center justify-between gap-2 py-0.5 text-xs text-ink-3"
    @click="emit('toggle')"
  >
    <span class="inline-flex items-center gap-1">
      <span
        class="inline-block text-[10px] transition-transform duration-[var(--duration-fast)] ease-[var(--ease-default)]"
        :class="open ? 'rotate-90' : ''"
      >▶</span>
      {{ title }}<template v-if="count !== undefined">（{{ count }} 条）</template>
    </span>
    <span class="whitespace-nowrap text-ui-600">{{ open ? '收起 ▴' : '展开 ▾' }}</span>
  </div>
</template>

<script setup lang="ts">
// 折叠头：当轮来源面板与逐轮来源回放的共用折叠头（消 twin 结构重复）。
withDefaults(
  defineProps<{
    title: string
    open: boolean
    count?: number
  }>(),
  { count: undefined }
)

const emit = defineEmits<{
  (e: 'toggle'): void
}>()
</script>
