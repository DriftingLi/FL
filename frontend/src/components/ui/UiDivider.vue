<script setup lang="ts">
/** 分割线：自写。水平可带中置文字，垂直用于行内分隔。 */
const props = withDefaults(
  defineProps<{
    orientation?: 'horizontal' | 'vertical'
    label?: string
    spacing?: 'sm' | 'md' | 'lg'
  }>(),
  { orientation: 'horizontal', spacing: 'md' }
)

const GAP: Record<string, string> = { sm: 'my-2', md: 'my-4', lg: 'my-6' }
</script>

<template>
  <hr
    v-if="props.orientation === 'horizontal' && !props.label"
    class="border-0 border-t border-line"
    :class="GAP[props.spacing]"
  />

  <div
    v-else-if="props.orientation === 'horizontal'"
    class="flex items-center gap-3"
    :class="GAP[props.spacing]"
  >
    <span class="h-px flex-1 bg-line" />
    <span class="text-xs text-ink-3">{{ props.label }}</span>
    <span class="h-px flex-1 bg-line" />
  </div>

  <span v-else class="mx-2 inline-block h-4 w-px shrink-0 self-center bg-line" />
</template>
