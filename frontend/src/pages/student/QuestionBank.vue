<template>
  <div class="question-bank">
    <!-- ===== 入口：5 卡片 ===== -->
    <div v-if="!mode" class="entry">
      <h2>题库练习</h2>
      <p class="entry-sub">选择练习方式，开始刷题</p>

      <el-row :gutter="20" class="card-grid">
        <!-- 顺序练习 -->
        <el-col :xs="24" :sm="12" :md="8">
          <el-card shadow="hover" class="practice-card card-sequential">
            <div class="card-head">
              <el-icon :size="28" color="#409eff"><Sort /></el-icon>
              <h3>顺序练习</h3>
            </div>
            <div class="card-stat">
              <span class="stat-num">{{ seqProgress.completed }}/{{ seqProgress.total || totalQuestions }}</span>
              <span class="stat-label">已练习/总题数</span>
            </div>
            <el-button type="primary" @click="startSequential">
              {{ seqProgress.completed > 0 ? '继续练习' : '开始练习' }}
            </el-button>
          </el-card>
        </el-col>

        <!-- 随机练习 -->
        <el-col :xs="24" :sm="12" :md="8">
          <el-card shadow="hover" class="practice-card card-random">
            <div class="card-head">
              <el-icon :size="28" color="#67c23a"><MagicStick /></el-icon>
              <h3>随机练习</h3>
            </div>
            <div class="card-select">
              <span class="select-label">每次题量</span>
              <el-select v-model="randomCount" size="small" style="width: 110px">
                <el-option v-for="o in randomCountOptions" :key="o.value" :label="o.label" :value="o.value" />
              </el-select>
            </div>
            <el-button type="success" :loading="loading" @click="startFree()">开始练习</el-button>
          </el-card>
        </el-col>

        <!-- 专项练习 -->
        <el-col :xs="24" :sm="12" :md="8">
          <el-card shadow="hover" class="practice-card card-special">
            <div class="card-head">
              <el-icon :size="28" color="#e6a23c"><Filter /></el-icon>
              <h3>专项练习</h3>
            </div>
            <div class="card-select">
              <span class="select-label">题型</span>
              <el-select v-model="specialType" size="small" placeholder="选择题型" style="width: 130px">
                <el-option v-for="o in questionTypeOptions" :key="o.value" :label="o.label" :value="o.value" />
              </el-select>
            </div>
            <el-button type="warning" :loading="loading" :disabled="!specialType" @click="startFree(specialType)">开始练习</el-button>
          </el-card>
        </el-col>

        <!-- 章节练习 -->
        <el-col :xs="24" :sm="12" :md="8">
          <el-card shadow="hover" class="practice-card card-chapter">
            <div class="card-head">
              <el-icon :size="28" color="#f56c6c"><Reading /></el-icon>
              <h3>章节练习</h3>
            </div>
            <div class="card-select">
              <span class="select-label">分类</span>
              <el-select v-model="chapterCategory" size="small" placeholder="选择分类" style="width: 130px">
                <el-option v-for="o in categoryOptions" :key="o.value" :label="o.label" :value="o.value" />
              </el-select>
            </div>
            <el-button type="danger" :loading="loading" :disabled="!chapterCategory" @click="startCategory">开始练习</el-button>
          </el-card>
        </el-col>

        <!-- 模拟考试 -->
        <el-col :xs="24" :sm="12" :md="8">
          <el-card shadow="hover" class="practice-card card-mock">
            <div class="card-head">
              <el-icon :size="28" color="#909399"><Document /></el-icon>
              <h3>模拟考试</h3>
            </div>
            <div class="card-stat">
              <template v-if="latestMockScore !== null">
                <span class="stat-num">{{ latestMockScore }}</span>
                <span class="stat-label">最近一次得分</span>
              </template>
              <template v-else>
                <span class="stat-num">—</span>
                <span class="stat-label">暂无考试记录</span>
              </template>
            </div>
            <el-button type="primary" @click="$router.push('/training/mock-exam')">进入模拟考试</el-button>
          </el-card>
        </el-col>
      </el-row>
    </div>

    <!-- ===== 刷题中 ===== -->
    <div v-if="mode" class="practice-area">
      <div class="practice-toolbar">
        <div class="progress-text">
          第 {{ currentIdx + 1 }}/{{ questions.length }} 题
          <span class="progress-stats">已答对 {{ correctCount }} · 已答错 {{ wrongCount }}</span>
          <el-tag v-if="mode === 'sequential'" size="small" type="primary" style="margin-left: 10px">顺序练习</el-tag>
          <el-tag v-else-if="mode === 'category'" size="small" type="danger" style="margin-left: 10px">
            {{ chapterCategory ? categoryMap[chapterCategory] : '章节练习' }}
          </el-tag>
          <el-tag v-else-if="specialType && mode === 'free'" size="small" type="warning" style="margin-left: 10px">
            {{ typeMap[specialType] }}
          </el-tag>
        </div>
        <el-button size="small" @click="confirmQuit">退出练习</el-button>
      </div>

      <el-card class="question-card">
        <div class="question-header">
          <el-tag size="small">{{ typeMap[currentQuestion.type] || '题目' }}</el-tag>
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
          <el-button v-if="currentIdx > 0" @click="prevQuestion">上一题</el-button>
          <el-button v-if="!submitted" type="primary" :disabled="!canSubmit" @click="submitAnswer">
            提交答案
          </el-button>
          <el-button v-if="currentIdx < questions.length - 1" type="primary" @click="nextQuestion">
            下一题
          </el-button>
          <el-button v-if="currentIdx === questions.length - 1" type="success" @click="nextQuestion">
            查看结果
          </el-button>
        </div>
      </el-card>
    </div>

    <!-- ===== 刷题完成 ===== -->
    <div v-if="showResult" class="practice-result">
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
          <el-button v-if="mode === 'sequential'" type="primary" @click="startSequential">继续顺序练习</el-button>
          <el-button v-else type="primary" @click="restartPractice">再来一组</el-button>
          <el-button @click="backToEntry">返回题库</el-button>
        </div>
      </el-card>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { Sort, MagicStick, Filter, Reading, Document } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { questionBankApi } from '@/api/questionBank'
import { practiceModeApi } from '@/api/practiceMode'
import { mockExamApi } from '@/api/mockExam'
import { typeMap, questionTypeOptions, categoryMap, categoryOptions, randomCountOptions } from '@/constants/question'
import type { CourseCategory, PracticeProgress, Question, QuestionType, SubmitResult } from '@/types/question'

// null = 入口；'sequential' | 'free' | 'category' = 刷题中
const mode = ref<'sequential' | 'free' | 'category' | null>(null)
const showResult = ref(false)

// 卡片选择器状态
const randomCount = ref(20)
const specialType = ref<QuestionType | ''>('')
const chapterCategory = ref<CourseCategory | ''>('')

// 卡片展示数据
const seqProgress = ref<PracticeProgress>({ completed: 0, total: 0, current_index: 0 })
const totalQuestions = ref(0)
const latestMockScore = ref<number | null>(null)

// ===== 刷题流程 =====
const loading = ref(false)
const questions = ref<Question[]>([])
const currentIdx = ref(0)
const answers = ref<Record<number, unknown>>({})
// 按题目ID存储每题的作答状态，切换上下题时保留状态
const textAnswerMap = ref<Record<number, string>>({})
const submittedMap = ref<Record<number, boolean>>({})
const resultMap = ref<Record<number, SubmitResult>>({})
const correctCount = ref(0)
const wrongCount = ref(0)

const currentQuestion = computed(() => questions.value[currentIdx.value] || ({} as Question))
// 当前题目的简答文本（v-model 双向绑定到 Map）
const textAnswer = computed({
  get: () => (currentQuestion.value.id ? textAnswerMap.value[currentQuestion.value.id] || '' : ''),
  set: (v: string) => {
    if (currentQuestion.value.id) textAnswerMap.value[currentQuestion.value.id] = v
  }
})
// 当前题目是否已提交
const submitted = computed(() => (currentQuestion.value.id ? !!submittedMap.value[currentQuestion.value.id] : false))
// 当前题目的解析结果
const lastResult = computed(() => (currentQuestion.value.id ? resultMap.value[currentQuestion.value.id] || ({} as SubmitResult) : ({} as SubmitResult)))
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

onMounted(() => {
  loadCardData()
})

async function loadCardData() {
  try {
    const [statsRes, progRes, histRes] = await Promise.all([
      questionBankApi.getStats(),
      practiceModeApi.getSequentialProgress(),
      mockExamApi.getMockExamHistory({ page: 1, page_size: 1 })
    ])
    totalQuestions.value = (statsRes.data?.total as number) || 0
    if (progRes.data) {
      seqProgress.value = progRes.data
    }
    const exams = histRes.data?.exams || []
    if (exams.length > 0 && exams[0].score != null) {
      latestMockScore.value = Number(exams[0].score)
    }
  } catch (e) {
    // 静默失败，卡片展示降级为默认值
  }
}

// ===== 开始各模式 =====
async function startSequential() {
  loading.value = true
  try {
    const res = await practiceModeApi.startSequential()
    const data = res.data || {}
    questions.value = data.questions || []
    if (questions.value.length === 0) {
      ElMessage.warning('题库暂无题目')
      return
    }
    mode.value = 'sequential'
    showResult.value = false
    // 获取持久化的答题状态
    const prog = await resolveProgress('sequential', questions.value.length)
    resetSession(prog.startIndex)
    restoreState(prog.answersState)
  } catch (e: any) {
    ElMessage.error(e.message || '加载题目失败')
  } finally {
    loading.value = false
  }
}

// 获取当前练习模式的进度 key（用于断点续练）
// 顺序练习: 'sequential'；专项练习: 'free:<type>'；章节练习: 'category:<category>'；随机练习: ''（不保存）
function getPracticeModeKey(): string {
  if (mode.value === 'sequential') return 'sequential'
  if (mode.value === 'free' && specialType.value) return `free:${specialType.value}`
  if (mode.value === 'category' && chapterCategory.value) return `category:${chapterCategory.value}`
  return ''
}

// 查询断点续练起始位置和持久化答题状态
async function resolveProgress(modeKey: string, total: number): Promise<{ startIndex: number; answersState: Record<string, unknown> }> {
  if (!modeKey) return { startIndex: 0, answersState: {} }
  try {
    const progRes = await practiceModeApi.getProgress(modeKey)
    const data = progRes.data || {}
    const idx = data.current_index || 0
    const startIndex = idx > 0 && idx < total ? idx : 0
    return { startIndex, answersState: data.answers_state || {} }
  } catch (e) {}
  return { startIndex: 0, answersState: {} }
}

// 从后端答题状态恢复 answers/submittedMap/resultMap/correctCount/wrongCount
function restoreState(answersState: Record<string, unknown>) {
  if (!answersState || Object.keys(answersState).length === 0) return
  const newAnswers: Record<number, unknown> = {}
  const newSubmittedMap: Record<number, boolean> = {}
  const newResultMap: Record<number, SubmitResult> = {}
  const newTextAnswerMap: Record<number, string> = {}
  let correct = 0
  let wrong = 0
  for (const [key, val] of Object.entries(answersState)) {
    const qid = Number(key)
    if (!qid) continue
    const result = val as SubmitResult
    newResultMap[qid] = result
    newSubmittedMap[qid] = true
    if (result.user_answer !== undefined && result.user_answer !== null) {
      newAnswers[qid] = result.user_answer
      if (typeof result.user_answer === 'string') {
        newTextAnswerMap[qid] = result.user_answer
      }
    }
    if (result.is_correct === true) correct++
    else if (result.is_correct === false) wrong++
  }
  answers.value = newAnswers
  submittedMap.value = newSubmittedMap
  resultMap.value = newResultMap
  textAnswerMap.value = newTextAnswerMap
  correctCount.value = correct
  wrongCount.value = wrong
}

// 构建可序列化的答题状态对象（key 为题目ID字符串）
function buildAnswersState(): Record<string, unknown> {
  const state: Record<string, unknown> = {}
  for (const [qid, result] of Object.entries(resultMap.value)) {
    state[qid] = result
  }
  return state
}

// 保存当前进度和答题状态到后端
async function saveCurrentProgress(index: number) {
  const modeKey = getPracticeModeKey()
  if (!modeKey) return
  try {
    await practiceModeApi.saveProgress(index, modeKey, questions.value.length, buildAnswersState())
    if (mode.value === 'sequential') {
      seqProgress.value.completed = index
    }
  } catch (e) {
    // 保存失败不阻断练习
  }
}

async function startFree(type?: string) {
  loading.value = true
  try {
    const params: Record<string, unknown> = {}
    let modeKey = ''
    if (type) {
      // 专项练习：返回该题型全部题目（按顺序），支持断点续练
      params.type = type
      params.count = 0
      modeKey = `free:${type}`
    } else {
      // 随机练习：随机抽取指定数量，不保存进度
      params.count = randomCount.value
    }
    const res = await practiceModeApi.getFreeQuestions(params)
    questions.value = res.data || []
    if (questions.value.length === 0) {
      ElMessage.warning('暂无符合条件的题目')
      return
    }
    mode.value = 'free'
    showResult.value = false
    const prog = await resolveProgress(modeKey, questions.value.length)
    resetSession(prog.startIndex)
    restoreState(prog.answersState)
  } catch (e: any) {
    ElMessage.error(e.message || '加载题目失败')
  } finally {
    loading.value = false
  }
}

async function startCategory() {
  if (!chapterCategory.value) return
  loading.value = true
  try {
    const modeKey = `category:${chapterCategory.value}`
    const res = await practiceModeApi.getCategoryQuestions({ category: chapterCategory.value, count: 0 })
    questions.value = res.data || []
    if (questions.value.length === 0) {
      ElMessage.warning('该分类下暂无题目')
      return
    }
    mode.value = 'category'
    showResult.value = false
    const prog = await resolveProgress(modeKey, questions.value.length)
    resetSession(prog.startIndex)
    restoreState(prog.answersState)
  } catch (e: any) {
    ElMessage.error(e.message || '加载题目失败')
  } finally {
    loading.value = false
  }
}

function resetSession(startIdx: number) {
  currentIdx.value = startIdx
  answers.value = {}
  textAnswerMap.value = {}
  submittedMap.value = {}
  resultMap.value = {}
  correctCount.value = 0
  wrongCount.value = 0
}

// ===== 答题交互 =====
function isOptionSelected(key: string) {
  const q = currentQuestion.value
  const ans = answers.value[q.id]
  if (!ans) return false
  if (q.type === 'multi_choice') return Array.isArray(ans) && ans.includes(key)
  return ans === key
}

function toggleOption(key: string) {
  const q = currentQuestion.value
  if (q.type === 'multi_choice') {
    if (!answers.value[q.id]) answers.value[q.id] = []
    const arr = answers.value[q.id] as string[]
    const idx = arr.indexOf(key)
    if (idx > -1) arr.splice(idx, 1)
    else arr.push(key)
  } else {
    answers.value[q.id] = key
  }
}

function optionClass(key: string) {
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
      practice_type: mode.value || 'free'
    })
    resultMap.value[q.id] = res.data || {}
    submittedMap.value[q.id] = true
    if (resultMap.value[q.id].is_correct) {
      correctCount.value++
    } else {
      wrongCount.value++
    }
    // 提交后持久化答题状态（游标不变，仅更新 answers_state）
    await saveCurrentProgress(currentIdx.value)
  } catch (e: any) {
    ElMessage.error(e.message || '提交答案失败')
  }
}

async function nextQuestion() {
  // 最后一题：直接查看结果，不再推进游标
  if (currentIdx.value === questions.value.length - 1) {
    showResult.value = true
    return
  }
  // 所有有断点的模式：推进游标并保存进度+答题状态
  const newIndex = currentIdx.value + 1
  currentIdx.value++
  await saveCurrentProgress(newIndex)
}

// 上一题：回到上一题，状态由 Map 自动恢复（进度不回退）
function prevQuestion() {
  if (currentIdx.value > 0) {
    currentIdx.value--
  }
}

async function confirmQuit() {
  try {
    await ElMessageBox.confirm('确定要退出本次练习吗？', '提示', { type: 'warning' })
    // 所有有断点的模式：退出时保存当前游标和答题状态
    await saveCurrentProgress(currentIdx.value)
    backToEntry()
  } catch (e) {}
}

function backToEntry() {
  mode.value = null
  showResult.value = false
  questions.value = []
  currentIdx.value = 0
  answers.value = {}
  textAnswerMap.value = {}
  submittedMap.value = {}
  resultMap.value = {}
  correctCount.value = 0
  wrongCount.value = 0
  loadCardData()
}

function restartPractice() {
  if (mode.value === 'free' && specialType.value) {
    startFree(specialType.value)
  } else if (mode.value === 'category') {
    startCategory()
  } else {
    startFree()
  }
}
</script>

<style scoped>
.question-bank { max-width: 1200px; margin: 0 auto; }
.question-bank h2 { margin-bottom: 6px; color: #303133; }
.entry-sub { color: #909399; font-size: 14px; margin-bottom: 24px; }

.card-grid { margin-bottom: 20px; }
.practice-card { display: flex; flex-direction: column; align-items: center; text-align: center; min-height: 220px; transition: transform 0.3s; margin-bottom: 20px; }
.practice-card:hover { transform: translateY(-5px); }
.practice-card :deep(.el-card__body) { display: flex; flex-direction: column; align-items: center; justify-content: space-between; width: 100%; height: 100%; min-height: 188px; padding: 24px; box-sizing: border-box; }
.card-head { display: flex; flex-direction: column; align-items: center; gap: 8px; margin-bottom: 14px; }
.card-head h3 { margin: 0; color: #303133; }
.card-stat { display: flex; flex-direction: column; align-items: center; margin-bottom: 14px; }
.stat-num { font-size: 24px; font-weight: bold; color: #409eff; }
.stat-label { font-size: 12px; color: #909399; margin-top: 4px; }
.card-select { display: flex; align-items: center; gap: 8px; margin-bottom: 14px; }
.select-label { font-size: 13px; color: #606266; white-space: nowrap; }
.card-sequential { border-top: 3px solid #409eff; }
.card-random { border-top: 3px solid #67c23a; }
.card-special { border-top: 3px solid #e6a23c; }
.card-chapter { border-top: 3px solid #f56c6c; }
.card-mock { border-top: 3px solid #909399; }

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
