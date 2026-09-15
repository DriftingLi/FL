import { ref, toValue, type MaybeRefOrGetter, type Ref } from 'vue'
import { ElMessage } from 'element-plus'
import { forumApi } from '@/api/forum'

/** 单文件大小上限：20MB */
const MAX_FILE_SIZE = 20 * 1024 * 1024

export interface UseForumImageUploadOptions {
  /**
   * 受控 URL 列表（v-model 组件传可写 computed，页面直用场景缺省内部自建）。
   * 上传成功追加、removeImage 删除都写回该列表。
   */
  urls?: Ref<string[]>
}

/**
 * 论坛图片上传（#389 单点）：张数上限 / 20MB / FormData 上传 / 粘贴与拖拽检测。
 * UI 形态（缩略图条、虚线区、监听挂在哪）留给组件与页面，
 * 上传校验与状态机统一在此 —— 回复框与发帖表单共用同一份口径。
 *
 * #1014：**拖拽**登记到本单点。区域内的 dragover 一律 preventDefault —— 不接管的话
 * 浏览器会直接把拖进来的图片当成一次导航（离开当前页去打开那张图），这是既有毛病。
 */
export function useForumImageUpload(max: MaybeRefOrGetter<number>, options: UseForumImageUploadOptions = {}) {
  const urls = options.urls ?? ref<string[]>([])
  const uploading = ref(false)
  /** 拖拽悬停中（虚线区高亮用） */
  const dragging = ref(false)

  /** 非图片一律丢掉：拖进来一个 pdf 不该被送进图片接口 */
  function extractImageFiles(list: ArrayLike<File> | null | undefined): File[] {
    return Array.from(list ?? []).filter(f => f.type.startsWith('image/'))
  }

  /** 批量上传（选择文件与粘贴共用）：跳过非图片与超限文件，顺序上传，达到张数上限即停 */
  async function uploadFiles(files: File[]): Promise<void> {
    const maxCount = toValue(max)
    const remaining = maxCount - urls.value.length
    if (remaining <= 0) {
      ElMessage.warning(`最多上传 ${maxCount} 张图片`)
      return
    }
    const toUpload = extractImageFiles(files).slice(0, remaining)
    if (toUpload.length === 0) return

    uploading.value = true
    try {
      for (const file of toUpload) {
        if (file.size > MAX_FILE_SIZE) {
          ElMessage.error(`"${file.name}" 超过 20MB，已跳过`)
          continue
        }
        const formData = new FormData()
        formData.append('file', file)
        try {
          const res = await forumApi.uploadImage(formData)
          if (res?.url) {
            if (urls.value.length >= toValue(max)) break
            urls.value = [...urls.value, res.url]
          } else {
            ElMessage.error(`"${file.name}" 上传失败`)
          }
        } catch {
          /* 错误已由拦截器提示 */
        }
      }
    } finally {
      uploading.value = false
    }
  }

  function removeImage(index: number): void {
    const next = [...urls.value]
    next.splice(index, 1)
    urls.value = next
  }

  /** 粘贴检测：剪贴板含图片文件时拦截默认行为并转入上传（达到上限时放行原生粘贴） */
  function handlePaste(event: ClipboardEvent): void {
    const items = event.clipboardData?.items
    if (!items) return
    if (urls.value.length >= toValue(max)) return
    const files: File[] = []
    for (const item of items) {
      if (item.kind === 'file' && item.type.startsWith('image/')) {
        const file = item.getAsFile()
        if (file) files.push(file)
      }
    }
    if (files.length > 0) {
      event.preventDefault()
      void uploadFiles(files)
    }
  }

  /** 拖拽进入区域：接管默认行为并高亮；已达上限时不再高亮（放下也不会传） */
  function handleDragOver(event: DragEvent): void {
    event.preventDefault()
    if (urls.value.length >= toValue(max)) return
    if (event.dataTransfer) event.dataTransfer.dropEffect = 'copy'
    dragging.value = true
  }

  /** 离开区域或放下：复位高亮。dragleave 在子元素之间也会冒泡，调用方用 .self 过滤 */
  function handleDragLeave(): void {
    dragging.value = false
  }

  /** 放下：接管默认行为（否则浏览器会导航到该文件），只收图片 */
  function handleDrop(event: DragEvent): void {
    event.preventDefault()
    dragging.value = false
    if (urls.value.length >= toValue(max)) return
    const files = extractImageFiles(event.dataTransfer?.files)
    if (files.length > 0) void uploadFiles(files)
  }

  return {
    urls,
    uploading,
    dragging,
    uploadFiles,
    removeImage,
    handlePaste,
    handleDragOver,
    handleDragLeave,
    handleDrop
  }
}
