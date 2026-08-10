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

    <div v-if="examStarted && !examFinished" class="exam-taking">
      <div class="exam-toolbar">
        <div class="timer" :class="{ warning: remainingTime < 300 }">
          <el-icon><Timer /></el-icon>
          <span>{{ formatClock(remainingTime) }}</span>
        </div>
        <el-button type="danger" @click="confirmSubmit">交卷</el-button>
      </div>

      <el-row :gutter="20">
        <el-col :xs="24" :md="18">
          <el-card class="question-card">
            <div class="question-header">
              <el-tag>{{ (typeMap as Record<string, string>)[currentQuestion.type] }}</el-tag>
              <span>第 {{ currentIdx + 1 }}/{{ questions.length }} 题（{{ currentQuestion.score }}分）</span>
            </div>
            <img v-if="currentQuestion.image_url" :src="currentQuestion.image_url" class="q-image" />
            <p class="q-content">{{ currentQuestion.content }}</p>
            <QuestionOptionPicker
              v-if="currentQuestion.type !== 'short_answer'"
              :options="currentOptions"
              :selected-keys="selectedOptionKeys"
              :multi-choice="currentQuestion.type === 'multi_choice'"
              @select="key => toggleOption(currentQuestion.id, key, currentQuestion.type === 'multi_choice')"
            />
            <el-input v-else v-model="answers[currentQuestion.id]" type="textarea" :rows="4" placeholder="请输入答案" />
          </el-card>

          <div class="nav-buttons">
            <el-button @click="prevQuestion" :disabled="currentIdx === 0">上一题</el-button>
            <el-button @click="nextQuestion" :disabled="currentIdx === questions.length - 1">下一题</el-button>
          </div>
        </el-col>

        <el-col :xs="24" :md="6">
          <el-card class="answer-card">
            <h4>答题卡</h4>
            <div class="card-grid">
              <div v-for="(q, idx) in questions" :key="q.id"
                   class="card-item" :class="{
                     current: idx === currentIdx,
                     answered: !isAnswerEmpty(answers[q.id])
                   }"
                   @click="currentIdx = idx">
                {{ idx + 1 }}
              </div>
            </div>
          </el-card>
        </el-col>
      </el-row>
    </div>

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
import { Timer } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { mockExamApi, type MockExamHistoryItem } from '@/api/mockExam'
import { typeMap } from '@/constants/question'
import type { Question } from '@/types/question'
import { formatClock, formatDateTime } from '@/utils/format'
import { useQuestionAnswer, useCountdown, buildQuestionOptions, isAnswerEmpty } from '@/composables/useQuestionAnswer'
import QuestionOptionPicker from '@/components/student/QuestionOptionPicker.vue'

const loading = ref(false)
const examStarted = ref(false)
const examFinished = ref(false)
const examForm = ref({ count: 40, duration: 90 })
const mockExamId = ref<number | null>(null)
const questions = ref<Question[]>([])
const { answers, toggleOption, reset: resetAnswers } = useQuestionAnswer()
const currentIdx = ref(0)
const examResult = ref<any>({})
const history = ref<MockExamHistoryItem[]>([])
const { remaining: remainingTime, start: startTimer, stop: stopTimer } = useCountdown({
  autosaveInterval: 30,
  onAutosave: saveProgress,
  onExpire: autoSubmit
})

const currentQuestion = computed(() => questions.value[currentIdx.value] || {})
// 当前题目渲染用选项（判断题渲染对/错模板）
const currentOptions = computed(() => buildQuestionOptions(currentQuestion.value))
// 当前题目已选中的选项 keys
const selectedOptionKeys = computed(() => {
  const q = currentQuestion.value
  const ans = answers.value[q.id]
  if (ans === undefined || ans === null) return []
  if (q.type === 'multi_choice') return Array.isArray(ans) ? ans : []
  return [ans]
})

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
    startTimer(res.remaining_time)
    examStarted.value = true
  } catch {
    /* 错误已由拦截器提示 */
  } finally {
    loading.value = false
  }
}

function prevQuestion() { if (currentIdx.value > 0) currentIdx.value-- }
function nextQuestion() { if (currentIdx.value < questions.value.length - 1) currentIdx.value++ }

async function saveProgress() {
  if (!mockExamId.value) return
  try {
    await mockExamApi.saveProgress(mockExamId.value, {
      answers: answers.value,
      remaining_time: remainingTime.value
    })
  } catch (e) {}
}

async function confirmSubmit() {
  try {
    await ElMessageBox.confirm('确定要交卷吗？', '提示', { type: 'warning' })
    await doSubmit()
  } catch (e) {}
}

async function autoSubmit() {
  ElMessage.warning('考试时间已到，自动交卷')
  await doSubmit()
}

async function doSubmit() {
  stopTimer()
  await saveProgress()
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
  resetAnswers()
  currentIdx.value = 0
  examResult.value = {}
  mockExamId.value = null
}
</script>

<style scoped>
.mock-exam { max-width: 1200px; margin: 0 auto; }
.exam-start h2, .exam-result h2 { margin-bottom: 20px; }
.history-card { margin-top: 20px; }
.exam-toolbar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 15px; padding: 10px 15px; background: #fff; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
.timer { font-size: 20px; font-weight: bold; display: flex; align-items: center; gap: 8px; }
.timer.warning { color: #f56c6c; }
.question-card { margin-bottom: 15px; }
.question-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 15px; }
.q-image { max-width: 100%; max-height: 250px; border-radius: 8px; margin-bottom: 10px; }
.q-content { font-size: 16px; line-height: 1.8; margin-bottom: 15px; }
.nav-buttons { display: flex; justify-content: center; gap: 15px; margin: 15px 0; }
.answer-card h4 { margin-bottom: 10px; }
.card-grid { display: flex; flex-wrap: wrap; gap: 5px; }
.card-item { width: 32px; height: 32px; line-height: 32px; text-align: center; border: 1px solid #dcdfe6; border-radius: 4px; cursor: pointer; font-size: 12px; }
.card-item.current { border-color: #409eff; background: #409eff; color: #fff; }
.card-item.answered { border-color: #67c23a; background: #f0f9eb; }
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
