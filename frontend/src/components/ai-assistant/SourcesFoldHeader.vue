<template>
  <!-- 折叠头（诊断筛选/来源面板共用）：标题 + 展开态，单行 -->
  <div
    class="flex cursor-pointer items-center justify-between gap-2 py-0.5 text-xs text-ink-3"
    @click="emit('toggle')"
  >
    <span>▸ {{ title }}<template v-if="count !== undefined">（{{ count }} 条）</template>{{ suffix }}</span>
    <span class="whitespace-nowrap text-ui-600">{{ open ? '收起 ▴' : '展开 ▾' }}</span>
  </div>
</template>

<script setup lang="ts">
// 折叠头：当轮来源面板与诊断筛选面板的共用折叠头（消 twin 结构重复）。
withDefaults(
  defineProps<{
    title: string
    open: boolean
    count?: number
    suffix?: string
  }>(),
  { count: undefined, suffix: '' }
)

const emit = defineEmits<{
  (e: 'toggle'): void
}>()
</script>
