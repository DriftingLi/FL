<template>
  <div>
    <UiPageHeader title="文件库" subtitle="上传的录课视频与课件在这里管理">
      <template #meta>
        <p class="text-sm text-ink-3">支持单文件最大 2GB；大文件自动分片上传，中断后重新选择即可</p>
      </template>
      <template #actions>
        <div class="flex items-center gap-3">
          <input
            ref="fileInput"
            type="file"
            class="hidden"
            accept=".pdf,.ppt,.pptx,.mp4,.webm,.png,.jpg,.jpeg,.gif,.webp,.bmp"
            @change="onFileSelected"
          />
          <UiButton variant="primary" :loading="uploading" @click="fileInput?.click()">
            上传文件
          </UiButton>
        </div>
      </template>
    </UiPageHeader>

    <UiCard v-if="uploading" class="mb-4">
      <div class="flex items-center gap-3">
        <p class="min-w-0 flex-1 truncate text-sm text-ink">{{ uploadingName }}</p>
        <span class="text-sm tabular-nums text-ink-3">{{ uploadPercent }}%</span>
      </div>
      <div class="mt-2 h-1.5 overflow-hidden rounded-pill bg-ui-soft">
        <div class="h-full rounded-pill bg-primary-500 transition-all duration-200" :style="{ width: uploadPercent + '%' }" />
      </div>
    </UiCard>

    <UiAsyncSection
      :error="loadError"
      :loading="loading"
      :empty="isEmpty"
      :retrying="retrying"
      error-title="文件库加载失败"
      error-description="未能获取文件列表，请检查网络后重试。"
      @retry="handleRetry"
    >
      <template #skeleton>
        <UiSkeleton variant="list" :count="3" />
      </template>

      <div class="flex flex-col gap-3">
        <UiCard v-for="file in files" :key="file.file_id" class="flex items-center gap-4">
          <UiIcon name="file-text" class="shrink-0 text-ink-3" />
          <div class="min-w-0 flex-1">
            <p class="truncate text-base font-medium text-ink">{{ file.file_name }}</p>
            <div class="mt-1 flex flex-wrap items-center gap-3 text-xs text-ink-3">
              <UiTag :tone="categoryTone(file.content_type)">{{ categoryLabel(file.content_type) }}</UiTag>
              <span class="tabular-nums">{{ formatSize(file.file_size) }}</span>
              <a :href="file.file_url" target="_blank" rel="noopener" class="text-primary-500 hover:underline">预览</a>
            </div>
          </div>
          <UiButton variant="ghost" @click="onDelete(file)">删除</UiButton>
        </UiCard>
      </div>

      <template #empty>
        <UiEmptyState
          title="文件库是空的"
          description="上传录课视频或课件后，可在此管理并挂接到章节。"
        />
      </template>
    </UiAsyncSection>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiIcon from '@/components/ui/UiIcon.vue'
import UiPageHeader from '@/components/ui/UiPageHeader.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiTag from '@/components/ui/UiTag.vue'
import UiAsyncSection from '@/components/ui/UiAsyncSection.vue'
import { useAsyncPage } from '@/composables/useAsyncPage'
import { ElMessageBox } from 'element-plus'
import { tutorFileApi, uploadFileInChunks, type FileDTO } from '@/api/tutorfile'

const fileInput = ref<HTMLInputElement | null>(null)
const uploading = ref(false)
const uploadingName = ref('')
const uploadPercent = ref(0)

const files = ref<FileDTO[]>([])

const {
  loading,
  loadError,
  retrying,
  isEmpty,
  retry: handleRetry,
  run: loadFiles
} = useAsyncPage(
  async () => (await tutorFileApi.listFiles()).files,
  {
    apply: (list) => {
      files.value = list
    },
    itemsRef: files
  }
)

onMounted(() => {
  loadFiles()
})

function categoryLabel(t: string) {
  return ({ video: '视频', document: '文档', ppt: '幻灯片', image: '图片' } as Record<string, string>)[t] || t
}

function categoryTone(t: string): 'brand' | 'success' | 'info' | 'neutral' {
  return ({ video: 'brand', document: 'info', ppt: 'success', image: 'neutral' } as const)[t] || 'neutral'
}

function formatSize(size: number) {
  if (size >= 1 << 30) return (size / (1 << 30)).toFixed(2) + ' GB'
  if (size >= 1 << 20) return (size / (1 << 20)).toFixed(1) + ' MB'
  return Math.max(1, Math.round(size / 1024)) + ' KB'
}

async function onFileSelected(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  uploading.value = true
  uploadingName.value = file.name
  uploadPercent.value = 0
  try {
    await uploadFileInChunks(file, (p) => (uploadPercent.value = p))
    await loadFiles()
  } catch {
    // 错误已由请求层统一提示；此处只收尾 UI
  } finally {
    uploading.value = false
  }
}

async function onDelete(file: FileDTO) {
  try {
    await ElMessageBox.confirm(`确认删除「${file.file_name}」？删除后无法恢复。`, '删除文件', {
      confirmButtonText: '删除',
      cancelButtonText: '取消',
      type: 'warning'
    })
  } catch {
    return
  }
  await tutorFileApi.deleteFile(file.file_id)
  await loadFiles()
}
</script>
