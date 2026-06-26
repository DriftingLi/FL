<script setup lang="ts">
// 评估报告页（Tesla 极简：白底 + Electric Blue 残值 + 6 维雷达 + 极简表格）
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Download, CircleCheck } from '@element-plus/icons-vue'
import PageHeader from '@/components/valuation/PageHeader.vue'
import ResultCard from '@/components/valuation/ResultCard.vue'
import DimensionRadar from '@/components/valuation/DimensionRadar.vue'
import { getEvaluationDetail } from '@/api/valuation/evaluation'
import { generateReport, getReportDownloadUrl } from '@/api/valuation/report'
import client from '@/api/valuation/client'
import { formatBytes, formatDateTime, formatInt, formatWan } from '@/utils/valuationFormat'
import type { EvaluationDetailResponse } from '@/types/valuation/evaluation'
import type { ItemStatus } from '@/types/valuation/evaluation'

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

// el-collapse v-model：报告部件明细默认全收起
const reportActiveNames = ref<string[]>([])

// 部件配置用于显示条目权重
onMounted(async () => {
  await loadDetail()
})

async function loadDetail() {
  if (!id.value) return
  loading.value = true
  try {
    data.value = await getEvaluationDetail(id.value)
    // 如果已经有 pdf 路径
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

function onDownload() {
  if (!id.value) return
  // 改为 axios blob + a.download：避免 window.open 触发「开新 tab + 弹下载」的双窗口行为
  const fileName = pdfInfo.value?.file_name || `evaluation_report_${id.value}.pdf`
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
      // 延迟释放，确保浏览器已开始下载
      setTimeout(() => URL.revokeObjectURL(url), 1500)
    })
    .catch(() => {
      // 拦截器已 ElMessage.error 提示；此处静默即可
    })
}

function backToResult() {
  router.push('/valuation/result')
}

// 按类别分组 items（兼容后端 omitempty 可能不返回 items 的情况）
const itemsByCategory = computed(() => {
  const items = data.value?.items ?? []
  const map = new Map<string, { name: string; items: typeof items }>()
  for (const it of items) {
    const arr = map.get(it.category_code) ?? { name: it.category_name, items: [] }
    arr.items.push(it)
    map.set(it.category_code, arr)
  }
  return Array.from(map.entries()).map(([code, v]) => ({ code, ...v }))
})

const totalCount = computed(() => data.value?.items?.length ?? 0)

/** 状态对应的颜色（与 ConditionTable 保持一致） */
const STATUS_COLOR: Record<ItemStatus, string> = {
  normal: '#16A34A',
  slight_wear: '#0EA5E9',
  need_repair: '#F59E0B',
  need_replace: '#DC2626'
}
const STATUS_TEXT: Record<ItemStatus, string> = {
  normal: '正常',
  slight_wear: '轻微磨损',
  need_repair: '需维修',
  need_replace: '需更换'
}
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

      <!-- 顶部双列：左侧残值卡片（主，14 列），右侧雷达图（次，10 列） -->
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
            <DimensionRadar :scores="data.dimension_scores" height="320px" />
          </section>
        </el-col>
      </el-row>

      <!-- 基本信息 -->
      <section class="card-surface section-block">
        <h2 class="section-title">基本信息</h2>
        <el-descriptions :column="{ xs: 1, sm: 2, md: 3 }" border size="small">
          <el-descriptions-item label="叉车类型">
            {{ data.forklift_type === 'electric' ? '电动叉车' : '内燃叉车' }}
          </el-descriptions-item>
          <el-descriptions-item label="品牌">{{ data.brand }}</el-descriptions-item>
          <el-descriptions-item label="型号">{{ data.model || '-' }}</el-descriptions-item>
          <el-descriptions-item label="原始价格">{{ formatWan(data.original_price) }}</el-descriptions-item>
          <el-descriptions-item label="购置年份">{{ data.purchase_year }}</el-descriptions-item>
          <el-descriptions-item label="成交年份">{{ data.sale_year }}</el-descriptions-item>
          <el-descriptions-item label="使用年限">{{ data.sale_year - data.purchase_year }} 年</el-descriptions-item>
          <el-descriptions-item label="累计使用小时">{{ formatInt(data.usage_hours) }} 小时</el-descriptions-item>
          <el-descriptions-item label="使用工况">{{ data.work_condition }}</el-descriptions-item>
          <el-descriptions-item v-if="data.fuel_type" label="燃料类型">
            {{ data.fuel_type }}
          </el-descriptions-item>
          <el-descriptions-item label="能否正常行驶">{{ data.can_drive ? '是' : '否' }}</el-descriptions-item>
          <el-descriptions-item label="液压功能">{{ data.hydraulic_ok ? '正常' : '异常' }}</el-descriptions-item>
        </el-descriptions>
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

      <!-- 部件状态明细 -->
      <section class="card-surface section-block">
        <h2 class="section-title">部件状态明细（{{ totalCount }} 条）</h2>
        <el-collapse v-model="reportActiveNames">
          <el-collapse-item
            v-for="cat in itemsByCategory"
            :key="cat.code"
            :name="cat.code"
            :title="cat.name"
          >
            <el-table :data="cat.items" row-key="id" size="default">
              <el-table-column prop="item_name" label="条目" />
              <el-table-column label="类别权重" width="110" align="center">
                <template #default="{ row }">
                  <span class="num">{{ (row.category_weight * 100).toFixed(0) }}%</span>
                </template>
              </el-table-column>
              <el-table-column label="条目权重" width="110" align="center">
                <template #default="{ row }">
                  <span class="num">{{ (row.item_weight * 100).toFixed(0) }}%</span>
                </template>
              </el-table-column>
              <el-table-column label="状态" width="130">
                <template #default="{ row }">
                  <el-tag
                    effect="plain"
                    :style="{ color: STATUS_COLOR[row.status as ItemStatus], borderColor: STATUS_COLOR[row.status as ItemStatus] }"
                  >
                    {{ STATUS_TEXT[row.status as ItemStatus] }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column label="评分" width="100" align="center">
                <template #default="{ row }">
                  <span class="num">{{ row.score.toFixed(2) }}</span>
                </template>
              </el-table-column>
            </el-table>
          </el-collapse-item>
        </el-collapse>
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
