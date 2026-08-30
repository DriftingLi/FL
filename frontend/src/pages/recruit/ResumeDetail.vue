<template>
  <div class="flex flex-col gap-4">
    <div class="flex items-center gap-2">
      <router-link to="/recruit/resumes" class="text-sm text-ui-600 hover:text-ui-700">← 返回简历库</router-link>
    </div>

    <div v-if="loading" class="rounded-card border border-line bg-panel p-8 text-center text-ink-3">加载中...</div>
    <div v-else-if="error" class="rounded-card border border-line bg-panel p-8 text-center">
      <p class="text-sm text-ink-2">{{ error }}</p>
      <el-button class="mt-3" size="small" @click="load">重试</el-button>
    </div>
    <div v-else-if="!data" class="rounded-card border border-line bg-panel p-8 text-center text-ink-3">未找到该简历</div>
    <div v-else class="rounded-card border border-line bg-panel p-6">
      <h1 class="text-lg font-bold text-ink">{{ data.real_name || data.real_name_masked || '学员简历' }}</h1>
      <p class="mt-1 text-sm text-ink-3">用户 ID：{{ data.user_id }} · 更新于 {{ data.updated_at }}</p>
      <div class="mt-4 grid gap-3 text-sm">
        <div class="flex gap-2"><span class="w-20 shrink-0 text-ink-3">期望岗位</span><span class="text-ink">{{ data.expected_specialty_extra || '-' }}</span></div>
        <div class="flex gap-2"><span class="w-20 shrink-0 text-ink-3">意向地区</span><span class="text-ink">{{ (data.expected_regions as any)?.join('、') || '-' }}</span></div>
        <div class="flex gap-2"><span class="w-20 shrink-0 text-ink-3">薪资</span><span class="text-ink">{{ data.salary_negotiable ? '面议' : `${data.salary_min ?? '-'} - ${data.salary_max ?? '-'}` }}</span></div>
        <div class="flex gap-2"><span class="w-20 shrink-0 text-ink-3">经验</span><span class="text-ink">{{ data.experience_years }} 年</span></div>
        <div class="flex gap-2"><span class="w-20 shrink-0 text-ink-3">到岗时间</span><span class="text-ink">{{ data.available_in || '-' }}</span></div>
        <div class="flex gap-2"><span class="w-20 shrink-0 text-ink-3">用工性质</span><span class="text-ink">{{ data.job_nature || '-' }}</span></div>
        <div class="flex gap-2"><span class="w-20 shrink-0 text-ink-3">自我介绍</span><span class="text-ink">{{ data.self_intro || '-' }}</span></div>
        <div class="flex gap-2"><span class="w-20 shrink-0 text-ink-3">工作经历</span>
          <ul v-if="data.resume_experiences && data.resume_experiences.length" class="flex-1 list-disc pl-4">
            <li v-for="(exp, i) in data.resume_experiences" :key="i" class="text-ink">{{ exp.company }} · {{ exp.role }}（{{ exp.start_month }} ~ {{ exp.end_month }}） - {{ exp.desc }}</li>
          </ul>
          <span v-else class="text-ink">-</span>
        </div>
        <div class="flex gap-2"><span class="w-20 shrink-0 text-ink-3">持证</span>
          <div v-if="data.resume_certifications && data.resume_certifications.length" class="flex flex-wrap gap-1">
            <span v-for="(c, i) in data.resume_certifications" :key="i" class="rounded bg-ui-50 px-1.5 py-0.5 text-xs text-ink-3">{{ c.credential_id ? `证件#${c.credential_id} ${c.cert_no || ''}` : (c.cert_no || '持证') }}</span>
          </div>
          <span v-else class="text-ink">-</span>
        </div>
      </div>
      <div class="mt-6">
        <el-button type="primary" :loading="contactLoading" @click="showDialog = true">申请交换联系方式</el-button>
        <el-button v-if="contact" size="small" class="ml-2" @click="loadContact">刷新联系方式</el-button>
        <p v-if="contactError" class="mt-2 text-xs text-red-500">{{ contactError }}</p>
        <div v-if="contact" class="mt-3 rounded border border-line bg-panel p-3 text-sm">
          <div><span class="text-ink-3">姓名：</span>{{ contact.real_name }}</div>
          <div><span class="text-ink-3">电话：</span>{{ contact.contact_phone }}</div>
          <div><span class="text-ink-3">微信：</span>{{ contact.wechat }}</div>
          <div v-if="contact.resume_file_url"><span class="text-ink-3">PDF：</span><a :href="contact.resume_file_url" target="_blank" class="text-ui-600">查看 PDF</a></div>
        </div>
      </div>
      <el-dialog v-model="showDialog" title="申请交换联系方式" width="420px">
        <el-input v-model="message" type="textarea" :rows="3" maxlength="200" show-word-limit placeholder="请填写申请附言（1-200字）" />
        <template #footer>
          <el-button @click="showDialog = false">取消</el-button>
          <el-button type="primary" :loading="submitting" @click="submitRequest">提交申请</el-button>
        </template>
      </el-dialog>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { recruitApi, type RecruitResumeItem } from '@/api/recruit'

const route = useRoute()
const loading = ref(false)
const error = ref('')
const data = ref<RecruitResumeItem | null>(null)
const showDialog = ref(false)
const message = ref('')
const submitting = ref(false)
const contact = ref<any>(null)
const contactLoading = ref(false)
const contactError = ref('')

async function load() {
  loading.value = true
  error.value = ''
  try {
    const id = String(route.params.id)
    const res = await recruitApi.getResume(id)
    data.value = res as any || null
    loadContact()
  } catch (e: any) {
    if (e?.response?.status === 404 || String(e?.message || '').includes('不存在')) {
      data.value = null
      error.value = ''
    } else {
      error.value = e?.message || '加载失败'
    }
  } finally {
    loading.value = false
  }
}

async function loadContact() {
  if (!data.value) return
  contactLoading.value = true
  contactError.value = ''
  try {
    const res = await recruitApi.getContact(data.value.user_id)
    contact.value = res as any
  } catch (e: any) {
    contact.value = null
    // 403 等表示无授权，不显示错误
    if (e?.response?.status === 403 || String(e?.message || '').includes('无有效授权')) {
      contactError.value = ''
    } else {
      contactError.value = e?.message || ''
    }
  } finally {
    contactLoading.value = false
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
</script>
