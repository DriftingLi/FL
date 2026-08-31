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
 * 论坛图片上传（#389 单点）：张数上限 / 20MB / FormData 上传 / 粘贴检测。
 * UI 形态（缩略图条、入口按钮、粘贴监听的生命周期）留给组件与页面，
 * 上传校验与状态机统一在此 —— 详情页回复内联与 ForumImageUploader 共用同一份口径。
 */
export function useForumImageUpload(max: MaybeRefOrGetter<number>, options: UseForumImageUploadOptions = {}) {
  const urls = options.urls ?? ref<string[]>([])
  const uploading = ref(false)

  /** 批量上传（选择文件与粘贴共用）：跳过非图片与超限文件，顺序上传，达到张数上限即停 */
  async function uploadFiles(files: File[]): Promise<void> {
    const maxCount = toValue(max)
    const remaining = maxCount - urls.value.length
    if (remaining <= 0) {
      ElMessage.warning(`最多上传 ${maxCount} 张图片`)
      return
    }
    const toUpload = files.filter(f => f.type.startsWith('image/')).slice(0, remaining)
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

  return { urls, uploading, uploadFiles, removeImage, handlePaste }
}
