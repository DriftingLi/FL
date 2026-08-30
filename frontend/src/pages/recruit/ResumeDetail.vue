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
      <h1 class="text-lg font-bold text-ink">{{ data.real_name || '学员简历' }}</h1>
      <p class="mt-1 text-sm text-ink-3">用户 ID：{{ data.user_id }}</p>
      <div class="mt-4 grid gap-2 text-sm">
        <div><span class="text-ink-3">地区：</span><span class="text-ink">{{ data.region || '-' }}</span></div>
        <div><span class="text-ink-3">期望岗位：</span><span class="text-ink">{{ data.expected_specialty_extra || '-' }}</span></div>
        <div><span class="text-ink-3">工作经验：</span><span class="text-ink">{{ data.experience_years ?? '-' }} 年</span></div>
      </div>
      <p class="mt-4 text-xs text-ink-3">详细简历信息将随后续工单逐步开放。</p>
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
    const res = await recruitApi.listResumes({ page: 1, page_size: 100 })
    const id = String(route.params.id)
    data.value = (res?.items || []).find((i) => String(i.user_id) === id) || null
    if (!data.value) {
      // 若列表为空或未命中，视为未找到（占位后端当前返回空列表）
      data.value = null
    }
  } catch (e: any) {
    error.value = e?.message || '加载失败'
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>
