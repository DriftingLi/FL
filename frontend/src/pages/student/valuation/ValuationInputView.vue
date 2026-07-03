<script setup lang="ts">
// 叉车残值评估参数录入（设计稿风格：白底细边框卡片 + 自定义表单控件 + 底部固定操作栏）
// 配置类型为单一下拉，选项来自 original_prices 级联查询（含传动/发动机/电池等复合配置）
// 三行级联布局：
//   行1 品牌：品牌 → 车辆类型
//   行2 系列吨位：系列 → 吨位 → 出厂年份（出厂年份 min 由所选系列 earliest_factory_year 决定）
//   行3 配置门架：配置类型 → 门架类型 → 门架高度
// "其它" 选项：series 可选 "其它"（级联查询时传 undefined 跳过 series 过滤）
// "无" 选项：mast_type 可选 "无"；mast_height_mm 用 0 表示 "无"
import { computed, onMounted, ref, watch } from 'vue'
import { Refresh, Promotion } from '@element-plus/icons-vue'
import PageHeader from '@/components/valuation/PageHeader.vue'
import { useEvaluationForm, OTHER_SERIES_VALUE, NONE_VALUE, NONE_MAST_HEIGHT } from '@/composables/useEvaluationForm'
import {
  listBrands,
  listVehicleTypes,
  listSeries,
  listTonnages,
  listConfigTypes,
  listMastTypes,
  listMastHeights,
  getEarliestFactoryYear,
  listConditionRatings,
  listProvinces,
  listCities
} from '@/api/valuation/dictionaries'
import { getEvaluationStats } from '@/api/valuation/evaluation'
import type {
  VehicleTypeOption,
  SeriesOption,
  TonnageOption,
  ConfigTypeOption,
  MastTypeOption,
  MastHeightOption,
  ConditionRatingOption
} from '@/types/valuation/evaluation'
import type { Brand } from '@/types/valuation/brand'

// ========== 字典数据 ==========
const brands = ref<Brand[]>([])
const vehicleTypes = ref<VehicleTypeOption[]>([])
const seriesList = ref<SeriesOption[]>([])
const tonnages = ref<TonnageOption[]>([])
const configTypes = ref<ConfigTypeOption[]>([])
const mastTypes = ref<MastTypeOption[]>([])
const mastHeights = ref<MastHeightOption[]>([])
const conditionRatings = ref<ConditionRatingOption[]>([])
const provinces = ref<string[]>([])
const cities = ref<string[]>([])
const loadingDict = ref(false)

// ========== 累计评估次数统计 ==========
const statsTotal = ref(0)
const loadingStats = ref(false)

// ========== "其它"/"无" 选项（前端常量，附加到下拉列表末尾） ==========
const otherSeriesOption: SeriesOption = { id: -1, brand: '', name: OTHER_SERIES_VALUE, earliest_factory_year: 1980 }
const noneMastTypeOption: MastTypeOption = { id: -1, name: NONE_VALUE }
const noneMastHeightOption: MastHeightOption = { id: -1, value_mm: NONE_MAST_HEIGHT }

// 合并选项后的可选项列表
// 若 API 返回的系列列表中已包含 "其它"，不再追加（避免重复）
const seriesOptions = computed(() => {
  if (seriesList.value.some((s) => s.name === OTHER_SERIES_VALUE)) {
    return seriesList.value
  }
  return [...seriesList.value, otherSeriesOption]
})
const mastTypeOptions = computed(() => [...mastTypes.value, noneMastTypeOption])
const mastHeightOptions = computed(() => [...mastHeights.value, noneMastHeightOption])

// ========== 表单 ==========
const { form, submitting, isValid, reset, submit } = useEvaluationForm()

// 当前品牌+车型+系列+吨位组合下的最早出厂年份下限
// 选完吨位后从 original_prices 级联查询 MIN(earliest_factory_year)；查询前默认 1980
const earliestFactoryYear = ref(1980)

// 出厂年份字段可见性：选完吨位后才显示
const showFactoryYear = computed(() => form.tonnage != null)

// ========== 级联加载 ==========
// 级联顺序：品牌 → 车辆类型 → 系列 → 吨位 →（出厂年份 + 配置类型）→ 门架类型 → 门架高度

// 品牌 → 车辆类型
watch(
  () => form.brand,
  async (b) => {
    form.vehicle_type = undefined
    form.series = undefined
    form.tonnage = undefined
    form.config_type = undefined
    form.mast_type = undefined
    form.mast_height_mm = undefined
    form.factory_year = undefined
    vehicleTypes.value = []
    seriesList.value = []
    tonnages.value = []
    configTypes.value = []
    mastTypes.value = []
    mastHeights.value = []

    if (!b) return
    vehicleTypes.value = await listVehicleTypes(b)
  }
)

// 车辆类型 → 系列
watch(
  () => form.vehicle_type,
  async (vt) => {
    form.series = undefined
    form.tonnage = undefined
    form.config_type = undefined
    form.mast_type = undefined
    form.mast_height_mm = undefined
    form.factory_year = undefined
    seriesList.value = []
    tonnages.value = []
    configTypes.value = []
    mastTypes.value = []
    mastHeights.value = []

    if (!form.brand || !vt) return
    seriesList.value = await listSeries(form.brand, vt)
  }
)

// 系列 → 吨位
watch(
  () => form.series,
  async (s) => {
    form.tonnage = undefined
    form.config_type = undefined
    form.mast_type = undefined
    form.mast_height_mm = undefined
    form.factory_year = undefined
    tonnages.value = []
    configTypes.value = []
    mastTypes.value = []
    mastHeights.value = []

    if (!form.brand || !form.vehicle_type || !s) return
    // 系列为 "其它" 时，吨位查询不传 series 参数
    const seriesParam = s === OTHER_SERIES_VALUE ? undefined : s
    tonnages.value = await listTonnages(form.brand, form.vehicle_type, seriesParam)
  }
)

// 吨位 → 配置类型 + 最早出厂年份下限
watch(
  () => form.tonnage,
  async () => {
    form.config_type = undefined
    form.mast_type = undefined
    form.mast_height_mm = undefined
    form.factory_year = undefined
    configTypes.value = []
    mastTypes.value = []
    mastHeights.value = []
    earliestFactoryYear.value = 1980

    if (!form.brand || !form.vehicle_type || !form.series || form.tonnage == null) return
    const seriesParam = form.series === OTHER_SERIES_VALUE ? undefined : form.series
    const [configs, year] = await Promise.all([
      listConfigTypes(form.brand, form.vehicle_type, seriesParam ?? form.series, form.tonnage),
      getEarliestFactoryYear(form.brand, form.vehicle_type, seriesParam, form.tonnage)
    ])
    configTypes.value = configs
    earliestFactoryYear.value = year
  }
)

// 配置类型 → 门架类型
watch(
  () => form.config_type,
  async (ct) => {
    form.mast_type = undefined
    form.mast_height_mm = undefined
    mastTypes.value = []
    mastHeights.value = []

    if (!form.brand || !form.vehicle_type || !form.series || form.tonnage == null || !ct) return
    const seriesParam = form.series === OTHER_SERIES_VALUE ? undefined : form.series
    mastTypes.value = await listMastTypes(
      form.brand, form.vehicle_type, seriesParam ?? form.series, form.tonnage, ct
    )
  }
)

// 门架类型 → 门架高度
watch(
  () => form.mast_type,
  async (mt) => {
    form.mast_height_mm = undefined
    mastHeights.value = []

    if (!form.brand || !form.vehicle_type || !form.series || form.tonnage == null ||
        !form.config_type || !mt) return
    const seriesParam = form.series === OTHER_SERIES_VALUE ? undefined : form.series
    mastHeights.value = await listMastHeights(
      form.brand, form.vehicle_type, seriesParam ?? form.series, form.tonnage, form.config_type, mt
    )
  }
)

// 省份 → 城市
watch(
  () => form.province,
  async (p) => {
    form.city = undefined
    cities.value = []
    if (!p) return
    cities.value = await listCities(p)
  }
)

// ========== 初始化：并行加载静态字典 ==========
onMounted(async () => {
  loadingDict.value = true
  loadingStats.value = true
  try {
    const [brandsList, crList, provList] = await Promise.all([
      listBrands(),
      listConditionRatings(),
      listProvinces()
    ])
    brands.value = brandsList
    conditionRatings.value = crList
    provinces.value = provList
  } finally {
    loadingDict.value = false
  }
  // 统计数据独立加载，失败不影响表单
  try {
    const s = await getEvaluationStats()
    statsTotal.value = s.total
  } catch {
    statsTotal.value = 0
  } finally {
    loadingStats.value = false
  }
})

// ========== 字段可见性 ==========
const showBrand = computed(() => brands.value.length > 0)
const showVehicleType = computed(() => vehicleTypes.value.length > 0)
const showSeries = computed(() => form.vehicle_type !== undefined)
const showTonnage = computed(() => form.series !== undefined)
const showConfigType = computed(() => form.tonnage != null)
const showMastType = computed(() => form.config_type !== undefined)
const showMastHeight = computed(() => form.mast_type !== undefined)
const showConditionRating = computed(() => conditionRatings.value.length > 0)
const showProvince = computed(() => provinces.value.length > 0)
const showCity = computed(() => cities.value.length > 0)

// 车况评级按 rating 排序展示
const sortedConditionRatings = computed(() =>
  [...conditionRatings.value].sort((a, b) => a.rating.localeCompare(b.rating))
)

function onSubmit() {
  submit()
}
</script>

<template>
  <div class="app-container input-page valuation-root">
    <PageHeader
      title="叉车残值评估"
      subtitle="forklift residual value · parameter input"
    >
      <template #actions>
        <el-button :icon="Refresh" @click="reset">重置</el-button>
        <el-button
          type="primary"
          :icon="Promotion"
          :loading="submitting"
          :disabled="!isValid"
          @click="onSubmit"
        >
          提交评估
        </el-button>
      </template>
    </PageHeader>

    <!-- 累计评估次数概览卡 -->
    <section class="stats-card card-surface" v-loading="loadingStats">
      <div class="stats-card-body">
        <span class="stats-card-label">累计评估次数</span>
        <div class="stats-card-value">
          <span class="num">{{ statsTotal }}</span>
          <span class="unit">次</span>
        </div>
        <p class="stats-card-suffix">已有用户提交的残值评估总数</p>
      </div>
    </section>

    <div v-loading="loadingDict" class="input-form-body">
      <!-- 品牌与车型（三行级联） -->
      <section class="input-section card-surface">
        <h2 class="section-title">品牌与车型</h2>

        <!-- 行1：品牌类型（品牌 → 车辆类型） -->
        <div class="form-row row-3">
          <el-form-item v-if="showBrand" label="品牌">
            <el-select
              v-model="form.brand"
              placeholder="请选择品牌"
              filterable
              clearable
            >
              <el-option
                v-for="b in brands"
                :key="b.id"
                :value="b.name"
                :label="b.name"
              />
            </el-select>
          </el-form-item>
          <el-form-item v-if="showVehicleType" label="车辆类型">
            <el-select
              v-model="form.vehicle_type"
              placeholder="请选择车辆类型"
              filterable
              clearable
              :disabled="!form.brand"
            >
              <el-option
                v-for="vt in vehicleTypes"
                :key="vt.id"
                :value="vt.name"
                :label="vt.name"
              />
            </el-select>
          </el-form-item>
        </div>

        <!-- 行2：系列吨位（系列 → 吨位 → 出厂年份） -->
        <div v-if="showSeries" class="form-row row-3">
          <el-form-item label="系列">
            <el-select
              v-model="form.series"
              placeholder="请选择系列"
              filterable
              clearable
              :disabled="!form.vehicle_type"
            >
              <el-option
                v-for="s in seriesOptions"
                :key="s.id"
                :value="s.name"
                :label="s.name"
              />
            </el-select>
          </el-form-item>
          <el-form-item v-if="showTonnage" label="吨位">
            <el-select
              v-model="form.tonnage"
              placeholder="请选择吨位"
              filterable
              clearable
              :disabled="!form.series"
            >
              <el-option
                v-for="t in tonnages"
                :key="t.id"
                :value="t.value"
                :label="`${t.value} 吨`"
              />
            </el-select>
          </el-form-item>
          <el-form-item v-if="showFactoryYear" label="出厂年份">
            <el-input-number
              v-model="form.factory_year"
              :min="earliestFactoryYear"
              :max="new Date().getFullYear()"
              :step="1"
              :disabled="form.tonnage == null"
              style="width: 100%"
              placeholder="如 2021"
            />
          </el-form-item>
        </div>

        <!-- 行3：配置门架（配置类型 → 门架类型 → 门架高度） -->
        <div v-if="showConfigType" class="form-row row-3">
          <el-form-item label="配置类型">
            <el-select
              v-model="form.config_type"
              placeholder="请选择配置类型"
              filterable
              clearable
              :disabled="form.tonnage == null"
            >
              <el-option
                v-for="c in configTypes"
                :key="c.id"
                :value="c.name"
                :label="c.name"
              />
            </el-select>
          </el-form-item>
          <el-form-item v-if="showMastType" label="门架类型">
            <el-select
              v-model="form.mast_type"
              placeholder="请选择门架类型"
              filterable
              clearable
              :disabled="!form.config_type"
            >
              <el-option
                v-for="m in mastTypeOptions"
                :key="m.id"
                :value="m.name"
                :label="m.name"
              />
            </el-select>
          </el-form-item>
          <el-form-item v-if="showMastHeight" label="门架高度">
            <el-select
              v-model="form.mast_height_mm"
              placeholder="请选择门架高度"
              filterable
              clearable
              :disabled="!form.mast_type"
            >
              <el-option
                v-for="mh in mastHeightOptions"
                :key="mh.id"
                :value="mh.value_mm"
                :label="mh.value_mm === NONE_MAST_HEIGHT ? '无' : `${mh.value_mm} mm`"
              />
            </el-select>
          </el-form-item>
        </div>
      </section>

      <!-- 使用信息：工时 / 原漆 -->
      <section class="input-section card-surface">
        <h2 class="section-title">使用信息</h2>
        <div class="form-row row-2">
          <el-form-item label="累计工时">
            <el-input-number
              v-model="form.usage_hours"
              :min="0"
              :max="100000"
              :step="100"
              style="width: 100%"
              placeholder="如 3500"
            />
          </el-form-item>
          <el-form-item label="是否原厂原漆">
            <div class="custom-toggle">
              <el-switch v-model="form.original_paint" />
              <span class="toggle-text" :class="form.original_paint ? 'active' : 'muted'">原厂原漆</span>
              <span class="toggle-text muted">/ 非原厂</span>
            </div>
          </el-form-item>
        </div>
      </section>

      <!-- 区域信息：省 → 市 -->
      <section v-if="showProvince" class="input-section card-surface">
        <h2 class="section-title">所在区域</h2>
        <div class="form-row row-2">
          <el-form-item label="省份">
            <el-select
              v-model="form.province"
              placeholder="请选择省份"
              filterable
              clearable
            >
              <el-option
                v-for="p in provinces"
                :key="p"
                :value="p"
                :label="p"
              />
            </el-select>
          </el-form-item>
          <el-form-item v-if="showCity" label="城市">
            <el-select
              v-model="form.city"
              placeholder="请选择城市"
              filterable
              clearable
              :disabled="!form.province"
            >
              <el-option
                v-for="c in cities"
                :key="c"
                :value="c"
                :label="c"
              />
            </el-select>
          </el-form-item>
        </div>
      </section>

      <!-- 证件与保养 -->
      <section class="input-section card-surface">
        <h2 class="section-title">证件与保养</h2>
        <div class="form-row row-3">
          <el-form-item label="是否有车牌">
            <div class="custom-toggle">
              <el-switch v-model="form.has_license_plate" />
              <span class="toggle-text" :class="form.has_license_plate ? 'active' : 'muted'">
                {{ form.has_license_plate ? '有' : '无' }}
              </span>
            </div>
          </el-form-item>
          <el-form-item label="特种设备登记证">
            <div class="custom-toggle">
              <el-switch v-model="form.has_registration_certificate" />
              <span class="toggle-text" :class="form.has_registration_certificate ? 'active' : 'muted'">
                {{ form.has_registration_certificate ? '有' : '无' }}
              </span>
            </div>
          </el-form-item>
          <el-form-item label="是否有保养记录">
            <div class="custom-toggle">
              <el-switch v-model="form.has_maintenance_records" />
              <span class="toggle-text" :class="form.has_maintenance_records ? 'active' : 'muted'">
                {{ form.has_maintenance_records ? '有' : '无' }}
              </span>
            </div>
          </el-form-item>
        </div>
      </section>

      <!-- 车况评级 -->
      <section v-if="showConditionRating" class="input-section card-surface">
        <h2 class="section-title">车况评级</h2>
        <el-form-item label="车况评级">
          <el-radio-group v-model="form.condition_rating" class="condition-pills">
            <el-radio-button
              v-for="cr in sortedConditionRatings"
              :key="cr.id"
              :value="cr.rating"
            >
              {{ cr.rating }} · {{ cr.label }}
            </el-radio-button>
          </el-radio-group>
        </el-form-item>
      </section>
    </div>

    <!-- 底部固定操作栏 -->
    <div class="bottom-action-bar">
      <div class="bottom-action-inner">
        <el-button :icon="Refresh" @click="reset">重置</el-button>
        <el-button
          type="primary"
          :icon="Promotion"
          :loading="submitting"
          :disabled="!isValid"
          @click="onSubmit"
        >
          提交评估
        </el-button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.input-page {
  padding: 0 0 var(--sp-20);
  min-height: calc(100vh - var(--header-h));
}

.input-form-body {
  padding-bottom: var(--sp-6);
}

/* ===== 累计评估次数概览卡 ===== */
.stats-card {
  margin-bottom: var(--sp-6);
  border-radius: var(--radius-xl);
  padding: var(--sp-6) var(--sp-7);
}
.stats-card-body {
  display: flex;
  flex-direction: column;
  gap: var(--sp-2);
}
.stats-card-label {
  font-size: var(--fs-sm);
  font-weight: var(--fw-medium);
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--color-text-tertiary);
}
.stats-card-value {
  display: flex;
  align-items: baseline;
  gap: 8px;
  font-family: var(--font-mono);
  font-feature-settings: 'tnum' 1;
}
.stats-card-value .num {
  font-size: 40px;
  font-weight: var(--fw-semibold);
  color: var(--color-primary);
  letter-spacing: -0.02em;
  line-height: 1.1;
}
.stats-card-value .unit {
  font-family: var(--font-text);
  font-size: var(--fs-lg);
  font-weight: var(--fw-medium);
  color: var(--color-text);
}
.stats-card-suffix {
  margin: 0;
  font-size: var(--fs-xs);
  color: var(--color-text-tertiary);
  letter-spacing: 0.04em;
}

.input-section {
  margin-bottom: var(--sp-6);
  border-radius: var(--radius-xl);
  padding: var(--sp-6);
  padding-bottom: var(--sp-7);
}

.section-title {
  font-size: var(--fs-lg);
  font-weight: var(--fw-medium);
  margin: 0 0 var(--sp-5);
  color: var(--color-text);
}

/* ===== 表单行布局 ===== */
.form-row {
  display: grid;
  gap: var(--sp-6);
}
.form-row.row-3 {
  grid-template-columns: repeat(3, 1fr);
}
.form-row.row-2 {
  grid-template-columns: repeat(2, 1fr);
}
.form-row + .form-row {
  margin-top: var(--sp-4);
}

/* ===== el-form-item 自定义 ===== */
:deep(.el-form-item) {
  margin-bottom: 0;
  display: flex;
  flex-direction: column;
}
:deep(.el-form-item__label) {
  font-size: var(--fs-sm);
  font-weight: var(--fw-medium);
  color: var(--color-text-secondary);
  line-height: 1.4;
  padding: 0 0 var(--sp-2);
  justify-content: flex-start;
}
:deep(.el-form-item__content) {
  line-height: 1;
}

/* ===== 下拉框 / 数字输入框统一样式 ===== */
:deep(.el-select),
:deep(.el-input-number) {
  width: 100%;
}
:deep(.el-select__wrapper),
:deep(.el-input-number .el-input__wrapper) {
  background: var(--color-bg);
  border-radius: var(--radius-md);
  box-shadow: 0 0 0 1px var(--color-border) inset;
  padding: 0 var(--sp-4);
  min-height: 44px;
  transition: all var(--t-fast) var(--ease);
}
:deep(.el-select__wrapper:hover),
:deep(.el-input-number .el-input__wrapper:hover) {
  box-shadow: 0 0 0 1px var(--color-accent) inset;
}
:deep(.el-select__wrapper.is-focused),
:deep(.el-input-number .el-input__wrapper.is-focus) {
  box-shadow: 0 0 0 1px var(--color-accent) inset, 0 0 0 3px var(--color-accent-light);
}
:deep(.el-select__placeholder) {
  color: var(--color-text-muted);
}
:deep(.el-select__selected-item) {
  color: var(--color-text);
  font-size: var(--fs-base);
}
:deep(.el-input__inner) {
  color: var(--color-text);
  font-size: var(--fs-base);
  background: transparent;
  height: 42px;
}
:deep(.el-input__inner::placeholder) {
  color: var(--color-text-muted);
}
:deep(.el-input-number__decrease),
:deep(.el-input-number__increase) {
  background: transparent;
  color: var(--color-text-secondary);
  border-color: var(--color-border);
}
:deep(.el-input-number__decrease:hover),
:deep(.el-input-number__increase:hover) {
  color: var(--color-accent);
}

/* ===== 自定义开关 + 文本 ===== */
.custom-toggle {
  display: flex;
  align-items: center;
  gap: var(--sp-3);
  min-height: 44px;
}
.toggle-text {
  font-size: var(--fs-base);
  font-weight: var(--fw-medium);
  transition: color var(--t-fast) var(--ease);
}
.toggle-text.active {
  color: var(--color-text);
}
.toggle-text.muted {
  color: var(--color-text-muted);
}
:deep(.el-switch) {
  --el-switch-on-color: var(--color-accent);
  --el-switch-off-color: var(--color-border);
  height: 24px;
}
:deep(.el-switch__core) {
  width: 44px !important;
  min-width: 44px;
  height: 24px;
  border-radius: var(--radius-full);
}
:deep(.el-switch__core .el-switch__action) {
  width: 20px;
  height: 20px;
  left: 2px;
}
:deep(.el-switch.is-checked .el-switch__core .el-switch__action) {
  left: calc(100% - 22px);
}

/* ===== 车况评级胶囊按钮 ===== */
.condition-pills {
  display: flex;
  flex-wrap: wrap;
  gap: var(--sp-3);
}
:deep(.condition-pills .el-radio-button__inner) {
  border-radius: var(--radius-full);
  border: 1px solid var(--color-border);
  background: transparent;
  color: var(--color-text-secondary);
  font-size: var(--fs-base);
  font-weight: var(--fw-medium);
  padding: var(--sp-2) var(--sp-5);
  box-shadow: none;
  transition: all var(--t-fast) var(--ease);
}
:deep(.condition-pills .el-radio-button__inner:hover) {
  border-color: var(--color-accent);
  color: var(--color-accent);
}
:deep(.condition-pills .el-radio-button.is-active .el-radio-button__inner) {
  background: var(--color-accent);
  border-color: var(--color-accent);
  color: var(--color-text-inverse);
  box-shadow: none;
}
:deep(.condition-pills .el-radio-button:first-child .el-radio-button__inner) {
  border-radius: var(--radius-full);
}
:deep(.condition-pills .el-radio-button:last-child .el-radio-button__inner) {
  border-radius: var(--radius-full);
}

/* ===== 底部固定操作栏 ===== */
.bottom-action-bar {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  z-index: 50;
  background: var(--color-surface);
  border-top: 1px solid var(--color-border);
  padding: var(--sp-4) var(--sp-7);
}
.bottom-action-inner {
  max-width: var(--container-max);
  margin: 0 auto;
  display: flex;
  justify-content: flex-end;
  gap: var(--sp-3);
}

/* ===== 移动端适配 ===== */
@media (max-width: 1024px) {
  .form-row.row-3,
  .form-row.row-2 {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 768px) {
  .input-page {
    padding: 0 0 var(--sp-16);
  }
  .input-section {
    margin-bottom: var(--sp-4);
    padding: var(--sp-5) var(--sp-4);
    border-radius: var(--radius-lg);
  }
  .section-title {
    margin: 0 0 var(--sp-4);
  }
  .stats-card {
    margin-bottom: var(--sp-4);
    padding: var(--sp-5) var(--sp-4);
    border-radius: var(--radius-lg);
  }
  .stats-card-value .num {
    font-size: 32px;
  }
  .form-row.row-3,
  .form-row.row-2 {
    grid-template-columns: 1fr;
    gap: var(--sp-4);
  }
  .form-row + .form-row {
    margin-top: var(--sp-4);
  }
  .bottom-action-bar {
    padding: var(--sp-3) var(--sp-4);
  }
  .bottom-action-inner {
    justify-content: stretch;
  }
  .bottom-action-inner :deep(.el-button) {
    flex: 1;
  }
  :deep(.condition-pills .el-radio-button__inner) {
    padding: var(--sp-2) var(--sp-4);
  }
}
</style>
