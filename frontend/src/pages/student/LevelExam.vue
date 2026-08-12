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

    <AnsweringSessionShell
      v-if="inExam"
      ref="shellRef"
      v-model:answers="examAnswers"
      v-model:remaining-time="remainingTime"
      v-model:current-index="qIdx"
      :questions="examQuestions"
      title="考试进行中"
      @autosave="saveProgress"
      @submit="handleSubmit"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { levelExamApi, type LevelExamSession } from '@/api/levelExam'
import { sessionStatusMap as statusMap } from '@/constants/question'
import type { Question } from '@/types/question'
import { formatDateTime } from '@/utils/format'
import { isAnswerEmpty } from '@/composables/useQuestionAnswer'
import AnsweringSessionShell from '@/components/student/AnsweringSessionShell.vue'

const statusType: Record<string, string> = { upcoming: 'info', ongoing: 'success', finished: '' }

const loading = ref(false)
const exams = ref<LevelExamSession[]>([])

const inExam = ref(false)
const participantId = ref<number | null>(null)
const examQuestions = ref<Question[]>([])
const examAnswers = ref<Record<number, unknown>>({})
const remainingTime = ref(0)
const qIdx = ref(0)
const shellRef = ref<InstanceType<typeof AnsweringSessionShell> | null>(null)
let refreshTimer: ReturnType<typeof setInterval> | null = null

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
    examAnswers.value = { ...(res.answers || {}) }
    shellRef.value?.begin(res.remaining_time)
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

async function handleSubmit(payload: { is_timeout: boolean; answers: Record<number, unknown>; remaining_time: number }) {
  try { await saveProgress() } catch (e) {}
  try {
    if (participantId.value) {
      await levelExamApi.submitExam(participantId.value, {
        is_timeout: payload.is_timeout,
        answers: payload.answers,
        remaining_time: payload.remaining_time
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
  participantId.value = null
  examQuestions.value = []
  examAnswers.value = {}
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
</style>
