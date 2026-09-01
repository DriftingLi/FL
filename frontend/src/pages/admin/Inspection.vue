<template>
  <div class="flex flex-col gap-6 p-4">
    <h1 class="text-xl font-bold text-ink">巡检视图</h1>
    <div class="rounded-card border border-line bg-panel p-4">
      <div class="text-sm text-ink-3">删除已解决帖计数</div>
      <div class="mt-1 text-2xl font-bold text-ink">{{ deletedCount }}</div>
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
      <div v-if="ledgerLoading" class="text-sm text-ink-3">加载中...</div>
      <div v-else-if="ledger.length === 0" class="text-sm text-ink-3">暂无数据</div>
      <div v-else class="grid gap-2">
        <div v-for="item in ledger" :key="String(item.id)" class="border border-line rounded p-2 text-xs">
          <div>用户 {{ item.user_id }} · {{ item.reason }} · {{ item.delta }} 分 · {{ refLabel(item.ref_type) }} {{ item.ref_id }}</div>
          <div class="text-ink-3">{{ item.created_at }}</div>
        </div>
      </div>
      <div class="mt-3 flex justify-end">
        <el-pagination
          v-model:current-page="page"
          v-model:page-size="pageSize"
          :total="total"
          :page-sizes="[10, 20, 50]"
          layout="total, sizes, prev, pager, next"
          @current-change="handlePageChange"
          @size-change="loadLedger"
        />
      </div>
    </div>

    <div class="rounded-card border border-line bg-panel p-4">
      <div class="flex items-center gap-2 mb-3">
        <span class="text-sm font-semibold text-ink">简历查看留痕</span>
        <UiButton size="small" @click="loadViews">刷新</UiButton>
      </div>
      <div v-if="viewsLoading" class="text-sm text-ink-3">加载中...</div>
      <div v-else-if="views.length === 0" class="text-sm text-ink-3">暂无数据</div>
      <div v-else class="grid gap-2">
        <div v-for="item in views" :key="String(item.id)" class="border border-line rounded p-2 text-xs">
          <div>招聘方 {{ item.recruiter_id }} · 学员 {{ item.resume_user_id }} · {{ item.viewed_at }}</div>
        </div>
      </div>
      <div class="mt-3 flex justify-end">
        <el-pagination
          v-model:current-page="viewsPage"
          :page-size="20"
          :total="viewsTotal"
          layout="total, prev, pager, next"
          @current-change="loadViews"
        />
      </div>
    </div>

    <div class="rounded-card border border-line bg-panel p-4">
      <div class="flex items-center gap-2 mb-3">
        <span class="text-sm font-semibold text-ink">联系方式申请记录</span>
        <UiButton size="small" @click="loadRequests">刷新</UiButton>
      </div>
      <div v-if="requestsLoading" class="text-sm text-ink-3">加载中...</div>
      <div v-else-if="requests.length === 0" class="text-sm text-ink-3">暂无数据</div>
      <div v-else class="grid gap-2">
        <div v-for="item in requests" :key="String(item.id)" class="border border-line rounded p-2 text-xs">
          <div>招聘方 {{ item.recruiter_id }} · 学员 {{ item.student_user_id }} · {{ requestStatusLabel(item.status) }}</div>
          <div class="text-ink-3">{{ item.created_at }}</div>
        </div>
      </div>
      <div class="mt-3 flex justify-end">
        <el-pagination
          v-model:current-page="requestsPage"
          :page-size="20"
          :total="requestsTotal"
          layout="total, prev, pager, next"
          @current-change="loadRequests"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { unwrappedRequest } from '@/api/request'
import { useAsyncPage } from '@/composables/useAsyncPage'
import UiButton from '@/components/ui/UiButton.vue'

// #411：默认锁定问答域（forum_topic），显式切换才跨域全量——卡片标题与内容同域。
const domain = ref<'forum_topic' | ''>('forum_topic')
const reason = ref('')
const userId = ref('')
const ledger = ref<LedgerItem[]>([])

// 流水列表三态 + 分页收编 useAsyncPage（#439）
const {
  loading: ledgerLoading,
  total,
  page,
  pageSize,
  run: loadLedger,
  handlePageChange
} = useAsyncPage(async () => {
  const params: Record<string, any> = { page: page.value, page_size: pageSize.value }
  if (domain.value) params.ref_type = domain.value
  if (reason.value) params.reason = reason.value
  if (userId.value) params.user_id = userId.value
  const res: any = await unwrappedRequest.get('/admin/points/ledger', { params, headers: { 'X-Silent': '1' } })
  ledger.value = res?.items || []
  total.value = res?.total ?? 0
})

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

const deletedCount = ref(0)
async function loadCount() {
  try {
    const res: any = await unwrappedRequest.get('/admin/inspection/deleted-after-accepted', { headers: { 'X-Silent': '1' } })
    deletedCount.value = res?.count ?? 0
  } catch {}
}
// 切换域即时重载（#411）：v-model 变更即刷新，不依赖下拉的 change 事件时序。
watch(domain, () => loadLedger())

// #418：招聘留痕/申请记录区块（只呈现事实字段，不泄漏学员明文联系方式）
const views = ref<TrailView[]>([])
const viewsLoading = ref(false)
const viewsPage = ref(1)
const viewsTotal = ref(0)
const requests = ref<TrailRequest[]>([])
const requestsLoading = ref(false)
const requestsPage = ref(1)
const requestsTotal = ref(0)

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
  status: string
  created_at: string
}

// 申请状态文案与学员侧一致（pending/approved/rejected/expired/revoked）
const requestStatusText: Record<string, string> = {
  pending: '待同意',
  approved: '已同意',
  rejected: '已拒绝',
  expired: '已过期',
  revoked: '已撤回',
}
function requestStatusLabel(s: string): string {
  return requestStatusText[s] || s
}

async function loadViews() {
  viewsLoading.value = true
  try {
    const res: any = await unwrappedRequest.get('/admin/recruit/views', {
      params: { page: viewsPage.value, page_size: 20 },
      headers: { 'X-Silent': '1' },
    })
    views.value = res?.items || []
    viewsTotal.value = res?.total ?? 0
  } catch {}
  viewsLoading.value = false
}

async function loadRequests() {
  requestsLoading.value = true
  try {
    const res: any = await unwrappedRequest.get('/admin/recruit/requests', {
      params: { page: requestsPage.value, page_size: 20 },
      headers: { 'X-Silent': '1' },
    })
    requests.value = res?.items || []
    requestsTotal.value = res?.total ?? 0
  } catch {}
  requestsLoading.value = false
}

onMounted(() => {
  loadCount()
  loadLedger()
  loadViews()
  loadRequests()
})
</script>
