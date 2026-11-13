<template>
  <div>
    <UiPageHeader title="文件库" subtitle="上传的录课视频与课件在这里管理">
      <template #meta>
        <p class="text-sm text-ink-3">支持单文件最大 2GB；大文件自动分片上传，中断后重新选择即可</p>
      </template>
      <template #actions>
        <UiUpload
          class="[&_.el-upload]:flex [&_.el-upload]:cursor-pointer"
          :show-file-list="false"
          :accept="TUTOR_FILE_ACCEPT"
          :http-request="handleUpload"
        >
          <UiButton variant="primary" :loading="uploading">上传文件</UiButton>
        </UiUpload>
      </template>
    </UiPageHeader>

    <UiCard v-if="uploading" class="mb-4">
      <div class="flex items-center gap-3">
        <p class="min-w-0 flex-1 truncate text-sm text-ink">{{ uploadingName }}</p>
        <span class="text-sm tabular-nums text-ink-3">{{ uploadPercent }}%</span>
      </div>
      <UiProgress class="mt-2" :value="uploadPercent" size="sm" />
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
              <UiTag :tone="describeCategory(file.content_type).tone">
                {{ describeCategory(file.content_type).label }}
              </UiTag>
              <span class="tabular-nums">{{ formatSize(file.file_size) }}</span>
            </div>
          </div>
          <UiButton variant="danger" @click="onDelete(file)">删除</UiButton>
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
import UiProgress from '@/components/ui/UiProgress.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiTag from '@/components/ui/UiTag.vue'
import UiUpload from '@/components/ui/UiUpload.vue'
import UiAsyncSection from '@/components/ui/UiAsyncSection.vue'
import { useAsyncPage } from '@/composables/useAsyncPage'
import { useConfirm } from '@/composables/useConfirm'
import { tutorFileApi, uploadFileInChunks, TUTOR_FILE_ACCEPT, type FileDTO } from '@/api/tutorfile'

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

/** 内容分类 → 语义色调（状态词表口径：单一 descriptor 派生 label 与 tone）。 */
function describeCategory(t: string): { label: string; tone: 'brand' | 'success' | 'info' | 'neutral' } {
  const map: Record<string, { label: string; tone: 'brand' | 'success' | 'info' | 'neutral' }> = {
    video: { label: '视频', tone: 'brand' },
    document: { label: '文档', tone: 'info' },
    ppt: { label: '幻灯片', tone: 'success' },
    image: { label: '图片', tone: 'neutral' }
  }
  return map[t] ?? { label: t, tone: 'neutral' }
}

function formatSize(size: number) {
  if (size >= 1 << 30) return (size / (1 << 30)).toFixed(2) + ' GB'
  if (size >= 1 << 20) return (size / (1 << 20)).toFixed(1) + ' MB'
  return Math.max(1, Math.round(size / 1024)) + ' KB'
}

/** UiUpload 的 http-request 槽：接管传输（分片上传），不再走 el-upload 自带单发。 */
async function handleUpload({ file }: { file: File }) {
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
    await useConfirm().confirmDanger(`确认删除「${file.file_name}」？删除后无法恢复。`, '删除文件')
  } catch {
    return
  }
  await tutorFileApi.deleteFile(file.file_id)
  await loadFiles()
}
</script>
