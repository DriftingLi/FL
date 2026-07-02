<script setup lang="ts">
// 评估报告页（设计稿风格：白底 + Electric Blue 残值 + 维度雷达 + 基本信息网格 + 免责声明）
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Download, CircleCheck } from '@element-plus/icons-vue'
import PageHeader from '@/components/valuation/PageHeader.vue'
import ResultCard from '@/components/valuation/ResultCard.vue'
import DimensionRadar from '@/components/valuation/DimensionRadar.vue'
import FutureValueChart from '@/components/valuation/FutureValueChart.vue'
import { getEvaluationDetail } from '@/api/valuation/evaluation'
import { generateReport, getReportDownloadUrl } from '@/api/valuation/report'
import { downloadEvaluationReportBlob } from '@/api/valuation/evaluation'
import {
  formatBoolean,
  formatBytes,
  formatDateTime,
  formatInt,
  formatMastHeight,
  formatTonnage
} from '@/utils/valuationFormat'
import { CONDITION_RATING_COLOR } from '@/utils/valuationConstants'
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
    window.open(getReportDownloadUrl(id.value), '_blank')
  }
}

function backToResult() {
  router.push('/valuation/result')
}

// 维度评分转 Map
const dimensionScoresMap = computed(() => {
  const arr = data.value?.dimension_scores ?? []
  const map: Record<string, number> = {}
  for (const d of arr) map[d.label] = d.value
  return map
})

// 使用年限 = 评估年份 - 出厂年份
const usageYears = computed(() => {
  const d = data.value
  if (!d || !d.factory_year || !d.sale_year) return 0
  return d.sale_year - d.factory_year
})

// 基本信息条目（按设计稿 3 列网格顺序排列）
const basicInfoItems = computed(() => {
  const d = data.value
  if (!d) return []
  return [
    { label: '品牌类型', value: d.brand_type },
    { label: '品牌', value: d.brand },
    { label: '车辆类型', value: d.vehicle_type },
    { label: '系列', value: d.series },
    { label: '吨位', value: formatTonnage(d.tonnage) },
    { label: '配置类型', value: d.config_type },
    { label: '门架类型', value: d.mast_type },
    { label: '门架高度', value: formatMastHeight(d.mast_height_mm) },
    { label: '出厂年份', value: String(d.factory_year) },
    { label: '评估年份', value: String(d.sale_year) },
    { label: '使用年限', value: `${usageYears.value} 年` },
    { label: '累计工时', value: `${formatInt(d.usage_hours)} 小时` },
    { label: '是否原厂原漆', value: formatBoolean(d.original_paint) },
    { label: '所在区域', value: `${d.province} / ${d.city}` },
    { label: '有车牌', value: formatBoolean(d.has_license_plate) },
    { label: '特种设备登记证', value: formatBoolean(d.has_registration_certificate) },
    { label: '有保养记录', value: formatBoolean(d.has_maintenance_records) },
    { label: '车况评级', value: d.condition_rating, isRating: true }
  ]
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

      <!-- 未来估价走势 -->
      <section class="card-surface section-block">
        <h2 class="section-title">未来估价走势</h2>
        <FutureValueChart
          :estimated-value="data.estimated_value"
          :age="usageYears"
          :k-time="data.k_time"
          :k-hours="data.k_hours"
          :k-brand="data.k_brand"
          :sale-year="data.sale_year || 0"
          height="320px"
        />
      </section>

      <!-- 基本信息（设计稿网格布局） -->
      <section class="card-surface section-block">
        <h2 class="section-title">基本信息</h2>
        <div class="info-grid">
          <div
            v-for="(item, idx) in basicInfoItems"
            :key="item.label"
            class="info-item"
            :class="{ 'info-item-alt': Math.floor(idx / 3) % 2 === 0 }"
          >
            <p class="info-label">{{ item.label }}</p>
            <p v-if="!item.isRating" class="info-value">{{ item.value }}</p>
            <span
              v-else
              class="info-rating"
              :style="{
                color: CONDITION_RATING_COLOR[item.value] || '#666',
                borderColor: CONDITION_RATING_COLOR[item.value] || '#666'
              }"
            >
              {{ item.value }}
            </span>
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
          <li>本评估报告中的残值估算结果仅供参考，不构成任何形式的交易建议或定价承诺。实际交易价格可能因市场行情、设备实际状况、交易双方议价能力等因素而与本评估结果存在差异。</li>
          <li>评估模型所采用的数据来源包括公开市场交易数据、行业统计信息及历史交易记录，本平台不对数据的完整性和准确性做出保证。评估结果可能随市场变化而发生波动。</li>
          <li>用户在使用本评估报告时，应结合自身实际情况和专业顾问意见做出独立判断。因依赖本报告内容所导致的任何直接或间接损失，本平台不承担任何法律责任。</li>
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
  padding: 0 0 var(--sp-16);
  background: var(--color-surface);
  min-height: calc(100vh - var(--header-h));
}
.top-row {
  margin-top: 0;
}
.radar-block,
.section-block {
  margin-top: var(--sp-5);
  padding: var(--sp-6) var(--sp-7);
}
.section-title {
  font-size: var(--fs-lg);
  font-weight: var(--fw-medium);
  margin: 0 0 var(--sp-5);
  color: var(--color-text);
}

/* ===== 基本信息网格 ===== */
.info-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  border-top: 1px solid var(--color-border);
  border-left: 1px solid var(--color-border);
}
.info-item {
  padding: var(--sp-3);
  border-right: 1px solid var(--color-border);
  border-bottom: 1px solid var(--color-border);
  background: var(--color-surface);
}
.info-item-alt {
  background: var(--color-bg-secondary);
}
.info-label {
  font-size: var(--fs-xs);
  color: var(--color-text-tertiary);
  margin: 0 0 var(--sp-1);
}
.info-value {
  font-size: var(--fs-sm);
  font-weight: var(--fw-medium);
  color: var(--color-text);
  margin: 0;
}
.info-rating {
  display: inline-block;
  font-size: var(--fs-sm);
  font-weight: var(--fw-medium);
  padding: 2px 10px;
  border: 1.5px solid;
  border-radius: var(--radius-sm);
  background: rgba(62, 106, 225, 0.06);
}

/* ===== 评估建议 ===== */
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
  font-size: var(--fs-sm);
  line-height: 1.75;
  color: var(--color-text-secondary);
}
.suggestion-num {
  font-family: var(--font-mono);
  font-size: var(--fs-xs);
  font-weight: var(--fw-medium);
  color: var(--color-accent);
  background: rgba(62, 106, 225, 0.08);
  padding: 2px 8px;
  border-radius: var(--radius-sm);
  flex-shrink: 0;
  margin-top: 2px;
}
.suggestion-text {
  flex: 1;
}

/* ===== 免责声明 ===== */
.disclaimer {
  background: var(--color-bg-muted);
  border-color: var(--color-border-light);
}
.disclaimer ol {
  margin: 0;
  padding-left: 20px;
  color: var(--color-text-secondary);
  font-size: var(--fs-sm);
  line-height: 1.8;
}
.disclaimer li + li {
  margin-top: var(--sp-3);
}

/* ===== PDF 提示条 ===== */
.pdf-hint {
  margin-top: var(--sp-5);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--sp-3) var(--sp-5);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-surface);
}
.pdf-hint-text {
  font-size: var(--fs-sm);
  color: var(--color-text-tertiary);
  font-family: var(--font-mono);
}

@media (max-width: 768px) {
  .report-view {
    padding-bottom: var(--sp-10);
  }
  .info-grid {
    grid-template-columns: 1fr;
  }
  .suggestion-list {
    grid-template-columns: 1fr;
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
