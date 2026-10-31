<script setup lang="ts">
/**
 * 字段说明（#2156 表单批）：标签与错误之间的一行灰字，讲格式、范围或来源。
 *
 * 不是必填项就不要在这里写「选填」—— 默认就是选填，写了只是噪音。
 */
</script>

<template>
  <p data-slot="field-description" class="m-0 text-xs leading-normal text-ink-3">
    <slot />
  </p>
</template>
