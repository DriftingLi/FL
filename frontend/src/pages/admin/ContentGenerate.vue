<template>
  <div class="content-generate-page">
    <div class="page-header">
      <h2>课程内容预生成</h2>
    </div>

    <div class="generate-card">
      <el-form label-width="100px">
        <el-form-item label="选择课程">
          <el-select
            v-model="selectedCourseId"
            placeholder="请选择课程"
            style="width: 100%"
            @change="handleCourseChange"
          >
            <el-option
              v-for="course in courses"
              :key="course.course_id"
              :label="course.name"
              :value="course.course_id"
            />
          </el-select>
        </el-form-item>

        <el-form-item label="选择章节" v-if="chapters.length > 0">
          <el-checkbox-group v-model="selectedChapterIds">
            <el-checkbox
              v-for="chapter in chapters"
              :key="chapter.chapter_id"
              :label="chapter.chapter_id"
            >
              {{ chapter.title }}
              <el-tag
                v-if="chapter.content && chapter.content.length > 100"
                type="success"
                size="small"
                style="margin-left: 8px"
              >
                已有内容
              </el-tag>
            </el-checkbox>
          </el-checkbox-group>
          <div class="select-actions">
            <UiButton variant="primary" size="small" class="select-all-btn" @click="selectAll">全选</UiButton>
            <UiButton variant="text" size="small" @click="selectNone">取消全选</UiButton>
          </div>
        </el-form-item>

        <el-form-item>
          <UiButton variant="primary" size="large" :loading="generating || isTaskRunning" :disabled="!selectedCourseId || selectedChapterIds.length === 0 || isTaskRunning" @click="handleGenerate">
            <el-icon><MagicStick /></el-icon>
            {{ isTaskRunning ? '生成中...' : '开始生成' }}
          </UiButton>
          <span v-if="isTaskRunning" class="generating-tip">
            <el-icon class="is-loading"><Loading /></el-icon>
            正在生成，请勿重复提交
          </span>
        </el-form-item>
      </el-form>
    </div>

    <UiCard v-if="generateTask" padding="lg" class="progress-card">
      <h3>生成进度</h3>
      <el-progress
        :percentage="progressPercent"
        :status="generateTask.status === 'completed' ? 'success' : (generateTask.status === 'failed' ? 'exception' : '')"
        :stroke-width="20"
        :text-inside="true"
        style="margin-bottom: 16px"
      />
      <div class="progress-detail">
        <span v-if="(generateTask.total ?? 0) > 0" class="progress-count">
          已完成 {{ generateTask.completed || 0 }} / {{ generateTask.total }} 个章节
        </span>
        <span v-if="generateTask.status === 'processing'" class="progress-status processing">
          <el-icon class="is-loading"><Loading /></el-icon> 正在调用 AI 生成内容，预计每个章节约 20-40 秒...
        </span>
        <span v-else-if="generateTask.status === 'completed'" class="progress-status completed">
          生成完成
        </span>
        <span v-else-if="generateTask.status === 'failed'" class="progress-status failed">
          生成失败
        </span>
        <span v-else-if="generateTask.status === 'pending'" class="progress-status pending">
          <el-icon class="is-loading"><Loading /></el-icon> 任务排队中...
        </span>
      </div>

      <div v-if="generateTask.results && generateTask.results.length > 0" class="result-list">
        <div
          v-for="item in generateTask.results"
          :key="item.chapter_id"
          class="result-item"
          :class="{ 'result-success': item.status === 'success', 'result-failed': item.status === 'failed' }"
        >
          <el-icon v-if="item.status === 'success'" class="status-icon success"><CircleCheck /></el-icon>
          <el-icon v-else class="status-icon failed"><CircleClose /></el-icon>
          <span class="chapter-title">{{ item.title }}</span>
          <el-tag :type="item.status === 'success' ? 'success' : 'danger'" size="small">
            {{ item.status === 'success' ? '生成成功' : '生成失败' }}
          </el-tag>
          <UiButton variant="primary" text v-if="item.status === 'success' && item.content" size="small" @click="previewChapter(item)">
            预览
          </UiButton>
          <span v-if="item.error" class="error-msg">{{ item.error }}</span>
        </div>
      </div>
    </UiCard>

    <el-dialog
      v-model="previewVisible"
      :title="`预览 - ${previewTitle}`"
      width="700px"
      destroy-on-close
    >
      <div class="preview-content markdown-body" v-html="renderedPreview"></div>
      <template #footer>
        <UiButton @click="previewVisible = false">关闭</UiButton>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onUnmounted } from 'vue'
import { MagicStick, CircleCheck, CircleClose, Loading } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { marked } from 'marked'
import { adminApi } from '@/api/admin'
import '@/assets/styles/markdown.css'
import UiButton from '@/components/ui/UiButton.vue'
import UiCard from '@/components/ui/UiCard.vue'

interface GenerateCourse {
  course_id: number
  name: string
}

interface GenerateChapter {
  chapter_id: number
  title: string
  content?: string
}

interface GenerateTask {
  task_id: string
  status: string
  total?: number
  completed?: number
  results?: {
    chapter_id: number
    title: string
    status: string
    content?: string
    error?: string
  }[]
}

const courses = ref<GenerateCourse[]>([])
const chapters = ref<GenerateChapter[]>([])
const selectedCourseId = ref<number | null>(null)
const selectedChapterIds = ref<number[]>([])
const generating = ref(false)
const generateTask = ref<GenerateTask | null>(null)
let pollTimer: ReturnType<typeof setInterval> | null = null

const previewVisible = ref(false)
const previewTitle = ref('')
const previewContent = ref('')

const renderedPreview = computed(() => {
  if (!previewContent.value) return ''
  return marked.parse(previewContent.value)
})

const progressPercent = computed(() => {
  if (!generateTask.value) return 0
  const { total = 0, completed = 0 } = generateTask.value
  if (generateTask.value.status === 'completed') return 100
  if (total === 0) return 0
  return Math.round((completed / total) * 100)
})

// 任务运行中（pending/processing）：用于锁住"开始生成"按钮，防止重复提交
const isTaskRunning = computed(() => {
  if (!generateTask.value) return false
  const s = generateTask.value.status
  return s === 'pending' || s === 'processing'
})

async function loadCourses() {
  try {
    const data = await adminApi.getCourses({ page: 1, page_size: 100 })
    if (data) {
      courses.value = data.courses
    }
  } catch (error) {
    console.error('加载课程失败:', error)
  }
}

async function handleCourseChange(courseId: number) {
  selectedChapterIds.value = []
  chapters.value = []

  if (!courseId) return

  try {
    const detail = await adminApi.getCourseDetail(courseId)
    if (detail) {
      chapters.value = detail.chapters || []
    }
  } catch (error) {
    console.error('加载章节失败:', error)
  }
}

function selectAll() {
  selectedChapterIds.value = chapters.value.map(c => c.chapter_id)
}

function selectNone() {
  selectedChapterIds.value = []
}

async function handleGenerate() {
  if (!selectedCourseId.value || selectedChapterIds.value.length === 0) {
    ElMessage.warning('请选择课程和章节')
    return
  }

  generating.value = true
  generateTask.value = null

  try {
    const task = await adminApi.generateContent({
      course_id: selectedCourseId.value,
      chapter_ids: selectedChapterIds.value
    })

    if (task) {
      generateTask.value = task
      startPolling(task.task_id)
    }
  } catch (error) {
    console.error('内容生成失败:', error)
    /* 错误已由拦截器提示 */
  } finally {
    generating.value = false
  }
}

function startPolling(taskId: string) {
  if (pollTimer) clearInterval(pollTimer)

  pollTimer = setInterval(async () => {
    try {
      const task = await adminApi.getGenerateStatus(taskId)
      if (task) {
        generateTask.value = task
        if (task.status === 'completed' || task.status === 'failed') {
          if (pollTimer) {
            clearInterval(pollTimer)
            pollTimer = null
          }
          const successCount = (task.results || []).filter((r: { status: string }) => r.status === 'success').length
          const total = task.total || task.results?.length || 0
          if (task.status === 'completed') {
            ElMessage.success(`生成完成：${successCount}/${total} 个章节成功`)
          }
          if (selectedCourseId.value) {
            handleCourseChange(selectedCourseId.value)
          }
        }
      }
    } catch (error) {
      if (pollTimer) {
        clearInterval(pollTimer)
        pollTimer = null
      }
    }
  }, 3000)
}

function previewChapter(item: { title: string; content?: string }) {
  previewTitle.value = item.title
  previewContent.value = item.content || ''
  previewVisible.value = true
}

onUnmounted(() => {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
})

loadCourses()
</script>

<style scoped>
.content-generate-page {
  padding: 20px;
  max-width: 800px;
  margin: 0 auto;
}

.page-header {
  text-align: center;
  margin-bottom: 30px;
}

.page-header h2 {
  font-size: 22px;
  color: var(--color-text-primary);
  margin-bottom: 8px;
}

/* .progress-card 容器（底色/圆角/内距/投影）已由 UiCard 承担，从共用规则中拆出 */
.generate-card {
  background: var(--color-text-inverse);
  border-radius: 12px;
  padding: 24px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
  margin-bottom: 20px;
}

.progress-card {
  margin-bottom: 20px;
}

.select-actions {
  margin-top: 8px;
}

/* 全选按钮：白色字体 + primary 背景 */
.select-all-btn {
  color: var(--color-text-inverse) !important;
}

.generating-tip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin-left: 12px;
  font-size: 13px;
  color: var(--color-primary-500);
}

.progress-count {
  font-weight: 500;
  color: var(--color-text-primary);
}

.progress-status.pending {
  color: var(--color-text-tertiary);
}

.progress-card h3 {
  font-size: 16px;
  color: var(--color-text-primary);
  margin-bottom: 16px;
}

.progress-detail {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  font-size: 14px;
  color: var(--color-text-secondary);
}

.progress-status {
  display: flex;
  align-items: center;
  gap: 4px;
}

.progress-status.processing {
  color: var(--color-primary-500);
}

.progress-status.completed {
  color: var(--color-success);
}

.progress-status.failed {
  color: var(--color-danger);
}

.result-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 16px;
}

.result-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  border-radius: 6px;
  background: var(--color-bg-page);
  flex-wrap: wrap;
}

.result-item.result-success {
  background: var(--color-success-light);
}

.result-item.result-failed {
  background: var(--color-danger-light);
}

.status-icon {
  font-size: 18px;
}

.status-icon.success {
  color: var(--color-success);
}

.status-icon.failed {
  color: var(--color-danger);
}

.chapter-title {
  flex: 1;
  font-size: 14px;
  color: var(--color-text-primary);
  min-width: 100px;
}

.error-msg {
  font-size: 12px;
  color: var(--color-danger);
}

.preview-content {
  max-height: 500px;
  overflow-y: auto;
  padding: 16px;
  background: var(--color-bg-page);
  border-radius: 8px;
}

@media screen and (max-width: 768px) {
  .content-generate-page {
    padding: 12px;
  }

  .generate-card {
    padding: 16px;
  }

  .generate-card :deep(.el-form-item__label) {
    width: 80px !important;
  }

  .page-header {
    margin-bottom: 16px;
  }

  .page-header h2 {
    font-size: 18px;
  }

  .result-item {
    padding: 8px 10px;
  }
}

@media screen and (max-width: 480px) {
  .generate-card :deep(.el-form-item__label) {
    width: 70px !important;
    font-size: 13px;
  }
}
</style>
