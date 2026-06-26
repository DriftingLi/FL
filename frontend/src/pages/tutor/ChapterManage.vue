<template>
  <div class="chapter-manage-page" v-loading="loading">
    <div class="page-header">
      <el-button text @click="goBack">
        <el-icon><ArrowLeft /></el-icon> 返回课程列表
      </el-button>
      <h2 v-if="courseInfo">{{ courseInfo.name }} - 章节管理</h2>
    </div>

    <div class="filter-bar">
      <el-radio-group v-model="filterType" size="small" @change="handleFilterChange">
        <el-radio-button value="">全部</el-radio-button>
        <el-radio-button value="document">文档</el-radio-button>
        <el-radio-button value="ppt">PPT</el-radio-button>
        <el-radio-button value="video">视频</el-radio-button>
        <el-radio-button value="image">图片</el-radio-button>
      </el-radio-group>
      <div class="batch-actions" v-if="selectedFileIds.length > 0">
        <span class="selected-count">已选 {{ selectedFileIds.length }} 个文件</span>
        <el-button type="danger" size="small" @click="handleBatchDelete">批量删除</el-button>
        <el-button size="small" @click="clearSelection">取消选择</el-button>
      </div>
    </div>

    <div v-if="filteredChapters.length === 0 && !loading" class="empty-state">
      <el-empty :description="filterType ? '没有该类型的文件' : '暂无章节'" />
    </div>

    <div v-else class="chapter-list">
      <div
        v-for="(chapter, index) in filteredChapters"
        :key="chapter.chapter_id"
        class="chapter-card"
        :class="{ 'has-file': chapter.files && chapter.files.length > 0 }"
      >
        <div class="chapter-header">
          <div class="chapter-title-row">
            <span class="chapter-index">{{ String(index + 1).padStart(2, '0') }}</span>
            <div class="chapter-title-edit">
              <el-input
                v-if="editingId === chapter.chapter_id"
                v-model="editTitle"
                size="small"
                @keyup.enter="saveTitle(chapter)"
                @blur="saveTitle(chapter)"
              />
              <span v-else class="chapter-title">{{ chapter.title }}</span>
            </div>
          </div>
          <div class="chapter-actions">
            <el-button
              size="small"
              text
              type="primary"
              @click="startEditTitle(chapter)"
            >
              <el-icon><Edit /></el-icon>
            </el-button>
            <el-button
              size="small"
              text
              type="primary"
              @click="openUploadDialog(chapter)"
            >
              上传文件
            </el-button>
          </div>
        </div>

        <div v-if="chapter.files && chapter.files.length > 0" class="chapter-files-list">
          <div
            v-for="file in chapter.files"
            :key="file.file_id"
            class="file-item-row"
          >
            <el-checkbox
              v-model="checkState[file.file_id]"
              @change="handleCheckChange"
            />
            <el-icon class="file-type-icon" :size="16">
              <component :is="getFileIcon(file.content_type)" />
            </el-icon>
            <span class="file-name" :title="file.file_name">{{ file.file_name }}</span>
            <el-tag size="small" :type="getContentTypeTagType(file.content_type)">
              {{ getContentTypeLabel(file.content_type) }}
            </el-tag>
            <span v-if="file.file_size" class="file-size">{{ formatSize(file.file_size) }}</span>
            <span v-if="file.created_at" class="file-time">{{ formatTime(file.created_at) }}</span>
            <el-button size="small" text type="danger" @click="handleDeleteFile(file)">
              删除
            </el-button>
          </div>
        </div>

        <div class="chapter-description">
          <el-input
            v-if="descEditingId === chapter.chapter_id"
            v-model="editDescription"
            size="small"
            type="textarea"
            :rows="2"
            placeholder="输入章节描述..."
            @blur="saveDescription(chapter)"
            @keyup.enter.ctrl="saveDescription(chapter)"
          />
          <div
            v-else
            class="desc-display"
            @click="startEditDescription(chapter)"
          >
            <span v-if="chapter.description" class="desc-text">{{ chapter.description }}</span>
            <span v-else class="desc-placeholder">点击添加描述...</span>
          </div>
        </div>
      </div>
    </div>

    <el-dialog
      v-model="uploadDialogVisible"
      :title="`上传文件 - ${uploadChapterTitle}`"
      width="680px"
      top="5vh"
      :close-on-click-modal="false"
      @close="handleUploadDialogClose"
    >
      <FileUpload
        v-if="uploadDialogVisible"
        ref="fileUploadRef"
        :chapter-id="uploadChapterId"
        @upload-all="handleUploadAll"
        @file-status="handleFileStatus"
      />
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Edit, Document, VideoCamera, Picture } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { tutorApi } from '@/api/tutor'
import FileUpload from '@/components/tutor/FileUpload.vue'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const courseInfo = ref(null)
const chapters = ref([])
const editingId = ref(null)
const editTitle = ref('')
const descEditingId = ref(null)
const editDescription = ref('')
const filterType = ref('')
const checkState = reactive({})
const selectedFileIds = ref([])

const uploadDialogVisible = ref(false)
const uploadChapterId = ref(null)
const uploadChapterTitle = ref('')
const fileUploadRef = ref(null)

const filteredChapters = computed(() => {
  if (!filterType.value) return chapters.value
  return chapters.value.filter(ch => ch.files && ch.files.some(f => f.content_type === filterType.value))
})

function getContentTypeTagType(contentType) {
  const types = {
    'document': '',
    'ppt': 'warning',
    'video': 'danger',
    'image': 'success'
  }
  return types[contentType] || 'info'
}

function getContentTypeLabel(contentType) {
  const labels = {
    'document': '文档',
    'ppt': 'PPT',
    'video': '视频',
    'image': '图片'
  }
  return labels[contentType] || contentType
}

function getFileIcon(contentType) {
  const icons = {
    'document': Document,
    'ppt': Document,
    'video': VideoCamera,
    'image': Picture
  }
  return icons[contentType] || Document
}

function formatSize(bytes) {
  if (!bytes) return ''
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + ' ' + units[i]
}

function formatTime(isoStr) {
  if (!isoStr) return ''
  const d = new Date(isoStr)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

async function loadChapters() {
  loading.value = true
  try {
    const courseId = route.params.id
    const res = await tutorApi.getCourseChapters(courseId)
    if (res.code === 200) {
      courseInfo.value = res.data.course
      chapters.value = res.data.chapters
    }
  } catch (e) {
    console.error('Failed to load chapters:', e)
    ElMessage.error('加载章节失败')
  } finally {
    loading.value = false
  }
}

function goBack() {
  router.push('/tutor/courses')
}

function handleFilterChange() {
  clearSelection()
}

function handleCheckChange() {
  selectedFileIds.value = Object.entries(checkState)
    .filter(([_, checked]) => checked)
    .map(([id]) => Number(id))
}

function clearSelection() {
  Object.keys(checkState).forEach(k => checkState[k] = false)
  selectedFileIds.value = []
}

function startEditTitle(chapter) {
  editingId.value = chapter.chapter_id
  editTitle.value = chapter.title
}

async function saveTitle(chapter) {
  if (!editTitle.value.trim()) {
    editingId.value = null
    return
  }
  if (editTitle.value.trim() === chapter.title) {
    editingId.value = null
    return
  }
  try {
    const res = await tutorApi.updateChapter(chapter.chapter_id, { title: editTitle.value.trim() })
    if (res.code === 200) {
      chapter.title = editTitle.value.trim()
      ElMessage.success('标题更新成功')
    }
  } catch (e) {
    console.error('Failed to update title:', e)
    ElMessage.error('更新标题失败')
  } finally {
    editingId.value = null
  }
}

function startEditDescription(chapter) {
  descEditingId.value = chapter.chapter_id
  editDescription.value = chapter.description || ''
}

async function saveDescription(chapter) {
  if (descEditingId.value !== chapter.chapter_id) return
  const newDesc = editDescription.value.trim()
  if (newDesc === (chapter.description || '')) {
    descEditingId.value = null
    return
  }
  try {
    const res = await tutorApi.updateChapter(chapter.chapter_id, { description: newDesc })
    if (res.code === 200) {
      chapter.description = newDesc
      ElMessage.success('描述更新成功')
    }
  } catch (e) {
    console.error('Failed to update description:', e)
    ElMessage.error('更新描述失败')
  } finally {
    descEditingId.value = null
  }
}

function openUploadDialog(chapter) {
  uploadChapterId.value = chapter.chapter_id
  uploadChapterTitle.value = chapter.title
  uploadDialogVisible.value = true
}

function handleUploadDialogClose() {
  if (fileUploadRef.value) {
    fileUploadRef.value.resetState()
  }
  loadChapters()
}

function handleUploadAll(result) {
  if (result.failed === 0) {
    ElMessage.success(`全部上传成功，共${result.total}个文件`)
  } else {
    ElMessage.warning(`上传完成：成功${result.success}个，失败${result.failed}个`)
  }
}

function handleFileStatus(fileInfo) {
}

async function handleDeleteFile(file) {
  try {
    await ElMessageBox.confirm(
      `确定要删除文件"${file.file_name}"吗？`,
      '确认删除',
      { confirmButtonText: '确定', cancelButtonText: '取消', type: 'warning' }
    )
    const res = await tutorApi.deleteFile(file.file_id)
    if (res.code === 200) {
      ElMessage.success('文件删除成功')
      await loadChapters()
    }
  } catch (e) {
    if (e !== 'cancel') {
      console.error('Failed to delete file:', e)
      ElMessage.error('删除文件失败')
    }
  }
}

async function handleBatchDelete() {
  if (selectedFileIds.value.length === 0) return
  try {
    await ElMessageBox.confirm(
      `确定要删除选中的 ${selectedFileIds.value.length} 个文件吗？此操作不可恢复。`,
      '批量删除确认',
      { confirmButtonText: '确定删除', cancelButtonText: '取消', type: 'warning' }
    )
    const res = await tutorApi.batchDeleteFiles({ file_ids: selectedFileIds.value })
    if (res.code === 200) {
      ElMessage.success(res.message)
      clearSelection()
      await loadChapters()
    }
  } catch (e) {
    if (e !== 'cancel') {
      console.error('Batch delete failed:', e)
      ElMessage.error('批量删除失败')
    }
  }
}

onMounted(() => {
  loadChapters()
})
</script>

<style scoped>
.chapter-manage-page {
  max-width: 1000px;
  margin: 0 auto;
}

.page-header {
  margin-bottom: 20px;
}

.page-header h2 {
  font-size: 20px;
  color: #303133;
  margin-top: 8px;
}

.filter-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
  flex-wrap: wrap;
  gap: 12px;
}

.batch-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.selected-count {
  font-size: 13px;
  color: #909399;
}

.chapter-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.chapter-card {
  background: #fff;
  border-radius: 8px;
  padding: 16px 20px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
  border: 1px solid #ebeef5;
  transition: border-color 0.2s;
}

.chapter-card.has-file {
  border-left: 3px solid #409eff;
}

.chapter-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.chapter-title-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 1;
  min-width: 0;
}

.chapter-index {
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #409eff;
  color: #fff;
  border-radius: 50%;
  font-size: 13px;
  font-weight: 600;
  flex-shrink: 0;
}

.chapter-title-edit {
  flex: 1;
  min-width: 0;
}

.chapter-title {
  font-size: 15px;
  color: #303133;
  font-weight: 500;
}

.chapter-actions {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.chapter-files-list {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid #f0f0f0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.file-item-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  border-radius: 4px;
  transition: background 0.2s;
}

.file-item-row:hover {
  background: #f5f7fa;
}

.file-type-icon {
  color: #409eff;
  flex-shrink: 0;
}

.file-name {
  font-size: 13px;
  color: #303133;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 200px;
  flex: 1;
  min-width: 0;
}

.file-size {
  font-size: 12px;
  color: #909399;
}

.file-time {
  font-size: 12px;
  color: #c0c4cc;
}

.chapter-description {
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid #f5f5f5;
}

.desc-display {
  cursor: pointer;
  padding: 4px 8px;
  border-radius: 4px;
  min-height: 28px;
  transition: background 0.2s;
}

.desc-display:hover {
  background: #f5f7fa;
}

.desc-text {
  font-size: 13px;
  color: #606266;
  line-height: 1.5;
}

.desc-placeholder {
  font-size: 13px;
  color: #c0c4cc;
  font-style: italic;
}

@media screen and (max-width: 768px) {
  .chapter-header {
    flex-direction: column;
    align-items: flex-start;
  }

  .chapter-actions {
    width: 100%;
    justify-content: flex-end;
  }

  .filter-bar {
    flex-direction: column;
    align-items: flex-start;
  }

  .file-item-row {
    flex-wrap: wrap;
  }

  .file-name {
    max-width: 140px;
  }
}
</style>
