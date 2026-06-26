<script setup lang="ts">
// 内燃叉车参数录入（Tesla 极简风：扁平、白底、4px 圆角）
import { onMounted, ref, watch, computed } from 'vue'
import { Refresh, Promotion } from '@element-plus/icons-vue'
import PageHeader from '@/components/valuation/PageHeader.vue'
import ConditionTable from '@/components/valuation/ConditionTable.vue'
import { useEvaluationForm } from '@/composables/useEvaluationForm'
import { usePartConfigs } from '@/composables/usePartConfigs'
import { listBrands } from '@/api/valuation/brands'
import { WORK_CONDITION_OPTIONS, FUEL_TYPE_OPTIONS } from '@/utils/valuationConstants'
import type { Brand } from '@/types/valuation/brand'
import type { PartConfigList } from '@/types/valuation/condition'

const { form, itemStatusMap, filledCount, totalCount, submitting, setAllNormal, reset, submit } =
  useEvaluationForm('combustion')

const { data: configs, loading: loadingConfigs, load: loadConfigs } = usePartConfigs('combustion')
const brands = ref<Brand[]>([])

const modelOptions = computed(() => {
  const b = brands.value.find((x) => x.name === form.brand)
  return b?.models ?? []
})

watch(
  () => form.brand,
  () => {
    if (!form.model || !modelOptions.value.includes(form.model)) {
      form.model = undefined
    }
  }
)

onMounted(async () => {
  await Promise.all([loadConfigs(), loadBrands()])
  if (configs.value) initItemStatusMap(configs.value)
})

async function loadBrands() {
  brands.value = await listBrands()
}

watch(configs, (list) => {
  if (list) initItemStatusMap(list)
})

function initItemStatusMap(list: PartConfigList) {
  for (const cat of list) {
    for (const it of cat.items) {
      if (!(it.item_code in itemStatusMap.value)) {
        itemStatusMap.value[it.item_code] = 'normal'
      }
    }
  }
}

const isFormValid = ref(false)
function refreshValidity() {
  if (!configs.value) {
    isFormValid.value = false
    return
  }
  const itemCodes = configs.value.flatMap((c) => c.items.map((i) => i.item_code))
  const checks = [
    !!form.brand,
    form.original_price !== undefined && form.original_price > 0,
    form.purchase_year !== undefined && form.sale_year !== undefined && form.sale_year >= form.purchase_year,
    form.usage_hours !== undefined && form.usage_hours >= 0,
    !!form.work_condition,
    !!form.fuel_type,
    itemCodes.every((c) => !!itemStatusMap.value[c])
  ]
  isFormValid.value = checks.every(Boolean)
}

watch(
  [form, itemStatusMap, configs],
  () => refreshValidity(),
  { deep: true }
)

async function onSubmit() {
  refreshValidity()
  if (!isFormValid.value) return
  await submit()
}

function onReset() {
  reset()
  if (configs.value) initItemStatusMap(configs.value)
}
</script>

<template>
  <div class="app-container input-page valuation-root">
    <PageHeader
      title="内燃叉车参数录入"
      subtitle="internal combustion · parameter input"
    >
      <template #actions>
        <el-button :icon="Refresh" @click="onReset">重置</el-button>
        <el-button type="primary" :icon="Promotion" :loading="submitting" :disabled="!isFormValid" @click="onSubmit">
          提交评估
        </el-button>
      </template>
    </PageHeader>

    <div v-loading="loadingConfigs">
      <!-- 基础信息 -->
      <section class="input-section card-surface">
        <h2 class="section-title">基础信息</h2>
        <el-row :gutter="24">
          <el-col :xs="24" :md="12" :lg="6">
            <el-form-item label="品牌" required>
              <el-select
                v-model="form.brand"
                placeholder="请选择品牌"
                filterable
              >
                <el-option
                  v-for="b in brands"
                  :key="b.name"
                  :value="b.name"
                  :label="b.name"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12" :lg="6">
            <el-form-item label="型号">
              <el-select
                v-model="form.model"
                placeholder="请先选择品牌"
                :disabled="!form.brand"
                clearable
              >
                <el-option
                  v-for="m in modelOptions"
                  :key="m"
                  :value="m"
                  :label="m"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12" :lg="6">
            <el-form-item label="燃料类型" required>
              <el-select
                v-model="form.fuel_type"
                placeholder="请选择"
              >
                <el-option
                  v-for="o in FUEL_TYPE_OPTIONS"
                  :key="o.value"
                  :value="o.value"
                  :label="o.label"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12" :lg="6">
            <el-form-item label="原始购买价格（万元）" required>
              <el-input-number
                v-model="form.original_price"
                :min="0"
                :max="9999"
                :step="0.1"
                :precision="2"
                style="width: 100%"
                placeholder="请输入"
              />
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="24">
          <el-col :xs="24" :md="12" :lg="6">
            <el-form-item label="使用工况" required>
              <el-select
                v-model="form.work_condition"
                placeholder="请选择"
              >
                <el-option
                  v-for="o in WORK_CONDITION_OPTIONS"
                  :key="o.value"
                  :value="o.value"
                  :label="o.label"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12" :lg="6">
            <el-form-item label="购置年份" required>
              <el-input-number
                v-model="form.purchase_year"
                :min="1980"
                :max="new Date().getFullYear() + 1"
                :step="1"
                style="width: 100%"
                placeholder="如 2021"
              />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12" :lg="6">
            <el-form-item label="成交年份" required>
              <el-input-number
                v-model="form.sale_year"
                :min="1980"
                :max="new Date().getFullYear() + 1"
                :step="1"
                style="width: 100%"
                placeholder="如 2024"
              />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12" :lg="6">
            <el-form-item label="累计使用小时" required>
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
        </el-row>

        <el-row :gutter="24">
          <el-col :xs="24" :md="12" :lg="6">
            <el-form-item label="能否正常行驶" required>
              <el-radio-group v-model="form.can_drive">
                <el-radio :value="true">能</el-radio>
                <el-radio :value="false">否</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12" :lg="6">
            <el-form-item label="液压功能是否正常" required>
              <el-radio-group v-model="form.hydraulic_ok">
                <el-radio :value="true">正常</el-radio>
                <el-radio :value="false">异常</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
        </el-row>
      </section>

      <!-- 部件状态 -->
      <section class="input-section">
        <div class="section-title-row">
          <h2 class="section-title">部件状态评估</h2>
          <div class="section-tools">
            <span class="fill-progress">{{ filledCount }} / {{ totalCount }}</span>
            <el-button size="small" @click="setAllNormal">全部置为正常</el-button>
          </div>
        </div>
        <ConditionTable v-model="itemStatusMap" :configs="configs" />
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
.section-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--sp-5);
  gap: var(--sp-4);
}
.section-title-row .section-title {
  margin: 0;
}
.section-tools {
  display: flex;
  align-items: center;
  gap: var(--sp-3);
}
.fill-progress {
  font-size: var(--fs-sm);
  font-weight: var(--fw-medium);
  color: var(--color-text-tertiary);
  font-family: var(--font-mono);
}

/* ===== 移动端适配 ===== */
@media (max-width: 768px) {
  .input-section {
    margin-bottom: var(--sp-4);
  }
  .section-title-row {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--sp-3);
    margin-bottom: var(--sp-4);
  }
  .section-title-row .section-title {
    margin: 0 0 var(--sp-3);
  }
}
</style>
