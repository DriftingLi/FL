<script setup lang="ts">
// 评估结果页（Tesla 极简：白底 + Electric Blue 残值 + 维度雷达 + 系数卡 + 建议）
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useEvaluationStore } from '@/stores/valuationEvaluation'
import { Edit, Document, Download } from '@element-plus/icons-vue'
import PageHeader from '@/components/valuation/PageHeader.vue'
import ResultCard from '@/components/valuation/ResultCard.vue'
import DimensionRadar from '@/components/valuation/DimensionRadar.vue'
import { downloadEvaluationReportBlob } from '@/api/valuation/evaluation'
import { COEFFICIENT_DEFS } from '@/utils/valuationConstants'
import { formatCoefficient } from '@/utils/valuationFormat'

const router = useRouter()
const store = useEvaluationStore()

// 守卫：没有结果时跳回首页
if (!store.currentResult) {
  router.replace('/valuation')
}

const r = computed(() => store.currentResult)
const id = computed(() => store.currentId)

function goEdit() {
  router.push('/valuation/input')
}

function goReport() {
  if (id.value) router.push(`/valuation/report/${id.value}`)
}

async function downloadPdf() {
  if (!id.value) return
  const fileName = `evaluation_report_${id.value}.pdf`
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
    // 延迟释放，确保浏览器已开始下载
    setTimeout(() => URL.revokeObjectURL(url), 1500)
  } catch {
    // 拦截器已 ElMessage.error
  }
}

// 维度评分 → Record<string, number>（兼容 DimensionRadar 旧 props 签名）
const dimensionScoresMap = computed(() => {
  const arr = r.value?.dimension_scores ?? []
  const map: Record<string, number> = {}
  for (const d of arr) map[d.label] = d.value
  return map
})

// 系数取值（安全访问）
function coefValue(key: string): number {
  const v = (r.value as unknown as Record<string, number> | null)?.[key]
  return typeof v === 'number' ? v : 0
}
</script>

<template>
  <div v-if="r && id" class="app-container result-view valuation-root">
    <PageHeader
      title="评估结果"
      :subtitle="`evaluation #${id}`"
    >
      <template #actions>
        <el-button :icon="Edit" @click="goEdit">返回修改</el-button>
        <el-button :icon="Document" @click="goReport">查看报告</el-button>
        <el-button type="primary" :icon="Download" @click="downloadPdf">下载 PDF</el-button>
      </template>
    </PageHeader>

    <!-- 顶部双列：残值卡片（主，14 列）+ 雷达图（次，10 列） -->
    <el-row :gutter="20" class="top-row">
      <el-col :xs="24" :lg="14">
        <ResultCard
          :estimated-value="r.estimated_value"
          :confidence-low="r.confidence_low"
          :confidence-high="r.confidence_high"
          :original-price="r.original_price || 0"
        />
      </el-col>
      <el-col :xs="24" :lg="10">
        <section class="card-surface radar-block">
          <h2 class="section-title">维度评分</h2>
          <DimensionRadar :scores="dimensionScoresMap" height="320px" />
        </section>
      </el-col>
    </el-row>

    <!-- 系数列表 -->
    <section class="card-surface section-block">
      <h2 class="section-title">
        <span class="title-icon">∑</span>
        系数列表
      </h2>
      <div class="coef-grid">
        <div v-for="def in COEFFICIENT_DEFS" :key="def.key" class="coef-cell">
          <div class="coef-label" :style="{ color: def.color }">{{ def.label }}</div>
          <div class="coef-value num">{{ formatCoefficient(coefValue(def.key)) }}</div>
          <div class="coef-desc">{{ def.description }}</div>
        </div>
      </div>
    </section>

    <!-- 评估建议 -->
    <section class="card-surface section-block">
      <h2 class="section-title">
        <span class="title-icon">💡</span>
        评估建议
      </h2>
      <ul v-if="r.suggestions && r.suggestions.length" class="suggestion-list">
        <li v-for="(s, idx) in r.suggestions" :key="idx">
          <span class="suggestion-num">{{ String(idx + 1).padStart(2, '0') }}</span>
          <span class="suggestion-text">{{ s }}</span>
        </li>
      </ul>
      <el-empty v-else description="暂无建议" />
    </section>
  </div>
  <el-empty v-else description="暂无评估结果" />
</template>

<style scoped>
.result-view {
  padding: 0;
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
  display: flex;
  align-items: center;
  gap: 8px;
}
.title-icon {
  color: var(--color-primary);
  font-size: 18px;
}
.coef-grid {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: var(--sp-4);
}
.coef-cell {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: var(--sp-4);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-bg-muted);
}
.coef-label {
  font-size: var(--fs-sm);
  font-weight: var(--fw-medium);
}
.coef-value {
  font-size: 22px;
  font-weight: var(--fw-semibold);
  color: var(--color-text);
}
.coef-desc {
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
@media (max-width: 768px) {
  .suggestion-list {
    grid-template-columns: 1fr;
  }
  .coef-grid {
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
}
</style>
