<script setup lang="ts">
/**
 * 按钮：包 `el-button`，收敛 variant / block / 图标写法。
 *
 * 颜色与圆角沿用 element-overrides.css 对 `.el-button--primary` 的渐变定义，
 * 此处不覆盖色值，只做语义化封装。
 */
const props = withDefaults(
  defineProps<{
    /**
     * secondary 为默认值；ghost 走 EP 的 plain 样式（带描边）；
     * text 走 EP 的 text 样式（无边框、无背景，常用于行内/导航操作）。
     * text 与 danger 等色调可叠加：`<UiButton variant="text" class="text-bad">删除</UiButton>`
     * —— 原子类在 utilities 层，天然压过 vendor 层的 .el-button--danger.is-text，无需 !important。
     */
    variant?: 'primary' | 'secondary' | 'ghost' | 'text' | 'danger' | 'success' | 'warning'
    size?: 'small' | 'default' | 'large'
    loading?: boolean
    block?: boolean
    /**
     * 图标：既接受全局注册名（字符串），也接受组件引用（`:icon="Plus"`）。
     * 存量 el-button 两种写法都在用，故类型放宽 —— 两者都能直接喂给 `<component :is>`。
     */
    icon?: string | object
    /** 圆形图标按钮（EP 的 circle），仅图标、无文字时用 */
    circle?: boolean
    /**
     * 链接态（EP 的 link）：无边框无背景、保留 variant 的色调。
     * 与 variant 组合使用：`<UiButton variant="primary" link>下载</UiButton>`。
     */
    link?: boolean
    /**
     * 朴素态（EP 的 plain）：淡底 + 描边，保留 variant 的色调。
     *
     * 与 `ghost` 变体的区别：ghost 是 **变体值**（plain + 默认色，无法指定色调），
     * 而本 prop 是**修饰开关**，可与任意 variant 组合 —— 存量里有 `plain type="primary"`、
     * `plain type="warning"` 这类写法，只有布尔修饰能表达。
     */
    plain?: boolean
    /**
     * 文字态（EP 的 text）：无边框无背景，保留 variant 的色调。
     * 与 `link` 同属修饰开关，可与任意 variant 组合；存量有 `text type="primary"` 写法。
     * 单独使用时直接写 `variant="text"` 更简洁（等价于 text + 默认色）。
     */
    text?: boolean
  }>(),
  {
    variant: 'secondary',
    size: 'default',
    loading: false,
    block: false,
    circle: false,
    link: false,
    plain: false,
    text: false
  }
)

const EP_TYPE: Record<string, 'primary' | 'default' | 'danger' | 'success' | 'warning'> = {
  primary: 'primary',
  secondary: 'default',
  ghost: 'default',
  text: 'default',
  danger: 'danger',
  success: 'success',
  warning: 'warning'
}
</script>

<template>
  <el-button
    :type="EP_TYPE[props.variant]"
    :size="props.size"
    :loading="props.loading"
    :plain="props.variant === 'ghost' || props.plain"
    :text="props.variant === 'text' || props.text"
    :link="props.link"
    :circle="props.circle"
    :class="props.block ? 'w-full' : undefined"
  >
    <el-icon v-if="props.icon">
      <component :is="props.icon" />
    </el-icon>
    <slot />
  </el-button>
</template>
