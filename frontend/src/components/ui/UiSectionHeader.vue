<script setup lang="ts">
/** 区块小标题：页面内的二级分组头，右侧可挂操作区。 */
defineProps<{
  title: string
  subtitle?: string
}>()
</script>

<template>
  <div class="mb-4 flex items-end justify-between gap-3">
    <div class="min-w-0">
      <h2 class="font-heading text-lg font-semibold text-ink">{{ title }}</h2>
      <p v-if="subtitle" class="mt-0.5 text-sm text-ink-3">{{ subtitle }}</p>
    </div>

    <div v-if="$slots.actions" class="flex shrink-0 items-center gap-2">
      <slot name="actions" />
    </div>
  </div>
</template>
