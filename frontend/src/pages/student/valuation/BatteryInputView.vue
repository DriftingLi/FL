// 电池 RUL 评估录入页（Tesla 极简风）
<script setup lang="ts">
// 极简风格：白底 + 4px 圆角 + Electric Blue 唯一彩色
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Refresh, Promotion, Upload } from '@element-plus/icons-vue'
import PageHeader from '@/components/valuation/PageHeader.vue'
import { useBatteryStore } from '@/stores/valuationBattery'
import {
  BATTERY_TYPE_LABELS,
  type BatteryType,
  type CreateBatteryRequest,
  type CycleData
} from '@/types/valuation/battery'

const router = useRouter()
const store = useBatteryStore()

// 表单状态
const batteryType = ref<BatteryType>('lfp')
const batteryModel = ref<string>('')
const cyclesJson = ref<string>('')
// 解析后的循环数（用于实时反馈）
const parsedCycles = ref<CycleData[]>([])
const jsonError = ref<string | null>(null)
const submitting = computed(() => store.loading)

/** 解析 JSON 文本 → parsedCycles */
function parseJson() {
  jsonError.value = null
  parsedCycles.value = []
  if (!cyclesJson.value.trim()) {
    return
  }
  try {
    const obj = JSON.parse(cyclesJson.value)
    let arr: unknown
    if (Array.isArray(obj)) {
      arr = obj
    } else if (obj && typeof obj === 'object' && 'cycles' in obj && Array.isArray((obj as { cycles: unknown }).cycles)) {
      arr = (obj as { cycles: unknown[] }).cycles
    } else {
      jsonError.value = 'JSON 必须为数组或包含 cycles 数组的对象'
      return
    }
    // 字段校验
    const validated: CycleData[] = []
    for (let i = 0; i < (arr as unknown[]).length; i++) {
      const it = (arr as unknown[])[i] as Record<string, unknown>
      if (!it || typeof it !== 'object') {
        jsonError.value = `第 ${i + 1} 条不是对象`
        return
      }
      const cycle_index = Number(it.cycle_index)
      const voltage_series = it.voltage_series as number[] | undefined
      const current_series = it.current_series as number[] | undefined
      const capacity = Number(it.capacity)
      if (!Number.isFinite(cycle_index) || cycle_index <= 0) {
        jsonError.value = `第 ${i + 1} 条 cycle_index 非法`
        return
      }
      if (!Array.isArray(voltage_series) || !Array.isArray(current_series)) {
        jsonError.value = `第 ${i + 1} 条 voltage_series/current_series 必须是数组`
        return
      }
      if (voltage_series.length !== current_series.length) {
        jsonError.value = `第 ${i + 1} 条电压/电流数组长度不一致`
        return
      }
      if (!Number.isFinite(capacity) || capacity <= 0) {
        jsonError.value = `第 ${i + 1} 条 capacity 必须 > 0`
        return
      }
      validated.push({
        cycle_index,
        voltage_series: voltage_series as number[],
        current_series: current_series as number[],
        capacity
      })
    }
    parsedCycles.value = validated
  } catch (e) {
    jsonError.value = 'JSON 解析失败：' + (e instanceof Error ? e.message : String(e))
  }
}

/** 加载示例数据（用于快速体验） */
function loadSample() {
  const samples: CycleData[] = []
  const n = 15
  for (let i = 0; i < n; i++) {
    const soh = 1.0 - 0.2 * i / (n - 1)
    const capacity = 1.1 * soh
    const voltage_series: number[] = []
    const current_series: number[] = []
    for (let p = 0; p < 100; p++) {
      if (p < 70) {
        voltage_series.push(3.2 + 0.4 * p / 70)
        current_series.push(1.0)
      } else {
        voltage_series.push(3.6)
        current_series.push(0.3 * Math.exp(-(p - 70) / 20) * (1 + 0.1 * i / n))
      }
    }
    samples.push({ cycle_index: i + 1, voltage_series, current_series, capacity })
  }
  cyclesJson.value = JSON.stringify(samples, null, 2)
  parseJson()
}

/** 解析文件 */
function handleFile(file: File) {
  const reader = new FileReader()
  reader.onload = (e) => {
    cyclesJson.value = String(e.target?.result || '')
    parseJson()
  }
  reader.readAsText(file, 'utf-8')
  return false // 阻止 el-upload 默认上传
}

/** 重置 */
function reset() {
  batteryType.value = 'lfp'
  batteryModel.value = ''
  cyclesJson.value = ''
  parsedCycles.value = []
  jsonError.value = null
}

/** 提交 */
async function submit() {
  parseJson()
  if (jsonError.value) {
    ElMessage.error(jsonError.value)
    return
  }
  if (parsedCycles.value.length < 10) {
    ElMessage.error('至少需要 10 个完整循环')
    return
  }
  const payload: CreateBatteryRequest = {
    battery_type: batteryType.value,
    battery_model: batteryModel.value || undefined,
    cycles: parsedCycles.value
  }
  try {
    const data = await store.submitCycles(payload)
    ElMessage.success(`评估完成：RUL=${data.rul_cycles} 循环，SOH=${data.soh_percent.toFixed(1)}%`)
    router.push('/valuation/battery/result')
  } catch (e) {
    // store.error 已设置，client 拦截器也提示过
    void e
  }
}

const cycleCount = computed(() => parsedCycles.value.length)
const isValid = computed(
  () => !jsonError.value && cycleCount.value >= 10 && !!batteryType.value
)
</script>

<template>
  <div class="battery-input valuation-root">
    <div class="app-container">
      <PageHeader
        title="电池健康度评估"
        subtitle="Battery RUL"
      >
        <template #actions>
          <el-button class="btn-ghost" :icon="Refresh" @click="reset">
            重置
          </el-button>
          <el-button
            class="btn-primary"
            :icon="Promotion"
            :disabled="!isValid || submitting"
            :loading="submitting"
            @click="submit"
          >
            开始评估
          </el-button>
        </template>
      </PageHeader>

      <div class="form-card">
        <!-- 电池类型 + 型号 -->
        <div class="form-row">
          <div class="form-item">
            <label class="form-label">电池类型</label>
            <el-select
              v-model="batteryType"
              size="large"
              class="form-input"
            >
              <el-option
                v-for="key in (Object.keys(BATTERY_TYPE_LABELS) as BatteryType[])"
                :key="key"
                :value="key"
                :label="BATTERY_TYPE_LABELS[key]"
              />
            </el-select>
          </div>
          <div class="form-item">
            <label class="form-label">电池型号（可选）</label>
            <el-input
              v-model="batteryModel"
              size="large"
              class="form-input"
              placeholder="如 LFP-100Ah、NCM-50Ah"
            />
          </div>
        </div>

        <!-- JSON 输入 -->
        <div class="form-item">
          <label class="form-label">循环数据（JSON）</label>
          <el-input
            v-model="cyclesJson"
            type="textarea"
            :rows="12"
            class="form-textarea"
            placeholder='点击"加载示例"快速体验，或粘贴/上传你的循环数据。数据结构：[{"cycle_index":1,"voltage_series":[...],"current_series":[...],"capacity":1.1}, ...]'
            @blur="parseJson"
            @change="parseJson"
          />
          <div class="form-toolbar">
            <el-upload
              :before-upload="(file: File) => { handleFile(file); return false; }"
              :show-file-list="false"
              accept=".json,application/json"
            >
              <el-button class="btn-ghost" :icon="Upload">
                上传 .json
              </el-button>
            </el-upload>
            <el-button class="btn-ghost" @click="loadSample">加载示例</el-button>
            <span v-if="jsonError" class="form-error">{{ jsonError }}</span>
            <span v-else-if="cycleCount > 0" class="form-hint">
              已解析 <strong>{{ cycleCount }}</strong> 个循环
            </span>
            <span v-else class="form-hint form-hint-muted">等待输入</span>
          </div>
        </div>

        <!-- 数据格式说明 -->
        <div class="form-tips">
          <p class="form-tips-title">数据格式说明</p>
          <ul>
            <li>每条循环需包含 <code>cycle_index</code>（整数 ≥ 1）、<code>voltage_series</code>（电压时序数组）、<code>current_series</code>（电流时序数组，长度一致）、<code>capacity</code>（放电容量）</li>
            <li>至少提交 <strong>10</strong> 个循环，推荐 <strong>20+</strong> 循环以获得更稳定的滑窗预测</li>
            <li>电压/电流时序建议聚焦充电 CC-CV 段（论文算法核心输入）</li>
          </ul>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.battery-input {
  min-height: calc(100vh - var(--header-h, 56px) - 40px);
  background: var(--color-bg);
}
.battery-input > .app-container {
  max-width: var(--container-max);
  margin: 0 auto;
  padding-top: var(--sp-8);
  padding-bottom: var(--sp-8);
}
.form-card {
  background: var(--color-bg);
  border: 1px solid var(--color-border);
  border-radius: 4px;
  padding: var(--sp-8) var(--sp-7);
  display: flex;
  flex-direction: column;
  gap: var(--sp-6);
}
.form-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--sp-6);
}
.form-item {
  display: flex;
  flex-direction: column;
  gap: var(--sp-2);
}
.form-label {
  font-size: var(--fs-base);
  font-weight: var(--fw-medium);
  color: var(--color-text);
}
.form-input {
  width: 100%;
}
.form-textarea {
  font-family: var(--font-mono);
  font-size: var(--fs-sm);
  line-height: 1.6;
}
.form-toolbar {
  display: flex;
  align-items: center;
  gap: var(--sp-3);
  flex-wrap: wrap;
}
.form-hint {
  font-size: var(--fs-sm);
  color: var(--color-text-secondary);
}
.form-hint-muted {
  color: var(--color-text-muted);
}
.form-hint strong {
  color: var(--color-primary);
}
.form-error {
  font-size: var(--fs-sm);
  color: #cf1322;
}
.form-tips {
  background: var(--color-bg-muted);
  border-radius: 4px;
  padding: var(--sp-4) var(--sp-5);
  border-left: 3px solid var(--color-primary);
}
.form-tips-title {
  margin: 0 0 var(--sp-2);
  font-size: var(--fs-base);
  font-weight: var(--fw-medium);
  color: var(--color-text);
}
.form-tips ul {
  margin: 0;
  padding-left: var(--sp-5);
  font-size: var(--fs-sm);
  color: var(--color-text-secondary);
  line-height: 1.8;
}
.form-tips code {
  background: var(--color-bg);
  padding: 0 4px;
  border-radius: 2px;
  font-family: var(--font-mono);
  font-size: var(--fs-xs);
  border: 1px solid var(--color-border);
}
.btn-primary {
  background: var(--color-primary);
  border-color: var(--color-primary);
  color: #fff;
}
.btn-primary:hover:not(:disabled) {
  background: var(--color-primary-hover);
  border-color: var(--color-primary-hover);
  color: #fff;
}
.btn-primary:disabled {
  background: var(--color-bg-muted);
  border-color: var(--color-border);
  color: var(--color-text-muted);
}
.btn-ghost {
  background: var(--color-bg);
  border: 1px solid var(--color-border-strong);
  color: var(--color-text);
}

/* ===== 移动端适配 ===== */
@media (max-width: 768px) {
  .battery-input > .app-container {
    padding-top: var(--sp-5);
    padding-bottom: var(--sp-5);
  }
  .form-card {
    padding: var(--sp-5) var(--sp-4);
  }
  .form-row {
    grid-template-columns: 1fr;
  }
  .form-tips {
    padding: var(--sp-3) var(--sp-4);
  }
}
</style>
