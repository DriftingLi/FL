<template>
  <div>
    <UiPageHeader
      title="我的课程"
      :subtitle="total > 0 ? `共 ${total} 门课程` : undefined"
    >
      <template #actions>
        <UiSelect
          v-model="credentialId"
          :options="credentialOptions"
          placeholder="全部证件"
          clearable
          class="!w-[180px]"
          @change="onCredentialChange"
        />
      </template>
    </UiPageHeader>

    <!-- 左栏：双卡片（专业方向 / 课程等级），与学员端课程中心共享筛选 module -->
    <div class="flex items-start gap-5 max-[900px]:flex-col max-[900px]:items-stretch">
      <aside class="flex w-[200px] shrink-0 flex-col gap-3 max-[900px]:w-full">
        <FacetCard title="专业方向">
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

        <FacetCard title="课程等级">
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

      <!-- 右栏：课程网格 -->
      <main class="min-w-0 flex-1">
        <UiErrorState
          v-if="loadError"
          title="课程加载失败"
          description="网络或服务端异常，可重试"
          :retrying="retrying"
          @retry="handleRetry"
        />

        <UiSkeleton v-else-if="loading" variant="card" :count="6" />

        <UiEmptyState
          v-else-if="courses.length === 0"
          title="暂无课程"
          description="当前筛选条件下没有课程，试试切换专业方向或课程等级。"
          action-text="重置筛选"
          @action="resetFilters"
        />

        <template v-else>
          <div class="grid grid-cols-[repeat(auto-fill,minmax(260px,1fr))] gap-4">
            <CourseCard
              v-for="course in courses"
              :key="course.course_id"
              :name="course.name"
              :description="course.description"
              :cover-image="course.cover_image"
              :specialty-id="course.specialty_id"
              @click="goToChapters(course.course_id)"
            >
              <template #tags>
                <div class="mb-1 flex flex-wrap gap-1.5">
                  <UiTag v-if="levelNameOf(course.level_id)" :tone="LEVEL_TONE[levelTagType(levelNameOf(course.level_id))]">
                    {{ levelNameOf(course.level_id) }}
                  </UiTag>
                  <UiTag v-if="specialtyNameOf(course.specialty_id)" tone="brand">
                    {{ specialtyNameOf(course.specialty_id) }}
                  </UiTag>
                  <UiTag v-if="credentialNameOf(course.credential_id)" tone="neutral">
                    {{ credentialNameOf(course.credential_id) }}
                  </UiTag>
                </div>
              </template>
              <template #meta>
                <div class="flex items-center justify-between text-xs text-ink-3">
                  <span>{{ course.chapter_count || 0 }} 个章节</span>
                  <UiButton size="small">
                    管理章节 <el-icon><ArrowRight /></el-icon>
                  </UiButton>
                </div>
              </template>
            </CourseCard>
          </div>

          <div v-if="total > pageSize" class="mt-5 flex justify-center">
            <el-pagination
              v-model:current-page="currentPage"
              :page-size="pageSize"
              :total="total"
              layout="prev, pager, next"
              @current-change="loadCourses"
            />
          </div>
        </template>
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowRight } from '@element-plus/icons-vue'
import { tutorApi, type TutorCourse } from '@/api/tutor'
import { trainingApi } from '@/api/training'
import { credentialApi, type CredentialDict } from '@/api/credential'
import { levelTagType, type LevelTagType } from '@/constants/level'
import { useCourseCatalog, treeCatalogAdapter } from '@/composables/useCourseCatalog'
import FacetCard from '@/components/catalog/FacetCard.vue'
import FacetItem from '@/components/catalog/FacetItem.vue'
import CourseCard from '@/components/catalog/CourseCard.vue'
import UiPageHeader from '@/components/ui/UiPageHeader.vue'
import UiSelect from '@/components/ui/UiSelect.vue'
import UiTag from '@/components/ui/UiTag.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'

/** 导师端课程行：后端额外返回 credential_id，TutorCourse 尚未收录 */
type TutorCourseRow = TutorCourse & { credential_id?: number }

const router = useRouter()
const credentials = ref<CredentialDict[]>([])
const credentialId = ref<number | string | undefined>(undefined)
const loading = ref(false)
const loadError = ref(false)
const retrying = ref(false)
const courses = ref<TutorCourseRow[]>([])
const currentPage = ref(1)
const pageSize = ref(12)
const total = ref(0)

const credentialOptions = computed(() =>
  credentials.value.map((c) => ({ label: c.name, value: c.id as number | string }))
)

/** el-tag type → UiTag tone。等级配色沿用学员端/管理端既有约定，不做改动。 */
const LEVEL_TONE: Record<LevelTagType, 'brand' | 'success' | 'warning' | 'danger' | 'neutral'> = {
  success: 'success',
  primary: 'brand',
  warning: 'warning',
  danger: 'danger',
  info: 'neutral'
}

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
  levelNameOf,
  specialtyNameOf
} = useCourseCatalog({
  adapter: treeCatalogAdapter(() => trainingApi.getCatalogTree()),
  onSelect: () => {
    currentPage.value = 1
    loadCourses()
  }
})

function credentialNameOf(id?: number): string {
  if (!id) return ''
  return credentials.value.find((c) => c.id === id)?.name || ''
}

async function loadCredentials() {
  try {
    const data = await credentialApi.listCredentials()
    credentials.value = data.credentials || []
  } catch {}
}

async function loadCourses() {
  loading.value = true
  loadError.value = false
  try {
    const params: Record<string, string | number> = {
      page: currentPage.value,
      page_size: pageSize.value
    }
    const cid = credentialId.value
    if (cid !== undefined && cid !== null && cid !== '') params.credential_id = Number(cid)
    if (specialtyId.value !== null) params.specialty_id = specialtyId.value
    if (levelId.value !== null) params.level_id = levelId.value
    const res = await tutorApi.getCourses(params)
    courses.value = res.courses
    total.value = res.total
  } catch (e) {
    loadError.value = true
    console.error('Failed to load courses:', e)
  } finally {
    loading.value = false
  }
}

async function handleRetry() {
  retrying.value = true
  try {
    await loadCourses()
  } finally {
    retrying.value = false
  }
}

/** 重置筛选：直接写 ref 而非调 selectDirection/selectLevel，避免每次选择都触发一次 onSelect → loadCourses */
function resetFilters() {
  specialtyId.value = null
  levelId.value = null
  credentialId.value = undefined
  currentPage.value = 1
  loadCourses()
}

/** 证件下拉变化：UiSelect 未声明 change，监听器经 attrs 透传到内部 el-select */
function onCredentialChange() {
  currentPage.value = 1
  loadCourses()
}

function goToChapters(courseId: number) {
  router.push(`/training/tutor/course/${courseId}/chapters`)
}

onMounted(() => {
  fetchCatalog()
  loadCourses()
  loadCredentials()
})
</script>
