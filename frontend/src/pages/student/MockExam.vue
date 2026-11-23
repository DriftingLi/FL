<template>
  <div class="mx-auto max-w-[1200px]">
    <div v-if="!inExam && !examFinished" class="exam-start">
      <UiCard>
        <h2>{{ paperMode ? `真题考试：${paperTitle}` : '模拟考试' }}</h2>
        <el-form v-if="!paperMode" :model="examForm" label-width="100px">
          <el-form-item label="本次考试分类">
            <UiSelect v-model="categoryId" placeholder="选择培训分类" :options="categories.map(category => ({ label: category.name, value: category.id }))" />
          </el-form-item>
          <el-form-item label="题目数量">
            <UiSelect v-model="examForm.count" :options="[{ label: '20 题', value: 20 }, { label: '40 题（默认）', value: 40 }, { label: '60 题', value: 60 }]" />
          </el-form-item>
          <el-form-item label="考试时长">
            <UiSelect v-model="examForm.duration" :options="[{ label: '60分钟', value: 60 }, { label: '90分钟', value: 90 }, { label: '120分钟', value: 120 }]" />
          </el-form-item>
          <el-form-item>
            <UiButton variant="primary" size="large" @click="startExam()" :loading="loading" :disabled="!categoryId">开始考试</UiButton>
          </el-form-item>
        </el-form>
        <div v-else class="text-center">
          <p class="m-0 mb-4 text-sm text-ink-3">整卷限时作答，交卷后出成绩与逐题解析。</p>
          <UiButton variant="primary" size="large" @click="startExam()" :loading="loading">开始考试</UiButton>
        </div>
      </UiCard>

      <UiCard v-if="ongoing.length > 0" class="mt-5">
        <h3>在途考试</h3>
        <el-table :data="ongoing" stripe>
          <el-table-column label="分类" min-width="100">
            <template #default="{ row }">{{ historyCategoryName(row.category_id) }}</template>
          </el-table-column>
          <el-table-column label="开始时间" min-width="170">
            <template #default="{ row }">{{ formatDateTime(row.start_time, '') }}</template>
          </el-table-column>
          <el-table-column label="操作" min-width="100">
            <template #default="{ row }">
              <UiButton size="small" :disabled="loading" @click="startExam(row.id)">继续考试</UiButton>
            </template>
          </el-table-column>
        </el-table>
      </UiCard>

      <UiCard class="mt-5">
        <h3>历史记录</h3>
        <UiSelect v-model="historyCategory" aria-label="历史记录分类" class="mb-3 w-[190px]" :options="historyOptions" />
        <el-table :data="history" stripe empty-text="暂无考试记录">
          <el-table-column prop="score" label="得分" width="80" />
          <el-table-column label="分类" min-width="100">
            <template #default="{ row }">{{ historyCategoryName(row.category_id) }}</template>
          </el-table-column>
          <el-table-column prop="created_at" label="时间" width="180">
            <template #default="{ row }">{{ formatDateTime(row.created_at, '') }}</template>
          </el-table-column>
          <el-table-column label="来源" min-width="90">
            <template #default="{ row }">
              <UiTag v-if="row.paper_id" size="small" tone="warning" effect="plain">真题卷</UiTag>
            </template>
          </el-table-column>
          <el-table-column label="操作" min-width="100">
            <template #default="{ row }">
              <UiButton size="small" @click="viewResult(row.id)">查看成绩</UiButton>
            </template>
          </el-table-column>
        </el-table>
        <UiPagination v-if="historyTotal > HISTORY_PAGE_SIZE" v-model:current-page="historyPage" :total="historyTotal" :page-size="HISTORY_PAGE_SIZE" />
      </UiCard>
    </div>

    <AnsweringSessionShell
      v-if="inExam && !examFinished"
      :ref="attachExamShell"
      v-model:answers="answers"
      v-model:remaining-time="remainingTime"
      v-model:current-index="currentIdx"
      :questions="questions"
      show-score
      @autosave="saveProgress"
      @submit="handleSubmit"
    />

    <div v-if="examFinished" class="exam-result">
      <UiCard>
        <h2>考试结果</h2>
        <div class="my-5 text-center">
          <div class="score-circle mb-2.5 inline-flex size-[150px] flex-col items-center justify-center rounded-pill border-[6px] border-bad" :class="examResult.accuracy >= 60 ? 'border-ok' : ''">
            <span class="text-4xl font-bold">{{ examResult.total_score }}</span>
            <span class="text-sm text-ink-3">/{{ examResult.max_score }}</span>
          </div>
          <p>正确率：{{ examResult.accuracy }}%</p>
          <p>正确：{{ examResult.correct_count }}/{{ examResult.total_questions }}题</p>
        </div>
        <UiButton variant="primary" @click="resetExam">返回</UiButton>
      </UiCard>

      <UiCard v-if="examResult.details" class="mt-[15px]">
        <h3>答题详情</h3>
        <!-- 判色三态（ADR-0068）：is_correct 为 null = 未判对错（简答），既不是对也不是错 —— 
             它现在也拿得到得分，标红会变成「8/10 分却画成错题」。 -->
        <div
          v-for="(d, idx) in examResult.details"
          :key="idx"
          class="detail-item mb-2 rounded-ctl p-2.5"
          :class="d.is_correct === true ? 'bg-ok-soft' : d.is_correct === false ? 'bg-bad-soft' : ''"
        >
          <p><strong>第{{ Number(idx) + 1 }}题：</strong>{{ d.content }}</p>
          <p>你的答案：{{ d.user_answer || '未作答' }} | 正确答案：{{ d.correct_answer }}</p>
          <!-- 本题得分与总分同源（ADR-0068 决策 3）：短答不再并排展示「0 分」与「AI 评分」两个数 -->
          <p>本题得分：{{ d.score }} / {{ d.max_score }}</p>
          <p v-if="d.ai_comment" class="text-sm text-ink-3">AI 评语：{{ d.ai_comment }}</p>
          <p v-if="d.explanation" class="text-sm text-ink-3">解析：{{ d.explanation }}</p>
        </div>
      </UiCard>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, type ComponentPublicInstance } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { mockExamApi, type MockExamHistoryItem, type MockExamHistoryQuery } from '@/api/mockExam'
import { realExamApi } from '@/api/realExam'
import { trainingApi, type TrainingCategoryDTO } from '@/api/training'
import { formatDateTime } from '@/utils/format'
import { useExamSession } from '@/composables/useExamSession'
import { useAsyncPage } from '@/composables/useAsyncPage'
import AnsweringSessionShell from '@/components/student/AnsweringSessionShell.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiTag from '@/components/ui/UiTag.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiSelect from '@/components/ui/UiSelect.vue'
import UiPagination from '@/components/ui/UiPagination.vue'

const route = useRoute()
const router = useRouter()

// ?paper=<id>：真题卷整卷考试（固定题集 + 卷时长），之后 save/submit/result 走同一 mock-exam 链路
const paperMode = computed(() => !!route.query.paper)
const paperId = computed(() => Number(route.query.paper) || 0)
const paperTitle = computed(() => (route.query.title as string) || '')

const loading = ref(false)
const examFinished = ref(false)
const examForm = ref({ count: 40, duration: 90 })
const categories = ref<TrainingCategoryDTO[]>([])
const historicalCategories = ref<TrainingCategoryDTO[]>([])
const categoryId = ref<number | null>(null)
const historyCategory = ref<NonNullable<MockExamHistoryQuery['category_id']>>('all')
const historyOptions = computed(() => [
  { label: '全部分类', value: 'all' },
  ...historicalCategories.value.map(category => ({ label: category.name, value: category.id })),
  { label: '历史未分类', value: 'unclassified' }
])
const HISTORY_PAGE_SIZE = 10
const historyPage = ref(1)
const historyTotal = ref(0)
const ongoing = ref<MockExamHistoryItem[]>([])
const mockExamId = ref<number | null>(null)
const examResult = ref<any>({})
const history = ref<MockExamHistoryItem[]>([])
let resumeId: number | undefined

function historyCategoryName(id?: number) {
  return id ? historicalCategories.value.find(category => category.id === id)?.name || '—' : '历史未分类'
}

// 答题会话编排（进入/续时/断点续传/交卷顺序约束收敛进 useExamSession）
const { inExam, currentIdx, remainingTime, questions, answers, shellRef, start, saveProgress, submit, reset } = useExamSession({
  enter: async () => {
    if (resumeId) {
      const res = await mockExamApi.resumeMockExam(resumeId)
      mockExamId.value = res.mock_exam_id
      return res
    }
    if (paperMode.value) {
      const res = await realExamApi.startExam(paperId.value)
      mockExamId.value = res.mock_exam_id
      return { questions: res.questions, remaining_time: res.remaining_time }
    }
    if (!categoryId.value) return null
    const res = await mockExamApi.startMockExam({
      category_id: categoryId.value,
      question_count: examForm.value.count,
      duration_minutes: examForm.value.duration
    })
    mockExamId.value = res.mock_exam_id
    return { questions: res.questions, remaining_time: res.remaining_time }
  },
  save: async (ans, remaining) => {
    if (!mockExamId.value) return
    await mockExamApi.saveProgress(mockExamId.value, {
      answers: ans,
      remaining_time: remaining
    })
  },
  submit: async () => {
    if (!mockExamId.value) return null
    return mockExamApi.submitMockExam(mockExamId.value)
  }
})

function attachExamShell(instance: Element | ComponentPublicInstance | null) {
  shellRef.value = instance as typeof shellRef.value
}

// 历史记录三态收编 useAsyncPage（#439）：loader 纯装配，失败收敛 loadError（不打断考试主流程）
const { run: loadHistory } = useAsyncPage(async () => {
  return mockExamApi.getMockExamHistory({
    page: historyPage.value, page_size: HISTORY_PAGE_SIZE, category_id: historyCategory.value
  })
}, {
  apply: (res) => {
    history.value = res.exams || []
    historyTotal.value = res.total
  }
})

const { run: loadOngoing } = useAsyncPage(() => mockExamApi.getInProgress({ page: 1, page_size: 10 }), {
  apply: (res) => { ongoing.value = res.exams }
})

const { run: loadCategories } = useAsyncPage(async () => Promise.all([
  trainingApi.getCategories(), trainingApi.getHistoricalCategories()
]), {
  apply: ([active, historical]) => {
    categories.value = active.categories
    historicalCategories.value = historical.categories
  }
})

watch(historyCategory, () => { historyPage.value = 1 })
watch([historyCategory, historyPage], () => { void loadHistory() })

onMounted(async () => {
  void loadHistory()
  void loadOngoing()
  void loadCategories()
  if (paperMode.value && !(await startExam())) router.push({ name: 'RealExamPapers' })
})

async function startExam(id?: number) {
  if (loading.value || (!id && !paperMode.value && !categoryId.value)) return false
  resumeId = id
  loading.value = true
  try {
    await start()
    return inExam.value
  } catch (e) {
    /* 空卷兜底由 useExamSession 抛错（#702）：拦截器不认识该本地错误，此处补一句提示 */
    if (e instanceof Error && e.message === '题库暂无可用的题目') {
      const { ElMessage } = await import('element-plus')
      ElMessage.warning('题库暂无可用的题目')
    }
    /* 后端错误已由拦截器提示 */
    return false
  } finally {
    resumeId = undefined
    loading.value = false
  }
}

async function viewResult(id: number) {
  try {
    examResult.value = await mockExamApi.getMockExamResult(id)
    examFinished.value = true
  } catch {
    /* 错误已由拦截器提示 */
  }
}

async function handleSubmit(payload: { is_timeout: boolean; answers: Record<number, unknown>; remaining_time: number }) {
  try {
    const res = await submit(payload)
    examResult.value = res || {}
    examFinished.value = true
  } catch {
    /* 错误已由拦截器提示 */
  }
}

function resetExam() {
  examFinished.value = false
  examResult.value = {}
  mockExamId.value = null
  reset()
  if (paperMode.value) {
    router.push({ name: 'RealExamPapers' })
  } else {
    void loadHistory()
    void loadOngoing()
  }
}
</script>

