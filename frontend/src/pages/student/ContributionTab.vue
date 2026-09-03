<script setup lang="ts">
/**
 * 学员投稿 tab（#517）：资料广场 + 「只看我的」私有视图 + 上传投稿抽屉。
 * 挂在 /training/materials 页内作为「学员投稿」tab 内容。
 * 广场仅展示 approved 投稿（后端已过滤），跟随当前证件（页面父级传入）。
 */
import { ref, watch, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Download, UploadFilled, Plus, Document, Warning } from '@element-plus/icons-vue'
import { contributionApi, type ContributionItem, type ContributionStatus } from '@/api/contribution'
import { resolveFileUrl } from '@/utils/fileUrl'
import { formatLocaleDateTime } from '@/utils/format'
import UiButton from '@/components/ui/UiButton.vue'
import UiSegmentTabs from '@/components/ui/UiSegmentTabs.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import { useAsyncPage } from '@/composables/useAsyncPage'
import { useCredentialStore } from '@/stores/credential'

const props = defineProps<{
  credentialId?: number | null
}>()

const credentialStore = useCredentialStore()
const activeView = ref<'plaza' | 'mine'>('plaza')
const sort = ref<'latest' | 'hot'>('latest')
const contributions = ref<ContributionItem[]>([])

/** 我的投稿（全部状态） */
const mine = ref<ContributionItem[]>([])

// 广场三态 + 分页
const {
  loading,
  loadError,
  retrying,
  retry: retryLoad,
  page: currentPage,
  pageSize,
  total,
  run: loadList,
  handlePageChange
} = useAsyncPage(async () => {
  const res = await contributionApi.listPublic({
    credential_id: props.credentialId ?? undefined,
    sort: sort.value,
    page: currentPage.value,
    page_size: pageSize.value
  })
  contributions.value = res.items || []
  total.value = res.total || 0
})

/** 我的投稿加载（轻量、失败静默降级） */
async function loadMine() {
  try {
    const res = await contributionApi.listMine({ page: 1, page_size: 50 })
    mine.value = res.items || []
  } catch (e) {
    console.error('加载我的投稿失败:', e)
  }
}

watch(() => props.credentialId, () => { currentPage.value = 1; loadList() })
watch(sort, () => { currentPage.value = 1; loadList() })

function onViewChange(v: string) {
  activeView.value = v as 'plaza' | 'mine'
  if (v === 'mine' && mine.value.length === 0) loadMine()
}

function effectiveCredentialId(): number | null {
  return props.credentialId ?? credentialStore.current?.id ?? null
}

// ===== 上传投稿 =====
const uploadVisible = ref(false)
const uploadTitle = ref('')
const uploadIntro = ref('')
const uploadAnonymous = ref(false)
const uploadFiles = ref<{ file: File; file_name: string; file_url: string; file_size: number; content_type: string }[]>([])
const uploading = ref(false)
const submitBusy = ref(false)

const FILE_TYPE_HINT = 'pdf/doc/docx/ppt/pptx/xls/xlsx/zip/mp4，单文件 ≤20MB，共 ≤50MB'
const MAX_FILES = 5
const MAX_FILE_SIZE = 20 * 1024 * 1024
const MAX_TOTAL_SIZE = 50 * 1024 * 1024

function extAllowed(name: string): boolean {
  const ext = name.split('.').pop()?.toLowerCase() || ''
  return ['pdf', 'doc', 'docx', 'ppt', 'pptx', 'xls', 'xlsx', 'zip', 'mp4'].includes(ext)
}

function formatSize(bytes?: number): string {
  if (!bytes || bytes <= 0) return '-'
  if (bytes < 1024) return `${bytes}B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)}KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)}MB`
  return `${(bytes / 1024 / 1024 / 1024).toFixed(1)}GB`
}

/** 选择文件（逐个先传） */
async function onPickFiles(e: Event) {
  const input = e.target as HTMLInputElement
  const files = input.files ? Array.from(input.files) : []
  input.value = ''
  if (files.length === 0) return
  const room = MAX_FILES - uploadFiles.value.length
  if (files.length > room) {
    ElMessage.warning(`一份投稿最多 ${MAX_FILES} 个文件，还能加 ${room} 个`)
    return
  }
  for (const f of files) {
    if (!extAllowed(f.name)) {
      ElMessage.warning(`${f.name} 格式不支持（${FILE_TYPE_HINT}）`)
      continue
    }
    if (f.size > MAX_FILE_SIZE) {
      ElMessage.warning(`${f.name} 超过单文件 20MB 限制`)
      continue
    }
    const totalSoFar = uploadFiles.value.reduce((a, x) => a + x.file_size, 0)
    if (totalSoFar + f.size > MAX_TOTAL_SIZE) {
      ElMessage.warning('投稿文件合计不能超过 50MB')
      continue
    }
    uploading.value = true
    try {
      const data = await contributionApi.uploadFile(f)
      uploadFiles.value.push({ file: f, ...data })
    } catch (e: any) {
      ElMessage.error(e?.message || '文件上传失败，请重试')
    } finally {
      uploading.value = false
    }
  }
}

function removeUploadFile(idx: number) {
  uploadFiles.value.splice(idx, 1)
}

async function submitContribution() {
  const credId = effectiveCredentialId()
  if (!credId) {
    ElMessage.warning('请先在侧栏选定目标证件再投稿')
    return
  }
  if (!uploadTitle.value.trim()) {
    ElMessage.warning('请填写标题')
    return
  }
  if (!uploadIntro.value.trim()) {
    ElMessage.warning('请填写简介')
    return
  }
  if (uploadFiles.value.length === 0) {
    ElMessage.warning('请至少上传 1 个文件')
    return
  }
  submitBusy.value = true
  try {
    await contributionApi.create({
      credential_id: credId,
      title: uploadTitle.value.trim(),
      intro: uploadIntro.value.trim(),
      is_anonymous: uploadAnonymous.value,
      files: uploadFiles.value.map((f) => ({
        file_url: f.file_url,
        file_name: f.file_name,
        file_size: f.file_size,
        content_type: f.content_type
      }))
    })
    ElMessage.success('投稿已提交，审核通过后将公开并奖励积分')
    uploadVisible.value = false
    uploadTitle.value = ''
    uploadIntro.value = ''
    uploadAnonymous.value = false
    uploadFiles.value = []
    activeView.value = 'mine'
    await loadMine()
  } catch (e: any) {
    ElMessage.error(e?.message || '投稿提交失败')
  } finally {
    submitBusy.value = false
  }
}

/** 撤回 pending 稿 */
async function onWithdraw(item: ContributionItem) {
  try {
    await ElMessageBox.confirm(`撤回后该稿将从审核队列移除，确定撤回「${item.title}」？`, '撤回投稿', {
      type: 'warning',
      confirmButtonText: '撤回',
      cancelButtonText: '再想想'
    })
  } catch {
    return
  }
  try {
    await contributionApi.withdraw(item.id)
    ElMessage.success('已撤回')
    await loadMine()
  } catch (e: any) {
    ElMessage.error(e?.message || '撤回失败')
  }
}

/** 下载（触发后端计数） */
async function onDownload(item: ContributionItem) {
  if (!item.files || item.files.length === 0) return
  try {
    // 先打计数端点（幂等），再打开文件
    await contributionApi.download(item.id)
  } catch {
    /* 计数失败不阻断下载（下载本身走静态直链） */
  }
  window.open(resolveFileUrl(item.files[0].file_url), '_blank')
}

const REPORT_REASONS = [
  { label: '盗版', value: 'piracy' },
  { label: '内容错误', value: 'content_error' },
  { label: '违规', value: 'violation' },
  { label: '已失效', value: 'stale' }
]

// 举报（四理由枚举单选；同一投稿重复举报后端合并）
const reportVisible = ref(false)
const reportReason = ref('piracy')
const reportTarget = ref<ContributionItem | null>(null)
const reportSubmitting = ref(false)

function onReport(item: ContributionItem) {
  reportTarget.value = item
  reportReason.value = 'piracy'
  reportVisible.value = true
}

async function submitReport() {
  if (!reportTarget.value) return
  reportSubmitting.value = true
  try {
    await contributionApi.report(reportTarget.value.id, reportReason.value)
    ElMessage.success('举报已提交，平台将尽快处理')
    reportVisible.value = false
  } catch (e: any) {
    ElMessage.error(e?.message || '举报提交失败')
  } finally {
    reportSubmitting.value = false
  }
}

const STATUS_LABEL: Record<ContributionStatus, string> = {
  pending: '审核中',
  approved: '已上架',
  rejected: '已驳回',
  withdrawn: '已撤回',
  archived: '已下架'
}

const STATUS_CLASS: Record<ContributionStatus, string> = {
  pending: 'bg-warning-light text-warning',
  approved: 'bg-success-light text-success',
  rejected: 'bg-danger-light text-danger',
  withdrawn: 'bg-canvas text-ink-3',
  archived: 'bg-canvas text-ink-3'
}

onMounted(() => {
  loadList()
  if (activeView.value === 'mine') loadMine()
})

defineExpose({ loadMine })
</script>

<template>
  <div>
    <!-- 顶栏：视角切换 + 广场操作 -->
    <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
      <UiSegmentTabs
        :model-value="activeView"
        :options="[{ label: '广场', value: 'plaza' }, { label: '我的投稿', value: 'mine' }]"
        @update:model-value="onViewChange"
      />
      <UiButton v-if="activeView === 'plaza'" variant="primary" size="small" @click="uploadVisible = true">
        <el-icon><Plus /></el-icon>
        上传资料
      </UiButton>
    </div>

    <!-- 广场（approved 投稿） -->
    <template v-if="activeView === 'plaza'">
      <div class="mb-3 flex items-center gap-3">
        <UiSegmentTabs
          :model-value="sort"
          :options="[{ label: '最新', value: 'latest' }, { label: '最热', value: 'hot' }]"
          @update:model-value="(v: string) => { sort = v as 'latest' | 'hot' }"
        />
      </div>
      <div class="min-h-[200px] rounded-card bg-panel shadow-card">
        <UiErrorState v-if="loadError" title="投稿加载失败" description="网络或服务端异常，可重试" :retrying="retrying" @retry="retryLoad" />
        <UiSkeleton v-else-if="loading" variant="list" :count="4" />
        <template v-else-if="contributions.length > 0">
          <div v-for="item in contributions" :key="item.id"
            class="border-b border-line px-5 py-4 last:border-b-0">
            <div class="flex items-start gap-3">
              <div class="flex size-11 shrink-0 items-center justify-center rounded-[8px] bg-primary-50 text-primary-500">
                <el-icon :size="20"><Document /></el-icon>
              </div>
              <div class="min-w-0 flex-1">
                <div class="flex items-center gap-2">
                  <span class="truncate text-[15px] font-medium text-ink">{{ item.title }}</span>
                </div>
                <p class="mt-0.5 line-clamp-2 text-xs text-ink-3">{{ item.intro }}</p>
                <div class="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-ink-3">
                  <span>{{ item.author?.anonymous ? '匿名学员' : item.author?.username || '—' }}</span>
                  <span>{{ item.downloads_count }} 次下载</span>
                  <span>{{ formatLocaleDateTime(item.created_at) }}</span>
                  <span v-if="item.files?.length">{{ item.files!.length }} 个文件</span>
                </div>
              </div>
              <div class="flex shrink-0 flex-col items-end gap-1.5">
                <UiButton v-if="item.files?.length" variant="primary" link size="small" @click="onDownload(item)">
                  <el-icon><Download /></el-icon>
                  下载
                </UiButton>
                <UiButton link size="small" @click="onReport(item)">
                  <el-icon><Warning /></el-icon>
                  举报
                </UiButton>
              </div>
            </div>
          </div>
        </template>
        <UiEmptyState v-else description="暂无学员投稿，快来上传第一份" />
      </div>
      <div v-if="total > pageSize" class="mt-4 flex justify-center">
        <el-pagination v-model:current-page="currentPage" :page-size="pageSize" :total="total"
          layout="total, prev, pager, next" @current-change="handlePageChange" />
      </div>
    </template>

    <!-- 我的投稿 -->
    <template v-else>
      <div class="min-h-[200px] rounded-card bg-panel shadow-card">
        <template v-if="mine.length > 0">
          <div v-for="item in mine" :key="item.id" class="border-b border-line px-5 py-3.5 last:border-b-0">
            <div class="flex items-center gap-3">
              <div class="min-w-0 flex-1">
                <div class="flex items-center gap-2">
                  <span class="truncate text-[15px] font-medium text-ink">{{ item.title }}</span>
                  <span class="shrink-0 rounded-full px-2 py-0.5 text-[11px]" :class="STATUS_CLASS[item.status]">{{ STATUS_LABEL[item.status] }}</span>
                </div>
                <div v-if="item.reject_reason" class="mt-1 text-xs text-danger">驳回/下架原因：{{ item.reject_reason }}</div>
                <div class="mt-1 text-xs text-ink-3">{{ formatLocaleDateTime(item.created_at) }} · {{ item.downloads_count }} 次下载</div>
              </div>
              <div class="flex shrink-0 items-center gap-2">
                <UiButton v-if="item.status === 'pending'" size="small" @click="onWithdraw(item)">撤回</UiButton>
              </div>
            </div>
          </div>
        </template>
        <UiEmptyState v-else description="还没有投稿，传一份资料赚积分吧" />
      </div>
    </template>

    <!-- 上传投稿抽屉 -->
    <el-drawer v-model="uploadVisible" title="上传资料" size="420px" append-to-body>
      <div class="flex flex-col gap-4">
        <div>
          <label class="mb-1 block text-sm text-ink-2">标题 <span class="text-danger">*</span></label>
          <el-input v-model="uploadTitle" maxlength="120" placeholder="如：叉车液压系统常见故障排查手册" show-word-limit />
        </div>
        <div>
          <label class="mb-1 block text-sm text-ink-2">简介 <span class="text-danger">*</span></label>
          <el-input v-model="uploadIntro" type="textarea" :rows="3" maxlength="2000" placeholder="这份资料讲了什么、适合谁、整理自哪里" show-word-limit />
        </div>
        <div>
          <label class="mb-1 block text-sm text-ink-2">文件（1–5 个，{{ FILE_TYPE_HINT }}）</label>
          <label class="flex cursor-pointer flex-col items-center justify-center gap-1 rounded-card border border-dashed border-line py-5 text-ink-3 transition-colors hover:border-primary-300 hover:text-primary-500">
            <el-icon :size="22" class="mb-1"><UploadFilled /></el-icon>
            <span class="text-xs">点击选择文件{{ uploading ? '（上传中…）' : '' }}</span>
            <input type="file" multiple class="hidden" :disabled="uploading || uploadFiles.length >= MAX_FILES" @change="onPickFiles" />
          </label>
          <div v-if="uploadFiles.length > 0" class="mt-2 flex flex-col gap-1.5">
            <div v-for="(f, idx) in uploadFiles" :key="f.file_url + idx" class="flex items-center justify-between rounded-lg bg-canvas px-3 py-2 text-xs">
              <span class="truncate text-ink">{{ f.file_name }} <span class="text-ink-3">({{ formatSize(f.file_size) }})</span></span>
              <button type="button" class="text-ink-3 hover:text-danger" @click="removeUploadFile(idx)">移除</button>
            </div>
          </div>
        </div>
        <div class="flex items-center gap-2">
          <el-switch v-model="uploadAnonymous" />
          <span class="text-xs text-ink-2">匿名投稿（公开后显示「匿名学员」，积分照常发放）</span>
        </div>
        <div class="mt-2 flex justify-end gap-2">
          <UiButton size="small" @click="uploadVisible = false">取消</UiButton>
          <UiButton variant="primary" size="small" :loading="submitBusy" @click="submitContribution">提交审核</UiButton>
        </div>
      </div>
    </el-drawer>
    <!-- 举报对话框（四理由） -->
    <UiDialog v-model="reportVisible" title="举报投稿" width="440px" :confirm-text="'提交举报'" :confirm-loading="reportSubmitting" @confirm="submitReport">
      <div class="flex flex-col gap-2">
        <p v-if="reportTarget" class="mb-1 text-xs text-ink-3">举报《{{ reportTarget.title }}》</p>
        <el-radio-group v-model="reportReason" class="flex flex-col items-start gap-2">
          <el-radio v-for="r in REPORT_REASONS" :key="r.value" :value="r.value">{{ r.label }}</el-radio>
        </el-radio-group>
      </div>
    </UiDialog>
  </div>
</template>