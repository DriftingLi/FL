<script setup lang="ts">
/**
 * 上传：包 `el-upload`，纯透传薄封装。
 *
 * 约定（#766 C2）：不设项目默认值，attrs / 事件 / slot（tip / file / trigger 等
 * 具名槽）全量透传。el-upload 无 v-model（状态由调用方经 file-list / 上传钩子管理），
 * 因此不声明 model。深色下 picture-card 虚线框的亮边缺陷已由
 * `--el-border-color-darker` 的全局映射修复（见 C1），本组件不重复处理。
 */
defineOptions({ name: 'UiUpload', inheritAttrs: false })
</script>

<template>
  <el-upload v-bind="$attrs">
    <template v-for="(_, name) in $slots" :key="name" #[name]="slotProps">
      <slot :name="name" v-bind="slotProps ?? {}" />
    </template>
  </el-upload>
</template>
