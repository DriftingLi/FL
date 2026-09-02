<template>
  <div class="flex flex-col gap-4">
    <h1 class="text-xl font-bold text-ink">我的投递</h1>

    <UiErrorState
      v-if="loadError"
      title="投递记录加载失败"
      description="网络或服务端异常，可重试"
      :retrying="retrying"
      @retry="handleRetry"
    />
    <UiSkeleton v-else-if="loading" variant="list" :count="4" />
    <div v-else-if="items.length === 0" class="rounded-card border border-line bg-panel p-8 text-center text-ink-3">暂无投递记录</div>
    <div v-else class="grid gap-3">
      <div v-for="item in items" :key="String(item.id)" class="rounded-card border border-line bg-panel p-4">
        <div class="flex items-center justify-between gap-3">
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <span class="text-sm font-semibold text-ink">{{ item.job_title || '职位 #' + item.job_posting_id }}</span>
              <el-tag :type="tagType(item.status)" size="small">{{ statusLabel(item.status) }}</el-tag>
              <el-tag v-if="item.employer_viewed_at" type="info" size="small">企业已查看</el-tag>
            </div>
            <div class="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-ink-3">
              <span v-if="item.company_name">{{ item.company_name }}</span>
              <span>投递于 {{ item.created_at }}</span>
            </div>
            <!-- #487：投递即授权（source=application approved）→ 展示企业联系方式 -->
            <CompanyContactInfo
              v-if="approvedContactFor(item)"
              :approved="true"
              :company-name="approvedContactFor(item)?.company_name"
              :contact-name="approvedContactFor(item)?.contact_name"
              :phone="approvedContactFor(item)?.contact_phone"
              :email="approvedContactFor(item)?.contact_email"
              :wechat="approvedContactFor(item)?.wechat"
            />
          </div>
          <UiButton v-if="item.status === 'applied'" size="small" @click="openWithdraw(item)">撤回</UiButton>
        </div>
      </div>
    </div>
    <div v-if="total > 0" class="flex justify-center">
      <el-pagination
        v-model:current-page="page"
        :page-size="pageSize"
        :total="total"
        layout="prev, pager, next"
        @current-change="handlePageChange"
      />
    </div>

    <!-- 撤回弹窗（spec #449 决定 10 的 UI 落点）：「一并撤回联系方式授权」默认不勾选 -->
    <el-dialog v-model="withdrawVisible" title="撤回投递" width="440px">
      <div class="text-sm text-ink">确定撤回这条投递吗？撤回后可以重新投递同一职位。</div>
      <el-checkbox v-model="revokeContact" class="mt-3">一并撤回对该企业的联系方式授权</el-checkbox>
      <template #footer>
        <UiButton @click="withdrawVisible = false">取消</UiButton>
        <UiButton variant="primary" :loading="withdrawing" @click="confirmWithdraw">确认撤回</UiButton>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { jobApi, type JobApplication } from '@/api/job'
import { resumeApi } from '@/api/resume'
import { useAsyncPage } from '@/composables/useAsyncPage'
import UiButton from '@/components/ui/UiButton.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import CompanyContactInfo from '@/components/recruit/CompanyContactInfo.vue'

const items = ref<JobApplication[]>([])
// #487：approved 的联系方式交换（投递产生/企业发起）用于企业联系方式展示
const approvedContacts = ref<any[]>([])
const withdrawVisible = ref(false)
const withdrawing = ref(false)
const revokeContact = ref(false) // 默认不勾选（决定 10）
const current = ref<JobApplication | null>(null)

const {
  loading,
  loadError,
  retrying,
  retry: handleRetry,
  total,
  page,
  pageSize,
  run: load,
  handlePageChange
} = useAsyncPage(async () => {
  const res = await jobApi.listMyApplications({ page: page.value, page_size: pageSize.value })
  items.value = res?.items || []
  total.value = res?.total || 0
  try {
    // 后端列表单页上限 20：翻页取全量 approved，避免申请多时较早企业的联系方式缺失（Standards 审查）
    approvedContacts.value = []
    for (let p = 1; p <= 10; p++) {
      const reqs: any = await resumeApi.listContactRequests({ page: p, page_size: 20 })
      const items = reqs?.items || []
      approvedContacts.value.push(...items.filter((r: any) => r.status === 'approved'))
      if (items.length < 20) break
    }
  } catch {}
})

// 找该投递对应企业的已授权联系方式（按 recruiter_id 匹配）
function approvedContactFor(item: JobApplication) {
  return approvedContacts.value.find((r: any) => r.recruiter_id === item.recruiter_id) || null
}

function statusLabel(s: string) {
  const m: Record<string, string> = { applied: '待处理', rejected: '不合适', withdrawn: '已撤回' }
  return m[s] || s
}
function tagType(s: string) {
  if (s === 'applied') return 'warning'
  if (s === 'rejected') return 'danger'
  return 'info'
}

function openWithdraw(item: JobApplication) {
  current.value = item
  revokeContact.value = false
  withdrawVisible.value = true
}

async function confirmWithdraw() {
  if (!current.value) return
  withdrawing.value = true
  try {
    await jobApi.withdrawApplication(current.value.id, revokeContact.value)
    ElMessage.success('投递已撤回')
    withdrawVisible.value = false
    load()
  } catch (e: any) {
    ElMessage.error(e?.message || '撤回失败')
  } finally {
    withdrawing.value = false
  }
}

onMounted(load)
</script>