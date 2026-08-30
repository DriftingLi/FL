<template>
  <div class="flex flex-col gap-4">
    <div class="flex items-center justify-between">
      <h1 class="text-xl font-bold text-ink">简历库</h1>
      <el-button size="small" :loading="loading" @click="load">刷新</el-button>
    </div>

    <div v-if="loading" class="rounded-card border border-line bg-panel p-8 text-center text-ink-3">加载中...</div>
    <div v-else-if="error" class="rounded-card border border-line bg-panel p-8 text-center">
      <p class="text-sm text-ink-2">{{ error }}</p>
      <el-button class="mt-3" size="small" @click="load">重试</el-button>
    </div>
    <div v-else-if="items.length === 0" class="rounded-card border border-line bg-panel p-8 text-center text-ink-3">
      暂无公开简历
    </div>
    <div v-else class="grid gap-3">
      <div
        v-for="item in items"
        :key="String(item.user_id)"
        class="rounded-card border border-line bg-panel p-4 hover:border-ui-200 transition-colors"
      >
        <div class="flex items-start justify-between gap-3">
          <div>
            <div class="text-sm font-semibold text-ink">{{ item.real_name || '匿名学员' }}</div>
            <div class="mt-1 text-xs text-ink-3">
              <span v-if="item.region">{{ item.region }}</span>
              <span v-if="item.expected_specialty_extra"> · {{ item.expected_specialty_extra }}</span>
              <span v-if="item.experience_years != null"> · {{ item.experience_years }}年经验</span>
            </div>
          </div>
          <router-link
            :to="`/recruit/resumes/${item.user_id}`"
            class="text-xs font-medium text-ui-600 hover:text-ui-700"
          >
            查看详情
          </router-link>
        </div>
      </div>
    </div>

    <div v-if="total > 0" class="text-xs text-ink-3 text-center">共 {{ total }} 份</div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { recruitApi, type RecruitResumeItem } from '@/api/recruit'

const loading = ref(false)
const error = ref('')
const items = ref<RecruitResumeItem[]>([])
const total = ref(0)

async function load() {
  loading.value = true
  error.value = ''
  try {
    const res = await recruitApi.listResumes({ page: 1, page_size: 20 })
    items.value = res?.items || []
    total.value = res?.total || 0
  } catch (e: any) {
    error.value = e?.message || '加载失败'
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>
