<script setup lang="ts">
// 叉车残值评估参数录入（统一表单，Tesla 极简风）
// 重构说明：合并旧 Electric/Combustion 两个录入页；所有字段选项从后端字典动态加载
//         字段可见性规则：字典为空则隐藏字段；车辆类型字典无 electric → 隐藏电池类型字段
import { computed, onMounted, ref, watch } from 'vue'
import { Refresh, Promotion } from '@element-plus/icons-vue'
import PageHeader from '@/components/valuation/PageHeader.vue'
import { useEvaluationForm } from '@/composables/useEvaluationForm'
import {
  listBrandTypes,
  listBrandsByType,
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
  BrandTypeOption,
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
const brandTypes = ref<BrandTypeOption[]>([])
const vehicleTypes = ref<VehicleTypeOption[]>([])
const brands = ref<Brand[]>([])
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

// 车辆类型字典是否含电动 → 控制 battery_type 字段可见性
const hasElectricVehicleType = computed(() =>
  vehicleTypes.value.some((v) => v.power_type === 'electric')
)

// ========== 表单 ==========
const { form, submitting, isValid, reset, submit } = useEvaluationForm({
  hasElectricVehicleType
})

// ========== 级联加载 ==========
watch(
  () => form.brand_type,
  async (bt) => {
    // 切换品牌类型：清空品牌及下游字段
    form.brand = undefined
    form.series = undefined
    seriesList.value = []
    if (!bt) {
      brands.value = []
      return
    }
    brands.value = await listBrandsByType(bt)
  }
)

watch(
  () => form.brand,
  async (b) => {
    form.series = undefined
    seriesList.value = []
    if (!b) return
    seriesList.value = await listSeries(b)
  }
)

watch(
  () => form.province,
  async (p) => {
    form.city = undefined
    cities.value = []
    if (!p) return
    cities.value = await listCities(p)
  }
)

// 车辆类型切换：若是非电动，清空已选电池类型（避免脏数据）
watch(
  () => form.vehicle_type,
  (vt) => {
    if (!vt) return
    const matched = vehicleTypes.value.find((v) => v.name === vt)
    if (matched && matched.power_type !== 'electric') {
      form.battery_type = undefined
    }
  }
)

// ========== 初始化：并行加载所有静态字典 ==========
onMounted(async () => {
  loadingDict.value = true
  try {
    const [
      btList,
      vtList,
      tnList,
      ctList,
      mtList,
      mhList,
      batList,
      crList,
      provList
    ] = await Promise.all([
      listBrandTypes(),
      listVehicleTypes(),
      listTonnages(),
      listConfigTypes(),
      listMastTypes(),
      listMastHeights(),
      listBatteryTypes(),
      listConditionRatings(),
      listProvinces()
    ])
    brandTypes.value = btList
    vehicleTypes.value = vtList
    tonnages.value = tnList
    configTypes.value = ctList
    mastTypes.value = mtList
    mastHeights.value = mhList
    batteryTypes.value = batList
    conditionRatings.value = crList
    provinces.value = provList
  } finally {
    loadingDict.value = false
  }
})

// ========== 字段可见性（字典为空则隐藏） ==========
const showBrandType = computed(() => brandTypes.value.length > 0)
const showBrand = computed(() => brands.value.length > 0)
const showVehicleType = computed(() => vehicleTypes.value.length > 0)
const showSeries = computed(() => seriesList.value.length > 0)
const showTonnage = computed(() => tonnages.value.length > 0)
const showConfigType = computed(() => configTypes.value.length > 0)
const showMastType = computed(() => mastTypes.value.length > 0)
const showMastHeight = computed(() => mastHeights.value.length > 0)
const showBatteryType = computed(
  () => batteryTypes.value.length > 0 && hasElectricVehicleType.value
)
const showConditionRating = computed(() => conditionRatings.value.length > 0)
const showProvince = computed(() => provinces.value.length > 0)
const showCity = computed(() => cities.value.length > 0)

// 车况评级按 rating 排序展示
const sortedConditionRatings = computed(() =>
  [...conditionRatings.value].sort((a, b) => a.rating.localeCompare(b.rating))
)

// 车辆类型按 power_type 分组展示更友好
const vehicleTypeGrouped = computed(() => {
  const electric = vehicleTypes.value.filter((v) => v.power_type === 'electric')
  const combustion = vehicleTypes.value.filter((v) => v.power_type === 'combustion')
  return { electric, combustion }
})

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
      <!-- 基础信息：品牌类型 → 品牌 → 系列 → 吨位 -->
      <section class="input-section card-surface">
        <h2 class="section-title">品牌与车型</h2>
        <el-row :gutter="24">
          <el-col v-if="showBrandType" :xs="24" :md="12" :lg="6">
            <el-form-item label="品牌类型" required>
              <el-select
                v-model="form.brand_type"
                placeholder="请选择品牌类型"
                filterable
                clearable
              >
                <el-option
                  v-for="bt in brandTypes"
                  :key="bt.name"
                  :value="bt.name"
                  :label="`${bt.name}（K=${bt.k_type.toFixed(2)}）`"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col v-if="showBrand" :xs="24" :md="12" :lg="6">
            <el-form-item label="品牌" required>
              <el-select
                v-model="form.brand"
                placeholder="请选择品牌"
                filterable
                clearable
                :disabled="!form.brand_type"
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
          <el-col v-if="showSeries" :xs="24" :md="12" :lg="6">
            <el-form-item label="系列" required>
              <el-select
                v-model="form.series"
                placeholder="请选择系列"
                filterable
                clearable
                :disabled="!form.brand"
              >
                <el-option
                  v-for="s in seriesList"
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

        <el-row :gutter="24">
          <el-col v-if="showVehicleType" :xs="24" :md="12" :lg="6">
            <el-form-item label="车辆类型" required>
              <el-select
                v-model="form.vehicle_type"
                placeholder="请选择车辆类型"
                filterable
                clearable
              >
                <el-option-group
                  v-if="vehicleTypeGrouped.electric.length"
                  label="电动"
                >
                  <el-option
                    v-for="vt in vehicleTypeGrouped.electric"
                    :key="vt.id"
                    :value="vt.name"
                    :label="vt.name"
                  />
                </el-option-group>
                <el-option-group
                  v-if="vehicleTypeGrouped.combustion.length"
                  label="内燃"
                >
                  <el-option
                    v-for="vt in vehicleTypeGrouped.combustion"
                    :key="vt.id"
                    :value="vt.name"
                    :label="vt.name"
                  />
                </el-option-group>
              </el-select>
            </el-form-item>
          </el-col>
          <el-col v-if="showConfigType" :xs="24" :md="12" :lg="6">
            <el-form-item label="配置类型" required>
              <el-select
                v-model="form.config_type"
                placeholder="请选择配置类型"
                filterable
                clearable
              >
                <el-option
                  v-for="c in configTypes"
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
              >
                <el-option
                  v-for="m in mastTypes"
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
              >
                <el-option
                  v-for="mh in mastHeights"
                  :key="mh.id"
                  :value="mh.value_mm"
                  :label="`${mh.value_mm} mm`"
                />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>

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
