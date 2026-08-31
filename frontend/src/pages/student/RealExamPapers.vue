<template>
  <div class="real-exam-papers">
    <div class="real-exam-header">
      <div>
        <h2 class="real-exam-title">真题练习</h2>
        <p class="real-exam-sub">
          当前证件：<strong>{{ currentCredentialName || '—' }}</strong>
          <span class="real-exam-sub-sep">·</span>
          共 {{ papers.length }} 套
        </p>
      </div>
    </div>

    <div v-if="loading" class="empty-wrap">
      <el-skeleton :rows="4" animated />
    </div>
    <div v-else-if="papers.length === 0" class="empty-wrap">
      <UiEmptyState :description="emptyDescription" />
    </div>
    <div v-else class="variant-c-timeline">
      <div v-for="[year, list] in grouped" :key="year" class="timeline-year">
        <div class="timeline-year-head">
          <span class="timeline-dot"></span>
          <span class="timeline-year-label">{{ year }}</span>
          <span class="timeline-year-count">{{ list.length }}套</span>
        </div>
        <div class="timeline-cards">
          <UiCard
            v-for="(p, i) in list"
            :key="p.paper_id"
            class="timeline-card stagger-in"
            :style="staggerStyle(i)"
            padding="sm"
          >
            <div class="timeline-card-title">{{ p.title }}</div>
            <div class="timeline-card-meta">
              <span class="meta-text">{{ p.source || '真题' }} · {{ p.question_count }}题 · {{ p.duration_minutes }}分钟</span>
            </div>
            <div class="timeline-card-actions">
              <template v-if="p.entitled">
                <UiButton variant="primary" plain size="small" @click="startPractice(p)">开始练习</UiButton>
                <UiButton variant="primary" size="small" @click="startExam(p)">模拟考试</UiButton>
              </template>
              <template v-else>
                <span class="meta-text">{{ p.price }} 积分/套</span>
                <UiButton variant="warning" plain size="small" :loading="redeemingId === p.paper_id" @click="redeem(p)">
                  积分解锁
                </UiButton>
              </template>
            </div>
          </UiCard>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useCredentialStore } from '@/stores/credential'
import { realExamApi, type RealExamPaper } from '@/api/realExam'
import { useCredentialRefetch } from '@/composables/useCredentialRefetch'
import { useStagger } from '@/composables/useStagger'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiCard from '@/components/ui/UiCard.vue'

const router = useRouter()
const credentialStore = useCredentialStore()

const staggerStyle = useStagger(12)

const papers = ref<RealExamPaper[]>([])
const loading = ref(true)
const redeemingId = ref<number | null>(null)

const currentCredentialName = computed(() => credentialStore.current?.name || '')

const emptyDescription = computed(() =>
  currentCredentialName.value ? `${currentCredentialName.value} 真题建设中，敬请期待` : '真题建设中，敬请期待'
)

async function loadPapers() {
  loading.value = true
  try {
    papers.value = (await realExamApi.listPapers()) || []
  } catch {
    papers.value = []
  } finally {
    loading.value = false
  }
}

// 首屏加载 + 证件切换即重拉（单点：watch store.current.id，见 useCredentialRefetch；
// 列表按 current_credential 分区）
onMounted(loadPapers)
useCredentialRefetch(loadPapers)

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
    await ElMessageBox.confirm(`该套真题需 ${p.price} 积分解锁，确认兑换？`, '积分兑换', {
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
    await loadPapers()
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e)
    if (msg.includes('已兑换')) {
      ElMessage.success('已解锁本卷')
      await loadPapers()
    } else {
      ElMessage.error(msg || '兑换失败')
    }
  } finally {
    redeemingId.value = null
  }
}
</script>

<style scoped>
.real-exam-papers {
  max-width: 1200px;
  margin: 0 auto;
}
.real-exam-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 18px;
  flex-wrap: wrap;
}
.real-exam-title {
  font-size: var(--text-2xl);
  line-height: 1.2;
  margin: 0 0 6px;
  color: var(--color-text-primary);
}
.real-exam-sub {
  font-size: 13px;
  color: var(--color-text-tertiary);
  margin: 0;
}
.real-exam-sub-sep {
  margin: 0 6px;
  color: var(--color-text-muted);
}
.empty-wrap {
  padding: 24px 0;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-lg);
}
.variant-c-timeline {
  display: flex;
  flex-direction: column;
  gap: 20px;
}
.timeline-year-head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 10px;
}
.timeline-dot {
  width: 10px;
  height: 10px;
  border-radius: 9999px;
  background: var(--color-primary-500);
  box-shadow: 0 0 0 4px var(--color-primary-100);
  flex-shrink: 0;
}
.timeline-year-label {
  font-size: 16px;
  font-weight: 700;
  color: var(--color-text-primary);
}
.timeline-year-count {
  font-size: 12px;
  color: var(--color-text-muted);
  background: var(--color-bg-page);
  border: 1px solid var(--color-border-light);
  padding: 2px 8px;
  border-radius: var(--radius-full);
}
.timeline-cards {
  margin-left: 20px;
  padding-left: 16px;
  border-left: 1px dashed var(--color-border);
  display: flex;
  flex-direction: column;
  gap: 10px;
}
/* 容器（描边 / 圆角 / 内距 / 底色）已由 UiCard 承担，此处只留悬浮反馈 */
.timeline-card {
  transition: border-color var(--duration-fast) var(--ease-default), box-shadow var(--duration-fast) var(--ease-default);
}
.timeline-card:hover {
  border-color: var(--color-border);
}
.timeline-card-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin-bottom: 8px;
}
.timeline-card-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.meta-text {
  font-size: 12px;
  color: var(--color-text-tertiary);
}
.timeline-card-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px dashed var(--color-border-light);
}
</style>
