<template>
  <div class="course-center">
    <el-tabs v-model="activeTab" class="cc-tabs" @tab-change="handleTabChange">
      <el-tab-pane label="热门" name="hot" />
      <el-tab-pane label="精品" name="featured" />
      <el-tab-pane label="所有" name="all" />
    </el-tabs>
    <div class="course-layout">
      <aside class="cc-sidebar">
        <FacetCard v-if="activeTab === 'all'" title="专业方向">
          <FacetItem
            :active="specialtyId === null"
            name="全部课程"
            :count="totalAll"
            @select="selectDirection(null)"
          />
          <FacetItem
            v-for="d in directions"
            :key="d.specialty_id"
            :active="specialtyId === d.specialty_id"
            :name="d.name"
            :count="countOfDirection(d.specialty_id)"
            @select="selectDirection(d.specialty_id)"
          />
        </FacetCard>

        <FacetCard v-if="activeTab === 'all'" title="课程等级">
          <FacetItem
            :active="levelId === null"
            name="全部等级"
            :count="scopedTotal"
            @select="selectLevel(null)"
          />
          <FacetItem
            v-for="l in levels"
            :key="l.level_id"
            :active="levelId === l.level_id"
            :name="l.name"
            :count="countOfLevel(l.level_id)"
            @select="selectLevel(l.level_id)"
          />
        </FacetCard>
      </aside>

      <main class="cc-main">
        <div class="cc-content" v-loading="loading">
          <div v-if="courses.length > 0" class="cc-grid">
            <CourseCard
              v-for="course in courses"
              :key="course.course_id"
              :name="course.name"
              :description="course.description"
              :cover-image="course.cover_image"
              :specialty-id="course.specialty_id"
              @click="openDetail(course)"
            >
              <template #cover>
                <span v-if="levelNameOf(course.level_id)" class="cc-cover-level">
                  {{ levelNameOf(course.level_id) }}
                </span>
                <div v-if="course.is_hot || course.is_featured" class="cc-cover-badges">
                  <span v-if="course.is_hot" class="cc-cover-badge cc-cover-badge--hot">热门</span>
                  <span v-if="course.is_featured" class="cc-cover-badge cc-cover-badge--featured">精品</span>
                </div>
              </template>
              <template #meta>
                <div class="cc-meta">
                  <span class="cc-meta-item">{{ course.chapter_count ?? 0 }} 章节</span>
                  <span v-if="course.theory_hours || course.practice_hours" class="cc-meta-item">
                    理论{{ course.theory_hours || 0 }}学时 · 实操{{ course.practice_hours || 0 }}学时
                  </span>
                  <el-tag v-if="course.points_price" size="small" type="warning" effect="plain">
                    {{ course.points_price }} 积分解锁
                  </el-tag>
                </div>
                <div class="cc-cert" v-if="course.certificate_name">
                  <el-tag size="small" type="success" effect="plain">
                    {{ course.certificate_name }}
                  </el-tag>
                </div>
              </template>
            </CourseCard>
          </div>

          <div v-else-if="!loading" class="cc-empty">
            <el-empty :description="credentialStore.current ? `“${credentialStore.current.name}” 内容建设中` : '该方向/等级下暂无课程'" :image-size="80" />
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
      <template #header>
        <div class="detail-header">
          <span class="detail-header-title">{{ detailCourse?.name || '课程详情' }}</span>
          <el-button
            v-if="detailCourse"
            :icon="courseFavorited ? StarFilled : Star"
            :type="courseFavorited ? 'warning' : 'default'"
            circle
            @click="toggleCourseFavorite"
          />
        </div>
      </template>
      <div v-loading="detailLoading">
        <template v-if="detailCourse">
          <div class="detail-brief">
            <el-tag v-if="detailCourse.level?.name" type="warning">{{ detailCourse.level.name }}</el-tag>
            <el-tag v-if="detailCourse.specialty?.name" type="primary" effect="plain">
              {{ detailCourse.specialty.name }}
            </el-tag>
          </div>
          <p class="detail-desc">{{ detailCourse.description || '暂无简介' }}</p>
          <div v-if="detailCourse?.points_price" class="detail-redeem">
            <el-tag type="warning" effect="plain">{{ detailCourse.points_price }} 积分解锁</el-tag>
            <el-button size="small" type="warning" @click="handleRedeem">兑换解锁</el-button>
          </div>

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

          <div v-if="courseLearning?.is_enrolled" class="detail-progress">
            <el-progress
              :percentage="Math.round(courseLearning.progress ?? 0)"
              :stroke-width="10"
              class="progress-bar"
            />
            <span class="progress-text">
              已完成 {{ courseLearning.completed_chapters ?? 0 }}/{{ courseLearning.total_chapters ?? detailChapters.length }} 章
            </span>
          </div>

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
                <el-tag v-if="chapterCompleted(ch.chapter_id)" size="small" type="success" effect="plain">已完成</el-tag>
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
          @click="goToChapter(continueChapter ?? detailChapters[0])"
        >{{ continueChapter ? `继续学习：${continueChapterTitle}` : '开始学习' }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowRight, Star, StarFilled } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { courseApi, type CourseDetail, type CourseSummary } from '@/api/course'
import { studentApi, type StudentCourseDetail } from '@/api/student'
import { favoriteApi } from '@/api/favorite'
import { trainingApi } from '@/api/training'
import { pointsApi } from '@/api/points'
import { useCourseCatalog, treeCatalogAdapter } from '@/composables/useCourseCatalog'
import { useCredentialStore } from '@/stores/credential'
import FacetCard from '@/components/catalog/FacetCard.vue'
import FacetItem from '@/components/catalog/FacetItem.vue'
import CourseCard from '@/components/catalog/CourseCard.vue'

const route = useRoute()
const router = useRouter()
let credentialStore: any = null
try {
  credentialStore = useCredentialStore()
} catch {
  credentialStore = { current: null, loadCurrent: async () => null, loadGrouped: async () => {} } as any
}

const loading = ref(false)
const courses = ref<CourseSummary[]>([])
const currentPage = ref(1)
const pageSize = ref(12)
const total = ref(0)
const activeTab = ref<'hot' | 'featured' | 'all'>('hot')



const {
  directions,
  levels,
  specialtyId,
  levelId,
  totalAll,
  scopedTotal,
  countOfDirection,
  countOfLevel,
  selectDirection,
  selectLevel,
  fetchCatalog,
  levelNameOf
} = useCourseCatalog({
  adapter: treeCatalogAdapter(() => trainingApi.getCatalogTree()),
  onSelect: () => {
    currentPage.value = 1
    loadCourses()
  }
})

function formatDuration(minutes: number) {
  if (minutes < 60) {
    return `${minutes}分钟`
  }
  const hours = Math.floor(minutes / 60)
  const mins = minutes % 60
  return mins > 0 ? `${hours}小时${mins}分钟` : `${hours}小时`
}

function handleTabChange() {
  currentPage.value = 1
  loadCourses()
}

async function loadCourses() {
  loading.value = true
  try {
    const params: Record<string, any> = {
      page: currentPage.value,
      page_size: pageSize.value,
      filter: activeTab.value
    }
    if (activeTab.value === 'all') {
      if (specialtyId.value !== null) {
        params.specialty_id = specialtyId.value
      }
      if (levelId.value !== null) {
        params.level_id = levelId.value
      }
    }
    if (credentialStore.current?.id) {
      params.credential_id = credentialStore.current.id
    }
    const data = await courseApi.getCourses(params)
    courses.value = data.courses
    total.value = data.total
  } catch (error) {
    console.error('加载课程失败:', error)
    /* 错误已由拦截器提示 */
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

// ===== 课程详情 =====
const detailVisible = ref(false)
const detailLoading = ref(false)
const detailCourse = ref<CourseDetail | null>(null)
const detailChapters = ref<{ chapter_id: number; title: string; duration?: number }[]>([])

// 学习状态（ADR-0017）：进度/完成章节数/最后学习章节；未学课程为 null
const courseLearning = ref<{ is_enrolled?: boolean; progress?: number; completed_chapters?: number; total_chapters?: number } | null>(null)
const completedChapterIds = ref<Set<number>>(new Set())

// 收藏状态（ADR-0018）
const courseFavorited = ref(false)
const courseFavoriteId = ref<number>(0)

const continueChapter = computed(() => {
  const learning = courseLearning.value
  if (!learning?.is_enrolled) return null
  const lastId = learningDetail.value?.last_chapter_id
  return lastId ? detailChapters.value.find((ch) => ch.chapter_id === lastId) || null : null
})

const continueChapterTitle = computed(() => continueChapter.value?.title || '')

// 单课程学习详情（每章完成状态/最后位置，ADR-0017）
const learningDetail = ref<StudentCourseDetail | null>(null)

function chapterCompleted(chapterId: number) {
  return completedChapterIds.value.has(chapterId)
}

async function loadCourseFavorite(courseId: number) {
  courseFavorited.value = false
  courseFavoriteId.value = 0
  try {
    const res = await favoriteApi.check({ target_type: 'course', target_id: courseId })
    courseFavorited.value = !!res?.favorited
    courseFavoriteId.value = res?.favorite_id || 0
  } catch (error) {
    console.error('查询收藏状态失败:', error)
  }
}

async function toggleCourseFavorite() {
  const courseId = detailCourse.value?.course_id
  if (!courseId) return
  try {
    if (courseFavorited.value) {
      await favoriteApi.remove(courseFavoriteId.value)
      courseFavorited.value = false
      courseFavoriteId.value = 0
      ElMessage.success('已取消收藏')
    } else {
      const res = await favoriteApi.add({ target_type: 'course', target_id: courseId })
      courseFavorited.value = true
      courseFavoriteId.value = res?.favorite_id || 0
      ElMessage.success('已收藏')
    }
  } catch (error) {
    console.error('收藏操作失败:', error)
    /* 错误已由拦截器提示 */
  }
}

async function loadLearningState(courseId: number) {
  courseLearning.value = null
  learningDetail.value = null
  completedChapterIds.value = new Set()
  try {
    // 课程详情（is_enrolled/进度/最后章节）与单课程学习详情（每章完成态）并行
    const [detail, learning] = await Promise.all([
      courseApi.getCourseDetail(courseId).catch(() => null),
      studentApi.getStudentCourseDetail(courseId).catch(() => null)
    ])
    courseLearning.value = detail
      ? {
          is_enrolled: detail.is_enrolled,
          progress: detail.progress,
          completed_chapters: detail.completed_chapters,
          total_chapters: detail.chapters?.length || learning?.total_chapters
        }
      : null
    learningDetail.value = learning
    completedChapterIds.value = new Set(
      (learning?.chapters || []).filter((ch) => ch.completed).map((ch) => ch.chapter_id)
    )
  } catch (error) {
    console.error('加载学习状态失败:', error)
  }
}

async function openDetail(course: CourseSummary) {
  detailCourse.value = course
  detailVisible.value = true
  detailLoading.value = true
  detailChapters.value = []
  try {
    const data = await courseApi.getCourseDetail(course.course_id)
    detailCourse.value = { ...course, ...(data.course_info || {}) }
    detailChapters.value = data.chapters || []
    loadLearningState(course.course_id)
    loadCourseFavorite(course.course_id)
  } catch (error) {
    console.error('加载课程详情失败:', error)
    /* 错误已由拦截器提示 */
  } finally {
    detailLoading.value = false
  }
}

// 按 id 打开详情（搜索/收藏跳转带 query.course_id）
async function openDetailById(courseId: number) {
  detailCourse.value = null
  detailVisible.value = true
  detailLoading.value = true
  detailChapters.value = []
  try {
    const data = await courseApi.getCourseDetail(courseId)
    detailCourse.value = (data.course_info || null) as CourseDetail | null
    detailChapters.value = data.chapters || []
    loadLearningState(courseId)
    loadCourseFavorite(courseId)
  } catch (error) {
    console.error('加载课程详情失败:', error)
    /* 错误已由拦截器提示 */
    detailVisible.value = false
  } finally {
    detailLoading.value = false
  }
}

async function handleRedeem() {
  if (!detailCourse.value?.course_id) return
  const price = detailCourse.value.points_price
  if (!price) return
  try {
    await ElMessageBox.confirm(`该课程需 ${price} 积分解锁，确认兑换？`, '积分兑换', {
      confirmButtonText: '确认兑换',
      cancelButtonText: '取消',
      type: 'warning'
    })
  } catch {
    return
  }
  try {
    await pointsApi.redeemCourse(detailCourse.value.course_id)
    ElMessage.success('兑换成功，已解锁')
    // 刷新详情以更新 points_price 仍显示但已可进入
    detailCourse.value.points_price = null as unknown as number
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e)
    ElMessage.error(msg || '兑换失败')
  }
}

function goToChapter(ch: { chapter_id: number }) {
  if (!detailCourse.value) return
  if (detailCourse.value.points_price) {
    ElMessage.warning('该课程需积分解锁，请先兑换')
    return
  }
  detailVisible.value = false
  router.push({
    name: 'ChapterView',
    params: { courseId: detailCourse.value.course_id, chapterId: ch.chapter_id }
  })
}

onMounted(() => {
  fetchCatalog()
  loadCourses()
  // 搜索/收藏跳转：?course_id= 自动打开课程详情
  const queryCourseId = Number(route.query.course_id)
  if (queryCourseId > 0) {
    openDetailById(queryCourseId)
  }
})

watch(() => credentialStore.current?.id, () => {
  currentPage.value = 1
  loadCourses()
})

window.addEventListener('credential-switched', () => {
  currentPage.value = 1
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

/* ===== 左栏 ===== */
.cc-sidebar {
  width: 200px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

/* ===== 右栏 ===== */
.cc-main {
  flex: 1;
  min-width: 0;
}

.cc-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: var(--space-4);
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

.cc-cover-badges {
  position: absolute;
  top: var(--space-2);
  right: var(--space-2);
  display: flex;
  gap: 4px;
}

.cc-cover-badge {
  padding: 2px 8px;
  border-radius: var(--radius-full);
  color: #fff;
  font-size: 11px;
  font-weight: 600;
}

.cc-cover-badge--hot {
  background: #f56c6c;
}

.cc-cover-badge--featured {
  background: #e6a23c;
}

.cc-tabs {
  margin-bottom: var(--space-2);
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

.detail-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-right: 24px;
}

.detail-header-title {
  font-size: 18px;
  font-weight: 600;
  color: #303133;
}

.detail-progress {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 14px 0 4px;
}

.progress-bar {
  flex: 1;
}

.progress-text {
  font-size: 13px;
  color: #909399;
  white-space: nowrap;
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

@media (max-width: 900px) {
  .course-layout {
    flex-direction: column;
    /* 纵向后子元素需横向拉伸，否则 .cc-main 宽度=内容宽度导致整页横向溢出 */
    align-items: stretch;
  }

  .cc-sidebar {
    width: 100%;
  }
}

@media screen and (max-width: 768px) {
  .detail-descriptions :deep(.el-descriptions__body .el-descriptions__table) {
    display: block;
  }
}
</style>
