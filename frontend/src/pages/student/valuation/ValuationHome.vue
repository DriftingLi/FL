<script setup lang="ts">
// 残值评估首页（设计稿风格：白底居中表单 + 自定义控件 + 底部固定操作栏）
// 改动：
// 1) 出厂年份改为下拉框（按 earliest_factory_year → 当前年 动态生成）
// 2) 所有字段一开始就出现，按级联逻辑 :disabled 锁定，而非 v-if
// 3) 操作栏文字允许换行 + 768px 以下提前变竖排
// 保留 useEvaluationForm composable 的级联加载、校验、提交等全部逻辑
import { computed, onMounted, ref, watch } from 'vue'
import {
  useEvaluationForm,
  OTHER_SERIES_VALUE,
  NONE_VALUE,
  NONE_MAST_HEIGHT
} from '@/composables/useEvaluationForm'
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
import type {
  SeriesOption,
  MastTypeOption,
  MastHeightOption,
  ConditionRatingOption,
  ConditionRating
} from '@/types/valuation/evaluation'
import type { Brand } from '@/types/valuation/brand'

// ========== 字典数据 ==========
const brands = ref<Brand[]>([])
const seriesList = ref<SeriesOption[]>([])
const tonnages = ref<number[]>([])
const configTypes = ref<{ id: number; name: string }[]>([])
const mastTypes = ref<MastTypeOption[]>([])
const mastHeights = ref<MastHeightOption[]>([])
const conditionRatings = ref<ConditionRatingOption[]>([])
const provinces = ref<string[]>([])
const cities = ref<string[]>([])
const vehicleTypes = ref<string[]>([])
const loadingDict = ref(false)

// ========== "其它"/"无" 选项（前端常量，附加到下拉列表末尾） ==========
const otherSeriesOption: SeriesOption = {
  id: -1,
  brand: '',
  name: OTHER_SERIES_VALUE,
  earliest_factory_year: 1980
}
const noneMastTypeOption: MastTypeOption = { id: -1, name: NONE_VALUE }
const noneMastHeightOption: MastHeightOption = { id: -1, value_mm: NONE_MAST_HEIGHT }

const seriesOptions = computed(() => {
  if (seriesList.value.some((s) => s.name === OTHER_SERIES_VALUE)) return seriesList.value
  return [...seriesList.value, otherSeriesOption]
})
const mastTypeOptions = computed(() => {
  if (mastTypes.value.some((m) => m.name === NONE_VALUE)) return mastTypes.value
  return [...mastTypes.value, noneMastTypeOption]
})
const mastHeightOptions = computed(() => {
  if (mastHeights.value.some((mh) => mh.value_mm === NONE_MAST_HEIGHT)) return mastHeights.value
  return [...mastHeights.value, noneMastHeightOption]
})

// ========== 表单 ==========
const { form, submitting, isValid, reset, submit } = useEvaluationForm()

// 选完吨位后从 original_prices 级联查询 MIN(earliest_factory_year)
const earliestFactoryYear = ref(1980)
const currentYear = new Date().getFullYear()

// 出厂年份下拉框选项：最新年份排在前
const factoryYearOptions = computed(() => {
  const years: number[] = []
  for (let y = earliestFactoryYear.value; y <= currentYear; y++) {
    years.push(y)
  }
  return years.reverse()
})

// ========== 级联加载 ==========
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
    const list = await listVehicleTypes(b)
    vehicleTypes.value = list.map((v) => v.name)
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
    const seriesParam = s === OTHER_SERIES_VALUE ? undefined : s
    const list = await listTonnages(form.brand, form.vehicle_type, seriesParam)
    tonnages.value = list.map((t) => t.value)
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
      form.brand,
      form.vehicle_type,
      seriesParam ?? form.series,
      form.tonnage,
      ct
    )
  }
)

// 门架类型 → 门架高度
watch(
  () => form.mast_type,
  async (mt) => {
    form.mast_height_mm = undefined
    mastHeights.value = []

    if (
      !form.brand ||
      !form.vehicle_type ||
      !form.series ||
      form.tonnage == null ||
      !form.config_type ||
      !mt
    )
      return
    const seriesParam = form.series === OTHER_SERIES_VALUE ? undefined : form.series
    mastHeights.value = await listMastHeights(
      form.brand,
      form.vehicle_type,
      seriesParam ?? form.series,
      form.tonnage,
      form.config_type,
      mt
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

// 车况评级按 rating 排序展示
const sortedConditionRatings = computed(() =>
  [...conditionRatings.value].sort((a, b) => a.rating.localeCompare(b.rating))
)

// 车况评级描述（与设计稿保持一致）
const RATING_DESC: Record<ConditionRating, string> = {
  A: '车况极佳，几乎无损耗',
  B: '正常使用磨损',
  C: '明显磨损或维修',
  D: '严重损耗或故障',
  E: '需大修或报废'
}

function onSubmit() {
  submit()
}

function onConditionSelect(rating: ConditionRating) {
  // 再次点击同一项则取消选择
  form.condition_rating = form.condition_rating === rating ? undefined : rating
}

// ========== 字符串字段代理（空字符串 ↔ undefined 互转） ==========
// 原生 select 配 :value=undefined 占位时，DOM value 会回退为 textContent，
// 污染 form 状态。这里用计算属性包裹 v-model，在 set 时把空串归一为 undefined。
function useStringField(getter: () => string | undefined) {
  return computed<string>({
    get: () => getter() ?? '',
    set: (v) => {
      // 通过闭包反射回 form：调用方传入 () => form.xxx
      // 用 Function 保持 setter 简洁（避免在模板里写箭头函数）
      const setter = getter as unknown as { __setVal?: (v: string) => void }
      setter.__setVal?.(v)
    }
  })
}
</script>

<template>
  <div class="valuation-home valuation-root" v-loading="loadingDict">
    <section class="form-section">
      <!-- Minimal brand strip（与设计稿保持一致：HRWAI | 叉车残值评估） -->
      <div class="brand-strip">
        <div class="brand-strip-inner">
          <span class="brand-name">HRWAI</span>
          <span class="brand-divider">|</span>
          <span class="brand-sub">叉车残值评估</span>
        </div>
      </div>

      <div class="form-container">
        <!-- Section title with gradient underline -->
        <div class="section-heading">
          <h2 class="section-title">填写叉车信息</h2>
          <span class="section-underline" aria-hidden="true"></span>
        </div>

        <!-- Form card -->
        <div class="form-card">
          <div class="form-card-border" aria-hidden="true"></div>

          <!-- Row 1: 品牌 + 车辆类型 -->
          <div class="form-row form-row-2">
            <div class="field">
              <label class="field-label" for="vh-brand">品牌</label>
              <div class="select-wrap">
                <select
                  id="vh-brand"
                  v-model="form.brand"
                  class="form-control"
                  :disabled="brands.length === 0"
                >
                  <option :value="undefined" disabled>请选择品牌</option>
                  <option v-for="b in brands" :key="b.id" :value="b.name">{{ b.name }}</option>
                </select>
                <span class="select-icon" aria-hidden="true">
                  <svg
                    width="16"
                    height="16"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  >
                    <polyline points="6 9 12 15 18 9" />
                  </svg>
                </span>
              </div>
            </div>
            <div class="field">
              <label class="field-label" for="vh-vehicle-type">车辆类型</label>
              <div class="select-wrap">
                <select
                  id="vh-vehicle-type"
                  v-model="form.vehicle_type"
                  class="form-control"
                  :disabled="!form.brand || vehicleTypes.length === 0"
                >
                  <option :value="undefined" disabled>请选择车辆类型</option>
                  <option v-for="vt in vehicleTypes" :key="vt" :value="vt">{{ vt }}</option>
                </select>
                <span class="select-icon" aria-hidden="true">
                  <svg
                    width="16"
                    height="16"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  >
                    <polyline points="6 9 12 15 18 9" />
                  </svg>
                </span>
              </div>
            </div>
          </div>

          <!-- Row 2: 系列 + 吨位 + 出厂年份 -->
          <div class="form-row form-row-3">
            <div class="field">
              <label class="field-label" for="vh-series">系列</label>
              <div class="select-wrap">
                <select
                  id="vh-series"
                  v-model="form.series"
                  class="form-control"
                  :disabled="!form.vehicle_type"
                >
                  <option :value="undefined" disabled>请选择系列</option>
                  <option v-for="s in seriesOptions" :key="s.id" :value="s.name">{{ s.name }}</option>
                </select>
                <span class="select-icon" aria-hidden="true">
                  <svg
                    width="16"
                    height="16"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  >
                    <polyline points="6 9 12 15 18 9" />
                  </svg>
                </span>
              </div>
            </div>
            <div class="field">
              <label class="field-label" for="vh-tonnage">吨位</label>
              <div class="select-wrap">
                <select
                  id="vh-tonnage"
                  v-model="form.tonnage"
                  class="form-control"
                  :disabled="!form.series"
                >
                  <option :value="undefined" disabled>请选择吨位</option>
                  <option v-for="t in tonnages" :key="t" :value="t">{{ t }} 吨</option>
                </select>
                <span class="select-icon" aria-hidden="true">
                  <svg
                    width="16"
                    height="16"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  >
                    <polyline points="6 9 12 15 18 9" />
                  </svg>
                </span>
              </div>
            </div>
            <div class="field">
              <label class="field-label" for="vh-factory-year">出厂年份</label>
              <div class="select-wrap">
                <select
                  id="vh-factory-year"
                  v-model="form.factory_year"
                  class="form-control"
                  :disabled="form.tonnage == null"
                >
                  <option :value="undefined" disabled>请选择出厂年份</option>
                  <option v-for="y in factoryYearOptions" :key="y" :value="y">{{ y }} 年</option>
                </select>
                <span class="select-icon" aria-hidden="true">
                  <svg
                    width="16"
                    height="16"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  >
                    <polyline points="6 9 12 15 18 9" />
                  </svg>
                </span>
              </div>
            </div>
          </div>

          <!-- Row 3: 配置类型 + 门架类型 + 门架高度 -->
          <div class="form-row form-row-3">
            <div class="field">
              <label class="field-label" for="vh-config-type">配置类型</label>
              <div class="select-wrap">
                <select
                  id="vh-config-type"
                  v-model="form.config_type"
                  class="form-control"
                  :disabled="form.tonnage == null"
                >
                  <option :value="undefined" disabled>请选择</option>
                  <option v-for="c in configTypes" :key="c.id" :value="c.name">
                    {{ c.name }}
                  </option>
                </select>
                <span class="select-icon" aria-hidden="true">
                  <svg
                    width="16"
                    height="16"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  >
                    <polyline points="6 9 12 15 18 9" />
                  </svg>
                </span>
              </div>
            </div>
            <div class="field">
              <label class="field-label" for="vh-mast-type">门架类型</label>
              <div class="select-wrap">
                <select
                  id="vh-mast-type"
                  v-model="form.mast_type"
                  class="form-control"
                  :disabled="!form.config_type"
                >
                  <option :value="undefined" disabled>请选择</option>
                  <option v-for="m in mastTypeOptions" :key="m.id" :value="m.name">
                    {{ m.name }}
                  </option>
                </select>
                <span class="select-icon" aria-hidden="true">
                  <svg
                    width="16"
                    height="16"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  >
                    <polyline points="6 9 12 15 18 9" />
                  </svg>
                </span>
              </div>
            </div>
            <div class="field">
              <label class="field-label" for="vh-mast-height">门架高度</label>
              <div class="select-wrap">
                <select
                  id="vh-mast-height"
                  v-model="form.mast_height_mm"
                  class="form-control"
                  :disabled="!form.mast_type"
                >
                  <option :value="undefined" disabled>请选择</option>
                  <option v-for="mh in mastHeightOptions" :key="mh.id" :value="mh.value_mm">
                    {{ mh.value_mm === NONE_MAST_HEIGHT ? '无' : `${mh.value_mm} mm` }}
                  </option>
                </select>
                <span class="select-icon" aria-hidden="true">
                  <svg
                    width="16"
                    height="16"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  >
                    <polyline points="6 9 12 15 18 9" />
                  </svg>
                </span>
              </div>
            </div>
          </div>

          <!-- Row 4: 累计工时 + 原厂原漆 -->
          <div class="form-row form-row-2">
            <div class="field">
              <label class="field-label" for="vh-usage-hours">累计工时</label>
              <input
                id="vh-usage-hours"
                v-model.number="form.usage_hours"
                type="number"
                class="form-control"
                :min="0"
                :max="100000"
                :step="100"
                placeholder="请输入工时数"
              />
            </div>
            <div class="field">
              <label class="field-label">原厂原漆</label>
              <div class="toggle-row">
                <button
                  type="button"
                  class="toggle-switch"
                  :class="{ 'is-on': form.original_paint }"
                  :aria-pressed="form.original_paint"
                  aria-label="原厂原漆开关"
                  @click="form.original_paint = !form.original_paint"
                >
                  <span class="toggle-thumb"></span>
                </button>
                <span class="toggle-state" :class="{ 'is-on': form.original_paint }">
                  {{ form.original_paint ? '是' : '否' }}
                </span>
              </div>
            </div>
          </div>

          <!-- Row 5: 省份 + 城市 -->
          <div class="form-row form-row-2">
            <div class="field">
              <label class="field-label" for="vh-province">省份</label>
              <div class="select-wrap">
                <select
                  id="vh-province"
                  v-model="form.province"
                  class="form-control"
                >
                  <option :value="undefined" disabled>请选择省份</option>
                  <option v-for="p in provinces" :key="p" :value="p">{{ p }}</option>
                </select>
                <span class="select-icon" aria-hidden="true">
                  <svg
                    width="16"
                    height="16"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  >
                    <polyline points="6 9 12 15 18 9" />
                  </svg>
                </span>
              </div>
            </div>
            <div class="field">
              <label class="field-label" for="vh-city">城市</label>
              <div class="select-wrap">
                <select
                  id="vh-city"
                  v-model="form.city"
                  class="form-control"
                  :disabled="!form.province"
                >
                  <option :value="undefined" disabled>请选择城市</option>
                  <option v-for="c in cities" :key="c" :value="c">{{ c }}</option>
                </select>
                <span class="select-icon" aria-hidden="true">
                  <svg
                    width="16"
                    height="16"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  >
                    <polyline points="6 9 12 15 18 9" />
                  </svg>
                </span>
              </div>
            </div>
          </div>

          <!-- Row 6: 证件与保养 -->
          <div class="form-row form-row-3">
            <div class="field">
              <label class="field-label">是否有车牌</label>
              <div class="toggle-row">
                <button
                  type="button"
                  class="toggle-switch"
                  :class="{ 'is-on': form.has_license_plate }"
                  :aria-pressed="form.has_license_plate"
                  aria-label="是否有车牌开关"
                  @click="form.has_license_plate = !form.has_license_plate"
                >
                  <span class="toggle-thumb"></span>
                </button>
                <span class="toggle-state" :class="{ 'is-on': form.has_license_plate }">
                  {{ form.has_license_plate ? '是' : '否' }}
                </span>
              </div>
            </div>
            <div class="field">
              <label class="field-label">特种设备登记证</label>
              <div class="toggle-row">
                <button
                  type="button"
                  class="toggle-switch"
                  :class="{ 'is-on': form.has_registration_certificate }"
                  :aria-pressed="form.has_registration_certificate"
                  aria-label="特种设备登记证开关"
                  @click="
                    form.has_registration_certificate = !form.has_registration_certificate
                  "
                >
                  <span class="toggle-thumb"></span>
                </button>
                <span
                  class="toggle-state"
                  :class="{ 'is-on': form.has_registration_certificate }"
                >
                  {{ form.has_registration_certificate ? '是' : '否' }}
                </span>
              </div>
            </div>
            <div class="field">
              <label class="field-label">保养记录</label>
              <div class="toggle-row">
                <button
                  type="button"
                  class="toggle-switch"
                  :class="{ 'is-on': form.has_maintenance_records }"
                  :aria-pressed="form.has_maintenance_records"
                  aria-label="保养记录开关"
                  @click="form.has_maintenance_records = !form.has_maintenance_records"
                >
                  <span class="toggle-thumb"></span>
                </button>
                <span class="toggle-state" :class="{ 'is-on': form.has_maintenance_records }">
                  {{ form.has_maintenance_records ? '是' : '否' }}
                </span>
              </div>
            </div>
          </div>

          <!-- 车况评级 -->
          <div class="form-section-block">
            <label class="field-label">车况评级</label>
            <div class="rating-pills">
              <button
                v-for="cr in sortedConditionRatings"
                :key="cr.id"
                type="button"
                class="rating-pill"
                :class="{ 'is-active': form.condition_rating === cr.rating }"
                :aria-pressed="form.condition_rating === cr.rating"
                @click="onConditionSelect(cr.rating)"
              >
                <span class="rating-pill-name">{{ cr.rating }} · {{ cr.label }}</span>
                <span class="rating-pill-desc">{{ RATING_DESC[cr.rating] }}</span>
              </button>
            </div>
          </div>

          <!-- Divider -->
          <div class="form-divider" aria-hidden="true"></div>

          <!-- Bottom action area -->
          <div class="action-bar">
            <div class="action-bar-text">
              <p class="action-bar-hint">完成必填项后即可提交</p>
              <p class="action-bar-sub">评估结果将保留在评估历史中</p>
            </div>
            <div class="action-bar-buttons">
              <button type="button" class="btn btn-outline" @click="reset">重置</button>
              <button
                type="button"
                class="btn btn-primary"
                :disabled="!isValid || submitting"
                @click="onSubmit"
              >
                <template v-if="submitting">
                  <svg
                    class="btn-spinner-icon"
                    width="16"
                    height="16"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    aria-hidden="true"
                  >
                    <path d="M21 12a9 9 0 1 1-6.219-8.56" />
                  </svg>
                  <span>提交中…</span>
                </template>
                <template v-else>提交评估</template>
              </button>
            </div>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.valuation-home {
  background: var(--color-bg-card, #FFFFFF);
  min-height: calc(100vh - var(--header-h, 72px));
}

.form-section {
  padding: 48px 0 80px;
}

/* ===== Brand strip (matches design's minimal header) ===== */
.brand-strip {
  max-width: 720px;
  margin: 0 auto 40px;
  padding: 0 20px;
}
.brand-strip-inner {
  display: flex;
  align-items: center;
  gap: 8px;
}
.brand-name {
  font-family: var(--font-display, 'DM Sans', sans-serif);
  font-size: 20px;
  font-weight: 700;
  letter-spacing: 0.04em;
  color: var(--color-text-primary, #0F172A);
}
.brand-divider {
  font-size: 14px;
  color: var(--color-text-muted, #94A3B8);
  user-select: none;
}
.brand-sub {
  font-size: 14px;
  color: var(--color-brand-500, #0EA5E9);
  font-weight: 500;
}

/* ===== Form container (centered 720px) ===== */
.form-container {
  max-width: 720px;
  margin: 0 auto;
  padding: 0 20px;
}

/* ===== Section heading ===== */
.section-heading {
  text-align: center;
  margin-bottom: 32px;
}
.section-title {
  font-family: var(--font-text, 'Noto Sans SC', sans-serif);
  font-size: 24px;
  font-weight: 700;
  color: var(--color-text-primary, #0F172A);
  margin: 0 0 12px;
  letter-spacing: -0.01em;
}
.section-underline {
  display: block;
  width: 96px;
  height: 2px;
  margin: 0 auto;
  background: var(--gradient-brand, linear-gradient(135deg, #0EA5E9 0%, #14B8A6 100%));
  border-radius: 9999px;
}

/* ===== Form card ===== */
.form-card {
  position: relative;
  background: rgba(255, 255, 255, 0.95);
  border: 1px solid var(--color-border, #E2E8F0);
  border-radius: 12px;
  padding: 24px;
  box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.1), 0 4px 6px -2px rgba(0, 0, 0, 0.05);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
}
.form-card-border {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 2px;
  background: var(--gradient-brand, linear-gradient(135deg, #0EA5E9 0%, #14B8A6 100%));
  border-radius: 12px 12px 0 0;
  pointer-events: none;
}

/* ===== Form rows ===== */
.form-row {
  display: grid;
  gap: 16px;
  margin-bottom: 16px;
}
.form-row-2 {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}
.form-row-3 {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

/* ===== Field ===== */
.field {
  min-width: 0;
  display: flex;
  flex-direction: column;
}
.field-label {
  display: block;
  font-size: 14px;
  font-weight: 500;
  margin-bottom: 6px;
  color: var(--color-text-secondary, #475569);
  font-family: var(--font-text, 'Noto Sans SC', sans-serif);
  transition: color 200ms ease;
}
/* 锁定态：label 同步变灰 */
.field:has(.form-control:disabled) .field-label {
  color: var(--color-text-muted, #94A3B8);
}

/* ===== Form control (input + select shared) ===== */
.form-control {
  width: 100%;
  height: 44px;
  padding: 0 12px;
  border: 1px solid var(--color-border, #E2E8F0);
  border-radius: 8px;
  font-size: 14px;
  font-family: var(--font-text, 'Noto Sans SC', sans-serif);
  color: var(--color-text-primary, #0F172A);
  background: var(--color-bg-card, #FFFFFF);
  outline: none;
  transition: border-color 150ms ease, box-shadow 150ms ease, background 150ms ease;
  -webkit-appearance: none;
  appearance: none;
}
.form-control::placeholder {
  color: var(--color-text-muted, #94A3B8);
}
.form-control:hover:not(:disabled) {
  border-color: #CBD5E1;
}
.form-control:focus {
  border-color: var(--color-brand-500, #0EA5E9);
  box-shadow: 0 0 0 3px rgba(14, 165, 233, 0.15);
}
.form-control:disabled {
  background: #F8FAFC;
  color: var(--color-text-muted, #94A3B8);
  cursor: not-allowed;
  border-style: dashed;
  border-color: #E2E8F0;
}

/* ===== Select wrapper with chevron icon ===== */
.select-wrap {
  position: relative;
}
.select-wrap .form-control {
  padding-right: 40px;
  cursor: pointer;
  text-overflow: ellipsis;
}
.select-icon {
  position: absolute;
  right: 12px;
  top: 50%;
  transform: translateY(-50%);
  width: 16px;
  height: 16px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--color-text-muted, #94A3B8);
  pointer-events: none;
  transition: color 150ms ease, opacity 150ms ease;
}
.select-wrap:hover:not(.is-disabled) .select-icon {
  color: var(--color-brand-500, #0EA5E9);
}
/* 锁定态：chevron 变灰且半透明 */
.select-wrap:has(.form-control:disabled) .select-icon {
  color: var(--color-text-muted, #94A3B8);
  opacity: 0.45;
}

/* ===== Toggle switch (custom CSS, matches design) ===== */
.toggle-row {
  display: flex;
  align-items: center;
  gap: 12px;
  height: 44px;
}
.toggle-switch {
  position: relative;
  width: 44px;
  height: 24px;
  border-radius: 9999px;
  border: none;
  background: var(--color-border, #E2E8F0);
  cursor: pointer;
  transition: background 200ms ease, box-shadow 200ms ease;
  flex-shrink: 0;
  padding: 0;
  outline: none;
}
.toggle-switch:hover {
  box-shadow: 0 0 0 4px rgba(14, 165, 233, 0.08);
}
.toggle-switch.is-on {
  background: var(--gradient-brand, linear-gradient(135deg, #0EA5E9 0%, #14B8A6 100%));
}
.toggle-switch.is-on:hover {
  box-shadow: 0 0 0 4px rgba(20, 184, 166, 0.18);
}
.toggle-switch:focus-visible {
  box-shadow: 0 0 0 3px rgba(14, 165, 233, 0.35);
}
.toggle-thumb {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 20px;
  height: 20px;
  background: #FFFFFF;
  border-radius: 50%;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
  transition: transform 200ms cubic-bezier(0.4, 0, 0.2, 1);
  pointer-events: none;
}
.toggle-switch.is-on .toggle-thumb {
  transform: translateX(20px);
}
.toggle-state {
  font-size: 14px;
  font-family: var(--font-text, 'Noto Sans SC', sans-serif);
  color: var(--color-text-muted, #94A3B8);
  font-weight: 500;
  transition: color 200ms ease;
}
.toggle-state.is-on {
  color: var(--color-text-primary, #0F172A);
  font-weight: 600;
}

/* ===== Section block (车况评级) ===== */
.form-section-block {
  margin-bottom: 24px;
}
.rating-pills {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}
.rating-pill {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  padding: 12px 8px;
  border: 1.5px solid var(--color-border, #E2E8F0);
  border-radius: 12px;
  background: var(--color-bg-card, #FFFFFF);
  cursor: pointer;
  text-align: center;
  transition: all 200ms ease;
  font-family: var(--font-text, 'Noto Sans SC', sans-serif);
  outline: none;
}
.rating-pill:hover:not(.is-active) {
  border-color: var(--color-brand-500, #0EA5E9);
  transform: translateY(-1px);
}
.rating-pill:focus-visible {
  box-shadow: 0 0 0 3px rgba(14, 165, 233, 0.25);
}
.rating-pill.is-active {
  background: var(--gradient-brand, linear-gradient(135deg, #0EA5E9 0%, #14B8A6 100%));
  border-color: transparent;
  transform: scale(1.02);
  box-shadow: 0 4px 12px rgba(14, 165, 233, 0.3);
}
.rating-pill-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-primary, #0F172A);
  white-space: nowrap;
}
.rating-pill.is-active .rating-pill-name {
  color: #FFFFFF;
}
.rating-pill-desc {
  font-size: 11px;
  color: var(--color-text-muted, #94A3B8);
  font-weight: 400;
  white-space: normal;
  word-break: keep-all;
  line-height: 1.4;
}
.rating-pill.is-active .rating-pill-desc {
  color: rgba(255, 255, 255, 0.85);
}

/* ===== Divider ===== */
.form-divider {
  height: 1px;
  background: var(--color-border, #E2E8F0);
  margin: 24px 0;
}

/* ===== Action bar ===== */
.action-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}
.action-bar-text {
  flex: 1 1 200px;
  min-width: 0;
}
.action-bar-hint {
  font-size: 14px;
  font-weight: 500;
  color: var(--color-text-secondary, #475569);
  margin: 0 0 2px;
  font-family: var(--font-text, 'Noto Sans SC', sans-serif);
  line-height: 1.5;
}
.action-bar-sub {
  font-size: 12px;
  color: var(--color-text-muted, #94A3B8);
  margin: 0;
  font-family: var(--font-text, 'Noto Sans SC', sans-serif);
  line-height: 1.5;
}
.action-bar-buttons {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}

/* ===== Buttons ===== */
.btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  height: 48px;
  border: none;
  border-radius: 8px;
  font-size: 14px;
  font-family: var(--font-text, 'Noto Sans SC', sans-serif);
  cursor: pointer;
  transition: all 200ms ease;
  outline: none;
  white-space: nowrap;
}
.btn:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}
.btn-outline {
  padding: 0 24px;
  font-weight: 500;
  background: var(--color-bg-card, #FFFFFF);
  color: var(--color-text-secondary, #475569);
  border: 1.5px solid var(--color-border, #E2E8F0);
}
.btn-outline:hover:not(:disabled) {
  border-color: var(--color-brand-500, #0EA5E9);
  color: var(--color-brand-500, #0EA5E9);
}
.btn-primary {
  padding: 0 32px;
  font-weight: 600;
  background: var(--gradient-brand, linear-gradient(135deg, #0EA5E9 0%, #14B8A6 100%));
  color: #FFFFFF;
  box-shadow: 0 4px 12px rgba(14, 165, 233, 0.25);
}
.btn-primary:hover:not(:disabled) {
  box-shadow: 0 6px 16px rgba(14, 165, 233, 0.35);
  filter: brightness(1.05);
}
.btn-primary:active:not(:disabled) {
  filter: brightness(0.95);
}
.btn-spinner-icon {
  animation: btn-spin 0.8s linear infinite;
}
@keyframes btn-spin {
  to {
    transform: rotate(360deg);
  }
}

/* ===== Tablet (>= 640px) ===== */
@media (min-width: 640px) {
  .rating-pills {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }
}

/* ===== Tablet (>= 1024px) ===== */
@media (min-width: 1024px) {
  .rating-pills {
    grid-template-columns: repeat(5, minmax(0, 1fr));
  }
  .form-row-3 {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

/* ===== Mobile (< 768px) ===== */
@media (max-width: 767px) {
  .form-section {
    padding: 32px 0 56px;
  }
  .brand-strip {
    margin-bottom: 24px;
    padding: 0 16px;
  }
  .form-container {
    padding: 0 16px;
  }
  .form-card {
    padding: 20px 16px;
    border-radius: 12px;
  }
  .section-title {
    font-size: 20px;
  }
  .form-row {
    margin-bottom: 12px;
  }
  .form-row-2,
  .form-row-3 {
    grid-template-columns: 1fr;
    gap: 12px;
  }
  .action-bar {
    flex-direction: column;
    align-items: stretch;
    gap: 16px;
  }
  .action-bar-text {
    flex: 1 1 auto;
    text-align: center;
  }
  .action-bar-buttons {
    width: 100%;
    display: grid;
    grid-template-columns: 1fr 1fr;
  }
  .btn {
    width: 100%;
  }
  .rating-pill-desc {
    font-size: 11px;
  }
}
</style>
