<template>
  <!-- 小胶囊（方案 B 工具栏）：图标 + 文字 + 可选角标，单行不换行 -->
  <button
    type="button"
    class="flex shrink-0 cursor-pointer items-center gap-1.5 whitespace-nowrap rounded-pill border border-line bg-panel px-3.5 py-1.5 text-[13px] text-ink-2 transition-all duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:border-ui-400 hover:bg-ui-50 hover:text-ui-600"
    :class="active ? 'border-ui-600 bg-ui-50 font-semibold text-ui-600' : ''"
    @click="emit('click', $event)"
  >
    <el-icon v-if="iconComponent" :size="14"><component :is="iconComponent" /></el-icon>
    <slot>{{ label }}</slot>
    <i v-if="badge" class="free-preview-badge">{{ badge }}</i>
  </button>
</template>

<script setup lang="ts">
// 小胶囊：AI 助手主界面功能入口与诊断筛选/快捷问的共用形态（消 twin class 串重复）。
import { computed } from 'vue'
import * as EPIcons from '@element-plus/icons-vue'
import type { Component } from 'vue'

const props = withDefaults(
  defineProps<{
    label?: string
    /** EP 图标组件引用（与 AIAssistantPage/UiActionChip 同形）；字符串则按全局名解析 */
    icon?: string | Component
    badge?: string
    active?: boolean
  }>(),
  { label: '', badge: '', active: false }
)

const emit = defineEmits<{
  (e: 'click', ev: MouseEvent): void
}>()

/** 图标归一化：组件引用直用；字符串先找 EP 内置（诊断筛选 ⚙ 无内置则回退 emoji 由调用方 label 自带） */
const iconComponent = computed(() => {
  if (!props.icon) return null
  if (typeof props.icon !== 'string') return props.icon
  return (EPIcons as Record<string, Component>)[props.icon] ?? null
})
</script>

<style scoped>
/* 限免角标：沿用 AIAssistantPage 既有 token 定义（R2 冻结区外的新共用件） */
.free-preview-badge {
  font-style: normal;
  font-size: 10px;
  font-weight: 600;
  color: #fff;
  background: var(--color-success);
  border-radius: 999px;
  padding: 1px 6px;
  line-height: 1.4;
}
</style>
