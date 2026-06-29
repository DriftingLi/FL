<script setup lang="ts">
// 叉车残值评估参数录入（统一表单，Tesla 极简风）
// 配置类型重构：移除 battery_type 独立字段，改为三维度（传动/发动机/电池）
// 三行级联布局：
//   行1 品牌类型：品牌 → 车辆类型（brand_type 由品牌自动派生）
//   行2 系列吨位：系列 → 吨位 → 出厂年份（出厂年份 min 由所选系列 earliest_factory_year 决定）
//   行3 配置维度：传动系统 / 发动机类型 / 电池配置（按 series 支持的维度显示）
//   行4 配置门架：配置类型(只读 computed) → 门架类型 → 门架高度
// "无" 选项：series / mast_type 可选 "无"；mast_height_mm 用 0 表示 "无"
import { computed, onMounted, ref, watch } from 'vue'
import { Refresh, Promotion } from '@element-plus/icons-vue'
import PageHeader from '@/components/valuation/PageHeader.vue'
import { useEvaluationForm, NONE_VALUE, NONE_MAST_HEIGHT } from '@/composables/useEvaluationForm'
import {
  listBrands,
  listVehicleTypes,
  listSeries,
  listTonnages,
  listMastTypes,
  listMastHeights,
  listSeriesConfigOptions,
  listConditionRatings,
  listProvinces,
  listCities
} from '@/api/valuation/dictionaries'
import type {
  VehicleTypeOption,
  SeriesOption,
  TonnageOption,
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
const mastTypes = ref<MastTypeOption[]>([])
const mastHeights = ref<MastHeightOption[]>([])
const conditionRatings = ref<ConditionRatingOption[]>([])
const provinces = ref<string[]>([])
const cities = ref<string[]>([])
const loadingDict = ref(false)

// ========== "无" 选项（前端常量，附加到下拉列表末尾） ==========
const noneSeriesOption: SeriesOption = { id: -1, brand: '', name: NONE_VALUE, earliest_factory_year: 1980 }
const noneMastTypeOption: MastTypeOption = { id: -1, name: NONE_VALUE }
const noneMastHeightOption: MastHeightOption = { id: -1, value_mm: NONE_MAST_HEIGHT }

// 合并 "无" 选项后的可选项列表
const seriesOptions = computed(() => [...seriesList.value, noneSeriesOption])
const mastTypeOptions = computed(() => [...mastTypes.value, noneMastTypeOption])
const mastHeightOptions = computed(() => [...mastHeights.value, noneMastHeightOption])

// ========== 表单 ==========
const { form, dimensionOptions, configType, submitting, isValid, reset, submit } = useEvaluationForm()

// 当前所选系列的最早出厂年份（未选系列时返回默认下限 1980）
const currentSeriesEarliestYear = computed(() => {
  if (!form.series) return 1980
  const s = seriesList.value.find((it) => it.name === form.series)
  return s?.earliest_factory_year ?? 1980
})

// 出厂年份字段可见性：选完吨位后才显示
const showFactoryYear = computed(() => form.tonnage != null)

// 维度可见性：仅当对应维度选项数组非空时显示
const showTransmission = computed(() => dimensionOptions.transmission.length > 0)
const showEngine = computed(() => dimensionOptions.engine.length > 0)
const showBattery = computed(() => dimensionOptions.battery.length > 0)
// 任一维度启用 → 显示"配置维度"区域
const showConfigDimensions = computed(
  () => showTransmission.value || showEngine.value || showBattery.value
)

// ========== 级联加载 ==========
// 级联顺序：品牌 → 车辆类型 → 系列 → 吨位 →（出厂年份 + 配置维度）→ 门架类型 → 门架高度

// 品牌 → 车辆类型 + 自动填充 brand_type
watch(
  () => form.brand,
  async (b) => {
    // 清空下游全部字段
    form.vehicle_type = undefined
    form.series = undefined
    form.tonnage = undefined
    form.transmission = undefined
    form.engine = undefined
    form.battery = undefined
    form.mast_type = undefined
    form.mast_height_mm = undefined
    form.factory_year = undefined
    vehicleTypes.value = []
    seriesList.value = []
    tonnages.value = []
    mastTypes.value = []
    mastHeights.value = []
    dimensionOptions.transmission = []
    dimensionOptions.engine = []
    dimensionOptions.battery = []

    if (!b) {
      form.brand_type = undefined
      return
    }
    // 自动填充 brand_type（从品牌字典派生，无需用户手选）
    const brandData = brands.value.find((br) => br.name === b)
    form.brand_type = brandData?.brand_type
    // 加载该品牌可选车辆类型
    vehicleTypes.value = await listVehicleTypes(b)
  }
)

// 车辆类型 → 系列
watch(
  () => form.vehicle_type,
  async (vt) => {
    form.series = undefined
    form.tonnage = undefined
    form.transmission = undefined
    form.engine = undefined
    form.battery = undefined
    form.mast_type = undefined
    form.mast_height_mm = undefined
    form.factory_year = undefined
    seriesList.value = []
    tonnages.value = []
    mastTypes.value = []
    mastHeights.value = []
    dimensionOptions.transmission = []
    dimensionOptions.engine = []
    dimensionOptions.battery = []

    if (!form.brand || !vt) return
    seriesList.value = await listSeries(form.brand, vt)
  }
)

// 系列 → 吨位 + 加载维度配置选项
watch(
  () => form.series,
  async (s) => {
    form.tonnage = undefined
    form.transmission = undefined
    form.engine = undefined
    form.battery = undefined
    form.mast_type = undefined
    form.mast_height_mm = undefined
    form.factory_year = undefined
    tonnages.value = []
    mastTypes.value = []
    mastHeights.value = []
    dimensionOptions.transmission = []
    dimensionOptions.engine = []
    dimensionOptions.battery = []

    if (!form.brand || !form.vehicle_type || !s) return

    // 并行加载：吨位列表 + 维度配置选项
    // 系列为 "无" 时，吨位查询不传 series 参数；维度配置仅对真实 series 查询
    const seriesParam = s === NONE_VALUE ? undefined : s
    const [tonnageList, opts] = await Promise.all([
      listTonnages(form.brand, form.vehicle_type, seriesParam),
      s === NONE_VALUE
        ? Promise.resolve({ transmission: [], engine: [], battery: [] })
        : listSeriesConfigOptions(form.brand, s)
    ])
    tonnages.value = tonnageList
    dimensionOptions.transmission = opts.transmission ?? []
    dimensionOptions.engine = opts.engine ?? []
    dimensionOptions.battery = opts.battery ?? []
  }
)

// 吨位 → 出厂年份字段解锁（出厂年份由 currentSeriesEarliestYear 限制 min）
// 配置维度选项已在 series watcher 中加载，吨位变化只需清空下游门架相关字段
watch(
  () => form.tonnage,
  async () => {
    form.mast_type = undefined
    form.mast_height_mm = undefined
    mastTypes.value = []
    mastHeights.value = []
  }
)

// 任一维度变化 → 清空下游门架字段（config_type 由 computed 自动更新）
watch(
  [() => form.transmission, () => form.engine, () => form.battery],
  async () => {
    form.mast_type = undefined
    form.mast_height_mm = undefined
    mastTypes.value = []
    mastHeights.value = []

    // 当三维度都已选（或维度未启用）且 config_type 非空时，加载门架类型
    const ct = configType.value
    if (!ct || !form.brand || !form.vehicle_type || !form.series || form.tonnage == null) return
    const seriesParam = form.series === NONE_VALUE ? undefined : form.series
    // listMastTypes 需要 series 参数，"无" 时传 undefined
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
        !configType.value || !mt) return
    const seriesParam = form.series === NONE_VALUE ? undefined : form.series
    mastHeights.value = await listMastHeights(
      form.brand, form.vehicle_type, seriesParam ?? form.series, form.tonnage, configType.value, mt
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
})

// ========== 字段可见性 ==========
const showBrand = computed(() => brands.value.length > 0)
const showVehicleType = computed(() => vehicleTypes.value.length > 0)
const showSeries = computed(() => form.vehicle_type !== undefined)
const showTonnage = computed(() => form.series !== undefined)
const showMastType = computed(() => form.tonnage != null && configType.value !== '')
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

    <div v-loading="loadingDict">
      <!-- 品牌与车型（三行级联） -->
      <section class="input-section card-surface">
        <h2 class="section-title">品牌与车型</h2>

        <!-- 行1：品牌类型（品牌 → 车辆类型） -->
        <el-row :gutter="24">
          <el-col v-if="showBrand" :xs="24" :md="12" :lg="6">
            <el-form-item label="品牌" required>
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
          </el-col>
          <el-col v-if="showVehicleType" :xs="24" :md="12" :lg="6">
            <el-form-item label="车辆类型" required>
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
          </el-col>
        </el-row>

        <!-- 行2：系列吨位（系列 → 吨位 → 出厂年份） -->
        <el-row v-if="showSeries" :gutter="24">
          <el-col :xs="24" :md="12" :lg="6">
            <el-form-item label="系列" required>
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
          </el-col>
          <el-col v-if="showTonnage" :xs="24" :md="12" :lg="6">
            <el-form-item label="吨位" required>
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
          </el-col>
          <el-col v-if="showFactoryYear" :xs="24" :md="12" :lg="6">
            <el-form-item label="出厂年份" required>
              <el-input-number
                v-model="form.factory_year"
                :min="currentSeriesEarliestYear"
                :max="new Date().getFullYear()"
                :step="1"
                :disabled="form.tonnage == null"
                style="width: 100%"
                placeholder="如 2021"
              />
            </el-form-item>
          </el-col>
        </el-row>

        <!-- 行3：配置维度（传动系统 / 发动机类型 / 电池配置） -->
        <el-row v-if="showConfigDimensions" :gutter="24">
          <el-col v-if="showTransmission" :xs="24" :md="12" :lg="6">
            <el-form-item label="传动系统" required>
              <el-select
                v-model="form.transmission"
                placeholder="请选择传动系统"
                filterable
                clearable
              >
                <el-option
                  v-for="t in dimensionOptions.transmission"
                  :key="t"
                  :value="t"
                  :label="t"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col v-if="showEngine" :xs="24" :md="12" :lg="6">
            <el-form-item label="发动机类型" required>
              <el-select
                v-model="form.engine"
                placeholder="请选择发动机类型"
                filterable
                clearable
              >
                <el-option
                  v-for="e in dimensionOptions.engine"
                  :key="e"
                  :value="e"
                  :label="e"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col v-if="showBattery" :xs="24" :md="12" :lg="6">
            <el-form-item label="电池配置" required>
              <el-select
                v-model="form.battery"
                placeholder="请选择电池配置"
                filterable
                clearable
              >
                <el-option
                  v-for="b in dimensionOptions.battery"
                  :key="b"
                  :value="b"
                  :label="b"
                />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>

        <!-- 行4：配置门架（配置类型只读 → 门架类型 → 门架高度） -->
        <el-row v-if="showMastType" :gutter="24">
          <el-col :xs="24" :md="12" :lg="6">
            <el-form-item label="配置类型">
              <el-input
                :model-value="configType"
                placeholder="请先完成上方配置维度"
                readonly
              />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12" :lg="6">
            <el-form-item label="门架类型" required>
              <el-select
                v-model="form.mast_type"
                placeholder="请选择门架类型"
                filterable
                clearable
                :disabled="!configType"
              >
                <el-option
                  v-for="m in mastTypeOptions"
                  :key="m.id"
                  :value="m.name"
                  :label="m.name"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col v-if="showMastHeight" :xs="24" :md="12" :lg="6">
            <el-form-item label="门架高度" required>
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
          </el-col>
        </el-row>
      </section>

      <!-- 使用信息：工时 / 原漆 -->
      <section class="input-section card-surface">
        <h2 class="section-title">使用信息</h2>
        <el-row :gutter="24">
          <el-col :xs="24" :md="12" :lg="6">
            <el-form-item label="累计工时" required>
              <el-input-number
                v-model="form.usage_hours"
                :min="0"
                :max="100000"
                :step="100"
                style="width: 100%"
                placeholder="如 3500"
              />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12" :lg="6">
            <el-form-item label="是否原厂原漆" required>
              <el-switch
                v-model="form.original_paint"
                active-text="原厂原漆"
                inactive-text="非原厂"
              />
            </el-form-item>
          </el-col>
        </el-row>
      </section>

      <!-- 区域信息：省 → 市 -->
      <section v-if="showProvince" class="input-section card-surface">
        <h2 class="section-title">所在区域</h2>
        <el-row :gutter="24">
          <el-col :xs="24" :md="12" :lg="6">
            <el-form-item label="省份" required>
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
          </el-col>
          <el-col v-if="showCity" :xs="24" :md="12" :lg="6">
            <el-form-item label="城市" required>
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
          </el-col>
        </el-row>
      </section>

      <!-- 证件与保养 -->
      <section class="input-section card-surface">
        <h2 class="section-title">证件与保养</h2>
        <el-row :gutter="24">
          <el-col :xs="24" :md="8">
            <el-form-item label="是否有车牌">
              <el-switch v-model="form.has_license_plate" />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="8">
            <el-form-item label="特种设备登记证">
              <el-switch v-model="form.has_registration_certificate" />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="8">
            <el-form-item label="是否有保养记录">
              <el-switch v-model="form.has_maintenance_records" />
            </el-form-item>
          </el-col>
        </el-row>
      </section>

      <!-- 车况评级 -->
      <section v-if="showConditionRating" class="input-section card-surface">
        <h2 class="section-title">车况评级</h2>
        <el-form-item label="车况评级" required>
          <el-radio-group v-model="form.condition_rating">
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
  </div>
</template>

<style scoped>
.input-page {
  padding: 0;
}
.input-section {
  margin-bottom: var(--sp-6);
}
.section-title {
  font-size: var(--fs-lg);
  font-weight: var(--fw-medium);
  margin: 0 0 var(--sp-5);
  color: var(--color-text);
}

/* ===== 移动端适配 ===== */
@media (max-width: 768px) {
  .input-section {
    margin-bottom: var(--sp-4);
    padding: var(--sp-5) var(--sp-4);
  }
  .section-title {
    margin: 0 0 var(--sp-4);
  }
}
</style>
