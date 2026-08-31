<template>
  <div class="flex flex-col gap-6 p-4">
    <h1 class="text-xl font-bold text-ink">巡检视图</h1>
    <div class="rounded-card border border-line bg-panel p-4">
      <div class="text-sm text-ink-3">删除已解决帖计数</div>
      <div class="mt-1 text-2xl font-bold text-ink">{{ deletedCount }}</div>
      <div class="mt-1 text-xs text-ink-3">楼主删除自己已解决的帖子时累加，不自动惩罚、不回滚积分</div>
    </div>
    <div class="rounded-card border border-line bg-panel p-4">
      <div class="flex items-center gap-2 mb-3">
        <span class="text-sm font-semibold text-ink">问答积分流水</span>
        <el-select v-model="reason" placeholder="按原因筛选" clearable class="!w-44" @change="loadLedger">
          <el-option label="答主被采纳" value="accepted_bonus" />
          <el-option label="楼主采纳" value="accept_action" />
          <el-option label="回收" value="rollback" />
        </el-select>
        <el-button size="small" @click="loadLedger">刷新</el-button>
      </div>
      <div v-if="ledgerLoading" class="text-sm text-ink-3">加载中...</div>
      <div v-else-if="ledger.length === 0" class="text-sm text-ink-3">暂无数据</div>
      <div v-else class="grid gap-2">
        <div v-for="item in ledger" :key="String(item.id)" class="border border-line rounded p-2 text-xs">
          <div>用户 {{ item.user_id }} · {{ item.reason }} · {{ item.delta }} 分 · 帖 {{ item.ref_id }}</div>
          <div class="text-ink-3">{{ item.created_at }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { unwrappedRequest } from '@/api/request'

const deletedCount = ref(0)
const reason = ref('')
const ledger = ref<any[]>([])
const ledgerLoading = ref(false)

async function loadCount() {
  try {
    const res: any = await unwrappedRequest.get('/admin/inspection/deleted-after-accepted')
    deletedCount.value = res?.count ?? 0
  } catch {}
}
async function loadLedger() {
  ledgerLoading.value = true
  try {
    const params: any = {}
    if (reason.value) params.reason = reason.value
    const res: any = await unwrappedRequest.get('/admin/points/ledger', { params })
    ledger.value = res?.items || []
  } catch {}
  ledgerLoading.value = false
}
onMounted(() => {
  loadCount()
  loadLedger()
})
</script>
