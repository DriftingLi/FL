<script setup lang="ts">
/**
 * 投稿审核管理（#517）：待审核队列（通过/驳回）+ 举报处置队列（下架/驳回举报）。
 * 后端 V1 即 tutor+admin 双角色鉴权；讲师端前端二期，本页现仅在管理端挂出。
 */
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import {
  adminContributionApi,
  type ContributionItem,
  type ContributionReportItem
} from '@/api/contribution'
import { formatLocaleDateTime } from '@/utils/format'
import { resolveFileUrl } from '@/utils/fileUrl'
import UiButton from '@/components/ui/UiButton.vue'

const activeTab = ref<'pending' | 'reports'>('pending')

// ===== 待审核队列 =====
const pendingItems = ref<ContributionItem[]>([])
const pendingLoading = ref(false)
const pendingPage = ref(1)
const pendingPageSize = 20
const pendingTotal = ref(0)

async function loadPending() {
  pendingLoading.value = true
  try {
    const res = await adminContributionApi.listPending({ page: pendingPage.value, page_size: pendingPageSize })
    pendingItems.value = res.items || []
    pendingTotal.value = res.total || 0
  } catch (e: any) {
    ElMessage.error(e?.message || '加载审核队列失败')
  } finally {
    pendingLoading.value = false
  }
}

// ===== 举报队列 =====
const reports = ref<ContributionReportItem[]>([])
const reportLoading = ref(false)
const reportStatus = ref<number>(-1)
const reportPage = ref(1)
const reportPageSize = 20
const reportTotal = ref(0)

async function loadReports() {
  reportLoading.value = true
  try {
    const st = reportStatus.value
    const res = await adminContributionApi.listReports({
      status: st >= 0 ? st : undefined,
      page: reportPage.value,
      page_size: reportPageSize
    })
    reports.value = res.items || []
    reportTotal.value = res.total || 0
  } catch (e: any) {
    ElMessage.error(e?.message || '加载举报队列失败')
  } finally {
    reportLoading.value = false
  }
}

const REPORT_REASON_LABEL: Record<string, string> = {
  piracy: '盗版',
  content_error: '内容错误',
  violation: '违规',
  stale: '已失效'
}

// ===== 操作 =====
async function onApprove(row: ContributionItem) {
  try {
    await ElMessageBox.confirm(`通过「${row.title}」？作者将 +50 分。`, '审核通过', {
      type: 'info',
      confirmButtonText: '通过并发分',
      cancelButtonText: '取消'
    })
  } catch {
    return
  }
  try {
    await adminContributionApi.approve(row.id)
    ElMessage.success('已通过，+50 分已入账')
    await loadPending()
  } catch (e: any) {
    ElMessage.error(e?.message || '操作失败')
  }
}

async function onReject(row: ContributionItem) {
  let reason = ''
  try {
    const { value } = await ElMessageBox.prompt(`驳回「${row.title}」，请填写原因（将送达作者）：`, '驳回投稿', {
      confirmButtonText: '驳回',
      cancelButtonText: '取消',
      inputType: 'textarea',
      inputValidator: (v: string) => (v && v.trim() ? true : '驳回原因必填')
    })
    reason = value?.trim() || ''
  } catch {
    return
  }
  try {
    await adminContributionApi.reject(row.id, reason)
    ElMessage.success('已驳回')
    await loadPending()
  } catch (e: any) {
    ElMessage.error(e?.message || '操作失败')
  }
}

async function onHandleReport(row: ContributionReportItem, action: 'archive' | 'dismiss') {
  if (action === 'archive') {
    try {
      await ElMessageBox.confirm(`下架被举报投稿 #${row.contribution_id}（${row.contribution_title || ''}）并回收已发积分？`, '处置举报：下架', {
        type: 'warning',
        confirmButtonText: '下架并回收',
        cancelButtonText: '取消'
      })
    } catch {
      return
    }
  }
  try {
    await adminContributionApi.handleReport(row.id, action)
    ElMessage.success(action === 'archive' ? '已下架并回收' : '已驳回举报')
    await loadReports()
  } catch (e: any) {
    ElMessage.error(e?.message || '操作失败')
  }
}

function handleTabChange() {
  if (activeTab.value === 'pending') {
    pendingPage.value = 1
    loadPending()
  } else {
    reportPage.value = 1
    loadReports()
  }
}

function handleReportStatusChange() {
  reportPage.value = 1
  loadReports()
}

function formatSize(bytes?: number): string {
  if (!bytes || bytes <= 0) return '-'
  if (bytes < 1024) return `${bytes}B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)}KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)}MB`
  return `${(bytes / 1024 / 1024 / 1024).toFixed(1)}GB`
}

onMounted(() => {
  loadPending()
})
</script>

<template>
  <div class="contribution-manage-page">
    <el-card>
      <template #header>
        <div class="card-header">
          <span class="card-title">资料投稿管理</span>
          <UiButton :icon="Refresh" circle @click="activeTab === 'pending' ? loadPending() : loadReports()" />
        </div>
      </template>

      <el-tabs v-model="activeTab" @tab-change="handleTabChange">
        <el-tab-pane label="待审核" name="pending" />
        <el-tab-pane label="举报处置" name="reports" />
      </el-tabs>

      <!-- ===== 待审核队列 ===== -->
      <template v-if="activeTab === 'pending'">
        <el-table v-loading="pendingLoading" :data="pendingItems" border>
          <el-table-column prop="id" label="ID" width="70" align="center" />
          <el-table-column label="投稿" min-width="220">
            <template #default="{ row }">
              <div class="font-medium">{{ row.title }}</div>
              <div class="text-xs text-muted">{{ row.intro }}</div>
            </template>
          </el-table-column>
          <el-table-column label="作者" width="110">
            <template #default="{ row }">{{ row.author?.anonymous ? '匿名': row.author?.username || '—' }}</template>
          </el-table-column>
          <el-table-column label="文件" width="140">
            <template #default="{ row }">
              <template v-for="f in (row.files || [])" :key="f.file_url">
                <a class="file-link" :href="resolveFileUrl(f.file_url)" target="_blank" rel="noopener">{{ f.file_name }}（{{ formatSize(f.file_size) }}）</a>
              </template>
            </template>
          </el-table-column>
          <el-table-column label="提交时间" width="160" align="center">
            <template #default="{ row }">{{ formatLocaleDateTime(row.created_at) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="150" fixed="right" align="center">
            <template #default="{ row }">
              <UiButton variant="success" size="small" link @click="onApprove(row)">通过</UiButton>
              <UiButton variant="danger" size="small" link @click="onReject(row)">驳回</UiButton>
            </template>
          </el-table-column>
        </el-table>
        <div class="pagination-wrapper" v-if="pendingTotal > pendingPageSize">
          <el-pagination v-model:current-page="pendingPage" :page-size="pendingPageSize" :total="pendingTotal"
            layout="total, prev, pager, next" @current-change="loadPending" />
        </div>
      </template>

      <!-- ===== 举报处置 ===== -->
      <template v-else>
        <div class="filter-bar">
          <el-radio-group :model-value="reportStatus" @update:model-value="(v: any) => { reportStatus = v as number }" @change="handleReportStatusChange">
            <el-radio-button :value="-1">全部</el-radio-button>
            <el-radio-button :value="0">待处理</el-radio-button>
            <el-radio-button :value="1">已处理</el-radio-button>
          </el-radio-group>
        </div>
        <el-table v-loading="reportLoading" :data="reports" border>
          <el-table-column prop="id" label="ID" width="70" align="center" />
          <el-table-column label="被举报投稿" min-width="220">
            <template #default="{ row }">#{{ row.contribution_id }} {{ row.contribution_title || '' }}</template>
          </el-table-column>
          <el-table-column label="举报理由" width="120">
            <template #default="{ row }">{{ REPORT_REASON_LABEL[row.reason] || row.reason }}</template>
          </el-table-column>
          <el-table-column label="状态" width="90" align="center">
            <template #default="{ row }">
              <el-tag size="small" :type="row.status === 1 ? 'success' : 'danger'">
                {{ row.status === 1 ? '已处理' : '待处理' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="时间" width="160" align="center">
            <template #default="{ row }">{{ formatLocaleDateTime(row.created_at) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="160" fixed="right" align="center">
            <template #default="{ row }">
              <template v-if="row.status === 0">
                <UiButton variant="danger" size="small" link @click="onHandleReport(row, 'archive')">下架并回收</UiButton>
                <UiButton size="small" link @click="onHandleReport(row, 'dismiss')">驳回举报</UiButton>
              </template>
              <span v-else class="report-done">—</span>
            </template>
          </el-table-column>
        </el-table>
        <div class="pagination-wrapper" v-if="reportTotal > reportPageSize">
          <el-pagination v-model:current-page="reportPage" :page-size="reportPageSize" :total="reportTotal"
            layout="total, prev, pager, next" @current-change="loadReports" />
        </div>
      </template>
    </el-card>
  </div>
</template>

<style scoped>
/* 页面布局与表格（沿用 admin 既有卡片风格；变量随 admin 主题） */
.contribution-manage-page { padding: 16px; }
.card-header { display: flex; align-items: center; justify-content: space-between; }
.card-title { font-size: 16px; font-weight: 600; }
.filter-bar { margin: 12px 0; }
.pagination-wrapper { display: flex; justify-content: center; margin-top: 16px; }
.file-link { display: block; color: var(--el-color-primary); font-size: 12px; line-height: 1.8; }
.file-link:hover { text-decoration: underline; }
.report-done { color: #999; }
.text-muted { color: #909399; margin-top: 2px; }
</style>