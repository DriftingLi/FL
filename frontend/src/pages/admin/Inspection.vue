<!--
  列表档位：useAdminTable（分页列表）—— 问答积分流水 / 简历查看留痕 / 联系方式申请记录 / 招聘职位巡检 / 职位举报队列
  列表档位：useAsyncPage（只读计数）—— 删除已解决帖计数
  两档共用同一条空态判据（utils/listState.isEmptyList，经各自 isEmpty 暴露）；两档都不得静默吞错。
  依据 ADR-0056 §9；判定口径见 docs/agents/ui-conventions.md「管理端列表两档归属」。
-->
<template>
  <div class="flex flex-col gap-6 p-4">
    <h1 class="text-xl font-bold text-ink">巡检视图</h1>
    <div class="rounded-card border border-line bg-panel p-4">
      <div class="text-sm text-ink-3">删除已解决帖计数</div>
      <!-- 第二档（只读计数）：useAsyncPage + UiAsyncSection，计数不硬套列表状态机（ADR-0056 §9） -->
      <UiAsyncSection
        :error="countError"
        :loading="countLoading"
        :retrying="countRetrying"
        :skeleton="false"
        error-title="计数加载失败"
        error-description="网络或服务端异常，可重试"
        @retry="retryCount"
      >
        <div class="mt-1 text-2xl font-bold text-ink">{{ deletedCount }}</div>
      </UiAsyncSection>
      <div class="mt-1 text-xs text-ink-3">楼主删除自己已解决的帖子时累加，不自动惩罚、不回滚积分</div>
    </div>
    <div class="rounded-card border border-line bg-panel p-4">
      <div class="flex items-center gap-2 mb-3">
        <span class="text-sm font-semibold text-ink">问答积分流水</span>
        <el-select v-model="domain" class="!w-44">
          <el-option label="问答域" value="forum_topic" />
          <el-option label="跨业务域全量" value="" />
        </el-select>
        <el-select v-model="reason" placeholder="按原因筛选" clearable class="!w-40" @change="loadLedger">
          <el-option label="答主被采纳" value="accepted_bonus" />
          <el-option label="楼主采纳" value="accept_action" />
          <el-option label="违规回收" value="rollback" />
        </el-select>
        <el-input v-model="userId" placeholder="按用户ID过滤" clearable class="!w-40" @change="loadLedger" />
        <UiButton size="small" @click="loadLedger">刷新</UiButton>
      </div>
      <UiAsyncSection
        :error="ledgerError"
        :loading="ledgerLoading"
        :empty="isEmptyLedger"
        :retrying="ledgerRetrying"
        :skeleton="false"
        error-title="流水加载失败"
        error-description="网络或服务端异常，可重试"
        @retry="retryLedger"
      >
        <div v-if="ledgerLoading" class="text-sm text-ink-3">加载中...</div>
        <div v-else class="grid gap-2">
        <div v-for="item in ledger" :key="String(item.id)" class="border border-line rounded p-2 text-xs">
          <div>用户 {{ item.user_id }} · {{ item.reason }} · {{ item.delta }} 分 · {{ refLabel(item.ref_type) }} {{ item.ref_id }}</div>
          <div class="text-ink-3">{{ item.created_at }}</div>
        </div>
      </div>
        <template #empty>
          <UiEmptyState description="暂无数据" size="sm" />
        </template>
      </UiAsyncSection>
      <div class="mt-3 flex justify-end">
        <UiPagination
      v-model:current-page="page"
      v-model:page-size="pageSize"
      :total="total"
      show-sizes
      :page-sizes="[10, 20, 50]"
      @current-change="handlePageChange"
      @size-change="handleLedgerSizeChange"
    />
      </div>
    </div>

    <div class="rounded-card border border-line bg-panel p-4">
      <div class="flex items-center gap-2 mb-3">
        <span class="text-sm font-semibold text-ink">简历查看留痕</span>
        <UiButton size="small" @click="loadViews">刷新</UiButton>
      </div>
      <UiAsyncSection
        :error="viewsError"
        :loading="viewsLoading"
        :empty="isEmptyViews"
        :retrying="viewsRetrying"
        :skeleton="false"
        error-title="留痕加载失败"
        error-description="网络或服务端异常，可重试"
        @retry="retryViews"
      >
        <div v-if="viewsLoading" class="text-sm text-ink-3">加载中...</div>
        <div v-else class="grid gap-2">
          <div v-for="item in views" :key="String(item.id)" class="border border-line rounded p-2 text-xs">
            <div>招聘方 {{ item.recruiter_id }} · 学员 {{ item.resume_user_id }} · {{ item.viewed_at }}</div>
          </div>
        </div>
        <template #empty>
          <UiEmptyState description="暂无数据" size="sm" />
        </template>
      </UiAsyncSection>
      <div class="mt-3 flex justify-end">
        <UiPagination
      v-model:current-page="viewsPage"
      :page-size="20"
      :total="viewsTotal"
      @current-change="loadViews"
    />
      </div>
    </div>

    <div class="rounded-card border border-line bg-panel p-4">
      <div class="flex items-center gap-2 mb-3">
        <span class="text-sm font-semibold text-ink">联系方式申请记录</span>
        <UiButton size="small" @click="loadRequests">刷新</UiButton>
      </div>
      <UiAsyncSection
        :error="requestsError"
        :loading="requestsLoading"
        :empty="isEmptyRequests"
        :retrying="requestsRetrying"
        :skeleton="false"
        error-title="申请记录加载失败"
        error-description="网络或服务端异常，可重试"
        @retry="retryRequests"
      >
        <div v-if="requestsLoading" class="text-sm text-ink-3">加载中...</div>
        <div v-else class="grid gap-2">
          <div v-for="item in requests" :key="String(item.id)" class="border border-line rounded p-2 text-xs">
            <div>招聘方 {{ item.recruiter_id }} · 学员 {{ item.student_user_id }} · {{ describeContactRequest(item.status).label }}</div>
            <div class="text-ink-3">{{ item.created_at }}</div>
          </div>
        </div>
        <template #empty>
          <UiEmptyState description="暂无数据" size="sm" />
        </template>
      </UiAsyncSection>
      <div class="mt-3 flex justify-end">
        <UiPagination
      v-model:current-page="requestsPage"
      :page-size="20"
      :total="requestsTotal"
      @current-change="loadRequests"
    />
      </div>
    </div>

    <div class="rounded-card border border-line bg-panel p-4">
      <div class="flex items-center gap-2 mb-3">
        <span class="text-sm font-semibold text-ink">招聘职位巡检</span>
        <el-input v-model="jobFilterRecruiter" placeholder="按企业 ID 筛" clearable class="!w-32" @change="loadJobs" />
        <UiButton size="small" @click="loadJobs">刷新</UiButton>
      </div>
      <UiAsyncSection
        :error="jobsError"
        :loading="jobsLoading"
        :empty="isEmptyJobs"
        :retrying="jobsRetrying"
        :skeleton="false"
        error-title="职位加载失败"
        error-description="网络或服务端异常，可重试"
        @retry="retryJobs"
      >
        <div v-if="jobsLoading" class="text-sm text-ink-3">加载中...</div>
        <div v-else class="grid gap-2">
          <div v-for="item in jobs" :key="String(item.id)" class="border border-line rounded p-2 text-xs">
            <div class="flex items-center justify-between gap-2">
              <div>
                <span class="font-semibold text-ink">{{ item.title }}</span>
                <span v-if="item.forced_offline" class="ml-2 text-red-500">已强制下架：{{ item.offline_reason }}</span>
                <span v-else-if="item.status === 'closed'" class="ml-2 text-ink-3">已下架</span>
                <span class="ml-2 text-ink-3">企业 {{ item.recruiter_id }} · {{ item.region }}</span>
              </div>
              <div class="flex items-center gap-2 shrink-0">
                <UiButton v-if="!item.forced_offline" size="small" @click="openForceOffline(item)">强制下架</UiButton>
              </div>
            </div>
          </div>
        </div>
        <template #empty>
          <UiEmptyState description="暂无数据" size="sm" />
        </template>
      </UiAsyncSection>
      <div class="mt-3 flex justify-end">
        <UiPagination
      v-model:current-page="jobsPage"
      :page-size="20"
      :total="jobsTotal"
      @current-change="loadJobs"
    />
      </div>
    </div>

    <div class="rounded-card border border-line bg-panel p-4">
      <div class="flex items-center gap-2 mb-3">
        <span class="text-sm font-semibold text-ink">职位举报队列</span>
        <UiButton size="small" @click="loadReports">刷新</UiButton>
      </div>
      <UiAsyncSection
        :error="reportsError"
        :loading="reportsLoading"
        :empty="isEmptyReports"
        :retrying="reportsRetrying"
        :skeleton="false"
        error-title="举报队列加载失败"
        error-description="网络或服务端异常，可重试"
        @retry="retryReports"
      >
        <div v-if="reportsLoading" class="text-sm text-ink-3">加载中...</div>
        <div v-else class="grid gap-2">
          <div v-for="item in reports" :key="String(item.id)" class="border border-line rounded p-2 text-xs">
            <div class="flex items-center justify-between gap-2">
              <div>
                <span class="font-semibold text-ink">{{ item.job_title }}</span>
                <span class="ml-2 text-ink-3">职位 #{{ item.job_posting_id }} · 举报人 {{ item.student_user_id }}</span>
                <div class="text-ink-3">{{ item.reason }}</div>
                <div class="text-ink-3">{{ item.created_at }}</div>
              </div>
              <UiButton size="small" @click="markHandled(item)">标记已处理</UiButton>
            </div>
          </div>
        </div>
        <template #empty>
          <UiEmptyState description="暂无待处理举报" size="sm" />
        </template>
      </UiAsyncSection>
      <div class="mt-3 flex justify-end">
        <UiPagination
      v-model:current-page="reportsPage"
      :page-size="20"
      :total="reportsTotal"
      @current-change="loadReports"
    />
      </div>
    </div>

    <UiDialog v-model="forceOfflineVisible" title="强制下架职位" width="440px" confirm-text="确认下架" :confirm-loading="forceOfflineing" @confirm="confirmForceOffline">
      <div class="text-sm text-ink">职位「{{ forceOfflineJob?.title }}」将被强制下架，学员侧立即不可见，企业不能自行重新上架。</div>
      <el-input v-model="forceOfflineReason" type="textarea" :rows="3" maxlength="500" show-word-limit placeholder="请填写下架原因（将邮件通知企业）" />
    </UiDialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { inspectionApi, type PageParams, type PointsLedgerParams } from '@/api/inspection'
import { useAdminTable } from '@/composables/useAdminTable'
import { useAsyncPage } from '@/composables/useAsyncPage'
import { describeContactRequest, type ContactRequestStatus } from '@/utils/contactRequestStatus'
import UiButton from '@/components/ui/UiButton.vue'
import UiPagination from '@/components/ui/UiPagination.vue'
import UiAsyncSection from '@/components/ui/UiAsyncSection.vue'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiDialog from '@/components/ui/UiDialog.vue'

// #411：默认锁定问答域（forum_topic），显式切换才跨域全量——卡片标题与内容同域。
const domain = ref<'forum_topic' | ''>('forum_topic')
const reason = ref('')
const userId = ref('')
// 流水列表（档位一：分页列表 → useAdminTable，#792 / ADR-0039）——三态 + 分页 + 列表托管。
// 解构改名保持模板零改动；domain/reason/userId 为页面自管筛选轴（由 fetch adapter 读取）。
const {
  loading: ledgerLoading,
  loadError: ledgerError,
  retrying: ledgerRetrying,
  list: ledger,
  total,
  currentPage: page,
  pageSize,
  isEmpty: isEmptyLedger,
  load: loadLedger,
  retry: retryLedger
} = useAdminTable<LedgerItem>({
  fetch: async (paging) => {
    const params: PointsLedgerParams = { page: paging.page, page_size: paging.pageSize }
    if (domain.value) params.ref_type = domain.value
    if (reason.value) params.reason = reason.value
    if (userId.value) params.user_id = userId.value
    return inspectionApi.pointsLedger<LedgerItem>(params)
  }
})

/** 翻页重装：useAdminTable 的 load 读取 currentPage */
function handlePageChange(): void {
  void loadLedger()
}

/** 页大小变化：回第一页重装（与既有三态件语义一致） */
function handleLedgerSizeChange(): void {
  page.value = 1
  void loadLedger()
}

interface LedgerItem {
  id: number
  user_id: number
  reason: string
  delta: number
  ref_type: string
  ref_id: string
  created_at: string
}

// 行内引用按业务域渲染量词（#411）：非问答行不再统一显示「帖 …」。
const refQuantifier: Record<string, string> = {
  forum_topic: '帖',
  task: '任务',
  course: '课程',
  shop: '商品',
  real_exam_paper: '商品',
  ai_chat: 'AI 对话',
  admin: '罚分',
  rollback: '回收',
}
function refLabel(refType: string): string {
  return refQuantifier[refType] || '引用'
}

// 单值计数（档位二：只读计数 → useAsyncPage + UiAsyncSection，ADR-0056 §9）：count 不硬套
// 列表状态机；失败由 loadError 承载并给出重试入口（原先 catch 空块静默吞错、无任何回执）。
const deletedCount = ref(0)
const {
  loading: countLoading,
  loadError: countError,
  retrying: countRetrying,
  run: loadCount,
  retry: retryCount
} = useAsyncPage(async () => {
  const res = await inspectionApi.deletedAfterAccepted()
  deletedCount.value = res?.count ?? 0
})
// 切换域即时重载（#411）：v-model 变更即刷新，不依赖下拉的 change 事件时序。
watch(domain, () => loadLedger())

// #418：招聘留痕 / 申请记录（档位一：分页列表；只呈现事实字段，不泄漏学员明文联系方式）
const {
  loading: viewsLoading,
  loadError: viewsError,
  retrying: viewsRetrying,
  list: views,
  total: viewsTotal,
  currentPage: viewsPage,
  isEmpty: isEmptyViews,
  load: loadViews,
  retry: retryViews
} = useAdminTable<TrailView>({
  pageSize: 20,
  fetch: (paging) => inspectionApi.resumeViews<TrailView>({ page: paging.page, page_size: paging.pageSize })
})
const {
  loading: requestsLoading,
  loadError: requestsError,
  retrying: requestsRetrying,
  list: requests,
  total: requestsTotal,
  currentPage: requestsPage,
  isEmpty: isEmptyRequests,
  load: loadRequests,
  retry: retryRequests
} = useAdminTable<TrailRequest>({
  pageSize: 20,
  fetch: (paging) => inspectionApi.contactRequests<TrailRequest>({ page: paging.page, page_size: paging.pageSize })
})

interface TrailView {
  id: number
  recruiter_id: number
  resume_user_id: number
  viewed_at: string
}

interface TrailRequest {
  id: number
  recruiter_id: number
  student_user_id: number
  // 联络授权状态：与学员/企业侧同一 union（admin 留痕只是消费面，不自立取值域）。
  status: ContactRequestStatus
  created_at: string
}

// #454：招聘职位巡检 + 举报队列（职位治理）——两段都是档位一：分页列表。
// jobFilterRecruiter 是页面自管筛选轴，由 fetch adapter 读取（不塞进 composable）。
const jobFilterRecruiter = ref('')
const {
  loading: jobsLoading,
  loadError: jobsError,
  retrying: jobsRetrying,
  list: jobs,
  total: jobsTotal,
  currentPage: jobsPage,
  isEmpty: isEmptyJobs,
  load: loadJobs,
  retry: retryJobs
} = useAdminTable<any>({
  pageSize: 20,
  fetch: (paging) => {
    const params: PageParams = { page: paging.page, page_size: paging.pageSize }
    if (jobFilterRecruiter.value) params.recruiter_id = jobFilterRecruiter.value
    return inspectionApi.jobs<any>(params)
  }
})
const {
  loading: reportsLoading,
  loadError: reportsError,
  retrying: reportsRetrying,
  list: reports,
  total: reportsTotal,
  currentPage: reportsPage,
  isEmpty: isEmptyReports,
  load: loadReports,
  retry: retryReports
} = useAdminTable<any>({
  pageSize: 20,
  fetch: (paging) => inspectionApi.jobReports<any>({ page: paging.page, page_size: paging.pageSize })
})
const forceOfflineVisible = ref(false)
const forceOfflineJob = ref<any>(null)
const forceOfflineReason = ref('')
const forceOfflineing = ref(false)

function openForceOffline(item: any) {
  forceOfflineJob.value = item
  forceOfflineReason.value = ''
  forceOfflineVisible.value = true
}

async function confirmForceOffline() {
  if (!forceOfflineJob.value) return
  if (!forceOfflineReason.value.trim()) {
    ElMessage.warning('下架原因不能为空')
    return
  }
  forceOfflineing.value = true
  try {
    await inspectionApi.forceOfflineJob(forceOfflineJob.value.id, forceOfflineReason.value.trim())
    ElMessage.success('职位已强制下架')
    forceOfflineVisible.value = false
    loadJobs()
    loadReports()
  } catch (e: any) {
    ElMessage.error(e?.message || '操作失败')
  } finally {
    forceOfflineing.value = false
  }
}

async function markHandled(item: any) {
  try {
    await inspectionApi.handleJobReport(item.id)
    ElMessage.success('举报已标记为已处理')
    loadReports()
  } catch (e: any) {
    ElMessage.error(e?.message || '操作失败')
  }
}

onMounted(() => {
  loadCount()
  loadLedger()
  loadViews()
  loadRequests()
  loadJobs()
  loadReports()
})
</script>
