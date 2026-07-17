<template>
  <div class="file-upload">
    <div class="filter-bar">
      <el-radio-group v-model="activeFilter" size="small" @change="handleFilterChange">
        <el-radio-button
          v-for="item in filterOptions"
          :key="item.value"
          :value="item.value"
        >
          {{ item.label }}
        </el-radio-button>
      </el-radio-group>
    </div>

    <div
      class="upload-area"
      :class="{ 'is-dragover': isDragover }"
      @dragover.prevent="isDragover = true"
      @dragleave.prevent="isDragover = false"
      @drop.prevent="handleDrop"
      @click="triggerSelect"
    >
      <el-icon class="upload-icon" :size="40"><UploadFilled /></el-icon>
      <p class="upload-text">将文件拖到此处，或<em>点击上传</em></p>
      <p class="upload-tip">支持格式：PDF、Word、PPT、MP4、WebM、Excel、CSV、图片</p>
      <p class="upload-tip">视频文件最大200MB，图片最大20MB，其他文件最大50MB</p>
    </div>

    <input
      ref="inputRef"
      type="file"
      :accept="currentAccept"
      multiple
      style="display: none"
      @change="handleSelect"
    />

    <div v-if="fileList.length > 0" class="file-list-section">
      <div class="file-list-header">
        <span class="summary-text">
          已上传 {{ successCount }}/{{ fileList.length }} 个文件
        </span>
        <el-button
          v-if="fileList.some(f => f.status === 'pending')"
          type="primary"
          size="small"
          :loading="isUploading"
          @click="startUploadAll"
        >
          {{ isUploading ? '上传中...' : '全部上传' }}
        </el-button>
        <el-button
          v-if="fileList.length > 0"
          size="small"
          @click="clearAll"
        >
          清空列表
        </el-button>
      </div>

      <div class="file-list">
        <div
          v-for="file in fileList"
          :key="file.uid"
          class="file-item"
          :class="{ 'is-error': file.status === 'error', 'is-success': file.status === 'success' }"
        >
          <div class="file-info">
            <el-icon class="file-type-icon" :size="20">
              <component :is="getFileIcon(file.ext)" />
            </el-icon>
            <div class="file-detail">
              <div class="file-name-row">
                <span class="file-name" :title="file.name">{{ file.name }}</span>
                <el-tag size="small" :type="getFileTypeTagType(file.category)" class="file-type-tag">
                  {{ getFileTypeLabel(file.category) }}
                </el-tag>
              </div>
              <span class="file-size">{{ formatSize(file.size) }}</span>
            </div>
          </div>

          <div class="file-status-area">
            <template v-if="file.status === 'pending'">
              <span class="status-text status-pending">等待上传</span>
            </template>
            <template v-else-if="file.status === 'uploading'">
              <el-progress
                :percentage="file.percentage"
                :stroke-width="6"
                :show-text="true"
                class="file-progress"
              />
            </template>
            <template v-else-if="file.status === 'success'">
              <div class="status-done">
                <el-icon class="status-icon success-icon"><CircleCheck /></el-icon>
                <span class="status-text status-success">上传成功</span>
              </div>
            </template>
            <template v-else-if="file.status === 'error'">
              <div class="status-done">
                <el-icon class="status-icon error-icon"><CircleClose /></el-icon>
                <span class="status-text status-error">上传失败</span>
              </div>
            </template>
          </div>

          <div class="file-actions">
            <el-button
              v-if="file.status === 'error'"
              size="small"
              type="warning"
              circle
              @click.stop="retryFile(file)"
            >
              <el-icon><RefreshRight /></el-icon>
            </el-button>
            <el-button
              size="small"
              type="danger"
              circle
              plain
              @click.stop="removeFile(file)"
            >
              <el-icon><Delete /></el-icon>
            </el-button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import {
  UploadFilled,
  Document,
  VideoCamera,
  Picture,
  RefreshRight,
  CircleCheck,
  CircleClose,
  Delete
} from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { tutorApi } from '@/api/tutor'

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

const inputRef = ref(null)
const isDragover = ref(false)
const isUploading = ref(false)
const fileList = ref([])
const activeFilter = ref(props.initialFilter || 'all')

let uidCounter = 0

const filterOptions = [
  { label: '全部', value: 'all', accept: '.pdf,.doc,.docx,.ppt,.pptx,.mp4,.webm,.jpg,.jpeg,.png,.gif,.webp,.svg,.xls,.xlsx,.csv' },
  { label: '文档', value: 'document', accept: '.pdf,.doc,.docx,.xls,.xlsx,.csv' },
  { label: 'PPT', value: 'ppt', accept: '.ppt,.pptx' },
  { label: '视频', value: 'video', accept: '.mp4,.webm' },
  { label: '图片', value: 'image', accept: '.jpg,.jpeg,.png,.gif,.webp,.svg' }
]

const currentAccept = computed(() => {
  const option = filterOptions.find(o => o.value === activeFilter.value)
  return option ? option.accept : filterOptions[0].accept
})

const successCount = computed(() => {
  return fileList.value.filter(f => f.status === 'success').length
})

const typeCategoryMap = {
  pdf: 'document', doc: 'document', docx: 'document',
  xls: 'document', xlsx: 'document', csv: 'document',
  ppt: 'ppt', pptx: 'ppt',
  mp4: 'video', webm: 'video',
  jpg: 'image', jpeg: 'image', png: 'image', gif: 'image', webp: 'image', svg: 'image'
}

const maxSizeMap = {
  video: 200 * 1024 * 1024,
  image: 20 * 1024 * 1024,
  document: 50 * 1024 * 1024,
  ppt: 50 * 1024 * 1024
}

const maxSizeMBMap = {
  video: 200,
  image: 20,
  document: 50,
  ppt: 50
}

function getFileCategory(ext) {
  return typeCategoryMap[ext] || 'document'
}

function handleFilterChange() {
  if (inputRef.value) {
    inputRef.value.value = ''
  }
}

function triggerSelect() {
  inputRef.value.click()
}

function handleSelect(event) {
  const files = Array.from(event.target.files)
  if (files.length > 0) {
    addFiles(files)
  }
  inputRef.value.value = ''
}

function handleDrop(event) {
  isDragover.value = false
  const files = Array.from(event.dataTransfer.files)
  if (files.length > 0) {
    addFiles(files)
  }
}

function addFiles(files) {
  for (const file of files) {
    const ext = file.name.split('.').pop().toLowerCase()
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

function formatSize(bytes) {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + ' ' + units[i]
}

function getFileIcon(ext) {
  const category = getFileCategory(ext)
  switch (category) {
    case 'video': return VideoCamera
    case 'image': return Picture
    default: return Document
  }
}

function getFileTypeTagType(category) {
  const types = {
    document: '',
    ppt: 'warning',
    video: 'danger',
    image: 'success'
  }
  return types[category] || 'info'
}

function getFileTypeLabel(category) {
  const labels = {
    document: '文档',
    ppt: 'PPT',
    video: '视频',
    image: '图片'
  }
  return labels[category] || '文件'
}

function removeFile(file) {
  const index = fileList.value.findIndex(f => f.uid === file.uid)
  if (index > -1) {
    fileList.value.splice(index, 1)
  }
}

function clearAll() {
  fileList.value = []
}

function emitFileStatus(file) {
  emit('file-status', {
    uid: file.uid,
    name: file.name,
    status: file.status,
    percentage: file.percentage,
    category: file.category
  })
}

async function uploadSingleFile(file) {
  file.status = 'uploading'
  file.percentage = 0
  emitFileStatus(file)

  const formData = new FormData()
  formData.append('file', file.raw)

  try {
    const res = await tutorApi.uploadChapterFile(
      props.chapterId,
      formData,
      (progressEvent) => {
        if (progressEvent.total) {
          file.percentage = Math.round((progressEvent.loaded * 100) / progressEvent.total)
          emitFileStatus(file)
        }
      }
    )

    if (res.code === 200 || res.code === 201) {
      file.status = 'success'
      file.percentage = 100
    } else {
      file.status = 'error'
      file.errorMsg = res.message || '上传失败'
    }
  } catch (e) {
    file.status = 'error'
    file.errorMsg = e.message || '上传失败'
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

async function retryFile(file) {
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
.file-upload {
  border: 1px solid #ebeef5;
  border-radius: 8px;
  padding: 20px;
  background: #fff;
}

.filter-bar {
  margin-bottom: 16px;
  overflow-x: auto;
  white-space: nowrap;
}

.upload-area {
  border: 2px dashed #dcdfe6;
  border-radius: 8px;
  padding: 30px 20px;
  text-align: center;
  transition: border-color 0.3s, background-color 0.3s;
  cursor: pointer;
}

.upload-area:hover,
.upload-area.is-dragover {
  border-color: #409eff;
  background-color: #f5f7fa;
}

.upload-icon {
  color: #c0c4cc;
  margin-bottom: 8px;
}

.upload-text {
  color: #606266;
  font-size: 14px;
  margin-bottom: 8px;
}

.upload-text em {
  color: #409eff;
  font-style: normal;
}

.upload-tip {
  color: #909399;
  font-size: 12px;
  margin: 4px 0;
}

.file-list-section {
  margin-top: 20px;
}

.file-list-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid #ebeef5;
}

.summary-text {
  font-size: 14px;
  color: #606266;
  font-weight: 500;
}

.file-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  max-height: 400px;
  overflow-y: auto;
}

.file-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  border: 1px solid #ebeef5;
  border-radius: 6px;
  transition: border-color 0.3s, background-color 0.3s;
}

.file-item.is-error {
  border-color: #f56c6c;
  background-color: #fef0f0;
}

.file-item.is-success {
  border-color: #67c23a;
  background-color: #f0f9eb;
}

.file-info {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 1;
  min-width: 0;
}

.file-type-icon {
  color: #909399;
  flex-shrink: 0;
}

.file-detail {
  flex: 1;
  min-width: 0;
}

.file-name-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.file-name {
  font-size: 14px;
  color: #303133;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 260px;
}

.file-type-tag {
  flex-shrink: 0;
}

.file-size {
  font-size: 12px;
  color: #909399;
  margin-top: 2px;
}

.file-status-area {
  width: 180px;
  flex-shrink: 0;
}

.file-progress {
  width: 100%;
}

.status-done {
  display: flex;
  align-items: center;
  gap: 6px;
}

.status-icon {
  font-size: 18px;
}

.success-icon {
  color: #67c23a;
}

.error-icon {
  color: #f56c6c;
}

.status-text {
  font-size: 13px;
}

.status-pending {
  color: #909399;
}

.status-success {
  color: #67c23a;
}

.status-error {
  color: #f56c6c;
}

.file-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

@media screen and (max-width: 768px) {
  .file-item {
    flex-wrap: wrap;
    gap: 8px;
  }

  .file-info {
    flex: 1 1 100%;
  }

  .file-status-area {
    flex: 1 1 calc(100% - 80px);
    width: auto;
  }

  .file-name {
    max-width: 160px;
  }

  .filter-bar :deep(.el-radio-group) {
    flex-wrap: wrap;
  }
}
</style>
