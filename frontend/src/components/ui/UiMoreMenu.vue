<script setup lang="ts">
import UiIcon from '@/components/ui/UiIcon.vue'
import UiButton from '@/components/ui/UiButton.vue'
/**
 * 溢出菜单：低治理动作的收纳入口。
 *
 * 边界（docs/agents/ui-conventions.md「溢出菜单」）：收的是举报 / 删除这类**低频、破坏性、
 * 非互动**的操作。**互动动作（回复 / 点赞）不进菜单** —— 它们留在卡片底部主操作行。
 *
 * 菜单项与**可见性判定都归调用方**：组件不认识任何业务语义（「自己的回复不显示举报」
 * 是调用方的规则，不是这里的 if）；items 为空时连触发按钮都不渲染，省掉一个永远点不出东西的入口。
 *
 * 两档触发（#1978 裁定 B —— 「档不是新组件」，ADR-0083）：
 * - `icon`（默认）：卡片/列表项右上角的「⋯」，靠 `label` 给读屏名；
 * - `text`：表格「操作」列的「操作 ▾」，管理端 10 个列表页的现有形态。
 *
 * 为什么文字档不给 `icon`/`disabled`：盘了 11 个落点，没有一处用到菜单项图标或置灰
 * （不可见的项由调用方直接不放进 items），先不加没人用的 API。
 */

export interface UiMoreMenuItem {
  key: string
  label: string
  /** danger：删除等破坏性动作（红字，与 UiActionChip 的 danger 同语义） */
  tone?: 'default' | 'danger'
  /** 该项之前加一条分隔线（承接 EP 的 divided，删除项常用） */
  divided?: boolean
}

withDefaults(defineProps<{
  items?: UiMoreMenuItem[]
  /** 无障碍标签（图标档只有 ⋯，读屏靠它） */
  label?: string
  /** 触发档：icon = ⋯ 图标（默认）/ text = 文字 + chevron */
  trigger?: 'icon' | 'text'
  /** 文字档的触发文案 */
  triggerText?: string
}>(), {
  items: () => [],
  label: '更多操作',
  trigger: 'icon',
  triggerText: '操作'
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
    <UiButton v-if="trigger === 'text'" variant="primary" link size="small">
      {{ triggerText }}<UiIcon name="chevron-down" class="el-icon--right" />
    </UiButton>

    <button
      v-else
      type="button"
      class="more-menu-trigger inline-flex cursor-pointer items-center justify-center rounded-[6px] border-0 bg-transparent p-1 text-base leading-none text-ink-3 transition-colors duration-[var(--duration-tap)] ease-[var(--ease-default)] hover:bg-canvas hover:text-ink-2"
      :aria-label="label"
      :title="label"
    >
      <UiIcon name="ellipsis" />
    </button>

    <template #dropdown>
      <el-dropdown-menu>
        <el-dropdown-item
          v-for="it in items"
          :key="it.key"
          :command="it.key"
          :divided="it.divided"
          :class="it.tone === 'danger' ? 'text-bad' : ''"
        >
          {{ it.label }}
        </el-dropdown-item>
      </el-dropdown-menu>
    </template>
  </el-dropdown>
</template>
