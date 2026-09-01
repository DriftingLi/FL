<template>
  <div class="flex flex-col gap-4">
    <div class="flex items-center gap-2">
      <router-link to="/training/jobs" class="text-sm text-ui-600 hover:text-ui-700">← 返回职位广场</router-link>
    </div>

    <UiErrorState
      v-if="loadError"
      title="职位加载失败"
      description="网络或服务端异常，可重试"
      :retrying="retrying"
      @retry="handleRetry"
    />
    <UiSkeleton v-else-if="loading" variant="list" :count="4" />
    <div v-else-if="!data" class="rounded-card border border-line bg-panel p-8 text-center text-ink-3">职位不存在或已下架</div>
    <div v-else class="rounded-card border border-line bg-panel p-6">
      <h1 class="text-lg font-bold text-ink">{{ data.title }}</h1>
      <div class="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-ink-3">
        <span v-if="data.specialty_name">{{ data.specialty_name }}</span>
        <span v-if="data.region">{{ data.region }}</span>
        <span v-if="data.salary_text">{{ data.salary_text }}</span>
        <span v-if="data.experience_req">经验：{{ data.experience_req }}</span>
        <span>发布于 {{ data.published_at.slice(0, 10) }}</span>
      </div>

      <div class="mt-4 rounded border border-line bg-panel p-3 text-sm">
        <div class="text-xs font-semibold text-ink-3">企业信息</div>
        <div class="mt-1 text-ink">企业：{{ data.company_name || '-' }}</div>
        <div class="text-ink">主营：{{ data.business_scope || '-' }}</div>
        <div class="text-ink">联系人：{{ data.contact_name || '-' }}</div>
      </div>

      <div class="mt-4 text-sm text-ink-2 whitespace-pre-wrap">{{ data.description || '-' }}</div>

      <div class="mt-6 flex items-center gap-2">
        <UiButton variant="primary" :loading="applying" @click="showApplyDialog = true">投递</UiButton>
        <UiButton variant="danger" plain :loading="reporting" @click="showReportDialog = true">举报</UiButton>
        <p v-if="applyError" class="mt-2 text-xs text-red-500">{{ applyError }}</p>
      </div>
    </div>

    <!-- 投递确认弹窗（spec #449 决定 1 的 UI 落点）：明确告知「投递即授权…与简历是否公开无关」 -->
    <el-dialog v-model="showApplyDialog" title="确认投递" width="440px">
      <div class="text-sm text-ink">
        投递即授权该企业查看你的联系方式，与简历是否公开无关。投递后企业可直接与你联系。
      </div>
      <template #footer>
        <UiButton @click="showApplyDialog = false">取消</UiButton>
        <UiButton variant="primary" :loading="applying" @click="confirmApply">确认投递</UiButton>
      </template>
    </el-dialog>

    <!-- 举报弹窗 -->
    <el-dialog v-model="showReportDialog" title="举报职位" width="440px">
      <el-input v-model="reportReason" type="textarea" :rows="3" maxlength="500" show-word-limit placeholder="请填写举报原因（如：交培训费包分配、冒用别家公司名义）" />
      <template #footer>
        <UiButton @click="showReportDialog = false">取消</UiButton>
        <UiButton variant="danger" :loading="reporting" @click="confirmReport">提交举报</UiButton>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { jobApi, type JobPosting } from '@/api/job'
import { useAsyncPage } from '@/composables/useAsyncPage'
import UiButton from '@/components/ui/UiButton.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'

const route = useRoute()
const data = ref<JobPosting | null>(null)
const showApplyDialog = ref(false)
const applying = ref(false)
const applyError = ref('')
const showReportDialog = ref(false)
const reporting = ref(false)
const reportReason = ref('')

const {
  loading,
  loadError,
  retrying,
  retry: handleRetry,
  run: load
} = useAsyncPage(async () => {
  const id = Number(route.params.id)
  const res = await jobApi.getPublicJob(id)
  data.value = (res as any) || null
})


async function confirmReport() {
  if (!data.value) return
  if (!reportReason.value.trim()) {
    ElMessage.warning('举报原因不能为空')
    return
  }
  reporting.value = true
  try {
    await jobApi.reportJob(data.value.id, reportReason.value.trim())
    ElMessage.success('举报已提交，感谢你的反馈')
    showReportDialog.value = false
    reportReason.value = ''
  } catch (e: any) {
    ElMessage.error(e?.message || '举报提交失败')
  } finally {
    reporting.value = false
  }
}

async function confirmApply() {
  if (!data.value) return
  applying.value = true
  applyError.value = ''
  try {
    await jobApi.applyJob(data.value.id)
    ElMessage.success('投递成功，企业已可查看你的联系方式')
    showApplyDialog.value = false
  } catch (e: any) {
    applyError.value = e?.message || '投递失败'
  } finally {
    applying.value = false
  }
}

onMounted(load)
</script>