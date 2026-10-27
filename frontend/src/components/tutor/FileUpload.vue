<template>
  <div class="file-upload border border-line rounded-ctl bg-panel p-5">
    <div class="filter-bar mb-4 overflow-x-auto whitespace-nowrap">
      <UiRadioGroup v-model="activeFilter" size="small" @change="handleFilterChange">
        <el-radio-button
          v-for="item in filterOptions"
          :key="item.value"
          :value="item.value"
        >
          {{ item.label }}
        </el-radio-button>
      </UiRadioGroup>
    </div>

    <div
      class="upload-area border-2 border-dashed border-line-strong rounded-ctl px-5 py-[30px] text-center cursor-pointer transition-[border-color,background-color] duration-[var(--duration-normal)] ease-[var(--ease-default)] hover:border-ui-500 hover:bg-canvas"
      :class="isDragover ? 'border-ui-500 bg-canvas' : ''"
      @dragover.prevent="isDragover = true"
      @dragleave.prevent="isDragover = false"
      @drop.prevent="handleDrop"
      @click="triggerSelect"
    >
      <UiIcon name="upload-cloud" class="upload-icon mb-2 text-[var(--color-text-disabled)]" :size="40" />
      <p class="upload-text text-ink-2 text-sm mb-2">将文件拖到此处，或<em class="not-italic text-ui-soft-ink">点击上传</em></p>
      <p class="upload-tip text-ink-3 text-xs my-1">支持格式：PDF 文档、PPT、MP4、WebM（图片请粘贴到图文正文）</p>
      <p class="upload-tip text-ink-3 text-xs my-1">视频文件最大200MB，其他文件最大50MB</p>
    </div>

    <input
      ref="inputRef"
      type="file"
      :accept="currentAccept"
      multiple
      class="hidden"
      @change="handleSelect"
    />

    <div v-if="fileList.length > 0" class="file-list-section mt-5">
      <div class="file-list-header flex items-center justify-between mb-3 pb-3 border-b border-line">
        <span class="summary-text text-sm text-ink-2 font-medium">
          已上传 {{ successCount }}/{{ fileList.length }} 个文件
        </span>
        <UiButton variant="primary" v-if="fileList.some(f => f.status === 'pending')" size="small" :loading="isUploading" @click="startUploadAll">
          {{ isUploading ? '上传中...' : '全部上传' }}
        </UiButton>
        <UiButton v-if="fileList.length > 0" size="small" @click="clearAll">
          清空列表
        </UiButton>
      </div>

      <div class="file-list flex flex-col gap-2.5 max-h-[400px] overflow-y-auto">
        <div
          v-for="file in fileList"
          :key="file.uid"
          class="file-item flex items-center gap-3 p-3 border border-line rounded-ctl transition-[border-color,background-color] duration-[var(--duration-normal)] ease-[var(--ease-default)] max-[768px]:flex-wrap max-[768px]:gap-2"
          :class="file.status === 'error' ? 'border-bad bg-bad-soft' : file.status === 'success' ? 'border-ok bg-ok-soft' : ''"
        >
          <div class="file-info flex items-center gap-2.5 flex-1 min-w-0 max-[768px]:basis-full">
            <UiIcon :name="getFileIcon(file.ext)" class="file-type-icon text-ink-3 shrink-0" :size="20" />
            <div class="file-detail flex-1 min-w-0">
              <div class="file-name-row flex items-center gap-2">
                <span class="file-name text-sm text-ink overflow-hidden text-ellipsis whitespace-nowrap max-w-[260px] max-[768px]:max-w-[160px]" :title="file.name">{{ file.name }}</span>
                <UiTag size="small" :tone="getFileTypeTagType(file.category)" class="file-type-tag shrink-0">
                  {{ getFileTypeLabel(file.category) }}
                </UiTag>
              </div>
              <span class="file-size text-xs text-ink-3 mt-0.5">{{ formatSize(file.size) }}</span>
            </div>
          </div>

          <div class="file-status-area w-[180px] shrink-0 max-[768px]:flex-[1_1_calc(100%-80px)] max-[768px]:w-auto">
            <span v-if="file.status === 'pending'" class="status-text status-pending text-sm text-ink-3">等待上传</span>
            <div v-else class="status-done flex items-center gap-1.5">
              <!--
                同一个图标换名字：转圈 → 对勾 / 叉号，形状真的变了才用形变（ADR-0079 白名单）。
                失败重试时回到 uploading，同一元素从叉号长回转圈。
              -->
              <UiIcon
                :name="file.status === 'uploading' ? 'loader-circle' : file.status === 'success' ? 'circle-check' : 'circle-x'"
                morph
                :class="[
                  'status-icon text-lg',
                  file.status === 'uploading' ? 'animate-spin text-ink-3' : '',
                  file.status === 'success' ? 'success-icon text-ok' : '',
                  file.status === 'error' ? 'error-icon text-bad' : ''
                ]"
              />
              <UiProgress
                v-if="file.status === 'uploading'"
                :value="file.percentage"
                :stroke-width="6"
                show-label
                class="file-progress flex-1"
              />
              <span v-else-if="file.status === 'success'" class="status-text status-success text-sm text-ok">上传成功</span>
              <span v-else class="status-text status-error text-sm text-bad">上传失败</span>
            </div>
          </div>

          <div class="file-actions flex items-center gap-1.5 shrink-0">
            <UiButton variant="warning" v-if="file.status === 'error'" size="small" circle aria-label="重试上传" @click.stop="retryFile(file)">
              <UiIcon name="rotate-cw" />
            </UiButton>
            <UiButton variant="danger" plain size="small" circle aria-label="移除文件" @click.stop="removeFile(file)">
              <UiIcon name="trash-2" />
            </UiButton>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import UiIcon from '@/components/ui/UiIcon.vue'
import { ref, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { tutorApi } from '@/api/tutor'
import type { UiTagTone } from '@/components/ui/UiTag.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiTag from '@/components/ui/UiTag.vue'
import UiRadioGroup from '@/components/ui/UiRadioGroup.vue'
import UiProgress from '@/components/ui/UiProgress.vue'

interface FileItem {
  uid: number
  raw: File
  name: string
  size: number
  ext: string
  category: string
  status: string
  percentage: number
  errorMsg: string
}

const emit = defineEmits(['upload-all', 'file-status'])

const props = defineProps({
  chapterId: {
    type: [String, Number],
    required: true
  },
  initialFilter: {
    type: String,
    default: 'all'
  }
})

const inputRef = ref<HTMLInputElement | null>(null)
const isDragover = ref(false)
const isUploading = ref(false)
const fileList = ref<FileItem[]>([])
const activeFilter = ref(props.initialFilter || 'all')

let uidCounter = 0

const filterOptions = [
  { label: '全部', value: 'all', accept: '.pdf,.ppt,.pptx,.mp4,.webm' },
  { label: '文档', value: 'document', accept: '.pdf' },
  { label: 'PPT', value: 'ppt', accept: '.ppt,.pptx' },
  { label: '视频', value: 'video', accept: '.mp4,.webm' }
]

const currentAccept = computed(() => {
  const option = filterOptions.find(o => o.value === activeFilter.value)
  return option ? option.accept : filterOptions[0].accept
})

const successCount = computed(() => {
  return fileList.value.filter(f => f.status === 'success').length
})

const typeCategoryMap: Record<string, string> = {
  pdf: 'document',
  ppt: 'ppt', pptx: 'ppt',
  mp4: 'video', webm: 'video'
}

const maxSizeMap: Record<string, number> = {
  video: 200 * 1024 * 1024,
  document: 50 * 1024 * 1024,
  ppt: 50 * 1024 * 1024
}

const maxSizeMBMap: Record<string, number> = {
  video: 200,
  document: 50,
  ppt: 50
}

function getFileCategory(ext: string) {
  return typeCategoryMap[ext] || 'document'
}

function handleFilterChange() {
  if (inputRef.value) {
    inputRef.value.value = ''
  }
}

function triggerSelect() {
  inputRef.value?.click()
}

function handleSelect(event: Event) {
  const target = event.target as HTMLInputElement
  const files = Array.from(target.files ?? [])
  if (files.length > 0) {
    addFiles(files)
  }
  if (inputRef.value) inputRef.value.value = ''
}

function handleDrop(event: DragEvent) {
  isDragover.value = false
  const files = Array.from(event.dataTransfer?.files ?? [])
  if (files.length > 0) {
    addFiles(files)
  }
}

function addFiles(files: File[]) {
  for (const file of files) {
    const ext = (file.name.split('.').pop() ?? '').toLowerCase()
    const category = getFileCategory(ext)

    if (!Object.keys(typeCategoryMap).includes(ext)) {
      ElMessage.error(`不支持的文件格式: ${file.name}`)
      continue
    }

    const maxSize = maxSizeMap[category] || 50 * 1024 * 1024
    if (file.size > maxSize) {
      const maxMB = maxSizeMBMap[category] || 50
      ElMessage.error(`文件"${file.name}"大小超出限制，最大允许${maxMB}MB`)
      continue
    }

    const fileItem = {
      uid: ++uidCounter,
      raw: file,
      name: file.name,
      size: file.size,
      ext,
      category,
      status: 'pending',
      percentage: 0,
      errorMsg: ''
    }

    fileList.value.push(fileItem)
    emitFileStatus(fileItem)
  }
}

function formatSize(bytes: number) {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + ' ' + units[i]
}

function getFileIcon(ext: string) {
  const category = getFileCategory(ext)
  switch (category) {
    case 'video': return 'video'
    default: return 'file-text'
  }
}

function getFileTypeTagType(category: string): UiTagTone {
  const types: Record<string, UiTagTone> = {
    document: 'primary',
    ppt: 'warning',
    video: 'danger'
  }
  return types[category] || 'info'
}

function getFileTypeLabel(category: string) {
  const labels: Record<string, string> = {
    document: '文档',
    ppt: 'PPT',
    video: '视频'
  }
  return labels[category] || '文件'
}

function removeFile(file: FileItem) {
  const index = fileList.value.findIndex(f => f.uid === file.uid)
  if (index > -1) {
    fileList.value.splice(index, 1)
  }
}

function clearAll() {
  fileList.value = []
}

function emitFileStatus(file: FileItem) {
  emit('file-status', {
    uid: file.uid,
    name: file.name,
    status: file.status,
    percentage: file.percentage,
    category: file.category
  })
}

async function uploadSingleFile(file: FileItem) {
  file.status = 'uploading'
  file.percentage = 0
  emitFileStatus(file)

  const formData = new FormData()
  formData.append('file', file.raw)

  try {
    await tutorApi.uploadChapterFile(
      Number(props.chapterId),
      formData,
      (progressEvent) => {
        if (progressEvent.total) {
          file.percentage = Math.round((progressEvent.loaded * 100) / progressEvent.total)
          emitFileStatus(file)
        }
      }
    )

    file.status = 'success'
    file.percentage = 100
  } catch (e) {
    file.status = 'error'
    file.errorMsg = (e instanceof Error ? e.message : '上传失败') || '上传失败'
  }

  emitFileStatus(file)
  return file.status === 'success'
}

async function startUploadAll() {
  const pendingFiles = fileList.value.filter(f => f.status === 'pending' || f.status === 'error')
  if (pendingFiles.length === 0) return

  isUploading.value = true

  for (const file of pendingFiles) {
    await uploadSingleFile(file)
  }

  isUploading.value = false

  const allDone = fileList.value.every(f => f.status === 'success' || f.status === 'error')
  if (allDone) {
    emit('upload-all', {
      total: fileList.value.length,
      success: successCount.value,
      failed: fileList.value.filter(f => f.status === 'error').length
    })
  }
}

async function retryFile(file: FileItem) {
  await uploadSingleFile(file)

  const allDone = fileList.value.every(f => f.status === 'success' || f.status === 'error')
  if (allDone) {
    emit('upload-all', {
      total: fileList.value.length,
      success: successCount.value,
      failed: fileList.value.filter(f => f.status === 'error').length
    })
  }
}

function resetState() {
  fileList.value = []
  isUploading.value = false
}

defineExpose({
  startUploadAll,
  resetState,
  fileList
})
</script>

<style scoped>
/* R1 允许：EP 内部覆盖。其余样式已全部原子化。 */
@media screen and (max-width: 768px) {
  .filter-bar :deep(.el-radio-group) {
    flex-wrap: wrap;
  }
}
</style>
