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
        <el-button type="primary" disabled>申请交换联系方式</el-button>
        <p class="mt-2 text-xs text-ink-3">联系方式交换将在下一票开放，本票为禁用态占位。</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { recruitApi, type RecruitResumeItem } from '@/api/recruit'

const route = useRoute()
const loading = ref(false)
const error = ref('')
const data = ref<RecruitResumeItem | null>(null)

async function load() {
  loading.value = true
  error.value = ''
  try {
    const id = String(route.params.id)
    const res = await recruitApi.getResume(id)
    data.value = res as any || null
  } catch (e: any) {
    // 404 等
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

onMounted(load)
</script>
