<template>
  <div class="flex flex-col gap-4">
    <div class="flex items-center justify-between">
      <h1 class="text-xl font-bold text-ink">简历库</h1>
      <UiButton size="small" :loading="loading" @click="reset">刷新</UiButton>
    </div>

    <div class="rounded-card border border-line bg-panel p-4 flex flex-wrap gap-2">
      <el-cascader
        v-model="filters.region_path"
        :options="regionOptions"
        :props="{ value: 'label', label: 'label', children: 'children' }"
        placeholder="意向地区（市）"
        clearable
        class="!w-44"
        @change="applyFilters"
      />
      <el-select v-model="filters.position_id" clearable placeholder="期望岗位" class="!w-36" @change="applyFilters">
        <!-- #492：期望岗位选项与学员简历同源（positions 岗位字典），参数 position_id -->
        <el-option v-for="p in positions" :key="p.position_id" :label="p.name" :value="p.position_id" />
      </el-select>
      <el-select v-model="filters.credential_id" clearable placeholder="证书" class="!w-36" @change="applyFilters">
        <el-option v-for="c in credentials" :key="c.id" :label="c.name" :value="c.id" />
      </el-select>
      <el-input v-model.number="filters.salary_min" placeholder="最低薪资" type="number" clearable class="!w-28" @change="applyFilters" />
      <el-input v-model.number="filters.salary_max" placeholder="最高薪资" type="number" clearable class="!w-28" @change="applyFilters" />
      <el-select v-model="filters.experience_min" clearable placeholder="经验年限" class="!w-32" @change="applyFilters">
        <!-- #492：经验年限档位「N 年及以上」（后端 >= 匹配） -->
        <el-option v-for="n in experienceOptions" :key="String(n)" :label="`${n}年及以上`" :value="n" />
      </el-select>
      <el-select v-model="filters.job_nature" clearable placeholder="用工性质" class="!w-32" @change="applyFilters">
        <!-- #492：新增用工性质筛选 -->
        <el-option label="全职" value="fulltime" />
        <el-option label="兼职" value="parttime" />
        <el-option label="合同" value="contract" />
      </el-select>
      <el-select v-model="filters.available_in" clearable placeholder="到岗时间" class="!w-32" @change="applyFilters">
        <el-option label="随时" value="immediate" />
        <el-option label="1周内" value="1w" />
        <el-option label="2周内" value="2w" />
        <el-option label="1月内" value="1m" />
      </el-select>
    </div>

    <!-- 空态 / 错误态（+retry）/ 首屏骨架：判据与互斥由 useAsyncPage + UiAsyncSection 负责（#1101） -->
    <UiAsyncSection
      :error="loadError"
      :loading="loading"
      :empty="isEmpty"
      :retrying="retrying"
      error-title="简历加载失败"
      error-description="网络或服务端异常，可重试"
      @retry="handleRetry"
    >
      <template #empty>
        <UiEmptyState description="暂无公开简历" />
      </template>

      <!-- 首屏才骨架（追加时旧列表原地保持——原 loading && items.length === 0 口径） -->
      <template #skeleton>
        <UiSkeleton v-if="items.length === 0" variant="list" :count="4" />
      </template>

      <!-- #493：响应式方形网格（手机 1 列 → 平板 2-3 列 → 桌面 4 列）；卡面仅核心字段 -->
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
      <div
        v-for="{ item, badge, avail } in cards"
        :key="String(item.user_id)"
        class="flex aspect-[4/3] flex-col rounded-card border border-line bg-panel p-4 transition-colors hover:border-ui-200 hover:shadow-card"
      >
        <div class="flex items-center justify-between gap-2">
          <div class="min-w-0 flex-1 truncate text-sm font-semibold text-ink">{{ item.real_name || item.real_name_masked || '匿名学员' }}</div>
          <!-- #489：联系状态角标——label/tone 来自联络授权 descriptor 单点（#1103） -->
          <UiTag v-if="badge" :tone="badge.tone" size="small">{{ badge.label }}</UiTag>
        </div>
        <div class="mt-1 flex flex-wrap gap-x-2 gap-y-1 text-xs text-ink-3">
          <!-- #1267：徽章说「已同意」而明文已收回时，这一格负责不让角标单独说谎 -->
          <UiTag v-if="avail" :tone="avail.tone" size="small">{{ avail.label }}</UiTag>
          <span v-if="item.expected_position_extra">{{ item.expected_position_extra }}</span>
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
            :to="href('RecruitResumeDetail', { id: String(item.user_id) })"
            class="shrink-0 text-xs font-medium text-ui-600 hover:text-ui-700"
          >
            查看详情
          </router-link>
        </div>
      </div>
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
import { ref, reactive, computed, onMounted } from 'vue'
import { href } from '@/config/pages'
import { recruitApi, type RecruitResumeItem } from '@/api/recruit'
import { companyAvailability, contactBadge } from '@/utils/contactRequestStatus'
import { buildCityLevelRegionOptions, joinRegionPath } from '@/utils/region'
import { positionApi } from '@/api/position'
import { credentialApi } from '@/api/credential'
import { useAsyncPage } from '@/composables/useAsyncPage'
import UiButton from '@/components/ui/UiButton.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiAsyncSection from '@/components/ui/UiAsyncSection.vue'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiTag from '@/components/ui/UiTag.vue'

const BATCH = 20
const items = ref<RecruitResumeItem[]>([])

// 卡片 = 简历 + 其联系状态角标 + 「授权在而明文不可用」那一格（两者的文案与 tone 都只出自
// descriptor 单点，页面不自写状态词）。徽章不因企业被停用而降级：那是两格正交事实
// （ADR-0064 决策 5 / 移动端 #1267）。
const cards = computed(() =>
  items.value.map((item) => ({
    item,
    badge: contactBadge(item.contact_state),
    avail: companyAvailability(item.company_disabled)
  }))
)

// 筛选轴（#1101）：直接以 getter 形态喂给 useAsyncPage 的 filterDeps，
// 任一轴变化即「清空累积 + 回第 1 批重装」，不再靠每个控件的 @change 回调兜底。
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

/** 生效筛选快照（#1101）：控件 @change 时快照一次，由 filterDeps 触发重置重装。 */
const applied = reactive({ ...filters })

function applyFilters(): void {
  Object.assign(applied, filters)
}

function buildParams(page: number) {
  const params: any = { page, page_size: BATCH }
  if (applied.region_path.length) params.region = joinRegionPath(applied.region_path)
  if (applied.position_id) params.position_id = applied.position_id
  if (applied.credential_id) params.credential_id = applied.credential_id
  if (applied.salary_min != null) params.salary_min = applied.salary_min
  if (applied.salary_max != null) params.salary_max = applied.salary_max
  if (applied.experience_min != null) params.experience_min = applied.experience_min
  if (applied.job_nature) params.job_nature = applied.job_nature
  if (applied.available_in) params.available_in = applied.available_in
  return params
}

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
  reset,
  run: load
} = useAsyncPage(
  async (page) => recruitApi.listResumes(buildParams(page ?? 1)),
  {
    mode: 'append',
    batchSize: BATCH,
    itemsRef: items,
    // 生效的筛选快照：控件 @change（失焦/回车/选中）时更新一次，filterDeps 看它，
    // 于是「筛选变化 → 清空累积 + 回第 1 批」成为声明式单点，且不多发半成品请求。
    filterDeps: [
      () => applied.region_path,
      () => applied.position_id,
      () => applied.credential_id,
      () => applied.salary_min,
      () => applied.salary_max,
      () => applied.experience_min,
      () => applied.job_nature,
      () => applied.available_in
    ]
  }
)

// #492：期望岗位选项与简历编辑同源（/positions 岗位字典）
const positions = ref<any[]>([])
const credentials = ref<any[]>([])
// #486：地区筛选项与录入同源（省→市两级级联，value 取 label），参数传市级
const regionOptions = buildCityLevelRegionOptions()
// #492：经验档位「N 年及以上」
const experienceOptions = [1, 3, 5, 10]

async function loadMeta() {
  try {
    const res = await positionApi.listPublic()
    if (res?.positions) positions.value = res.positions
  } catch {}
  try {
    const res = await credentialApi.listCredentials()
    if (res?.credentials) credentials.value = res.credentials
  } catch {}
}

onMounted(() => {
  loadMeta()
  load()
})
</script>
