<script setup lang="ts">
// 评估结果页（Tesla 极简：白底 + Electric Blue 残值 + 6 维雷达 + 建议）
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useEvaluationStore } from '@/stores/valuationEvaluation'
import { Edit, Document, Download } from '@element-plus/icons-vue'
import PageHeader from '@/components/valuation/PageHeader.vue'
import ResultCard from '@/components/valuation/ResultCard.vue'
import DimensionRadar from '@/components/valuation/DimensionRadar.vue'
import { getReportDownloadUrl } from '@/api/valuation/report'
import client from '@/api/valuation/client'

const router = useRouter()
const store = useEvaluationStore()

// 守卫：没有结果时跳回首页
if (!store.currentResult) {
  router.replace('/valuation')
}

const r = computed(() => store.currentResult)
const id = computed(() => store.currentId)
const type = computed(() => store.currentType)

function goEdit() {
  if (type.value === 'electric') router.push('/valuation/input/electric')
  else router.push('/valuation/input/combustion')
}

function goReport() {
  if (id.value) router.push(`/valuation/report/${id.value}`)
}

function downloadPdf() {
  if (!id.value) return
  // axios blob + a.download：避免 window.open 触发「开新 tab + 弹下载」的双窗口
  const fileName = `evaluation_report_${id.value}.pdf`
  client
    .get(getReportDownloadUrl(id.value), { responseType: 'blob' })
    .then((resp) => {
      const blob = resp.data as Blob
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = fileName
      a.style.display = 'none'
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      setTimeout(() => URL.revokeObjectURL(url), 1500)
    })
    .catch(() => {
      // 拦截器已 ElMessage.error
    })
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
          <DimensionRadar :scores="r.dimension_scores || {}" height="320px" />
        </section>
      </el-col>
    </el-row>

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
