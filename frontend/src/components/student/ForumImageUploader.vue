<!--
  图片粘贴/拖拽区（图二形态 / ADR-0052）。

  样式全走原子类（R4：scoped 块已删）。
  ⚠️ 不引 Tailwind preflight，浏览器默认 border-width 是 medium(3px)，
  所以虚线框写成 `border border-dashed` —— 只写 border-dashed 而不给宽度，
  其余三条边会渲染成 3px。

  监听分工（#1017 起）：
  - **拖拽与粘贴都不在本组件挂 DOM 监听**：父级（ForumMarkdownInput）在**整张卡片**上
    接住后经 `defineExpose` 转发进来。粘贴原来是 document 级（多处同时挂载会互相抢
    事件，见 ForumComposer 的历史注释）；拖拽原来只挂在虚线区上 —— 拖到正文 / 工具栏上
    浏览器会执行 drop 默认动作直接导航走，而「拖到正文上」恰恰是最自然的动作。
  - 本组件保留 window 级 `dragend` / `drop` 兜底：ESC 取消拖拽或指针离开窗口时，
    浏览器不保证补发 dragleave，高亮会卡在「拖拽悬停」态。
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

    <!-- 上传入口：整块虚线区，点击 / 粘贴 / 拖拽三入口（图二） -->
    <button
      type="button"
      class="flex w-full items-center justify-center gap-2 rounded-[6px] border border-dashed px-3 py-3 text-sm transition-colors duration-[var(--duration-base)] ease-[var(--ease-default)]"
      :class="zoneClass"
      :aria-disabled="isFull ? 'true' : undefined"
      :title="zoneTitle"
      @click="triggerSelect"
    >
      <el-icon v-if="uploading" class="animate-spin text-base"><Loading /></el-icon>
      <!-- 纸夹用内联 SVG：EP 图标集里没有语义合适的纸夹，且这样能与工具栏图标同一套描边风格 -->
      <svg
        v-else
        class="size-4 shrink-0"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.8"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <path
          d="M20.5 11.5l-8.2 8.2a5.4 5.4 0 0 1-7.6-7.6l8.2-8.2a3.6 3.6 0 0 1 5.1 5.1l-8.2 8.2a1.8 1.8 0 0 1-2.6-2.6l7.6-7.6"
        />
      </svg>
      <span>{{ uploading ? '上传中…' : '粘贴、拖拽或点击添加图片' }}</span>
      <span class="text-xs text-ink-3">{{ props.modelValue.length }}/{{ props.max }}</span>
    </button>

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
import { Loading, Close } from '@element-plus/icons-vue'
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
const { uploading, dragging, uploadFiles, removeImage, handlePaste, handleDragOver, handleDragLeave, handleDrop } =
  useForumImageUpload(() => props.max, { urls })

const accept = 'image/*'
const fileInput = ref<HTMLInputElement | null>(null)

const isFull = computed(() => props.modelValue.length >= props.max)
const zoneTitle = computed(() => (isFull.value ? `最多 ${props.max} 张图片` : '粘贴、拖拽或点击添加图片'))

/** 虚线区四态：已达上限 / 上传中 / 拖拽悬停 / 常态。冲突工具类不共存（cursor 只在分支里给） */
const zoneClass = computed(() => {
  if (isFull.value) return 'cursor-not-allowed border-line bg-canvas text-ink-3 opacity-70'
  if (uploading.value) return 'cursor-wait border-line-strong bg-canvas text-ink-2 opacity-80'
  if (dragging.value) return 'cursor-copy border-ui-500 bg-ui-50 text-ui-600'
  return 'cursor-pointer border-line-strong bg-canvas text-ink-2 hover:border-ui-500 hover:bg-ui-50 hover:text-ui-600'
})

function triggerSelect() {
  // 已达上限 / 上传中：点了也不弹选择器（不用原生 disabled —— 那会让 title 提示失效）
  if (isFull.value || uploading.value) return
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

// 粘贴与拖拽监听都由父级卡片转发进来（见文件头注释）
onMounted(() => {
  window.addEventListener('dragend', onWindowDragEnd)
  window.addEventListener('drop', onWindowDragEnd)
})

onBeforeUnmount(() => {
  window.removeEventListener('dragend', onWindowDragEnd)
  window.removeEventListener('drop', onWindowDragEnd)
})

/** 兜底复位：不传事件 = 无条件清高亮（ESC 取消拖拽 / 指针离开窗口都不会补发 dragleave） */
function onWindowDragEnd() {
  handleDragLeave()
}

defineExpose({ handlePaste, handleDragOver, handleDragLeave, handleDrop })
</script>
