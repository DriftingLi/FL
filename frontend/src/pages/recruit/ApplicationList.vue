<template>
  <div class="flex flex-col gap-4">
    <div class="flex items-center gap-2">
      <router-link to="/recruit/jobs" class="text-sm text-ui-600 hover:text-ui-700">← 返回职位管理</router-link>
    </div>
    <div class="flex items-center justify-between">
      <h1 class="text-xl font-bold text-ink">投递列表{{ jobTitle ? `：${jobTitle}` : '' }}</h1>
      <span v-if="unreadCount > 0" class="text-xs text-ink-3">{{ unreadCount }} 条未读</span>
    </div>

    <UiErrorState
      v-if="loadError"
      title="投递加载失败"
      description="网络或服务端异常，可重试"
      :retrying="retrying"
      @retry="handleRetry"
    />
    <UiSkeleton v-else-if="loading" variant="list" :count="4" />
    <div v-else-if="items.length === 0" class="rounded-card border border-line bg-panel p-8 text-center text-ink-3">暂无投递</div>
    <div v-else class="grid gap-3">
      <div
        v-for="item in items"
        :key="String(item.id)"
        class="rounded-card border border-line bg-panel p-4"
        :class="{ 'border-ui-200': !item.employer_viewed_at }"
      >
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <span class="text-sm font-semibold text-ink">{{ item.student_real_name_masked || '匿名学员' }}</span>
              <el-tag v-if="!item.employer_viewed_at" type="warning" size="small">未读</el-tag>
              <el-tag :type="tagType(item.status)" size="small">{{ statusLabel(item.status) }}</el-tag>
            </div>
            <div class="mt-1 text-xs text-ink-3">投递于 {{ item.created_at }}</div>
            <div v-if="resumeUpdated(item)" class="mt-1 text-xs text-orange-500">该候选人自你收到投递后又更新过简历</div>
          </div>
          <div class="flex shrink-0 items-center gap-2">
            <UiButton size="small" @click="openDetail(item)">查看</UiButton>
            <UiButton v-if="item.status === 'applied'" size="small" variant="danger" @click="rejectItem(item)">标记不合适</UiButton>
          </div>
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

    <!-- 投递详情抽屉（#490）：内嵌在线简历 PDF + 明文联系方式（投递即授权）+ 标记不合适 -->
    <el-drawer v-model="detailVisible" title="投递详情" size="480px">
      <div v-if="current" class="flex flex-col gap-3 text-sm">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2">
            <span class="font-semibold text-ink">{{ current.student_real_name_masked || '匿名学员' }}</span>
            <el-tag :type="tagType(current.status)" size="small">{{ statusLabel(current.status) }}</el-tag>
          </div>
        </div>
        <div class="text-xs text-ink-3">投递于 {{ current.created_at }}</div>
        <div v-if="resumeUpdated(current)" class="text-xs text-orange-500">该候选人自你收到投递后又更新过简历</div>

        <!-- 内嵌在线简历 PDF（复用 #485 能力） -->
        <OnlineResumePdf v-if="current.student_user_id" :endpoint="`/api/recruit/resumes/${current.student_user_id}/pdf`" />

        <!-- 明文联系区块（投递即授权，当场可取） -->
        <div v-if="contact" class="rounded-card border border-line bg-panel p-3">
          <div class="mb-1 text-xs font-semibold text-ink-3">联系方式（投递即授权）</div>
          <div><span class="text-ink-3">姓名：</span>{{ contact.real_name }}</div>
          <div><span class="text-ink-3">电话：</span>{{ contact.contact_phone }}</div>
          <div><span class="text-ink-3">微信：</span>{{ contact.wechat }}</div>
          <div v-if="contact.resume_file_url"><span class="text-ink-3">附件：</span><a :href="contact.resume_file_url" target="_blank" class="text-ui-600">下载上传简历</a></div>
        </div>
        <div v-else-if="contactLoading" class="text-xs text-ink-3">联系方式加载中…</div>
        <div v-else-if="contactError" class="text-xs text-red-500">{{ contactError }}</div>

        <!-- 标记不合适（仅 applied） -->
        <UiButton v-if="current.status === 'applied'" variant="danger" :loading="rejecting" @click="rejectItem(current)">标记不合适</UiButton>
      </div>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { jobApi, type JobApplication } from '@/api/job'
import { recruitApi } from '@/api/recruit'
import { useAsyncPage } from '@/composables/useAsyncPage'
import UiButton from '@/components/ui/UiButton.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import OnlineResumePdf from '@/components/recruit/OnlineResumePdf.vue'

const route = useRoute()
const items = ref<JobApplication[]>([])
const jobTitle = ref('')
const unreadCount = ref(0)
const detailVisible = ref(false)
const current = ref<JobApplication | null>(null)
const contact = ref<any>(null)
const contactLoading = ref(false)
const contactError = ref('')
const rejecting = ref(false)

const jobId = Number(route.params.id)

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
  const res = await jobApi.listJobApplications(jobId, { page: page.value, page_size: pageSize.value })
  items.value = res?.items || []
  total.value = res?.total || 0
  unreadCount.value = res?.unread_count || 0
  jobTitle.value = res?.job_title || ''
})

function statusLabel(s: string) {
  const m: Record<string, string> = { applied: '投递中', rejected: '不合适', withdrawn: '已撤回' }
  return m[s] || s
}
function tagType(s: string) {
  if (s === 'applied') return 'warning'
  if (s === 'rejected') return 'danger'
  return 'info'
}

// 漂移提示：投递那一刻的简历更新时间 < 当前简历更新时间
function resumeUpdated(item: JobApplication) {
  if (!item.resume_updated_at_snapshot || !item.student_resume_updated_at) return false
  return item.student_resume_updated_at > item.resume_updated_at_snapshot
}

async function openDetail(item: JobApplication) {
  current.value = item
  contact.value = null
  contactError.value = ''
  detailVisible.value = true
  try {
    const res = await jobApi.getApplicationDetail(item.id)
    // 已读后刷新未读计数
    if (res?.employer_viewed_at && !item.employer_viewed_at) {
      load()
    }
  } catch {}
  // #490：投递即授权 → 直接取明文联系方式
  loadContact()
}

async function loadContact() {
  if (!current.value) return
  contactLoading.value = true
  contactError.value = ''
  try {
    const res = await recruitApi.getContact(current.value.student_user_id)
    contact.value = res as any
  } catch (e: any) {
    contact.value = null
    contactError.value = e?.message || '联系方式不可用'
  } finally {
    contactLoading.value = false
  }
}

async function rejectItem(item: JobApplication) {
  rejecting.value = true
  try {
    await jobApi.rejectApplication(item.id)
    ElMessage.success('已标记为不合适')
    if (current.value) current.value.status = 'rejected'
    load()
  } catch (e: any) {
    ElMessage.error(e?.message || '操作失败')
  } finally {
    rejecting.value = false
  }
}

onMounted(load)
</script>