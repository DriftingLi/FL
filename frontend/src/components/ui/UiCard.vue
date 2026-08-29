<script setup lang="ts">
/**
 * 基础卡片容器
 *
 * 自写而非包 `el-card`：el-card 的圆角与内边距和视觉规范冲突（见方案第 9 节）。
 *
 * 约定（R1）：模板只用 Tailwind 原子类，不硬编码色值。
 * 颜色 / 圆角 / 阴影一律走 tailwind.css 的 @theme 别名（panel / line / ink / rounded-card /
 * shadow-card），最终解析到 design-tokens.css 的 token，改 token 即可整体换肤。
 */
const props = withDefaults(
  defineProps<{
    /** flat 描边卡片；raised 带投影；interactive 可点击，带 hover / tap 反馈 */
    variant?: 'flat' | 'raised' | 'interactive'
    padding?: 'none' | 'sm' | 'md' | 'lg'
  }>(),
  { variant: 'flat', padding: 'md' }
)

const PADDING: Record<string, string> = {
  none: '',
  sm: 'p-3',
  md: 'p-5',
  lg: 'p-6'
}

const VARIANT: Record<string, string> = {
  flat: 'shadow-card',
  raised: 'shadow-raised',
  // ui-card-interactive 是语义钩子：global.css 的移动端 44px 触控规则靠它选中可点卡片，
  // 纯原子类组合无法被外部规则稳定命中。时长对齐 --duration-tap（120ms）。
  interactive:
    'ui-card-interactive shadow-card cursor-pointer transition-[box-shadow,transform] duration-[120ms] ease-[var(--ease-default)] hover:-translate-y-px hover:shadow-raised active:translate-y-0'
}
</script>

<template>
  <section
    class="rounded-card border border-line bg-panel"
    :class="[VARIANT[props.variant], PADDING[props.padding]]"
    :tabindex="props.variant === 'interactive' ? 0 : undefined"
    :role="props.variant === 'interactive' ? 'button' : undefined"
  >
    <header
      v-if="$slots.header || $slots.actions"
      class="mb-4 flex items-center justify-between gap-3"
    >
      <div class="min-w-0">
        <slot name="header" />
      </div>
      <div v-if="$slots.actions" class="flex shrink-0 items-center gap-2">
        <slot name="actions" />
      </div>
    </header>

    <slot />

    <footer v-if="$slots.footer" class="mt-4">
      <slot name="footer" />
    </footer>
  </section>
</template>
