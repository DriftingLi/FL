<script setup lang="ts">
// 评估报告页（Tesla 极简：白底 + Electric Blue 残值 + 维度雷达 + 系数计算过程）
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Download, CircleCheck } from '@element-plus/icons-vue'
import PageHeader from '@/components/valuation/PageHeader.vue'
import ResultCard from '@/components/valuation/ResultCard.vue'
import DimensionRadar from '@/components/valuation/DimensionRadar.vue'
import { getEvaluationDetail } from '@/api/valuation/evaluation'
import { generateReport, getReportDownloadUrl } from '@/api/valuation/report'
import { downloadEvaluationReportBlob } from '@/api/valuation/evaluation'
import {
  formatBoolean,
  formatBytes,
  formatDateTime,
  formatInt,
  formatMastHeight,
  formatTonnage,
  formatWan,
  formatCoefficient
} from '@/utils/valuationFormat'
import { COEFFICIENT_DEFS, CONDITION_RATING_COLOR } from '@/utils/valuationConstants'
import type { EvaluationDetailResponse } from '@/types/valuation/evaluation'

const route = useRoute()
const router = useRouter()

const id = computed(() => {
  const v = route.params.id
  if (typeof v === 'string') return parseInt(v, 10)
  if (Array.isArray(v) && v[0]) return parseInt(v[0], 10)
  return 0
})

const data = ref<EvaluationDetailResponse | null>(null)
const loading = ref(false)
const generating = ref(false)
const pdfInfo = ref<{ file_name: string; file_size: number } | null>(null)

onMounted(async () => {
  await loadDetail()
})

async function loadDetail() {
  if (!id.value) return
  loading.value = true
  try {
    data.value = await getEvaluationDetail(id.value)
    // 若已有 pdf 路径，恢复 pdf 信息
    const pdfPath = data.value?.report_pdf_path
    if (pdfPath) {
      const filename = pdfPath.split(/[\\/]/).pop() ?? ''
      pdfInfo.value = { file_name: filename, file_size: 0 }
    }
  } finally {
    loading.value = false
  }
}

async function onGenerate() {
  if (!id.value || generating.value) return
  generating.value = true
  try {
    const r = await generateReport(id.value)
    pdfInfo.value = { file_name: r.file_name, file_size: r.file_size }
  } finally {
    generating.value = false
  }
}

async function onDownload() {
  if (!id.value) return
  const fileName = pdfInfo.value?.file_name || `evaluation_report_${id.value}.pdf`
  try {
    // 优先用 downloadEvaluationReportBlob（统一接口）；fallback 用 getReportDownloadUrl + window open
    const blob = await downloadEvaluationReportBlob(id.value)
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = fileName
    a.style.display = 'none'
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    setTimeout(() => URL.revokeObjectURL(url), 1500)
  } catch {
    // 兜底：浏览器直接打开下载 URL
    window.open(getReportDownloadUrl(id.value), '_blank')
  }
}

function backToResult() {
  router.push('/valuation/result')
}

// 维度评分转 Map（兼容 DimensionRadar 旧 props 签名）
const dimensionScoresMap = computed(() => {
  const arr = data.value?.dimension_scores ?? []
  const map: Record<string, number> = {}
  for (const d of arr) map[d.label] = d.value
  return map
})

// 系数取值（安全访问；residual_rate 为本地计算的派生值）
function coefValue(key: string): number {
  if (key === 'residual_rate') {
    const d = data.value
    if (!d || !d.original_price) return 0
    return Math.min(1.0, d.estimated_value / d.original_price)
  }
  const v = (data.value as unknown as Record<string, number> | null)?.[key]
  return typeof v === 'number' ? v : 0
}

// 使用年限 = 评估年份 - 出厂年份
const usageYears = computed(() => {
  const d = data.value
  if (!d || !d.factory_year || !d.sale_year) return 0
  return d.sale_year - d.factory_year
})
</script>

<template>
  <div v-loading="loading">
    <div v-if="data" class="app-container report-view valuation-root">
      <PageHeader
        :title="`评估报告 #${id}`"
        :subtitle="`生成于 ${formatDateTime(new Date().toISOString())}`"
      >
        <template #actions>
          <el-button :icon="ArrowLeft" @click="backToResult">返回结果</el-button>
          <el-button :icon="Download" @click="onDownload" v-if="pdfInfo">下载 PDF</el-button>
          <el-button type="primary" :icon="CircleCheck" :loading="generating" @click="onGenerate">
            {{ pdfInfo ? '重新生成 PDF' : '生成 PDF' }}
          </el-button>
        </template>
      </PageHeader>

      <!-- 顶部双列：左侧残值卡片，右侧雷达图 -->
      <el-row :gutter="20" class="top-row">
        <el-col :xs="24" :lg="14">
          <ResultCard
            :estimated-value="data.estimated_value"
            :confidence-low="data.confidence_low"
            :confidence-high="data.confidence_high"
            :original-price="data.original_price"
          />
        </el-col>
        <el-col :xs="24" :lg="10">
          <section class="card-surface radar-block">
            <h2 class="section-title">维度评分</h2>
            <DimensionRadar :scores="dimensionScoresMap" height="320px" />
          </section>
        </el-col>
      </el-row>

      <!-- 基本信息 -->
      <section class="card-surface section-block">
        <h2 class="section-title">基本信息</h2>
        <el-descriptions :column="{ xs: 1, sm: 2, md: 3 }" border size="small">
          <el-descriptions-item label="品牌类型">{{ data.brand_type }}</el-descriptions-item>
          <el-descriptions-item label="品牌">{{ data.brand }}</el-descriptions-item>
          <el-descriptions-item label="车辆类型">{{ data.vehicle_type }}</el-descriptions-item>
          <el-descriptions-item label="系列">{{ data.series }}</el-descriptions-item>
          <el-descriptions-item label="吨位">{{ formatTonnage(data.tonnage) }}</el-descriptions-item>
          <el-descriptions-item label="配置类型">{{ data.config_type }}</el-descriptions-item>
          <el-descriptions-item label="门架类型">{{ data.mast_type }}</el-descriptions-item>
          <el-descriptions-item label="门架高度">{{ formatMastHeight(data.mast_height_mm) }}</el-descriptions-item>
          <el-descriptions-item label="出厂年份">{{ data.factory_year }}</el-descriptions-item>
          <el-descriptions-item label="评估年份">{{ data.sale_year }}</el-descriptions-item>
          <el-descriptions-item label="使用年限">{{ usageYears }} 年</el-descriptions-item>
          <el-descriptions-item label="累计工时">{{ formatInt(data.usage_hours) }} 小时</el-descriptions-item>
          <el-descriptions-item label="是否原厂原漆">{{ formatBoolean(data.original_paint) }}</el-descriptions-item>
          <el-descriptions-item label="所在区域">{{ data.province }} / {{ data.city }}</el-descriptions-item>
          <el-descriptions-item label="有车牌">{{ formatBoolean(data.has_license_plate) }}</el-descriptions-item>
          <el-descriptions-item label="特种设备登记证">{{ formatBoolean(data.has_registration_certificate) }}</el-descriptions-item>
          <el-descriptions-item label="有保养记录">{{ formatBoolean(data.has_maintenance_records) }}</el-descriptions-item>
          <el-descriptions-item label="车况评级">
            <el-tag
              effect="plain"
              :style="{
                color: CONDITION_RATING_COLOR[data.condition_rating] || '#666',
                borderColor: CONDITION_RATING_COLOR[data.condition_rating] || '#666'
              }"
            >
              {{ data.condition_rating }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="原始价格">{{ formatWan(data.original_price) }}</el-descriptions-item>
        </el-descriptions>
      </section>

      <!-- 系数计算过程 -->
      <section class="card-surface section-block">
        <h2 class="section-title">系数计算过程</h2>
        <p class="calc-formula num">
          残值 = 原价 × Kt_adj × Kc × Km
        </p>
        <p class="calc-formula-sub num">
          Kt_adj = Kt^(Kh / Kb)（品牌系数与使用强度系数修正时间衰减）
        </p>
        <p class="calc-result num">
          = {{ formatWan(data.original_price) }}
          × {{ formatCoefficient(coefValue('k_time_adjusted')) }}
          × {{ formatCoefficient(coefValue('k_condition')) }}
          × {{ formatCoefficient(coefValue('k_market')) }}
          = <span class="calc-final">{{ formatWan(data.estimated_value) }}</span>
        </p>
        <div class="coef-detail-grid">
          <div v-for="def in COEFFICIENT_DEFS" :key="def.key" class="coef-detail-cell">
            <div class="coef-detail-label" :style="{ color: def.color }">{{ def.label }}</div>
            <div class="coef-detail-value num">{{ formatCoefficient(coefValue(def.key)) }}</div>
            <div class="coef-detail-desc">{{ def.description }}</div>
          </div>
        </div>
      </section>

      <!-- 评估建议 -->
      <section class="card-surface section-block" v-if="data.suggestions && data.suggestions.length">
        <h2 class="section-title">评估建议</h2>
        <ul class="suggestion-list">
          <li v-for="(s, idx) in data.suggestions" :key="idx">
            <span class="suggestion-num">{{ String(idx + 1).padStart(2, '0') }}</span>
            <span class="suggestion-text">{{ s }}</span>
          </li>
        </ul>
      </section>

      <!-- 免责声明 -->
      <section class="card-surface section-block disclaimer">
        <h2 class="section-title">免责声明</h2>
        <ol>
          <li>本报告基于系统算法模型与用户提交数据计算得出，仅作为残值评估的参考依据。</li>
          <li>实际成交价格受市场行情、交易双方议价能力、设备具体状况等因素影响，可能与本报告存在偏差。</li>
          <li>使用本报告进行商业决策所产生的任何后果，由使用方自行承担。</li>
        </ol>
      </section>

      <!-- PDF 已生成提示 -->
      <div v-if="pdfInfo" class="pdf-hint">
        <span class="pdf-hint-text">
          已生成报告：{{ pdfInfo.file_name }}（{{ formatBytes(pdfInfo.file_size) }}）
        </span>
        <el-button type="primary" link :icon="Download" @click="onDownload">下载</el-button>
      </div>
    </div>
    <el-empty v-else description="未找到该评估记录" />
  </div>
</template>

<style scoped>
.report-view {
  padding: 0;
}
.top-row {
  margin-top: 0;
}
.radar-block,
.section-block {
  margin-top: var(--sp-5);
}
.section-title {
  font-size: var(--fs-lg);
  font-weight: var(--fw-medium);
  margin: 0 0 var(--sp-5);
  color: var(--color-text);
}
.calc-formula {
  font-size: var(--fs-base);
  color: var(--color-text-secondary);
  margin: 0 0 var(--sp-3);
}
.calc-formula-sub {
  font-size: var(--fs-sm);
  color: var(--color-text-tertiary);
  margin: 0 0 var(--sp-4);
}
.calc-result {
  font-size: var(--fs-md);
  color: var(--color-text);
  margin: 0 0 var(--sp-5);
  line-height: 1.8;
  word-break: break-all;
}
.calc-final {
  color: var(--color-primary);
  font-weight: var(--fw-semibold);
}
.coef-detail-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: var(--sp-4);
}
.coef-detail-cell {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: var(--sp-4);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-bg-muted);
}
.coef-detail-label {
  font-size: var(--fs-sm);
  font-weight: var(--fw-medium);
}
.coef-detail-value {
  font-size: 20px;
  font-weight: var(--fw-semibold);
  color: var(--color-text);
}
.coef-detail-desc {
  font-size: var(--fs-xs);
  color: var(--color-text-tertiary);
  line-height: 1.5;
}
.suggestion-list {
  margin: 0;
  padding: 0;
  list-style: none;
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--sp-3) var(--sp-6);
}
.suggestion-list li {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  font-size: var(--fs-base);
  line-height: 1.6;
  color: var(--color-text);
}
.suggestion-num {
  font-family: var(--font-mono);
  font-size: var(--fs-sm);
  font-weight: var(--fw-medium);
  color: var(--color-primary);
  background: rgba(62, 106, 225, 0.08);
  padding: 2px 8px;
  border-radius: var(--radius-sm);
  flex-shrink: 0;
  margin-top: 2px;
}
.suggestion-text {
  flex: 1;
}
.disclaimer ol {
  margin: 0;
  padding-left: 20px;
  color: var(--color-text-secondary);
  font-size: var(--fs-sm);
  line-height: 1.8;
}
.pdf-hint {
  margin-top: var(--sp-5);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--sp-3) var(--sp-5);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  background: var(--color-bg-muted);
}
.pdf-hint-text {
  font-size: var(--fs-sm);
  color: var(--color-text-tertiary);
  font-family: var(--font-mono);
}
@media (max-width: 768px) {
  .suggestion-list {
    grid-template-columns: 1fr;
  }
  .coef-detail-grid {
    grid-template-columns: 1fr 1fr;
  }
  .radar-block,
  .section-block {
    margin-top: var(--sp-4);
    padding: var(--sp-5) var(--sp-4);
  }
  .radar-block :deep(.echarts) {
    height: 260px !important;
  }
  .pdf-hint {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--sp-2);
  }
}
</style>
