<template>
  <section class="mx-auto flex w-full max-w-5xl flex-col gap-5 px-4 py-6">
    <h1 class="m-0 text-2xl font-semibold text-ink">课程中心</h1>
    <div class="flex flex-wrap items-center gap-3">
      <UiSegmentTabs :model-value="filter ?? 'all'" :options="filterOptions" @update:model-value="selectFilter" />
      <UiSelect class="w-full sm:w-60" aria-label="课程分类" :model-value="categoryId ?? 0"
        :options="categoryOptions" @update:model-value="selectCategory" />
    </div>
    <UiAsyncSection :loading="loading" :error="loadError" :empty="isEmpty" :retrying="retrying"
      error-title="课程加载失败" @retry="retry">
      <template #empty>
        <UiEmptyState title="暂无课程" description="请选择其他分类或稍后再试。" />
      </template>
      <div class="grid grid-cols-1 gap-4 min-[620px]:grid-cols-2 lg:grid-cols-3">
        <RouterLink v-for="item in courses" :key="item.course_id" :to="href('PublicCourseDetail', { courseId: item.course_id })"
          class="min-w-0 rounded-card text-inherit no-underline">
          <CourseCard :name="item.name" :description="item.description" :cover-image="item.cover_image" cover-ratio="16:9">
            <template #tags>
              <UiTag>{{ item.category.name }}</UiTag>
            </template>
            <template #meta>
              <UiTag v-if="item.points_price" tone="warning">{{ item.points_price }} 积分解锁</UiTag>
            </template>
          </CourseCard>
        </RouterLink>
      </div>
      <UiPagination v-if="pages > 1" :current-page="page" :page-size="12" :total="total" @current-change="handlePageChange" />
    </UiAsyncSection>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { courseApi, type PublicCoursePageDTO } from '@/api/course'
import { trainingApi } from '@/api/training'
import CourseCard from '@/components/catalog/CourseCard.vue'
import UiAsyncSection from '@/components/ui/UiAsyncSection.vue'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiPagination from '@/components/ui/UiPagination.vue'
import UiSegmentTabs from '@/components/ui/UiSegmentTabs.vue'
import UiSelect from '@/components/ui/UiSelect.vue'
import UiTag from '@/components/ui/UiTag.vue'
import { useAsyncPage } from '@/composables/useAsyncPage'
import { href } from '@/config/pages'

const route = useRoute()
const router = useRouter()
const filterOptions = [
  { label: '全部课程', value: 'all' },
  { label: '热门', value: 'hot' },
  { label: '精品', value: 'featured' }
]
const filter = computed(() => route.query.filter === 'hot' || route.query.filter === 'featured' ? route.query.filter : undefined)
const categoryId = computed(() => {
  const raw = route.query.category_id
  if (typeof raw !== 'string' || !/^[1-9][0-9]*$/.test(raw)) return undefined
  const id = Number(raw)
  return Number.isSafeInteger(id) ? id : undefined
})
const categories = ref<{ id: number; name: string }[]>([])
const categoryOptions = computed(() => [
  { label: '全部分类', value: 0 },
  ...categories.value.map(({ id, name }) => ({ label: name, value: id }))
])
const courses = ref<PublicCoursePageDTO['courses']>([])
const pages = ref(0)
const total = ref(0)
const { page, run, loading, loadError, isEmpty, retrying, retry, handlePageChange } = useAsyncPage(
  (currentPage) => courseApi.getPublicCourses({ category_id: categoryId.value, page: currentPage, filter: filter.value }),
  {
    credentialScoped: false,
    filterDeps: [categoryId, filter],
    itemsRef: courses,
    apply: (result) => {
      courses.value = result.courses
      pages.value = result.pages
      total.value = result.total
    }
  }
)

function selectCategory(value: unknown) {
  const category_id = typeof value === 'number' && value > 0 ? String(value) : undefined
  void router.push({ ...href('CourseList'), query: { category_id, filter: filter.value } })
}

function selectFilter(value: string) {
  void router.push({ ...href('CourseList'), query: { category_id: categoryId.value, filter: value === 'all' ? undefined : value } })
}

onMounted(() => {
  void run()
  void trainingApi.getCategories().then(result => { categories.value = result.categories }).catch(() => undefined)
})
</script>
