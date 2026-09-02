<template>
  <div class="flex flex-col gap-2">
    <!-- 加载骨架 -->
    <div v-if="loading" class="flex h-[420px] flex-col items-center justify-center gap-3 rounded-card border border-line bg-ui-50 text-sm text-ink-3">
      <UiLoading size="lg" />
      <span>简历加载中…</span>
    </div>

    <!-- 失败错误态（可重试） -->
    <div v-else-if="error" class="rounded-card border border-line bg-panel p-6 text-center">
      <div class="text-sm text-ink-2">{{ errorText || '简历加载失败' }}</div>
      <UiButton class="mt-3" variant="primary" size="small" @click="loadPdf">重试</UiButton>
    </div>

    <!-- PDF 内嵌 -->
    <iframe
      v-else-if="srcUrl"
      :src="srcUrl"
      class="h-[420px] w-full rounded-card border border-line bg-white"
      title="在线简历"
    ></iframe>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onBeforeUnmount } from 'vue'
import { getValidAccessToken } from '@/api/client'
import UiButton from '@/components/ui/UiButton.vue'
import UiLoading from '@/components/ui/UiLoading.vue'

const props = withDefaults(
  defineProps<{
    /** PDF 端点路径（带鉴权头请求） */
    endpoint: string
    /** 加载失败文案 */
    errorText?: string
  }>(),
  {}
)

const srcUrl = ref('')
const loading = ref(true)
const error = ref(false)
let objectUrl: string | null = null

function revoke() {
  if (objectUrl) {
    URL.revokeObjectURL(objectUrl)
    objectUrl = null
  }
  srcUrl.value = ''
}

async function loadPdf() {
  revoke()
  loading.value = true
  error.value = false
  try {
    const token = await getValidAccessToken()
    if (!token) throw new Error('未登录')
    const res = await fetch(props.endpoint, {
      headers: { Authorization: `Bearer ${token}` }
    })
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    const blob = await res.blob()
    objectUrl = URL.createObjectURL(blob)
    srcUrl.value = objectUrl
  } catch {
    error.value = true
  } finally {
    loading.value = false
  }
}

watch(() => props.endpoint, loadPdf, { immediate: true })
onBeforeUnmount(revoke)
</script>
