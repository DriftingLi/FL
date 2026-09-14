<template>
  <div class="mx-auto max-w-[760px] p-5">
    <div class="mb-3">
      <UiButton variant="text" size="small" @click="goBack">
        <el-icon><ArrowLeft /></el-icon>
        <span class="ml-1">返回</span>
      </UiButton>
    </div>

    <UiErrorState
      v-if="loadError"
      title="题目加载失败"
      description="题目可能已下架，或不在当前证件的题库内"
      :retrying="retrying"
      @retry="retryLoad"
    />
    <UiSkeleton v-else-if="loading" variant="card" :count="1" />

    <template v-else-if="question">
      <div class="mb-2 flex items-center gap-2">
        <UiTag size="small">{{ typeLabel }}</UiTag>
        <UiActionChip
          icon="fav"
          :label="favorited ? '已收藏' : '收藏'"
          tone="fav"
          :active="favorited"
          compact
          @click="toggleFavorite"
        />
      </div>

      <p class="text-[15px] leading-[1.7] text-ink">{{ question.content }}</p>
      <img v-if="question.image_url" :src="question.image_url" alt="" class="mt-3 max-h-[320px] rounded-card" />

      <div class="mt-4">
        <template v-if="submitted && lastResult">
          <AnswerResultCard
            :correct-answer="lastResult.correct_answer || ''"
            :user-answer="lastResult.user_answer"
            :is-correct="!!lastResult.is_correct"
            :duration-seconds="lastDuration"
            :accuracy-rate="lastResult.accuracy_rate"
            :common-wrong="lastResult.common_wrong"
            :question-type="question.type"
          />
          <AIExplanationCard :ai-explanation="lastResult.ai_explanation" :fallback-explanation="lastResult.explanation" />
          <KnowledgeCard :tags="knowledgeTags" />
          <CommentCard :question-id="question.id" />
          <NoteCard :question-id="question.id" />
          <div class="mt-3 flex gap-2">
            <UiButton size="small" @click="goQuestionBank">去题库练习同类题</UiButton>
          </div>
        </template>
        <template v-else>
          <QuestionOptionPicker
            v-if="question.type !== 'short_answer'"
            :options="currentOptions"
            :selected-keys="selectedOptionKeys"
            :multi-choice="question.type === 'multi_choice'"
            :disabled="submitted"
            @select="onSelect"
          />
          <el-input v-else v-model="textAnswer" type="textarea" :rows="4" placeholder="请输入答案" />
          <div class="mt-3 flex gap-2">
            <UiButton variant="primary" size="small" :disabled="!canSubmit" :loading="submitting" @click="onSubmit">提交</UiButton>
            <UiButton size="small" @click="goQuestionBank">去题库练习</UiButton>
          </div>
        </template>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
// 题目承载页（ADR-0049 决策 4）：搜索结果的「题目」落点。
//
// 口径：**练习同源**——单题会话走 usePracticeSession 的 single 变体（与错题重做同一形态），
// 提交走 /practice-mode/submit（判分与练习同管线），解析/评论/笔记/知识点沿用练习的外围三件。
// 题干本身来自题库池口径的 by-id 读取（#981）：draft、源标记真题题、非当前证件一律 404，
// 所以这里不会成为「真题题免费刷」的入口。
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft } from '@element-plus/icons-vue'
import { questionInteractionApi } from '@/api/questionInteraction'
import { practiceModeApi } from '@/api/practiceMode'
import type { QuestionDTO } from '@/api/generated/questionBank'
import type { Question, QuestionType, SubmitResult } from '@/types/question'
import { usePracticeSession } from '@/composables/usePracticeSession'
import { questionPeripheralAdapters, useQuestionPeripherals } from '@/composables/useQuestionPeripherals'
import QuestionOptionPicker from '@/components/student/QuestionOptionPicker.vue'
import AnswerResultCard from '@/components/practice/AnswerResultCard.vue'
import AIExplanationCard from '@/components/practice/AIExplanationCard.vue'
import KnowledgeCard from '@/components/practice/KnowledgeCard.vue'
import CommentCard from '@/components/practice/CommentCard.vue'
import NoteCard from '@/components/practice/NoteCard.vue'
import UiActionChip from '@/components/ui/UiActionChip.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiTag from '@/components/ui/UiTag.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'

const route = useRoute()
const router = useRouter()

const TYPE_LABELS: Record<string, string> = {
  single_choice: '单选题',
  multi_choice: '多选题',
  true_false: '判断题',
  fault_image: '识图题',
  short_answer: '简答题'
}

const question = ref<Question | null>(null)
const loading = ref(false)
const loadError = ref(false)
const retrying = ref(false)
const submitting = ref(false)

/** 生成物 QuestionDTO → 页面消费的 UI 模型 Question：显式映射，不做隐式断言（ADR-0048 决策 7）。 */
function toUIQuestion(dto: QuestionDTO): Question {
  return {
    id: dto.id,
    type: dto.type as QuestionType,
    content: dto.content,
    options: (dto.options ?? null) as Record<string, string> | null,
    image_url: dto.image_url,
    status: dto.status as Question['status'],
    reject_reason: dto.reject_reason,
    score: dto.score,
    created_by: dto.created_by,
    created_by_type: dto.created_by_type,
    credential_id: dto.credential_id ?? null,
    created_at: dto.created_at,
    updated_at: dto.updated_at,
    answer: dto.answer,
    explanation: dto.explanation
  }
}

async function fetchQuestion(): Promise<void> {
  const id = Number(route.params.id)
  if (!id) return
  loading.value = true
  loadError.value = false
  try {
    question.value = toUIQuestion(await questionInteractionApi.getQuestion(id))
  } catch {
    loadError.value = true
    question.value = null
  } finally {
    loading.value = false
  }
}

// 单题变体：无推进节奏、无断点进度（与错题重做同一形态）
const session = usePracticeSession({
  start: async (mode) => {
    if (mode !== 'single' || !question.value) return null
    return { questions: [question.value], startIndex: 0, answersState: null }
  },
  submit: async (payload) => {
    const answer = Array.isArray(payload.user_answer) ? payload.user_answer.join(', ') : String(payload.user_answer ?? '')
    const res = await practiceModeApi.submitAnswer({
      question_id: payload.question_id,
      user_answer: answer,
      practice_type: 'free'
    })
    return {
      ...res,
      is_correct: res?.is_correct ?? null,
      correct_answer: res?.correct_answer ?? '',
      explanation: res?.explanation ?? '',
      question_id: payload.question_id,
      user_answer: Array.isArray(payload.user_answer) ? payload.user_answer : answer
    } as SubmitResult
  },
  saveProgress: async () => {}
})

const { currentOptions, selectedOptionKeys, toggleOption, textAnswer, submitted, lastResult, canSubmit, start, submitAnswer } = session
const { favorited, toggleFavorite, knowledgeTags, lastDuration, recordDuration } = useQuestionPeripherals(
  session,
  questionPeripheralAdapters({ knowledgeTrigger: 'result' })
)

const typeLabel = computed(() => TYPE_LABELS[question.value?.type ?? ''] ?? '题目')

function onSelect(key: string) {
  if (!question.value) return
  toggleOption(question.value.id, key, question.value.type === 'multi_choice')
}

async function onSubmit() {
  recordDuration()
  submitting.value = true
  try {
    await submitAnswer()
  } finally {
    submitting.value = false
  }
}

async function load() {
  await fetchQuestion()
  if (question.value) await start('single')
}

async function retryLoad() {
  if (retrying.value) return
  retrying.value = true
  await load()
  retrying.value = false
}

function goBack() {
  router.back()
}

function goQuestionBank() {
  void router.push('/training/question-bank')
}

onMounted(() => {
  void load()
})
</script>
