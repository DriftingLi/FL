<template>
  <div class="level-exam">
    <div v-if="!inExam" class="exam-list">
      <h2>考试中心</h2>
      <div class="level-info">
        <el-tag :type="levelTagType" size="large" effect="dark">
          当前等级：{{ levelMap[userLevel] }}学徒
        </el-tag>
        <span class="level-tip" v-if="nextLevel">
          通过{{ levelMap[userLevel] }}等级考试可晋级为{{ levelMap[nextLevel] }}学徒
        </span>
        <span class="level-tip" v-else>
          您已达到最高等级
        </span>
      </div>

      <el-row :gutter="20" class="exam-cards">
        <el-col :xs="24" :sm="12">
          <el-card shadow="hover" class="exam-type-card level-exam-card">
            <div class="exam-type-icon">📋</div>
            <h3>等级考试</h3>
            <p>通过等级考试晋升学徒等级</p>
            <el-button type="primary" @click="showLevelExams = true; showMockExam = false" style="margin-top:10px">
              查看考试
            </el-button>
          </el-card>
        </el-col>
        <el-col :xs="24" :sm="12">
          <el-card shadow="hover" class="exam-type-card mock-exam-card">
            <div class="exam-type-icon">📝</div>
            <h3>模拟考试</h3>
            <p>全真模拟考试环境练习</p>
            <el-button type="warning" @click="showMockExam = true; showLevelExams = false" style="margin-top:10px">
              开始模拟
            </el-button>
          </el-card>
        </el-col>
      </el-row>

      <div v-if="showLevelExams" class="level-exam-section">
        <h3>等级考试列表</h3>
        <el-table :data="exams" stripe v-loading="loading">
          <el-table-column prop="name" label="考试名称" />
          <el-table-column prop="level" label="等级" width="100">
            <template #default="{ row }">{{ levelMap[row.level] }}</template>
          </el-table-column>
          <el-table-column prop="start_time" label="开始时间" width="180">
            <template #default="{ row }">{{ formatDateTime(row.start_time) }}</template>
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
              <el-tag v-if="row.status === 'ongoing' && !row.has_participated && !row.can_enter" type="info" size="small">等级不符</el-tag>
              <el-button v-if="row.status === 'ongoing' && row.has_participated && row.participant_status === 'in_progress'" type="warning" size="small" @click="enterExam(row.id)">继续考试</el-button>
              <el-button v-if="row.has_participated && (row.participant_status === 'submitted' || row.participant_status === 'timeout')" type="success" size="small" @click="viewResult(row)">查看结果</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <div v-if="showMockExam" class="mock-exam-section">
        <el-card>
          <h3>模拟考试</h3>
          <el-form :model="mockForm" label-width="100px" style="max-width:400px;margin-top:15px">
            <el-form-item label="考试等级">
              <el-select v-model="mockForm.level">
                <el-option label="初级" value="beginner" />
                <el-option label="中级" value="intermediate" />
                <el-option label="高级" value="advanced" />
              </el-select>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" size="large" @click="startMockExam" :loading="mockLoading">开始模拟考试</el-button>
            </el-form-item>
          </el-form>
        </el-card>
      </div>
    </div>

    <div v-if="inExam" class="exam-taking">
      <div class="exam-toolbar">
        <div class="exam-title">{{ examTitle }}</div>
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
              <el-tag>{{ typeMap[currentQ.type] }}</el-tag>
              <span>第 {{ qIdx + 1 }}/{{ examQuestions.length }} 题</span>
            </div>
            <img v-if="currentQ.image_url" :src="currentQ.image_url" class="q-image" />
            <p class="q-content">{{ currentQ.content }}</p>
            <div v-if="currentQ.type !== 'short_answer'" class="q-options">
              <template v-if="currentQ.type === 'true_false'">
                <div v-for="opt in [{ key: '对', label: '正确' }, { key: '错', label: '错误' }]" :key="opt.key"
                     class="q-option" :class="{ selected: isOptSelected(opt.key) }"
                     @click="toggleOpt(opt.key)">
                  <span class="opt-label">{{ opt.key }}</span>
                  <span>{{ opt.label }}</span>
                </div>
              </template>
              <template v-else>
                <div v-for="(label, key) in currentQ.options" :key="key"
                     class="q-option" :class="{ selected: isOptSelected(key) }"
                     @click="toggleOpt(key)">
                  <span class="opt-label">{{ key }}</span>
                  <span>{{ label }}</span>
                </div>
              </template>
            </div>
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
import { levelExamApi } from '@/api/levelExam'
import { mockExamApi } from '@/api/mockExam'
import { useAuthStore } from '@/stores/auth'
import { useUserStore } from '@/stores/user'

const authStore = useAuthStore()
const userStore = useUserStore()

const levelMap = { beginner: '初级', intermediate: '中级', advanced: '高级', expert: '顶级' }
const statusMap = { upcoming: '未开始', ongoing: '进行中', finished: '已结束' }
const statusType = { upcoming: 'info', ongoing: 'success', finished: '' }
const typeMap = { single_choice: '单选题', multi_choice: '多选题', true_false: '判断题', fault_image: '故障识图', short_answer: '简答题' }

const userLevel = ref(authStore.userInfo?.level || 'beginner')
const nextLevel = computed(() => {
  const order = { beginner: 'intermediate', intermediate: 'advanced', advanced: 'expert' }
  return order[userLevel.value]
})
const levelTagType = computed(() => {
  const map = { beginner: 'success', intermediate: 'warning', advanced: 'danger', expert: '' }
  return map[userLevel.value] || 'success'
})

const showLevelExams = ref(false)
const showMockExam = ref(false)
const loading = ref(false)
const exams = ref([])

const inExam = ref(false)
const examMode = ref('')
const examTitle = ref('')
const participantId = ref(null)
const mockExamId = ref(null)
const examQuestions = ref([])
const examAnswers = ref<any>({})
const qIdx = ref(0)
const remainingTime = ref(0)
let timer = null

const mockForm = ref({ level: 'beginner', duration: 90 })
const mockLoading = ref(false)
let refreshTimer = null

const currentQ = computed(() => examQuestions.value[qIdx.value] || {})

function formatDateTime(dtStr) {
  if (!dtStr) return ''
  const d = new Date(dtStr)
  if (isNaN(d.getTime())) return dtStr
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function isAnswered(qid) {
  const a = examAnswers.value[qid]
  if (a === undefined || a === null || a === '') return false
  if (Array.isArray(a)) return a.length > 0
  return true
}

function findResumeIndex(questions, answers) {
  for (let i = 0; i < questions.length; i++) {
    const qid = questions[i].id
    const a = answers[qid]
    if (a === undefined || a === null || a === '') return i
    if (Array.isArray(a) && a.length === 0) return i
  }
  return 0
}

onMounted(async () => {
  if (!authStore.userInfo?.level) {
    try {
      await userStore.fetchProfile()
      userLevel.value = userStore.profile?.level || 'beginner'
    } catch (e) {}
  }
  mockForm.value.level = userLevel.value
  await loadExams()
  startRefreshTimer()
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
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
    exams.value = res.data || []
  } catch (e) {} finally { loading.value = false }
}

async function enterExam(sessionId) {
  try {
    const res = await levelExamApi.enterExam(sessionId)
    participantId.value = res.data.participant_id
    mockExamId.value = null
    examQuestions.value = res.data.questions
    examAnswers.value = res.data.answers || {}
    remainingTime.value = res.data.remaining_time
    examMode.value = 'level'
    examTitle.value = '等级考试'
    inExam.value = true
    qIdx.value = findResumeIndex(res.data.questions, res.data.answers || {})
    startTimer()
  } catch (e) {
    ElMessage.error(e.message || '进入考试失败')
  }
}

async function startMockExam() {
  mockLoading.value = true
  try {
    const res = await mockExamApi.startMockExam(mockForm.value)
    mockExamId.value = res.data.mock_exam_id
    participantId.value = null
    examQuestions.value = res.data.questions
    examAnswers.value = {}
    remainingTime.value = res.data.remaining_time
    examMode.value = 'mock'
    examTitle.value = '模拟考试'
    inExam.value = true
    startTimer()
  } catch (e) {
    ElMessage.error(e.message || '开始考试失败')
  } finally {
    mockLoading.value = false
  }
}

function startTimer() {
  if (timer) clearInterval(timer)
  timer = setInterval(() => {
    if (remainingTime.value <= 0) { clearInterval(timer); autoSubmit(); return }
    remainingTime.value--
    if (remainingTime.value % 30 === 0) saveProgress()
  }, 1000)
}

function formatTime(s) {
  return `${String(Math.floor(s / 60)).padStart(2, '0')}:${String(s % 60).padStart(2, '0')}`
}

function isOptSelected(key) {
  const a = examAnswers.value[currentQ.value.id]
  if (!a) return false
  if (currentQ.value.type === 'multi_choice') return Array.isArray(a) && a.includes(key)
  return a === key
}

function toggleOpt(key) {
  const qid = currentQ.value.id
  if (currentQ.value.type === 'multi_choice') {
    if (!examAnswers.value[qid]) examAnswers.value[qid] = []
    const idx = examAnswers.value[qid].indexOf(key)
    if (idx > -1) examAnswers.value[qid].splice(idx, 1)
    else examAnswers.value[qid].push(key)
  } else {
    examAnswers.value[qid] = key
  }
}

async function saveProgress() {
  try {
    if (examMode.value === 'level' && participantId.value) {
      await levelExamApi.saveAnswer(participantId.value, { answers: examAnswers.value, remaining_time: remainingTime.value })
    } else if (examMode.value === 'mock' && mockExamId.value) {
      await mockExamApi.saveProgress(mockExamId.value, { answers: examAnswers.value, remaining_time: remainingTime.value })
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
  if (timer) clearInterval(timer)
  try { await saveProgress() } catch (e) {}
  try {
    if (examMode.value === 'level' && participantId.value) {
      await levelExamApi.submitExam(participantId.value, {
        is_timeout: remainingTime.value <= 0,
        answers: examAnswers.value,
        remaining_time: remainingTime.value
      })
      ElMessage.success('交卷成功，请等待导师批改')
    } else if (examMode.value === 'mock' && mockExamId.value) {
      const res = await mockExamApi.submitMockExam(mockExamId.value)
      const result = res.data || {}
      const status = result.accuracy >= 60 ? '通过' : '未通过'
      ElMessageBox.alert(
        `得分：${result.total_score}/${result.max_score}\n正确率：${result.accuracy}%\n结果：${status}`,
        '模拟考试结果',
        { confirmButtonText: '确定' }
      )
    }
    resetExamState()
    await loadExams()
  } catch (e) {
    ElMessage.error(e.message || '交卷失败')
  }
}

function resetExamState() {
  inExam.value = false
  examMode.value = ''
  examTitle.value = ''
  participantId.value = null
  mockExamId.value = null
  examQuestions.value = []
  examAnswers.value = {}
  qIdx.value = 0
  remainingTime.value = 0
}

async function viewResult(row) {
  if (row.participant_id) {
    try {
      const res = await levelExamApi.getExamResult(row.participant_id)
      const data = res.data
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
    } catch (e) {
      ElMessage.error('获取结果失败')
    }
  }
}
</script>

<style scoped>
.level-exam { max-width: 1200px; margin: 0 auto; }
.level-exam h2 { margin-bottom: 10px; }
.level-info { display: flex; align-items: center; gap: 12px; margin-bottom: 20px; }
.level-tip { color: #909399; font-size: 14px; }
.exam-cards { margin-bottom: 25px; }
.exam-type-card { text-align: center; padding: 20px; cursor: default; }
.exam-type-icon { font-size: 48px; margin-bottom: 10px; }
.exam-type-card h3 { margin: 10px 0 5px; }
.exam-type-card p { color: #909399; font-size: 14px; }
.level-exam-card { border-top: 3px solid #409eff; }
.mock-exam-card { border-top: 3px solid #e6a23c; }
.level-exam-section, .mock-exam-section { margin-top: 20px; }
.exam-toolbar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 15px; padding: 10px 15px; background: #fff; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
.exam-title { font-size: 16px; font-weight: bold; }
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
</style>
