<script setup lang="ts">
/**
 * 提示条：包 `el-alert`，纯透传薄封装（#900 批新增）。
 *
 * 约定（#766 C2 / ADR-0038）：不设项目默认值，attrs / 事件 / slot 全量透传给 el-alert，
 * 调用方写法与裸用一致，只换标签名。v-model 转发 `el-alert` 的关闭态（`:closable` 由调用方决定）。
 *
 * 为什么本批补这个封装：ADR-0035 的评审口径是「业务页面出现裸 EP 控件即打回」，
 * 而本批的内容渲染口径要在管理端编辑器与发布端预览里给「越界语法」提示——
 * 引入裸 `el-alert` 会开一个新的裸控件面，故按既有约定补封装层。
 */
defineOptions({ name: 'UiAlert', inheritAttrs: false })

const visible = defineModel<boolean>({ default: true })
</script>

<template>
  <el-alert v-if="visible" v-bind="$attrs">
    <template v-for="(_, name) in $slots" :key="name" #[name]="slotProps">
      <slot :name="name" v-bind="slotProps ?? {}" />
    </template>
  </el-alert>
</template>
