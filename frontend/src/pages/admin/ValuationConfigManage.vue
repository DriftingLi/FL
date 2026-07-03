<script setup lang="ts">
// 残值评估配置管理（管理员）
// 重构说明：从 15 tab 缩减为 2 tab
//   Tab 1 原价表：CRUD original-prices（学生端表单依赖该表数据）
//   Tab 2 算法参数：聚合展示 5 类参数（全局系数 / 品牌系数 / 车况系数 / 车况修正项 / 区域系数）
//                  每类独立保存，仅提交变更项（dirty 检测），不提供新增/删除
// 000015：新增"车况修正项"区，按 key 前缀 kc_ 过滤 coefficient_configs 行单独展示
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Edit, Delete, Refresh, Check, RefreshLeft } from '@element-plus/icons-vue'
import PageHeader from '@/components/valuation/PageHeader.vue'
import {
  adminResources,
  listAlgorithmParameters,
  updateCoefficient,
  updateBrandCoefficient,
  updateConditionCoefficient,
  updateRegionCoefficient,
  type AdminRow,
  type AlgorithmParameters
} from '@/api/valuation/admin'
import type { CoefficientConfig } from '@/types/valuation/evaluation'

// ========== Tab 1: 原价表 ==========
interface OriginalPriceTabState {
  loading: boolean
  list: AdminRow[]
}

const originalPriceState = reactive<OriginalPriceTabState>({
  loading: false,
  list: []
})

// 原价表筛选
const originalPriceFilter = reactive({
  brand: '',
  vehicle_type: '',
  series: '',
  config_type: ''
})

const filteredOriginalPrices = computed(() => {
  const rows = originalPriceState.list
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

async function loadOriginalPrices() {
  originalPriceState.loading = true
  try {
    originalPriceState.list = await adminResources.originalPrices.list()
  } catch {
    originalPriceState.list = []
  } finally {
    originalPriceState.loading = false
  }
}

// 原价表字段定义（已移除 brand_type 列）
interface FieldDef {
  prop: string
  label: string
  type: 'input' | 'number' | 'switch'
  required?: boolean
  width?: number
}

const ORIGINAL_PRICE_FIELDS: FieldDef[] = [
  { prop: 'brand', label: '品牌', type: 'input', required: true, width: 120 },
  { prop: 'vehicle_type', label: '车辆类型', type: 'input', required: true, width: 120 },
  { prop: 'series', label: '系列', type: 'input', width: 100 },
  { prop: 'tonnage', label: '吨位', type: 'number', width: 80 },
  { prop: 'config_type', label: '配置类型', type: 'input', width: 150 },
  { prop: 'mast_type', label: '门架类型', type: 'input', width: 100 },
  { prop: 'mast_height_mm', label: '门架高度(mm)', type: 'number', width: 120 },
  { prop: 'original_price', label: '原价（万元）', type: 'number', required: true, width: 120 }
]

// 通用编辑对话框
const dialogVisible = ref(false)
const dialogTitle = ref('')
const editingRow = ref<AdminRow | null>(null)
const formData = reactive<AdminRow>({})
const submitting = ref(false)

function openCreate() {
  editingRow.value = null
  dialogTitle.value = '新增原价记录'
  Object.keys(formData).forEach((k) => delete formData[k])
  for (const f of ORIGINAL_PRICE_FIELDS) {
    if (f.type === 'switch') formData[f.prop] = true
    else if (f.type === 'number') formData[f.prop] = 0
    else formData[f.prop] = ''
  }
  dialogVisible.value = true
}

function openEdit(row: AdminRow) {
  editingRow.value = row
  dialogTitle.value = '编辑原价记录'
  Object.keys(formData).forEach((k) => delete formData[k])
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
    const id = adminResources.originalPrices.getIdOf(editingRow.value)
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
    await ElMessageBox.confirm('确定删除该原价记录？', '删除确认', { type: 'warning' })
    await adminResources.originalPrices.remove(id)
    ElMessage.success('已删除')
    await loadOriginalPrices()
  } catch {
    // 用户取消或拦截器已提示
  }
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

// 服务器原始数据（用于 dirty 比较 & 重置）
const originalCoefficients = ref<CoeffRow[]>([])
const originalBrands = ref<BrandRow[]>([])
const originalConditionRatings = ref<ConditionRatingRow[]>([])
const originalRegionCoefficients = ref<RegionCoefficientRow[]>([])

// 本地编辑副本
const coefficientsDraft = ref<CoeffRow[]>([])
const brandsDraft = ref<BrandRow[]>([])
const conditionRatingsDraft = ref<ConditionRatingRow[]>([])
const regionCoefficientsDraft = ref<RegionCoefficientRow[]>([])

const algorithmLoading = ref(false)
const savingCoefficients = ref(false)
const savingBrands = ref(false)
const savingConditionRatings = ref(false)
const savingRegionCoefficients = ref(false)
const savingKcModifiers = ref(false)

async function loadAlgorithmParams() {
  algorithmLoading.value = true
  try {
    const data: AlgorithmParameters = await listAlgorithmParameters()
    originalCoefficients.value = data.coefficients.map((c) => ({ ...c }))
    originalBrands.value = data.brands.map((b) => ({ ...b }))
    originalConditionRatings.value = data.condition_ratings.map((c) => ({ ...c }))
    originalRegionCoefficients.value = data.region_coefficients.map((r) => ({ ...r }))
    coefficientsDraft.value = data.coefficients.map((c) => ({ ...c }))
    brandsDraft.value = data.brands.map((b) => ({ ...b }))
    conditionRatingsDraft.value = data.condition_ratings.map((c) => ({ ...c }))
    regionCoefficientsDraft.value = data.region_coefficients.map((r) => ({ ...r }))
  } catch {
    originalCoefficients.value = []
    originalBrands.value = []
    originalConditionRatings.value = []
    originalRegionCoefficients.value = []
    coefficientsDraft.value = []
    brandsDraft.value = []
    conditionRatingsDraft.value = []
    regionCoefficientsDraft.value = []
  } finally {
    algorithmLoading.value = false
  }
}

// dirty 检测：比较 draft 与 original
function isCoefficientsDirty(): boolean {
  if (coefficientsDraft.value.length !== originalCoefficients.value.length) return true
  return coefficientsDraft.value.some((c, i) => {
    const o = originalCoefficients.value[i]
    return !o || o.key !== c.key || o.value !== c.value
  })
}

// 000015：按 key 前缀 kc_ 拆分全局系数与车况修正项
// 全局系数区只展示非 kc_ 前缀的行；车况修正项区只展示 kc_ 前缀的行
// 底层数据共享 coefficientsDraft / originalCoefficients，保存/重置仍走 saveCoefficients/resetCoefficients
const globalCoefficientsDraft = computed(() =>
  coefficientsDraft.value.filter((c) => !c.key.startsWith('kc_'))
)
const kcModifiersDraft = computed(() =>
  coefficientsDraft.value.filter((c) => c.key.startsWith('kc_'))
)
// 车况修正项 dirty：仅检查 kc_ 前缀的行
function isKcModifiersDirty(): boolean {
  const draftKc = coefficientsDraft.value.filter((c) => c.key.startsWith('kc_'))
  const origKc = originalCoefficients.value.filter((c) => c.key.startsWith('kc_'))
  if (draftKc.length !== origKc.length) return true
  return draftKc.some((c) => {
    const o = origKc.find((x) => x.key === c.key)
    return !o || o.value !== c.value
  })
}
function isBrandsDirty(): boolean {
  if (brandsDraft.value.length !== originalBrands.value.length) return true
  return brandsDraft.value.some((b, i) => {
    const o = originalBrands.value[i]
    return !o || o.id !== b.id || o.k_brand !== b.k_brand || o.is_active !== b.is_active
  })
}
function isConditionRatingsDirty(): boolean {
  if (conditionRatingsDraft.value.length !== originalConditionRatings.value.length) return true
  return conditionRatingsDraft.value.some((c, i) => {
    const o = originalConditionRatings.value[i]
    return !o || o.id !== c.id || o.label !== c.label || o.base_coefficient !== c.base_coefficient
  })
}
function isRegionCoefficientsDirty(): boolean {
  if (regionCoefficientsDraft.value.length !== originalRegionCoefficients.value.length) return true
  return regionCoefficientsDraft.value.some((r, i) => {
    const o = originalRegionCoefficients.value[i]
    return !o || o.id !== r.id || o.coefficient !== r.coefficient
  })
}

// 保存：仅提交变更项（限定到非 kc_ 前缀的全局系数）
async function saveCoefficients() {
  const dirtyItems = coefficientsDraft.value.filter((c) => {
    if (c.key.startsWith('kc_')) return false // 跳过车况修正项，由 saveKcModifiers 处理
    const o = originalCoefficients.value.find((x) => x.key === c.key)
    return !o || o.value !== c.value
  })
  if (dirtyItems.length === 0) {
    ElMessage.info('无变更')
    return
  }
  savingCoefficients.value = true
  try {
    await Promise.all(dirtyItems.map((c) => updateCoefficient(c.key, c.value)))
    ElMessage.success(`已保存 ${dirtyItems.length} 项全局系数`)
    await loadAlgorithmParams()
  } catch {
    // 拦截器已提示
  } finally {
    savingCoefficients.value = false
  }
}

// 000015：保存车况修正项（仅 kc_ 前缀的行）
async function saveKcModifiers() {
  const dirtyItems = coefficientsDraft.value.filter((c) => {
    if (!c.key.startsWith('kc_')) return false
    const o = originalCoefficients.value.find((x) => x.key === c.key)
    return !o || o.value !== c.value
  })
  if (dirtyItems.length === 0) {
    ElMessage.info('无变更')
    return
  }
  savingKcModifiers.value = true
  try {
    await Promise.all(dirtyItems.map((c) => updateCoefficient(c.key, c.value)))
    ElMessage.success(`已保存 ${dirtyItems.length} 项车况修正项`)
    await loadAlgorithmParams()
  } catch {
    // 拦截器已提示
  } finally {
    savingKcModifiers.value = false
  }
}

async function saveBrands() {
  const dirtyItems = brandsDraft.value.filter((b) => {
    const o = originalBrands.value.find((x) => x.id === b.id)
    return !o || o.k_brand !== b.k_brand || o.is_active !== b.is_active
  })
  if (dirtyItems.length === 0) {
    ElMessage.info('无变更')
    return
  }
  savingBrands.value = true
  try {
    await Promise.all(
      dirtyItems.map((b) => updateBrandCoefficient(b.id, b.k_brand, b.is_active))
    )
    ElMessage.success(`已保存 ${dirtyItems.length} 项品牌系数`)
    await loadAlgorithmParams()
  } catch {
    // 拦截器已提示
  } finally {
    savingBrands.value = false
  }
}

async function saveConditionRatings() {
  const dirtyItems = conditionRatingsDraft.value.filter((c) => {
    const o = originalConditionRatings.value.find((x) => x.id === c.id)
    return !o || o.label !== c.label || o.base_coefficient !== c.base_coefficient
  })
  if (dirtyItems.length === 0) {
    ElMessage.info('无变更')
    return
  }
  // 必填校验
  for (const c of dirtyItems) {
    if (!c.label || !c.label.trim()) {
      ElMessage.warning(`评级 ${c.rating} 的中文标签不能为空`)
      return
    }
  }
  savingConditionRatings.value = true
  try {
    await Promise.all(
      dirtyItems.map((c) =>
        updateConditionCoefficient(c.id, c.label, c.base_coefficient)
      )
    )
    ElMessage.success(`已保存 ${dirtyItems.length} 项车况系数`)
    await loadAlgorithmParams()
  } catch {
    // 拦截器已提示
  } finally {
    savingConditionRatings.value = false
  }
}

async function saveRegionCoefficients() {
  const dirtyItems = regionCoefficientsDraft.value.filter((r) => {
    const o = originalRegionCoefficients.value.find((x) => x.id === r.id)
    return !o || o.coefficient !== r.coefficient
  })
  if (dirtyItems.length === 0) {
    ElMessage.info('无变更')
    return
  }
  savingRegionCoefficients.value = true
  try {
    await Promise.all(
      dirtyItems.map((r) => updateRegionCoefficient(r.id, r.coefficient))
    )
    ElMessage.success(`已保存 ${dirtyItems.length} 项区域系数`)
    await loadAlgorithmParams()
  } catch {
    // 拦截器已提示
  } finally {
    savingRegionCoefficients.value = false
  }
}

// 重置：恢复全局系数 draft 到服务器值（仅非 kc_ 前缀的行）
function resetCoefficients() {
  // 全量重置即可：kc_ 前缀的行也会被重置，但用户在车况修正项区点重置时
  // 通常只关心 kc_ 行；为简化实现，此处统一重置全部 coefficients 行
  coefficientsDraft.value = originalCoefficients.value.map((c) => ({ ...c }))
}

// 000015：重置车况修正项（仅 kc_ 前缀的行恢复服务器值）
function resetKcModifiers() {
  // 找出 kc_ 行的原始值，覆盖 draft 中对应行
  const origKcMap = new Map(
    originalCoefficients.value
      .filter((c) => c.key.startsWith('kc_'))
      .map((c) => [c.key, { ...c }])
  )
  coefficientsDraft.value = coefficientsDraft.value.map((c) => {
    if (c.key.startsWith('kc_')) {
      return origKcMap.get(c.key) ?? { ...c }
    }
    return c
  })
}
function resetBrands() {
  brandsDraft.value = originalBrands.value.map((b) => ({ ...b }))
}
function resetConditionRatings() {
  conditionRatingsDraft.value = originalConditionRatings.value.map((c) => ({ ...c }))
}
function resetRegionCoefficients() {
  regionCoefficientsDraft.value = originalRegionCoefficients.value.map((r) => ({ ...r }))
}

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
          <el-button :icon="Refresh" @click="onRefresh">刷新当前</el-button>
        </template>
      </PageHeader>

      <el-tabs v-model="activeTab" type="border-card" @tab-change="onTabChange">
        <!-- Tab 1: 原价表 -->
        <el-tab-pane label="原价表" name="originalPrices">
          <div class="tab-toolbar">
            <span class="tab-tip">维护叉车基准原价记录（学生端表单级联查询依赖此表）</span>
            <el-button type="primary" :icon="Plus" @click="openCreate">新增</el-button>
          </div>
          <div class="filter-bar">
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
            <el-button :icon="RefreshLeft" size="small" @click="resetOriginalPriceFilter">重置筛选</el-button>
          </div>
          <el-table
            v-loading="originalPriceState.loading"
            :data="filteredOriginalPrices"
            stripe
            border
            style="width: 100%"
            empty-text="暂无数据"
          >
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
            <el-table-column label="操作" width="160" fixed="right" align="center">
              <template #default="{ row }">
                <el-button type="primary" link size="small" :icon="Edit" @click="openEdit(row)">
                  编辑
                </el-button>
                <el-button type="danger" link size="small" :icon="Delete" @click="handleDelete(row)">
                  删除
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <!-- Tab 2: 算法参数 -->
        <el-tab-pane label="算法参数" name="algorithm">
          <div class="tab-toolbar">
            <span class="tab-tip">
              调整核心公式参数（残值 = 基准原价 × Kt_adj × Kc × Km，其中 Kt_adj = Kt^(Kh/Kb)）
            </span>
          </div>

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
                  <el-button :icon="RefreshLeft" size="small" @click="resetCoefficients">重置</el-button>
                  <el-button
                    type="primary"
                    :icon="Check"
                    size="small"
                    :loading="savingCoefficients"
                    :disabled="!isCoefficientsDirty()"
                    @click="saveCoefficients"
                  >
                    保存本节
                  </el-button>
                </div>
              </div>
              <el-table :data="globalCoefficientsDraft" stripe border style="width: 100%" empty-text="暂无参数">
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
                  <el-button :icon="RefreshLeft" size="small" @click="resetBrands">重置</el-button>
                  <el-button
                    type="primary"
                    :icon="Check"
                    size="small"
                    :loading="savingBrands"
                    :disabled="!isBrandsDirty()"
                    @click="saveBrands"
                  >
                    保存本节
                  </el-button>
                </div>
              </div>
              <el-table :data="brandsDraft" stripe border style="width: 100%" empty-text="暂无品牌">
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
                    <el-switch v-model="row.is_active" />
                  </template>
                </el-table-column>
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
                  <el-button :icon="RefreshLeft" size="small" @click="resetConditionRatings">重置</el-button>
                  <el-button
                    type="primary"
                    :icon="Check"
                    size="small"
                    :loading="savingConditionRatings"
                    :disabled="!isConditionRatingsDirty()"
                    @click="saveConditionRatings"
                  >
                    保存本节
                  </el-button>
                </div>
              </div>
              <el-table :data="conditionRatingsDraft" stripe border style="width: 100%" empty-text="暂无车况评级">
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
                  <el-button :icon="RefreshLeft" size="small" @click="resetKcModifiers">重置</el-button>
                  <el-button
                    type="primary"
                    :icon="Check"
                    size="small"
                    :loading="savingKcModifiers"
                    :disabled="!isKcModifiersDirty()"
                    @click="saveKcModifiers"
                  >
                    保存本节
                  </el-button>
                </div>
              </div>
              <el-table :data="kcModifiersDraft" stripe border style="width: 100%" empty-text="暂无车况修正项">
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
                  <el-button type="success" :icon="Plus" size="small" @click="openCreateRegion">新增区域</el-button>
                  <el-button :icon="RefreshLeft" size="small" @click="resetRegionCoefficients">重置</el-button>
                  <el-button
                    type="primary"
                    :icon="Check"
                    size="small"
                    :loading="savingRegionCoefficients"
                    :disabled="!isRegionCoefficientsDirty()"
                    @click="saveRegionCoefficients"
                  >
                    保存本节
                  </el-button>
                </div>
              </div>
              <el-table :data="regionCoefficientsDraft" stripe border style="width: 100%" empty-text="暂无区域系数">
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
              </el-table>
            </el-collapse-item>
          </el-collapse>
        </el-tab-pane>
      </el-tabs>

      <!-- 原价表编辑对话框 -->
      <el-dialog
        v-model="dialogVisible"
        :title="dialogTitle"
        width="560px"
        destroy-on-close
      >
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
            <el-switch
              v-else-if="f.type === 'switch'"
              v-model="formData[f.prop]"
              active-text="启用"
              inactive-text="禁用"
            />
          </el-form-item>
        </el-form>
        <template #footer>
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="submitting" @click="handleSubmit">
            {{ editingRow ? '保存' : '创建' }}
          </el-button>
        </template>
      </el-dialog>

      <!-- 区域系数新增对话框 -->
      <el-dialog
        v-model="regionCreateDialogVisible"
        title="新增区域系数"
        width="480px"
        destroy-on-close
      >
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
        <template #footer>
          <el-button @click="regionCreateDialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="creatingRegion" @click="handleCreateRegion">
            创建
          </el-button>
        </template>
      </el-dialog>
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
.filter-bar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: var(--sp-3);
  margin-bottom: var(--sp-4);
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
  color: var(--color-accent, #3e6ae1);
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
