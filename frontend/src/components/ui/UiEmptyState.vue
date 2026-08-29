<script setup lang="ts">
import UiButton from './UiButton.vue'

/**
 * 空状态：包 `el-empty`，补上标题与主行动按钮。
 *
 * 注意 `el-empty` 的默认插槽渲染在插画与描述**之后**，标题塞进去会掉到底部，
 * 所以标题与描述一起放进 `#description` 插槽，保证「插画 → 标题 → 描述 → 按钮」的顺序。
 * 本组件只负责呈现，是否触发由调用方的 action 事件决定。
 */
const props = withDefaults(
  defineProps<{
    title?: string
    description?: string
    size?: 'sm' | 'md'
    actionText?: string
  }>(),
  { size: 'md' }
)

const emit = defineEmits<{ action: [] }>()

const IMAGE_SIZE: Record<string, number> = { sm: 60, md: 90 }
</script>

<template>
  <el-empty :image-size="IMAGE_SIZE[props.size]">
    <template #description>
      <h3 v-if="props.title" class="font-heading text-base font-semibold text-ink">
        {{ props.title }}
      </h3>
      <p v-if="props.description" class="mt-1 text-sm text-ink-3">
        {{ props.description }}
      </p>
    </template>

    <UiButton v-if="props.actionText" variant="primary" @click="emit('action')">
      {{ props.actionText }}
    </UiButton>
    <slot />
  </el-empty>
</template>
