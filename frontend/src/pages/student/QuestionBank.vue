<template>
  <div class="mx-auto max-w-[1200px]">
    <!-- ===== 入口 ===== -->
    <div v-if="!mode" class="flex flex-col gap-6">
      <UiSectionHeader title="题库练习" subtitle="选择练习方式，开始刷题" />
      <div class="self-start">
        <UiButton variant="ghost" :aria-expanded="showHistory" @click="showHistory = !showHistory">
          {{ showHistory ? '收起练习记录' : '练习记录与统计' }}
        </UiButton>
      </div>
      <PracticeHistoryPanel v-if="showHistory" />

      <div class="flex items-center gap-3">
        <label for="practice-category" class="text-sm text-ink">本次练习分类</label>
        <UiSelect
          id="practice-category"
          v-model="categoryId"
          placeholder="选择培训分类"
          class="w-[200px]"
          :disabled="launching"
          :options="categories.map(category => ({ label: category.name, value: category.id }))"
        />
      </div>

      <!-- 练习统计 -->
      <div class="grid gap-4 sm:grid-cols-3">
        <UiStatCard
          label="今日做题"
          :value="practiceStats.today_count"
          :loading="practiceStatsLoading"
          tone="brand"
        />
        <UiStatCard
          label="累计做题"
          :value="practiceStats.total_count"
          :loading="practiceStatsLoading"
          tone="ok"
        />
        <UiStatCard
          label="累计做题天数"
          :value="practiceStats.total_days"
          unit="天"
          :loading="practiceStatsLoading"
          tone="warn"
        />
      </div>

      <!-- 4 种练习方式 -->
      <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <!-- 顺序练习 -->
        <UiCard variant="raised" class="stagger-in" :style="stagger(0)">
          <div class="flex items-center gap-2">
            <span class="grid h-9 w-9 place-items-center rounded-ctl bg-ui-soft text-ui-soft-ink">
              <UiIcon name="arrow-up-down" :size="20" />
            </span>
            <h3 class="font-heading text-base font-semibold text-ink">顺序练习</h3>
          </div>
          <UiButton
            variant="primary"
            block
            class="mt-4"
            :disabled="!categoryId"
            :loading="launching"
            @click="beginPractice('sequential', 'sequential')"
          >
            开始或继续练习
          </UiButton>
        </UiCard>

        <!-- 随机练习 -->
        <UiCard variant="raised" class="stagger-in" :style="stagger(1)">
          <div class="flex items-center gap-2">
            <span class="grid h-9 w-9 place-items-center rounded-ctl bg-ok-soft text-ok">
              <UiIcon name="wand-sparkles" :size="20" />
            </span>
            <h3 class="font-heading text-base font-semibold text-ink">随机练习</h3>
          </div>
          <div class="mt-3 flex items-center justify-between gap-2">
            <span class="text-sm text-ink-3">每次题量</span>
            <UiSelect v-model="randomCount" size="small" class="w-[110px]" :options="(randomCountOptions).map((o) => ({ label: o.label, value: o.value }))" />
          </div>
          <UiButton variant="success" block class="mt-4" :loading="launching" :disabled="!categoryId" @click="beginPractice('free', 'random', randomCount)">
            开始练习
          </UiButton>
        </UiCard>

        <!-- 专项练习 -->
        <UiCard variant="raised" class="stagger-in" :style="stagger(2)">
          <div class="flex items-center gap-2">
            <span class="grid h-9 w-9 place-items-center rounded-ctl bg-warn-soft text-warn">
              <UiIcon name="funnel" :size="20" />
            </span>
            <h3 class="font-heading text-base font-semibold text-ink">专项练习</h3>
          </div>
          <div class="mt-3 flex items-center justify-between gap-2">
            <span class="text-sm text-ink-3">题型</span>
            <UiSelect v-model="specialType" size="small" placeholder="选择题型" class="w-[130px]" :options="(questionTypeOptions).map((o) => ({ label: o.label, value: o.value }))" />
          </div>
          <UiButton
            variant="warning"
            block
            class="mt-4"
            :loading="launching"
            :disabled="!categoryId || !specialType"
            @click="beginPractice('free', 'special:' + specialType)"
          >
            开始练习
          </UiButton>
        </UiCard>

        <!-- 标签练习 -->
        <UiCard variant="raised" class="stagger-in" :style="stagger(3)">
          <div class="flex items-center gap-2">
            <span class="grid h-9 w-9 place-items-center rounded-ctl bg-ui-accent-400/10 text-ui-accent-600">
              <UiIcon name="bookmark" :size="20" />
            </span>
            <h3 class="font-heading text-base font-semibold text-ink">标签练习</h3>
          </div>
          <div class="mt-3 flex items-center justify-between gap-2">
            <span class="text-sm text-ink-3">考点标签</span>
            <UiSelect v-model="tagPracticeId" size="small" filterable placeholder="选择标签" class="w-[130px]" :loading="tagsLoading" :options="tags.map(tag => ({ label: tag.name, value: tag.id }))" />
          </div>
          <UiButton
            variant="primary"
            block
            class="mt-4"
            :loading="launching"
            :disabled="!categoryId || !tagPracticeId"
            @click="beginPractice('tag', 'tag:' + tagPracticeId)"
          >
            开始或继续练习
          </UiButton>
        </UiCard>
      </div>
    </div>

    <!-- ===== 刷题中 ===== -->
    <div v-if="mode" class="mt-2.5">
      <div class="mb-[15px] flex items-center justify-between rounded-ctl bg-panel px-[15px] py-2.5 shadow-card">
        <div class="text-base text-ink">
          第 {{ currentIdx + 1 }}/{{ questions.length }} 题
          <span class="ml-3 text-sm text-ink-3">已答对 {{ correctCount }} · 已答错 {{ wrongCount }}</span>
          <UiTag size="small" class="ml-2">{{ activeCategoryName }}</UiTag>
          <UiTag v-if="mode === 'sequential'" size="small" tone="primary" style="margin-left: 10px">顺序练习</UiTag>
          <UiTag v-else-if="mode === 'tag'" size="small" style="margin-left: 10px">
            标签：{{ currentTagName }}
          </UiTag>
          <UiTag v-else-if="specialType && selectedMode?.startsWith('special:')" size="small" tone="warning" style="margin-left: 10px">
            {{ typeMap[specialType] }}
          </UiTag>
        </div>
        <UiButton size="small" @click="confirmQuit">退出练习</UiButton>
      </div>

      <UiCard v-if="currentQuestion" class="mb-[15px]">
        <div class="mb-[15px] flex items-center gap-2">
          <UiTag size="small">{{ typeMap[currentQuestion.type] || '题目' }}</UiTag>
          <!-- #511：收藏统一药丸（激活琥珀填充） -->
          <UiActionChip icon="fav" :label="favorited ? '已收藏' : '收藏'" tone="fav" :active="favorited" compact @click="toggleFavorite" />
        </div>
        <img v-if="currentQuestion.image_url" :src="currentQuestion.image_url" class="mb-2.5 max-h-[250px] max-w-full rounded-ctl" loading="lazy" decoding="async" />
        <p class="mb-[15px] whitespace-pre-wrap text-base leading-[1.8]">{{ currentQuestion.content }}</p>

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
        <UiInput v-else v-model="textAnswer" type="textarea" :rows="4" placeholder="请输入答案" :disabled="submitted" />

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
          <UiAlert
            v-if="lastResult.ai_score !== undefined && lastResult.ai_score !== null"
            :title="lastResult.is_correct ? '回答正确' : '回答错误'"
            :type="lastResult.is_correct ? 'success' : 'error'"
            :closable="false"
            show-icon
          >
            <div>AI 评分：{{ lastResult.ai_score }} 分 · {{ lastResult.ai_comment || '' }}</div>
          </UiAlert>
          <UiAlert
            v-else
            :title="lastResult.is_correct ? '回答正确' : '回答错误'"
            :type="lastResult.is_correct ? 'success' : 'error'"
            :closable="false"
            show-icon
          />
          <KnowledgeCard :tags="knowledgeTags" />
          <CommentCard :question-id="currentQuestion.id" />
          <NoteCard :question-id="currentQuestion.id" />
        </div>

        <div class="mt-5 flex justify-center">
          <UiButton v-if="currentIdx > 0" @click="prevQuestion">上一题</UiButton>
          <UiButton variant="primary" v-if="!submitted" :disabled="!canSubmit" @click="handleSubmit">
            提交答案
          </UiButton>
          <UiButton variant="primary" v-if="currentIdx < questions.length - 1" @click="nextQuestion">
            下一题
          </UiButton>
        </div>
      </UiCard>
    </div>
  </div>
</template>

<script setup lang="ts">
import UiIcon from '@/components/ui/UiIcon.vue'
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { isAxiosError } from 'axios'
import { ElMessage } from 'element-plus'
import { practiceModeApi } from '@/api/practiceMode'
import { trainingApi } from '@/api/training'
import type { QuestionTag, TrainingCategoryDTO } from '@/api/training'
import { useStagger } from '@/composables/useStagger'
import UiButton from '@/components/ui/UiButton.vue'
import UiActionChip from '@/components/ui/UiActionChip.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiSectionHeader from '@/components/ui/UiSectionHeader.vue'
import UiStatCard from '@/components/ui/UiStatCard.vue'

const stagger = useStagger()
import { typeMap, questionTypeOptions, randomCountOptions } from '@/constants/question'
import type { QuestionType } from '@/types/question'
import {
  usePracticeSession,
  type PracticeMode,
  type PracticeStartData
} from '@/composables/usePracticeSession'
import { useQuestionPeripherals, questionPeripheralAdapters } from '@/composables/useQuestionPeripherals'
import QuestionOptionPicker from '@/components/student/QuestionOptionPicker.vue'
import PracticeHistoryPanel from '@/components/student/PracticeHistoryPanel.vue'
import AnswerResultCard from '@/components/practice/AnswerResultCard.vue'
import AIExplanationCard from '@/components/practice/AIExplanationCard.vue'
import KnowledgeCard from '@/components/practice/KnowledgeCard.vue'
import CommentCard from '@/components/practice/CommentCard.vue'
import NoteCard from '@/components/practice/NoteCard.vue'
import { useAsyncPage } from '@/composables/useAsyncPage'
import { useConfirm } from '@/composables/useConfirm'
import UiTag from '@/components/ui/UiTag.vue'
import UiAlert from '@/components/ui/UiAlert.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiSelect from '@/components/ui/UiSelect.vue'

const categoryId = ref<number | null>(null)
// route 用于读取公开网校带过来的 ?category_id=（#2760）。
const route = useRoute()
const showHistory = ref(false)
const categories = ref<TrainingCategoryDTO[]>([])
const activeCategoryName = computed(() => categories.value.find(category => category.id === categoryId.value)?.name)
const randomCount = ref(20)
const specialType = ref<QuestionType | ''>('')
const tagPracticeId = ref<number | null>(null)
const tags = ref<QuestionTag[]>([])
const tagsLoading = ref(false)
const selectedMode = ref<string>()
const launching = ref(false)
let startInput: Parameters<typeof practiceModeApi.startCategoryPractice>[0]
let attemptKey = ''

const currentTagName = computed(() => {
  const t = tags.value.find(item => item.id === tagPracticeId.value)
  return t?.name || ''
})

const practiceStats = ref({ today_count: 0, total_count: 0, total_days: 0 })
const practiceStatsLoading = ref(true)

async function loadPracticeStats() {
  practiceStatsLoading.value = true
  try {
    const data = await practiceModeApi.getPracticeStats() as any
    practiceStats.value = {
      today_count: Number(data?.today_count ?? 0),
      total_count: Number(data?.total_count ?? 0),
      total_days: Number(data?.total_days ?? 0)
    }
  } catch {
    practiceStats.value = { today_count: 0, total_count: 0, total_days: 0 }
  } finally {
    practiceStatsLoading.value = false
  }
}

const session = usePracticeSession({
  start: async (): Promise<PracticeStartData> => {
    const attempt = await practiceModeApi.startCategoryPractice(startInput)
    attemptKey = attempt.attempt_key
    return { questions: attempt.questions, startIndex: attempt.current_index, answersState: attempt.answers_state }
  },
  submit: (payload) => practiceModeApi.submitCategoryAnswer({
    attempt_key: attemptKey,
    question_id: payload.question_id,
    user_answer: payload.user_answer
  }),
  saveProgress: async () => {}
})

const {
  mode, questions, currentIdx, answers, toggleOption,
  correctCount, wrongCount,
  currentQuestion, textAnswer, submitted, lastResult, currentOptions,
  selectedOptionKeys, canSubmit,
  start, submitAnswer, nextQuestion, prevQuestion, quit
} = session

// ===== 外围交互：收藏 / 知识点 / 作答计时（#616，页内自建 watch 收敛进 module）=====
const { favorited, toggleFavorite, knowledgeTags, lastDuration, recordDuration } = useQuestionPeripherals(
  session,
  questionPeripheralAdapters({ knowledgeTrigger: 'result' })
)

async function beginPractice(displayMode: PracticeMode, practiceMode: string, count?: number) {
  if (!categoryId.value || launching.value) return
  launching.value = true
  selectedMode.value = practiceMode
  startInput = { category_id: categoryId.value, mode: practiceMode, count }
  try {
    try {
      await start(displayMode)
    } catch (error) {
      if (!isAxiosError(error) || error.response?.status !== 409) throw error
      try {
        await useConfirm().confirm('本轮已无可继续的题目。确定重新开始？', '重新练习')
      } catch {
        return
      }
      startInput = { ...startInput, restart: true }
      await start(displayMode)
    }
  } catch (error) {
    ElMessage.warning(isAxiosError(error) ? error.response?.data?.message || '无法开始本次练习，请重试' : '无法开始本次练习，请重试')
  } finally {
    launching.value = false
  }
}

async function handleSubmit() {
  recordDuration()
  await submitAnswer()
  void loadPracticeStats()
}

async function confirmQuit() {
  try {
    await useConfirm().confirm('确定要退出本次练习吗？', '提示', { type: 'warning' })
    await quit()
    attemptKey = ''
    void loadPracticeStats()
  } catch (e) {}
}

async function loadTags() {
  tagsLoading.value = true
  try {
    tags.value = (await trainingApi.getTags()).tags
  } finally {
    tagsLoading.value = false
  }
}

const { run: loadEntry } = useAsyncPage(trainingApi.getCategories, {
  facets: [{ load: loadTags }],
  apply: (data) => {
    categories.value = data.categories
    preselectCategoryFromQuery()
  }
})

// preselectCategoryFromQuery 按 URL 的 category_id 预选分类（#2760 US-13/US-14）。
//
// 来源：公开网校的题库目录点「去练习」会带 ?category_id=<id> 过来（可能中间过一次登录，
// 由登录后的 redirect 把参数带回来）。这里在**分类列表加载完之后**才判：
// 判据是「该 id 在当前可选分类里」，而不是「URL 里有这个 id」——后者会让一个失效/越权
// 的分类 id 直接落进 categoryId，接着 beginPractice 打到后端 400。
//
// 缺失、无效、不在可选集内一律**不动** categoryId（保持 null，让学员手选）：
// 这正是 US-14「分类失效时仍能手动选择」的落点。不报错、不提示——
// 一个残留的查询参数不该在页面上弹一次失败。
function preselectCategoryFromQuery() {
  const raw = route.query.category_id
  if (typeof raw !== 'string' || !/^[1-9][0-9]*$/.test(raw)) return
  const id = Number(raw)
  if (!Number.isSafeInteger(id)) return
  if (!categories.value.some(category => category.id === id)) return
  categoryId.value = id
}

onMounted(() => {
  void loadEntry()
  void loadPracticeStats()
})
</script>

