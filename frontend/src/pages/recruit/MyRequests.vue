<template>
  <div class="flex flex-col gap-4">
    <h1 class="text-xl font-bold text-ink">我的申请</h1>
    <UiAsyncSection
      :error="loadError"
      :loading="loading"
      :empty="isEmpty"
      :retrying="retrying"
      error-title="申请记录加载失败"
      error-description="网络或服务端异常，可重试"
      @retry="handleRetry"
    >
      <template #skeleton>
        <UiSkeleton variant="list" :count="4"  />
      </template>

      <div class="grid gap-3">
      <div v-for="item in items" :key="String(item.id)" class="rounded-card border border-line bg-panel p-4">
        <div class="flex items-center justify-between">
          <div class="text-sm text-ink">学员 ID：{{ item.student_user_id }}</div>
          <UiTag :tone="describeContactRequest(item.status).tone" size="small">{{ describeContactRequest(item.status).label }}</UiTag>
        </div>
        <div class="mt-2 text-xs text-ink-3">附言：{{ item.message }}</div>
        <div class="mt-1 text-xs text-ink-3">申请时间：{{ item.created_at }}</div>
      </div>
    </div>
      <template #empty>
        <UiEmptyState description="暂无申请记录"  />
      </template>
    </UiAsyncSection>
    <div v-if="total > 0" class="text-xs text-ink-3 text-center">共 {{ total }} 条</div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { recruitApi, type RecruitContactRequest } from '@/api/recruit'
import { describeContactRequest } from '@/utils/contactRequestStatus'
import { useAsyncPage } from '@/composables/useAsyncPage'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiAsyncSection from '@/components/ui/UiAsyncSection.vue'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiTag from '@/components/ui/UiTag.vue'

const items = ref<RecruitContactRequest[]>([])

// 三态收编 useAsyncPage（#439）：loader 纯装配，错误收敛 loadError
const {
  loading,
  loadError,
  retrying,
  isEmpty,
  retry: handleRetry,
  total,
  run: load
} = useAsyncPage(async () => {
  const res = await recruitApi.listMyRequests({ page: 1, page_size: 20 })
  items.value = res?.items || []
  total.value = res?.total || 0
}, { itemsRef: items })

onMounted(load)
</script>
