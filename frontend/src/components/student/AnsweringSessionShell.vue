<template>
  <div class="exam-taking">
    <div class="exam-toolbar">
      <div v-if="title" class="exam-title">{{ title }}</div>
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
            <span>第 {{ currentIndex + 1 }}/{{ questions.length }} 题<span v-if="showScore">（{{ currentQ.score }}分）</span></span>
          </div>
          <img v-if="currentQ.image_url" :src="currentQ.image_url" class="q-image" loading="lazy" decoding="async" />
          <p class="q-content">{{ currentQ.content }}</p>
          <QuestionOptionPicker
            v-if="currentQ.type !== 'short_answer'"
            :options="currentOptions"
            :selected-keys="selectedOptionKeys"
            :multi-choice="currentQ.type === 'multi_choice'"
            @select="key => toggleOpt(currentQ.id, key, currentQ.type === 'multi_choice')"
          />
          <el-input v-else v-model="answers[currentQ.id]" type="textarea" :rows="4" placeholder="请输入答案" />
        </el-card>
        <div class="nav-buttons">
          <el-button @click="currentIndex--" :disabled="currentIndex === 0">上一题</el-button>
          <el-button @click="currentIndex++" :disabled="currentIndex === questions.length - 1">下一题</el-button>
        </div>
      </el-col>
      <el-col :xs="24" :md="6">
        <el-card class="answer-card">
          <h4>答题卡</h4>
          <div class="card-grid">
            <div v-for="(q, idx) in questions" :key="q.id"
                 class="card-item" :class="{ current: idx === currentIndex, answered: isAnswered(q.id) }"
                 @click="currentIndex = idx">{{ idx + 1 }}</div>
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { computed, watch } from 'vue'
import { Timer } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { Question } from '@/types/question'
import { typeMap } from '@/constants/question'
import { formatClock } from '@/utils/format'
import { buildQuestionOptions, isAnswerEmpty, toggleAnswer } from '@/composables/useQuestionAnswer'
import { useCountdown } from '@/composables/useCountdown'
import QuestionOptionPicker from '@/components/student/QuestionOptionPicker.vue'

// 答题会话壳：考试进行中的工具栏/倒计时/题目卡片/答题卡/交卷交互。
// 练习/考试共享同一交互形态；持久化与交卷后的流程由页面注入回调。
const props = withDefaults(
  defineProps<{
    questions: Question[]
    title?: string
    showScore?: boolean
  }>(),
  { title: '', showScore: false }
)

const answers = defineModel<Record<number, unknown>>('answers', { default: () => ({}) })
const remainingTime = defineModel<number>('remainingTime', { default: 0 })
const currentIndex = defineModel<number>('currentIndex', { default: 0 })

const emit = defineEmits<{
  autosave: []
  submit: [payload: { is_timeout: boolean; answers: Record<number, unknown>; remaining_time: number }]
}>()

const { remaining: timer, start: startTimer, stop: stopTimer } = useCountdown({
  autosaveInterval: 30,
  onAutosave: () => emit('autosave'),
  onExpire: autoSubmit
})

// 内部倒计时 → remainingTime 模型（页面提交/保存进度读取同一值）
watch(timer, (v) => {
  remainingTime.value = v
})

const currentQ = computed(() => props.questions[currentIndex.value] || {})
// 当前题目渲染用选项（判断题渲染对/错模板）
const currentOptions = computed(() => buildQuestionOptions(currentQ.value))
// 当前题目已选中的选项 keys
const selectedOptionKeys = computed(() => {
  const q = currentQ.value
  const ans = answers.value[q.id]
  if (ans === undefined || ans === null) return []
  if (q.type === 'multi_choice') return Array.isArray(ans) ? ans : []
  return [ans]
})

function toggleOpt(qid: number, key: string | number, multiChoice: boolean) {
  answers.value[qid] = toggleAnswer(answers.value[qid], key, multiChoice)
}

function isAnswered(qid: number) {
  return !isAnswerEmpty(answers.value[qid])
}

/** 开始倒计时（进入考试/恢复考试时调用） */
function begin(seconds: number) {
  startTimer(seconds)
  remainingTime.value = seconds
}

async function confirmSubmit() {
  try {
    await ElMessageBox.confirm('确定要交卷吗？', '提示', { type: 'warning' })
    await doSubmit()
  } catch {}
}

async function autoSubmit() {
  ElMessage.warning('考试时间已到，自动交卷')
  await doSubmit()
}

async function doSubmit() {
  stopTimer()
  emit('submit', {
    is_timeout: remainingTime.value <= 0,
    answers: { ...answers.value },
    remaining_time: remainingTime.value
  })
}

defineExpose({ begin })
</script>

<style scoped>
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
