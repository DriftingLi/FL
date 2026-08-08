<template>
  <div class="course-center">
    <div class="course-layout">
      <aside class="cc-sidebar">
        <div class="cc-sidebar-title">专业方向</div>
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
      </aside>

      <main class="cc-main">
        <div class="cc-level-pills">
          <button
            class="cc-pill"
            :class="{ active: levelId === null }"
            @click="selectLevel(null)"
          >
            全部等级
          </button>
          <button
            v-for="l in levels"
            :key="l.level_id"
            class="cc-pill"
            :class="{ active: levelId === l.level_id }"
            @click="selectLevel(l.level_id)"
          >
            {{ l.name }}
          </button>
        </div>

        <div class="cc-content" v-loading="loading">
          <div v-if="courses.length > 0" class="cc-grid">
            <div
              v-for="course in courses"
              :key="course.course_id"
              class="cc-card"
              @click="openDetail(course)"
            >
              <div class="cc-cover" :class="coverClass(course.specialty_id)">
                <img
                  v-if="course.cover_image"
                  v-lazy="course.cover_image"
                  :alt="course.name"
                  @error="course.cover_image = ''"
                />
                <div v-if="!course.cover_image" class="cc-cover-placeholder">
                  <span>{{ course.name.charAt(0) }}</span>
                </div>
                <span v-if="levelNameOf(course.level_id)" class="cc-cover-level">
                  {{ levelNameOf(course.level_id) }}
                </span>
              </div>
              <div class="cc-body">
                <h3 class="cc-name">{{ course.name }}</h3>
                <p class="cc-desc">{{ course.description || '暂无简介' }}</p>
                <div class="cc-meta">
                  <span class="cc-meta-item">{{ course.chapter_count ?? 0 }} 章节</span>
                  <span v-if="course.theory_hours || course.practice_hours" class="cc-meta-item">
                    理论{{ course.theory_hours || 0 }}学时 · 实操{{ course.practice_hours || 0 }}学时
                  </span>
                </div>
                <div class="cc-cert" v-if="course.certificate_name">
                  <el-tag size="small" type="success" effect="plain">
                    {{ course.certificate_name }}
                  </el-tag>
                </div>
              </div>
            </div>
          </div>

          <div v-else-if="!loading" class="cc-empty">
            <el-empty description="该方向/等级下暂无课程" :image-size="80" />
          </div>
        </div>

        <div class="cc-pagination" v-if="total > pageSize">
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
      </main>
    </div>

    <!-- 课程详情 -->
    <el-dialog
      v-model="detailVisible"
      :title="detailCourse?.name || '课程详情'"
      width="680px"
      destroy-on-close
    >
      <div v-loading="detailLoading">
        <template v-if="detailCourse">
          <div class="detail-brief">
            <el-tag v-if="detailCourse.level?.name" type="warning">{{ detailCourse.level.name }}</el-tag>
            <el-tag v-if="detailCourse.specialty?.name" type="primary" effect="plain">
              {{ detailCourse.specialty.name }}
            </el-tag>
          </div>
          <p class="detail-desc">{{ detailCourse.description || '暂无简介' }}</p>

          <el-descriptions :column="2" border size="small" class="detail-descriptions">
            <el-descriptions-item label="理论学时">{{ detailCourse.theory_hours ?? '-' }} 学时</el-descriptions-item>
            <el-descriptions-item label="实操学时">{{ detailCourse.practice_hours ?? '-' }} 学时</el-descriptions-item>
            <el-descriptions-item label="章节数">{{ detailCourse.chapter_count ?? detailChapters.length }}</el-descriptions-item>
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
import { ArrowRight } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { courseApi, type CourseDetail, type CourseSummary } from '@/api/course'
import { trainingApi, type CatalogDirectionNode, type CatalogLevel } from '@/api/training'
import { vLazy } from '@/composables/useLazyLoad'

const router = useRouter()

const loading = ref(false)
const courses = ref<CourseSummary[]>([])
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

function levelNameOf(levelIdValue?: number | null) {
  if (!levelIdValue) return ''
  return levels.value.find(l => l.level_id === levelIdValue)?.name || ''
}

const coverClassMap: Record<number, string> = {
  1: 'cc-cover-operation',
  2: 'cc-cover-maintenance',
  3: 'cc-cover-safety',
  4: 'cc-cover-battery'
}

function coverClass(specialtyIdValue?: number | null) {
  return specialtyIdValue ? coverClassMap[specialtyIdValue] || 'cc-cover-default' : 'cc-cover-default'
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
    if (specialtyId.value !== null) {
      params.specialty_id = specialtyId.value
    }
    if (levelId.value !== null) {
      params.level_id = levelId.value
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

async function loadCatalog() {
  try {
    const [treeRes, levelsRes] = await Promise.all([
      trainingApi.getCatalogTree(),
      trainingApi.getLevels()
    ])
    if (treeRes.code === 200) {
      directions.value = treeRes.data.specialties || []
    }
    if (levelsRes.code === 200) {
      levels.value = levelsRes.data.levels || []
    }
  } catch (error) {
    console.error('加载目录失败:', error)
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
  loadCatalog()
  loadCourses()
})
</script>

<style scoped>
.course-center {
  display: flex;
  flex-direction: column;
  gap: var(--space-6);
}

.course-layout {
  display: flex;
  align-items: flex-start;
  gap: var(--space-5);
}

/* ===== 左栏：专业方向导航 ===== */
.cc-sidebar {
  width: 200px;
  flex-shrink: 0;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-lg);
  padding: var(--space-3) var(--space-2);
}

.cc-sidebar-title {
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

.cc-nav-count {
  font-size: var(--text-xs);
  color: var(--color-text-tertiary);
}

/* ===== 右栏 ===== */
.cc-main {
  flex: 1;
  min-width: 0;
}

.cc-level-pills {
  display: flex;
  gap: var(--space-2);
  flex-wrap: wrap;
  margin-bottom: var(--space-4);
}

.cc-pill {
  padding: var(--space-1) var(--space-4);
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

.cc-pill:hover {
  border-color: var(--color-primary-300);
  color: var(--color-primary-500);
}

.cc-pill.active {
  border-color: var(--color-primary-500);
  background: var(--color-primary-500);
  color: white;
}

.cc-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: var(--space-4);
}

.cc-card {
  background: var(--color-bg-card);
  border-radius: var(--radius-xl);
  overflow: hidden;
  cursor: pointer;
  box-shadow: var(--shadow-xs);
  border: 1px solid var(--color-border-light);
  transition: all var(--duration-normal) var(--ease-default);
}

.cc-card:hover {
  box-shadow: var(--shadow-lg);
  transform: translateY(-4px);
}

.cc-cover {
  position: relative;
  height: 120px;
  overflow: hidden;
}

.cc-cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  transition: transform var(--duration-slow) var(--ease-default);
}

.cc-card:hover .cc-cover img {
  transform: scale(1.05);
}

.cc-cover-placeholder {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}

.cc-cover-placeholder span {
  font-size: 40px;
  color: rgba(255, 255, 255, 0.8);
  font-weight: var(--font-bold);
  font-family: var(--font-display);
}

.cc-cover-level {
  position: absolute;
  top: var(--space-2);
  left: var(--space-2);
  padding: 2px 10px;
  border-radius: var(--radius-full);
  background: rgba(0, 0, 0, 0.4);
  color: #fff;
  font-size: 12px;
  font-weight: 600;
}

.cc-cover-operation .cc-cover-placeholder,
.cc-cover-operation:not(:has(img)) {
  background: linear-gradient(135deg, #2563eb 0%, #7c3aed 100%);
}

.cc-cover-maintenance .cc-cover-placeholder,
.cc-cover-maintenance:not(:has(img)) {
  background: linear-gradient(135deg, #0f766e 0%, #14b8a6 100%);
}

.cc-cover-safety .cc-cover-placeholder,
.cc-cover-safety:not(:has(img)) {
  background: linear-gradient(135deg, #b45309 0%, #f59e0b 100%);
}

.cc-cover-battery .cc-cover-placeholder,
.cc-cover-battery:not(:has(img)) {
  background: linear-gradient(135deg, #dc2626 0%, #f97316 100%);
}

.cc-cover-default .cc-cover-placeholder,
.cc-cover-default:not(:has(img)) {
  background: linear-gradient(135deg, #6b7280 0%, #9ca3af 100%);
}

.cc-body {
  padding: var(--space-3) var(--space-4) var(--space-4);
}

.cc-name {
  font-size: var(--text-base);
  font-weight: var(--font-semibold);
  color: var(--color-text-primary);
  margin-bottom: var(--space-1);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cc-desc {
  font-size: var(--text-xs);
  color: var(--color-text-secondary);
  margin-bottom: var(--space-2);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  min-height: 34px;
}

.cc-meta {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-3);
  font-size: var(--text-xs);
  color: var(--color-text-tertiary);
  margin-bottom: var(--space-2);
}
.cc-cert {
  display: flex;
}

.cc-empty {
  padding: 60px 0;
}

.cc-pagination {
  display: flex;
  justify-content: center;
  margin-top: var(--space-5);
}

/* ===== 详情弹窗 ===== */
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

@media screen and (max-width: 768px) {
  .detail-descriptions :deep(.el-descriptions__body .el-descriptions__table) {
    display: block;
  }

  .course-layout {
    flex-direction: column;
    /* 纵向后子元素需横向拉伸，否则 .cc-main 宽度=内容宽度导致整页横向溢出 */
    align-items: stretch;
  }

  .cc-sidebar {
    width: 100%;
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-1);
  }

  .cc-sidebar-title {
    width: 100%;
  }

  .cc-nav-item {
    width: auto;
  }
}
</style>
