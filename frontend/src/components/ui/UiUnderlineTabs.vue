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
 * ARIA 与 `UiSegmentTabs` 同口径（role=tablist/tab + aria-selected），**方向键漫游没有做**：
 * 两件是同一类控件，要补应当一起补；只给这一件加会让「分段控件」这一族出现两套键盘行为。
 */
export interface UiUnderlineOption {
  label: string
  value: string
}

const props = withDefaults(
  defineProps<{
    modelValue: string
    options: UiUnderlineOption[]
    disabled?: boolean
  }>(),
  {
    disabled: false
  }
)

const emit = defineEmits<{
  (e: 'update:modelValue', v: string): void
  (e: 'change', v: string): void
}>()

function select(option: UiUnderlineOption) {
  if (props.disabled) return
  emit('update:modelValue', option.value)
  emit('change', option.value)
}
</script>

<template>
  <div class="ui-underline-tabs flex items-center gap-0.5" role="tablist">
    <button
      v-for="opt in options"
      :key="opt.value"
      type="button"
      role="tab"
      :aria-selected="modelValue === opt.value"
      :disabled="disabled"
      class="-mb-px cursor-pointer border-0 border-b-2 bg-transparent px-2.5 py-2 text-sm font-[inherit] transition-colors duration-[var(--duration-base)] ease-[var(--ease-default)] disabled:cursor-not-allowed disabled:opacity-50"
      :class="
        modelValue === opt.value
          ? 'border-ui-500 text-ui-600'
          : 'border-transparent text-ink-3 hover:text-ink-2'
      "
      @click="select(opt)"
    >
      {{ opt.label }}
    </button>
  </div>
</template>
