<template>
  <div class="flex flex-col gap-6">
    <!-- #511：榜单/全部切换统一分段控件 -->
    <UiSegmentTabs
      :model-value="activeTab"
      :options="tabOptions"
      @update:model-value="(v: string) => { activeTab = v as 'hot' | 'featured' | 'all'; handleTabChange() }"
      class="mb-2"
    />
    <div class="flex items-start gap-5 max-[900px]:flex-col max-[900px]:items-stretch">
      <aside class="flex w-[200px] shrink-0 flex-col gap-3 max-[900px]:w-full">
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

      <main class="min-w-0 flex-1">
        <div class="cc-content">
          <!-- 加载：骨架屏（替代原全容器 v-loading，避免整块变灰遮挡已有内容） -->
          <UiSkeleton v-if="loading" variant="card" :count="6" />

          <!-- 错误：带重试 -->
          <UiErrorState
            v-else-if="loadError"
            title="课程加载失败"
            description="网络或服务端异常，可重试"
            :retrying="retrying"
            @retry="retryLoad"
          />

          <template v-else>
            <div v-if="courses.length > 0" class="grid grid-cols-[repeat(auto-fill,minmax(260px,1fr))] gap-4 max-[560px]:grid-cols-1">
              <CourseCard
                v-for="(course, i) in courses"
                :key="course.course_id"
                class="stagger-in"
                :style="stagger(i)"
                :name="course.name"
              :description="course.description"
              :cover-image="course.cover_image"
              :specialty-id="course.specialty_id"
              @click="openDetail(course)"
            >
              <template #cover>
                <span v-if="levelNameOf(course.level_id)" class="cc-cover-level absolute left-2 top-2 rounded-pill bg-black/40 px-2.5 py-0.5 text-xs font-semibold text-panel">
                  {{ levelNameOf(course.level_id) }}
                </span>
                <div v-if="course.is_hot || course.is_featured" class="cc-cover-badges absolute right-2 top-2 flex gap-1">
                  <span v-if="course.is_hot" class="cc-cover-badge rounded-pill bg-bad px-2 py-0.5 text-[11px] font-semibold text-panel">热门</span>
                  <span v-if="course.is_featured" class="cc-cover-badge rounded-pill bg-warn px-2 py-0.5 text-[11px] font-semibold text-panel">精品</span>
                </div>
              </template>
              <template #meta>
                <div class="cc-meta mb-2 flex flex-wrap gap-3 text-xs text-ink-3">
                  <span class="cc-meta-item">{{ course.chapter_count ?? 0 }} 章节</span>
                  <span v-if="course.theory_hours || course.practice_hours" class="cc-meta-item">
                    理论{{ course.theory_hours || 0 }}学时 · 实操{{ course.practice_hours || 0 }}学时
                  </span>
                  <el-tag v-if="course.points_price" size="small" type="warning" effect="plain">
                    {{ course.points_price }} 积分解锁
                  </el-tag>
                </div>
                <div class="cc-cert flex" v-if="course.certificate_name">
                  <el-tag size="small" type="success" effect="plain">
                    {{ course.certificate_name }}
                  </el-tag>
                </div>
              </template>
            </CourseCard>
          </div>

          <UiEmptyState
              v-else
              size="sm"
              :description="
                credentialStore.current
                  ? `“${credentialStore.current.name}” 内容建设中`
                  : '该方向/等级下暂无课程'
              "
            />
          </template>
        </div>

        <div class="cc-pagination mt-5 flex justify-center" v-if="total > pageSize">
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
        <div class="detail-header flex items-center justify-between gap-3 pr-6">
          <span class="detail-header-title text-lg font-semibold text-ink">{{ detailCourse?.name || '课程详情' }}</span>
          <UiButton v-if="detailCourse" :icon="courseFavorited ? StarFilled : Star" :type="courseFavorited ? 'warning' : 'default'" circle @click="toggleCourseFavorite"/>
        </div>
      </template>
      <div v-loading="detailLoading">
        <template v-if="detailCourse">
          <div class="detail-brief mb-3 flex flex-wrap gap-2">
            <el-tag v-if="detailCourse.level?.name" type="warning">{{ detailCourse.level.name }}</el-tag>
            <el-tag v-if="detailCourse.specialty?.name" type="primary" effect="plain">
              {{ detailCourse.specialty.name }}
            </el-tag>
          </div>
          <p class="detail-desc mb-4 text-sm text-ink-2">{{ detailCourse.description || '暂无简介' }}</p>
          <div v-if="detailCourse?.points_price" class="detail-redeem">
            <el-tag type="warning" effect="plain">{{ detailCourse.points_price }} 积分解锁</el-tag>
            <UiButton variant="warning" size="small" @click="handleRedeem">兑换解锁</UiButton>
          </div>

          <el-descriptions :column="2" border size="small" class="detail-descriptions mb-4">
            <el-descriptions-item label="理论学时">{{ detailCourse.theory_hours ?? '-' }} 学时</el-descriptions-item>
            <el-descriptions-item label="实操学时">{{ detailCourse.practice_hours ?? '-' }} 学时</el-descriptions-item>
            <el-descriptions-item label="章节数">{{ detailCourse.chapter_count ?? detailChapters.length }}</el-descriptions-item>
            <el-descriptions-item label="课程时长">{{ detailCourse.duration ? formatDuration(detailCourse.duration) : '-' }}</el-descriptions-item>
            <el-descriptions-item label="关联证书">
              <template v-if="detailCourse.certificate_template">
                {{ detailCourse.certificate_template.name }}
                <span v-if="detailCourse.certificate_template.validity_days" class="cert-valid text-xs text-ink-3">
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
                  class="prereq-tag mr-1 my-0.5"
                >{{ p.name }}</el-tag>
              </template>
              <span v-else>—</span>
            </el-descriptions-item>
          </el-descriptions>

          <div v-if="courseLearning?.is_enrolled" class="detail-progress mt-3.5 mb-1 flex items-center gap-3">
            <UiProgress
              :value="Math.round(courseLearning.progress ?? 0)"
              size="lg"
              tone="brand"
              class="progress-bar flex-1"
            />
            <span class="progress-text whitespace-nowrap text-[13px] text-ink-3">
              已完成 {{ courseLearning.completed_chapters ?? 0 }}/{{ courseLearning.total_chapters ?? detailChapters.length }} 章
            </span>
          </div>

          <div class="chapter-section mt-5">
            <h4 class="mb-3 text-base font-semibold text-ink">章节内容（{{ detailChapters.length }}）</h4>
            <div v-if="detailChapters.length > 0" class="chapter-list overflow-hidden rounded-card border border-line">
              <div
                v-for="(ch, i) in detailChapters"
                :key="ch.chapter_id"
                class="chapter-row flex cursor-pointer items-center gap-3 border-b border-line px-4 py-3 transition-colors duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:bg-canvas last:border-b-0"
                @click="goToChapter(ch)"
              >
                <span class="chapter-index flex size-6 shrink-0 items-center justify-center rounded-[6px] bg-ui-100 text-xs font-semibold text-ui-600">{{ i + 1 }}</span>
                <span class="chapter-title flex-1 text-sm text-ink">{{ ch.title }}</span>
                <el-tag v-if="chapterCompleted(ch.chapter_id)" size="small" type="success" effect="plain">已完成</el-tag>
                <span v-if="ch.duration" class="chapter-duration text-xs text-ink-3">{{ ch.duration }}分钟</span>
                <el-icon class="chapter-arrow text-ink-3"><ArrowRight /></el-icon>
              </div>
            </div>
            <UiEmptyState v-else description="该课程暂无章节内容" size="sm" />
          </div>
        </template>
      </div>
      <template #footer>
        <UiButton @click="detailVisible = false">关闭</UiButton>
        <UiButton variant="primary" v-if="detailChapters.length > 0" @click="goToChapter(continueChapter ?? detailChapters[0])">{{ continueChapter ? `继续学习：${continueChapterTitle}` : '开始学习' }}</UiButton>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowRight, Star, StarFilled } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { courseApi, type CourseDetail, type CourseSummary } from '@/api/course'
import { studentApi, type StudentCourseDetail } from '@/api/student'
import { favoriteApi } from '@/api/favorite'
import { trainingApi } from '@/api/training'
import { pointsApi } from '@/api/points'
import { useAsyncPage } from '@/composables/useAsyncPage'
import { useCourseCatalog, treeCatalogAdapter } from '@/composables/useCourseCatalog'
import { useStagger } from '@/composables/useStagger'
import { useCredentialRefetch } from '@/composables/useCredentialRefetch'
import { useCredentialStore } from '@/stores/credential'
import FacetCard from '@/components/catalog/FacetCard.vue'
import FacetItem from '@/components/catalog/FacetItem.vue'
import CourseCard from '@/components/catalog/CourseCard.vue'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'
import UiProgress from '@/components/ui/UiProgress.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiSegmentTabs from '@/components/ui/UiSegmentTabs.vue'

const stagger = useStagger()

const route = useRoute()
const router = useRouter()
let credentialStore: any = null
try {
  credentialStore = useCredentialStore()
} catch {
  credentialStore = { current: null, loadCurrent: async () => null, loadGrouped: async () => {} } as any
}

const courses = ref<CourseSummary[]>([])
const activeTab = ref<'hot' | 'featured' | 'all'>('hot')

// #511：榜单/全部切换分段选项
const tabOptions = [
  { label: '热门', value: 'hot' },
  { label: '精品', value: 'featured' },
  { label: '所有', value: 'all' }
]

// 三态 + 分页三件套收编（#388）：loader 只负责拉数据与写响应
const {
  loading,
  loadError,
  retrying,
  retry: retryLoad,
  page: currentPage,
  pageSize,
  total,
  run: loadCourses,
  handlePageChange,
  handleSizeChange
} = useAsyncPage(
  async () => {
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
    // credential_id 由主 client 请求拦截器默认注入（#387）
    const data = await courseApi.getCourses(params)
    courses.value = data.courses
    total.value = data.total
  },
  { defaultPageSize: 12 }
)

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

// 证件切换即重拉（单点：watch store.current.id，见 useCredentialRefetch）
useCredentialRefetch(() => {
  currentPage.value = 1
  loadCourses()
})
</script>

<style scoped>
/*
 * 仅保留 :deep 的 EP 覆盖（R1 允许）：el-descriptions 内部 table 在窄屏下改块级，
 * 无法用原子类表达，必须 scoped + :deep。
 */
@media screen and (max-width: 768px) {
  .detail-descriptions :deep(.el-descriptions__body .el-descriptions__table) {
    display: block;
  }
}
</style>
