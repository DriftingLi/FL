<script setup lang="ts">
// 残值评估配置管理（管理员）
// 重构说明：从 15 tab 缩减为 2 tab
//   Tab 1 原价表：CRUD original-prices（学生端表单依赖该表数据）
//   Tab 2 算法参数：聚合展示 5 类参数（全局系数 / 品牌系数 / 车况系数 / 车况修正项 / 区域系数）
//                  每类独立保存，仅提交变更项（dirty 检测），不提供新增/删除
// 000015：新增"车况修正项"区，按 key 前缀 kc_ 过滤 coefficient_configs 行单独展示
//
// 列表档位：useAdminTable（分页列表）—— 原价表（无分页、一次拉全量，本地筛选走 computed）
// 列表档位：useAsyncPage（只读计数）—— 算法参数聚合装载（草稿供 useDirtyDraft，五分区保存侧不走本档）
// 第十二波票 3（#1168，ADR-0056 §9）：两档归位——读面/错误态收进档位（此前 useCrudTable 与
// 手写 loadAlgorithmParams 各吞一份错，DB 故障渲染成「空表」）；弹窗 CRUD 交互与
// useDirtyDraft 五态（dirty 检测/仅存变更/重置/保存 loading/成功 reload）是页面专属，留页面。
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Plus, Refresh, Check, RefreshLeft, ArrowDown } from '@element-plus/icons-vue'
import PageHeader from '@/components/valuation/PageHeader.vue'
import {
  adminResources,
  listAlgorithmParameters,
  updateCoefficient,
  updateBrandCoefficient,
  updateConditionCoefficient,
  updateRegionCoefficient,
  type AdminRow,
  type AdminResourceId,
  type AlgorithmParameters
} from '@/api/valuation/admin'
import type { CoefficientConfig } from '@/types/valuation/evaluation'
import { useAdminTable } from '@/composables/useAdminTable'
import { useAsyncPage } from '@/composables/useAsyncPage'
import { useConfirm } from '@/composables/useConfirm'
import { useDirtyDraft } from '@/composables/useDirtyDraft'
import UiButton from '@/components/ui/UiButton.vue'
import UiFilterBar from '@/components/ui/UiFilterBar.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiAsyncSection from '@/components/ui/UiAsyncSection.vue'
import UiSwitch from '@/components/ui/UiSwitch.vue'

// ========== Tab 1: 原价表 ==========
interface OriginalPriceField {
  prop: string
  label: string
  type: 'input' | 'number' | 'switch'
  required?: boolean
  width?: number
  defaultValue?: string | number | boolean
}

const ORIGINAL_PRICE_FIELDS: OriginalPriceField[] = [
  { prop: 'brand', label: '品牌', type: 'input', required: true, width: 120 },
  { prop: 'vehicle_type', label: '车辆类型', type: 'input', required: true, width: 120 },
  { prop: 'series', label: '系列', type: 'input', width: 100 },
  { prop: 'tonnage', label: '吨位', type: 'number', width: 80 },
  { prop: 'config_type', label: '配置类型', type: 'input', width: 150, defaultValue: '无' },
  { prop: 'mast_type', label: '门架类型', type: 'input', width: 100, defaultValue: '无' },
  { prop: 'mast_height_mm', label: '门架高度(mm)', type: 'number', width: 120 },
  { prop: 'earliest_factory_year', label: '最早出厂年份', type: 'number', required: true, width: 120, defaultValue: 2000 },
  { prop: 'original_price', label: '原价（万元）', type: 'number', required: true, width: 120 }
]

// 档位一：列表装载与错误/重试态走 useAdminTable（吞错渲染成空表的旧形态随 useCrudTable 一起退役）
const {
  loading: originalPriceLoading,
  loadError: originalPriceLoadError,
  retrying: originalPriceRetrying,
  retry: retryOriginalPrices,
  list: originalPriceList,
  load: loadOriginalPrices
} = useAdminTable<AdminRow>({
  fetch: async () => {
    const rows = await adminResources.originalPrices.list()
    return { list: rows, total: rows.length }
  }
})

// 弹窗态与提交是页面专属（九列动态表单由 ORIGINAL_PRICE_FIELDS 驱动）
const dialogVisible = ref(false)
const dialogTitle = ref('')
const editingRow = ref<AdminRow | null>(null)
const formData = reactive<Record<string, any>>({})
const submitting = ref(false)

function resetForm() {
  Object.keys(formData).forEach(k => delete formData[k])
}

function openCreate() {
  editingRow.value = null
  dialogTitle.value = '新增原价记录'
  resetForm()
  for (const f of ORIGINAL_PRICE_FIELDS) {
    formData[f.prop] =
      f.defaultValue !== undefined ? f.defaultValue : f.type === 'switch' ? true : f.type === 'number' ? 0 : ''
  }
  dialogVisible.value = true
}

function openEdit(row: AdminRow) {
  editingRow.value = row
  dialogTitle.value = '编辑原价记录'
  resetForm()
  Object.assign(formData, row)
  dialogVisible.value = true
}

async function handleSubmit() {
  for (const f of ORIGINAL_PRICE_FIELDS) {
    if (f.required) {
      const v = formData[f.prop]
      if (v == null || v === '') {
        ElMessage.warning(`请填写${f.label}`)
        return
      }
    }
  }
  submitting.value = true
  try {
    const payload: Record<string, unknown> = { ...formData }
    const id: AdminResourceId | null | undefined = adminResources.originalPrices.getIdOf(editingRow.value)
    if (id != null) {
      await adminResources.originalPrices.update(id, payload)
      ElMessage.success('更新成功')
    } else {
      await adminResources.originalPrices.create(payload)
      ElMessage.success('创建成功')
    }
    dialogVisible.value = false
    await loadOriginalPrices()
  } catch {
    // 拦截器已提示
  } finally {
    submitting.value = false
  }
}

async function handleDelete(row: AdminRow) {
  const id = adminResources.originalPrices.getIdOf(row)
  if (id == null) return
  try {
    await useConfirm().confirmDanger('确定删除该原价记录？', '删除确认')
  } catch {
    return
  }
  try {
    await adminResources.originalPrices.remove(id)
    ElMessage.success('已删除')
    await loadOriginalPrices()
  } catch {
    // 拦截器已提示
  }
}

// 操作下拉菜单统一入口（原价表）
function handleAction(cmd: string, row: AdminRow) {
  switch (cmd) {
    case 'edit':
      openEdit(row)
      break
    case 'delete':
      handleDelete(row)
      break
  }
}

// 原价表筛选（本地过滤）
const originalPriceFilter = reactive({
  brand: '',
  vehicle_type: '',
  series: '',
  config_type: ''
})

const filteredOriginalPrices = computed(() => {
  const rows = originalPriceList.value
  const brand = originalPriceFilter.brand.trim()
  const vehicleType = originalPriceFilter.vehicle_type.trim()
  const series = originalPriceFilter.series.trim()
  const configType = originalPriceFilter.config_type.trim()
  if (!brand && !vehicleType && !series && !configType) return rows
  return rows.filter((row) => {
    const matchBrand = !brand || String(row.brand ?? '').toLowerCase().includes(brand.toLowerCase())
    const matchVehicleType =
      !vehicleType || String(row.vehicle_type ?? '').toLowerCase().includes(vehicleType.toLowerCase())
    const matchSeries = !series || String(row.series ?? '').toLowerCase().includes(series.toLowerCase())
    const matchConfigType =
      !configType || String(row.config_type ?? '').toLowerCase().includes(configType.toLowerCase())
    return matchBrand && matchVehicleType && matchSeries && matchConfigType
  })
})

function resetOriginalPriceFilter() {
  originalPriceFilter.brand = ''
  originalPriceFilter.vehicle_type = ''
  originalPriceFilter.series = ''
  originalPriceFilter.config_type = ''
}

// ========== Tab 2: 算法参数 ==========
type CoeffRow = CoefficientConfig
interface BrandRow {
  id: number
  name: string
  k_brand: number
  is_active: boolean
}
interface ConditionRatingRow {
  id: number
  rating: string
  label: string
  base_coefficient: number
}
interface RegionCoefficientRow {
  id: number
  province: string
  city: string
  coefficient: number
}

// coefficients 共享一条 draft：global 与 kcModifiers 按 key 前缀 kc_ 派生
const isKc = (c: CoeffRow) => c.key.startsWith('kc_')
const isGlobal = (c: CoeffRow) => !c.key.startsWith('kc_')

const coefficients = useDirtyDraft<CoeffRow>({
  identity: (c) => c.key,
  equals: (a, b) => a.key === b.key && a.value === b.value
})
const brands = useDirtyDraft<BrandRow>({
  identity: (b) => b.id,
  equals: (a, b) => a.id === b.id && a.k_brand === b.k_brand && a.is_active === b.is_active
})
const conditionRatings = useDirtyDraft<ConditionRatingRow>({
  identity: (c) => c.id,
  equals: (a, b) => a.id === b.id && a.label === b.label && a.base_coefficient === b.base_coefficient
})
const regionCoefficients = useDirtyDraft<RegionCoefficientRow>({
  identity: (r) => r.id,
  equals: (a, b) => a.id === b.id && a.coefficient === b.coefficient
})

// 模板用的 draft 视图（coefficients 派生 global / kc 两个视图）
const globalCoefficientsDraft = computed(() => coefficients.draft.value.filter(isGlobal))
const kcModifiersDraft = computed(() => coefficients.draft.value.filter(isKc))
const brandsDraft = brands.draft
const conditionRatingsDraft = conditionRatings.draft
const regionCoefficientsDraft = regionCoefficients.draft

// 档位二：算法参数聚合装载走 useAsyncPage（错误/重试上档位通道，不再吞成空草稿）；
// 失败仍清四份 draft（与旧行为一致：错误态与陈旧数据不同屏），保存侧维持 useDirtyDraft 五态。
const {
  loading: algorithmLoading,
  loadError: algorithmLoadError,
  retrying: algorithmRetrying,
  retry: retryAlgorithmLoad,
  run: loadAlgorithmParams
} = useAsyncPage(async () => {
  try {
    const data: AlgorithmParameters = await listAlgorithmParameters()
    coefficients.setAll(data.coefficients)
    brands.setAll(data.brands)
    conditionRatings.setAll(data.condition_ratings)
    regionCoefficients.setAll(data.region_coefficients)
  } catch (e) {
    coefficients.clear()
    brands.clear()
    conditionRatings.clear()
    regionCoefficients.clear()
    throw e
  }
})

// 各分区独立 saving 态，保留各按钮独立 loading 的精确行为
const savingCoefficients = ref(false)
const savingKcModifiers = ref(false)
const savingBrands = ref(false)
const savingConditionRatings = ref(false)
const savingRegionCoefficients = ref(false)

// ----- dirty 检测 -----
// 注意：isCoefficientsDirty 沿用原实现的「全量比较」语义（整条 coefficients 数组任一项变更都点亮全局系数区），
// 而非仅检查 isGlobal 子集，以保持与重构前完全一致的行为。
const isCoefficientsDirty = () => coefficients.isDirty()
const isKcModifiersDirty = () => coefficients.isDirty(isKc)
const isBrandsDirty = () => brands.isDirty()
const isConditionRatingsDirty = () => conditionRatings.isDirty()
const isRegionCoefficientsDirty = () => regionCoefficients.isDirty()

// ----- 保存（仅提交变更项；成功后统一 reload）-----
async function saveCoefficients() {
  savingCoefficients.value = true
  try {
    if (
      await coefficients.save({
        filter: isGlobal,
        persist: (c) => updateCoefficient(c.key, c.value),
        successLabel: (n) => `已保存 ${n} 项全局系数`
      })
    ) {
      await loadAlgorithmParams()
    }
  } finally {
    savingCoefficients.value = false
  }
}

async function saveKcModifiers() {
  savingKcModifiers.value = true
  try {
    if (
      await coefficients.save({
        filter: isKc,
        persist: (c) => updateCoefficient(c.key, c.value),
        successLabel: (n) => `已保存 ${n} 项车况修正项`
      })
    ) {
      await loadAlgorithmParams()
    }
  } finally {
    savingKcModifiers.value = false
  }
}

async function saveBrands() {
  savingBrands.value = true
  try {
    if (
      await brands.save({
        persist: (b) => updateBrandCoefficient(b.id, b.k_brand, b.is_active),
        successLabel: (n) => `已保存 ${n} 项品牌系数`
      })
    ) {
      await loadAlgorithmParams()
    }
  } finally {
    savingBrands.value = false
  }
}

async function saveConditionRatings() {
  savingConditionRatings.value = true
  try {
    if (
      await conditionRatings.save({
        persist: (c) => updateConditionCoefficient(c.id, c.label, c.base_coefficient),
        successLabel: (n) => `已保存 ${n} 项车况系数`,
        validate: (c) => (!c.label || !c.label.trim() ? `评级 ${c.rating} 的中文标签不能为空` : undefined)
      })
    ) {
      await loadAlgorithmParams()
    }
  } finally {
    savingConditionRatings.value = false
  }
}

async function saveRegionCoefficients() {
  savingRegionCoefficients.value = true
  try {
    if (
      await regionCoefficients.save({
        persist: (r) => updateRegionCoefficient(r.id, r.coefficient),
        successLabel: (n) => `已保存 ${n} 项区域系数`
      })
    ) {
      await loadAlgorithmParams()
    }
  } finally {
    savingRegionCoefficients.value = false
  }
}

// ----- 重置 -----
const resetCoefficients = () => coefficients.reset() // 全量重置（含 kc_，与原实现一致）
const resetKcModifiers = () => coefficients.reset(isKc) // 仅重置 kc_ 行
const resetBrands = () => brands.reset()
const resetConditionRatings = () => conditionRatings.reset()
const resetRegionCoefficients = () => regionCoefficients.reset()

// 区域系数新增：调用 POST /admin/region-coefficients
const regionCreateDialogVisible = ref(false)
const creatingRegion = ref(false)
const regionCreateForm = reactive({
  province: '',
  city: '',
  coefficient: 1.0
})

function openCreateRegion() {
  regionCreateForm.province = ''
  regionCreateForm.city = ''
  regionCreateForm.coefficient = 1.0
  regionCreateDialogVisible.value = true
}

async function handleCreateRegion() {
  const province = regionCreateForm.province.trim()
  const city = regionCreateForm.city.trim()
  if (!province) {
    ElMessage.warning('请填写省份')
    return
  }
  if (!city) {
    ElMessage.warning('请填写城市')
    return
  }
  creatingRegion.value = true
  try {
    await adminResources.regionCoefficients.create({
      province,
      city,
      coefficient: regionCreateForm.coefficient
    })
    ElMessage.success('已新增区域系数')
    regionCreateDialogVisible.value = false
    await loadAlgorithmParams()
  } catch {
    // 拦截器已提示
  } finally {
    creatingRegion.value = false
  }
}

// ========== Tab 切换 ==========
const activeTab = ref<string>('originalPrices')
const algorithmLoaded = ref(false)

function onTabChange(name: string) {
  if (name === 'algorithm' && !algorithmLoaded.value) {
    algorithmLoaded.value = true
    loadAlgorithmParams()
  }
}

// 算法参数折叠面板默认全部展开（含 000015 新增的 kcModifiers）
const activeCollapse = ref<string[]>(['coefficients', 'brands', 'condition', 'kcModifiers', 'region'])

onMounted(() => {
  loadOriginalPrices()
})

// 顶部刷新按钮：根据当前 tab 刷新
function onRefresh() {
  if (activeTab.value === 'originalPrices') {
    loadOriginalPrices()
  } else if (activeTab.value === 'algorithm') {
    loadAlgorithmParams()
  }
}
</script>

<template>
  <div class="config-manage valuation-root">
    <div class="app-container">
      <PageHeader title="残值评估配置" subtitle="valuation config">
        <template #actions>
          <UiButton :icon="Refresh" @click="onRefresh">刷新当前</UiButton>
        </template>
      </PageHeader>

      <el-tabs v-model="activeTab" type="border-card" @tab-change="onTabChange">
        <!-- Tab 1: 原价表 -->
        <el-tab-pane label="原价表" name="originalPrices">
          <div class="tab-toolbar">
            <span class="tab-tip">维护叉车基准原价记录（学生端表单级联查询依赖此表）</span>
            <UiButton variant="primary" :icon="Plus" @click="openCreate">新增</UiButton>
          </div>
          <UiFilterBar>
        <template #filters>

            <el-input
              v-model="originalPriceFilter.brand"
              placeholder="筛选品牌"
              clearable
              size="small"
              style="width: 140px"
            />
            <el-input
              v-model="originalPriceFilter.vehicle_type"
              placeholder="筛选车辆类型"
              clearable
              size="small"
              style="width: 140px"
            />
            <el-input
              v-model="originalPriceFilter.series"
              placeholder="筛选系列"
              clearable
              size="small"
              style="width: 120px"
            />
            <el-input
              v-model="originalPriceFilter.config_type"
              placeholder="筛选配置类型"
              clearable
              size="small"
              style="width: 160px"
            />
            <UiButton :icon="RefreshLeft" size="small" @click="resetOriginalPriceFilter">重置筛选</UiButton>
        </template>
      </UiFilterBar>
          <UiAsyncSection
            :error="originalPriceLoadError"
            :loading="originalPriceLoading"
            :retrying="originalPriceRetrying"
            :skeleton="false"
            error-title="原价表加载失败"
            error-description="网络或服务端异常，可重试"
            @retry="retryOriginalPrices"
          >
          <el-table v-loading="originalPriceLoading"
            :data="filteredOriginalPrices"
            stripe
            border
            style="width: 100%">

            <el-table-column
              v-for="col in ORIGINAL_PRICE_FIELDS"
              :key="col.prop"
              :prop="col.prop"
              :label="col.label"
              :width="col.width"
              align="center"
            >
              <template #default="{ row }">
                <span>{{ row[col.prop] ?? '-' }}</span>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="90" fixed="right" align="center">
              <template #default="{ row }">
                <el-dropdown trigger="click" @command="(cmd: string) => handleAction(cmd, row)">
                  <UiButton variant="primary" link size="small">
                    操作<el-icon class="el-icon--right"><ArrowDown /></el-icon>
                  </UiButton>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item command="edit">编辑</el-dropdown-item>
                      <el-dropdown-item command="delete" divided>删除</el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </template>
            </el-table-column>
                      <template #empty>
              <UiEmptyState description="暂无数据" size="sm" />
            </template>
          </el-table>
          </UiAsyncSection>
        </el-tab-pane>

        <!-- Tab 2: 算法参数 -->
        <el-tab-pane label="算法参数" name="algorithm">
          <div class="tab-toolbar">
            <span class="tab-tip">
              调整核心公式参数（残值 = 基准原价 × Kt_adj × Kc × Km，其中 Kt_adj = Kt^(Kh/Kb)）
            </span>
          </div>

          <UiAsyncSection
            :error="algorithmLoadError"
            :loading="algorithmLoading"
            :retrying="algorithmRetrying"
            :skeleton="false"
            error-title="算法参数加载失败"
            error-description="网络或服务端异常，可重试"
            @retry="retryAlgorithmLoad"
          >
          <el-collapse v-model="activeCollapse" v-loading="algorithmLoading">
            <!-- 1. 全局系数 -->
            <el-collapse-item name="coefficients">
              <template #title>
                <div class="collapse-title">
                  <span>全局系数</span>
                  <span v-if="isCoefficientsDirty()" class="dirty-dot" title="有未保存变更">●</span>
                </div>
              </template>
              <div class="section-toolbar">
                <span class="section-tip">影响时间衰减、使用强度、置信区间等核心计算</span>
                <div class="section-actions">
                  <UiButton :icon="RefreshLeft" size="small" @click="resetCoefficients">重置</UiButton>
                  <UiButton variant="primary" :icon="Check" size="small" :loading="savingCoefficients" :disabled="!isCoefficientsDirty()" @click="saveCoefficients">
                    保存本节
                  </UiButton>
                </div>
              </div>
              <el-table :data="globalCoefficientsDraft" stripe border style="width: 100%">

                <el-table-column prop="key" label="参数键" width="200" />
                <el-table-column prop="description" label="参数说明" min-width="320" />
                <el-table-column label="参数值" width="180">
                  <template #default="{ row }">
                    <el-input-number
                      v-model="row.value"
                      :step="0.001"
                      :precision="4"
                      :min="0"
                      :max="100000"
                      style="width: 100%"
                    />
                  </template>
                </el-table-column>
                              <template #empty>
                  <UiEmptyState description="暂无参数" size="sm" />
                </template>
              </el-table>
            </el-collapse-item>

            <!-- 2. 品牌系数 -->
            <el-collapse-item name="brands">
              <template #title>
                <div class="collapse-title">
                  <span>品牌系数（Kb）</span>
                  <span v-if="isBrandsDirty()" class="dirty-dot" title="有未保存变更">●</span>
                </div>
              </template>
              <div class="section-toolbar">
                <span class="section-tip">Kb = k_brand，直接作为品牌系数参与 Kt_adj 计算</span>
                <div class="section-actions">
                  <UiButton :icon="RefreshLeft" size="small" @click="resetBrands">重置</UiButton>
                  <UiButton variant="primary" :icon="Check" size="small" :loading="savingBrands" :disabled="!isBrandsDirty()" @click="saveBrands">
                    保存本节
                  </UiButton>
                </div>
              </div>
              <el-table :data="brandsDraft" stripe border style="width: 100%">

                <el-table-column prop="name" label="品牌名称" min-width="180" />
                <el-table-column label="K_brand 系数" width="180">
                  <template #default="{ row }">
                    <el-input-number
                      v-model="row.k_brand"
                      :step="0.01"
                      :precision="2"
                      :min="0"
                      :max="10"
                      style="width: 100%"
                    />
                  </template>
                </el-table-column>
                <el-table-column label="启用" width="120" align="center">
                  <template #default="{ row }">
                    <UiSwitch v-model="row.is_active" />
                  </template>
                </el-table-column>
                              <template #empty>
                  <UiEmptyState description="暂无品牌" size="sm" />
                </template>
              </el-table>
            </el-collapse-item>

            <!-- 3. 车况系数 -->
            <el-collapse-item name="condition">
              <template #title>
                <div class="collapse-title">
                  <span>车况系数（Kc）</span>
                  <span v-if="isConditionRatingsDirty()" class="dirty-dot" title="有未保存变更">●</span>
                </div>
              </template>
              <div class="section-toolbar">
                <span class="section-tip">Kc = base_coefficient，按车况评级 A~E 给出基础调整系数</span>
                <div class="section-actions">
                  <UiButton :icon="RefreshLeft" size="small" @click="resetConditionRatings">重置</UiButton>
                  <UiButton variant="primary" :icon="Check" size="small" :loading="savingConditionRatings" :disabled="!isConditionRatingsDirty()" @click="saveConditionRatings">
                    保存本节
                  </UiButton>
                </div>
              </div>
              <el-table :data="conditionRatingsDraft" stripe border style="width: 100%">

                <el-table-column prop="rating" label="评级" width="100" align="center" />
                <el-table-column label="中文标签" min-width="180">
                  <template #default="{ row }">
                    <el-input v-model="row.label" placeholder="如 优秀" />
                  </template>
                </el-table-column>
                <el-table-column label="基础系数" width="180">
                  <template #default="{ row }">
                    <el-input-number
                      v-model="row.base_coefficient"
                      :step="0.01"
                      :precision="2"
                      :min="0"
                      :max="10"
                      style="width: 100%"
                    />
                  </template>
                </el-table-column>
                              <template #empty>
                  <UiEmptyState description="暂无车况评级" size="sm" />
                </template>
              </el-table>
            </el-collapse-item>

            <!-- 4. 车况修正项（000015 新增：油漆/保养/证件，按 kc_ 前缀过滤） -->
            <el-collapse-item name="kcModifiers">
              <template #title>
                <div class="collapse-title">
                  <span>车况修正项（油漆/保养/证件）</span>
                  <span v-if="isKcModifiersDirty()" class="dirty-dot" title="有未保存变更">●</span>
                </div>
              </template>
              <div class="section-toolbar">
                <span class="section-tip">
                  Kc 修正项：油漆/保养为加性叠加（base + bonus），证件为乘性扣减（×(1-pct)），缺双证时复合放大
                </span>
                <div class="section-actions">
                  <UiButton :icon="RefreshLeft" size="small" @click="resetKcModifiers">重置</UiButton>
                  <UiButton variant="primary" :icon="Check" size="small" :loading="savingKcModifiers" :disabled="!isKcModifiersDirty()" @click="saveKcModifiers">
                    保存本节
                  </UiButton>
                </div>
              </div>
              <el-table :data="kcModifiersDraft" stripe border style="width: 100%">

                <el-table-column prop="key" label="参数键" width="260" />
                <el-table-column prop="description" label="参数说明" min-width="380" />
                <el-table-column label="参数值" width="180">
                  <template #default="{ row }">
                    <el-input-number
                      v-model="row.value"
                      :step="0.01"
                      :precision="4"
                      :min="0"
                      :max="1"
                      style="width: 100%"
                    />
                  </template>
                </el-table-column>
                              <template #empty>
                  <UiEmptyState description="暂无车况修正项" size="sm" />
                </template>
              </el-table>
            </el-collapse-item>

            <!-- 5. 区域系数 -->
            <el-collapse-item name="region">
              <template #title>
                <div class="collapse-title">
                  <span>区域系数（Km）</span>
                  <span v-if="isRegionCoefficientsDirty()" class="dirty-dot" title="有未保存变更">●</span>
                </div>
              </template>
              <div class="section-toolbar">
                <span class="section-tip">Km = coefficient，按省市区域调整市场系数</span>
                <div class="section-actions">
                  <UiButton variant="success" :icon="Plus" size="small" @click="openCreateRegion">新增区域</UiButton>
                  <UiButton :icon="RefreshLeft" size="small" @click="resetRegionCoefficients">重置</UiButton>
                  <UiButton variant="primary" :icon="Check" size="small" :loading="savingRegionCoefficients" :disabled="!isRegionCoefficientsDirty()" @click="saveRegionCoefficients">
                    保存本节
                  </UiButton>
                </div>
              </div>
              <el-table :data="regionCoefficientsDraft" stripe border style="width: 100%">

                <el-table-column prop="province" label="省份" width="140" />
                <el-table-column prop="city" label="城市" width="160" />
                <el-table-column label="区域系数" width="200">
                  <template #default="{ row }">
                    <el-input-number
                      v-model="row.coefficient"
                      :step="0.01"
                      :precision="2"
                      :min="0"
                      :max="10"
                      style="width: 100%"
                    />
                  </template>
                </el-table-column>
                              <template #empty>
                  <UiEmptyState description="暂无区域系数" size="sm" />
                </template>
              </el-table>
            </el-collapse-item>
          </el-collapse>
          </UiAsyncSection>
        </el-tab-pane>
      </el-tabs>

      <!-- 原价表编辑对话框 -->
      <UiDialog
        v-model="dialogVisible"
        :title="dialogTitle"
        width="560px"
        destroy-on-close
       :confirm-text="editingRow ? '保存' : '创建'" :confirm-loading="submitting" @confirm="handleSubmit">
        <el-form :model="formData" label-width="120px">
          <el-form-item
            v-for="f in ORIGINAL_PRICE_FIELDS"
            :key="f.prop"
            :label="f.label"
            :required="f.required"
          >
            <el-input
              v-if="f.type === 'input'"
              v-model="formData[f.prop]"
              :placeholder="`请输入${f.label}`"
            />
            <el-input-number
              v-else-if="f.type === 'number'"
              v-model="formData[f.prop]"
              :step="f.prop === 'tonnage' || f.prop === 'original_price' ? 0.1 : 1"
              :precision="f.prop === 'tonnage' || f.prop === 'original_price' ? 2 : 0"
              style="width: 100%"
            />
            <UiSwitch
              v-else-if="f.type === 'switch'"
              v-model="formData[f.prop]"
              active-text="启用"
              inactive-text="禁用"
            />
          </el-form-item>
        </el-form>
      </UiDialog>

      <!-- 区域系数新增对话框 -->
      <UiDialog
        v-model="regionCreateDialogVisible"
        title="新增区域系数"
        width="480px"
        destroy-on-close
       confirm-text="创建" :confirm-loading="creatingRegion" @confirm="handleCreateRegion">
        <el-form :model="regionCreateForm" label-width="100px">
          <el-form-item label="省份" required>
            <el-input v-model="regionCreateForm.province" placeholder="如：江苏" />
          </el-form-item>
          <el-form-item label="城市" required>
            <el-input v-model="regionCreateForm.city" placeholder="如：苏州" />
          </el-form-item>
          <el-form-item label="区域系数">
            <el-input-number
              v-model="regionCreateForm.coefficient"
              :step="0.01"
              :precision="2"
              :min="0"
              :max="10"
              style="width: 100%"
            />
          </el-form-item>
        </el-form>
      </UiDialog>
    </div>
  </div>
</template>

<style scoped>
.config-manage {
  min-height: calc(100vh - var(--header-h, 56px) - 40px);
  background: var(--color-bg);
  padding-bottom: var(--sp-8);
}
.app-container {
  padding-top: var(--sp-6);
}
.tab-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--sp-4);
  gap: var(--sp-3);
}
.tab-tip {
  font-size: var(--fs-sm);
  color: var(--color-text-tertiary);
}

/* ===== 算法参数折叠面板 ===== */
.collapse-title {
  display: flex;
  align-items: center;
  gap: var(--sp-2);
  font-size: var(--fs-base);
  font-weight: var(--fw-medium);
}
.dirty-dot {
  color: var(--color-accent-500);
  font-size: 10px;
  line-height: 1;
}
.section-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--sp-3);
  gap: var(--sp-3);
}
.section-tip {
  font-size: var(--fs-sm);
  color: var(--color-text-tertiary);
}
.section-actions {
  display: flex;
  gap: var(--sp-2);
}

:deep(.el-tabs__content) {
  padding: var(--sp-4) var(--sp-5);
}
:deep(.el-collapse-item__header) {
  font-size: var(--fs-base);
  font-weight: var(--fw-medium);
  padding: 0 var(--sp-2);
}
:deep(.el-collapse-item__content) {
  padding: var(--sp-3) var(--sp-2) var(--sp-5);
}

@media (max-width: 768px) {
  .tab-toolbar,
  .section-toolbar {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--sp-2);
  }
  :deep(.el-tabs__content) {
    padding: var(--sp-3) var(--sp-2);
  }
}
</style>
