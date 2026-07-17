<template>
  <div class="chapter-edit-page" v-loading="loading">
    <template v-if="chapterNotFound">
      <el-empty description="章节不存在或已删除">
        <el-button type="primary" @click="goBackToChapters">返回章节列表</el-button>
      </el-empty>
    </template>

    <template v-else-if="chapterDetail">
      <!-- 面包屑 -->
      <div class="breadcrumb">
        <el-button text @click="goBackToChapters">
          <el-icon><ArrowLeft /></el-icon> 返回章节列表
        </el-button>
        <span class="separator">/</span>
        <span class="course-name">{{ courseName || '课程' }}</span>
        <span class="separator">/</span>
        <span class="chapter-name">{{ chapterDetail.title }}</span>
      </div>

      <!-- 章节标题 + 元信息编辑 -->
      <div class="chapter-header">
        <div class="title-row">
          <h1 class="chapter-title">{{ chapterDetail.title }}</h1>
          <el-button size="small" type="primary" @click="openMetaDialog">
            <el-icon><Edit /></el-icon> 编辑信息
          </el-button>
        </div>
      </div>

      <!-- Tabs 切换 -->
      <div class="chapter-content-area">
        <el-tabs v-model="activeTab" class="content-tabs">
          <!-- 图文 Tab（始终显示，可编辑） -->
          <el-tab-pane label="图文" name="content">
            <div class="content-tab">
              <div class="tab-toolbar">
                <span class="tab-tip">使用 Markdown 编辑，支持实时预览</span>
                <el-button
                  type="primary"
                  size="small"
                  :loading="savingContent"
                  :disabled="!contentChanged"
                  @click="saveContent"
                >
                  <el-icon><Check /></el-icon> 保存正文
                </el-button>
              </div>
              <MarkdownEditor
                v-model="editContent"
                :height="560"
                placeholder="请输入章节正文内容（支持 Markdown 语法）..."
              />
            </div>
          </el-tab-pane>

          <!-- 媒体类型 Tabs（按 TYPE_ORDER 顺序） -->
          <el-tab-pane
            v-for="group in fileGroups"
            :key="group.type"
            :name="group.type"
          >
            <template #label>
              <span class="tab-label">
                <el-icon :size="16" :style="{ color: group.color }">
                  <component :is="group.icon" />
                </el-icon>
                {{ group.label }}
                <el-tag size="small" type="info" class="tab-count">{{ group.files.length }}</el-tag>
              </span>
            </template>

            <div class="media-tab">
              <!-- 已上传文件列表 -->
              <div class="file-section">
                <div class="section-header">
                  <h3>已上传文件（{{ group.files.length }}）</h3>
                  <el-button
                    type="primary"
                    size="small"
                    @click="openUploadDialog(group.type)"
                  >
                    <el-icon><Upload /></el-icon> 上传{{ group.label }}
                  </el-button>
                </div>

                <div v-if="group.files.length > 0" class="file-list">
                  <div
                    v-for="file in group.files"
                    :key="file.file_id"
                    class="file-item"
                    :class="{ 'is-active': selectedFileId === file.file_id }"
                    @click="selectFile(file.file_id)"
                  >
                    <el-icon class="file-icon" :size="18">
                      <component :is="group.icon" />
                    </el-icon>
                    <div class="file-info">
                      <span class="file-name" :title="file.file_name">{{ file.file_name }}</span>
                      <span class="file-size" v-if="file.file_size">{{ formatSize(file.file_size) }}</span>
                    </div>
                    <el-button
                      size="small"
                      type="danger"
                      text
                      @click.stop="handleDeleteFile(file)"
                    >
                      <el-icon><Delete /></el-icon>
                    </el-button>
                  </div>
                </div>
                <el-empty v-else :description="`暂无${group.label}文件`" :image-size="60" />
              </div>

              <!-- 选中文件预览 -->
              <div class="preview-section" v-if="selectedFile">
                <h3>预览：{{ selectedFile.file_name }}</h3>
                <div class="preview-content">
                  <template v-if="group.type === 'video'">
                    <VideoPlayer :src="selectedFile.file_url" />
                  </template>
                  <template v-else-if="group.type === 'document'">
                    <DocumentViewer :src="selectedFile.file_url" :fileName="selectedFile.file_name" />
                  </template>
                  <template v-else-if="group.type === 'ppt'">
                    <PptViewer
                      :src="selectedFile.file_url"
                      :fileName="selectedFile.file_name"
                      :chapterId="chapterDetail.chapter_id"
                    />
                  </template>
                  <template v-else-if="group.type === 'image'">
                    <ImageViewer :src="selectedFile.file_url" :fileName="selectedFile.file_name" />
                  </template>
                </div>
              </div>
            </div>
          </el-tab-pane>
        </el-tabs>
      </div>

      <!-- 上一章 / 下一章 -->
      <div class="chapter-navigation">
        <div class="nav-prev">
          <el-button
            :disabled="!chapterDetail.previous_chapter_id"
            @click="navigateToChapter(chapterDetail.previous_chapter_id)"
            text
          >
            <el-icon><ArrowLeft /></el-icon>
            <div class="nav-btn-content" v-if="chapterDetail.previous_chapter_id">
              <span class="nav-label">上一章节</span>
              <span class="nav-title">{{ getPrevChapterTitle }}</span>
            </div>
            <span v-else class="nav-label">没有上一章节</span>
          </el-button>
        </div>
        <div class="nav-next">
          <el-button
            :disabled="!chapterDetail.next_chapter_id"
            @click="navigateToChapter(chapterDetail.next_chapter_id)"
            text
          >
            <div class="nav-btn-content" v-if="chapterDetail.next_chapter_id">
              <span class="nav-label">下一章节</span>
              <span class="nav-title">{{ getNextChapterTitle }}</span>
            </div>
            <span v-else class="nav-label">没有下一章节</span>
            <el-icon><ArrowRight /></el-icon>
          </el-button>
        </div>
      </div>
    </template>

    <!-- 元信息编辑弹窗 -->
    <el-dialog v-model="metaDialogVisible" title="编辑章节信息" width="500px">
      <el-form :model="metaForm" label-width="100px">
        <el-form-item label="章节标题">
          <el-input v-model="metaForm.title" placeholder="请输入章节标题" maxlength="100" show-word-limit />
        </el-form-item>
        <el-form-item label="章节描述">
          <el-input
            v-model="metaForm.description"
            type="textarea"
            :rows="3"
            placeholder="章节描述（可选）"
          />
        </el-form-item>
        <el-form-item label="预计时长">
          <el-input-number v-model="metaForm.duration" :min="0" :max="9999" />
          <span style="margin-left: 8px; color: #909399; font-size: 13px;">分钟</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="metaDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="savingMeta" @click="saveMeta">保存</el-button>
      </template>
    </el-dialog>

    <!-- 上传文件弹窗 -->
    <el-dialog
      v-model="uploadDialogVisible"
      :title="`上传${uploadTypeLabel}文件`"
      width="720px"
      :close-on-click-modal="false"
      @close="handleUploadDialogClose"
    >
      <FileUpload
        v-if="uploadDialogVisible"
        ref="fileUploadRef"
        :chapter-id="chapterDetail?.chapter_id"
        :initial-filter="uploadType"
        @upload-all="handleUploadAll"
      />
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  ArrowLeft, ArrowRight, Edit, Check, Upload, Delete,
  VideoCamera, Document, Picture
} from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { tutorApi } from '@/api/tutor'
import MarkdownEditor from '@/components/tutor/MarkdownEditor.vue'
import FileUpload from '@/components/tutor/FileUpload.vue'
import VideoPlayer from '@/components/student/VideoPlayer.vue'
import DocumentViewer from '@/components/student/DocumentViewer.vue'
import PptViewer from '@/components/student/PptViewer.vue'
import ImageViewer from '@/components/student/ImageViewer.vue'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const chapterNotFound = ref(false)
const chapterDetail = ref<any>(null)
const courseName = ref('')
const chapters = ref<any[]>([])
const activeTab = ref('content')

// 正文编辑
const editContent = ref('')
const originalContent = ref('')
const savingContent = ref(false)
const contentChanged = computed(() => editContent.value !== originalContent.value)

// 文件分组
const TYPE_ORDER = ['video', 'document', 'ppt', 'image']
const TYPE_CONFIG: Record<string, { label: string, icon: any, color: string }> = {
  video: { label: '视频', icon: VideoCamera, color: '#f56c6c' },
  document: { label: '文档', icon: Document, color: '#409eff' },
  ppt: { label: 'PPT', icon: Document, color: '#e6a23c' },
  image: { label: '图片', icon: Picture, color: '#67c23a' }
}

const chapterFiles = computed(() => chapterDetail.value?.files || [])

const fileGroups = computed(() => {
  const groups: Record<string, any[]> = {}
  for (const file of chapterFiles.value) {
    const type = file.content_type || 'document'
    if (!groups[type]) groups[type] = []
    groups[type].push(file)
  }
  // 导师端始终展示所有媒体类型 tab，即使没有文件也可切换进入上传
  return TYPE_ORDER.map(type => ({
    type,
    label: TYPE_CONFIG[type]?.label || type,
    icon: TYPE_CONFIG[type]?.icon || Document,
    color: TYPE_CONFIG[type]?.color || '#909399',
    files: groups[type] || []
  }))
})

// 选中文件预览
const selectedFileId = ref<number | null>(null)
const selectedFile = computed(() => {
  if (selectedFileId.value == null) return null
  return chapterFiles.value.find((f: any) => f.file_id === selectedFileId.value) || null
})

function selectFile(fileId: number) {
  selectedFileId.value = selectedFileId.value === fileId ? null : fileId
}

// 元信息编辑
const metaDialogVisible = ref(false)
const savingMeta = ref(false)
const metaForm = ref({ title: '', description: '', duration: 0 })

function openMetaDialog() {
  if (!chapterDetail.value) return
  metaForm.value = {
    title: chapterDetail.value.title || '',
    description: chapterDetail.value.description || '',
    duration: chapterDetail.value.duration || 0
  }
  metaDialogVisible.value = true
}

async function saveMeta() {
  if (!chapterDetail.value) return
  if (!metaForm.value.title.trim()) {
    ElMessage.warning('章节标题不能为空')
    return
  }
  savingMeta.value = true
  try {
    const res = await tutorApi.updateChapter(chapterDetail.value.chapter_id, {
      title: metaForm.value.title.trim(),
      description: metaForm.value.description,
      duration: metaForm.value.duration
    })
    if (res.code === 200) {
      chapterDetail.value.title = metaForm.value.title.trim()
      chapterDetail.value.description = metaForm.value.description
      chapterDetail.value.duration = metaForm.value.duration
      ElMessage.success('章节信息更新成功')
      metaDialogVisible.value = false
    }
  } catch (e: any) {
    ElMessage.error(e.message || '更新失败')
  } finally {
    savingMeta.value = false
  }
}

// 正文保存
async function saveContent() {
  if (!chapterDetail.value) return
  savingContent.value = true
  try {
    const res = await tutorApi.updateChapter(chapterDetail.value.chapter_id, {
      content: editContent.value
    })
    if (res.code === 200) {
      chapterDetail.value.content = editContent.value
      originalContent.value = editContent.value
      ElMessage.success('正文保存成功')
    }
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
  } finally {
    savingContent.value = false
  }
}

// 上传弹窗
const uploadDialogVisible = ref(false)
const uploadType = ref('')
const fileUploadRef = ref<any>(null)

const uploadTypeLabel = computed(() => {
  if (!uploadType.value) return ''
  return TYPE_CONFIG[uploadType.value]?.label || ''
})

function openUploadDialog(type: string) {
  uploadType.value = type
  uploadDialogVisible.value = true
}

function handleUploadDialogClose() {
  if (fileUploadRef.value) {
    fileUploadRef.value.resetState()
  }
  loadChapterDetail()
}

function handleUploadAll(result: any) {
  if (result.failed === 0) {
    ElMessage.success(`全部上传成功，共${result.total}个文件`)
  } else {
    ElMessage.warning(`上传完成：成功${result.success}个，失败${result.failed}个`)
  }
  // 上传完成后立即刷新章节详情，让用户看到新文件
  loadChapterDetail()
}

// 删除文件
async function handleDeleteFile(file: any) {
  try {
    await ElMessageBox.confirm(
      `确定要删除文件"${file.file_name}"吗？`,
      '确认删除',
      { confirmButtonText: '确定', cancelButtonText: '取消', type: 'warning' }
    )
    const res = await tutorApi.deleteFile(file.file_id)
    if (res.code === 200) {
      ElMessage.success('文件删除成功')
      if (selectedFileId.value === file.file_id) {
        selectedFileId.value = null
      }
      await loadChapterDetail()
    }
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(e.message || '删除失败')
    }
  }
}

// 数据加载
async function loadChapterDetail() {
  loading.value = true
  chapterNotFound.value = false
  try {
    const chapterId = route.params.chapterId
    const res = await tutorApi.getChapterDetail(chapterId)
    if (res.code === 200) {
      chapterDetail.value = res.data
      editContent.value = res.data.content || ''
      originalContent.value = res.data.content || ''
      // 默认 tab：图文优先，否则第一个媒体 tab
      activeTab.value = 'content'
      selectedFileId.value = null
      // 顺便加载课程信息拿课程名 + 章节列表（用于上下章标题）
      await loadCourseInfo()
    } else {
      chapterNotFound.value = true
    }
  } catch (e: any) {
    if (e?.response?.status === 404) {
      chapterNotFound.value = true
    } else {
      ElMessage.error('加载章节详情失败')
    }
  } finally {
    loading.value = false
  }
}

async function loadCourseInfo() {
  try {
    const courseId = route.params.courseId
    const res = await tutorApi.getCourseChapters(courseId)
    if (res.code === 200) {
      courseName.value = res.data.course?.name || ''
      chapters.value = res.data.chapters || []
    }
  } catch (e) {
    // 静默失败
  }
}

const getPrevChapterTitle = computed(() => {
  if (!chapterDetail.value?.previous_chapter_id) return ''
  const prev = chapters.value.find(c => c.chapter_id === chapterDetail.value.previous_chapter_id)
  return prev ? prev.title : ''
})

const getNextChapterTitle = computed(() => {
  if (!chapterDetail.value?.next_chapter_id) return ''
  const next = chapters.value.find(c => c.chapter_id === chapterDetail.value.next_chapter_id)
  return next ? next.title : ''
})

function navigateToChapter(chapterId: any) {
  if (!chapterId) return
  const courseId = route.params.courseId
  router.push({
    name: 'TutorChapterEdit',
    params: { courseId, chapterId }
  })
}

function goBackToChapters() {
  const courseId = route.params.courseId
  router.push(`/training/tutor/course/${courseId}/chapters`)
}

function formatSize(bytes: number) {
  if (!bytes) return ''
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + ' ' + units[i]
}

// 切换章节时重新加载
watch(() => route.params.chapterId, (newId) => {
  if (newId) {
    loadChapterDetail()
  }
})

// 切换 Tab 时重置选中文件，避免跨 Tab 预览错误
watch(activeTab, () => {
  selectedFileId.value = null
})

onMounted(() => {
  loadChapterDetail()
})
</script>

<style scoped>
.chapter-edit-page {
  max-width: 1200px;
  margin: 0 auto;
  padding-bottom: 60px;
}

.breadcrumb {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  color: #606266;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.separator {
  color: #c0c4cc;
}

.course-name,
.chapter-name {
  color: #303133;
}

.chapter-header {
  margin-bottom: 20px;
}

.title-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.chapter-title {
  font-size: 22px;
  font-weight: 600;
  color: #303133;
  margin: 0;
}

.chapter-content-area {
  background: #fff;
  border-radius: 8px;
  border: 1px solid #ebeef5;
  padding: 20px;
  margin-bottom: 24px;
}

.content-tabs :deep(.el-tabs__header) {
  position: sticky;
  top: 0;
  background: #fff;
  z-index: 2;
  margin-bottom: 16px;
}

.tab-label {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.tab-count {
  margin-left: 4px;
}

/* 图文 Tab */
.content-tab {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.tab-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.tab-tip {
  font-size: 13px;
  color: #909399;
}

/* 媒体 Tab */
.media-tab {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.file-section {
  border: 1px solid #ebeef5;
  border-radius: 6px;
  padding: 16px;
  background: #fafbfc;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.section-header h3 {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
  margin: 0;
}

.file-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.file-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  background: #fff;
  border: 1px solid #ebeef5;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s;
}

.file-item:hover {
  border-color: #409eff;
}

.file-item.is-active {
  border-color: #409eff;
  background: #ecf5ff;
}

.file-icon {
  color: #409eff;
  flex-shrink: 0;
}

.file-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.file-name {
  font-size: 14px;
  color: #303133;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-size {
  font-size: 12px;
  color: #909399;
}

.preview-section {
  border: 1px solid #ebeef5;
  border-radius: 6px;
  padding: 16px;
  background: #fff;
}

.preview-section h3 {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
  margin: 0 0 12px 0;
}

.preview-content {
  min-height: 200px;
}

/* 上下章导航 */
.chapter-navigation {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 0;
  border-top: 1px solid #ebeef5;
}

.nav-prev,
.nav-next {
  flex: 1;
  max-width: 48%;
}

.nav-next {
  text-align: right;
}

.nav-btn-content {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
}

.nav-next .nav-btn-content {
  align-items: flex-end;
}

.nav-label {
  font-size: 12px;
  color: #909399;
}

.nav-title {
  font-size: 14px;
  color: #303133;
  font-weight: 500;
  margin-top: 2px;
}

@media screen and (max-width: 768px) {
  .chapter-navigation {
    flex-direction: column;
  }
  .nav-prev,
  .nav-next {
    max-width: 100%;
    text-align: left;
  }
  .nav-next .nav-btn-content {
    align-items: flex-start;
  }
}
</style>
