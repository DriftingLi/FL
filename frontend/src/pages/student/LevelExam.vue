<template>
  <div class="level-exam">
    <div v-if="!inExam" class="exam-list">
      <h2>考试中心</h2>
      <p class="section-desc">下方为当前可参加的考试场次，进入后请在规定时间内完成作答</p>

      <div class="level-exam-section">
        <h3>考试场次</h3>
        <el-table :data="exams" stripe v-loading="loading">
          <el-table-column prop="name" label="考试名称" />
          <el-table-column prop="start_time" label="开始时间" width="180">
            <template #default="{ row }">{{ formatDateTime(row.start_time, '') }}</template>
          </el-table-column>
          <el-table-column prop="duration" label="时长(分钟)" width="100" />
          <el-table-column prop="status" label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="statusType[row.status]" size="small">{{ statusMap[row.status] }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="160">
            <template #default="{ row }">
              <el-button v-if="row.status === 'ongoing' && !row.has_participated && row.can_enter" type="primary" size="small" @click="enterExam(row.id)">进入考试</el-button>
              <el-button v-if="row.status === 'ongoing' && row.has_participated && row.participant_status === 'in_progress'" type="warning" size="small" @click="enterExam(row.id)">继续考试</el-button>
              <el-button v-if="row.has_participated && (row.participant_status === 'submitted' || row.participant_status === 'timeout')" type="success" size="small" @click="viewResult(row)">查看结果</el-button>
            </template>
          </el-table-column>
        </el-table>

        <el-empty v-if="!loading && exams.length === 0" description="暂无可参加的考试场次" />
      </div>
    </div>

    <div v-if="inExam" class="exam-taking">
      <div class="exam-toolbar">
        <div class="exam-title">{{ examTitle }}</div>
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
              <el-tag>{{ (typeMap as Record<string, string>)[currentQ.type] }}</el-tag>
              <span>第 {{ qIdx + 1 }}/{{ examQuestions.length }} 题</span>
            </div>
            <img v-if="currentQ.image_url" :src="currentQ.image_url" class="q-image" />
            <p class="q-content">{{ currentQ.content }}</p>
            <QuestionOptionPicker
              v-if="currentQ.type !== 'short_answer'"
              :options="currentOptions"
              :selected-keys="selectedOptionKeys"
              :multi-choice="currentQ.type === 'multi_choice'"
              @select="key => toggleOpt(currentQ.id, key, currentQ.type === 'multi_choice')"
            />
            <el-input v-else v-model="examAnswers[currentQ.id]" type="textarea" :rows="4" placeholder="请输入答案" />
          </el-card>
          <div class="nav-buttons">
            <el-button @click="qIdx--" :disabled="qIdx === 0">上一题</el-button>
            <el-button @click="qIdx++" :disabled="qIdx === examQuestions.length - 1">下一题</el-button>
          </div>
        </el-col>
        <el-col :xs="24" :md="6">
          <el-card class="answer-card">
            <h4>答题卡</h4>
            <div class="card-grid">
              <div v-for="(q, idx) in examQuestions" :key="q.id"
                   class="card-item" :class="{ current: idx === qIdx, answered: isAnswered(q.id) }"
                   @click="qIdx = idx">{{ idx + 1 }}</div>
            </div>
          </el-card>
        </el-col>
      </el-row>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { Timer } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { levelExamApi, type LevelExamSession } from '@/api/levelExam'
import { typeMap, sessionStatusMap as statusMap } from '@/constants/question'
import type { Question } from '@/types/question'
import { formatClock, formatDateTime } from '@/utils/format'
import { useQuestionAnswer, useCountdown, buildQuestionOptions, isAnswerEmpty } from '@/composables/useQuestionAnswer'
import QuestionOptionPicker from '@/components/student/QuestionOptionPicker.vue'

const statusType: Record<string, string> = { upcoming: 'info', ongoing: 'success', finished: '' }

const loading = ref(false)
const exams = ref<LevelExamSession[]>([])

const inExam = ref(false)
const examTitle = ref('')
const participantId = ref<number | null>(null)
const examQuestions = ref<Question[]>([])
const { answers: examAnswers, toggleOption: toggleOpt, reset: resetAnswers } = useQuestionAnswer()
const qIdx = ref(0)
const { remaining: remainingTime, start: startTimer, stop: stopTimer } = useCountdown({
  autosaveInterval: 30,
  onAutosave: saveProgress,
  onExpire: autoSubmit
})
let refreshTimer: ReturnType<typeof setInterval> | null = null

const currentQ = computed(() => examQuestions.value[qIdx.value] || {})
// 当前题目渲染用选项（判断题渲染对/错模板）
const currentOptions = computed(() => buildQuestionOptions(currentQ.value))
// 当前题目已选中的选项 keys
const selectedOptionKeys = computed(() => {
  const q = currentQ.value
  const ans = examAnswers.value[q.id]
  if (ans === undefined || ans === null) return []
  if (q.type === 'multi_choice') return Array.isArray(ans) ? ans : []
  return [ans]
})

function isAnswered(qid: number) {
  return !isAnswerEmpty(examAnswers.value[qid])
}

function findResumeIndex(questions: { id: number }[], answers: Record<string, unknown>) {
  for (let i = 0; i < questions.length; i++) {
    const qid = questions[i].id
    if (isAnswerEmpty(answers[qid])) return i
  }
  return 0
}

onMounted(async () => {
  await loadExams()
  startRefreshTimer()
})

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
})

function startRefreshTimer() {
  if (refreshTimer) clearInterval(refreshTimer)
  refreshTimer = setInterval(() => {
    if (!inExam.value) loadExams()
  }, 30000)
}

async function loadExams() {
  loading.value = true
  try {
    const res = await levelExamApi.getAvailableExams()
    exams.value = res || []
  } catch (e) {} finally { loading.value = false }
}

async function enterExam(sessionId: number) {
  try {
    const res = await levelExamApi.enterExam(sessionId)
    participantId.value = res.participant_id
    examQuestions.value = res.questions
    resetAnswers(res.answers || {})
    startTimer(res.remaining_time)
    examTitle.value = '考试进行中'
    inExam.value = true
    qIdx.value = findResumeIndex(res.questions, res.answers || {})
  } catch {
    /* 错误已由拦截器提示 */
  }
}

async function saveProgress() {
  try {
    if (participantId.value) {
      await levelExamApi.saveAnswer(participantId.value, { answers: examAnswers.value, remaining_time: remainingTime.value })
    }
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
  try { await saveProgress() } catch (e) {}
  try {
    if (participantId.value) {
      await levelExamApi.submitExam(participantId.value, {
        is_timeout: remainingTime.value <= 0,
        answers: examAnswers.value,
        remaining_time: remainingTime.value
      })
      ElMessage.success('交卷成功，请等待导师批改')
    }
    resetExamState()
    await loadExams()
  } catch {
    /* 错误已由拦截器提示 */
  }
}

function resetExamState() {
  inExam.value = false
  examTitle.value = ''
  participantId.value = null
  examQuestions.value = []
  resetAnswers()
  qIdx.value = 0
  remainingTime.value = 0
}

async function viewResult(row: { id: number; status?: string; name?: string; participant_id?: number; [key: string]: unknown }) {
  if (row.participant_id) {
    try {
      const res = await levelExamApi.getExamResult(row.participant_id)
      const data = res
      const participant = data.participant
      if (participant.score === null || participant.score === undefined) {
        ElMessage.info('考试正在批改中，请耐心等待导师评分')
      } else {
        const status = participant.is_passed ? '通过 🎉' : '未通过'
        ElMessageBox.alert(
          `得分：${participant.score}分\n结果：${status}`,
          '考试结果',
          { confirmButtonText: '确定' }
        )
      }
    } catch {
      /* 错误已由拦截器提示 */
    }
  }
}
</script>

<style scoped>
.level-exam { max-width: 1200px; margin: 0 auto; }
.level-exam h2 { margin-bottom: 10px; }
.section-desc { color: #909399; font-size: 14px; margin-bottom: 20px; }
.level-exam-section { margin-top: 20px; }
.level-exam-section h3 { margin-bottom: 12px; }
.exam-toolbar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 15px; padding: 10px 15px; background: #fff; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
.exam-title { font-size: 16px; font-weight: bold; }
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
</style>
