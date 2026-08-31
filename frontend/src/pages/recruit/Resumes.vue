<template>
  <div class="flex flex-col gap-4">
    <div class="flex items-center justify-between">
      <h1 class="text-xl font-bold text-ink">简历库</h1>
      <UiButton size="small" :loading="loading" @click="load">刷新</UiButton>
    </div>

    <div class="rounded-card border border-line bg-panel p-4 flex flex-wrap gap-2">
      <el-select v-model="filters.region" clearable placeholder="意向地区" class="!w-32" @change="load">
        <el-option v-for="r in regionOptions" :key="r" :label="r" :value="r" />
      </el-select>
      <el-select v-model="filters.specialty_id" clearable placeholder="期望岗位" class="!w-36" @change="load">
        <el-option v-for="s in specialties" :key="s.specialty_id" :label="s.name" :value="s.specialty_id" />
      </el-select>
      <el-select v-model="filters.credential_id" clearable placeholder="证书" class="!w-36" @change="load">
        <el-option v-for="c in credentials" :key="c.id" :label="c.name" :value="c.id" />
      </el-select>
      <el-input v-model.number="filters.salary_min" placeholder="最低薪资" type="number" clearable class="!w-28" @change="load" />
      <el-input v-model.number="filters.salary_max" placeholder="最高薪资" type="number" clearable class="!w-28" @change="load" />
      <el-select v-model="filters.experience_years" clearable placeholder="经验年限" class="!w-28" @change="load">
        <el-option v-for="n in experienceOptions" :key="String(n)" :label="`${n}年`" :value="n" />
      </el-select>
      <el-select v-model="filters.available_in" clearable placeholder="到岗时间" class="!w-32" @change="load">
        <el-option label="随时" value="immediate" />
        <el-option label="1周内" value="1w" />
        <el-option label="2周内" value="2w" />
        <el-option label="1月内" value="1m" />
      </el-select>
    </div>

    <div v-if="loading" class="rounded-card border border-line bg-panel p-8 text-center text-ink-3">加载中...</div>
    <div v-else-if="error" class="rounded-card border border-line bg-panel p-8 text-center">
      <p class="text-sm text-ink-2">{{ error }}</p>
      <UiButton class="mt-3" size="small" @click="load">重试</UiButton>
    </div>
    <div v-else-if="items.length === 0" class="rounded-card border border-line bg-panel p-8 text-center text-ink-3">
      暂无公开简历
    </div>
    <div v-else class="grid gap-3">
      <div
        v-for="item in items"
        :key="String(item.user_id)"
        class="rounded-card border border-line bg-panel p-4 hover:border-ui-200 transition-colors"
      >
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0 flex-1">
            <div class="text-sm font-semibold text-ink">{{ item.real_name || item.real_name_masked || '匿名学员' }}</div>
            <div class="mt-1 flex flex-wrap gap-x-2 gap-y-1 text-xs text-ink-3">
              <span v-if="item.expected_specialty_extra">{{ item.expected_specialty_extra }}</span>
              <span v-if="item.expected_regions && item.expected_regions.length">意向：{{ (item.expected_regions as any).join('、') }}</span>
              <span v-if="item.salary_negotiable">薪资面议</span>
              <span v-else-if="item.salary_min != null || item.salary_max != null">薪资：{{ item.salary_min ?? '-' }}-{{ item.salary_max ?? '-' }}</span>
              <span>{{ item.experience_years }}年经验</span>
              <span v-if="item.available_in">到岗：{{ availableLabel(item.available_in) }}</span>
              <span v-if="item.job_nature">{{ jobNatureLabel(item.job_nature) }}</span>
            </div>
            <div v-if="item.self_intro" class="mt-2 text-xs text-ink-2 line-clamp-2">{{ item.self_intro }}</div>
            <div class="mt-1 flex flex-wrap gap-1">
              <span v-for="cert in (item.resume_certifications || [])" :key="JSON.stringify(cert)" class="rounded bg-ui-50 px-1.5 py-0.5 text-[10px] text-ink-3">{{ certTag(cert) }}</span>
            </div>
            <div class="mt-1 text-[10px] text-ink-3">更新于 {{ item.updated_at }}</div>
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

    <div v-if="total > 0" class="text-xs text-ink-3 text-center">共 {{ total }} 份</div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, watch } from 'vue'
import { recruitApi, type RecruitResumeItem } from '@/api/recruit'
import { unwrappedRequest } from '@/api/request'
import UiButton from '@/components/ui/UiButton.vue'

const loading = ref(false)
const error = ref('')
const items = ref<RecruitResumeItem[]>([])
const total = ref(0)

const specialties = ref<any[]>([])
const credentials = ref<any[]>([])
const regionOptions = ref<string[]>(['江苏苏州', '浙江杭州', '江苏南京', '上海', '北京'])
const experienceOptions = [0, 1, 2, 3, 5, 10]

const filters = reactive<{
  region: string
  specialty_id: number | null
  credential_id: number | null
  salary_min: number | null
  salary_max: number | null
  experience_years: number | null
  available_in: string
}>({
  region: '',
  specialty_id: null,
  credential_id: null,
  salary_min: null,
  salary_max: null,
  experience_years: null,
  available_in: ''
})

function availableLabel(v: string) {
  const m: Record<string, string> = { immediate: '随时', '1w': '1周内', '2w': '2周内', '1m': '1月内' }
  return m[v] || v
}
function jobNatureLabel(v: string) {
  const m: Record<string, string> = { fulltime: '全职', parttime: '兼职', contract: '合同' }
  return m[v] || v
}
function certTag(c: any) {
  if (!c || typeof c !== 'object') return ''
  return c.credential_id ? `持证#${c.credential_id}` : c.cert_no || '持证'
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const params: any = { page: 1, page_size: 20 }
    if (filters.region) params.region = filters.region
    if (filters.specialty_id) params.specialty_id = filters.specialty_id
    if (filters.credential_id) params.credential_id = filters.credential_id
    if (filters.salary_min != null) params.salary_min = filters.salary_min
    if (filters.salary_max != null) params.salary_max = filters.salary_max
    if (filters.experience_years != null) params.experience_years = filters.experience_years
    if (filters.available_in) params.available_in = filters.available_in
    const res = await recruitApi.listResumes(params)
    items.value = res?.items || []
    total.value = res?.total || 0
  } catch (e: any) {
    error.value = e?.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function loadMeta() {
  try {
    const res: any = await unwrappedRequest.get('/catalog/tree')
    if (res?.specialties) specialties.value = res.specialties
  } catch {}
  try {
    const res: any = await unwrappedRequest.get('/credentials')
    if (res?.credentials) credentials.value = res.credentials
  } catch {}
}

watch(
  () => [filters.region, filters.specialty_id, filters.credential_id, filters.available_in],
  () => {}
)

onMounted(() => {
  loadMeta()
  load()
})
</script>
