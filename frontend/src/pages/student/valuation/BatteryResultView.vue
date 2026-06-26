// 电池 RUL 评估结果页（Tesla 极简：白底 + RUL 大字 + 6 维雷达 + 建议 + PDF 下载）
<script setup lang="ts">
// 视觉风格：复用现有 ResultView 的 section / suggestion / card-surface 风格
import { computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Edit, Download } from '@element-plus/icons-vue'
import PageHeader from '@/components/valuation/PageHeader.vue'
import BatteryResultCard from '@/components/valuation/BatteryResultCard.vue'
import BatteryRadar from '@/components/valuation/BatteryRadar.vue'
import { useBatteryStore } from '@/stores/valuationBattery'
import { downloadBatteryReportBlob, generateBatteryReport } from '@/api/valuation/battery'
import { BATTERY_TYPE_LABELS, type BatteryType } from '@/types/valuation/battery'

const router = useRouter()
const store = useBatteryStore()

// 守卫：没有结果时跳回录入页
if (!store.currentResult) {
  router.replace('/valuation/battery')
}

const r = computed(() => store.currentResult)
const id = computed(() => store.currentId)

// 报告生成状态
const generating = computed(() => false)

// 进入页面时拉取详情（拿完整 suggestions、feature_importance）
onMounted(async () => {
  if (id.value) {
    try {
      await store.fetchDetail(id.value)
    } catch (e) {
      void e
    }
  }
})

const detail = computed(() => store.currentDetail)
const topFeatures = computed(() => (detail.value?.feature_importance || []).slice(0, 5))
const suggestions = computed(() => detail.value?.suggestions || r.value?.suggestions || [])
const batteryTypeName = computed(
  () => BATTERY_TYPE_LABELS[(r.value?.battery_type || 'lfp') as BatteryType]
)

function goEdit() {
  store.reset()
  router.push('/valuation/battery')
}

async function downloadPdf() {
  if (!id.value) return
  try {
    // 先确保后端有报告（已生成过则快速返回）
    await generateBatteryReport(id.value)
    const blob = await downloadBatteryReportBlob(id.value)
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `battery_report_${id.value}.pdf`
    a.style.display = 'none'
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    setTimeout(() => URL.revokeObjectURL(url), 1500)
    ElMessage.success('PDF 报告下载已开始')
  } catch (e) {
    void e
  }
}
</script>

<template>
  <div v-if="r && id" class="app-container battery-result-view valuation-root">
    <PageHeader
      title="电池评估结果"
      :subtitle="`battery #${id} · ${batteryTypeName}`"
    >
      <template #actions>
        <el-button :icon="Edit" @click="goEdit">重新评估</el-button>
        <el-button type="primary" :icon="Download" :loading="generating" @click="downloadPdf">
          下载 PDF
        </el-button>
      </template>
    </PageHeader>

    <!-- 顶部双列：RUL 卡片（14 列）+ 特征雷达（10 列） -->
    <el-row :gutter="20" class="top-row">
      <el-col :xs="24" :lg="14">
        <BatteryResultCard
          :rul-cycles="r.rul_cycles"
          :soh-percent="r.soh_percent"
          :confidence-low="r.confidence_low"
          :confidence-high="r.confidence_high"
          :confidence="r.confidence"
        />
      </el-col>
      <el-col :xs="24" :lg="10">
        <section class="card-surface radar-block">
          <h2 class="section-title">特征重要性（Top 维）</h2>
          <BatteryRadar :features="detail?.feature_importance || []" height="320px" />
        </section>
      </el-col>
    </el-row>

    <!-- Top-5 关键特征 -->
    <section class="card-surface section-block">
      <h2 class="section-title">
        <span class="title-icon">📄</span>
        Top-5 关键特征
      </h2>
      <div v-if="topFeatures.length" class="feature-list">
        <div v-for="(item, index) in topFeatures" :key="index" class="feature-item">
          <div class="feature-head">
            <span class="feature-rank">{{ String(index + 1).padStart(2, '0') }}</span>
            <span class="feature-name">{{ item.name }}</span>
            <el-tag class="feature-group" effect="plain">{{ item.group }}</el-tag>
          </div>
          <el-progress
            :percentage="Math.round(item.normalized * 100)"
            :color="'#3E6AE1'"
            :show-text="false"
            :stroke-width="6"
          />
        </div>
      </div>
      <el-empty v-else description="暂无特征数据" />
    </section>

    <!-- 评估建议 -->
    <section class="card-surface section-block">
      <h2 class="section-title">
        <span class="title-icon">💡</span>
        评估建议
      </h2>
      <ul v-if="suggestions.length" class="suggestion-list">
        <li v-for="(s, idx) in suggestions" :key="idx">
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
.battery-result-view {
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
.feature-list {
  display: flex;
  flex-direction: column;
  gap: var(--sp-4);
}
.feature-item {
  display: flex;
  flex-direction: column;
  gap: var(--sp-2);
}
.feature-head {
  display: flex;
  align-items: center;
  gap: var(--sp-3);
}
.feature-rank {
  font-family: var(--font-mono);
  font-size: var(--fs-sm);
  font-weight: var(--fw-medium);
  color: var(--color-primary);
  background: rgba(62, 106, 225, 0.08);
  padding: 2px 8px;
  border-radius: var(--radius-sm);
}
.feature-name {
  font-size: var(--fs-base);
  font-weight: var(--fw-medium);
  color: var(--color-text);
}
.feature-group {
  font-size: var(--fs-xs);
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
