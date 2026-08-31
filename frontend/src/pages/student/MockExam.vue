<template>
  <div class="mock-exam">
    <div v-if="!inExam && !examFinished" class="exam-start">
      <el-card>
        <h2>{{ paperMode ? `真题考试：${paperTitle}` : '模拟考试' }}</h2>
        <el-form v-if="!paperMode" :model="examForm" label-width="100px">
          <el-form-item label="题目数量">
            <el-select v-model="examForm.count">
              <el-option label="20 题" :value="20" />
              <el-option label="40 题（默认）" :value="40" />
              <el-option label="60 题" :value="60" />
            </el-select>
          </el-form-item>
          <el-form-item label="考试时长">
            <el-select v-model="examForm.duration">
              <el-option label="60分钟" :value="60" />
              <el-option label="90分钟" :value="90" />
              <el-option label="120分钟" :value="120" />
            </el-select>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" size="large" @click="startExam" :loading="loading">开始考试</el-button>
          </el-form-item>
        </el-form>
        <div v-else class="paper-exam-entry">
          <p class="paper-exam-hint">整卷限时作答，交卷后出成绩与逐题解析。</p>
          <el-button type="primary" size="large" @click="startExam" :loading="loading">开始考试</el-button>
        </div>
      </el-card>

      <el-card class="history-card" v-if="history.length > 0">
        <h3>历史记录</h3>
        <el-table :data="history" stripe>
          <el-table-column prop="score" label="得分" width="80" />
          <el-table-column prop="created_at" label="时间" width="180">
            <template #default="{ row }">{{ formatDateTime(row.created_at, '') }}</template>
          </el-table-column>
          <el-table-column label="来源" min-width="90">
            <template #default="{ row }">
              <el-tag v-if="row.paper_id" size="small" type="warning" effect="plain">真题卷</el-tag>
            </template>
          </el-table-column>
        </el-table>
      </el-card>
    </div>

    <AnsweringSessionShell
      v-if="inExam && !examFinished"
      ref="shellRef"
      v-model:answers="answers"
      v-model:remaining-time="remainingTime"
      v-model:current-index="currentIdx"
      :questions="questions"
      show-score
      @autosave="saveProgress"
      @submit="handleSubmit"
    />

    <div v-if="examFinished" class="exam-result">
      <el-card>
        <h2>考试结果</h2>
        <div class="score-display">
          <div class="score-circle" :class="{ passed: examResult.accuracy >= 60 }">
            <span class="score-num">{{ examResult.total_score }}</span>
            <span class="score-total">/{{ examResult.max_score }}</span>
          </div>
          <p>正确率：{{ examResult.accuracy }}%</p>
          <p>正确：{{ examResult.correct_count }}/{{ examResult.total_questions }}题</p>
        </div>
        <el-button type="primary" @click="resetExam">返回</el-button>
      </el-card>

      <el-card v-if="examResult.details" class="detail-card">
        <h3>答题详情</h3>
        <div v-for="(d, idx) in examResult.details" :key="idx" class="detail-item" :class="{ correct: d.is_correct, wrong: !d.is_correct }">
          <p><strong>第{{ idx + 1 }}题：</strong>{{ d.content }}</p>
          <p>你的答案：{{ d.user_answer || '未作答' }} | 正确答案：{{ d.correct_answer }}</p>
          <p v-if="d.explanation" class="explanation">解析：{{ d.explanation }}</p>
        </div>
      </el-card>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { mockExamApi, type MockExamHistoryItem } from '@/api/mockExam'
import { realExamApi } from '@/api/realExam'
import { formatDateTime } from '@/utils/format'
import { useExamSession } from '@/composables/useExamSession'

const route = useRoute()
const router = useRouter()

// ?paper=<id>：真题卷整卷考试（固定题集 + 卷时长），之后 save/submit/result 走同一 mock-exam 链路
const paperMode = computed(() => !!route.query.paper)
const paperId = computed(() => Number(route.query.paper) || 0)
const paperTitle = computed(() => (route.query.title as string) || '')

const loading = ref(false)
const examFinished = ref(false)
const examForm = ref({ count: 40, duration: 90 })
const mockExamId = ref<number | null>(null)
const examResult = ref<any>({})
const history = ref<MockExamHistoryItem[]>([])

// 答题会话编排（进入/续时/断点续传/交卷顺序约束收敛进 useExamSession）
const { inExam, currentIdx, remainingTime, questions, answers, shellRef, start, saveProgress, submit, reset } = useExamSession({
  enter: async () => {
    if (paperMode.value) {
      const res = await realExamApi.startExam(paperId.value)
      mockExamId.value = res.mock_exam_id
      return { questions: res.questions, remaining_time: res.remaining_time }
    }
    const res = await mockExamApi.startMockExam({
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

onMounted(async () => {
  try {
    const res = await mockExamApi.getMockExamHistory({ page: 1, page_size: 5 })
    history.value = res.exams || []
  } catch (e) {}
  // 真题卷入口：进页即开考（未兑换/失败由拦截器提示后返回列表）
  if (paperMode.value) {
    try {
      await startExam()
    } catch {
      router.push({ name: 'RealExamPapers' })
    }
  }
})

async function startExam() {
  loading.value = true
  try {
    await start()
  } catch {
    /* 错误已由拦截器提示 */
  } finally {
    loading.value = false
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
  }
}
</script>

<style scoped>
.mock-exam { max-width: 1200px; margin: 0 auto; }
.exam-start h2, .exam-result h2 { margin-bottom: 20px; }
.history-card { margin-top: 20px; }
.score-display { text-align: center; margin: 20px 0; }
.score-circle { display: inline-flex; flex-direction: column; align-items: center; justify-content: center; width: 150px; height: 150px; border-radius: 50%; border: 6px solid var(--color-danger); margin-bottom: 10px; }
.score-circle.passed { border-color: var(--color-success); }
.score-num { font-size: 36px; font-weight: bold; }
.score-total { font-size: 14px; color: var(--color-text-tertiary); }
.detail-card { margin-top: 15px; }
.detail-item { padding: 10px; margin-bottom: 8px; border-radius: 8px; }
.detail-item.correct { background: var(--color-success-light); }
.detail-item.wrong { background: var(--color-danger-light); }
.explanation { color: var(--color-text-tertiary); font-size: 13px; }
.paper-exam-entry { text-align: center; }
.paper-exam-hint { color: var(--color-text-tertiary); font-size: 13px; margin: 0 0 16px; }
</style>
