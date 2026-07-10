<template>
  <div class="knowledge-practice">
    <div class="kp-sidebar">
      <div class="sidebar-header">
        <el-button @click="$router.push('/training/question-bank')" :icon="ArrowLeft" size="small">返回题库</el-button>
        <h4>知识点分类</h4>
      </div>

      <div class="level-filter">
        <el-radio-group v-model="selectedLevel" size="small" @change="loadProgress">
          <el-radio-button value="beginner">初级</el-radio-button>
          <el-radio-button value="intermediate">中级</el-radio-button>
          <el-radio-button value="advanced">高级</el-radio-button>
        </el-radio-group>
      </div>

      <div class="kp-tree" v-loading="loadingProgress">
        <div v-for="parent in filteredProgress" :key="parent.id" class="kp-parent-group">
          <div
            class="kp-parent-item"
            :class="{ active: selectedKpId === parent.id }"
            @click="selectKnowledgePoint(parent)"
          >
            <div class="kp-info">
              <el-icon class="expand-icon" :class="{ expanded: expandedParents.has(parent.id) }">
                <ArrowRight />
              </el-icon>
              <span class="kp-name">{{ parent.name }}</span>
            </div>
            <div class="kp-stats">
              <el-progress
                :percentage="parent.accuracy"
                :stroke-width="4"
                :show-text="false"
                :color="getProgressColor(parent.accuracy)"
                style="width: 40px"
              />
              <span class="kp-count">{{ parent.answered }}/{{ parent.total_questions }}</span>
            </div>
          </div>

          <div v-if="expandedParents.has(parent.id)" class="kp-children">
            <div
              v-for="child in parent.children"
              :key="child.id"
              class="kp-child-item"
              :class="{ active: selectedKpId === child.id }"
              @click="selectKnowledgePoint(child, parent)"
            >
              <span class="kp-name">{{ child.name }}</span>
              <div class="kp-stats">
                <el-progress
                  :percentage="child.accuracy"
                  :stroke-width="4"
                  :show-text="false"
                  :color="getProgressColor(child.accuracy)"
                  style="width: 40px"
                />
                <span class="kp-count">{{ child.answered }}/{{ child.total_questions }}</span>
              </div>
            </div>
          </div>
        </div>

        <el-empty v-if="filteredProgress.length === 0 && !loadingProgress" description="暂无知识点" :image-size="60" />
      </div>
    </div>

    <div class="kp-main">
      <div v-if="!selectedKpId" class="no-selection">
        <el-icon :size="64" color="#c0c4cc"><Collection /></el-icon>
        <p>请从左侧选择一个知识点开始练习</p>
      </div>

      <div v-else class="practice-content">
        <div class="practice-header">
          <div class="header-left">
            <h3>{{ selectedKpName }}</h3>
            <el-tag size="small" :type="levelTagType">{{ levelLabel }}</el-tag>
          </div>
          <div class="header-right">
            <span class="progress-text">
              进度：{{ answeredCount }}/{{ questions.length }}
            </span>
            <el-button-group>
              <el-button size="small" :type="isRandom ? 'primary' : ''" @click="toggleRandom">
                <el-icon><Sort /></el-icon>
                {{ isRandom ? '随机模式' : '顺序模式' }}
              </el-button>
              <el-button size="small" @click="refreshQuestions" :loading="loadingQuestions">
                <el-icon><Refresh /></el-icon>
                刷新
              </el-button>
            </el-button-group>
          </div>
        </div>

        <div class="question-nav-bar">
          <div class="nav-dots">
            <span
              v-for="(q, idx) in questions"
              :key="q.id"
              class="nav-dot"
              :class="{
                active: idx === currentIndex,
                answered: answeredMap[q.id],
                correct: answerStatusMap[q.id] === true,
                wrong: answerStatusMap[q.id] === false
              }"
              @click="goToQuestion(idx)"
              :title="`第${idx + 1}题`"
            >{{ idx + 1 }}</span>
          </div>
        </div>

        <div v-if="loadingQuestions" class="loading-area">
          <el-skeleton :rows="8" animated />
        </div>

        <div v-else-if="questions.length === 0" class="no-questions">
          <el-empty description="该知识点下暂无题目" />
        </div>

        <div v-else class="question-area">
          <el-card class="question-card">
            <div class="question-meta">
              <el-tag :type="typeTagMap[currentQuestion.type]" size="small">
                {{ typeLabelMap[currentQuestion.type] }}
              </el-tag>
              <el-tag
                :type="currentQuestion.level === 'beginner' ? 'success' : currentQuestion.level === 'intermediate' ? 'warning' : 'danger'"
                size="small"
                effect="plain"
              >
                {{ levelLabelMap[currentQuestion.level] }}
              </el-tag>
              <el-tag
                v-if="currentQuestion.kp_name && currentQuestion.kp_name !== selectedKpName"
                size="small"
                effect="plain"
                type="info"
              >
                {{ currentQuestion.kp_name }}
              </el-tag>
              <el-tag
                v-if="answeredMap[currentQuestion.id]"
                :type="answerStatusMap[currentQuestion.id] ? 'success' : 'danger'"
                size="small"
                effect="dark"
              >
                {{ answerStatusMap[currentQuestion.id] ? '已答对' : '已答错' }}
              </el-tag>
              <span class="question-index">第 {{ currentIndex + 1 }}/{{ questions.length }} 题</span>
            </div>

            <div class="question-content">
              <img v-if="currentQuestion.image_url" :src="currentQuestion.image_url" class="question-image" />
              <p class="question-text">{{ currentQuestion.content }}</p>
            </div>

            <div class="options-area" v-if="currentQuestion.type !== 'short_answer'">
              <template v-if="currentQuestion.type === 'true_false'">
                <div
                  v-for="opt in [{ key: '对', label: '正确' }, { key: '错', label: '错误' }]"
                  :key="opt.key"
                  class="option-item"
                  :class="{
                    selected: isSelected(opt.key),
                    correct: showResult && isCorrectOption(opt.key),
                    wrong: showResult && isSelected(opt.key) && !isCorrectOption(opt.key)
                  }"
                  @click="selectOption(opt.key)"
                >
                  <span class="option-label">{{ opt.key }}</span>
                  <span class="option-text">{{ opt.label }}</span>
                  <el-icon v-if="showResult && isCorrectOption(opt.key)" class="result-icon correct"><Check /></el-icon>
                  <el-icon v-if="showResult && isSelected(opt.key) && !isCorrectOption(opt.key)" class="result-icon wrong"><Close /></el-icon>
                </div>
              </template>
              <template v-else>
                <div
                  v-for="(label, key) in currentQuestion.options"
                  :key="key"
                  class="option-item"
                  :class="{
                    selected: isSelected(key),
                    correct: showResult && isCorrectOption(key),
                    wrong: showResult && isSelected(key) && !isCorrectOption(key)
                  }"
                  @click="selectOption(key)"
                >
                  <span class="option-label">{{ key }}</span>
                  <span class="option-text">{{ label }}</span>
                  <el-icon v-if="showResult && isCorrectOption(key)" class="result-icon correct"><Check /></el-icon>
                  <el-icon v-if="showResult && isSelected(key) && !isCorrectOption(key)" class="result-icon wrong"><Close /></el-icon>
                </div>
              </template>
            </div>

            <div v-if="currentQuestion.type === 'short_answer'" class="short-answer-area">
              <el-input
                v-model="shortAnswer"
                type="textarea"
                :rows="4"
                placeholder="请输入您的答案"
                :disabled="showResult"
              />
            </div>
          </el-card>

          <el-card v-if="showResult" class="result-card">
            <div v-if="currentQuestion.type === 'short_answer' && lastResult?.ai_score != null" class="ai-result-box">
              <div class="ai-result-header">
                <el-icon><Monitor /></el-icon>
                <span>AI 评分结果</span>
              </div>
              <div class="ai-result-score">
                <strong>{{ lastResult.ai_score }}</strong> / {{ lastResult.max_score || 10 }}分
              </div>
              <p v-if="lastResult.ai_comment" class="ai-result-comment">{{ lastResult.ai_comment }}</p>
              <div class="ai-result-status">
                <el-tag :type="lastResult.is_correct ? 'success' : 'danger'" size="small">
                  {{ lastResult.is_correct ? '合格' : '需改进' }}
                </el-tag>
              </div>
            </div>
            <div v-else-if="currentQuestion.type === 'short_answer'" class="result-status" style="color: #e6a23c;">
              <el-icon :size="24"><Monitor /></el-icon>
              <span>AI评分暂不可用，请参考下方答案</span>
            </div>
            <div v-else :class="['result-status', lastResult?.is_correct ? 'correct' : 'wrong']">
              <el-icon :size="24"><component :is="lastResult?.is_correct ? 'Check' : 'Close'" /></el-icon>
              <span>{{ lastResult?.is_correct ? '回答正确！' : '回答错误' }}</span>
            </div>
            <div v-if="currentQuestion.type === 'short_answer' && lastResult?.reference_answer" class="reference-answer">
              <h4>参考答案</h4>
              <p>{{ lastResult.reference_answer }}</p>
            </div>
            <div class="correct-answer" v-if="currentQuestion.type !== 'short_answer' && !lastResult?.is_correct">
              正确答案：<strong>{{ lastResult?.correct_answer }}</strong>
            </div>
            <div class="explanation" v-if="lastResult?.explanation">
              <h4>解析</h4>
              <p>{{ lastResult.explanation }}</p>
            </div>
          </el-card>

          <div class="action-bar">
            <el-button @click="prevQuestion" :disabled="currentIndex === 0">
              <el-icon><ArrowLeft /></el-icon> 上一题
            </el-button>
            <el-button v-if="!showResult" type="primary" @click="submitAnswer" :disabled="!hasAnswer" :loading="submitting">
              {{ submitting ? (currentQuestion.type === 'short_answer' ? 'AI评分中...' : '提交中...') : '提交答案' }}
            </el-button>
            <el-button v-if="showResult" type="primary" @click="nextQuestion">
              {{ currentIndex < questions.length - 1 ? '下一题' : '完成练习' }}
            </el-button>
            <el-button @click="nextQuestion" :disabled="currentIndex >= questions.length - 1">
              下一题 <el-icon><ArrowRight /></el-icon>
            </el-button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, reactive } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  ArrowLeft, ArrowRight, Check, Close, Collection,
  Sort, Refresh, Monitor
} from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { practiceModeApi } from '@/api/practiceMode'

const route = useRoute()
const router = useRouter()

const levelLabelMap = { beginner: '初级', intermediate: '中级', advanced: '高级' }
const typeLabelMap = {
  single_choice: '单选题', multi_choice: '多选题', true_false: '判断题',
  fault_image: '故障识图', short_answer: '简答题'
}
const typeTagMap = {
  single_choice: '', multi_choice: 'warning', true_false: 'info',
  fault_image: 'danger', short_answer: 'success'
}

const selectedLevel = ref<string | undefined>(route.query.level as string || 'beginner')
const levelLabel = computed(() => levelLabelMap[selectedLevel.value] || '初级')
const levelTagType = computed(() => {
  const map = { beginner: 'success', intermediate: 'warning', advanced: 'danger' }
  return map[selectedLevel.value] || ''
})

const progressData = ref([])
const loadingProgress = ref(false)
const expandedParents = ref(new Set())

const selectedKpId = ref(null)
const selectedKpName = ref('')
const selectedKpParentId = ref(null)

const questions = ref([])
const currentIndex = ref(0)
const loadingQuestions = ref(false)
const isRandom = ref(false)

const selectedOptions = ref([])
const shortAnswer = ref('')
const showResult = ref(false)
const lastResult = ref(null)
const submitting = ref(false)

const answeredMap = reactive<any>({})
const answerStatusMap = reactive<any>({})

const currentQuestion = computed(() => questions.value[currentIndex.value] || {})

const answeredCount = computed(() => {
  return questions.value.filter(q => answeredMap[q.id]).length
})

const hasAnswer = computed(() => {
  if (currentQuestion.value.type === 'short_answer') return !!shortAnswer.value.trim()
  return selectedOptions.value.length > 0
})

const filteredProgress = computed(() => {
  return progressData.value.filter(p => p.level === selectedLevel.value)
})

onMounted(() => {
  loadProgress()
})

async function loadProgress() {
  loadingProgress.value = true
  try {
    const res = await practiceModeApi.getKnowledgePointProgress()
    progressData.value = res.data || []
  } catch (e) {
    ElMessage.error('获取知识点进度失败')
  } finally {
    loadingProgress.value = false
  }
}

function selectKnowledgePoint(kp, parent?) {
  if (parent) {
    selectedKpId.value = kp.id
    selectedKpName.value = kp.name
    selectedKpParentId.value = parent.id
    if (!expandedParents.value.has(parent.id)) {
      const newSet = new Set(expandedParents.value)
      newSet.add(parent.id)
      expandedParents.value = newSet
    }
  } else {
    selectedKpId.value = kp.id
    selectedKpName.value = kp.name
    selectedKpParentId.value = null
    const newSet = new Set(expandedParents.value)
    if (newSet.has(kp.id)) {
      newSet.delete(kp.id)
    } else {
      newSet.add(kp.id)
    }
    expandedParents.value = newSet
  }
  loadQuestions()
}

async function loadQuestions() {
  if (!selectedKpId.value) return
  loadingQuestions.value = true
  resetAnswerState()
  try {
    const res = await practiceModeApi.getKnowledgePointPractice({
      knowledge_point_id: selectedKpId.value,
      random: isRandom.value
    })
    const data = res.data || {}
    questions.value = data.questions || []
    currentIndex.value = 0

    for (const q of questions.value) {
      if (q.answered) {
        answeredMap[q.id] = true
      }
    }
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '获取题目失败')
    questions.value = []
  } finally {
    loadingQuestions.value = false
  }
}

function toggleRandom() {
  isRandom.value = !isRandom.value
  if (questions.value.length > 0) {
    loadQuestions()
  }
}

function refreshQuestions() {
  loadQuestions()
  loadProgress()
}

function resetAnswerState() {
  selectedOptions.value = []
  shortAnswer.value = ''
  showResult.value = false
  lastResult.value = null
}

function goToQuestion(idx) {
  if (idx < 0 || idx >= questions.value.length) return
  currentIndex.value = idx
  resetAnswerState()
}

function prevQuestion() {
  if (currentIndex.value > 0) {
    currentIndex.value--
    resetAnswerState()
  }
}

function nextQuestion() {
  if (currentIndex.value < questions.value.length - 1) {
    currentIndex.value++
    resetAnswerState()
  } else {
    ElMessage.success('已完成该知识点所有题目！')
    loadProgress()
  }
}

function isSelected(key) {
  return selectedOptions.value.includes(key)
}

function isCorrectOption(key) {
  if (!lastResult.value) return false
  const correct = lastResult.value.correct_answer
  if (currentQuestion.value.type === 'multi_choice') {
    return correct.split(',').map(s => s.trim()).includes(key)
  }
  return correct === key
}

function selectOption(key) {
  if (showResult.value) return
  if (currentQuestion.value.type === 'multi_choice') {
    const idx = selectedOptions.value.indexOf(key)
    if (idx > -1) selectedOptions.value.splice(idx, 1)
    else selectedOptions.value.push(key)
  } else {
    selectedOptions.value = [key]
  }
}

async function submitAnswer() {
  submitting.value = true
  try {
    let answer
    if (currentQuestion.value.type === 'short_answer') {
      answer = shortAnswer.value
    } else if (currentQuestion.value.type === 'multi_choice') {
      answer = selectedOptions.value
    } else {
      answer = selectedOptions.value[0]
    }

    const res = await practiceModeApi.submitAnswer({
      question_id: currentQuestion.value.id,
      user_answer: answer,
      practice_type: 'knowledge_point'
    })
    lastResult.value = res.data
    showResult.value = true

    const qId = currentQuestion.value.id
    answeredMap[qId] = true
    answerStatusMap[qId] = res.data.is_correct
  } catch (e) {
    ElMessage.error(e.message || '提交失败')
  } finally {
    submitting.value = false
  }
}

function getProgressColor(accuracy) {
  if (accuracy >= 80) return '#67c23a'
  if (accuracy >= 60) return '#e6a23c'
  return '#f56c6c'
}
</script>

<style scoped>
.knowledge-practice {
  display: flex;
  gap: 20px;
  max-width: 1400px;
  margin: 0 auto;
  min-height: calc(100vh - 120px);
}

.kp-sidebar {
  width: 300px;
  flex-shrink: 0;
  background: #fff;
  border-radius: 8px;
  padding: 16px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
  overflow-y: auto;
  max-height: calc(100vh - 120px);
}

.sidebar-header {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 16px;
}

.sidebar-header h4 {
  margin: 0;
  flex: 1;
  color: #303133;
}

.level-filter {
  margin-bottom: 16px;
}

.level-filter .el-radio-group {
  width: 100%;
  display: flex;
}

.level-filter .el-radio-button {
  flex: 1;
}

.kp-tree {
  max-height: calc(100vh - 280px);
  overflow-y: auto;
}

.kp-parent-group {
  margin-bottom: 4px;
}

.kp-parent-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s;
}

.kp-parent-item:hover {
  background: #f5f7fa;
}

.kp-parent-item.active {
  background: #ecf5ff;
  border-left: 3px solid #409eff;
}

.kp-info {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: 1;
  min-width: 0;
}

.expand-icon {
  transition: transform 0.2s;
  font-size: 12px;
  color: #909399;
}

.expand-icon.expanded {
  transform: rotate(90deg);
}

.kp-name {
  font-size: 14px;
  color: #303133;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.kp-stats {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.kp-count {
  font-size: 12px;
  color: #909399;
  white-space: nowrap;
}

.kp-children {
  padding-left: 24px;
}

.kp-child-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s;
  border-left: 2px solid #e4e7ed;
}

.kp-child-item:hover {
  background: #f5f7fa;
}

.kp-child-item.active {
  background: #ecf5ff;
  border-left-color: #409eff;
}

.kp-child-item .kp-name {
  font-size: 13px;
  color: #606266;
}

.kp-main {
  flex: 1;
  min-width: 0;
}

.no-selection {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 400px;
  color: #c0c4cc;
}

.no-selection p {
  margin-top: 16px;
  font-size: 16px;
}

.practice-content {
  background: #fff;
  border-radius: 8px;
  padding: 20px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
}

.practice-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
  flex-wrap: wrap;
  gap: 10px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.header-left h3 {
  margin: 0;
  color: #303133;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.progress-text {
  font-size: 14px;
  color: #606266;
  white-space: nowrap;
}

.question-nav-bar {
  margin-bottom: 16px;
  padding: 12px;
  background: #fafafa;
  border-radius: 8px;
  overflow-x: auto;
}

.nav-dots {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.nav-dot {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: 6px;
  font-size: 12px;
  cursor: pointer;
  transition: all 0.2s;
  background: #fff;
  border: 1px solid #dcdfe6;
  color: #606266;
}

.nav-dot:hover {
  border-color: #409eff;
  color: #409eff;
}

.nav-dot.active {
  background: #409eff;
  border-color: #409eff;
  color: #fff;
}

.nav-dot.answered {
  background: #f0f9eb;
  border-color: #b3e19d;
  color: #67c23a;
}

.nav-dot.correct {
  background: #f0f9eb;
  border-color: #67c23a;
  color: #67c23a;
}

.nav-dot.wrong {
  background: #fef0f0;
  border-color: #f56c6c;
  color: #f56c6c;
}

.nav-dot.active.answered,
.nav-dot.active.correct,
.nav-dot.active.wrong {
  background: #409eff;
  border-color: #409eff;
  color: #fff;
}

.loading-area {
  padding: 40px;
}

.question-area {
  max-width: 800px;
}

.question-card {
  margin-bottom: 16px;
}

.question-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.question-index {
  margin-left: auto;
  font-size: 13px;
  color: #909399;
}

.question-image {
  max-width: 100%;
  max-height: 300px;
  border-radius: 8px;
  margin-bottom: 16px;
}

.question-text {
  font-size: 16px;
  line-height: 1.8;
  color: #303133;
}

.options-area {
  margin-top: 20px;
}

.option-item {
  display: flex;
  align-items: center;
  padding: 12px 15px;
  margin-bottom: 8px;
  border: 1px solid #dcdfe6;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.3s;
}

.option-item:hover {
  border-color: #409eff;
  background: #ecf5ff;
}

.option-item.selected {
  border-color: #409eff;
  background: #ecf5ff;
}

.option-item.correct {
  border-color: #67c23a;
  background: #f0f9eb;
}

.option-item.wrong {
  border-color: #f56c6c;
  background: #fef0f0;
}

.option-label {
  width: 30px;
  height: 30px;
  line-height: 30px;
  text-align: center;
  border-radius: 50%;
  background: #f5f7fa;
  margin-right: 12px;
  font-weight: bold;
  flex-shrink: 0;
}

.option-text {
  flex: 1;
}

.result-icon {
  margin-left: auto;
}

.result-icon.correct {
  color: #67c23a;
}

.result-icon.wrong {
  color: #f56c6c;
}

.short-answer-area {
  margin-top: 16px;
}

.result-card {
  margin-bottom: 16px;
}

.result-status {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 18px;
  font-weight: bold;
  margin-bottom: 10px;
}

.result-status.correct {
  color: #67c23a;
}

.result-status.wrong {
  color: #f56c6c;
}

.correct-answer {
  margin-bottom: 10px;
  color: #67c23a;
}

.explanation h4 {
  margin-bottom: 5px;
  color: #606266;
}

.explanation p {
  color: #909399;
  line-height: 1.6;
}

.ai-result-box {
  padding: 16px;
  background: linear-gradient(135deg, #f0f9eb 0%, #e1f3d8 100%);
  border: 1px solid #e1f3d8;
  border-radius: 8px;
  margin-bottom: 12px;
}

.ai-result-header {
  display: flex;
  align-items: center;
  gap: 6px;
  color: #67c23a;
  font-weight: 600;
  margin-bottom: 8px;
}

.ai-result-score { font-size: 18px; color: #409eff; margin-bottom: 6px; }
.ai-result-score strong { font-size: 24px; }
.ai-result-comment { color: #606266; font-size: 14px; line-height: 1.6; margin-bottom: 8px; }
.ai-result-status { margin-top: 4px; }
.reference-answer { margin-bottom: 10px; padding: 10px; background: #f5f7fa; border-radius: 6px; }
.reference-answer h4 { margin-bottom: 5px; color: #606266; }
.reference-answer p { color: #303133; line-height: 1.8; white-space: pre-wrap; }

.action-bar {
  display: flex;
  justify-content: center;
  gap: 12px;
  margin-top: 20px;
}

.no-questions {
  padding: 60px 0;
}

@media (max-width: 768px) {
  .knowledge-practice {
    flex-direction: column;
  }

  .kp-sidebar {
    width: 100%;
    max-height: 300px;
  }

  .practice-header {
    flex-direction: column;
    align-items: flex-start;
  }

  .header-right {
    width: 100%;
    justify-content: space-between;
  }
}
</style>
