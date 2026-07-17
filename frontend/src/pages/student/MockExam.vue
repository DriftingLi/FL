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
            <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
          </el-table-column>
        </el-table>
      </el-card>
    </div>

    <div v-if="examStarted && !examFinished" class="exam-taking">
      <div class="exam-toolbar">
        <div class="timer" :class="{ warning: remainingTime < 300 }">
          <el-icon><Timer /></el-icon>
          <span>{{ formatTime(remainingTime) }}</span>
        </div>
        <el-button type="danger" @click="confirmSubmit">交卷</el-button>
      </div>

      <el-row :gutter="20">
        <el-col :xs="24" :md="18">
          <el-card class="question-card">
            <div class="question-header">
              <el-tag>{{ typeMap[currentQuestion.type] }}</el-tag>
              <span>第 {{ currentIdx + 1 }}/{{ questions.length }} 题（{{ currentQuestion.score }}分）</span>
            </div>
            <img v-if="currentQuestion.image_url" :src="currentQuestion.image_url" class="q-image" />
            <p class="q-content">{{ currentQuestion.content }}</p>
            <div v-if="currentQuestion.type !== 'short_answer'" class="q-options">
              <template v-if="currentQuestion.type === 'true_false'">
                <div v-for="opt in [{ key: '对', label: '正确' }, { key: '错', label: '错误' }]" :key="opt.key"
                     class="q-option" :class="{ selected: isOptionSelected(opt.key) }"
                     @click="toggleOption(opt.key)">
                  <span class="opt-label">{{ opt.key }}</span>
                  <span>{{ opt.label }}</span>
                </div>
              </template>
              <template v-else>
                <div v-for="(label, key) in currentQuestion.options" :key="key"
                     class="q-option" :class="{ selected: isOptionSelected(key) }"
                     @click="toggleOption(key)">
                  <span class="opt-label">{{ key }}</span>
                  <span>{{ label }}</span>
                </div>
              </template>
            </div>
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
                     answered: answers[q.id] !== undefined && answers[q.id] !== '' && answers[q.id] !== null
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
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { Timer } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { mockExamApi } from '@/api/mockExam'
import { typeMap } from '@/constants/question'

const loading = ref(false)
const examStarted = ref(false)
const examFinished = ref(false)
const examForm = ref({ count: 40, duration: 90 })
const mockExamId = ref(null)
const questions = ref([])
const answers = ref<any>({})
const currentIdx = ref(0)
const remainingTime = ref(0)
const examResult = ref<any>({})
const history = ref([])
let timer = null

const currentQuestion = computed(() => questions.value[currentIdx.value] || {})

onMounted(async () => {
  try {
    const res = await mockExamApi.getMockExamHistory({ page: 1, page_size: 5 })
    history.value = res.data?.exams || []
  } catch (e) {}
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})

async function startExam() {
  loading.value = true
  try {
    const res = await mockExamApi.startMockExam(examForm.value)
    mockExamId.value = res.data.mock_exam_id
    questions.value = res.data.questions
    remainingTime.value = res.data.remaining_time
    examStarted.value = true
    startTimer()
  } catch (e) {
    ElMessage.error(e.message || '开始考试失败')
  } finally {
    loading.value = false
  }
}

function startTimer() {
  timer = setInterval(() => {
    if (remainingTime.value <= 0) {
      clearInterval(timer)
      autoSubmit()
      return
    }
    remainingTime.value--
    if (remainingTime.value % 30 === 0) {
      saveProgress()
    }
  }, 1000)
}

function formatTime(seconds) {
  const m = Math.floor(seconds / 60)
  const s = seconds % 60
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

function formatDateTime(dtStr) {
  if (!dtStr) return ''
  const d = new Date(dtStr)
  if (isNaN(d.getTime())) return dtStr
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function isOptionSelected(key) {
  const ans = answers.value[currentQuestion.value.id]
  if (!ans) return false
  if (currentQuestion.value.type === 'multi_choice') {
    return Array.isArray(ans) && ans.includes(key)
  }
  return ans === key
}

function toggleOption(key) {
  const qid = currentQuestion.value.id
  if (currentQuestion.value.type === 'multi_choice') {
    if (!answers.value[qid]) answers.value[qid] = []
    const idx = answers.value[qid].indexOf(key)
    if (idx > -1) answers.value[qid].splice(idx, 1)
    else answers.value[qid].push(key)
  } else {
    answers.value[qid] = key
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
  if (timer) clearInterval(timer)
  await saveProgress()
  try {
    const res = await mockExamApi.submitMockExam(mockExamId.value)
    examResult.value = res.data || {}
    examFinished.value = true
  } catch (e) {
    ElMessage.error('交卷失败')
  }
}

function resetExam() {
  examStarted.value = false
  examFinished.value = false
  questions.value = []
  answers.value = {}
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
.q-options { display: flex; flex-direction: column; gap: 8px; }
.q-option { display: flex; align-items: center; padding: 10px 15px; border: 1px solid #dcdfe6; border-radius: 8px; cursor: pointer; }
.q-option:hover { border-color: #409eff; }
.q-option.selected { border-color: #409eff; background: #ecf5ff; }
.opt-label { width: 28px; height: 28px; line-height: 28px; text-align: center; border-radius: 50%; background: #f5f7fa; margin-right: 10px; font-weight: bold; }
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
