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
      <div v-if="directions.length > 0" class="direction-pills">
        <span class="filter-label">专业方向</span>
        <button
          v-for="d in directions"
          :key="d.specialty_id"
          class="pill"
          :class="{ active: currentDirectionId === d.specialty_id }"
          @click="selectDirection(d.specialty_id)"
        >
          {{ d.name }}
        </button>
      </div>
      <div v-if="currentDirectionId !== null && currentLevels.length > 0" class="direction-pills">
        <span class="filter-label">课程等级</span>
        <button
          v-for="l in currentLevels"
          :key="l.level_id"
          class="pill level-pill"
          :class="{ active: currentLevelId === l.level_id }"
          @click="currentLevelId = l.level_id; currentPage = 1; loadCourses()"
        >
          {{ l.name }}
        </button>
      </div>
    </div>

    <div class="course-content" v-loading="loading">
      <div v-if="courses.length > 0" class="course-grid">
        <div
          v-for="course in courses"
          :key="course.course_id"
          class="course-card"
          @click="openDetail(course)"
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
            <span v-if="levelNameOf(course.level_id)" class="level-tag">{{ levelNameOf(course.level_id) }}</span>
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
              <span class="meta-item" v-if="course.theory_hours || course.practice_hours">
                <el-icon><Clock /></el-icon>
                理论{{ course.theory_hours || 0 }}学时 · 实操{{ course.practice_hours || 0 }}学时
              </span>
            </div>
            <div class="card-action">
              <span class="action-text">查看详情</span>
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

    <!-- 课程详情（学时/前置/证书/章节） -->
    <el-dialog
      v-model="detailVisible"
      :title="detailCourse?.name || '课程详情'"
      width="680px"
      class="course-detail-dialog"
      destroy-on-close
    >
      <div v-loading="detailLoading">
        <template v-if="detailCourse">
          <div class="detail-brief">
            <el-tag v-if="detailCourse.level?.name" type="warning">{{ detailCourse.level.name }}</el-tag>
            <el-tag v-if="detailCourse.specialty?.name" type="primary" effect="plain">{{ detailCourse.specialty.name }}</el-tag>
            <el-tag v-if="detailCourse.category" effect="plain">{{ getCategoryName(detailCourse.category) }}</el-tag>
          </div>
          <p class="detail-desc">{{ detailCourse.description || '暂无简介' }}</p>

          <el-descriptions :column="2" border size="small" class="detail-descriptions">
            <el-descriptions-item label="理论学时">{{ detailCourse.theory_hours ?? '-' }} 学时</el-descriptions-item>
            <el-descriptions-item label="实操学时">{{ detailCourse.practice_hours ?? '-' }} 学时</el-descriptions-item>
            <el-descriptions-item label="章节数">{{ detailCourse.chapter_count ?? '-' }}</el-descriptions-item>
            <el-descriptions-item label="课程时长">{{ detailCourse.duration ? formatDuration(detailCourse.duration) : '-' }}</el-descriptions-item>
            <el-descriptions-item label="关联证书">
              <template v-if="detailCourse.certificate_template">
                {{ detailCourse.certificate_template.name }}
                <span v-if="detailCourse.certificate_template.validity_days" class="cert-valid">
                  （有效期 {{ detailCourse.certificate_template.validity_days }} 天）
                </span>
              </template>
              <span v-else>—</span>
            </el-descriptions-item>
            <el-descriptions-item label="前置课程">
              <template v-if="detailCourse.prerequisites && detailCourse.prerequisites.length > 0">
                <el-tag
                  v-for="p in detailCourse.prerequisites"
                  :key="p.course_id"
                  size="small"
                  type="info"
                  class="prereq-tag"
                >{{ p.name }}</el-tag>
              </template>
              <span v-else>—</span>
            </el-descriptions-item>
          </el-descriptions>

          <div class="chapter-section">
            <h4>章节内容（{{ detailChapters.length }}）</h4>
            <div v-if="detailChapters.length > 0" class="chapter-list">
              <div
                v-for="(ch, i) in detailChapters"
                :key="ch.chapter_id"
                class="chapter-row"
                @click="goToChapter(ch)"
              >
                <span class="chapter-index">{{ i + 1 }}</span>
                <span class="chapter-title">{{ ch.title }}</span>
                <span v-if="ch.duration" class="chapter-duration">{{ ch.duration }}分钟</span>
                <el-icon class="chapter-arrow"><ArrowRight /></el-icon>
              </div>
            </div>
            <el-empty v-else description="该课程暂无章节内容" :image-size="60" />
          </div>
        </template>
      </div>
      <template #footer>
        <el-button @click="detailVisible = false">关闭</el-button>
        <el-button
          v-if="detailChapters.length > 0"
          type="primary"
          @click="goToChapter(detailChapters[0])"
        >开始学习</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Document, Timer, Clock, ArrowRight } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { courseApi, type CourseDetail, type CourseSummary } from '@/api/course'
import { trainingApi } from '@/api/training'
import { vLazy } from '@/composables/useLazyLoad'

const router = useRouter()

const loading = ref(false)
const courses = ref<CourseSummary[]>([])
const currentCategory = ref('')
const currentPage = ref(1)
const pageSize = ref(12)
const total = ref(0)

const directions = ref<{ specialty_id: number; name: string; levels?: { level_id: number; name: string }[] }[]>([])
const currentDirectionId = ref<number | null>(null)
const currentLevelId = ref<number | null>(null)

const currentLevels = computed(() => {
  const d = directions.value.find(dir => dir.specialty_id === currentDirectionId.value)
  return d?.levels || []
})

function levelNameOf(levelId?: number | null) {
  if (!levelId) return ''
  for (const d of directions.value) {
    const l = (d.levels || []).find(item => item.level_id === levelId)
    if (l) return l.name
  }
  return ''
}

const categories = [
  { value: '', label: '全部' },
  { value: 'CATEGORY_01', label: '基础理论' },
  { value: 'CATEGORY_02', label: '安全规范' },
  { value: 'CATEGORY_03', label: '实操技能' },
  { value: 'CATEGORY_04', label: '进阶提升' }
]

const categoryMap: Record<string, string> = {
  'CATEGORY_01': '基础理论',
  'CATEGORY_02': '安全规范',
  'CATEGORY_03': '实操技能',
  'CATEGORY_04': '进阶提升'
}

const categoryClassMap: Record<string, string> = {
  'CATEGORY_01': 'cat-theory',
  'CATEGORY_02': 'cat-safety',
  'CATEGORY_03': 'cat-practice',
  'CATEGORY_04': 'cat-advanced'
}

function getCategoryName(category?: string) {
  return category ? categoryMap[category] || '其他' : '其他'
}

function getCategoryClass(category?: string) {
  return category ? categoryClassMap[category] || 'cat-default' : 'cat-default'
}

function formatDuration(minutes: number) {
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
    const params: Record<string, any> = {
      page: currentPage.value,
      page_size: pageSize.value
    }
    if (currentCategory.value) {
      params.category = currentCategory.value
    }
    if (currentDirectionId.value !== null) {
      params.specialty_id = currentDirectionId.value
    }
    if (currentLevelId.value !== null) {
      params.level_id = currentLevelId.value
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

async function loadDirections() {
  try {
    const res = await trainingApi.getCatalogTree()
    if (res.code === 200) {
      directions.value = res.data.specialties || []
    }
  } catch (error) {
    console.error('加载目录失败:', error)
  }
}

function selectDirection(directionId: number) {
  currentDirectionId.value = currentDirectionId.value === directionId ? null : directionId
  currentLevelId.value = null
  currentPage.value = 1
  loadCourses()
}

function handlePageChange() {
  loadCourses()
}

function handleSizeChange() {
  currentPage.value = 1
  loadCourses()
}

// ===== 课程详情 =====
const detailVisible = ref(false)
const detailLoading = ref(false)
const detailCourse = ref<CourseDetail | null>(null)
const detailChapters = ref<{ chapter_id: number; title: string; duration?: number }[]>([])

async function openDetail(course: CourseSummary) {
  detailCourse.value = course
  detailVisible.value = true
  detailLoading.value = true
  detailChapters.value = []
  try {
    const res = await courseApi.getCourseDetail(course.course_id)
    if (res.code === 200) {
      detailCourse.value = { ...course, ...(res.data.course_info || {}) }
      detailChapters.value = res.data.chapters || []
    }
  } catch (error) {
    console.error('加载课程详情失败:', error)
    ElMessage.error('加载课程详情失败，请稍后重试')
  } finally {
    detailLoading.value = false
  }
}

function goToChapter(ch: { chapter_id: number }) {
  if (!detailCourse.value) return
  detailVisible.value = false
  router.push({
    name: 'ChapterView',
    params: { courseId: detailCourse.value.course_id, chapterId: ch.chapter_id }
  })
}

onMounted(() => {
  loadCourses()
  loadDirections()
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
  flex-direction: column;
  align-items: center;
  gap: var(--space-2);
}

.category-pills {
  display: flex;
  gap: var(--space-2);
  flex-wrap: wrap;
  justify-content: center;
}

.direction-pills {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  flex-wrap: wrap;
  justify-content: center;
}

.filter-label {
  font-size: var(--text-xs);
  color: var(--color-text-tertiary);
  font-weight: var(--font-medium);
  margin-right: var(--space-1);
}

.direction-pills .pill {
  border-color: var(--color-primary-200);
  background: var(--color-primary-50);
}

.level-pill {
  font-size: var(--text-xs);
  padding: var(--space-1) var(--space-3);
  border-radius: var(--radius-sm);
}

.level-tag {
  position: absolute;
  top: 8px;
  left: 8px;
  padding: 2px 10px;
  border-radius: var(--radius-full);
  background: rgba(230, 162, 60, 0.92);
  color: #fff;
  font-size: 12px;
  font-weight: 600;
}

.cert-valid {
  color: var(--color-text-tertiary);
  font-size: 12px;
}

.prereq-tag {
  margin: 2px 4px 2px 0;
}

.chapter-section {
  margin-top: var(--space-5);
}

.chapter-section h4 {
  font-size: var(--text-base);
  font-weight: var(--font-semibold);
  color: var(--color-text-primary);
  margin-bottom: var(--space-3);
}

.chapter-list {
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-lg);
  overflow: hidden;
}

.chapter-row {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-3) var(--space-4);
  border-bottom: 1px solid var(--color-border-light);
  cursor: pointer;
  transition: background var(--duration-fast) var(--ease-default);
}

.chapter-row:last-child {
  border-bottom: none;
}

.chapter-row:hover {
  background: var(--color-bg-muted);
}

.chapter-index {
  width: 24px;
  height: 24px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--color-primary-100);
  color: var(--color-primary-600);
  border-radius: var(--radius-sm);
  font-size: 12px;
  font-weight: 600;
}

.chapter-title {
  flex: 1;
  font-size: var(--text-sm);
  color: var(--color-text-primary);
}

.chapter-duration {
  font-size: var(--text-xs);
  color: var(--color-text-tertiary);
}

.chapter-arrow {
  color: var(--color-text-tertiary);
}

.detail-brief {
  display: flex;
  gap: var(--space-2);
  margin-bottom: var(--space-3);
  flex-wrap: wrap;
}

.detail-desc {
  font-size: var(--text-sm);
  color: var(--color-text-secondary);
  margin-bottom: var(--space-4);
}

.detail-descriptions {
  margin-bottom: var(--space-4);
}

@media (max-width: 768px) {
  .detail-descriptions :deep(.el-descriptions__body .el-descriptions__table) {
    display: block;
  }
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
