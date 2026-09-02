<template>
  <div class="flex flex-col gap-4">
    <div class="flex items-center justify-between">
      <h1 class="text-xl font-bold text-ink">简历库</h1>
      <UiButton size="small" :loading="loading" @click="resetAndLoad">刷新</UiButton>
    </div>

    <div class="rounded-card border border-line bg-panel p-4 flex flex-wrap gap-2">
      <el-cascader
        v-model="filters.region_path"
        :options="regionOptions"
        :props="{ value: 'label', label: 'label', children: 'children' }"
        placeholder="意向地区（市）"
        clearable
        class="!w-44"
        @change="resetAndLoad"
      />
      <el-select v-model="filters.position_id" clearable placeholder="期望岗位" class="!w-36" @change="resetAndLoad">
        <!-- #492：期望岗位选项与学员简历同源（positions 岗位字典），参数 position_id -->
        <el-option v-for="p in positions" :key="p.position_id" :label="p.name" :value="p.position_id" />
      </el-select>
      <el-select v-model="filters.credential_id" clearable placeholder="证书" class="!w-36" @change="resetAndLoad">
        <el-option v-for="c in credentials" :key="c.id" :label="c.name" :value="c.id" />
      </el-select>
      <el-input v-model.number="filters.salary_min" placeholder="最低薪资" type="number" clearable class="!w-28" @change="resetAndLoad" />
      <el-input v-model.number="filters.salary_max" placeholder="最高薪资" type="number" clearable class="!w-28" @change="resetAndLoad" />
      <el-select v-model="filters.experience_min" clearable placeholder="经验年限" class="!w-32" @change="resetAndLoad">
        <!-- #492：经验年限档位「N 年及以上」（后端 >= 匹配） -->
        <el-option v-for="n in experienceOptions" :key="String(n)" :label="`${n}年及以上`" :value="n" />
      </el-select>
      <el-select v-model="filters.job_nature" clearable placeholder="用工性质" class="!w-32" @change="resetAndLoad">
        <!-- #492：新增用工性质筛选 -->
        <el-option label="全职" value="fulltime" />
        <el-option label="兼职" value="parttime" />
        <el-option label="合同" value="contract" />
      </el-select>
      <el-select v-model="filters.available_in" clearable placeholder="到岗时间" class="!w-32" @change="resetAndLoad">
        <el-option label="随时" value="immediate" />
        <el-option label="1周内" value="1w" />
        <el-option label="2周内" value="2w" />
        <el-option label="1月内" value="1m" />
      </el-select>
    </div>

    <UiErrorState
      v-if="loadError"
      title="简历加载失败"
      description="网络或服务端异常，可重试"
      :retrying="retrying"
      @retry="handleRetry"
    />
    <UiSkeleton v-else-if="loading && items.length === 0" variant="list" :count="4" />
    <div v-else-if="items.length === 0" class="rounded-card border border-line bg-panel p-8 text-center text-ink-3">
      暂无公开简历
    </div>

    <!-- #493：响应式方形网格（手机 1 列 → 平板 2-3 列 → 桌面 4 列）；卡面仅核心字段 -->
    <div v-else class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
      <div
        v-for="item in items"
        :key="String(item.user_id)"
        class="flex aspect-[4/3] flex-col rounded-card border border-line bg-panel p-4 transition-colors hover:border-ui-200 hover:shadow-card"
      >
        <div class="flex items-center justify-between gap-2">
          <div class="min-w-0 flex-1 truncate text-sm font-semibold text-ink">{{ item.real_name || item.real_name_masked || '匿名学员' }}</div>
          <!-- #489：联系状态角标 -->
          <el-tag v-if="item.contact_state === 'approved'" type="success" size="small">已授权</el-tag>
          <el-tag v-else-if="item.contact_state === 'pending'" type="warning" size="small">待学员确认</el-tag>
        </div>
        <div class="mt-1 flex flex-wrap gap-x-2 gap-y-1 text-xs text-ink-3">
          <span v-if="item.expected_specialty_extra">{{ item.expected_specialty_extra }}</span>
          <span v-if="item.expected_regions && item.expected_regions.length">意向：{{ (item.expected_regions as any).join('、') }}</span>
          <span v-if="item.salary_negotiable">薪资面议</span>
          <span v-else-if="item.salary_min != null || item.salary_max != null">薪资：{{ item.salary_min ?? '-' }}-{{ item.salary_max ?? '-' }}</span>
          <span>{{ item.experience_years }}年经验</span>
        </div>
        <div class="mt-auto flex items-end justify-between gap-2 pt-2">
          <div class="min-w-0">
            <div class="text-[10px] text-ink-3">更新于 {{ item.updated_at.slice(0, 10) }}</div>
          </div>
          <router-link
            :to="`/recruit/resumes/${item.user_id}`"
            class="shrink-0 text-xs font-medium text-ui-600 hover:text-ui-700"
          >
            查看详情
          </router-link>
        </div>
      </div>
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
import { recruitApi, type RecruitResumeItem } from '@/api/recruit'
import { buildCityLevelRegionOptions, joinRegionPath } from '@/utils/region'
import { unwrappedRequest } from '@/api/request'
import { useAsyncPage } from '@/composables/useAsyncPage'
import UiButton from '@/components/ui/UiButton.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'

const BATCH = 20
const items = ref<RecruitResumeItem[]>([])
const loadingMore = ref(false)
const hasMore = ref(true)

function buildParams(page: number) {
  const params: any = { page, page_size: BATCH }
  if (filters.region_path.length) params.region = joinRegionPath(filters.region_path)
  if (filters.position_id) params.position_id = filters.position_id
  if (filters.credential_id) params.credential_id = filters.credential_id
  if (filters.salary_min != null) params.salary_min = filters.salary_min
  if (filters.salary_max != null) params.salary_max = filters.salary_max
  if (filters.experience_min != null) params.experience_min = filters.experience_min
  if (filters.job_nature) params.job_nature = filters.job_nature
  if (filters.available_in) params.available_in = filters.available_in
  return params
}

const { loading, loadError, retrying, retry: handleRetry, run: load } = useAsyncPage(async () => {
  const res = await recruitApi.listResumes(buildParams(1))
  items.value = res?.items || []
  hasMore.value = (res?.items?.length || 0) >= BATCH
})

// #493：筛选变化 → 清空累积并回第一批
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
    const res = await recruitApi.listResumes(buildParams(nextPage))
    const batch = res?.items || []
    items.value.push(...batch)
    if (batch.length < BATCH) hasMore.value = false
  } catch {
    /* 拦截器已 toast */
  } finally {
    loadingMore.value = false
  }
}

// #492：期望岗位选项与简历编辑同源（/positions 岗位字典）
const positions = ref<any[]>([])
const credentials = ref<any[]>([])
// #486：地区筛选项与录入同源（省→市两级级联，value 取 label），参数传市级
const regionOptions = buildCityLevelRegionOptions()
// #492：经验档位「N 年及以上」
const experienceOptions = [1, 3, 5, 10]

const filters = reactive<{
  region_path: string[]
  position_id: number | null
  credential_id: number | null
  salary_min: number | null
  salary_max: number | null
  experience_min: number | null
  job_nature: string
  available_in: string
}>({
  region_path: [],
  position_id: null,
  credential_id: null,
  salary_min: null,
  salary_max: null,
  experience_min: null,
  job_nature: '',
  available_in: ''
})

async function loadMeta() {
  try {
    const res: any = await unwrappedRequest.get('/positions')
    if (res?.positions) positions.value = res.positions
  } catch {}
  try {
    const res: any = await unwrappedRequest.get('/credentials')
    if (res?.credentials) credentials.value = res.credentials
  } catch {}
}

onMounted(() => {
  loadMeta()
  load()
})
</script>
