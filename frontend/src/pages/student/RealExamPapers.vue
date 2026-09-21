<template>
  <div class="mx-auto max-w-[1200px]">
    <div class="mb-[18px] flex flex-wrap items-start justify-between gap-4">
      <div>
        <h2 class="real-exam-title m-0 mb-1.5 text-2xl leading-[1.2] text-ink">真题练习</h2>
        <p class="real-exam-sub m-0 text-[13px] text-ink-3">
          当前证件：<strong>{{ currentCredentialName || '—' }}</strong>
          <span class="real-exam-sub-sep mx-1.5 text-ink-muted">·</span>
          共 {{ papers.length }} 套
        </p>
      </div>
    </div>

    <UiAsyncSection
      :error="loadError"
      :loading="loading"
      :empty="isEmpty"
      :retrying="retrying"
      error-title="真题卷加载失败"
      error-description="网络或服务端异常，可重试"
      @retry="handleRetry"
    >
      <template #skeleton>
        <div class="empty-wrap rounded-card border border-line bg-panel py-6">
          <el-skeleton :rows="4" animated />
        </div>
      </template>
      <template #empty>
        <div class="empty-wrap rounded-card border border-line bg-panel py-6">
          <UiEmptyState :description="emptyDescription" />
        </div>
      </template>
    <div class="variant-c-timeline flex flex-col gap-5">
      <div v-for="[year, list] in grouped" :key="year" class="timeline-year">
        <div class="timeline-year-head mb-2.5 flex items-center gap-2.5">
          <span class="timeline-dot size-2.5 shrink-0 rounded-full bg-ui-500 ring-4 ring-ui-100"></span>
          <span class="timeline-year-label text-base font-bold text-ink">{{ year }}</span>
          <span class="timeline-year-count rounded-pill border border-line bg-canvas px-2 py-0.5 text-xs text-ink-muted">{{ list.length }}套</span>
        </div>
        <div class="timeline-cards ml-5 flex flex-col gap-2.5 border-l border-dashed border-line-strong pl-4">
          <UiCard
            v-for="(p, i) in list"
            :key="p.paper_id"
            class="timeline-card stagger-in transition-colors duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:border-line-strong"
            :style="staggerStyle(i)"
            padding="sm"
          >
            <div class="timeline-card-title mb-2 text-sm font-semibold text-ink">{{ p.title }}</div>
            <div class="timeline-card-meta flex flex-wrap items-center gap-2">
              <span class="meta-text text-xs text-ink-3">{{ p.source || '真题' }} · {{ p.question_count }}题 · {{ p.duration_minutes }}分钟</span>
            </div>
            <div class="timeline-card-actions mt-2.5 flex items-center gap-2.5 border-t border-dashed border-line pt-2.5">
              <template v-if="p.entitled">
                <UiButton variant="primary" plain size="small" @click="startPractice(p)">开始练习</UiButton>
                <UiButton variant="primary" size="small" @click="startExam(p)">模拟考试</UiButton>
              </template>
              <template v-else>
                <span class="meta-text text-xs text-ink-3">{{ p.price }} 积分/套</span>
                <UiButton variant="warning" plain size="small" :loading="redeemingId === p.paper_id" @click="redeem(p)">
                  积分解锁
                </UiButton>
              </template>
            </div>
          </UiCard>
        </div>
      </div>
    </div>
    </UiAsyncSection>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useCredentialStore } from '@/stores/credential'
import { realExamApi, type RealExamPaper } from '@/api/realExam'
import { useAsyncPage } from '@/composables/useAsyncPage'
import { useStagger } from '@/composables/useStagger'
import UiAsyncSection from '@/components/ui/UiAsyncSection.vue'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiCard from '@/components/ui/UiCard.vue'
import { useConfirm } from '@/composables/useConfirm'

const router = useRouter()
const credentialStore = useCredentialStore()

const staggerStyle = useStagger(12)

const papers = ref<RealExamPaper[]>([])
const redeemingId = ref<number | null>(null)

// 三态收编 useAsyncPage（#439）：loader 纯装配，错误收敛 loadError
const {
  loading,
  loadError,
  retrying,
  retry: handleRetry,
  isEmpty,
  run: loadPapers
} = useAsyncPage(async () => {
  papers.value = (await realExamApi.listPapers()) || []
}, { itemsRef: papers })

const currentCredentialName = computed(() => credentialStore.current?.name || '')

const emptyDescription = computed(() =>
  currentCredentialName.value ? `${currentCredentialName.value} 真题建设中，敬请期待` : '真题建设中，敬请期待'
)

// 首屏加载（#605：证件切换即重拉已内聚进 useAsyncPage——列表按 current_credential 分区）
onMounted(loadPapers)

const grouped = computed(() => {
  const map = new Map<string, RealExamPaper[]>()
  for (const p of papers.value) {
    const key = p.year ? `${p.year}` : '年份未知'
    if (!map.has(key)) map.set(key, [])
    map.get(key)!.push(p)
  }
  return [...map.entries()].sort((a, b) => b[0].localeCompare(a[0]))
})

function startPractice(p: RealExamPaper) {
  router.push({ name: 'RealExamPractice', params: { paperId: p.paper_id }, query: { title: p.title } })
}

function startExam(p: RealExamPaper) {
  router.push({ name: 'MockExam', query: { paper: p.paper_id, title: p.title } })
}

async function redeem(p: RealExamPaper) {
  try {
    await useConfirm().confirm(`该套真题需 ${p.price} 积分解锁，确认兑换？`, '积分兑换', {
      confirmButtonText: '确认兑换',
      cancelButtonText: '取消',
      type: 'warning'
    })
  } catch {
    return
  }
  redeemingId.value = p.paper_id
  try {
    await realExamApi.redeemPaper(p.paper_id)
    ElMessage.success('兑换成功，已解锁本卷')
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e)
    ElMessage.error(msg || '兑换失败')
  } finally {
    // 无论成败都重取列表：兑换状态是服务端事实（entitled），不再靠文案猜「其实已解锁」。
    // 旧写法在此判 msg.includes('已兑换') 并弹成功提示——那是 ADR-0024 禁的字符串比对
    // 搬到消费侧，成因是本端点曾把积分族错误一律答成 404（ADR-0062 票9 已改回复用积分域表）。
    await loadPapers()
    redeemingId.value = null
  }
}
</script>
