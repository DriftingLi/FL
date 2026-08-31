<template>
  <div class="real-exam-practice">
    <div class="practice-toolbar">
      <div class="progress-text">
        <span class="paper-title">{{ paperTitle }}</span>
        <span class="progress-stats">第 {{ currentIdx + 1 }}/{{ questions.length }} 题 · 已答对 {{ correctCount }} · 已答错 {{ wrongCount }}</span>
      </div>
      <UiButton size="small" @click="confirmQuit">退出练习</UiButton>
    </div>

    <el-card v-if="currentQuestion" v-loading="loading" class="question-card">
      <div class="question-header">
        <el-tag size="small">{{ typeMap[currentQuestion.type] || '题目' }}</el-tag>
        <el-icon class="fav-star" :class="{ active: favorited }" @click="toggleFavorite">
          <StarFilled v-if="favorited" /><Star v-else />
        </el-icon>
      </div>
      <img v-if="currentQuestion.image_url" :src="currentQuestion.image_url" class="q-image" loading="lazy" decoding="async" />
      <p class="q-content">{{ currentQuestion.content }}</p>

      <QuestionOptionPicker
        v-if="currentQuestion.type !== 'short_answer'"
        :options="currentOptions"
        :selected-keys="selectedOptionKeys"
        :multi-choice="currentQuestion.type === 'multi_choice'"
        :disabled="submitted"
        :correct-answer="submitted && lastResult ? lastResult.correct_answer : undefined"
        :user-answer="submitted ? answers[currentQuestion.id] : undefined"
        @select="key => { if (!currentQuestion) return; toggleOption(currentQuestion.id, key, currentQuestion.type === 'multi_choice') }"
      />
      <el-input v-else v-model="textAnswer" type="textarea" :rows="4" placeholder="请输入答案" :disabled="submitted" />

      <div v-if="submitted && lastResult" class="result-area">
        <AnswerResultCard
          :correct-answer="lastResult.correct_answer"
          :user-answer="lastResult.user_answer"
          :is-correct="!!lastResult.is_correct"
          :duration-seconds="lastDuration"
          :accuracy-rate="lastResult.accuracy_rate"
          :common-wrong="lastResult.common_wrong"
          :question-type="currentQuestion.type"
        />
        <AIExplanationCard :ai-explanation="(lastResult as any).ai_explanation" :fallback-explanation="lastResult.explanation" />
        <el-alert
          :title="lastResult.is_correct ? '回答正确' : '回答错误'"
          :type="lastResult.is_correct ? 'success' : 'error'"
          :closable="false"
          show-icon
        />
        <KnowledgeCard :tags="knowledgeTags" />
        <CommentCard :question-id="currentQuestion.id" />
        <NoteCard :question-id="currentQuestion.id" />
      </div>

      <div class="q-actions">
        <UiButton v-if="currentIdx > 0" @click="prevQuestion">上一题</UiButton>
        <UiButton variant="primary" v-if="!submitted" :disabled="!canSubmit" @click="handleSubmit">
          提交答案
        </UiButton>
        <UiButton variant="primary" v-if="currentIdx < questions.length - 1" @click="nextQuestion">
          下一题
        </UiButton>
        <UiButton variant="primary" v-else @click="confirmQuit">完成练习</UiButton>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Star, StarFilled } from '@element-plus/icons-vue'
import { realExamApi } from '@/api/realExam'
import { practiceModeApi } from '@/api/practiceMode'
import { favoriteApi } from '@/api/favorite'
import { questionInteractionApi } from '@/api/questionInteraction'
import { typeMap } from '@/constants/question'
import {
  usePracticeSession,
  type PracticeStartData
} from '@/composables/usePracticeSession'
import QuestionOptionPicker from '@/components/student/QuestionOptionPicker.vue'
import AnswerResultCard from '@/components/practice/AnswerResultCard.vue'
import AIExplanationCard from '@/components/practice/AIExplanationCard.vue'
import KnowledgeCard from '@/components/practice/KnowledgeCard.vue'
import CommentCard from '@/components/practice/CommentCard.vue'
import NoteCard from '@/components/practice/NoteCard.vue'
import UiButton from '@/components/ui/UiButton.vue'

const route = useRoute()
const router = useRouter()

const paperId = computed(() => Number(route.params.paperId) || 0)
const paperTitle = computed(() => (route.query.title as string) || '真题练习')

// 真题卷练习：卷序固定，进度键 paper:<paperID>，单题提交 practice_type=paper
const {
  questions, currentIdx, answers, toggleOption,
  correctCount, wrongCount,
  currentQuestion, textAnswer, submitted, lastResult, currentOptions,
  selectedOptionKeys, canSubmit, loading,
  start, submitAnswer, nextQuestion, prevQuestion, quit
} = usePracticeSession({
  start: async (): Promise<PracticeStartData | null> => {
    if (!paperId.value) return null
    const res = await realExamApi.startPractice(paperId.value)
    const qs = res?.questions || []
    if (!qs.length) return null
    let startIndex = res?.current_index || 0
    let answersState: Record<string, unknown> | null = null
    try {
      const prog = await practiceModeApi.getProgress(`paper:${paperId.value}`)
      if (prog) {
        const idx = prog.current_index || 0
        if (idx > 0 && idx < qs.length) startIndex = idx
        answersState = prog.answers_state || null
      }
    } catch {
      // 断点查询失败降级为从卷首开始
    }
    return { questions: qs, startIndex, answersState }
  },
  submit: async (payload) => {
    try {
      return await practiceModeApi.submitAnswer({
        question_id: payload.question_id,
        user_answer: payload.user_answer as string,
        practice_type: payload.practice_type || 'paper'
      })
    } catch {
      return null
    }
  },
  saveProgress: async (payload) => {
    try {
      await practiceModeApi.saveProgress(payload.index, `paper:${paperId.value}`, payload.total, payload.answersState)
    } catch {
      // 保存失败不阻断练习
    }
  }
})

const questionStartTime = ref<number>(Date.now())
const lastDuration = ref<number | undefined>(undefined)
const knowledgeTags = ref<any[]>([])
const favorited = ref(false)
const favoriteId = ref(0)

watch(currentQuestion, async (q) => {
  questionStartTime.value = Date.now()
  lastDuration.value = undefined
  favorited.value = false
  favoriteId.value = 0
  knowledgeTags.value = []
  if (!q) return
  try {
    const res = await favoriteApi.check({ target_type: 'question', target_id: q.id })
    favorited.value = !!res?.favorited
    favoriteId.value = res?.favorite_id || 0
  } catch {
    // 查询失败降级为未收藏
  }
  try {
    knowledgeTags.value = (await questionInteractionApi.listKnowledge(q.id)) || []
  } catch {
    // 知识卡查询失败不阻断
  }
})

async function toggleFavorite() {
  const q = currentQuestion.value
  if (!q) return
  try {
    if (favorited.value && favoriteId.value) {
      await favoriteApi.remove(favoriteId.value)
      favorited.value = false
      favoriteId.value = 0
      ElMessage.success('已取消收藏')
    } else {
      const res = await favoriteApi.add({ target_type: 'question', target_id: q.id })
      favorited.value = true
      favoriteId.value = res?.favorite_id || 0
      ElMessage.success('已收藏')
    }
  } catch {
    // 收藏失败静默
  }
}

async function handleSubmit() {
  lastDuration.value = (Date.now() - questionStartTime.value) / 1000
  await submitAnswer()
}

async function confirmQuit() {
  try {
    await ElMessageBox.confirm('退出后将保存当前进度，下次可继续练习。', '退出练习', {
      confirmButtonText: '退出',
      cancelButtonText: '继续练习',
      type: 'warning'
    })
  } catch {
    return
  }
  quit()
  router.push({ name: 'RealExamPapers' })
}

start('paper')
  .then(ok => {
    if (!ok) {
      ElMessage.warning('该卷暂无可练习的题目')
      router.push({ name: 'RealExamPapers' })
    }
  })
  .catch(() => {
    // 未兑换/加载失败已由拦截器提示，返回列表
    router.push({ name: 'RealExamPapers' })
  })
</script>

<style scoped>
.real-exam-practice {
  max-width: 1200px;
  margin: 0 auto;
}
.practice-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
  flex-wrap: wrap;
}
.progress-text {
  font-size: 13px;
  color: var(--color-text-tertiary);
}
.paper-title {
  font-weight: 600;
  color: var(--color-text-primary);
  margin-right: 12px;
}
.progress-stats {
  margin-left: 4px;
}
.question-card :deep(.el-card__body) {
  padding: 20px;
}
.question-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}
.fav-star {
  cursor: pointer;
  font-size: 18px;
  color: var(--color-text-muted);
}
.fav-star.active {
  color: var(--color-warning);
}
.q-image {
  max-width: 100%;
  border-radius: 8px;
  margin-bottom: 10px;
}
.q-content {
  font-size: 15px;
  line-height: 1.7;
  color: var(--color-text-primary);
  margin: 0 0 16px;
  white-space: pre-wrap;
}
.result-area {
  margin-top: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.q-actions {
  margin-top: 18px;
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
