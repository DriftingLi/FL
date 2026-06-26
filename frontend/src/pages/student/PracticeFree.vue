<template>
  <div class="practice-free">
    <div class="practice-header">
      <el-button @click="$router.back()" :icon="ArrowLeft">返回</el-button>
      <h3>自由刷题 - {{ levelLabel }}</h3>
      <el-tag>第 {{ currentIndex + 1 }}/{{ questions.length }} 题</el-tag>
    </div>

    <div v-if="questions.length > 0" class="question-area">
      <el-card class="question-card">
        <div class="question-type">
          <el-tag :type="typeTagMap[currentQuestion.type]">{{ typeLabelMap[currentQuestion.type] }}</el-tag>
        </div>
        <div class="question-content">
          <img v-if="currentQuestion.image_url" :src="currentQuestion.image_url" class="question-image" />
          <p class="question-text">{{ currentQuestion.content }}</p>
        </div>

        <div class="options-area" v-if="currentQuestion.type !== 'short_answer'">
          <template v-if="currentQuestion.type === 'true_false'">
            <div v-for="opt in [{ key: '对', label: '正确' }, { key: '错', label: '错误' }]" :key="opt.key"
                 class="option-item"
                 :class="{
                   'selected': isSelected(opt.key),
                   'correct': showResult && isCorrectOption(opt.key),
                   'wrong': showResult && isSelected(opt.key) && !isCorrectOption(opt.key)
                 }"
                 @click="selectOption(opt.key)">
              <span class="option-label">{{ opt.key }}</span>
              <span class="option-text">{{ opt.label }}</span>
              <el-icon v-if="showResult && isCorrectOption(opt.key)" class="result-icon correct"><Check /></el-icon>
              <el-icon v-if="showResult && isSelected(opt.key) && !isCorrectOption(opt.key)" class="result-icon wrong"><Close /></el-icon>
            </div>
          </template>
          <template v-else>
            <div v-for="(label, key) in currentQuestion.options" :key="key" class="option-item"
                 :class="{
                   'selected': isSelected(key),
                   'correct': showResult && isCorrectOption(key),
                   'wrong': showResult && isSelected(key) && !isCorrectOption(key)
                 }"
                 @click="selectOption(key)">
              <span class="option-label">{{ key }}</span>
              <span class="option-text">{{ label }}</span>
              <el-icon v-if="showResult && isCorrectOption(key)" class="result-icon correct"><Check /></el-icon>
              <el-icon v-if="showResult && isSelected(key) && !isCorrectOption(key)" class="result-icon wrong"><Close /></el-icon>
            </div>
          </template>
        </div>

        <div v-if="currentQuestion.type === 'short_answer'" class="short-answer-area">
          <el-input v-model="shortAnswer" type="textarea" :rows="4" placeholder="请输入您的答案" :disabled="showResult" />
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
        <el-button v-if="!showResult" type="primary" @click="submitAnswer" :disabled="!hasAnswer" :loading="submitting">
          {{ submitting ? (currentQuestion.type === 'short_answer' ? 'AI评分中...' : '提交中...') : '提交答案' }}
        </el-button>
        <el-button v-if="showResult" type="primary" @click="nextQuestion">
          {{ currentIndex < questions.length - 1 ? '下一题' : '完成练习' }}
        </el-button>
      </div>
    </div>

    <el-empty v-else description="暂无题目" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowLeft, Check, Close, Monitor } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { practiceModeApi } from '@/api/practiceMode'
import { useAuthStore } from '@/stores/auth'
import { useUserStore } from '@/stores/user'

const router = useRouter()
const authStore = useAuthStore()
const userStore = useUserStore()

const levelLabelMap = { beginner: '初级', intermediate: '中级', advanced: '高级' }
const typeLabelMap = { single_choice: '单选题', multi_choice: '多选题', true_false: '判断题', fault_image: '故障识图', short_answer: '简答题' }
const typeTagMap = { single_choice: '', multi_choice: 'warning', true_false: 'info', fault_image: 'danger', short_answer: 'success' }

const userLevel = ref(authStore.userInfo?.level || 'beginner')
const levelLabel = computed(() => levelLabelMap[userLevel.value] || '初级')
const questions = ref([])
const currentIndex = ref(0)
const selectedOptions = ref([])
const shortAnswer = ref('')
const showResult = ref(false)
const lastResult = ref(null)
const submitting = ref(false)

const currentQuestion = computed(() => questions.value[currentIndex.value] || {})
const hasAnswer = computed(() => {
  if (currentQuestion.value.type === 'short_answer') return !!shortAnswer.value.trim()
  return selectedOptions.value.length > 0
})

onMounted(async () => {
  if (!authStore.userInfo?.level) {
    try {
      await userStore.fetchProfile()
      userLevel.value = userStore.profile?.level || 'beginner'
    } catch (e) {}
  }

  try {
    const res = await practiceModeApi.getFreeQuestions({})
    questions.value = res.data || []
  } catch (e) {
    ElMessage.error('获取题目失败')
  }
})

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
      practice_type: 'free'
    })
    lastResult.value = res.data
    showResult.value = true
  } catch (e) {
    ElMessage.error(e.message || '提交失败')
  } finally {
    submitting.value = false
  }
}

function nextQuestion() {
  if (currentIndex.value < questions.value.length - 1) {
    currentIndex.value++
    selectedOptions.value = []
    shortAnswer.value = ''
    showResult.value = false
    lastResult.value = null
  } else {
    ElMessage.success('练习完成！')
    router.push('/question-bank')
  }
}
</script>

<style scoped>
.practice-free { max-width: 800px; margin: 0 auto; }
.practice-header { display: flex; align-items: center; gap: 15px; margin-bottom: 20px; }
.practice-header h3 { flex: 1; }
.question-card { margin-bottom: 15px; }
.question-type { margin-bottom: 15px; }
.question-image { max-width: 100%; max-height: 300px; border-radius: 8px; margin-bottom: 15px; }
.question-text { font-size: 16px; line-height: 1.8; }
.options-area { margin-top: 20px; }
.option-item { display: flex; align-items: center; padding: 12px 15px; margin-bottom: 8px; border: 1px solid #dcdfe6; border-radius: 8px; cursor: pointer; transition: all 0.3s; }
.option-item:hover { border-color: #409eff; background: #ecf5ff; }
.option-item.selected { border-color: #409eff; background: #ecf5ff; }
.option-item.correct { border-color: #67c23a; background: #f0f9eb; }
.option-item.wrong { border-color: #f56c6c; background: #fef0f0; }
.option-label { width: 30px; height: 30px; line-height: 30px; text-align: center; border-radius: 50%; background: #f5f7fa; margin-right: 12px; font-weight: bold; }
.option-text { flex: 1; }
.result-icon { margin-left: auto; }
.result-icon.correct { color: #67c23a; }
.result-icon.wrong { color: #f56c6c; }
.short-answer-area { margin-top: 15px; }
.result-card { margin-bottom: 15px; }
.result-status { display: flex; align-items: center; gap: 8px; font-size: 18px; font-weight: bold; margin-bottom: 10px; }
.result-status.correct { color: #67c23a; }
.result-status.wrong { color: #f56c6c; }
.correct-answer { margin-bottom: 10px; color: #67c23a; }
.explanation h4 { margin-bottom: 5px; color: #606266; }
.explanation p { color: #909399; line-height: 1.6; }
.action-bar { text-align: center; margin-top: 20px; }

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
</style>
