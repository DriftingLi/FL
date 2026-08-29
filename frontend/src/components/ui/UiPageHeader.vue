<script setup lang="ts">
/**
 * 页面大标题：路由级页面顶部。
 * `back` 为 true 时显示返回按钮，点击事件交给调用方决定（前进/后退/go(-1) 语义各异）。
 */
const props = withDefaults(
  defineProps<{
    title: string
    subtitle?: string
    back?: boolean
  }>(),
  { back: false }
)

const emit = defineEmits<{ back: [] }>()
</script>

<template>
  <header class="mb-6">
    <button
      v-if="props.back"
      type="button"
      class="mb-2 inline-flex items-center gap-1 rounded-ctl px-1 py-0.5 text-sm text-ink-3 transition-colors duration-150 hover:bg-canvas hover:text-ui-600"
      @click="emit('back')"
    >
      <el-icon><ArrowLeft /></el-icon>
      <span>返回</span>
    </button>

    <div class="flex flex-wrap items-end justify-between gap-3">
      <div class="min-w-0">
        <h1 class="font-heading text-2xl font-semibold text-ink">{{ props.title }}</h1>
        <p v-if="props.subtitle" class="mt-1 text-sm text-ink-3">{{ props.subtitle }}</p>
        <div v-if="$slots.meta" class="mt-2">
          <slot name="meta" />
        </div>
      </div>

      <div v-if="$slots.actions" class="flex shrink-0 items-center gap-2">
        <slot name="actions" />
      </div>
    </div>
  </header>
</template>
