<template>
  <div class="flex flex-col gap-4">
    <h1 class="text-xl font-bold text-ink">我的申请</h1>
    <div v-if="loading" class="rounded-card border border-line bg-panel p-8 text-center text-ink-3">加载中...</div>
    <div v-else-if="items.length === 0" class="rounded-card border border-line bg-panel p-8 text-center text-ink-3">暂无申请记录</div>
    <div v-else class="grid gap-3">
      <div v-for="item in items" :key="String(item.id)" class="rounded-card border border-line bg-panel p-4">
        <div class="flex items-center justify-between">
          <div class="text-sm text-ink">学员 ID：{{ item.student_user_id }}</div>
          <el-tag :type="tagType(item.status)" size="small">{{ statusLabel(item.status) }}</el-tag>
        </div>
        <div class="mt-2 text-xs text-ink-3">附言：{{ item.message }}</div>
        <div class="mt-1 text-xs text-ink-3">申请时间：{{ item.created_at }}</div>
      </div>
    </div>
    <div v-if="total > 0" class="text-xs text-ink-3 text-center">共 {{ total }} 条</div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { recruitApi } from '@/api/recruit'

const loading = ref(false)
const items = ref<any[]>([])
const total = ref(0)

function statusLabel(s: string) {
  const m: Record<string, string> = { pending: '待处理', approved: '已同意', rejected: '已拒绝', expired: '已过期', revoked: '已撤回' }
  return m[s] || s
}
function tagType(s: string) {
  if (s === 'approved') return 'success'
  if (s === 'rejected' || s === 'revoked') return 'danger'
  if (s === 'expired') return 'info'
  return ''
}

async function load() {
  loading.value = true
  try {
    const res: any = await recruitApi.listMyRequests({ page: 1, page_size: 20 })
    items.value = res?.items || []
    total.value = res?.total || 0
  } catch {}
  loading.value = false
}
onMounted(load)
</script>
