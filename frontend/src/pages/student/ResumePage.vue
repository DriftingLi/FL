<template>
  <div class="mx-auto max-w-[720px] p-4">
    <UiCard padding="lg" class="resume-card">
      <div class="mb-4 flex items-center justify-between">
        <UiButton variant="text" @click="goBack">返回</UiButton>
        <div class="flex items-center gap-2">
          <span class="text-[13px] text-ink-2">公开给招聘方</span>
          <el-switch :model-value="visibilityOpen" @change="toggleVisibility" />
        </div>
      </div>
      <div v-if="viewCount > 0" class="rounded-card border border-line bg-ui-50 px-4 py-3 mb-4 text-sm text-ink">
        近 7 天 {{ viewCount }} 家企业查看过你的简历
      </div>

      <UiEmptyState
        v-if="resumeMissing"
        title="简历尚未创建"
        description="完善后可被招聘企业看到"
        action-text="去填写"
        @action="goEdit"
      />
      <UiErrorState
        v-else-if="resumeError"
        title="简历加载失败"
        description="网络或服务端异常，可重试"
        :retrying="false"
        @retry="load()"
      />
      <template v-else>
        <!-- 操作区（#491）：编辑简历 / PDF 附件上传·更换·删除 -->
        <div class="mb-4 flex flex-wrap items-center gap-2">
          <UiButton variant="primary" size="small" @click="goEdit">编辑简历</UiButton>
          <UiButton size="small" @click="triggerPdf">
            {{ resumeFileUrl ? '更换 PDF 附件' : '上传 PDF 附件' }}
          </UiButton>
          <UiButton v-if="resumeFileUrl" size="small" variant="danger" plain :loading="deletingPdf" @click="deletePdf">删除 PDF 附件</UiButton>
          <input ref="pdfInput" type="file" accept=".pdf" hidden @change="onPdfChange" />
          <span v-if="resumeFileUrl" class="text-xs text-ink-3">附件：<a :href="resumeFileUrl" target="_blank" class="text-ui-600">查看上传简历</a></span>
        </div>

        <!-- 内嵌自己的打码在线简历 PDF（所见即招聘者所见，#485/#491） -->
        <OnlineResumePdf endpoint="/api/resume/pdf" error-text="简历 PDF 加载失败" />

        <!-- 自己的明文联系区块（仅本人可见，#491） -->
        <div class="mt-4 rounded-card border border-line bg-panel p-3 text-sm">
          <div class="mb-1 text-xs font-semibold text-ink-3">我的联系方式（企业授权后可见）</div>
          <div class="grid gap-1 text-ink">
            <div><span class="text-ink-3 mr-2">姓名</span>{{ realName || '-' }}</div>
            <div><span class="text-ink-3 mr-2">电话</span>{{ contactPhone || '-' }}</div>
            <div><span class="text-ink-3 mr-2">微信</span>{{ wechat || '-' }}</div>
          </div>
        </div>

        <!-- 收到的交换申请面板（#491 迁移至此） -->
        <div v-if="contactRequests.length > 0" class="mt-4 rounded-card border border-line bg-panel p-4">
          <div class="text-sm font-semibold text-ink mb-2">收到的简历查看申请</div>
          <div v-for="req in contactRequests" :key="req.id" class="border-t border-line py-2">
            <div class="flex items-center justify-between gap-2">
              <div class="text-sm text-ink">
                <div>{{ req.company_name }} · {{ req.contact_name }}</div>
                <div class="text-xs text-ink-3">附言：{{ req.message }}</div>
                <div class="text-xs text-ink-3">{{ statusLabel(req.status) }} · {{ req.created_at }}</div>
              </div>
              <div class="flex gap-1">
                <UiButton variant="primary" v-if="req.status === 'pending'" size="small" @click="approveReq(req.id)">同意</UiButton>
                <UiButton v-if="req.status === 'pending'" size="small" @click="rejectReq(req.id)">拒绝</UiButton>
                <UiButton variant="danger" v-if="req.status === 'approved'" size="small" @click="revokeReq(req.id)">撤回</UiButton>
              </div>
            </div>
            <!-- #487：已同意条目就地展开企业联系方式 -->
            <CompanyContactInfo
              v-if="req.status === 'approved'"
              :approved="true"
              :company-name="req.company_name"
              :contact-name="req.contact_name"
              :phone="req.contact_phone"
              :email="req.contact_email"
              :wechat="req.wechat"
            />
          </div>
        </div>
      </template>
    </UiCard>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { resumeApi } from '@/api/resume'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiCard from '@/components/ui/UiCard.vue'
import OnlineResumePdf from '@/components/recruit/OnlineResumePdf.vue'
import CompanyContactInfo from '@/components/recruit/CompanyContactInfo.vue'

const router = useRouter()
const pdfInput = ref<HTMLInputElement | null>(null)
const deletingPdf = ref(false)
const resumeMissing = ref(false)
const resumeError = ref(false)
const viewCount = ref(0)
const visibilityOpen = ref(false)
const contactRequests = ref<any[]>([])
const realName = ref('')
const contactPhone = ref('')
const wechat = ref('')
const resumeFileUrl = ref('')

function goBack() {
  router.push({ name: 'StudentProfile' })
}
function goEdit() {
  router.push({ name: 'StudentResumeEdit' })
}

function statusLabel(s: string) {
  const m: Record<string, string> = { pending: '待处理', approved: '已同意', rejected: '已拒绝', revoked: '已撤回', expired: '已过期' }
  return m[s] || s
}

function triggerPdf() {
  pdfInput.value?.click()
}

async function onPdfChange(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  if (file.type !== 'application/pdf' && !file.name.toLowerCase().endsWith('.pdf')) {
    ElMessage.error('仅支持 PDF 文件')
    input.value = ''
    return
  }
  if (file.size > 50 * 1024 * 1024) {
    ElMessage.error('文件大小超出限制，最大允许50MB')
    input.value = ''
    return
  }
  const fd = new FormData()
  fd.append('file', file)
  try {
    const res: any = await resumeApi.uploadPdf(fd)
    resumeFileUrl.value = res.url || res.data?.url || ''
    ElMessage.success('上传成功')
    load()
  } catch {}
  input.value = ''
}

async function deletePdf() {
  // 删除不可恢复，先确认（Standards 审查）
  try {
    await ElMessageBox.confirm('确定删除已上传的 PDF 附件吗？此操作不可恢复。', '删除附件', { type: 'warning' })
  } catch {
    return
  }
  deletingPdf.value = true
  try {
    await resumeApi.deletePdf()
    resumeFileUrl.value = ''
    ElMessage.success('附件已删除')
  } catch {} finally {
    deletingPdf.value = false
  }
}

async function toggleVisibility(val: boolean) {
  const vis = val ? 'open' : 'hidden'
  try {
    await resumeApi.updateVisibility(vis as any)
    visibilityOpen.value = val
    ElMessage.success(val ? '已公开' : '已设为不公开')
  } catch {
    visibilityOpen.value = !val
  }
}

async function load() {
  resumeMissing.value = false
  resumeError.value = false
  try {
    const data: any = await resumeApi.get()
    if (!data) return
    realName.value = data.real_name || ''
    contactPhone.value = data.contact_phone || ''
    wechat.value = data.wechat || ''
    resumeFileUrl.value = data.resume_file_url || ''
    visibilityOpen.value = data.visibility === 'open'
  } catch (e) {
    const kind = (e as { kind?: string }).kind
    if (kind === 'notfound') {
      resumeMissing.value = true
    } else {
      resumeError.value = true
    }
  }
  try {
    const stats: any = await resumeApi.getViewStats()
    viewCount.value = stats?.count || 0
  } catch {}
}

async function loadContactRequests() {
  try {
    const res: any = await resumeApi.listContactRequests({ page: 1, page_size: 20 })
    contactRequests.value = res?.items || []
  } catch {}
}
async function approveReq(id: number) {
  try { await resumeApi.approveContactRequest(id); ElMessage.success('已同意'); loadContactRequests() } catch (e: any) { ElMessage.error(e?.message || '操作失败') }
}
async function rejectReq(id: number) {
  try { await resumeApi.rejectContactRequest(id); ElMessage.success('已拒绝'); loadContactRequests() } catch (e: any) { ElMessage.error(e?.message || '操作失败') }
}
async function revokeReq(id: number) {
  try { await resumeApi.revokeContactRequest(id); ElMessage.success('已撤回'); loadContactRequests() } catch (e: any) { ElMessage.error((e as any)?.message || '操作失败') }
}

onMounted(() => {
  load()
  loadContactRequests()
})
</script>
