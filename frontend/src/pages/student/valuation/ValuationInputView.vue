<script setup lang="ts">
// 叉车残值评估参数录入（统一表单，Tesla 极简风）
// 三行级联布局：
//   行1 品牌类型：品牌 → 车辆类型（brand_type 由品牌自动派生）
//   行2 系列吨位：系列 → 吨位
//   行3 配置门架：配置类型 → 门架类型 → 门架高度
// "无" 选项：series / config_type / mast_type 可选 "无"；mast_height_mm 用 0 表示 "无"
import { computed, onMounted, ref, watch } from 'vue'
import { Refresh, Promotion } from '@element-plus/icons-vue'
import PageHeader from '@/components/valuation/PageHeader.vue'
import { useEvaluationForm, NONE_VALUE, NONE_MAST_HEIGHT } from '@/composables/useEvaluationForm'
import {
  listBrands,
  listVehicleTypes,
  listSeries,
  listTonnages,
  listConfigTypes,
  listMastTypes,
  listMastHeights,
  listBatteryTypes,
  listConditionRatings,
  listProvinces,
  listCities
} from '@/api/valuation/dictionaries'
import type {
  VehicleTypeOption,
  SeriesOption,
  TonnageOption,
  ConfigTypeOption,
  MastTypeOption,
  MastHeightOption,
  BatteryTypeOption,
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
const batteryTypes = ref<BatteryTypeOption[]>([])
const conditionRatings = ref<ConditionRatingOption[]>([])
const provinces = ref<string[]>([])
const cities = ref<string[]>([])
const loadingDict = ref(false)

// ========== "无" 选项（前端常量，附加到下拉列表末尾） ==========
const noneSeriesOption: SeriesOption = { id: -1, brand: '', name: NONE_VALUE }
const noneConfigOption: ConfigTypeOption = { id: -1, name: NONE_VALUE }
const noneMastTypeOption: MastTypeOption = { id: -1, name: NONE_VALUE }
const noneMastHeightOption: MastHeightOption = { id: -1, value_mm: NONE_MAST_HEIGHT }

// 合并 "无" 选项后的可选项列表
const seriesOptions = computed(() => [...seriesList.value, noneSeriesOption])
const configTypeOptions = computed(() => [...configTypes.value, noneConfigOption])
const mastTypeOptions = computed(() => [...mastTypes.value, noneMastTypeOption])
const mastHeightOptions = computed(() => [...mastHeights.value, noneMastHeightOption])

// ========== 表单 ==========
// hasElectricVehicleType 需在 form 之前定义
const hasElectricVehicleType = computed(() =>
  vehicleTypes.value.some((v) => v.power_type === 'electric')
)
const { form, submitting, isValid, reset, submit } = useEvaluationForm({
  hasElectricVehicleType
})

// 当前选中的车辆类型是否电动
const isElectricVehicle = computed(() => {
  if (!form.vehicle_type) return false
  const vt = vehicleTypes.value.find((v) => v.name === form.vehicle_type)
  return vt?.power_type === 'electric'
})

// 电池类型字段可见性：仅电动车辆且字典含电池类型时显示
const showBatteryType = computed(
  () => batteryTypes.value.length > 0 && isElectricVehicle.value
)

// ========== 级联加载 ==========
// 级联顺序：品牌 → 车辆类型 → 系列 → 吨位 → 配置类型 → 门架类型 → 门架高度

// 品牌 → 车辆类型 + 自动填充 brand_type
watch(
  () => form.brand,
  async (b) => {
    // 清空下游全部字段
    form.vehicle_type = undefined
    form.series = undefined
    form.tonnage = undefined
    form.config_type = undefined
    form.mast_type = undefined
    form.mast_height_mm = undefined
    vehicleTypes.value = []
    seriesList.value = []
    tonnages.value = []
    configTypes.value = []
    mastTypes.value = []
    mastHeights.value = []

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
    form.config_type = undefined
    form.mast_type = undefined
    form.mast_height_mm = undefined
    seriesList.value = []
    tonnages.value = []
    configTypes.value = []
    mastTypes.value = []
    mastHeights.value = []

    // 非电动车辆清空电池类型
    if (vt) {
      const matched = vehicleTypes.value.find((v) => v.name === vt)
      if (matched && matched.power_type !== 'electric') {
        form.battery_type = undefined
      }
    }

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
    tonnages.value = []
    configTypes.value = []
    mastTypes.value = []
    mastHeights.value = []

    if (!form.brand || !form.vehicle_type || !s) return
    // 系列为 "无" 时，吨位查询不传 series 参数（后端返回该品牌+车型的全部吨位）
    const seriesParam = s === NONE_VALUE ? undefined : s
    tonnages.value = await listTonnages(form.brand, form.vehicle_type, seriesParam)
  }
)

// 吨位 → 配置类型
watch(
  () => form.tonnage,
  async (t) => {
    form.config_type = undefined
    form.mast_type = undefined
    form.mast_height_mm = undefined
    configTypes.value = []
    mastTypes.value = []
    mastHeights.value = []

    if (!form.brand || !form.vehicle_type || !form.series || t == null) return
    configTypes.value = await listConfigTypes(
      form.brand, form.vehicle_type, form.series, t
    )
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
    mastTypes.value = await listMastTypes(
      form.brand, form.vehicle_type, form.series, form.tonnage, ct
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
    mastHeights.value = await listMastHeights(
      form.brand, form.vehicle_type, form.series, form.tonnage, form.config_type, mt
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
    const [brandsList, batList, crList, provList] = await Promise.all([
      listBrands(),
      listBatteryTypes(),
      listConditionRatings(),
      listProvinces()
    ])
    brands.value = brandsList
    batteryTypes.value = batList
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

        <!-- 行2：系列吨位（系列 → 吨位） -->
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
        </el-row>

        <!-- 行3：配置门架（配置类型 → 门架类型 → 门架高度） -->
        <el-row v-if="showConfigType" :gutter="24">
          <el-col :xs="24" :md="12" :lg="6">
            <el-form-item label="配置类型" required>
              <el-select
                v-model="form.config_type"
                placeholder="请选择配置类型"
                filterable
                clearable
                :disabled="form.tonnage == null"
              >
                <el-option
                  v-for="c in configTypeOptions"
                  :key="c.id"
                  :value="c.name"
                  :label="c.name"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col v-if="showMastType" :xs="24" :md="12" :lg="6">
            <el-form-item label="门架类型" required>
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

        <!-- 电池类型（仅电动车辆显示） -->
        <el-row v-if="showBatteryType" :gutter="24">
          <el-col :xs="24" :md="12" :lg="6">
            <el-form-item label="电池类型">
              <el-select
                v-model="form.battery_type"
                placeholder="请选择电池类型"
                filterable
                clearable
              >
                <el-option
                  v-for="b in batteryTypes"
                  :key="b.id"
                  :value="b.name"
                  :label="b.name"
                />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
      </section>

      <!-- 使用信息：年份 / 工时 / 原漆 -->
      <section class="input-section card-surface">
        <h2 class="section-title">使用信息</h2>
        <el-row :gutter="24">
          <el-col :xs="24" :md="12" :lg="6">
            <el-form-item label="出厂年份" required>
              <el-input-number
                v-model="form.factory_year"
                :min="1980"
                :max="new Date().getFullYear()"
                :step="1"
                style="width: 100%"
                placeholder="如 2021"
              />
            </el-form-item>
          </el-col>
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
