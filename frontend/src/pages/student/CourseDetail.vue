<template>
  <div class="course-detail-page" v-loading="loading">
    <div v-if="courseInfo" class="detail-layout">
      <div class="sidebar" :class="{ 'sidebar-collapsed': isTablet }">
        <ChapterNav
          v-if="!isTablet"
          :chapters="chapters"
          :courseId="courseId"
          :activeChapterId="activeChapterId"
          @select="handleChapterSelect"
        />
        <div v-else class="sidebar-compact">
          <div
            v-for="(chapter, index) in chapters"
            :key="chapter.chapter_id"
            class="compact-item"
            :class="{ 'is-active': activeChapterId === chapter.chapter_id }"
            @click="handleChapterSelect(chapter)"
            :title="chapter.title"
          >
            {{ String(index + 1).padStart(2, '0') }}
          </div>
        </div>
      </div>

      <div class="main-content">
        <div class="course-info-card">
          <div class="cover-wrapper">
            <img
              v-if="courseInfo.cover_image"
              v-lazy="courseInfo.cover_image"
              :alt="courseInfo.name"
              class="cover-image"
              @error="courseInfo.cover_image = ''"
            />
            <div v-if="!courseInfo.cover_image" class="cover-placeholder-large">
              <span>{{ courseInfo.name.charAt(0) }}</span>
            </div>
          </div>

          <div class="info-content">
            <el-tag :type="getCategoryTagType(courseInfo.category)" size="small">
              {{ getCategoryName(courseInfo.category) }}
            </el-tag>
            <h1 class="course-title">{{ courseInfo.name }}</h1>
            <p class="course-description">{{ courseInfo.description || '暂无课程简介' }}</p>

            <div class="progress-section" v-if="progress > 0">
              <div class="progress-header">
                <span>学习进度</span>
                <span class="progress-value">{{ progress }}%</span>
              </div>
              <el-progress :percentage="progress" :stroke-width="10" />
            </div>

            <div class="action-buttons">
              <el-button
                v-if="chapters.length > 0"
                type="primary"
                size="large"
                @click="startLearning"
              >
                开始学习
              </el-button>
              <el-button @click="goBack">返回课程列表</el-button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <el-empty v-else-if="!loading" description="课程不存在或已下架" />

    <div v-if="isMobile && courseInfo" class="mobile-chapter-btn" @click="drawerVisible = true">
      <el-icon><List /></el-icon>
      <span>章节</span>
    </div>

    <el-drawer
      v-model="drawerVisible"
      direction="ltr"
      size="280px"
      :show-close="false"
      class="chapter-drawer"
    >
      <template #header>
        <span class="drawer-title">课程章节</span>
      </template>
      <ChapterNav
        :chapters="chapters"
        :courseId="courseId"
        :activeChapterId="activeChapterId"
        @select="handleChapterSelectFromDrawer"
      />
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, onBeforeUnmount } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { List } from '@element-plus/icons-vue'
import { courseApi } from '@/api/course'
import ChapterNav from '@/components/student/ChapterNav.vue'
import { vLazy } from '@/composables/useLazyLoad'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const courseInfo = ref(null)
const chapters = ref([])
const progress = ref(0)
const activeChapterId = ref(null)
const drawerVisible = ref(false)
const isTablet = ref(false)
const isMobile = ref(false)

const courseId = computed(() => route.params.id)

const categoryMap = {
  'CATEGORY_01': '基础理论',
  'CATEGORY_02': '安全规范',
  'CATEGORY_03': '实操技能',
  'CATEGORY_04': '进阶提升'
}

function getCategoryName(category) {
  return categoryMap[category] || '其他'
}

function getCategoryTagType(category) {
  const types = {
    'CATEGORY_01': '',
    'CATEGORY_02': 'success',
    'CATEGORY_03': 'warning',
    'CATEGORY_04': 'danger'
  }
  return types[category] || 'info'
}

async function loadCourseDetail() {
  loading.value = true
  try {
    const res = await courseApi.getCourseDetail(courseId.value)
    if (res.code === 200) {
      courseInfo.value = res.data.course_info
      chapters.value = res.data.chapters
      progress.value = res.data.progress || 0
    }
  } catch (error) {
    console.error('加载课程详情失败:', error)
    ElMessage.error('加载课程详情失败')
  } finally {
    loading.value = false
  }
}

function handleChapterSelect(chapter) {
  activeChapterId.value = chapter.chapter_id
  router.push({
    name: 'ChapterView',
    params: { courseId: courseId.value, chapterId: chapter.chapter_id }
  })
}

function handleChapterSelectFromDrawer(chapter) {
  drawerVisible.value = false
  handleChapterSelect(chapter)
}

function startLearning() {
  if (chapters.value.length > 0) {
    const firstChapter = chapters.value[0]
    handleChapterSelect(firstChapter)
  }
}

function goBack() {
  router.push('/courses')
}

function checkViewport() {
  const width = window.innerWidth
  isMobile.value = width < 768
  isTablet.value = width >= 768 && width < 1024
}

onMounted(() => {
  loadCourseDetail()
  checkViewport()
  window.addEventListener('resize', checkViewport)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', checkViewport)
})
</script>

<style scoped>
.course-detail-page {
  padding: 20px;
  max-width: 1200px;
  margin: 0 auto;
}

.detail-layout {
  display: flex;
  gap: 24px;
  align-items: flex-start;
}

.sidebar {
  width: 280px;
  flex-shrink: 0;
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
  position: sticky;
  top: 80px;
  max-height: calc(100vh - 100px);
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.sidebar-collapsed {
  width: 60px;
}

.sidebar-compact {
  padding: 8px 4px;
  display: flex;
  flex-direction: column;
  gap: 4px;
  overflow-y: auto;
  max-height: calc(100vh - 100px);
}

.compact-item {
  width: 44px;
  height: 44px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  cursor: pointer;
  font-size: 11px;
  font-weight: 600;
  color: #606266;
  background: #f5f7fa;
  transition: all 0.2s ease;
  margin: 0 auto;
}

.compact-item:hover {
  background: #ecf5ff;
  color: #409eff;
}

.compact-item.is-active {
  background: #409eff;
  color: #fff;
}

.main-content {
  flex: 1;
  min-width: 0;
}

.course-info-card {
  display: flex;
  gap: 30px;
  background: #fff;
  border-radius: 12px;
  padding: 24px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
}

.cover-wrapper {
  flex-shrink: 0;
  width: 300px;
  height: 200px;
  border-radius: 8px;
  overflow: hidden;
}

.cover-image {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.cover-placeholder-large {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.cover-placeholder-large span {
  font-size: 72px;
  color: rgba(255, 255, 255, 0.8);
  font-weight: bold;
}

.info-content {
  flex: 1;
  display: flex;
  flex-direction: column;
}

.course-title {
  font-size: 24px;
  font-weight: 600;
  color: #303133;
  margin: 12px 0;
}

.course-description {
  color: #606266;
  line-height: 1.6;
  flex: 1;
}

.progress-section {
  margin: 16px 0;
}

.progress-header {
  display: flex;
  justify-content: space-between;
  margin-bottom: 8px;
  font-size: 14px;
  color: #606266;
}

.progress-value {
  color: #409eff;
  font-weight: 600;
}

.action-buttons {
  display: flex;
  gap: 12px;
  margin-top: 16px;
  flex-wrap: wrap;
}

.drawer-title {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
}

.chapter-drawer :deep(.el-drawer__header) {
  margin-bottom: 0;
  padding: 16px;
  border-bottom: 1px solid #ebeef5;
}

.chapter-drawer :deep(.el-drawer__body) {
  padding: 0;
}

.mobile-chapter-btn {
  position: fixed;
  bottom: 80px;
  right: 16px;
  width: 52px;
  height: 52px;
  background: #409eff;
  color: #fff;
  border-radius: 50%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  box-shadow: 0 4px 12px rgba(64, 158, 255, 0.4);
  z-index: 10;
  font-size: 11px;
  gap: 2px;
  transition: transform 0.2s ease;
}

.mobile-chapter-btn:active {
  transform: scale(0.95);
}

.mobile-chapter-btn .el-icon {
  font-size: 18px;
}

@media screen and (max-width: 1023px) and (min-width: 768px) {
  .course-detail-page {
    padding: 16px;
  }

  .course-info-card {
    padding: 20px;
    gap: 20px;
  }

  .cover-wrapper {
    width: 220px;
    height: 160px;
  }

  .course-title {
    font-size: 20px;
  }
}

@media screen and (max-width: 767px) {
  .course-detail-page {
    padding: 12px;
  }

  .detail-layout {
    flex-direction: column;
    gap: 16px;
  }

  .sidebar {
    display: none;
  }

  .course-info-card {
    flex-direction: column;
    gap: 16px;
    padding: 16px;
  }

  .cover-wrapper {
    width: 100%;
    height: 180px;
  }

  .course-title {
    font-size: 20px;
    margin: 8px 0;
  }

  .action-buttons {
    gap: 8px;
  }

  .action-buttons .el-button {
    flex: 1;
    min-width: 0;
  }
}

@media screen and (max-width: 480px) {
  .course-title {
    font-size: 18px;
  }

  .cover-wrapper {
    height: 150px;
  }
}
</style>
