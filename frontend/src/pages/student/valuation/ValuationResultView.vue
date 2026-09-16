<script setup lang="ts">
// 评估结果页（设计稿风格：白底 + Electric Blue 残值 + 维度雷达 + 建议）
// 数据源：store 详情（提交后由 store.submitEvaluation 写入）；
// 刷新/直达恢复：路由带 id 时兜底 fetchDetail
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useEvaluationStore } from '@/stores/valuationEvaluation'
import { Edit, Download } from '@element-plus/icons-vue'
import PageHeader from '@/components/valuation/PageHeader.vue'
import ResultCard from '@/components/valuation/ResultCard.vue'
import DimensionRadar from '@/components/valuation/DimensionRadar.vue'
import FutureValueChart from '@/components/valuation/FutureValueChart.vue'
import ResultSuggestions from '@/components/valuation/ResultSuggestions.vue'
import { downloadEvaluationReportBlob } from '@/api/valuation/evaluation'
import { downloadReport } from '@/composables/useReportDownload'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiButton from '@/components/ui/UiButton.vue'

const router = useRouter()
const route = useRoute()
const store = useEvaluationStore()

// 路由 id：刷新/直达时用于兜底恢复
const routeId = computed(() => Number(route.query.id) || 0)

// 无结果且无路由 id → 无恢复来源，跳回首页
if (!store.currentResult && !routeId.value) {
  router.replace({ name: 'ValuationHome' })
}
// 有路由 id 但 store 无结果（刷新/直达）→ 兜底拉详情（模板 v-if 兜底渲染）
// 匿名用户刷新：详情接口需登录 → fetchDetail 失败 → 跳回表单页（评估结果不可恢复，需重新评估）
if (!store.currentResult && routeId.value) {
  void store.fetchDetail(routeId.value).then(ok => {
    if (!ok && !store.currentResult) {
      router.replace({ name: 'ValuationHome' })
    }
  })
}

const r = computed(() => store.currentResult)
const id = computed(() => store.currentId)

function goEdit() {
  router.push('/valuation/input')
}

async function downloadPdf() {
  if (!id.value) return
  const evalId: number = id.value
  await downloadReport(
    () => downloadEvaluationReportBlob(evalId),
    `evaluation_report_${evalId}.pdf`
  )
}
</script>

<template>
  <div v-if="r && id" class="app-container result-view valuation-root valuation-view">
    <PageHeader
      title="评估结果"
      :subtitle="`evaluation #${id}`"
    >
      <template #actions>
        <UiButton :icon="Edit" @click="goEdit">返回修改</UiButton>
        <UiButton variant="primary" :icon="Download" @click="downloadPdf">下载 PDF</UiButton>
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
          <DimensionRadar :scores="r.dimension_scores || []" height="320px" />
        </section>
      </el-col>
    </el-row>

    <!-- 未来估价走势 -->
    <section class="card-surface section-block">
      <h2 class="section-title">未来估价走势</h2>
      <FutureValueChart
        :estimated-value="r.estimated_value"
        :decay-anchor="r.decay_anchor || 0"
        :sale-year="r.sale_year || 0"
        height="320px"
      />
    </section>

    <!-- 评估建议 -->
    <section class="card-surface section-block">
      <h2 class="section-title">评估建议</h2>
      <ResultSuggestions :items="r.suggestions || []" />
    </section>
  </div>
  <UiEmptyState v-else description="暂无评估结果" />
</template>

<style scoped>
.result-view {
  padding: 0 0 var(--sp-16);
  background: var(--color-surface);
  min-height: calc(100vh - var(--header-h));
}
/* .top-row / .radar-block / .section-block / .section-title 与 768px 分区块规则
   已收敛到 assets/styles/valuation-view-sections.css（layer(base)，本组件 scoped 可覆盖）。 */
@media (max-width: 768px) {
  .result-view {
    padding-bottom: var(--sp-10);
  }
}
</style>
