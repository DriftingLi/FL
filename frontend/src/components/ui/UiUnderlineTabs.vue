<script setup lang="ts">
/**
 * 下划线选项卡（underline tabs）：给编辑器顶栏用，切换**看什么**（编写 / 预览）。
 *
 * 与「分段控件」`UiSegmentTabs` 的分工（docs/agents/ui-conventions.md）：
 *   数据档位（会被保存的选择，如「纯文本 | Markdown」）→ `UiSegmentTabs`（胶囊 + 实心品牌色滑块）；
 *   视图档位（临时切换看什么，如「编写 | 预览」）→ 本组件（下划线，更轻，可与同一行的工具栏共存）。
 * 两者不可互换：同一个卡片里并排两组同款胶囊，用户分不清哪组会永久保存。
 *
 * ⚠️ 配色约束（改动前必读）：
 * 1. 激活项下划线**必须**用品牌语义色（`--color-ui-*`）。不能用 `bg-panel`/`border-panel` ——
 *    它与最常见的父容器（卡片）同值，放进去彻底隐身（浅色 #FFFFFF on #FFFFFF、深色 #1E293B on #1E293B）。
 * 2. 项目刻意不引 Tailwind preflight：button 保留浏览器默认不透明背景（ButtonFace 浅灰），
 *    必须显式 `bg-transparent`，否则顶栏会出现一块灰底（UiSegmentTabs 踩过同一个坑）。
 * 3. `-mb-px` 是为了让激活下划线压住父级顶栏的 1px 底边（调用方请给顶栏 `border-b`）。
 *
 * 不做右侧插槽：顶栏 = `flex` 行里「本组件 + 工具栏」，由调用方拼，组件不替调用方决定间距。
 *
 * ARIA 契约（#2156）：role=tablist/tab + aria-selected 保留，并补齐了选项卡该有的键盘行为——
 * 方向键移动并切换、roving tabindex（整组只有一个 Tab 落点）、Home/End 到两端。
 * 给了 `panelId` 时每个 tab 挂 `aria-controls` 与稳定 id，调用方据此给内容区挂 `role="tabpanel"`
 * 与 `aria-labelledby`；不给则不渲染关联属性（没有面板就不假称有）。
 *
 * 与 `UiSegmentTabs` 的差别在**角色**而不在键盘手感：那边是单选组（radiogroup，选一个参数），
 * 这边是选项卡（tablist，换一块视图区）。两件都支持方向键，同一族不会出现两套手感。
 */
import { useRovingSelection, type RovingOption } from '@/composables/useRovingSelection'

export type UiUnderlineOption = RovingOption

const props = withDefaults(
  defineProps<{
    modelValue: string
    options: UiUnderlineOption[]
    disabled?: boolean
    /**
     * 它所控制的内容区 id（#2156）。给了就给每个 tab 挂 `aria-controls` 与稳定的 `id`，
     * 调用方据此给内容区挂 `role="tabpanel"` + `aria-labelledby="<panelId>-tab-<当前值>"`。
     * 不给则不渲染关联属性 —— 没有面板就不假称有。
     */
    panelId?: string
  }>(),
  {
    disabled: false,
    panelId: ''
  }
)

const emit = defineEmits<{
  (e: 'update:modelValue', v: string): void
  (e: 'change', v: string): void
}>()

/** roving tabindex 与方向键手感由 `useRovingSelection` 统一提供（与分段控件同族）。 */
const { activeValue, setBtnRef, onKeydown } = useRovingSelection<UiUnderlineOption>({
  options: () => props.options,
  current: () => props.modelValue,
  disabled: () => props.disabled,
  select: (opt) => select(opt)
})

function select(option: UiUnderlineOption) {
  if (props.disabled) return
  emit('update:modelValue', option.value)
  emit('change', option.value)
}
</script>

<template>
  <div class="ui-underline-tabs flex items-center gap-0.5" role="tablist">
    <button
      v-for="(opt, i) in options"
      :key="opt.value"
      :ref="(el) => setBtnRef(el, i)"
      type="button"
      role="tab"
      :id="panelId ? `${panelId}-tab-${opt.value}` : undefined"
      :aria-controls="panelId || undefined"
      :aria-selected="modelValue === opt.value"
      :tabindex="activeValue === opt.value ? 0 : -1"
      :disabled="disabled"
      class="-mb-px cursor-pointer rounded-t-ctl border-0 border-b-2 bg-transparent px-2.5 py-2 text-sm font-[inherit] transition-colors duration-[var(--duration-base)] ease-[var(--ease-default)] focus-visible:ring-2 focus-visible:ring-ui-500/60 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
      :class="
        modelValue === opt.value
          ? 'border-ui-500 text-ui-soft-ink'
          : 'border-transparent text-ink-3 hover:text-ink-2'
      "
      @click="select(opt)"
      @keydown="onKeydown($event, i)"
    >
      {{ opt.label }}
    </button>
  </div>
</template>
