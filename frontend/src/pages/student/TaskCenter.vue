<template>
  <div class="mx-auto max-w-[960px] px-4 pb-10">
    <div class="mb-4 flex items-center justify-between">
      <h1 class="m-0 text-2xl font-semibold text-ink">任务中心</h1>
    </div>

    <div class="mb-4 flex flex-wrap items-center gap-4 rounded-card border border-line bg-panel px-4 py-3.5 shadow-card">
      <div class="flex min-w-[90px] flex-col gap-0.5">
        <span class="text-[11px] tracking-[0.04em] text-ink-3">当前积分</span>
        <span class="font-heading text-xl font-extrabold text-ink">{{ points.balance }}</span>
      </div>
      <div class="h-8 w-px bg-line"></div>
      <div class="flex min-w-[90px] flex-col gap-0.5">
        <span class="text-[11px] tracking-[0.04em] text-ink-3">今日可得</span>
        <span class="font-heading text-base font-extrabold text-ink">+{{ todayEarnable }}</span>
      </div>
      <div class="h-8 w-px bg-line"></div>
      <div class="flex min-w-[90px] flex-col gap-0.5">
        <span class="text-[11px] tracking-[0.04em] text-ink-3">累计获得</span>
        <span class="font-heading text-base font-extrabold text-ink">{{ points.totalEarned }}</span>
      </div>
      <RouterLink
        to="/training/task-center/points"
        class="ml-auto inline-flex items-center gap-1 rounded-pill border border-line-strong bg-panel px-3 py-1.5 text-xs font-semibold text-ink-2 transition-colors duration-150 hover:border-ui-300 hover:text-ui-600"
      >
        积分明细
        <el-icon><ArrowRight /></el-icon>
      </RouterLink>
    </div>

    <UiErrorState
      v-if="loadError"
      title="任务加载失败"
      description="网络或服务端异常，可重试"
      :retrying="retrying"
      @retry="retryLoad"
    />

    <UiSkeleton v-else-if="loading" variant="card" :count="6" />

    <div v-else class="flex flex-col gap-5">
      <div
        v-for="(group, gi) in grouped"
        :key="group.key"
        class="stagger-in"
        :style="staggerStyle(gi)"
      >
        <div class="mb-2.5 flex items-center gap-2">
          <span class="text-[15px] font-bold text-ink">{{ group.label }}</span>
          <span class="text-xs text-ink-3">{{ group.desc }}</span>
          <span class="ml-auto rounded-pill border border-line bg-canvas px-2 py-0.5 text-xs text-ink-3">{{ group.tasks.length }}项</span>
        </div>
        <div class="flex flex-col gap-2.5">
          <UiCard
            v-for="task in group.tasks"
            :key="task.code"
            padding="base"
            class="flex items-center justify-between gap-3"
            :class="[task.status, { 'opacity-70 !bg-canvas': task.status === 'claimed' }]"
          >
            <div class="flex min-w-0 flex-1 items-start gap-3">
              <div
                class="flex size-9 shrink-0 items-center justify-center rounded-full text-lg"
                :class="iconTone(task.status)"
              >
                <el-icon v-if="task.status === 'claimed'"><CircleCheckFilled /></el-icon>
                <el-icon v-else-if="task.status === 'claimable'"><Trophy /></el-icon>
                <el-icon v-else><List /></el-icon>
              </div>
              <div class="min-w-0 flex-1">
                <div class="mb-0.5 text-sm font-semibold text-ink">{{ task.title }}</div>
                <div class="text-xs leading-[1.4] text-ink-3">{{ task.desc }}</div>
                <div v-if="task.total && task.total > 1" class="mt-1.5 flex items-center gap-2">
                  <UiProgress
                    :value="Math.round(((task.progress || 0) / task.total) * 100)"
                    size="sm"
                    tone="brand"
                    class="w-[90px] shrink-0"
                  />
                  <span class="font-mono text-[11px] text-ink-3">{{ task.progress }}/{{ task.total }}</span>
                </div>
              </div>
            </div>
            <div class="flex shrink-0 items-center gap-2.5">
              <span class="min-w-9 text-right text-sm font-bold text-warn-strong">+{{ task.points }}</span>
              <UiButton variant="primary" v-if="task.status === 'claimable'" size="small" :loading="claimingCode === task.code" @click="handleClaim(task.code)">
                领取
              </UiButton>
              <UiButton variant="ghost" v-else-if="task.status === 'todo'" size="small" disabled>去完成</UiButton>
              <el-tag v-else type="success" size="small" effect="plain">已领取</el-tag>
            </div>
          </UiCard>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { CircleCheckFilled, Trophy, List, ArrowRight } from '@element-plus/icons-vue'
import { pointsApi, type PointsTaskItem } from '@/api/points'
import { groupLabelMap, groupDescMap, claimDupMessage, isClaimExhausted } from '@/utils/taskCenter'
import type { TaskGroup } from '@/utils/taskCenter'
import { useAsyncPage } from '@/composables/useAsyncPage'
import { useStagger } from '@/composables/useStagger'
import UiProgress from '@/components/ui/UiProgress.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiCard from '@/components/ui/UiCard.vue'

const staggerStyle = useStagger(6)

/** 任务图标圆底的状态色调（todo 中性 / claimable 琥珀 / claimed 绿） */
const iconTone = (status: string) =>
  status === 'todo'
    ? 'border border-line bg-canvas text-ink-3'
    : status === 'claimable'
      ? 'border border-warn-soft bg-warn-soft text-warn-strong'
      : 'border border-ok-soft bg-ok-soft text-ok-strong'

const tasks = ref<PointsTaskItem[]>([])
const points = ref({ balance: 0, totalEarned: 0 })
const claimingCode = ref<string | null>(null)

// 三态收编（#388）：加载失败进错误态可重试（原「静默回退本地占位」退役，
// 占位数据不再是后端异常的正确呈现）
const { loading, loadError, retrying, retry: retryLoad, run: refresh } = useAsyncPage(async () => {
  const [bal, ts] = await Promise.all([pointsApi.getBalance(), pointsApi.getTasks()])
  points.value = { balance: bal.balance, totalEarned: bal.total_earned }
  tasks.value = ts.tasks || []
})

const todayEarnable = computed(() =>
  tasks.value.filter((t) => t.status !== 'claimed').reduce((sum, t) => sum + t.points, 0),
)

const grouped = computed(() => {
  const order: TaskGroup[] = ['daily', 'newbie', 'growth']
  return order.map((key) => ({
    key,
    label: groupLabelMap[key],
    desc: groupDescMap[key],
    tasks: tasks.value.filter((t) => t.group === key),
  }))
})

async function handleClaim(code: string) {
  const task = tasks.value.find((t) => t.code === code)
  if (!task || task.status !== 'claimable') return
  claimingCode.value = code
  try {
    const res = await pointsApi.claim(code)
    task.status = 'claimed'
    task.progress = task.total
    points.value.balance = res.balance
    points.value.totalEarned = res.total_earned
    // 强制刷新以确保与服务端一致
    tasks.value = [...tasks.value]
    ElMessage.success(`已领取 +${task.points} 积分`)
    // 后台静默刷新，确保幂等状态持久
    void refresh()
  } catch (e: unknown) {
    // 幂等错误按「客户端错误分类 + 任务分组」语义分级（#409）：不再匹配后端中文字串。
    // 领取请求已走静默通道（X-Silent），这里只弹一次提示，拦截器不叠加 toast。
    const kind = (e as { kind?: string }).kind
    const msg = e instanceof Error ? e.message : String(e)
    if (isClaimExhausted(task.group, kind, msg)) {
      task.status = 'claimed'
      task.progress = task.total
      tasks.value = [...tasks.value]
      ElMessage.warning(claimDupMessage(task.group))
    } else {
      ElMessage.error(msg || '领取失败')
    }
  } finally {
    claimingCode.value = null
  }
}

onMounted(() => {
  refresh()
})
</script>
