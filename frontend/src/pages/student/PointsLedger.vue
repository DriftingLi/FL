<template>
  <div class="mx-auto max-w-[960px] px-4 pb-10">
    <div class="mb-4 flex items-center gap-1 text-sm">
      <RouterLink to="/training/task-center" class="text-ink-3 transition-colors hover:text-ui-600">任务中心</RouterLink>
      <span class="text-ink-3">/</span>
      <h1 class="m-0 text-xl font-semibold text-ink">积分明细</h1>
    </div>

    <UiErrorState
      v-if="loadError"
      title="明细加载失败"
      description="网络或服务端异常，可重试"
      :retrying="retrying"
      @retry="retryLoad"
    />

    <UiSkeleton v-else-if="loading" variant="card" :count="3" />

    <template v-else>
      <!-- 账户四格卡 -->
      <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <div class="rounded-card border border-line bg-panel p-4">
          <div class="text-xs text-ink-3">当前积分</div>
          <div class="mt-1 font-heading text-2xl font-extrabold tabular-nums text-ink">{{ balance.balance }}</div>
        </div>
        <div class="rounded-card border border-line bg-panel p-4">
          <div class="text-xs text-ink-3">累计收入</div>
          <div class="mt-1 font-heading text-xl font-extrabold tabular-nums text-ok-strong">+{{ balance.total_earned }}</div>
        </div>
        <div class="rounded-card border border-line bg-panel p-4">
          <div class="text-xs text-ink-3">累计支出</div>
          <div class="mt-1 font-heading text-xl font-extrabold tabular-nums text-bad-strong">−{{ balance.total_spent }}</div>
        </div>
        <div class="rounded-card border border-line bg-panel p-4">
          <div class="text-xs text-ink-3">积分有效期</div>
          <div class="mt-1 inline-flex items-center gap-1.5 rounded-pill bg-ok-soft px-2.5 py-1 text-xs font-semibold text-ok-strong">
            <el-icon><CircleCheckFilled /></el-icon>当前永久有效
          </div>
        </div>
      </div>

      <!-- 筛选 + 规则抽屉开关 -->
      <div class="mt-5 flex flex-wrap items-center gap-3">
        <UiSegmentTabs
          :model-value="filter"
          :options="filterOptions"
          @update:model-value="(v: string) => { filter = v as 'all' | 'in' | 'out'; handlePageChange() }"
        />
        <UiButton variant="ghost" size="small" class="ml-auto" :icon="QuestionFilled" @click="rulesVisible = true">积分规则</UiButton>
      </div>

      <!-- 流水列表 -->
      <div class="mt-3 overflow-hidden rounded-card border border-line bg-panel">
        <UiEmptyState v-if="!ledger.items.length" description="暂无积分流水" size="sm" />
        <template v-else>
          <div
            v-for="item in ledger.items"
            :key="item.id"
            class="flex items-center gap-3 border-b border-line px-4 py-3 last:border-b-0"
          >
            <div
              class="flex size-8 shrink-0 items-center justify-center rounded-lg text-sm font-bold"
              :class="deltaKind(item.delta) === 'in' ? 'bg-ok-soft text-ok-strong' : 'bg-bad-soft text-bad-strong'"
            >
              {{ deltaKind(item.delta) === 'in' ? '＋' : '－' }}
            </div>
            <div class="min-w-0 flex-1">
              <div class="text-sm font-medium text-ink">{{ ledgerReasonLabel(item.reason, item.delta) }}</div>
              <div class="mt-0.5 text-xs text-ink-3">{{ formatDateTime(item.created_at) }}</div>
            </div>
            <div class="hidden text-xs text-ink-3 sm:block">
              过期<span class="ml-1 tabular-nums">{{ item.expires_at ? formatDateTime(item.expires_at) : '—' }}</span>
            </div>
            <div
              class="min-w-[70px] text-right text-sm font-bold tabular-nums"
              :class="deltaKind(item.delta) === 'in' ? 'text-ok-strong' : 'text-bad-strong'"
            >
              {{ item.delta > 0 ? '+' : '' }}{{ item.delta }}
            </div>
          </div>
          <div class="flex items-center justify-between px-4 py-3">
            <span class="text-xs text-ink-3">共 {{ ledger.total }} 条</span>
            <el-pagination
              v-model:current-page="page"
              :page-size="pageSize"
              :total="total"
              layout="prev, pager, next"
              small
              background
              @current-change="handlePageChange"
            />
          </div>
        </template>
      </div>
    </template>

    <!-- 积分规则抽屉 -->
    <el-drawer v-model="rulesVisible" title="积分规则" size="360px" append-to-body>
      <div class="flex flex-col gap-5 px-1">
        <section>
          <h4 class="mb-1.5 text-sm font-semibold text-ink">如何获得</h4>
          <p class="text-xs leading-relaxed text-ink-3">每日任务（签到、刷题、学习时长）当日 0 点重置、当日达成当日领；新手任务一次性领取；成长任务每日达成每日领。</p>
        </section>
        <section>
          <h4 class="mb-1.5 text-sm font-semibold text-ink">如何消耗</h4>
          <p class="text-xs leading-relaxed text-ink-3">积分商城兑换课程与真题卷、AI 学习助手按 token 计费扣减；兑换失败自动退回。</p>
        </section>
        <section>
          <h4 class="mb-1.5 text-sm font-semibold text-ink">上传资料赚分</h4>
          <p class="text-xs leading-relaxed text-ink-3">学员投稿通过审核 +50；下载量达 10/50/200 次分别追加 +30/+80/+200。匿名投稿不影响积分。投稿违规下架将按平台规则回收该稿已发奖励（明细中「系统退回」可追溯）。</p>
        </section>
        <section>
          <h4 class="mb-1.5 text-sm font-semibold text-ink">违规扣减</h4>
          <p class="text-xs leading-relaxed text-ink-3">论坛违规内容与违规投稿经管理员核实后扣减，可在明细中追溯。</p>
        </section>
        <section>
          <h4 class="mb-1.5 text-sm font-semibold text-ink">有效期</h4>
          <p class="text-xs leading-relaxed text-ink-3">本期积分永久有效，不设过期。若后续启用过期策略：按先进先出（FIFO）扣减，到期前 30 天站内信提醒。</p>
        </section>
      </div>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { CircleCheckFilled, QuestionFilled } from '@element-plus/icons-vue'
import { pointsApi, type PointsBalance, type PointsLedgerData } from '@/api/points'
import { ledgerReasonMeta, deltaKind } from '@/utils/pointsReason'
import { formatDateTime } from '@/utils/format'
import { useAsyncPage } from '@/composables/useAsyncPage'
import UiSegmentTabs from '@/components/ui/UiSegmentTabs.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'

const balance = ref<PointsBalance>({ balance: 0, total_earned: 0, total_spent: 0 })
const ledger = ref<PointsLedgerData>({ items: [], total: 0, page: 1, pages: 0 })
const filter = ref<'all' | 'in' | 'out'>('all')
const rulesVisible = ref(false)

const filterOptions = [
  { label: '全部', value: 'all' },
  { label: '收入', value: 'in' },
  { label: '支出', value: 'out' }
]

// 三态 + 分页（#388 模式）
const {
  loading,
  loadError,
  retrying,
  retry: retryLoad,
  page,
  pageSize,
  total,
  run: refresh,
  handlePageChange
} = useAsyncPage(async () => {
  const [bal, ledgerRes] = await Promise.all([
    pointsApi.getBalance(),
    // #512：收支方向由后端分页过滤（direction 透传），前端不跨页漏项
    pointsApi.getLedger({ page: page.value, page_size: pageSize.value, direction: filter.value === 'all' ? undefined : filter.value })
  ])
  balance.value = { ...balance.value, ...bal }
  ledger.value = ledgerRes
  total.value = ledgerRes.total || 0
})

function ledgerReasonLabel(reason: string, delta: number): string {
  // 未收录 reason 按 delta 方向给默认文案，label 兜底原文
  const meta = ledgerReasonMeta(reason)
  if (meta.label !== reason) return meta.label
  return deltaKind(delta) === 'in' ? '积分获得' : '积分消耗'
}

onMounted(refresh)
</script>
