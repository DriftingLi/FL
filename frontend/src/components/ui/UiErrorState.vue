<script setup lang="ts">
import UiButton from './UiButton.vue'

/**
 * 错误态：自写，抽自 `components/student/VideoPlayer.vue:3-8` 的重试模式。
 *
 * 与 UiEmptyState 的区别：错误态一定有「重试」动作，且重试中要禁用按钮避免重复提交。
 */
const props = withDefaults(
  defineProps<{
    title?: string
    description?: string
    retrying?: boolean
    retryText?: string
  }>(),
  { title: '加载失败', retrying: false, retryText: '重试' }
)

const emit = defineEmits<{ retry: [] }>()
</script>

<template>
  <div class="flex flex-col items-center justify-center gap-3 py-10 text-center">
    <el-icon class="text-2xl text-bad"><WarningFilled /></el-icon>

    <div>
      <p class="font-heading text-base font-semibold text-ink">{{ props.title }}</p>
      <p v-if="props.description" class="mt-1 text-sm text-ink-3">
        {{ props.description }}
      </p>
    </div>

    <UiButton
      variant="primary"
      :loading="props.retrying"
      @click="emit('retry')"
    >
      {{ props.retryText }}
    </UiButton>

    <slot />
  </div>
</template>
