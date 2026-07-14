<template>
  <div class="question-bank">
    <!-- 入口：等级展示 + 刷题配置 -->
    <div v-if="!practiceStarted && !practiceFinished">
      <el-row :gutter="20">
        <el-col :span="24">
          <h2>题库练习</h2>
          <div class="user-level-badge">
            <el-tag :type="levelTagType" size="large" effect="dark">
              当前等级：{{ levelLabelMap[userLevel] }}学徒
            </el-tag>
            <span class="level-hint">{{ levelHint }}</span>
          </div>
        </el-col>
      </el-row>

      <el-row :gutter="20" class="level-cards">
        <el-col :xs="24" :sm="8" v-for="level in visibleLevels" :key="level.value">
          <el-card
            class="level-card"
            :class="['level-' + level.value, { 'level-locked': level.locked }]"
            shadow="hover"
          >
            <div class="level-icon">{{ level.icon }}</div>
            <h3>{{ level.label }}</h3>
            <p>{{ level.desc }}</p>
            <div class="level-stats">
              <span>{{ level.count }}道题</span>
            </div>
            <div v-if="level.locked" class="level-lock">
              <el-icon :size="20"><Lock /></el-icon>
              <span>未解锁</span>
            </div>
          </el-card>
        </el-col>
      </el-row>

      <el-card class="practice-entry">
        <h3>开始刷题</h3>
        <p class="entry-tip">系统将根据您的等级自动抽取题目，可按题型筛选</p>
        <el-form :inline="true" :model="practiceForm" class="entry-form">
          <el-form-item label="题型">
            <el-select v-model="practiceForm.type" placeholder="全部题型" clearable style="width: 160px">
              <el-option label="单选题" value="single_choice" />
              <el-option label="多选题" value="multi_choice" />
              <el-option label="判断题" value="true_false" />
              <el-option label="故障识图" value="fault_image" />
              <el-option label="简答题" value="short_answer" />
            </el-select>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" size="large" :loading="loading" @click="startPractice">
              开始刷题
            </el-button>
          </el-form-item>
        </el-form>
      </el-card>

      <el-row :gutter="20" class="quick-links" v-if="userLevel !== 'expert'">
        <el-col :span="24">
          <el-button type="warning" size="large" @click="$router.push('/training/wrong-questions')" style="width:100%">
            错题本
          </el-button>
        </el-col>
      </el-row>
    </div>

    <!-- 刷题中 -->
    <div v-if="practiceStarted && !practiceFinished" class="practice-area">
      <div class="practice-toolbar">
        <div class="progress-text">
          第 {{ currentIdx + 1 }}/{{ questions.length }} 题
          <span class="progress-stats">已答对 {{ correctCount }} · 已答错 {{ wrongCount }}</span>
        </div>
        <el-button size="small" @click="confirmQuit">退出练习</el-button>
      </div>

      <el-card class="question-card">
        <div class="question-header">
          <el-tag size="small">{{ typeMap[currentQuestion.type] || '题目' }}</el-tag>
          <el-tag size="small" type="info">{{ levelLabelMap[currentQuestion.level] || '其他' }}</el-tag>
        </div>
        <img v-if="currentQuestion.image_url" :src="currentQuestion.image_url" class="q-image" />
        <p class="q-content">{{ currentQuestion.content }}</p>

        <div v-if="currentQuestion.type !== 'short_answer'" class="q-options">
          <template v-if="currentQuestion.type === 'true_false'">
            <div v-for="opt in [{ key: '对', label: '正确' }, { key: '错', label: '错误' }]" :key="opt.key"
                 class="q-option"
                 :class="optionClass(opt.key)"
                 @click="!submitted && toggleOption(opt.key)">
              <span class="opt-label">{{ opt.key }}</span>
              <span>{{ opt.label }}</span>
            </div>
          </template>
          <template v-else>
            <div v-for="(label, key) in currentQuestion.options" :key="key"
                 class="q-option"
                 :class="optionClass(key)"
                 @click="!submitted && toggleOption(key)">
              <span class="opt-label">{{ key }}</span>
              <span>{{ label }}</span>
            </div>
          </template>
        </div>
        <el-input v-else v-model="textAnswer" type="textarea" :rows="4" placeholder="请输入答案" :disabled="submitted" />

        <div class="q-feedback" v-if="submitted">
          <el-alert
            :title="lastResult.is_correct ? '回答正确' : '回答错误'"
            :type="lastResult.is_correct ? 'success' : 'error'"
            :closable="false"
            show-icon
          >
            <template v-if="!lastResult.is_correct">
              <div>正确答案：{{ lastResult.correct_answer }}</div>
            </template>
            <div v-if="lastResult.explanation" class="feedback-explanation">
              解析：{{ lastResult.explanation }}
            </div>
            <div v-if="lastResult.ai_score !== undefined && lastResult.ai_score !== null">
              AI 评分：{{ lastResult.ai_score }} 分 · {{ lastResult.ai_comment || '' }}
            </div>
          </el-alert>
        </div>

        <div class="q-actions">
          <el-button v-if="!submitted" type="primary" :disabled="!canSubmit" @click="submitAnswer">
            提交答案
          </el-button>
          <el-button v-else type="primary" @click="nextQuestion">
            {{ currentIdx === questions.length - 1 ? '查看结果' : '下一题' }}
          </el-button>
        </div>
      </el-card>
    </div>

    <!-- 刷题完成 -->
    <div v-if="practiceFinished" class="practice-result">
      <el-card>
        <h2>本次练习结果</h2>
        <div class="score-display">
          <div class="score-circle" :class="{ passed: accuracy >= 60 }">
            <span class="score-num">{{ correctCount }}</span>
            <span class="score-total">/{{ questions.length }}</span>
          </div>
          <p>正确率：{{ accuracy }}%</p>
          <p>答对 {{ correctCount }} 题 · 答错 {{ wrongCount }} 题</p>
        </div>
        <div class="result-actions">
          <el-button type="primary" @click="restartPractice">再来一组</el-button>
          <el-button @click="backToEntry">返回题库</el-button>
        </div>
      </el-card>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { Lock } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { questionBankApi } from '@/api/questionBank'
import { practiceModeApi } from '@/api/practiceMode'
import { useAuthStore } from '@/stores/auth'
import { useUserStore } from '@/stores/user'

const authStore = useAuthStore()
const userStore = useUserStore()

const levelLabelMap = { beginner: '初级', intermediate: '中级', advanced: '高级', expert: '顶级' }
const levelAllowedLevels = {
  beginner: ['beginner'],
  intermediate: ['beginner', 'intermediate'],
  advanced: ['beginner', 'intermediate', 'advanced']
}
const typeMap = {
  single_choice: '单选题',
  multi_choice: '多选题',
  true_false: '判断题',
  fault_image: '故障识图',
  short_answer: '简答题'
}

const userLevel = ref(authStore.userInfo?.level || 'beginner')

const levelTagType = computed(() => {
  const map = { beginner: 'success', intermediate: 'warning', advanced: 'danger', expert: '' }
  return map[userLevel.value] || 'success'
})

const levelHint = computed(() => {
  if (userLevel.value === 'expert') return '您已达到最高等级，可自由刷题'
  const hints = {
    beginner: '刷题范围：初级题库，每次10道',
    intermediate: '刷题范围：初级+中级题库，每次20道',
    advanced: '刷题范围：全部题库，每次30道'
  }
  return hints[userLevel.value] || hints.beginner
})

const levels = ref([
  { value: 'beginner', label: '初级学徒', icon: '🟢', desc: '叉车基础操作与安全规范', count: 0 },
  { value: 'intermediate', label: '中级学徒', icon: '🟡', desc: '故障诊断与维修技能', count: 0 },
  { value: 'advanced', label: '高级学徒', icon: '🔴', desc: '高级维修与教学能力', count: 0 }
])

const visibleLevels = computed(() => {
  const allowed = levelAllowedLevels[userLevel.value] || ['beginner']
  return levels.value.map(l => ({
    ...l,
    locked: !allowed.includes(l.value)
  }))
})

// ===== 刷题流程 =====
const loading = ref(false)
const practiceStarted = ref(false)
const practiceFinished = ref(false)
const practiceForm = ref({ type: '' })

const questions = ref([])
const currentIdx = ref(0)
const answers = ref<any>({})
const textAnswer = ref('')
const submitted = ref(false)
const lastResult = ref<any>({})
const correctCount = ref(0)
const wrongCount = ref(0)

const currentQuestion = computed(() => questions.value[currentIdx.value] || {})
const canSubmit = computed(() => {
  const q = currentQuestion.value
  if (!q || !q.id) return false
  if (q.type === 'short_answer') return textAnswer.value.trim() !== ''
  const ans = answers.value[q.id]
  if (ans === undefined || ans === null) return false
  if (Array.isArray(ans)) return ans.length > 0
  return ans !== ''
})
const accuracy = computed(() => {
  if (questions.value.length === 0) return 0
  return Math.round((correctCount.value / questions.value.length) * 100)
})

onMounted(async () => {
  if (!authStore.userInfo?.level) {
    try {
      await userStore.fetchProfile()
      userLevel.value = userStore.profile?.level || 'beginner'
    } catch (e) {}
  }
  await loadStats()
})

async function loadStats() {
  try {
    const res = await questionBankApi.getStats()
    if (res.data) {
      const stats = res.data
      levels.value[0].count = stats.by_level?.beginner || 0
      levels.value[1].count = stats.by_level?.intermediate || 0
      levels.value[2].count = stats.by_level?.advanced || 0
    }
  } catch (e) {}
}

async function startPractice() {
  loading.value = true
  try {
    const params: Record<string, any> = {}
    if (practiceForm.value.type) params.type = practiceForm.value.type
    const res = await practiceModeApi.getFreeQuestions(params)
    questions.value = res.data || []
    if (questions.value.length === 0) {
      ElMessage.warning('暂无符合条件的题目')
      return
    }
    currentIdx.value = 0
    answers.value = {}
    textAnswer.value = ''
    submitted.value = false
    lastResult.value = {}
    correctCount.value = 0
    wrongCount.value = 0
    practiceStarted.value = true
    practiceFinished.value = false
  } catch (e) {
    ElMessage.error(e.message || '加载题目失败')
  } finally {
    loading.value = false
  }
}

function isOptionSelected(key) {
  const q = currentQuestion.value
  const ans = answers.value[q.id]
  if (!ans) return false
  if (q.type === 'multi_choice') return Array.isArray(ans) && ans.includes(key)
  return ans === key
}

function toggleOption(key) {
  const q = currentQuestion.value
  if (q.type === 'multi_choice') {
    if (!answers.value[q.id]) answers.value[q.id] = []
    const idx = answers.value[q.id].indexOf(key)
    if (idx > -1) answers.value[q.id].splice(idx, 1)
    else answers.value[q.id].push(key)
  } else {
    answers.value[q.id] = key
  }
}

function optionClass(key) {
  if (!submitted.value) {
    return { selected: isOptionSelected(key) }
  }
  const q = currentQuestion.value
  const correctArr = Array.isArray(lastResult.value.correct_answer)
    ? lastResult.value.correct_answer
    : String(lastResult.value.correct_answer || '').split(',')
  const userAns = answers.value[q.id]
  const userArr = Array.isArray(userAns) ? userAns : [userAns]
  const isCorrectOpt = correctArr.map(String).includes(String(key))
  const isUserOpt = userArr.map(String).includes(String(key))
  return {
    selected: isUserOpt,
    'opt-correct': isCorrectOpt,
    'opt-wrong': isUserOpt && !isCorrectOpt
  }
}

async function submitAnswer() {
  const q = currentQuestion.value
  if (!canSubmit.value) return
  const userAnswer = q.type === 'short_answer' ? textAnswer.value : answers.value[q.id]
  try {
    const res = await practiceModeApi.submitAnswer({
      question_id: q.id,
      user_answer: userAnswer,
      practice_type: 'free'
    })
    lastResult.value = res.data || {}
    submitted.value = true
    if (lastResult.value.is_correct) {
      correctCount.value++
    } else {
      wrongCount.value++
    }
  } catch (e) {
    ElMessage.error(e.message || '提交答案失败')
  }
}

function nextQuestion() {
  if (currentIdx.value === questions.value.length - 1) {
    practiceFinished.value = true
    return
  }
  currentIdx.value++
  textAnswer.value = ''
  submitted.value = false
  lastResult.value = {}
}

async function confirmQuit() {
  try {
    await ElMessageBox.confirm('确定要退出本次练习吗？已答题目不会保存进度。', '提示', { type: 'warning' })
    resetPractice()
  } catch (e) {}
}

function resetPractice() {
  practiceStarted.value = false
  practiceFinished.value = false
  questions.value = []
  currentIdx.value = 0
  answers.value = {}
  textAnswer.value = ''
  submitted.value = false
  lastResult.value = {}
  correctCount.value = 0
  wrongCount.value = 0
}

function restartPractice() {
  resetPractice()
  startPractice()
}

function backToEntry() {
  resetPractice()
}
</script>

<style scoped>
.question-bank { max-width: 1200px; margin: 0 auto; }
.question-bank h2 { margin-bottom: 10px; color: #303133; }
.user-level-badge { display: flex; align-items: center; gap: 12px; margin-bottom: 20px; }
.level-hint { color: #909399; font-size: 14px; }
.level-cards { margin-bottom: 30px; }
.level-card { text-align: center; transition: transform 0.3s; margin-bottom: 15px; position: relative; }
.level-card:hover { transform: translateY(-5px); }
.level-card.level-locked { opacity: 0.6; }
.level-icon { font-size: 48px; margin-bottom: 10px; }
.level-card h3 { margin: 10px 0; }
.level-card p { color: #909399; font-size: 14px; }
.level-stats { margin-top: 10px; color: #409eff; font-weight: bold; }
.level-lock { position: absolute; top: 10px; right: 10px; display: flex; align-items: center; gap: 4px; color: #909399; font-size: 12px; }
.level-beginner { border-top: 3px solid #67c23a; }
.level-intermediate { border-top: 3px solid #e6a23c; }
.level-advanced { border-top: 3px solid #f56c6c; }

.practice-entry { margin-bottom: 20px; text-align: center; }
.practice-entry h3 { margin: 0 0 8px; color: #303133; }
.entry-tip { color: #909399; font-size: 14px; margin-bottom: 16px; }
.entry-form { display: flex; justify-content: center; gap: 12px; }
.quick-links { margin-top: 20px; }
.quick-links .el-col { margin-bottom: 10px; }

.practice-area { margin-top: 10px; }
.practice-toolbar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 15px; padding: 10px 15px; background: #fff; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
.progress-text { font-size: 15px; color: #303133; }
.progress-stats { margin-left: 12px; color: #909399; font-size: 13px; }
.question-card { margin-bottom: 15px; }
.question-header { display: flex; gap: 8px; align-items: center; margin-bottom: 15px; }
.q-image { max-width: 100%; max-height: 250px; border-radius: 8px; margin-bottom: 10px; }
.q-content { font-size: 16px; line-height: 1.8; margin-bottom: 15px; white-space: pre-wrap; }
.q-options { display: flex; flex-direction: column; gap: 8px; }
.q-option { display: flex; align-items: center; padding: 10px 15px; border: 1px solid #dcdfe6; border-radius: 8px; cursor: pointer; transition: all 0.2s; }
.q-option:hover { border-color: #409eff; }
.q-option.selected { border-color: #409eff; background: #ecf5ff; }
.q-option.opt-correct { border-color: #67c23a; background: #f0f9eb; }
.q-option.opt-wrong { border-color: #f56c6c; background: #fef0f0; }
.opt-label { width: 28px; height: 28px; line-height: 28px; text-align: center; border-radius: 50%; background: #f5f7fa; margin-right: 10px; font-weight: bold; }
.q-feedback { margin-top: 15px; }
.feedback-explanation { margin-top: 6px; color: #606266; }
.q-actions { display: flex; justify-content: center; margin-top: 20px; }

.practice-result { margin-top: 20px; }
.practice-result h2 { text-align: center; margin-bottom: 20px; }
.score-display { text-align: center; margin: 20px 0; }
.score-circle { display: inline-flex; flex-direction: column; align-items: center; justify-content: center; width: 150px; height: 150px; border-radius: 50%; border: 6px solid #f56c6c; margin-bottom: 10px; }
.score-circle.passed { border-color: #67c23a; }
.score-num { font-size: 36px; font-weight: bold; }
.score-total { font-size: 14px; color: #909399; }
.result-actions { display: flex; justify-content: center; gap: 12px; margin-top: 20px; }
</style>
