<template>
  <div class="tutor-courses-page">
    <div class="page-header">
      <h2>我的课程</h2>
    </div>

    <div class="course-layout">
      <!-- 左栏：双卡片（专业方向 / 课程等级），形式与学员端课程中心一致 -->
      <aside class="cc-sidebar">
        <div class="cc-filter-card">
          <div class="cc-card-title">专业方向</div>
          <button
            class="cc-nav-item"
            :class="{ active: specialtyId === null }"
            @click="selectSpecialty(null)"
          >
            <span class="cc-nav-name">全部课程</span>
            <span class="cc-nav-count">{{ totalAll }}</span>
          </button>
          <button
            v-for="d in directions"
            :key="d.specialty_id"
            class="cc-nav-item"
            :class="{ active: specialtyId === d.specialty_id }"
            @click="selectSpecialty(d.specialty_id)"
          >
            <span class="cc-nav-name">{{ d.name }}</span>
            <span class="cc-nav-count">{{ directionCount(d) }}</span>
          </button>
        </div>

        <div class="cc-filter-card">
          <div class="cc-card-title">课程等级</div>
          <button
            class="cc-nav-item"
            :class="{ active: levelId === null }"
            @click="selectLevel(null)"
          >
            <span class="cc-nav-name">全部等级</span>
            <span class="cc-nav-count">{{ scopedTotal }}</span>
          </button>
          <button
            v-for="l in levels"
            :key="l.level_id"
            class="cc-nav-item"
            :class="{ active: levelId === l.level_id }"
            @click="selectLevel(l.level_id)"
          >
            <span class="cc-nav-name">{{ l.name }}</span>
            <span class="cc-nav-count">{{ countOfLevel(l.level_id) }}</span>
          </button>
        </div>
      </aside>

      <!-- 右栏：课程网格 -->
      <main class="cc-main">
        <div v-loading="loading" class="course-grid">
          <el-empty v-if="!loading && courses.length === 0" description="该方向/等级下暂无课程" />

          <div
            v-for="course in courses"
            :key="course.course_id"
            class="course-card"
            @click="goToChapters(course.course_id)"
          >
            <div class="card-cover" :class="coverClass(course.specialty_id)">
              <img v-if="course.cover_image" :src="course.cover_image" :alt="course.name" class="cover-img" />
              <div v-else class="cover-placeholder">
                <span>{{ course.name.charAt(0) }}</span>
              </div>
            </div>
            <div class="card-body">
              <div class="card-tags">
                <el-tag v-if="levelNameOf(course.level_id)" :type="levelTagType(levelNameOf(course.level_id))" size="small">
                  {{ levelNameOf(course.level_id) }}
                </el-tag>
                <el-tag v-if="specialtyNameOf(course.specialty_id)" type="primary" effect="plain" size="small">
                  {{ specialtyNameOf(course.specialty_id) }}
                </el-tag>
              </div>
              <h3 class="card-title">{{ course.name }}</h3>
              <p class="card-desc">{{ course.description || '暂无简介' }}</p>
              <div class="card-footer">
                <span>{{ course.chapter_count || 0 }} 个章节</span>
                <el-button type="primary" size="small">
                  管理章节 <el-icon><ArrowRight /></el-icon>
                </el-button>
              </div>
            </div>
          </div>
        </div>

        <div v-if="total > pageSize" class="pagination-wrapper">
          <el-pagination
            v-model:current-page="currentPage"
            :page-size="pageSize"
            :total="total"
            layout="prev, pager, next"
            @current-change="loadCourses"
          />
        </div>
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowRight } from '@element-plus/icons-vue'
import { tutorApi, type TutorCourse } from '@/api/tutor'
import { trainingApi, type CatalogDirectionNode, type CatalogLevel } from '@/api/training'
import { levelTagType } from '@/constants/level'

const router = useRouter()
const loading = ref(false)
const courses = ref<TutorCourse[]>([])
const currentPage = ref(1)
const pageSize = ref(12)
const total = ref(0)

const directions = ref<CatalogDirectionNode[]>([])
const levels = ref<CatalogLevel[]>([])
const specialtyId = ref<number | null>(null)
const levelId = ref<number | null>(null)

const totalAll = computed(() =>
  directions.value.reduce((sum, d) => sum + directionCount(d), 0)
)

function directionCount(d: CatalogDirectionNode): number {
  return (d.levels || []).reduce((sum, lv) => sum + (lv.courses?.length || 0), 0)
}

// 当前方向筛选范围内的课程总数（等级卡片计数随方向筛选联动）
const scopedTotal = computed(() => {
  const dirs = specialtyId.value === null
    ? directions.value
    : directions.value.filter(d => d.specialty_id === specialtyId.value)
  return dirs.reduce((sum, d) => sum + directionCount(d), 0)
})

function countOfLevel(levelIdValue: number): number {
  const dirs = specialtyId.value === null
    ? directions.value
    : directions.value.filter(d => d.specialty_id === specialtyId.value)
  let n = 0
  for (const d of dirs) {
    for (const lv of d.levels || []) {
      if (lv.level_id === levelIdValue) n += lv.courses?.length || 0
    }
  }
  return n
}

const coverClassMap: Record<number, string> = {
  1: 'cover-operation',
  2: 'cover-maintenance',
  3: 'cover-safety',
  4: 'cover-battery'
}

function coverClass(specialtyIdValue?: number | null) {
  return specialtyIdValue ? coverClassMap[specialtyIdValue] || 'cover-default' : 'cover-default'
}

function specialtyNameOf(id?: number | null) {
  if (!id) return ''
  return directions.value.find(d => d.specialty_id === id)?.name || ''
}

function levelNameOf(id?: number | null) {
  if (!id) return ''
  return levels.value.find(l => l.level_id === id)?.name || ''
}

async function loadCatalog() {
  try {
    // 拦截器已解包信封
    const [treeData, levelsData] = await Promise.all([
      trainingApi.getCatalogTree(),
      trainingApi.getLevels()
    ])
    directions.value = treeData.specialties || []
    levels.value = levelsData.levels || []
  } catch (e) {
    // 静默失败：方向/等级标签降级为空
  }
}

async function loadCourses() {
  loading.value = true
  try {
    const params: Record<string, unknown> = {
      page: currentPage.value,
      page_size: pageSize.value
    }
    if (specialtyId.value !== null) params.specialty_id = specialtyId.value
    if (levelId.value !== null) params.level_id = levelId.value
    const res = await tutorApi.getCourses(params)
    courses.value = res.courses
    total.value = res.total
  } catch (e) {
    console.error('Failed to load courses:', e)
  } finally {
    loading.value = false
  }
}

function selectSpecialty(id: number | null) {
  specialtyId.value = id
  currentPage.value = 1
  loadCourses()
}

function selectLevel(id: number | null) {
  levelId.value = id
  currentPage.value = 1
  loadCourses()
}

function goToChapters(courseId: number) {
  router.push(`/training/tutor/course/${courseId}/chapters`)
}

onMounted(() => {
  loadCatalog()
  loadCourses()
})
</script>

<style scoped>
.tutor-courses-page {
  max-width: 1200px;
  margin: 0 auto;
}

.page-header {
  margin-bottom: 20px;
}

.page-header h2 {
  font-size: 22px;
  color: #303133;
  margin-bottom: 8px;
}

.course-layout {
  display: flex;
  align-items: flex-start;
  gap: var(--space-5);
}

/* ===== 左栏：双卡片 ===== */
.cc-sidebar {
  width: 200px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.cc-filter-card {
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-lg);
  padding: var(--space-3) var(--space-2);
}

.cc-card-title {
  font-size: var(--text-xs);
  color: var(--color-text-tertiary);
  padding: var(--space-1) var(--space-3);
  margin-bottom: var(--space-1);
}

.cc-nav-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: var(--space-2) var(--space-3);
  margin-bottom: 2px;
  border: none;
  border-radius: var(--radius-md);
  background: transparent;
  cursor: pointer;
  font-family: var(--font-body);
  font-size: var(--text-sm);
  color: var(--color-text-primary);
  transition: background var(--duration-fast) var(--ease-default);
}

.cc-nav-item:hover {
  background: var(--color-bg-sidebar-hover);
}

.cc-nav-item.active {
  background: var(--color-primary-50);
  color: var(--color-primary-600);
  font-weight: var(--font-semibold);
}

.cc-nav-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cc-nav-count {
  font-size: var(--text-xs);
  color: var(--color-text-tertiary);
}

/* ===== 右栏：课程网格（与学员端课程中心一致） ===== */
.cc-main {
  flex: 1;
  min-width: 0;
}

.course-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: var(--space-4);
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

.card-cover {
  height: 120px;
  overflow: hidden;
}

.cover-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.cover-placeholder {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}

.cover-placeholder span {
  font-size: 40px;
  color: rgba(255, 255, 255, 0.8);
  font-weight: var(--font-bold);
  font-family: var(--font-display);
}

.cover-operation .cover-placeholder,
.cover-operation:not(:has(img)) {
  background: linear-gradient(135deg, #2563eb 0%, #7c3aed 100%);
}

.cover-maintenance .cover-placeholder,
.cover-maintenance:not(:has(img)) {
  background: linear-gradient(135deg, #0f766e 0%, #14b8a6 100%);
}

.cover-safety .cover-placeholder,
.cover-safety:not(:has(img)) {
  background: linear-gradient(135deg, #b45309 0%, #f59e0b 100%);
}

.cover-battery .cover-placeholder,
.cover-battery:not(:has(img)) {
  background: linear-gradient(135deg, #dc2626 0%, #f97316 100%);
}

.cover-default .cover-placeholder,
.cover-default:not(:has(img)) {
  background: linear-gradient(135deg, #6b7280 0%, #9ca3af 100%);
}

.card-body {
  padding: var(--space-3) var(--space-4) var(--space-4);
}

.card-tags {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
  margin-bottom: var(--space-1);
}

.card-title {
  font-size: var(--text-base);
  font-weight: var(--font-semibold);
  color: var(--color-text-primary);
  margin: 0 0 var(--space-1);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.card-desc {
  font-size: var(--text-xs);
  color: var(--color-text-secondary);
  line-height: 1.5;
  margin-bottom: var(--space-2);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  min-height: 34px;
}

.card-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: var(--text-xs);
  color: var(--color-text-tertiary);
}

.pagination-wrapper {
  display: flex;
  justify-content: center;
  margin-top: var(--space-5);
}

@media (max-width: 900px) {
  .course-layout {
    flex-direction: column;
    align-items: stretch;
  }

  .cc-sidebar {
    width: 100%;
  }
}
</style>
