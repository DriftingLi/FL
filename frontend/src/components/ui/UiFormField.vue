<script setup lang="ts">
// 表单字段容器（ADR-0053 §8 票②）：标签 + 必填标记 + 错误态 + 控件插槽。
//
// 为什么落在封装层：标签 / 必填 / 错误态这三件事没有估值域专属语义，任何表单都用得上；
// 而估值表单里「同样的容器类 + 同样的标签类 + 同样的错误结构」逐字重复了 15 次。
//
// **纯增量纪律**（默认值 = 改造前行为）：`required` 与 `error` 都不传时，渲染出的 DOM
// 与改造前逐字一致——只有 `.field` 容器 + `.field-label` 标签 + 插槽内容，不多一个节点、
// 不多一个属性。故调用方不传就是零视觉变更。
//
// `.field` / `.field-label` / `:has(.form-control:disabled)` 三条规则从估值表单搬到这里：
// 作用域 CSS 的规则只作用于本组件模板内的节点，父组件里的同名规则选不到子组件渲染出的元素。
// 而 `:has()` 里的 `.form-control` 是插槽内容（在父作用域编译），故禁用态标签变灰仍然成立。
withDefaults(
  defineProps<{
    /** 字段标签文案 */
    label: string
    /** label 的 for 指向的控件 id；省略时不渲染 for（如一组开关的组标签） */
    forId?: string
    /** 是否显示必填标记（默认否：不传即为改造前行为） */
    required?: boolean
    /** 校验错误文案；空串不渲染错误行（默认即为改造前行为） */
    error?: string
  }>(),
  { forId: '', required: false, error: '' }
)
</script>

<template>
  <div class="field">
    <label class="field-label" :for="forId || undefined">
      {{ label }}
      <span v-if="required" class="field-required" aria-hidden="true">*</span>
    </label>
    <slot />
    <p v-if="error" class="field-error" role="alert">{{ error }}</p>
  </div>
</template>

<style scoped>
.field {
  min-width: 0;
  display: flex;
  flex-direction: column;
}
.field-label {
  display: block;
  font-size: 14px;
  font-weight: 500;
  margin-bottom: 6px;
  color: var(--color-text-secondary, #475569);
  font-family: var(--font-text, 'Noto Sans SC', sans-serif);
  transition: color var(--duration-base) var(--ease-default);
}
/* 控件禁用（如字典未加载完）时标签变灰：:has() 里的 .form-control 是插槽内容，
   在父作用域编译，故本规则仍能命中 */
.field:has(.form-control:disabled) .field-label {
  color: var(--color-text-muted, #94A3B8);
}
.field-required {
  color: var(--color-danger, #ef4444);
  margin-left: 2px;
}
.field-error {
  margin: 6px 0 0;
  font-size: var(--fs-xs, 12px);
  line-height: 1.4;
  color: var(--color-danger, #ef4444);
}
</style>
