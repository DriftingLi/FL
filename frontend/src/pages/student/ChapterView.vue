<template>
  <div class="chapter-view-page" v-loading="loading">
    <template v-if="chapterNotFound">
      <el-empty description="章节不存在或已删除">
        <el-button type="primary" @click="goBackToCourse">返回课程</el-button>
      </el-empty>
    </template>

    <template v-else-if="chapterDetail">
      <BreadcrumbNav
        :courseName="courseName"
        :courseId="courseId"
        :chapterName="chapterDetail.title"
      />

      <div class="chapter-header">
        <div class="header-left">
          <h1 class="chapter-title">{{ chapterDetail.title }}</h1>
          <el-tag
            v-if="chapterDetail.study_status === 'completed'"
            type="success"
            size="small"
          >
            已完成
          </el-tag>
        </div>
      </div>

      <div class="chapter-content-area">
        <div v-if="chapterDetail.content" class="content-text markdown-body" v-html="renderedContent"></div>

        <template v-if="chapterFiles.length > 0">
          <div
            v-for="group in fileGroups"
            :key="group.type"
            class="file-section"
          >
            <div class="section-header">
              <el-icon :size="20" class="section-icon" :style="{ color: group.color }">
                <component :is="group.icon" />
              </el-icon>
              <h3 class="section-title">{{ group.label }}</h3>
              <el-tag size="small" type="info">{{ group.files.length }}个文件</el-tag>
            </div>
            <div class="section-content">
              <template v-if="group.type === 'video'">
                <div v-for="file in group.files" :key="file.file_id" class="media-item">
                  <VideoPlayer :src="file.file_url" />
                </div>
              </template>
              <template v-else-if="group.type === 'document'">
                <div v-for="file in group.files" :key="file.file_id" class="media-item">
                  <DocumentViewer :src="file.file_url" :fileName="file.file_name" />
                </div>
              </template>
              <template v-else-if="group.type === 'ppt'">
                <div v-for="file in group.files" :key="file.file_id" class="media-item">
                  <PptViewer :src="file.file_url" :fileName="file.file_name" :chapterId="chapterDetail.chapter_id" />
                </div>
              </template>
              <template v-else-if="group.type === 'image'">
                <div class="image-gallery">
                  <div v-for="file in group.files" :key="file.file_id" class="gallery-item">
                    <ImageViewer :src="file.file_url" :fileName="file.file_name" />
                  </div>
                </div>
              </template>
            </div>
          </div>
        </template>

        <el-empty v-if="!chapterDetail.content && chapterFiles.length === 0" description="该章节暂无内容" />
      </div>

      <div class="study-actions">
        <template v-if="!isStudying">
          <el-button type="primary" @click="beginStudy" size="large">
            开始学习本章
          </el-button>
        </template>
        <template v-else>
          <div class="study-timer">
            <el-icon><Timer /></el-icon>
            已学习: {{ formatStudyTime(studySeconds) }}
          </div>
          <el-button type="success" @click="completeStudy" size="large">
            完成本章学习
          </el-button>
        </template>
      </div>

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
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Timer, ArrowLeft, ArrowRight, VideoCamera, Document, Picture } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { marked } from 'marked'
import { markedHighlight } from 'marked-highlight'
import hljs from 'highlight.js'
import { courseApi } from '@/api/course'
import '@/assets/styles/markdown.css'
import VideoPlayer from '@/components/student/VideoPlayer.vue'
import DocumentViewer from '@/components/student/DocumentViewer.vue'
import PptViewer from '@/components/student/PptViewer.vue'
import ImageViewer from '@/components/student/ImageViewer.vue'
import BreadcrumbNav from '@/components/student/BreadcrumbNav.vue'

marked.use(
  markedHighlight({
    langPrefix: 'hljs language-',
    highlight(code, lang) {
      if (lang && hljs.getLanguage(lang)) {
        return hljs.highlight(code, { language: lang }).value
      }
      return hljs.highlightAuto(code).value
    }
  }),
  { breaks: true, gfm: true }
)

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const chapterNotFound = ref(false)
const chapterDetail = ref(null)
const courseName = ref('')
const chapters = ref([])
const isStudying = ref(false)
const studySeconds = ref(0)
let studyTimer = null
let studyStartTime = null

const courseId = computed(() => route.params.courseId)
const chapterId = computed(() => route.params.chapterId)

const chapterFiles = computed(() => {
  return chapterDetail.value?.files || []
})

const TYPE_ORDER = ['video', 'document', 'ppt', 'image']
const TYPE_CONFIG = {
  video: { label: '视频', icon: VideoCamera, color: '#f56c6c' },
  document: { label: '文档', icon: Document, color: '#409eff' },
  ppt: { label: 'PPT', icon: Document, color: '#e6a23c' },
  image: { label: '图片', icon: Picture, color: '#67c23a' }
}

const fileGroups = computed(() => {
  const groups = {}
  for (const file of chapterFiles.value) {
    const type = file.content_type || 'document'
    if (!groups[type]) {
      groups[type] = []
    }
    groups[type].push(file)
  }
  return TYPE_ORDER
    .filter(type => groups[type] && groups[type].length > 0)
    .map(type => ({
      type,
      label: TYPE_CONFIG[type]?.label || type,
      icon: TYPE_CONFIG[type]?.icon || Document,
      color: TYPE_CONFIG[type]?.color || '#909399',
      files: groups[type]
    }))
})

const renderedContent = computed(() => {
  if (!chapterDetail.value?.content) return ''
  return marked.parse(chapterDetail.value.content)
})

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

function formatStudyTime(seconds) {
  const mins = Math.floor(seconds / 60)
  const secs = seconds % 60
  return `${String(mins).padStart(2, '0')}:${String(secs).padStart(2, '0')}`
}

async function loadChapterDetail() {
  loading.value = true
  chapterNotFound.value = false
  stopStudy()

  try {
    const res = await courseApi.getChapterDetail(courseId.value, chapterId.value)
    if (res.code === 200) {
      chapterDetail.value = res.data
    } else {
      chapterNotFound.value = true
    }
  } catch (error) {
    if (error?.response?.status === 404) {
      chapterNotFound.value = true
    } else {
      console.error('加载章节详情失败:', error)
      ElMessage.error('加载章节详情失败')
    }
  } finally {
    loading.value = false
  }
}

async function loadCourseInfo() {
  try {
    const res = await courseApi.getCourseDetail(courseId.value)
    if (res.code === 200) {
      courseName.value = res.data.course_info?.name || ''
      chapters.value = res.data.chapters || []
    }
  } catch (error) {
    console.error('加载课程信息失败:', error)
  }
}

function beginStudy() {
  isStudying.value = true
  studyStartTime = Date.now()
  studySeconds.value = 0
  studyTimer = setInterval(() => {
    studySeconds.value = Math.floor((Date.now() - studyStartTime) / 1000)
  }, 1000)
}

function stopStudy() {
  if (studyTimer) {
    clearInterval(studyTimer)
    studyTimer = null
  }
  isStudying.value = false
}

async function completeStudy() {
  stopStudy()
  const duration = Math.max(Math.ceil(studySeconds.value / 60), 1)

  try {
    const res = await courseApi.updateProgress(courseId.value, {
      chapter_id: chapterDetail.value.chapter_id,
      duration: duration
    })
    if (res.code === 200) {
      ElMessage.success('学习进度已保存')
      await loadChapterDetail()
    }
  } catch (error) {
    console.error('保存学习进度失败:', error)
    ElMessage.error('保存学习进度失败，请稍后重试')
  }
}

function navigateToChapter(targetChapterId) {
  if (!targetChapterId) return
  router.push({
    name: 'ChapterView',
    params: { courseId: courseId.value, chapterId: targetChapterId }
  })
}

function goBackToCourse() {
  router.push({ name: 'CourseDetail', params: { id: courseId.value } })
}

watch(() => route.params.chapterId, (newVal) => {
  if (newVal) {
    loadChapterDetail()
  }
})

onMounted(() => {
  loadChapterDetail()
  loadCourseInfo()
})

onUnmounted(() => {
  stopStudy()
})
</script>

<style scoped>
.chapter-view-page {
  max-width: 900px;
  margin: 0 auto;
  padding: 0 20px 40px;
}

.chapter-header {
  margin-bottom: 20px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.chapter-title {
  font-size: 22px;
  font-weight: 600;
  color: #303133;
  margin: 0;
}

.chapter-content-area {
  background: #fff;
  border-radius: 12px;
  padding: 24px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
  min-height: 300px;
  margin-bottom: 20px;
}

.content-text {
  line-height: 1.8;
  color: #303133;
  font-size: 15px;
  margin-bottom: 20px;
}

.file-section {
  margin-top: 24px;
  padding-top: 20px;
  border-top: 1px solid #ebeef5;
}

.file-section:first-child {
  margin-top: 0;
  padding-top: 0;
  border-top: none;
}

.section-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
}

.section-icon {
  flex-shrink: 0;
}

.section-title {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
  margin: 0;
}

.section-content {
  width: 100%;
}

.media-item {
  margin-bottom: 20px;
}

.media-item:last-child {
  margin-bottom: 0;
}

.image-gallery {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 16px;
}

.gallery-item {
  border-radius: 8px;
  overflow: hidden;
}

.study-actions {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 20px;
  padding: 20px;
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
  margin-bottom: 20px;
  flex-wrap: wrap;
}

.study-timer {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 18px;
  color: #409eff;
  font-weight: 600;
}

.chapter-navigation {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 0;
  border-top: 1px solid #ebeef5;
}

.nav-prev,
.nav-next {
  max-width: 45%;
}

.nav-prev :deep(.el-button),
.nav-next :deep(.el-button) {
  padding: 12px 16px;
  height: auto;
}

.nav-prev :deep(.el-button) {
  text-align: left;
}

.nav-next :deep(.el-button) {
  text-align: right;
  margin-left: auto;
}

.nav-btn-content {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.nav-label {
  font-size: 12px;
  color: #909399;
}

.nav-title {
  font-size: 14px;
  color: #303133;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 200px;
}

.nav-next .nav-btn-content {
  align-items: flex-end;
}

@media screen and (max-width: 768px) {
  .chapter-view-page {
    padding: 0 12px 30px;
  }

  .chapter-title {
    font-size: 18px;
  }

  .chapter-content-area {
    padding: 16px;
  }

  .image-gallery {
    grid-template-columns: 1fr;
  }

  .study-actions {
    padding: 16px;
    gap: 12px;
  }

  .study-timer {
    font-size: 16px;
  }

  .nav-title {
    max-width: 120px;
  }
}
</style>
