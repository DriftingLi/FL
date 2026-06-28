<script setup lang="ts">
// 评估历史记录页（Tesla 极简风：白底 + 表格 + 分页）
// 重构说明：表格列改为新字段（品牌类型/品牌/车辆类型/系列/吨位/车况评级）
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowLeft, Refresh } from '@element-plus/icons-vue'
import PageHeader from '@/components/valuation/PageHeader.vue'
import { listEvaluations } from '@/api/valuation/evaluation'
import type { EvaluationDetail } from '@/types/valuation/evaluation'
import { CONDITION_RATING_COLOR } from '@/utils/valuationConstants'
import { formatTonnage, formatWan } from '@/utils/valuationFormat'

const router = useRouter()

const list = ref<EvaluationDetail[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(10)
const loading = ref(false)
// 筛选项
const filterVehicleType = ref<string>('')
const filterBrand = ref<string>('')

async function load() {
  loading.value = true
  try {
    const params: {
      page: number
      page_size: number
      vehicle_type?: string
      brand?: string
    } = {
      page: page.value,
      page_size: pageSize.value
    }
    if (filterVehicleType.value) params.vehicle_type = filterVehicleType.value
    if (filterBrand.value) params.brand = filterBrand.value
    const result = await listEvaluations(params)
    list.value = result.list || []
    total.value = result.total || 0
  } catch {
    // 拦截器已提示
    list.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function onPageChange(p: number) {
  page.value = p
  load()
}

function onSizeChange(s: number) {
  pageSize.value = s
  page.value = 1
  load()
}

function onFilterChange() {
  page.value = 1
  load()
}

function goReport(id: number) {
  router.push(`/valuation/report/${id}`)
}

function goBack() {
  router.push('/valuation')
}

function formatValue(v: number): string {
  return v.toFixed(2)
}

function formatRate(estimated: number, original: number): string {
  if (!original || original <= 0) return '-'
  return ((estimated / original) * 100).toFixed(1) + '%'
}

function formatTime(t?: string): string {
  if (!t) return '-'
  // 后端返回 RFC3339，取日期部分
  return t.replace('T', ' ').slice(0, 16)
}

function ratingColor(rating: string): string {
  return CONDITION_RATING_COLOR[rating] || '#666'
}

onMounted(load)
</script>

<template>
  <div class="app-container history-view valuation-root">
    <PageHeader
      title="评估历史记录"
      subtitle="evaluation history"
    >
      <template #actions>
        <el-button :icon="ArrowLeft" @click="goBack">返回首页</el-button>
        <el-button :icon="Refresh" :loading="loading" @click="load">刷新</el-button>
      </template>
    </PageHeader>

    <!-- 筛选栏 -->
    <section class="filter-bar card-surface">
      <span class="filter-label">车辆类型：</span>
      <el-input
        v-model="filterVehicleType"
        placeholder="如 电动平衡重"
        clearable
        style="width: 200px"
        @clear="onFilterChange"
        @keyup.enter="onFilterChange"
      />
      <span class="filter-label">品牌：</span>
      <el-input
        v-model="filterBrand"
        placeholder="如 杭叉"
        clearable
        style="width: 160px"
        @clear="onFilterChange"
        @keyup.enter="onFilterChange"
      />
      <el-button type="primary" @click="onFilterChange">查询</el-button>
    </section>

    <!-- 列表表格 -->
    <section class="card-surface table-section">
      <el-table
        v-loading="loading"
        :data="list"
        stripe
        style="width: 100%"
        empty-text="暂无评估记录"
        @row-click="(row: EvaluationDetail) => goReport(row.id)"
      >
        <el-table-column prop="id" label="编号" width="80" align="center" />
        <el-table-column label="品牌类型" width="120">
          <template #default="{ row }">
            {{ row.brand_type || '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="brand" label="品牌" width="120" />
        <el-table-column label="车辆类型" width="140">
          <template #default="{ row }">
            {{ row.vehicle_type || '-' }}
          </template>
        </el-table-column>
        <el-table-column label="系列" width="120">
          <template #default="{ row }">
            {{ row.series || '-' }}
          </template>
        </el-table-column>
        <el-table-column label="吨位" width="100" align="right">
          <template #default="{ row }">
            <span class="num">{{ formatTonnage(row.tonnage) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="车况评级" width="100" align="center">
          <template #default="{ row }">
            <el-tag
              effect="plain"
              :style="{
                color: ratingColor(row.condition_rating),
                borderColor: ratingColor(row.condition_rating)
              }"
            >
              {{ row.condition_rating || '-' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="残值（万元）" width="130" align="right">
          <template #default="{ row }">
            <span class="value-cell">{{ formatValue(row.estimated_value) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="残值率" width="110" align="right">
          <template #default="{ row }">
            <span class="rate-cell">{{ formatRate(row.estimated_value, row.original_price) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="原始价格" width="120" align="right">
          <template #default="{ row }">
            <span class="num">{{ formatWan(row.original_price) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="评估时间" min-width="160">
          <template #default="{ row }">
            {{ formatTime(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="100" align="center" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click.stop="goReport(row.id)">
              查看报告
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- 分页 -->
      <div class="pagination-wrap">
        <el-pagination
          v-model:current-page="page"
          v-model:page-size="pageSize"
          :total="total"
          :page-sizes="[10, 20, 50]"
          layout="total, sizes, prev, pager, next, jumper"
          background
          @current-change="onPageChange"
          @size-change="onSizeChange"
        />
      </div>
    </section>
  </div>
</template>

<style scoped>
.history-view {
  padding: 0;
}
.filter-bar {
  padding: var(--sp-4) var(--sp-6);
  margin-bottom: var(--sp-5);
  display: flex;
  align-items: center;
  gap: var(--sp-3);
}
.filter-label {
  font-size: var(--fs-base);
  font-weight: var(--fw-medium);
  color: var(--color-text);
}
.table-section {
  padding: var(--sp-4) var(--sp-6) var(--sp-5);
}
.value-cell {
  font-family: var(--font-mono);
  font-weight: var(--fw-medium);
  color: var(--color-primary);
}
.rate-cell {
  font-family: var(--font-mono);
  font-size: var(--fs-sm);
  color: var(--color-text-secondary);
}
.pagination-wrap {
  display: flex;
  justify-content: flex-end;
  margin-top: var(--sp-4);
}

:deep(.el-table__row) {
  cursor: pointer;
}

@media (max-width: 768px) {
  .filter-bar {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--sp-2);
    padding: var(--sp-3) var(--sp-4);
  }
  .table-section {
    padding: var(--sp-3) var(--sp-4);
  }
  .pagination-wrap {
    justify-content: center;
  }
}
</style>
