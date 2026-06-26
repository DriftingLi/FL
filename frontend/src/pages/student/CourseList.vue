<template>
  <div class="course-list-page">
    <div class="page-header">
      <h1 class="page-title">课程中心</h1>
      <p class="page-subtitle">选择您感兴趣的课程，开始学习叉车维修知识</p>
    </div>

    <div class="filter-bar">
      <div class="category-pills">
        <button
          v-for="cat in categories"
          :key="cat.value"
          class="pill"
          :class="{ active: currentCategory === cat.value }"
          @click="currentCategory = cat.value; currentPage = 1; loadCourses()"
        >
          {{ cat.label }}
        </button>
      </div>
    </div>

    <div class="course-content" v-loading="loading">
      <div v-if="courses.length > 0" class="course-grid">
        <div
          v-for="course in courses"
          :key="course.course_id"
          class="course-card"
          @click="goToDetail(course.course_id)"
        >
          <div class="card-cover" :class="getCategoryClass(course.category)">
            <img
              v-if="course.cover_image"
              v-lazy="course.cover_image"
              :alt="course.name"
              @error="course.cover_image = ''"
            />
            <div v-if="!course.cover_image" class="cover-placeholder">
              <span>{{ course.name.charAt(0) }}</span>
            </div>
            <span class="category-tag">{{ getCategoryName(course.category) }}</span>
          </div>
          <div class="card-body">
            <h3 class="course-name">{{ course.name }}</h3>
            <p class="course-desc">{{ course.description || '暂无简介' }}</p>
            <div class="course-meta">
              <span class="meta-item">
                <el-icon><Document /></el-icon>
                {{ course.chapter_count }} 章节
              </span>
              <span class="meta-item" v-if="course.duration">
                <el-icon><Timer /></el-icon>
                {{ formatDuration(course.duration) }}
              </span>
            </div>
            <div class="card-action">
              <span class="action-text">开始学习</span>
              <el-icon><ArrowRight /></el-icon>
            </div>
          </div>
        </div>
      </div>

      <div v-else-if="!loading" class="empty-state">
        <div class="empty-icon">
          <svg width="80" height="80" viewBox="0 0 80 80" fill="none">
            <rect x="10" y="16" width="60" height="48" rx="8" stroke="var(--color-border)" stroke-width="2" fill="none"/>
            <path d="M28 32H52" stroke="var(--color-border)" stroke-width="2" stroke-linecap="round"/>
            <path d="M28 40H44" stroke="var(--color-border)" stroke-width="2" stroke-linecap="round"/>
            <path d="M28 48H38" stroke="var(--color-border)" stroke-width="2" stroke-linecap="round"/>
          </svg>
        </div>
        <p class="empty-title">暂无课程数据</p>
        <p class="empty-desc">请稍后再来查看</p>
      </div>
    </div>

    <div class="pagination-wrapper" v-if="total > pageSize">
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="pageSize"
        :total="total"
        :page-sizes="[12, 24, 36]"
        layout="total, sizes, prev, pager, next, jumper"
        @size-change="handleSizeChange"
        @current-change="handlePageChange"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Document, Timer, ArrowRight } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { courseApi } from '@/api/course'
import { vLazy } from '@/composables/useLazyLoad'

const router = useRouter()

const loading = ref(false)
const courses = ref([])
const currentCategory = ref('')
const currentPage = ref(1)
const pageSize = ref(12)
const total = ref(0)

const categories = [
  { value: '', label: '全部' },
  { value: 'CATEGORY_01', label: '基础理论' },
  { value: 'CATEGORY_02', label: '安全规范' },
  { value: 'CATEGORY_03', label: '实操技能' },
  { value: 'CATEGORY_04', label: '进阶提升' }
]

const categoryMap = {
  'CATEGORY_01': '基础理论',
  'CATEGORY_02': '安全规范',
  'CATEGORY_03': '实操技能',
  'CATEGORY_04': '进阶提升'
}

const categoryClassMap = {
  'CATEGORY_01': 'cat-theory',
  'CATEGORY_02': 'cat-safety',
  'CATEGORY_03': 'cat-practice',
  'CATEGORY_04': 'cat-advanced'
}

function getCategoryName(category) {
  return categoryMap[category] || '其他'
}

function getCategoryClass(category) {
  return categoryClassMap[category] || 'cat-default'
}

function formatDuration(minutes) {
  if (minutes < 60) {
    return `${minutes}分钟`
  }
  const hours = Math.floor(minutes / 60)
  const mins = minutes % 60
  return mins > 0 ? `${hours}小时${mins}分钟` : `${hours}小时`
}

async function loadCourses() {
  loading.value = true
  try {
    const params = {
      page: currentPage.value,
      page_size: pageSize.value
    }
    if (currentCategory.value) {
      params.category = currentCategory.value
    }
    const res = await courseApi.getCourses(params)
    if (res.code === 200) {
      courses.value = res.data.courses
      total.value = res.data.total
    }
  } catch (error) {
    console.error('加载课程失败:', error)
    ElMessage.error('加载课程失败，请稍后重试')
  } finally {
    loading.value = false
  }
}

function handlePageChange() {
  loadCourses()
}

function handleSizeChange() {
  currentPage.value = 1
  loadCourses()
}

function goToDetail(courseId) {
  router.push(`/course/${courseId}`)
}

onMounted(() => {
  loadCourses()
})
</script>

<style scoped>
.course-list-page {
  display: flex;
  flex-direction: column;
  gap: var(--space-6);
}

.page-header {
  text-align: center;
}

.page-title {
  font-family: var(--font-display);
  font-size: var(--text-3xl);
  font-weight: var(--font-bold);
  color: var(--color-text-primary);
  margin-bottom: var(--space-2);
}

.page-subtitle {
  font-size: var(--text-base);
  color: var(--color-text-tertiary);
}

.filter-bar {
  display: flex;
  justify-content: center;
}

.category-pills {
  display: flex;
  gap: var(--space-2);
  flex-wrap: wrap;
  justify-content: center;
}

.pill {
  padding: var(--space-2) var(--space-5);
  border-radius: var(--radius-full);
  border: 1.5px solid var(--color-border);
  background: var(--color-bg-card);
  color: var(--color-text-secondary);
  font-size: var(--text-sm);
  font-weight: var(--font-medium);
  cursor: pointer;
  transition: all var(--duration-fast) var(--ease-default);
  font-family: var(--font-body);
}

.pill:hover {
  border-color: var(--color-primary-300);
  color: var(--color-primary-500);
}

.pill.active {
  border-color: var(--color-primary-500);
  background: var(--color-primary-500);
  color: white;
}

.course-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: var(--space-5);
}

.course-card {
  background: var(--color-bg-card);
  border-radius: var(--radius-xl);
  overflow: hidden;
  cursor: pointer;
  box-shadow: var(--shadow-xs);
  border: 1px solid var(--color-border-light);
  transition: all var(--duration-normal) var(--ease-default);
}

.course-card:hover {
  box-shadow: var(--shadow-lg);
  transform: translateY(-4px);
}

.course-card:hover .card-cover img {
  transform: scale(1.05);
}

.course-card:hover .card-action {
  opacity: 1;
  transform: translateY(0);
}

.card-cover {
  position: relative;
  height: 180px;
  overflow: hidden;
}

.card-cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  transition: transform var(--duration-slow) var(--ease-default);
}

.cover-placeholder {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}

.cover-placeholder span {
  font-size: 56px;
  color: rgba(255, 255, 255, 0.8);
  font-weight: var(--font-bold);
  font-family: var(--font-display);
}

.cat-theory .cover-placeholder,
.cat-theory:not(:has(img)) {
  background: linear-gradient(135deg, #1E40AF 0%, #3B82F6 100%);
}

.cat-safety .cover-placeholder,
.cat-safety:not(:has(img)) {
  background: linear-gradient(135deg, #047857 0%, #10B981 100%);
}

.cat-practice .cover-placeholder,
.cat-practice:not(:has(img)) {
  background: linear-gradient(135deg, #B45309 0%, #F59E0B 100%);
}

.cat-advanced .cover-placeholder,
.cat-advanced:not(:has(img)) {
  background: linear-gradient(135deg, #7C3AED 0%, #A78BFA 100%);
}

.cat-default .cover-placeholder,
.cat-default:not(:has(img)) {
  background: var(--gradient-brand);
}

.category-tag {
  position: absolute;
  top: var(--space-3);
  right: var(--space-3);
  padding: var(--space-1) var(--space-3);
  background: rgba(255, 255, 255, 0.9);
  backdrop-filter: blur(8px);
  border-radius: var(--radius-full);
  font-size: var(--text-xs);
  font-weight: var(--font-semibold);
  color: var(--color-primary-600);
}

.card-body {
  padding: var(--space-5);
}

.course-name {
  font-size: var(--text-lg);
  font-weight: var(--font-semibold);
  color: var(--color-text-primary);
  margin-bottom: var(--space-2);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.course-desc {
  font-size: var(--text-sm);
  color: var(--color-text-tertiary);
  margin-bottom: var(--space-3);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  line-height: var(--leading-relaxed);
  min-height: 42px;
}

.course-meta {
  display: flex;
  gap: var(--space-4);
  padding-top: var(--space-3);
  border-top: 1px solid var(--color-border-light);
  margin-bottom: var(--space-3);
}

.meta-item {
  display: flex;
  align-items: center;
  gap: var(--space-1);
  font-size: var(--text-xs);
  color: var(--color-text-tertiary);
}

.card-action {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: var(--space-1);
  opacity: 0;
  transform: translateY(4px);
  transition: all var(--duration-fast) var(--ease-default);
}

.action-text {
  font-size: var(--text-sm);
  font-weight: var(--font-semibold);
  color: var(--color-primary-500);
}

.card-action .el-icon {
  font-size: 14px;
  color: var(--color-primary-500);
}

.empty-state {
  text-align: center;
  padding: var(--space-16) 0;
}

.empty-icon {
  margin-bottom: var(--space-4);
}

.empty-title {
  font-size: var(--text-lg);
  font-weight: var(--font-semibold);
  color: var(--color-text-secondary);
  margin-bottom: var(--space-2);
}

.empty-desc {
  font-size: var(--text-sm);
  color: var(--color-text-tertiary);
}

.pagination-wrapper {
  display: flex;
  justify-content: center;
  padding-top: var(--space-4);
}

@media screen and (max-width: 768px) {
  .page-title {
    font-size: var(--text-2xl);
  }

  .category-pills {
    justify-content: flex-start;
    overflow-x: auto;
    flex-wrap: nowrap;
    padding-bottom: var(--space-2);
    -webkit-overflow-scrolling: touch;
  }

  .pill {
    flex-shrink: 0;
  }

  .course-grid {
    grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
    gap: var(--space-4);
  }

  .card-cover {
    height: 150px;
  }
}

@media screen and (max-width: 480px) {
  .course-grid {
    grid-template-columns: 1fr;
    gap: var(--space-3);
  }

  .card-cover {
    height: 140px;
  }

  .page-title {
    font-size: var(--text-xl);
  }
}
</style>
