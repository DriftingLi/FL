<template>
  <div class="exam-page" v-loading="loading">
    <div v-if="!showResult && examData" class="exam-container">
      <div class="exam-header">
        <div class="header-left">
          <el-button text @click="goBack">
            <el-icon><ArrowLeft /></el-icon> 返回课程
          </el-button>
          <h2 class="exam-title">{{ examData.course_name }} - 在线考核</h2>
        </div>
        <div class="header-right">
          <div class="progress-info">
            答题进度: {{ answeredCount }} / {{ examData.total_questions }}
          </div>
          <el-progress
            :percentage="progressPercent"
            :stroke-width="8"
            :color="progressColor"
            style="width: 150px"
          />
        </div>
      </div>

      <div class="question-nav">
        <span class="nav-label">答题卡:</span>
        <div class="nav-dots">
          <button
            v-for="(q, index) in examData.questions"
            :key="q.question_id"
            class="nav-dot"
            :class="{
              'dot-answered': isAnswered(q.question_id),
              'dot-marked': isMarked(q.question_id),
              'dot-current': currentIndex === index
            }"
            @click="goToQuestion(index)"
            :title="`第${index + 1}题`"
          >
            {{ index + 1 }}
          </button>
        </div>
        <div class="nav-legend">
          <span><i class="legend-dot dot-answered"></i> 已答</span>
          <span><i class="legend-dot dot-marked"></i> 标记</span>
          <span><i class="legend-dot"></i> 未答</span>
        </div>
      </div>

      <div class="question-card">
        <div class="question-header">
          <span class="question-type-tag" :type="currentQuestion.type === 'multi_choice' ? 'warning' : ''">
            {{ currentQuestion.type === 'multi_choice' ? '多选题' : '单选题' }}
          </span>
          <span class="question-number">第 {{ currentIndex + 1 }} 题 / 共 {{ examData.total_questions }} 题</span>
          <span class="question-score-tag">
            {{ currentQuestion.type === 'multi_choice' ? examData.multi_score : examData.single_score }}分
          </span>
          <el-button
            text
            type="warning"
            size="small"
            @click="toggleMark(currentQuestion.question_id)"
          >
            <el-icon><Flag /></el-icon>
            {{ isMarked(currentQuestion.question_id) ? '取消标记' : '标记此题' }}
          </el-button>
        </div>

        <div class="question-body">
          <p class="question-text">{{ currentQuestion.question_text }}</p>

          <div class="options-list">
            <div
              v-for="(optionText, optionKey) in currentQuestion.options"
              :key="optionKey"
              class="option-item"
              :class="{ 'option-selected': isSelected(optionKey) }"
              @click="selectOption(optionKey)"
            >
              <div class="option-radio" :class="{ 'radio-checked': isSelected(optionKey) }">
                <span v-if="isSelected(optionKey)">
                  <el-icon v-if="currentQuestion.type === 'multi_choice'"><Check /></el-icon>
                  <span v-else class="radio-inner"></span>
                </span>
              </div>
              <span class="option-key">{{ optionKey }}</span>
              <span class="option-text">{{ optionText }}</span>
            </div>
          </div>
        </div>
      </div>

      <div class="exam-footer">
        <el-button
          size="large"
          :disabled="currentIndex === 0"
          @click="prevQuestion"
        >
          上一题
        </el-button>
        <el-button
          v-if="currentIndex < examData.questions.length - 1"
          size="large"
          type="primary"
          @click="nextQuestion"
        >
          下一题
        </el-button>
        <el-button
          v-else
          size="large"
          type="success"
          @click="confirmSubmit"
          :loading="submitting"
        >
          交卷
        </el-button>
      </div>
    </div>

    <div v-if="showResult && resultData" class="result-container">
      <div class="result-header">
        <el-button text @click="goBack" class="back-btn">
          <el-icon><ArrowLeft /></el-icon> 返回课程
        </el-button>
        <div class="score-circle" :class="getScoreClass(resultData.score, resultData.total_score)">
          <span class="score-number">{{ resultData.score }}</span>
          <span class="score-total">/ {{ resultData.total_score }}</span>
        </div>
        <h2 class="result-title">{{ getScoreTitle(resultData.score, resultData.total_score) }}</h2>
        <p class="result-desc">
          答对 {{ resultData.correct_count }} 题 / 共 {{ resultData.total_questions }} 题
        </p>
        <div class="result-actions">
          <el-button type="warning" @click="confirmRetake">
            重新答题
          </el-button>
        </div>
      </div>

      <div class="detail-list">
        <div
          v-for="(item, index) in resultData.details"
          :key="item.question_id"
          class="detail-item"
          :class="{ 'detail-correct': item.is_correct, 'detail-wrong': !item.is_correct }"
        >
          <div class="detail-question-header">
            <el-tag :type="item.type === 'multi_choice' ? 'warning' : ''" size="small">
              第 {{ index + 1 }} 题 ({{ item.type === 'multi_choice' ? '多选' : '单选' }})
            </el-tag>
            <el-tag :type="item.is_correct ? 'success' : 'danger'" size="small">
              {{ item.is_correct ? '正确' : '错误' }}
            </el-tag>
            <el-tag type="info" size="small">
              {{ item.score }}分 / {{ item.total_score }}分
            </el-tag>
          </div>
          <p class="detail-question-text">{{ item.question_text }}</p>

          <div class="detail-options">
            <div
              v-for="(optText, optKey) in getDetailOptions(item)"
              :key="optKey"
              class="detail-option"
              :class="{
                'option-correct': isCorrectOption(item, optKey),
                'option-wrong-select': isWrongSelectedOption(item, optKey),
                'option-user-select': isUserSelectedOption(item, optKey) && !isCorrectOption(item, optKey)
              }"
            >
              <span class="detail-option-key">{{ optKey }}</span>
              <span class="detail-option-text">{{ optText }}</span>
              <el-tag v-if="isCorrectOption(item, optKey)" type="success" size="small" class="option-tag">正确答案</el-tag>
              <el-tag v-if="isWrongSelectedOption(item, optKey)" type="danger" size="small" class="option-tag">你的选择</el-tag>
            </div>
          </div>

          <div v-if="item.explanation" class="detail-explanation">
            <el-icon><InfoFilled /></el-icon>
            <span>{{ item.explanation }}</span>
          </div>
        </div>
      </div>

      <div class="detail-footer">
        <el-button type="warning" @click="confirmRetake">
          重新答题
        </el-button>
        <el-button @click="goBack">返回课程</el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Flag, Check, InfoFilled } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { examApi } from '@/api/exam'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const submitting = ref(false)
const examData = ref(null)
const resultData = ref(null)
const showResult = ref(false)

const currentIndex = ref(0)
const answers = ref({})
const markedQuestions = ref(new Set())

const currentQuestion = computed(() => {
  if (!examData.value?.questions) return null
  return examData.value.questions[currentIndex.value]
})

const answeredCount = computed(() => {
  return Object.keys(answers.value).filter(key => {
    const val = answers.value[key]
    return val !== undefined && val !== null && val !== ''
  }).length
})

const progressPercent = computed(() => {
  if (!examData.value) return 0
  return Math.round((answeredCount.value / examData.value.total_questions) * 100)
})

const progressColor = computed(() => {
  const percent = progressPercent.value
  if (percent >= 80) return '#67c23a'
  if (percent >= 50) return '#e6a23c'
  return '#f56c6c'
})

function isAnswered(questionId) {
  const val = answers.value[String(questionId)]
  return val !== undefined && val !== null && val !== ''
}

function isMarked(questionId) {
  return markedQuestions.value.has(questionId)
}

function isSelected(optionKey) {
  const qId = String(currentQuestion.value?.question_id)
  const answer = answers.value[qId]

  if (currentQuestion.value?.type === 'multi_choice') {
    if (Array.isArray(answer)) {
      return answer.includes(optionKey)
    }
    return false
  }

  return answer === optionKey
}

function selectOption(optionKey) {
  const qId = String(currentQuestion.value.question_id)

  if (currentQuestion.value.type === 'multi_choice') {
    if (!Array.isArray(answers.value[qId])) {
      answers.value[qId] = []
    }
    const idx = answers.value[qId].indexOf(optionKey)
    if (idx > -1) {
      answers.value[qId].splice(idx, 1)
    } else {
      answers.value[qId].push(optionKey)
    }
  } else {
    answers.value[qId] = optionKey
  }
}

function toggleMark(questionId) {
  if (markedQuestions.value.has(questionId)) {
    markedQuestions.value.delete(questionId)
  } else {
    markedQuestions.value.add(questionId)
  }
}

function goToQuestion(index) {
  currentIndex.value = index
}

function prevQuestion() {
  if (currentIndex.value > 0) {
    currentIndex.value--
  }
}

function nextQuestion() {
  if (examData.value && currentIndex.value < examData.value.questions.length - 1) {
    currentIndex.value++
  }
}

function confirmSubmit() {
  const unanswered = examData.value.total_questions - answeredCount.value

  let message = '确定要交卷吗？'
  if (unanswered > 0) {
    message = `您还有 ${unanswered} 道题目未作答，确定要交卷吗？`
  }

  ElMessageBox.confirm(message, '交卷确认', {
    confirmButtonText: '确定交卷',
    cancelButtonText: '继续答题',
    type: 'warning'
  }).then(async () => {
    await submitExamAnswers()
  }).catch(() => {})
}

async function submitExamAnswers() {
  submitting.value = true
  try {
    const courseId = route.params.courseId
    const res = await examApi.submitExam(courseId, answers.value)

    if (res.code === 200) {
      resultData.value = res.data
      showResult.value = true
      ElMessage.success('交卷成功！')
    }
  } catch (error) {
    console.error('提交试卷失败:', error)
    ElMessage.error('提交试卷失败，请稍后重试')
  } finally {
    submitting.value = false
  }
}

function confirmRetake() {
  ElMessageBox.confirm('重新答题将覆盖之前的成绩记录，确定要重新答题吗？', '重新答题确认', {
    confirmButtonText: '确定重新答题',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(() => {
    retakeExam()
  }).catch(() => {})
}

function retakeExam() {
  resultData.value = null
  showResult.value = false
  answers.value = {}
  markedQuestions.value = new Set()
  currentIndex.value = 0
}

function getDetailOptions(item) {
  if (item.options && typeof item.options === 'object') {
    return item.options
  }
  return {}
}

function isCorrectOption(item, optKey) {
  const correct = item.correct_answer
  if (Array.isArray(correct)) {
    return correct.includes(optKey)
  }
  return correct === optKey
}

function isUserSelectedOption(item, optKey) {
  const userAns = item.user_answer
  if (Array.isArray(userAns)) {
    return userAns.includes(optKey)
  }
  return userAns === optKey
}

function isWrongSelectedOption(item, optKey) {
  return isUserSelectedOption(item, optKey) && !isCorrectOption(item, optKey)
}

async function loadExamQuestions() {
  loading.value = true
  try {
    const courseId = route.params.courseId
    const res = await examApi.getExamQuestions(courseId)

    if (res.code === 200) {
      examData.value = res.data

      try {
        const historyRes = await examApi.getExamResult(courseId)
        if (historyRes.code === 200 && historyRes.data) {
          resultData.value = historyRes.data
          showResult.value = true
        }
      } catch {
        // 没有历史考核记录，正常情况
      }
    }
  } catch (error) {
    console.error('加载试卷失败:', error)
    ElMessage.error('加载试卷失败')
  } finally {
    loading.value = false
  }
}

function getScoreClass(score, total) {
  const percent = (score / total) * 100
  if (percent >= 80) return 'score-excellent'
  if (percent >= 60) return 'score-pass'
  return 'score-fail'
}

function getScoreTitle(score, total) {
  const percent = (score / total) * 100
  if (percent >= 90) return '优秀！'
  if (percent >= 80) return '良好！'
  if (percent >= 60) return '及格！'
  return '需要继续努力！'
}

function formatAnswer(answer) {
  if (Array.isArray(answer)) {
    return answer.join(', ')
  }
  return answer || ''
}

function goBack() {
  router.push(`/course/${route.params.courseId}`)
}

onMounted(() => {
  loadExamQuestions()
})
</script>

<style scoped>
.exam-page {
  padding: 20px;
  max-width: 900px;
  margin: 0 auto;
  min-height: calc(100vh - 120px);
}

.exam-container {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.exam-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: #fff;
  padding: 16px 24px;
  border-radius: 12px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
}

.header-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.exam-title {
  font-size: 18px;
  font-weight: 600;
  color: #303133;
  margin: 0;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 16px;
}

.progress-info {
  font-size: 14px;
  color: #606266;
  white-space: nowrap;
}

.question-nav {
  background: #fff;
  padding: 16px 24px;
  border-radius: 12px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
}

.nav-label {
  font-size: 14px;
  color: #606266;
  margin-right: 12px;
  font-weight: 500;
}

.nav-dots {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
}

.nav-dot {
  width: 32px;
  height: 32px;
  border-radius: 6px;
  border: 2px solid #dcdfe6;
  background: #fff;
  cursor: pointer;
  font-size: 13px;
  font-weight: 500;
  color: #606266;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  justify-content: center;
}

.nav-dot:hover {
  border-color: #409eff;
  color: #409eff;
}

.nav-dot.dot-answered {
  background: #409eff;
  border-color: #409eff;
  color: #fff;
}

.nav-dot.dot-marked {
  border-color: #e6a23c;
  background: #fdf6ec;
  color: #e6a23c;
}

.nav-dot.dot-marked.dot-answered {
  background: linear-gradient(135deg, #409eff 50%, #e6a23c 50%);
  border-color: #409eff;
  color: #fff;
}

.nav-dot.dot-current {
  box-shadow: 0 0 0 2px #409eff;
}

.nav-legend {
  display: flex;
  gap: 16px;
  font-size: 12px;
  color: #909399;
}

.legend-dot {
  display: inline-block;
  width: 14px;
  height: 14px;
  border-radius: 3px;
  border: 1.5px solid #dcdfe6;
  vertical-align: middle;
  margin-right: 4px;
}

.legend-dot.dot-answered {
  background: #409eff;
  border-color: #409eff;
}

.legend-dot.dot-marked {
  background: #fdf6ec;
  border-color: #e6a23c;
}

.question-card {
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
  overflow: hidden;
}

.question-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px 24px;
  background: #fafafa;
  border-bottom: 1px solid #ebeef5;
  flex-wrap: wrap;
}

.question-type-tag {
  font-size: 12px;
  padding: 4px 10px;
  border-radius: 4px;
  background: #409eff;
  color: #fff;
  font-weight: 500;
}

.question-type-tag[type="warning"] {
  background: #e6a23c;
}

.question-number {
  font-size: 14px;
  color: #909399;
  flex: 1;
}

.question-score-tag {
  font-size: 13px;
  padding: 2px 10px;
  border-radius: 4px;
  background: #f0f9eb;
  color: #67c23a;
  font-weight: 600;
  border: 1px solid #e1f3d8;
}

.question-body {
  padding: 24px;
}

.question-text {
  font-size: 16px;
  line-height: 1.8;
  color: #303133;
  margin-bottom: 24px;
  font-weight: 500;
}

.options-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.option-item {
  display: flex;
  align-items: center;
  padding: 14px 16px;
  border: 2px solid #e4e7ed;
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.2s ease;
  background: #fff;
  min-height: 52px;
}

.option-item:hover {
  border-color: #c6e2ff;
  background: #ecf5ff;
}

.option-item.option-selected {
  border-color: #409eff;
  background: #ecf5ff;
}

.option-radio {
  width: 22px;
  height: 22px;
  border: 2px solid #dcdfe6;
  border-radius: 50%;
  margin-right: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition: all 0.2s ease;
}

.option-selected .option-radio {
  border-color: #409eff;
  background: #409eff;
}

.radio-inner {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #fff;
}

.option-key {
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f5f7fa;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 600;
  color: #606266;
  margin-right: 12px;
  flex-shrink: 0;
}

.option-selected .option-key {
  background: #409eff;
  color: #fff;
}

.option-text {
  font-size: 15px;
  color: #303133;
  line-height: 1.5;
}

.exam-footer {
  display: flex;
  justify-content: center;
  gap: 16px;
  padding: 20px;
}

.result-container {
  background: #fff;
  border-radius: 12px;
  padding: 40px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
}

.result-header {
  text-align: center;
  margin-bottom: 32px;
  padding-bottom: 24px;
  border-bottom: 1px solid #ebeef5;
}

.back-btn {
  float: left;
  margin-bottom: 8px;
}

.score-circle {
  width: 160px;
  height: 160px;
  border-radius: 50%;
  display: inline-flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  margin-bottom: 20px;
}

.score-excellent {
  background: linear-gradient(135deg, #67c23a 0%, #85ce61 100%);
  box-shadow: 0 8px 24px rgba(103, 194, 58, 0.3);
}

.score-pass {
  background: linear-gradient(135deg, #409eff 0%, #66b1ff 100%);
  box-shadow: 0 8px 24px rgba(64, 158, 255, 0.3);
}

.score-fail {
  background: linear-gradient(135deg, #f56c6c 0%, #f78989 100%);
  box-shadow: 0 8px 24px rgba(245, 108, 108, 0.3);
}

.score-number {
  font-size: 48px;
  font-weight: 700;
  color: #fff;
  line-height: 1;
}

.score-total {
  font-size: 18px;
  color: rgba(255, 255, 255, 0.85);
  margin-top: 4px;
}

.result-title {
  font-size: 24px;
  font-weight: 600;
  color: #303133;
  margin: 0 0 8px;
}

.result-desc {
  font-size: 16px;
  color: #909399;
  margin: 0 0 16px;
}

.result-actions {
  display: flex;
  justify-content: center;
  gap: 16px;
}

.detail-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.detail-item {
  border: 2px solid #ebeef5;
  border-radius: 12px;
  padding: 20px;
  transition: all 0.2s ease;
}

.detail-correct {
  border-left: 4px solid #67c23a;
}

.detail-wrong {
  border-left: 4px solid #f56c6c;
}

.detail-question-header {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}

.detail-question-text {
  font-size: 15px;
  color: #303133;
  line-height: 1.6;
  margin: 0 0 16px;
  font-weight: 500;
}

.detail-options {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 16px;
}

.detail-option {
  display: flex;
  align-items: center;
  padding: 10px 14px;
  border: 1.5px solid #ebeef5;
  border-radius: 8px;
  background: #fafafa;
  gap: 10px;
}

.detail-option.option-correct {
  border-color: #67c23a;
  background: #f0f9eb;
}

.detail-option.option-wrong-select {
  border-color: #f56c6c;
  background: #fef0f0;
}

.detail-option.option-user-select {
  border-color: #e6a23c;
  background: #fdf6ec;
}

.detail-option-key {
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  font-size: 13px;
  font-weight: 600;
  background: #e4e7ed;
  color: #606266;
  flex-shrink: 0;
}

.option-correct .detail-option-key {
  background: #67c23a;
  color: #fff;
}

.option-wrong-select .detail-option-key {
  background: #f56c6c;
  color: #fff;
}

.option-user-select .detail-option-key {
  background: #e6a23c;
  color: #fff;
}

.detail-option-text {
  font-size: 14px;
  color: #303133;
  line-height: 1.5;
  flex: 1;
}

.option-tag {
  flex-shrink: 0;
}

.detail-explanation {
  display: flex;
  gap: 8px;
  padding: 12px 16px;
  background: #f4f4f5;
  border-radius: 8px;
  font-size: 14px;
  color: #606266;
  line-height: 1.6;
}

.detail-correct .detail-explanation {
  background: #f0f9eb;
  color: #67c23a;
}

.detail-wrong .detail-explanation {
  background: #fef0f0;
  color: #f56c6c;
}

.detail-footer {
  display: flex;
  justify-content: center;
  gap: 16px;
  margin-top: 24px;
  padding-top: 20px;
  border-top: 1px solid #ebeef5;
}

@media screen and (max-width: 768px) {
  .exam-page {
    padding: 12px;
  }

  .exam-header {
    flex-direction: column;
    gap: 12px;
    padding: 12px 16px;
    align-items: flex-start;
  }

  .header-left {
    gap: 8px;
    flex-wrap: wrap;
  }

  .exam-title {
    font-size: 16px;
  }

  .header-right {
    width: 100%;
    justify-content: space-between;
  }

  .question-nav {
    padding: 12px 16px;
  }

  .nav-dots {
    gap: 6px;
  }

  .nav-dot {
    width: 36px;
    height: 36px;
    font-size: 14px;
  }

  .question-header {
    padding: 12px 16px;
    gap: 8px;
  }

  .question-body {
    padding: 16px;
  }

  .question-text {
    font-size: 15px;
    margin-bottom: 16px;
  }

  .option-item {
    padding: 12px;
  }

  .exam-footer {
    padding: 16px;
    gap: 12px;
  }

  .exam-footer .el-button {
    flex: 1;
  }

  .result-container {
    padding: 20px;
  }

  .score-circle {
    width: 120px;
    height: 120px;
  }

  .score-number {
    font-size: 36px;
  }

  .score-total {
    font-size: 14px;
  }

  .result-title {
    font-size: 20px;
  }

  .detail-item {
    padding: 14px;
  }
}

@media screen and (max-width: 480px) {
  .exam-title {
    font-size: 14px;
  }

  .progress-info {
    font-size: 12px;
  }

  .question-text {
    font-size: 14px;
  }

  .option-text {
    font-size: 14px;
  }

  .result-container {
    padding: 16px;
  }

  .score-circle {
    width: 100px;
    height: 100px;
  }

  .score-number {
    font-size: 28px;
  }

  .result-title {
    font-size: 18px;
  }

  .result-desc {
    font-size: 14px;
  }
}
</style>
