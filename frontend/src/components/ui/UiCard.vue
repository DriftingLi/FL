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
    /**
     * flat 描边卡片；raised 带投影；interactive 带 hover / tap 反馈。
     *
     * ⚠️ interactive **只给视觉反馈**，不自任按钮（#2156）：改前它挂 `role="button"` 与
     * `tabindex="0"` 却没有 Enter/Space 处理，读屏播报「按钮」而键盘按不动。可点由**调用方**用
     * 真实的 `<button>` / `<a>` 包住本卡片承担；卡片自己始终是容器语义。
     */
    variant?: 'flat' | 'raised' | 'interactive'
    padding?: 'none' | 'sm' | 'base' | 'md' | 'lg'
    /**
     * 头部下的分隔线。开着时 header 行下方补一条 `border-line` + 内缩的下内边距，
     * 用来区分「标题行」与「内容区」（管理端列表卡、图表卡的常态）。
     *
     * 与 `el-card` 的 header 分隔线差别只在**端点**：EP 那条是通栏的，
     * 这里跟着卡片内边距内缩（#1974 迁移时的统一口径 —— 通栏线要么靠负 margin 伪造，
     * 要么把 header 移出 padded 区，两种都得给卡片加结构性 API，不值得）。
     */
    divider?: boolean
  }>(),
  { variant: 'flat', padding: 'md', divider: false }
)

const PADDING: Record<string, string> = {
  none: '',
  sm: 'p-3',
  // 16px：存量卡片最常用的一档（components/practice 四个卡片都是 16px），
  // 夹在 sm(12) 与 md(20) 之间，缺它会导致收敛时被迫改间距。默认值仍是 md。
  base: 'p-4',
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
  >
    <header
      v-if="$slots.header || $slots.actions"
      class="flex items-center justify-between gap-3"
      :class="props.divider ? 'mb-4 border-b border-line pb-4' : 'mb-4'"
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
