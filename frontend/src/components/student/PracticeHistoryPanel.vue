<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { practiceModeApi, type HistoryItemDTO, type HistoryResultDTO, type PracticeStatsDTO } from '@/api/practiceMode'
import { trainingApi, type TrainingCategoryDTO } from '@/api/training'
import { useAsyncPage } from '@/composables/useAsyncPage'
import { formatDateTime } from '@/utils/format'
import UiAsyncSection from '@/components/ui/UiAsyncSection.vue'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiPagination from '@/components/ui/UiPagination.vue'
import UiSelect from '@/components/ui/UiSelect.vue'
import UiTag from '@/components/ui/UiTag.vue'

const categoryId = ref<number | 'all' | 'unclassified'>('all')
const categories = ref<TrainingCategoryDTO[]>([])
const records = ref<HistoryItemDTO[]>([])
const stats = ref<PracticeStatsDTO>()
const categoryOptions = computed(() => [
  { label: '全部分类', value: 'all' },
  ...categories.value.map(category => ({ label: category.name, value: category.id })),
  { label: '历史未分类', value: 'unclassified' }
])

const { loading, loadError, retrying, retry, page, pageSize, total, run, handlePageChange } = useAsyncPage<[HistoryResultDTO, PracticeStatsDTO]>(
  (): Promise<[HistoryResultDTO, PracticeStatsDTO]> => Promise.all([
    practiceModeApi.getHistory({ category_id: categoryId.value, page: page.value, page_size: pageSize.value }),
    practiceModeApi.getStats(categoryId.value)
  ]),
  { apply: ([history, report]) => {
    records.value = history.records
    total.value = history.total
    stats.value = report
  } }
)

const { run: loadCategories } = useAsyncPage(trainingApi.getHistoricalCategories, {
  apply: (data) => { categories.value = data.categories }
})

watch(categoryId, () => {
  page.value = 1
  void run()
})

onMounted(() => {
  void loadCategories()
  void run()
})

function categoryName(id?: number) {
  return id == null ? '历史未分类' : categories.value.find(category => category.id === id)?.name ?? '—'
}
</script>

<template>
  <section class="rounded-card border border-line bg-panel p-4">
    <div class="mb-3 flex flex-wrap items-center justify-between gap-3">
      <h3 class="text-base font-semibold text-ink">练习记录与统计</h3>
      <UiSelect v-model="categoryId" aria-label="练习历史分类" class="w-[170px]" :options="categoryOptions" />
    </div>
    <UiAsyncSection :error="loadError" :loading="loading" :retrying="retrying" error-title="练习记录加载失败" @retry="retry">
      <p v-if="stats" class="mb-3 text-sm text-ink-3">作答 {{ stats.total }} 次 · 答对 {{ stats.correct }} 次 · 正确率 {{ stats.accuracy }}%</p>
      <ul v-if="records.length" class="m-0 divide-y divide-line p-0">
        <li v-for="record in records" :key="record.id" class="list-none py-3">
          <div class="flex flex-wrap items-center gap-2 text-sm">
            <UiTag size="small" :tone="record.is_correct ? 'success' : 'danger'">{{ record.is_correct ? '答对' : '答错' }}</UiTag>
            <span>{{ categoryName(record.category_id) }}</span>
            <time :datetime="record.created_at" class="text-ink-3">{{ formatDateTime(record.created_at, '') }}</time>
          </div>
          <p class="mb-0 mt-2 line-clamp-2 text-sm text-ink">{{ record.question?.content || '题目 #' + record.question_id }}</p>
        </li>
      </ul>
      <UiEmptyState v-else description="暂无练习记录" />
      <UiPagination v-if="total > pageSize" v-model:current-page="page" :page-size="pageSize" :total="total" @current-change="handlePageChange" />
    </UiAsyncSection>
  </section>
</template>
