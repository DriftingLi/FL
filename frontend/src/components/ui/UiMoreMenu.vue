<script setup lang="ts">
/**
 * 溢出菜单：卡片/列表项右上角「⋯」触发的**治理动作收纳**。
 *
 * 边界（docs/agents/ui-conventions.md「溢出菜单」）：收的是举报 / 删除这类**低频、破坏性、
 * 非互动**的操作。**互动动作（回复 / 点赞）不进菜单** —— 它们留在卡片底部主操作行。
 *
 * 菜单项与**可见性判定都归调用方**：组件不认识任何业务语义（「自己的回复不显示举报」
 * 是调用方的规则，不是这里的 if）；items 为空时连触发按钮都不渲染，省掉一个永远点不出东西的入口。
 */
import { MoreFilled } from '@element-plus/icons-vue'

export interface UiMoreMenuItem {
  key: string
  label: string
  /** danger：删除等破坏性动作（红字，与 UiActionChip 的 danger 同语义） */
  tone?: 'default' | 'danger'
}

withDefaults(defineProps<{
  items?: UiMoreMenuItem[]
  /** 无障碍标签（视觉上只有 ⋯，读屏靠它） */
  label?: string
}>(), {
  items: () => [],
  label: '更多操作'
})

const emit = defineEmits<{ select: [key: string] }>()
</script>

<template>
  <el-dropdown
    v-if="items.length > 0"
    trigger="click"
    placement="bottom-end"
    @command="(key: string) => emit('select', key)"
  >
    <button
      type="button"
      class="more-menu-trigger inline-flex cursor-pointer items-center justify-center rounded-[6px] border-0 bg-transparent p-1 text-base leading-none text-ink-3 transition-colors duration-[var(--duration-tap)] ease-[var(--ease-default)] hover:bg-canvas hover:text-ink-2"
      :aria-label="label"
      :title="label"
    >
      <el-icon><MoreFilled /></el-icon>
    </button>
    <template #dropdown>
      <el-dropdown-menu>
        <el-dropdown-item
          v-for="it in items"
          :key="it.key"
          :command="it.key"
          :class="it.tone === 'danger' ? 'text-bad' : ''"
        >
          {{ it.label }}
        </el-dropdown-item>
      </el-dropdown-menu>
    </template>
  </el-dropdown>
</template>
