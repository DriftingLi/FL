<template>
  <div class="flex flex-col gap-4">
    <h1 class="text-xl font-bold text-ink">职位广场</h1>

    <div class="rounded-card border border-line bg-panel p-4 flex flex-wrap gap-2">
      <el-select v-model="filters.position_id" clearable placeholder="岗位" class="!w-40" @change="resetAndLoad">
        <el-option v-for="p in positions" :key="p.position_id" :label="p.name" :value="p.position_id" />
      </el-select>
      <el-input v-model="filters.region" placeholder="地区" clearable class="!w-32" @change="resetAndLoad" />
      <el-input v-model.number="filters.salary_min" placeholder="最低薪资" type="number" clearable class="!w-28" @change="resetAndLoad" />
      <el-input v-model.number="filters.salary_max" placeholder="最高薪资" type="number" clearable class="!w-28" @change="resetAndLoad" />
      <el-input v-model="filters.experience" placeholder="经验要求" clearable class="!w-28" @change="resetAndLoad" />
    </div>

    <UiErrorState
      v-if="loadError"
      title="职位加载失败"
      description="网络或服务端异常，可重试"
      :retrying="retrying"
      @retry="handleRetry"
    />
    <UiSkeleton v-else-if="loading && items.length === 0" variant="list" :count="4" />
    <div v-else-if="items.length === 0" class="rounded-card border border-line bg-panel p-8 text-center text-ink-3">
      暂无招聘中的职位
    </div>

    <!-- #493：响应式方形网格（手机 1 列 → 平板 2-3 列 → 桌面 4 列） -->
    <div v-else class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
      <router-link
        v-for="item in items"
        :key="String(item.id)"
        :to="`/training/jobs/${item.id}`"
        class="flex aspect-[4/3] flex-col rounded-card border border-line bg-panel p-4 hover:border-ui-200 hover:shadow-card transition-colors"
      >
        <div class="flex items-start justify-between gap-2">
          <div class="min-w-0 flex-1 text-sm font-semibold text-ink line-clamp-1">{{ item.title }}</div>
          <!-- #488：状态角标 -->
          <el-tag v-if="item.apply_state === 'applied'" type="success" size="small">已投递</el-tag>
          <el-tag v-else-if="item.apply_state === 'not_hired'" type="danger" size="small">未录用</el-tag>
        </div>
        <div class="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-ink-3">
          <span v-if="item.position_name">{{ item.position_name }}</span>
          <span v-if="item.region">{{ item.region }}</span>
          <span v-if="item.salary_text">{{ item.salary_text }}</span>
          <span v-if="item.experience_req">经验：{{ item.experience_req }}</span>
        </div>
        <div class="mt-auto pt-2">
          <div v-if="item.company_name" class="text-xs text-ink-2 truncate">{{ item.company_name }}</div>
          <div class="mt-1 text-[10px] text-ink-3">发布于 {{ item.published_at.slice(0, 10) }}</div>
        </div>
      </router-link>
    </div>

    <!-- 加载更多（#493）：每批 20，不足一批即到底 -->
    <div v-if="hasMore" class="flex justify-center">
      <UiButton :loading="loadingMore" @click="loadMore">加载更多</UiButton>
    </div>
    <div v-else-if="items.length > 0" class="text-center text-xs text-ink-3 pb-2">没有更多了</div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { jobApi, type JobPosting } from '@/api/job'
import { unwrappedRequest } from '@/api/request'
import { useAsyncPage } from '@/composables/useAsyncPage'
import UiButton from '@/components/ui/UiButton.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'

const items = ref<JobPosting[]>([])
const loadingMore = ref(false)
const hasMore = ref(true)
const BATCH = 20

interface PositionItem {
  position_id: number
  name: string
}

const positions = ref<PositionItem[]>([])

const filters = reactive<{
  position_id: number | null
  region: string
  salary_min: number | null
  salary_max: number | null
  experience: string
}>({
  position_id: null,
  region: '',
  salary_min: null,
  salary_max: null,
  experience: ''
})

function buildParams(page: number) {
  const params: any = { page, page_size: BATCH }
  if (filters.position_id) params.position_id = filters.position_id
  if (filters.region) params.region = filters.region
  if (filters.salary_min != null) params.salary_min = filters.salary_min
  if (filters.salary_max != null) params.salary_max = filters.salary_max
  if (filters.experience) params.experience = filters.experience
  return params
}

const { loading, loadError, retrying, retry: handleRetry, run: load } = useAsyncPage(async () => {
  const res = await jobApi.listPublicJobs(buildParams(1))
  items.value = res?.items || []
  hasMore.value = (res?.items?.length || 0) >= BATCH
})

// #493：筛选变化 → 清空已累积列表并回第一页
function resetAndLoad() {
  items.value = []
  hasMore.value = true
  load()
}

// #493：加载更多（append 第 N+1 批）
async function loadMore() {
  if (loadingMore.value) return
  loadingMore.value = true
  try {
    const nextPage = Math.floor(items.value.length / BATCH) + 1
    const res = await jobApi.listPublicJobs(buildParams(nextPage))
    const batch = res?.items || []
    items.value.push(...batch)
    if (batch.length < BATCH) hasMore.value = false
  } catch {
    /* 拦截器已 toast */
  } finally {
    loadingMore.value = false
  }
}

onMounted(async () => {
  try {
    const res: any = await unwrappedRequest.get('/positions', { headers: { 'X-Silent': '1' } })
    positions.value = res?.positions || []
  } catch {}
  load()
})
</script>
