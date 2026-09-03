<script setup lang="ts">
/**
 * 输入框：包 `el-input`，统一 size 语义。
 * 圆角与聚焦环由 element-overrides.css 的 `.el-input__wrapper` 统一处理，
 * 这里不重复声明样式，只做 v-model 与 size 的收口。
 *
 * 约定（R2 纯增量）：variant / maxlength / showWordLimit / autosize 都是后加的，
 * 默认值一律对齐改造前行为 —— boxed 走 EP 原厂外观、不传 maxlength 就不显示计数，
 * 让已经在本组件上的存量调用方零 diff。
 */
const value = defineModel<string>({ default: '' })

withDefaults(
  defineProps<{
    placeholder?: string
    size?: 'small' | 'default' | 'large'
    disabled?: boolean
    clearable?: boolean
    /** textarea / password 等；默认 text，与 el-input 自身默认值一致 */
    type?: 'text' | 'textarea' | 'password'
    /** 仅 type="textarea" 生效。不传时 el-input 取自身默认值 2 */
    rows?: number
    /** 最大输入长度。EP 的 show-word-limit 依赖它，不传则无计数 */
    maxlength?: number
    /** 右下角显示「已输入/上限」计数，需配合 maxlength */
    showWordLimit?: boolean
    /** 高度随内容自适应，仅 type="textarea" 生效（EP 的 autosize） */
    autosize?: boolean | { minRows?: number; maxRows?: number }
    /**
     * boxed：EP 原厂带边框外观（默认，与改造前一致）
     * bare ：去边框去阴影、透明底，聚焦反馈交给外层容器的 focus-within
     *        —— 论坛输入区用它，让高亮落在外层卡片边框上而不是文本框本身。
     */
    variant?: 'boxed' | 'bare'
  }>(),
  {
    size: 'default',
    disabled: false,
    clearable: false,
    type: 'text',
    showWordLimit: false,
    variant: 'boxed'
  }
)
</script>

<template>
  <el-input
    v-model="value"
    :class="variant === 'bare' ? 'ui-input-bare' : undefined"
    :placeholder="placeholder"
    :size="size"
    :disabled="disabled"
    :clearable="clearable"
    :type="type"
    :rows="rows"
    :maxlength="maxlength"
    :show-word-limit="showWordLimit"
    :autosize="autosize"
  />
</template>

<style scoped>
/*
 * bare 形态（R1 允许的 :deep）：清掉 EP 自带的边框与内阴影。
 *
 * 必须把 hover / focus 态一并列出 —— EP 的 .el-textarea__inner:hover 与
 * .is-focus 特异性更高，只写基础态盖不住，聚焦时会重新冒出一圈 1px 边框。
 * 这里靠选择器数量取胜，不用 !important（base 层的 !important 会反过来
 * 压过 utilities 层的 `!` 前缀工具类，见 tailwind.css 顶部注释）。
 *
 * 未层叠的 scoped 样式天然高于 @layer vendor 里的 EP 规则，无需提权。
 */
.ui-input-bare :deep(.el-textarea__inner),
.ui-input-bare :deep(.el-textarea__inner:hover),
.ui-input-bare :deep(.el-textarea__inner:focus),
.ui-input-bare :deep(.el-input__wrapper),
.ui-input-bare :deep(.el-input__wrapper:hover),
.ui-input-bare :deep(.el-input__wrapper.is-focus) {
  border: none;
  box-shadow: none;
  padding: 0;
  background: transparent;
}

.ui-input-bare :deep(.el-textarea__inner) {
  resize: none;
}
</style>
