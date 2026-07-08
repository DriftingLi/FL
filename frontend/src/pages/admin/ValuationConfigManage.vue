<script setup lang="ts">
// 残值评估配置管理（管理员）
// 重构说明：从 15 tab 缩减为 2 tab
//   Tab 1 原价表：CRUD original-prices（学生端表单依赖该表数据）
//   Tab 2 算法参数：聚合展示 5 类参数（全局系数 / 品牌系数 / 车况系数 / 车况修正项 / 区域系数）
//                  每类独立保存，仅提交变更项（dirty 检测），不提供新增/删除
// 000015：新增"车况修正项"区，按 key 前缀 kc_ 过滤 coefficient_configs 行单独展示
//
// 重构（2026-07）：抽出通用组合式函数消除重复
//   - useCrudTable：Tab 1 原价表 CRUD（列表/弹窗/必填校验/删除确认/loading 态）
//   - useDirtyDraft：Tab 2 各分区 dirty 检测 + 仅保存变更项 + 重置
//     coefficients 分区共享一条 draft，global / kcModifiers 通过 filter 派生两个视图
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
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
  type AdminResourceId,
  type AlgorithmParameters
} from '@/api/valuation/admin'
import type { CoefficientConfig } from '@/types/valuation/evaluation'
import { useCrudTable, type FieldDef } from '@/composables/useCrudTable'
import { useDirtyDraft } from '@/composables/useDirtyDraft'

// ========== Tab 1: 原价表 ==========
const ORIGINAL_PRICE_FIELDS: FieldDef[] = [
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

const {
  loading: originalPriceLoading,
  list: originalPriceList,
  dialogVisible,
  dialogTitle,
  editingRow,
  formData,
  submitting,
  load: loadOriginalPrices,
  openCreate,
  openEdit,
  submit: handleSubmit,
  remove: handleDelete
} = useCrudTable<AdminRow, AdminResourceId>(
  {
    fetch: () => adminResources.originalPrices.list(),
    create: (p) => adminResources.originalPrices.create(p),
    update: (id, p) => adminResources.originalPrices.update(id, p),
    remove: (id) => adminResources.originalPrices.remove(id),
    getId: (row) => adminResources.originalPrices.getIdOf(row)
  },
  ORIGINAL_PRICE_FIELDS,
  '原价记录'
)

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

const algorithmLoading = ref(false)
// 各分区独立 saving 态，保留各按钮独立 loading 的精确行为
const savingCoefficients = ref(false)
const savingKcModifiers = ref(false)
const savingBrands = ref(false)
const savingConditionRatings = ref(false)
const savingRegionCoefficients = ref(false)

async function loadAlgorithmParams() {
  algorithmLoading.value = true
  try {
    const data: AlgorithmParameters = await listAlgorithmParameters()
    coefficients.setAll(data.coefficients)
    brands.setAll(data.brands)
    conditionRatings.setAll(data.condition_ratings)
    regionCoefficients.setAll(data.region_coefficients)
  } catch {
    coefficients.clear()
    brands.clear()
    conditionRatings.clear()
    regionCoefficients.clear()
  } finally {
    algorithmLoading.value = false
  }
}

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
            v-loading="originalPriceLoading"
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
