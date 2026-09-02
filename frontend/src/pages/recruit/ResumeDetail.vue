<template>
  <div class="flex flex-col gap-4">
    <div class="flex items-center gap-2">
      <router-link to="/recruit/resumes" class="text-sm text-ui-600 hover:text-ui-700">← 返回简历库</router-link>
    </div>

    <UiErrorState
      v-if="loadError"
      title="简历加载失败"
      description="网络或服务端异常，可重试"
      :retrying="retrying"
      @retry="handleRetry"
    />
    <UiSkeleton v-else-if="loading" variant="list" :count="4" />
    <div v-else-if="!data" class="rounded-card border border-line bg-panel p-8 text-center text-ink-3">未找到该简历</div>
    <div v-else class="rounded-card border border-line bg-panel p-6">
      <h1 class="text-lg font-bold text-ink">{{ data.real_name || data.real_name_masked || '学员简历' }}</h1>
      <p class="mt-1 text-sm text-ink-3">更新于 {{ data.updated_at }}</p>

      <!-- 在线简历 PDF 内嵌（#485：未授权即可预览打码版） -->
      <div class="mt-4">
        <OnlineResumePdf :endpoint="`/api/recruit/resumes/${data.user_id}/pdf`" error-text="简历 PDF 加载失败" />
      </div>

      <div class="mt-6">
        <!-- #489：按钮状态机 none/pending/approved -->
        <template v-if="!contact">
          <UiButton v-if="data.contact_state !== 'pending'" variant="primary" :loading="contactLoading" @click="showDialog = true">申请交换联系方式</UiButton>
          <UiButton v-else disabled>等待学员处理中</UiButton>
          <p v-if="contactError" class="mt-2 text-xs text-red-500">{{ contactError }}</p>
          <p v-else-if="data.contact_state === 'pending'" class="mt-2 text-xs text-ink-3">申请已提交，学员处理后即可查看联系方式</p>
        </template>
        <div v-if="contact" class="mt-3 rounded border border-line bg-panel p-3 text-sm">
          <div class="mb-2 text-xs font-semibold text-ink-3">已授权联系信息</div>
          <div><span class="text-ink-3">姓名：</span>{{ contact.real_name }}</div>
          <div><span class="text-ink-3">电话：</span>{{ contact.contact_phone }}</div>
          <div><span class="text-ink-3">微信：</span>{{ contact.wechat }}</div>
          <div v-if="contact.resume_file_url"><span class="text-ink-3">PDF：</span><a :href="contact.resume_file_url" target="_blank" class="text-ui-600">查看 PDF</a></div>
          <!-- #489：授权后补齐证书原图与工作照 -->
          <div v-if="contactPhotos.length" class="mt-2"><span class="text-ink-3">工作照：</span>
            <div class="mt-1 flex flex-wrap gap-2">
              <img v-for="(p, i) in contactPhotos" :key="i" :src="p" class="h-16 w-16 rounded object-cover" />
            </div>
          </div>
          <div v-if="certImages.length" class="mt-2"><span class="text-ink-3">证书原图：</span>
            <div class="mt-1 flex flex-wrap gap-2">
              <img v-for="(c, i) in certImages" :key="i" :src="c" class="h-16 w-16 rounded object-cover" />
            </div>
          </div>
        </div>
      </div>
      <el-dialog v-model="showDialog" title="申请交换联系方式" width="420px">
        <el-input v-model="message" type="textarea" :rows="3" maxlength="200" show-word-limit placeholder="请填写申请附言（1-200字）" />
        <template #footer>
          <UiButton @click="showDialog = false">取消</UiButton>
          <UiButton variant="primary" :loading="submitting" @click="submitRequest">提交申请</UiButton>
        </template>
      </el-dialog>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { recruitApi, type RecruitResumeItem } from '@/api/recruit'
import { useAsyncPage } from '@/composables/useAsyncPage'
import UiButton from '@/components/ui/UiButton.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import OnlineResumePdf from '@/components/recruit/OnlineResumePdf.vue'

const route = useRoute()
const data = ref<RecruitResumeItem | null>(null)
const showDialog = ref(false)
const message = ref('')
const submitting = ref(false)
const contact = ref<any>(null)
const contactLoading = ref(false)
const contactError = ref('')

const {
  loading,
  loadError,
  retrying,
  retry: handleRetry,
  run: load
} = useAsyncPage(async () => {
  const id = String(route.params.id)
  try {
    const res = await recruitApi.getResume(id)
    data.value = res as any || null
    loadContact()
    startApprovedPolling()
  } catch (e: any) {
    if (e?.response?.status === 404 || String(e?.message || '').includes('不存在')) {
      data.value = null
    } else {
      throw e
    }
  }
})

// #489：授权后透出的工作照与证书原图
const contactPhotos = computed(() => {
  try {
    return (contact.value?.photos || []).filter(Boolean)
  } catch { return [] }
})
const certImages = computed(() => {
  try {
    const certs = contact.value?.resume_certifications || []
    const imgs: string[] = []
    for (const c of certs) {
      if (Array.isArray(c.image_urls)) imgs.push(...c.image_urls.filter(Boolean))
    }
    return imgs
  } catch { return [] }
})

async function loadContact() {
  if (!data.value) return
  contactLoading.value = true
  contactError.value = ''
  try {
    const res = await recruitApi.getContact(data.value.user_id)
    contact.value = res as any
  } catch (e: any) {
    contact.value = null
    if (e?.response?.status === 403 || String(e?.message || '').includes('无有效授权')) {
      contactError.value = ''
    } else {
      contactError.value = e?.message || ''
    }
  } finally {
    contactLoading.value = false
  }
}

// #489：学员同意后无需手动刷新——pending/approved 时轮询明文（10s 间隔，pending 等待同意最长 60 次）
let pollTimer: any = null
function startApprovedPolling() {
  stopApprovedPolling()
  const st = data.value?.contact_state
  if (st !== 'approved' && st !== 'pending') return
  const maxTries = st === 'pending' ? 60 : 6
  let count = 0
  pollTimer = setInterval(async () => {
    count++
    if (count > maxTries) {
      stopApprovedPolling()
      return
    }
    try {
      const res = await recruitApi.getContact(data.value!.user_id)
      if (res) {
        contact.value = res as any
        stopApprovedPolling()
      }
    } catch (e: any) {
      // 学员撤回授权 → 403：明文立即消失并停止轮询（撤回实时生效，Standards 审查）
      if (e?.response?.status === 403 || String(e?.message || '').includes('无有效授权')) {
        contact.value = null
        stopApprovedPolling()
      }
    }
  }, 10000)
}
function stopApprovedPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

async function submitRequest() {
  if (!message.value.trim()) {
    ElMessage.warning('附言不能为空')
    return
  }
  if (message.value.length > 200) {
    ElMessage.warning('附言不能超过200字')
    return
  }
  if (!data.value) return
  submitting.value = true
  try {
    await recruitApi.createContactRequest({ student_user_id: data.value.user_id, message: message.value })
    ElMessage.success('申请已提交，等待学员处理')
    showDialog.value = false
    message.value = ''
  } catch (e: any) {
    ElMessage.error(e?.message || '提交失败')
  } finally {
    submitting.value = false
  }
}

onMounted(load)
onBeforeUnmount(stopApprovedPolling)
</script>
