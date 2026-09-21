<template>
  <div class="flex flex-col gap-4">
    <h1 class="text-xl font-bold text-ink">职位广场</h1>

    <div class="rounded-card border border-line bg-panel p-4 flex flex-wrap gap-2">
      <el-select v-model="filters.position_id" clearable placeholder="岗位" class="!w-40" @change="applyFilters">
        <el-option v-for="p in positions" :key="p.position_id" :label="p.name" :value="p.position_id" />
      </el-select>
      <el-input v-model="filters.region" placeholder="地区" clearable class="!w-32" @change="applyFilters" />
      <el-input v-model.number="filters.salary_min" placeholder="最低薪资" type="number" clearable class="!w-28" @change="applyFilters" />
      <el-input v-model.number="filters.salary_max" placeholder="最高薪资" type="number" clearable class="!w-28" @change="applyFilters" />
      <el-input v-model="filters.experience" placeholder="经验要求" clearable class="!w-28" @change="applyFilters" />
    </div>

    <!-- 空态 / 错误态（+retry）/ 首屏骨架：判据与互斥由 useAsyncPage + UiAsyncSection 负责（#1101） -->
    <UiAsyncSection
      :error="loadError"
      :loading="loading"
      :empty="isEmpty"
      :retrying="retrying"
      error-title="职位加载失败"
      error-description="网络或服务端异常，可重试"
      @retry="handleRetry"
    >
      <template #empty>
        <UiEmptyState description="暂无招聘中的职位" />
      </template>

      <!-- 首屏才骨架（追加时旧列表原地保持——原 loading && items.length === 0 的口径） -->
      <template #skeleton>
        <UiSkeleton v-if="items.length === 0" variant="list" :count="4" />
      </template>

      <!-- #493：响应式方形网格（手机 1 列 → 平板 2-3 列 → 桌面 4 列）。
           追加走 loadMore，run 不覆盖已累积条目，翻页时旧列表原地保持。 -->
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
      <router-link
        v-for="item in items"
        :key="String(item.id)"
        :to="href('JobDetail', { id: String(item.id) })"
        class="flex aspect-[4/3] flex-col rounded-card border border-line bg-panel p-4 hover:border-ui-200 hover:shadow-card transition-colors"
      >
        <div class="flex items-start justify-between gap-2">
          <div class="min-w-0 flex-1 text-sm font-semibold text-ink line-clamp-1">{{ item.title }}</div>
          <!-- #488：状态角标 -->
          <UiTag v-if="item.apply_state === 'applied'" tone="success" size="small">已投递</UiTag>
          <UiTag v-else-if="item.apply_state === 'not_hired'" tone="danger" size="small">未录用</UiTag>
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
    </UiAsyncSection>

    <!-- 加载更多（#493）：每批 20，不足一批即到底 -->
    <div v-if="hasMore" class="flex justify-center">
      <UiButton :loading="loadingMore" @click="loadMore">加载更多</UiButton>
    </div>
    <div v-else-if="items.length > 0" class="text-center text-xs text-ink-3 pb-2">没有更多了</div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { href } from '@/config/pages'
import { jobApi, type JobPosting } from '@/api/job'
import { positionApi } from '@/api/position'
import { useAsyncPage } from '@/composables/useAsyncPage'
import UiButton from '@/components/ui/UiButton.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiAsyncSection from '@/components/ui/UiAsyncSection.vue'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiTag from '@/components/ui/UiTag.vue'

const items = ref<JobPosting[]>([])
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

/** 生效筛选快照（#1101）：控件 @change 时快照一次，由 filterDeps 触发重置重装。 */
const applied = reactive({ ...filters })

function applyFilters(): void {
  Object.assign(applied, filters)
}

function buildParams(page: number) {
  const params: any = { page, page_size: BATCH }
  if (applied.position_id) params.position_id = applied.position_id
  if (applied.region) params.region = applied.region
  if (applied.salary_min != null) params.salary_min = applied.salary_min
  if (applied.salary_max != null) params.salary_max = applied.salary_max
  if (applied.experience) params.experience = applied.experience
  return params
}

// 招聘域不受证件过滤（client.ts 注入豁免同口径），不随切换重置 load-more 累积列表（#604 opt-out）。
// 追加式分页（#493）+「筛选变化 → 清空累积并回第 1 批」都收在 useAsyncPage（#1101）：
// filterDeps 任一轴变化即清空 + 回第 1 批 + 重装，页面不再手写 resetAndLoad / loadMore / hasMore。
const {
  loading,
  loadError,
  retrying,
  retry: handleRetry,
  isEmpty,
  hasMore,
  loadingMore,
  loadMore,
  run: load
} = useAsyncPage(
  async (page) => jobApi.listPublicJobs(buildParams(page ?? 1)),
  {
    credentialScoped: false,
    mode: 'append',
    batchSize: BATCH,
    itemsRef: items,
    // 生效的筛选快照：控件 @change（失焦/回车/选中）时更新一次，filterDeps 看它，
    // 于是「筛选变化 → 清空累积 + 回第 1 批」成为声明式单点，且不多发半成品请求。
    filterDeps: [
      () => applied.position_id,
      () => applied.region,
      () => applied.salary_min,
      () => applied.salary_max,
      () => applied.experience
    ]
  }
)

onMounted(async () => {
  try {
    const res = await positionApi.listPublic({ silent: true })
    positions.value = res?.positions || []
  } catch {}
  load()
})
</script>
