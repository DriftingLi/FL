<template>
  <div class="content-generate-page">
    <div class="page-header">
      <h2>课程内容预生成</h2>
      <p class="subtitle">使用AI自动生成课程章节内容</p>
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
            <el-button text type="primary" size="small" @click="selectAll">全选</el-button>
            <el-button text size="small" @click="selectNone">取消全选</el-button>
          </div>
        </el-form-item>

        <el-form-item>
          <el-button
            type="primary"
            size="large"
            :loading="generating"
            :disabled="!selectedCourseId || selectedChapterIds.length === 0"
            @click="handleGenerate"
          >
            <el-icon><MagicStick /></el-icon>
            开始生成
          </el-button>
        </el-form-item>
      </el-form>
    </div>

    <div v-if="generateTask" class="progress-card">
      <h3>生成进度</h3>
      <el-progress
        :percentage="progressPercent"
        :status="generateTask.status === 'completed' ? 'success' : (generateTask.status === 'failed' ? 'exception' : '')"
        :stroke-width="20"
        :text-inside="true"
        style="margin-bottom: 16px"
      />
      <div class="progress-detail">
        <span v-if="generateTask.total > 0">
          {{ generateTask.completed || 0 }} / {{ generateTask.total }} 个章节
        </span>
        <span v-if="generateTask.status === 'processing'" class="progress-status processing">
          <el-icon class="is-loading"><Loading /></el-icon> 生成中...
        </span>
        <span v-else-if="generateTask.status === 'completed'" class="progress-status completed">
          生成完成
        </span>
        <span v-else-if="generateTask.status === 'failed'" class="progress-status failed">
          生成失败
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
          <el-button
            v-if="item.status === 'success' && item.content"
            text
            type="primary"
            size="small"
            @click="previewChapter(item)"
          >
            预览
          </el-button>
          <span v-if="item.error" class="error-msg">{{ item.error }}</span>
        </div>
      </div>
    </div>

    <el-dialog
      v-model="previewVisible"
      :title="`预览 - ${previewTitle}`"
      width="700px"
      destroy-on-close
    >
      <div class="preview-content markdown-body" v-html="renderedPreview"></div>
      <template #footer>
        <el-button @click="previewVisible = false">关闭</el-button>
      </template>
    </el-dialog>

    <div class="tips-card">
      <h4>使用说明</h4>
      <ul>
        <li>AI将根据课程名称和章节标题自动生成培训内容</li>
        <li>生成的内容会直接写入对应章节，覆盖原有内容</li>
        <li>已标记"已有内容"的章节如需更新，请勾选后重新生成</li>
        <li>生成过程可能需要几分钟，请耐心等待</li>
        <li>如AI服务未配置，生成将返回错误提示</li>
      </ul>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onUnmounted } from 'vue'
import { MagicStick, CircleCheck, CircleClose, Loading } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { marked } from 'marked'
import { adminApi } from '@/api/admin'
import '@/assets/styles/markdown.css'

const courses = ref([])
const chapters = ref([])
const selectedCourseId = ref(null)
const selectedChapterIds = ref([])
const generating = ref(false)
const generateTask = ref(null)
let pollTimer = null

const previewVisible = ref(false)
const previewTitle = ref('')
const previewContent = ref('')

const renderedPreview = computed(() => {
  if (!previewContent.value) return ''
  return marked.parse(previewContent.value)
})

const progressPercent = computed(() => {
  if (!generateTask.value) return 0
  if (generateTask.value.status === 'completed') return 100
  if (generateTask.value.status === 'failed') return generateTask.value.total > 0
    ? Math.round((generateTask.value.completed / generateTask.value.total) * 100)
    : 0
  if (!generateTask.value.total || generateTask.value.total === 0) return 0
  return Math.round((generateTask.value.completed / generateTask.value.total) * 100)
})

async function loadCourses() {
  try {
    const res = await adminApi.getCourses({ page: 1, page_size: 100 })
    if (res.code === 200) {
      courses.value = res.data.courses
    }
  } catch (error) {
    console.error('加载课程失败:', error)
  }
}

async function handleCourseChange(courseId) {
  selectedChapterIds.value = []
  chapters.value = []

  if (!courseId) return

  try {
    const res = await adminApi.getCourseDetail(courseId)
    if (res.code === 200) {
      chapters.value = res.data.chapters || []
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
    const res = await adminApi.generateContent({
      course_id: selectedCourseId.value,
      chapter_ids: selectedChapterIds.value
    })

    if (res.code === 200) {
      generateTask.value = res.data
      startPolling(res.data.task_id)
    }
  } catch (error) {
    console.error('内容生成失败:', error)
    ElMessage.error('内容生成失败，请检查AI服务配置')
  } finally {
    generating.value = false
  }
}

function startPolling(taskId) {
  if (pollTimer) clearInterval(pollTimer)

  pollTimer = setInterval(async () => {
    try {
      const res = await adminApi.getGenerateStatus(taskId)
      if (res.code === 200) {
        generateTask.value = res.data
        if (res.data.status === 'completed' || res.data.status === 'failed') {
          clearInterval(pollTimer)
          pollTimer = null
          const successCount = (res.data.results || []).filter(r => r.status === 'success').length
          const total = res.data.total || res.data.results?.length || 0
          if (res.data.status === 'completed') {
            ElMessage.success(`生成完成：${successCount}/${total} 个章节成功`)
          }
          handleCourseChange(selectedCourseId.value)
        }
      }
    } catch (error) {
      clearInterval(pollTimer)
      pollTimer = null
    }
  }, 3000)
}

function previewChapter(item) {
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
  color: #303133;
  margin-bottom: 8px;
}

.subtitle {
  color: #909399;
  font-size: 14px;
}

.generate-card,
.progress-card,
.tips-card {
  background: #fff;
  border-radius: 12px;
  padding: 24px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
  margin-bottom: 20px;
}

.select-actions {
  margin-top: 8px;
}

.progress-card h3 {
  font-size: 16px;
  color: #303133;
  margin-bottom: 16px;
}

.progress-detail {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  font-size: 14px;
  color: #606266;
}

.progress-status {
  display: flex;
  align-items: center;
  gap: 4px;
}

.progress-status.processing {
  color: #409eff;
}

.progress-status.completed {
  color: #67c23a;
}

.progress-status.failed {
  color: #f56c6c;
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
  background: #f5f7fa;
  flex-wrap: wrap;
}

.result-item.result-success {
  background: #f0f9eb;
}

.result-item.result-failed {
  background: #fef0f0;
}

.status-icon {
  font-size: 18px;
}

.status-icon.success {
  color: #67c23a;
}

.status-icon.failed {
  color: #f56c6c;
}

.chapter-title {
  flex: 1;
  font-size: 14px;
  color: #303133;
  min-width: 100px;
}

.error-msg {
  font-size: 12px;
  color: #f56c6c;
}

.preview-content {
  max-height: 500px;
  overflow-y: auto;
  padding: 16px;
  background: #fafafa;
  border-radius: 8px;
}

.tips-card h4 {
  font-size: 15px;
  color: #303133;
  margin-bottom: 10px;
}

.tips-card ul {
  padding-left: 20px;
  color: #909399;
  font-size: 13px;
  line-height: 2;
}

@media screen and (max-width: 768px) {
  .content-generate-page {
    padding: 12px;
  }

  .generate-card,
  .progress-card,
  .tips-card {
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
