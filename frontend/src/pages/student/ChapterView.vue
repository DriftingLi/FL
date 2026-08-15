<template>
  <div class="chapter-view-page" v-loading="loading">
    <template v-if="chapterNotFound">
      <el-empty description="章节不存在或已删除">
        <el-button type="primary" @click="goBackToCourse">返回课程</el-button>
      </el-empty>
    </template>

    <template v-else-if="chapterDetail">
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
        <el-tabs v-if="chapterDetail.content || chapterFiles.length > 0" v-model="activeTab" class="content-tabs">
          <el-tab-pane
            v-if="chapterDetail.content"
            label="图文"
            name="content"
          >
            <div class="content-text markdown-body" v-html="renderedContent"></div>
          </el-tab-pane>

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
          </el-tab-pane>
        </el-tabs>

        <el-empty v-if="!chapterDetail.content && chapterFiles.length === 0" description="该章节暂无内容" />
      </div>

      <ChapterDiscussion v-if="chapterDetail.chapter_id" :chapter-id="chapterDetail.chapter_id" />

      <div class="chapter-navigation">
        <div class="nav-prev">
          <el-button
            :disabled="!chapterDetail.previous_chapter_id"
            @click="navigateToChapter(chapterDetail.previous_chapter_id ?? 0)"
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
            @click="navigateToChapter(chapterDetail.next_chapter_id ?? 0)"
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
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, ArrowRight, VideoCamera, Document, Picture } from '@element-plus/icons-vue'
import { marked } from 'marked'
import { markedHighlight } from 'marked-highlight'
import hljs from 'highlight.js'
import { courseApi, type ChapterDetail } from '@/api/course'
import { useCourseStore } from '@/stores/course'
import { useStudyTracker } from '@/composables/useStudyTracker'
import '@/assets/styles/markdown.css'
import VideoPlayer from '@/components/student/VideoPlayer.vue'
import DocumentViewer from '@/components/student/DocumentViewer.vue'
import PptViewer from '@/components/student/PptViewer.vue'
import ImageViewer from '@/components/student/ImageViewer.vue'
import ChapterDiscussion from '@/components/student/ChapterDiscussion.vue'

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
const courseStore = useCourseStore()

interface ChapterItem {
  chapter_id: number
  title: string
}

const loading = ref(false)
const chapterNotFound = ref(false)
const chapterDetail = ref<ChapterDetail | null>(null)
const courseName = ref('')
const chapters = ref<ChapterItem[]>([])
const activeTab = ref('')

const courseId = computed(() => route.params.courseId as string)
const chapterId = computed(() => route.params.chapterId)

// 学习时长上报收敛到 useStudyTracker：页面只注入 loadDetail / reportDuration 两个 adapter，
// 60s 阈值、取整、visibility 暂停恢复、切章/卸载追报的时序均由 composable 吸收。
// reportDuration 返回「后端已确认上报的累计秒数」，据此推进上报游标（detail===null 不再脆弱判据）。
let confirmedTotalSeconds = 0
const studyTracker = useStudyTracker({
  loadDetail: () => {
    if (!chapterDetail.value?.chapter_id || !courseId.value) return null
    return { courseId: Number(courseId.value), chapterId: chapterDetail.value.chapter_id }
  },
  reportDuration: async (incrementalSeconds) => {
    // 增量秒数按现状规则取整为分钟上报（不足 1 分钟按 1 分钟）
    const durationMinutes = Math.max(Math.ceil(incrementalSeconds / 60), 1)
    try {
      // 拦截器已解包信封；detail 恒为 null（无业务负载），确认信号是「不抛错」而非 detail 值
      const detail = await courseApi.updateProgress(Number(courseId.value), {
        chapter_id: chapterDetail.value!.chapter_id,
        duration: durationMinutes
      })
      void detail // detail 不承载确认计数；视为 0 额外，整段增量计入已确认累计
      confirmedTotalSeconds += incrementalSeconds
    } catch (error) {
      // 上报失败不推进游标（非最终追报时拦截器已提示）
      console.warn('上报学习时长增量失败:', error)
    }
    return confirmedTotalSeconds
  }
})

const chapterFiles = computed(() => {
  return chapterDetail.value?.files || []
})

const TYPE_ORDER = ['video', 'document', 'ppt', 'image']
const TYPE_CONFIG: Record<string, { label: string; icon: any; color: string }> = {
  video: { label: '视频', icon: VideoCamera, color: '#f56c6c' },
  document: { label: '文档', icon: Document, color: '#409eff' },
  ppt: { label: 'PPT', icon: Document, color: '#e6a23c' },
  image: { label: '图片', icon: Picture, color: '#67c23a' }
}

const fileGroups = computed(() => {
  const groups: Record<string, any[]> = {}
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

// 计算默认激活的 Tab：优先图文，其次按 TYPE_ORDER 第一个有内容的媒体类型
const defaultTab = computed(() => {
  if (chapterDetail.value?.content) return 'content'
  const firstGroup = fileGroups.value[0]
  return firstGroup ? firstGroup.type : ''
})

watch(
  defaultTab,
  (newTab) => {
    if (newTab && newTab !== activeTab.value) activeTab.value = newTab
  },
  { immediate: true }
)

const getPrevChapterTitle = computed(() => {
  const detail = chapterDetail.value
  if (!detail?.previous_chapter_id) return ''
  const prev = chapters.value.find(c => c.chapter_id === detail.previous_chapter_id)
  return prev ? prev.title : ''
})

const getNextChapterTitle = computed(() => {
  const detail = chapterDetail.value
  if (!detail?.next_chapter_id) return ''
  const next = chapters.value.find(c => c.chapter_id === detail.next_chapter_id)
  return next ? next.title : ''
})

async function loadChapterDetail() {
  loading.value = true
  chapterNotFound.value = false
  // 切换章节前先上报当前章节的增量时长，再停表（先报增量再停表）
  await studyTracker.reportIncremental(false)
  studyTracker.stop()

  try {
    // 拦截器已解包信封；章节不存在由后端 404 触发 catch 分支
    const detail = await courseApi.getChapterDetail(Number(courseId.value), Number(chapterId.value))
    chapterDetail.value = detail
    // 章节加载成功后启动学习计时
    studyTracker.begin()
  } catch (error) {
    const err = error as { response?: { status?: number } }
    if (err?.response?.status === 404) {
      chapterNotFound.value = true
    } else {
      console.error('加载章节详情失败:', error)
      /* 错误已由拦截器提示 */
    }
  } finally {
    loading.value = false
  }
}

async function loadCourseInfo() {
  // 复用侧栏章节模式已加载的课程数据，避免重复请求；未命中时由 store 发起请求
  try {
    await courseStore.loadCourse(courseId.value)
    courseName.value = courseStore.courseInfo?.name || ''
    chapters.value = courseStore.chapters || []
  } catch (error) {
    console.error('加载课程信息失败:', error)
  }
}

function navigateToChapter(targetChapterId: string | number) {
  if (!targetChapterId) return
  router.push({
    name: 'ChapterView',
    params: { courseId: courseId.value, chapterId: targetChapterId }
  })
}

function goBackToCourse() {
  router.push({ name: 'CourseList' })
}

watch(() => route.params.chapterId, (newVal) => {
  if (newVal) {
    loadChapterDetail()
  }
})

// 页面可见性变化：切到后台暂停计时并上报，回到前台恢复（时序收敛进 composable）
function handleVisibilityChange() {
  if (document.hidden) {
    studyTracker.pause()
  } else {
    // 回到前台：仅在仍有有效章节且在学习状态时恢复计时
    if (chapterDetail.value) studyTracker.resume()
  }
}

// 页面关闭/刷新时追报未提交时长
function handleBeforeUnload() {
  if (!studyTracker.isStudying.value) return
  // keepalive 上报：composable 内部按 final 语义强制报满
  void studyTracker.reportIncremental(true)
}

onMounted(() => {
  loadChapterDetail()
  loadCourseInfo()
  document.addEventListener('visibilitychange', handleVisibilityChange)
  window.addEventListener('beforeunload', handleBeforeUnload)
})

onBeforeUnmount(() => {
  // 离开组件时使用 fetch keepalive 上报未提交时长，再停表
  void studyTracker.reportIncremental(true)
  studyTracker.stop()
  document.removeEventListener('visibilitychange', handleVisibilityChange)
  window.removeEventListener('beforeunload', handleBeforeUnload)
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

.content-tabs {
  min-height: 400px;
}

.content-tabs :deep(.el-tabs__header) {
  margin-bottom: 20px;
  position: sticky;
  top: 0;
  z-index: 10;
  background: #fff;
  padding-top: 8px;
}

.content-tabs :deep(.el-tabs__nav-wrap)::after {
  height: 1px;
}

.tab-label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.tab-count {
  margin-left: 2px;
  transform: scale(0.85);
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

  .nav-title {
    max-width: 120px;
  }
}
</style>
