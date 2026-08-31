<template>
<div class="flex flex-col gap-6 p-4">
<h1 class="text-xl font-bold text-ink">{{ deletedCount }}</h1>
<div class="rounded-card border border-line bg-panel p-4">
<div class="text-sm text-ink-3">删除已解决帖计数</div>
</div>
<div class="rounded-card border border-line bg-panel p-4">
<div class="flex items-center gap-2 mb-3">
<span class="text-sm font-semibold text-ink">问答积分流水</span>
<el-select v-model="domain" class="!w-44">
<el-option label="问答域" value="forum_topic" />
<el-option label="跨业务域全量" value="" />
</el-select>
<el-select v-model="reason" placeholder="按原因筛选" clearable class="!w-40" @change="loadLedger">
<el-option label="答主被采纳" value="accepted_bonus" />
<el-option label="楼主采纳" value="accept_action" />
<el-option label="违规回收" value="rollback" />
</el-select>
<el-input v-model="userId" placeholder="按用户ID过滤" clearable class="!w-40" @change="loadLedger" />
<el-button size="small" @click="loadLedger">刷新</el-button>
</div>
<div v-if="ledgerLoading" class="text-sm text-ink-3">加载中...</div>
<div v-else-if="ledger.length === 0" class="text-sm text-ink-3">暂无数据</div>
<div v-else class="grid gap-2">
<div v-for="item in ledger" :key="String(item.id)" class="border border-line rounded p-2 text-xs">
<div>用户 {{ item.user_id }} · {{ item.reason }} · {{ item.delta }} 分 · {{ refLabel(item.ref_type) }} {{ item.ref_id }}</div>
<div class="text-ink-3">{{ item.created_at }}</div>
</div>
</div>
<div class="mt-3 flex justify-end">
<el-pagination
v-model:current-page="page"
v-model:page-size="pageSize"
:total="total"
:page-sizes="[10, 20, 50]"
layout="total, sizes, prev, pager, next"
@current-change="loadLedger"
@size-change="loadLedger"
/>
</div>
</div>
</div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { unwrappedRequest } from '@/api/request'

// #411：默认锁定问答域（forum_topic），显式切换才跨域全量——卡片标题与内容同域。
const domain = ref<'forum_topic' | ''>('forum_topic')
const reason = ref('')
const userId = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const ledger = ref<LedgerItem[]>([])
const ledgerLoading = ref(false)

interface LedgerItem {
  id: number
  user_id: number
  reason: string
  delta: number
  ref_type: string
  ref_id: string
  created_at: string
}

// 行内引用按业务域渲染量词（#411）：非问答行不再统一显示「帖 …」。
const refQuantifier: Record<string, string> = {
  forum_topic: '帖',
  task: '任务',
  course: '课程',
  shop: '商品',
  real_exam_paper: '商品',
  ai_chat: 'AI 对话',
  admin: '罚分',
  rollback: '回收',
}
function refLabel(refType: string): string {
  return refQuantifier[refType] || '引用'
}

const deletedCount = ref(0)
async function loadCount() {
  try {
    const res: any = await unwrappedRequest.get('/admin/inspection/deleted-after-accepted', { headers: { 'X-Silent': '1' } })
    deletedCount.value = res?.count ?? 0
  } catch {}
}
async function loadLedger() {
  ledgerLoading.value = true
  try {
    const params: Record<string, any> = { page: page.value, page_size: pageSize.value }
    if (domain.value) params.ref_type = domain.value
    if (reason.value) params.reason = reason.value
    if (userId.value) params.user_id = userId.value
    const res: any = await unwrappedRequest.get('/admin/points/ledger', { params, headers: { 'X-Silent': '1' } })
    ledger.value = res?.items || []
    total.value = res?.total ?? 0
  } catch {}
  ledgerLoading.value = false
}
// 切换域即时重载（#411）：v-model 变更即刷新，不依赖下拉的 change 事件时序。
watch(domain, () => loadLedger())
onMounted(() => {
  loadCount()
  loadLedger()
})
</script>