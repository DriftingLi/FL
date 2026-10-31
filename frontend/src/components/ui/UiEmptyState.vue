<script setup lang="ts">
import type { IconName } from '@/icons/registry'
import UiIcon from './UiIcon.vue'
import UiButton from './UiButton.vue'

/**
 * 空状态（**独立占内容区**的那种：整页 / 列表 / 面板）。
 *
 * #2156 空态批：不再包 `el-empty`。它的出厂灰色插画占位全站从未调过（element-overrides.css
 * 里没有一条 `.el-empty`），看起来像页面没做完；媒体位改由调用方给一个图标名，
 * 不传就不渲染媒体位（空态不只是"没数据"，也可以是"还没开始"）。
 *
 * 边界（docs/agents/ui-conventions.md「空态两级」）：卡片正文内嵌的一行提示**不进本组件**，
 * 保持纯文案即可 —— 换成组件会多出图标与整块留白，比问题本身更重。
 *
 * 纯增量纪律（ADR-0038）：`title` / `description` / `size` / `actionText` 语义不变，
 * 只换内部结构，现有调用方零改动。
 */
const props = withDefaults(
  defineProps<{
    title?: string
    description?: string
    /** sm = 面板内嵌；md = 整页/列表主空态 */
    size?: 'sm' | 'md'
    actionText?: string
    /** 媒体位图标（走图标唯一入口 `UiIcon` 的注册名）；不传则不渲染媒体位 */
    icon?: IconName
  }>(),
  { size: 'md' }
)

const emit = defineEmits<{ action: [] }>()
</script>

<template>
  <div
    data-slot="empty"
    class="flex flex-col items-center justify-center text-center"
    :class="props.size === 'sm' ? 'gap-2 py-6' : 'gap-3 py-10'"
  >
    <div
      v-if="props.icon"
      data-slot="empty-media"
      class="flex shrink-0 items-center justify-center rounded-ctl bg-canvas text-ink-3"
      :class="props.size === 'sm' ? 'size-10' : 'size-12'"
    >
      <UiIcon :name="props.icon" :class="props.size === 'sm' ? 'text-lg' : 'text-xl'" />
    </div>

    <div class="flex max-w-sm min-w-0 flex-col items-center gap-1">
      <h3
        v-if="props.title"
        data-slot="empty-title"
        class="font-heading text-base font-semibold text-balance text-ink"
      >
        {{ props.title }}
      </h3>
      <p
        v-if="props.description"
        data-slot="empty-description"
        class="m-0 text-sm leading-snug text-balance text-ink-3"
      >
        {{ props.description }}
      </p>
    </div>

    <div v-if="props.actionText || $slots.default" class="mt-1">
      <UiButton v-if="props.actionText" variant="primary" @click="emit('action')">
        {{ props.actionText }}
      </UiButton>
      <slot />
    </div>
  </div>
</template>
