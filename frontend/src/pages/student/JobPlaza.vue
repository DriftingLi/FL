<template>
  <div class="flex flex-col gap-4">
    <h1 class="text-xl font-bold text-ink">职位广场</h1>

    <div class="rounded-card border border-line bg-panel p-4 flex flex-wrap gap-2">
      <el-select v-model="filters.specialty_id" clearable placeholder="专业方向" class="!w-40" @change="load">
        <el-option v-for="s in specialties" :key="s.specialty_id" :label="s.name" :value="s.specialty_id" />
      </el-select>
      <el-input v-model="filters.region" placeholder="地区" clearable class="!w-32" @change="load" />
      <el-input v-model.number="filters.salary_min" placeholder="最低薪资" type="number" clearable class="!w-28" @change="load" />
      <el-input v-model.number="filters.salary_max" placeholder="最高薪资" type="number" clearable class="!w-28" @change="load" />
      <el-input v-model="filters.experience" placeholder="经验要求" clearable class="!w-28" @change="load" />
    </div>

    <UiErrorState
      v-if="loadError"
      title="职位加载失败"
      description="网络或服务端异常，可重试"
      :retrying="retrying"
      @retry="handleRetry"
    />
    <UiSkeleton v-else-if="loading" variant="list" :count="4" />
    <div v-else-if="items.length === 0" class="rounded-card border border-line bg-panel p-8 text-center text-ink-3">
      暂无招聘中的职位
    </div>
    <div v-else class="grid gap-3">
      <router-link
        v-for="item in items"
        :key="String(item.id)"
        :to="`/training/jobs/${item.id}`"
        class="rounded-card border border-line bg-panel p-4 hover:border-ui-200 transition-colors block"
      >
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0 flex-1">
            <div class="text-sm font-semibold text-ink">{{ item.title }}</div>
            <div class="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-ink-3">
              <span v-if="item.specialty_name">{{ item.specialty_name }}</span>
              <span v-if="item.region">{{ item.region }}</span>
              <span v-if="item.salary_text">{{ item.salary_text }}</span>
              <span v-if="item.experience_req">经验：{{ item.experience_req }}</span>
            </div>
            <div class="mt-2 text-xs text-ink-2 line-clamp-2">{{ item.description }}</div>
          </div>
          <div class="shrink-0 text-right">
            <div v-if="item.company_name" class="text-xs text-ink-2">{{ item.company_name }}</div>
            <div class="mt-1 text-[10px] text-ink-3">{{ item.published_at.slice(0, 10) }}</div>
          </div>
        </div>
      </router-link>
    </div>
    <div v-if="total > 0" class="flex justify-center">
      <el-pagination
        v-model:current-page="page"
        :page-size="pageSize"
        :total="total"
        layout="prev, pager, next"
        @current-change="handlePageChange"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { jobApi, type JobPosting } from '@/api/job'
import { trainingApi, type CatalogDirection } from '@/api/training'
import { useAsyncPage } from '@/composables/useAsyncPage'
import UiErrorState from '@/components/ui/UiErrorState.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'

const items = ref<JobPosting[]>([])
const specialties = ref<CatalogDirection[]>([])

const filters = reactive<{
  specialty_id: number | null
  region: string
  salary_min: number | null
  salary_max: number | null
  experience: string
}>({
  specialty_id: null,
  region: '',
  salary_min: null,
  salary_max: null,
  experience: ''
})

const {
  loading,
  loadError,
  retrying,
  retry: handleRetry,
  total,
  page,
  pageSize,
  run: load,
  handlePageChange
} = useAsyncPage(async () => {
  const params: any = { page: page.value, page_size: pageSize.value }
  if (filters.specialty_id) params.specialty_id = filters.specialty_id
  if (filters.region) params.region = filters.region
  if (filters.salary_min != null) params.salary_min = filters.salary_min
  if (filters.salary_max != null) params.salary_max = filters.salary_max
  if (filters.experience) params.experience = filters.experience
  const res = await jobApi.listPublicJobs(params)
  items.value = res?.items || []
  total.value = res?.total || 0
})

onMounted(async () => {
  try {
    const tree: any = await trainingApi.getCatalogTree()
    specialties.value = tree?.specialties || []
  } catch {}
  load()
})
</script>