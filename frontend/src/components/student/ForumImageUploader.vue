<!--
  样式全走原子类（R4：scoped 块已删）。
  ⚠️ 不引 Tailwind preflight，浏览器默认 border-width 是 medium(3px)，
  所以下面凡是虚线边框都写成 `border border-dashed` —— 只写 border-dashed
  而不给宽度，其余三条边会渲染成 3px。
-->
<template>
  <div class="flex w-full flex-col gap-2">
    <!-- 已选图片缩略图 -->
    <div v-if="props.modelValue.length > 0" class="flex flex-wrap gap-1.5">
      <div
        v-for="(url, index) in props.modelValue"
        :key="url + index"
        class="relative size-12 shrink-0 overflow-hidden rounded-[6px] border border-line"
      >
        <el-image :src="resolveFileUrl(url)" fit="cover" class="h-full w-full" />
        <button
          type="button"
          class="absolute right-0 top-0 flex size-4 items-center justify-center rounded-bl-[6px] border-0 bg-black/55 p-0 text-[10px] text-panel transition-colors duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:bg-bad/90"
          @click="removeImage(index)"
        >
          <el-icon><Close /></el-icon>
        </button>
      </div>
    </div>

    <!-- 上传入口：小图标按钮（达到上限后隐藏） -->
    <div class="flex items-center gap-2">
      <button
        v-if="props.modelValue.length < props.max"
        type="button"
        class="inline-flex items-center gap-1 rounded-[6px] border border-dashed border-line-strong bg-canvas px-2.5 py-[5px] text-ink-2 transition-colors duration-[var(--duration-base)] ease-[var(--ease-default)] hover:border-ui-500 hover:bg-ui-50 hover:text-ui-600 disabled:cursor-not-allowed disabled:opacity-60"
        :disabled="uploading"
        title="添加图片（也可直接粘贴图片）"
        @click="triggerSelect"
      >
        <el-icon class="text-base" :class="{ 'animate-spin': uploading }">
          <Loading v-if="uploading" />
          <Picture v-else />
        </el-icon>
        <span v-if="props.modelValue.length > 0" class="text-xs text-ink-3">
          {{ props.modelValue.length }}/{{ props.max }}
        </span>
      </button>
    </div>

    <input
      ref="fileInput"
      type="file"
      :accept="accept"
      multiple
      class="hidden"
      @change="handleSelect"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { Loading, Picture, Close } from '@element-plus/icons-vue'
import { resolveFileUrl } from '@/utils/fileUrl'
import { useForumImageUpload } from '@/composables/useForumImageUpload'

const props = withDefaults(defineProps<{
  /** 已上传成功的图片 URL 数组（v-model） */
  modelValue: string[]
  /** 图片数量上限 */
  max?: number
}>(), {
  max: 9
})

const emit = defineEmits(['update:modelValue'])

// 上传校验与状态机进 useForumImageUpload（#389 单点）：URL 列表经可写 computed 受控回写父级
const urls = computed({
  get: () => props.modelValue,
  set: v => emit('update:modelValue', v)
})
const { uploading, uploadFiles, removeImage, handlePaste } = useForumImageUpload(() => props.max, { urls })

const accept = 'image/*'
const fileInput = ref<HTMLInputElement | null>(null)

function triggerSelect() {
  fileInput.value?.click()
}

// 选择文件（含多选）
function handleSelect(event: Event) {
  const target = event.target as HTMLInputElement
  const files = Array.from(target.files ?? [])
  if (files.length > 0) {
    void uploadFiles(files)
  }
  target.value = ''
}

onMounted(() => {
  document.addEventListener('paste', handlePaste)
})

onBeforeUnmount(() => {
  document.removeEventListener('paste', handlePaste)
})
</script>
