<template>
  <div class="mock-exam">
    <div v-if="!examStarted && !examFinished" class="exam-start">
      <el-card>
        <h2>模拟考试</h2>
        <el-form :model="examForm" label-width="100px">
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
      </el-card>

      <el-card class="history-card" v-if="history.length > 0">
        <h3>历史记录</h3>
        <el-table :data="history" stripe>
          <el-table-column prop="score" label="得分" width="80" />
          <el-table-column prop="status" label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="row.status === 'submitted' ? 'success' : 'warning'" size="small">
                {{ row.status === 'submitted' ? '已完成' : '进行中' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="created_at" label="时间" width="180">
            <template #default="{ row }">{{ formatDateTime(row.created_at, '') }}</template>
          </el-table-column>
        </el-table>
      </el-card>
    </div>

    <AnsweringSessionShell
      v-if="examStarted && !examFinished"
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
import { ref, onMounted } from 'vue'
import { mockExamApi, type MockExamHistoryItem } from '@/api/mockExam'
import type { Question } from '@/types/question'
import { formatDateTime } from '@/utils/format'
import AnsweringSessionShell from '@/components/student/AnsweringSessionShell.vue'

const loading = ref(false)
const examStarted = ref(false)
const examFinished = ref(false)
const examForm = ref({ count: 40, duration: 90 })
const mockExamId = ref<number | null>(null)
const questions = ref<Question[]>([])
const answers = ref<Record<number, unknown>>({})
const currentIdx = ref(0)
const remainingTime = ref(0)
const examResult = ref<any>({})
const history = ref<MockExamHistoryItem[]>([])
const shellRef = ref<InstanceType<typeof AnsweringSessionShell> | null>(null)

onMounted(async () => {
  try {
    const res = await mockExamApi.getMockExamHistory({ page: 1, page_size: 5 })
    history.value = res.exams || []
  } catch (e) {}
})

async function startExam() {
  loading.value = true
  try {
    const res = await mockExamApi.startMockExam(examForm.value)
    mockExamId.value = res.mock_exam_id
    questions.value = res.questions
    answers.value = {}
    shellRef.value?.begin(res.remaining_time)
    examStarted.value = true
  } catch {
    /* 错误已由拦截器提示 */
  } finally {
    loading.value = false
  }
}

async function saveProgress() {
  if (!mockExamId.value) return
  try {
    await mockExamApi.saveProgress(mockExamId.value, {
      answers: answers.value,
      remaining_time: remainingTime.value
    })
  } catch (e) {}
}

async function handleSubmit(payload: { is_timeout: boolean; answers: Record<number, unknown>; remaining_time: number }) {
  void payload
  try { await saveProgress() } catch (e) {}
  try {
    if (!mockExamId.value) return
    const res = await mockExamApi.submitMockExam(mockExamId.value)
    examResult.value = res || {}
    examFinished.value = true
  } catch {
    /* 错误已由拦截器提示 */
  }
}

function resetExam() {
  examStarted.value = false
  examFinished.value = false
  questions.value = []
  answers.value = {}
  currentIdx.value = 0
  remainingTime.value = 0
  examResult.value = {}
  mockExamId.value = null
}
</script>

<style scoped>
.mock-exam { max-width: 1200px; margin: 0 auto; }
.exam-start h2, .exam-result h2 { margin-bottom: 20px; }
.history-card { margin-top: 20px; }
.score-display { text-align: center; margin: 20px 0; }
.score-circle { display: inline-flex; flex-direction: column; align-items: center; justify-content: center; width: 150px; height: 150px; border-radius: 50%; border: 6px solid #f56c6c; margin-bottom: 10px; }
.score-circle.passed { border-color: #67c23a; }
.score-num { font-size: 36px; font-weight: bold; }
.score-total { font-size: 14px; color: #909399; }
.detail-card { margin-top: 15px; }
.detail-item { padding: 10px; margin-bottom: 8px; border-radius: 8px; }
.detail-item.correct { background: #f0f9eb; }
.detail-item.wrong { background: #fef0f0; }
.explanation { color: #909399; font-size: 13px; }
</style>
