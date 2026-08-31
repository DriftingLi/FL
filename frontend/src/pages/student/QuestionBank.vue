<template>
  <div class="question-bank">
    <!-- ===== 入口 ===== -->
    <div v-if="!mode" class="flex flex-col gap-6">
      <UiSectionHeader title="题库练习" subtitle="选择练习方式，开始刷题" />

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
            <span class="grid h-9 w-9 place-items-center rounded-ctl bg-ui-50 text-ui-600">
              <el-icon :size="20"><Sort /></el-icon>
            </span>
            <h3 class="font-heading text-base font-semibold text-ink">顺序练习</h3>
          </div>
          <p class="mt-3 text-sm text-ink-3">
            {{ seqProgress.completed }}/{{ seqProgress.total || totalQuestions }} 已练习
          </p>
          <UiButton
            variant="primary"
            block
            class="mt-4"
            @click="startSequential"
          >
            {{ seqProgress.completed > 0 ? '继续练习' : '开始练习' }}
          </UiButton>
        </UiCard>

        <!-- 随机练习 -->
        <UiCard variant="raised" class="stagger-in" :style="stagger(1)">
          <div class="flex items-center gap-2">
            <span class="grid h-9 w-9 place-items-center rounded-ctl bg-ok-soft text-ok">
              <el-icon :size="20"><MagicStick /></el-icon>
            </span>
            <h3 class="font-heading text-base font-semibold text-ink">随机练习</h3>
          </div>
          <div class="mt-3 flex items-center justify-between gap-2">
            <span class="text-sm text-ink-3">每次题量</span>
            <el-select v-model="randomCount" size="small" class="w-[110px]">
              <el-option v-for="o in randomCountOptions" :key="o.value" :label="o.label" :value="o.value" />
            </el-select>
          </div>
          <UiButton variant="success" block class="mt-4" :loading="loading" @click="startFree()">
            开始练习
          </UiButton>
        </UiCard>

        <!-- 专项练习 -->
        <UiCard variant="raised" class="stagger-in" :style="stagger(2)">
          <div class="flex items-center gap-2">
            <span class="grid h-9 w-9 place-items-center rounded-ctl bg-warn-soft text-warn">
              <el-icon :size="20"><Filter /></el-icon>
            </span>
            <h3 class="font-heading text-base font-semibold text-ink">专项练习</h3>
          </div>
          <div class="mt-3 flex items-center justify-between gap-2">
            <span class="text-sm text-ink-3">题型</span>
            <el-select v-model="specialType" size="small" placeholder="选择题型" class="w-[130px]">
              <el-option v-for="o in questionTypeOptions" :key="o.value" :label="o.label" :value="o.value" />
            </el-select>
          </div>
          <UiButton
            variant="warning"
            block
            class="mt-4"
            :loading="loading"
            :disabled="!specialType"
            @click="startFree()"
          >
            开始练习
          </UiButton>
        </UiCard>

        <!-- 标签练习 -->
        <UiCard variant="raised" class="stagger-in" :style="stagger(3)">
          <div class="flex items-center gap-2">
            <span class="grid h-9 w-9 place-items-center rounded-ctl bg-ui-accent-400/10 text-ui-accent-600">
              <el-icon :size="20"><CollectionTag /></el-icon>
            </span>
            <h3 class="font-heading text-base font-semibold text-ink">标签练习</h3>
          </div>
          <div class="mt-3 flex items-center justify-between gap-2">
            <span class="text-sm text-ink-3">考点标签</span>
            <el-select
              v-model="tagPracticeId"
              size="small"
              filterable
              placeholder="选择标签"
              class="w-[130px]"
              :loading="tagsLoading"
            >
              <el-option v-for="t in tags" :key="t.id" :label="`${t.name}（${t.question_count ?? 0}题）`" :value="t.id" />
            </el-select>
          </div>
          <p v-if="tagPracticeId && tagProgress.total > 0" class="mt-2 text-xs text-ink-3">
            {{ tagProgress.completed }}/{{ tagProgress.total }} 已练习
          </p>
          <UiButton
            variant="primary"
            block
            class="mt-4"
            :loading="loading"
            :disabled="!tagPracticeId"
            @click="startTagPractice"
          >
            {{ tagProgress.completed > 0 ? '继续练习' : '开始练习' }}
          </UiButton>
        </UiCard>
      </div>
    </div>

    <!-- ===== 刷题中 ===== -->
    <div v-if="mode" class="practice-area">
      <div class="practice-toolbar">
        <div class="progress-text">
          第 {{ currentIdx + 1 }}/{{ questions.length }} 题
          <span class="progress-stats">已答对 {{ correctCount }} · 已答错 {{ wrongCount }}</span>
          <el-tag v-if="mode === 'sequential'" size="small" type="primary" style="margin-left: 10px">顺序练习</el-tag>
          <el-tag v-else-if="mode === 'tag'" size="small" style="margin-left: 10px">
            标签：{{ currentTagName }}
          </el-tag>
          <el-tag v-else-if="specialType && mode === 'free'" size="small" type="warning" style="margin-left: 10px">
            {{ typeMap[specialType] }}
          </el-tag>
        </div>
        <el-button size="small" @click="confirmQuit">退出练习</el-button>
      </div>

      <el-card v-if="currentQuestion" class="question-card">
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
            v-if="lastResult.ai_score !== undefined && lastResult.ai_score !== null"
            :title="lastResult.is_correct ? '回答正确' : '回答错误'"
            :type="lastResult.is_correct ? 'success' : 'error'"
            :closable="false"
            show-icon
          >
            <div>AI 评分：{{ lastResult.ai_score }} 分 · {{ lastResult.ai_comment || '' }}</div>
          </el-alert>
          <el-alert
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

        <div class="q-actions">
          <el-button v-if="currentIdx > 0" @click="prevQuestion">上一题</el-button>
          <UiButton variant="primary" v-if="!submitted" :disabled="!canSubmit" @click="handleSubmit">
            提交答案
          </UiButton>
          <UiButton variant="primary" v-if="currentIdx < questions.length - 1" @click="nextQuestion">
            下一题
          </UiButton>
        </div>
      </el-card>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { Sort, MagicStick, Filter, CollectionTag, Star, StarFilled } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { questionBankApi } from '@/api/questionBank'
import { favoriteApi } from '@/api/favorite'
import { practiceModeApi, type PracticeModeKey } from '@/api/practiceMode'
import { trainingApi } from '@/api/training'
import type { QuestionTag } from '@/api/training'
import { useStagger } from '@/composables/useStagger'
import UiButton from '@/components/ui/UiButton.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiSectionHeader from '@/components/ui/UiSectionHeader.vue'
import UiStatCard from '@/components/ui/UiStatCard.vue'

const stagger = useStagger()
import { typeMap, questionTypeOptions, randomCountOptions } from '@/constants/question'
import type { PracticeProgress, QuestionType } from '@/types/question'
import {
  usePracticeSession,
  type PracticeMode,
  type PracticeStartData
} from '@/composables/usePracticeSession'
import QuestionOptionPicker from '@/components/student/QuestionOptionPicker.vue'
import AnswerResultCard from '@/components/practice/AnswerResultCard.vue'
import AIExplanationCard from '@/components/practice/AIExplanationCard.vue'
import KnowledgeCard from '@/components/practice/KnowledgeCard.vue'
import CommentCard from '@/components/practice/CommentCard.vue'
import NoteCard from '@/components/practice/NoteCard.vue'
import { questionInteractionApi } from '@/api/questionInteraction'
import { useCredentialRefetch } from '@/composables/useCredentialRefetch'

// null = 入口；'sequential' | 'free' | 'tag' = 刷题中

// 卡片选择器状态
const randomCount = ref(20)
const specialType = ref<QuestionType | ''>('')
const tagPracticeId = ref<number | null>(null)
const tags = ref<QuestionTag[]>([])
const tagsLoading = ref(false)

const currentTagName = computed(() => {
  const t = tags.value.find(item => item.id === tagPracticeId.value)
  return t?.name || ''
})

// 卡片展示数据
const seqProgress = ref<PracticeProgress>({ completed: 0, total: 0, current_index: 0 })
const tagProgress = ref<PracticeProgress>({ completed: 0, total: 0, current_index: 0 })
const totalQuestions = ref(0)

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

// 选择标签时查询该标签的练习进度（断点续练展示）
watch(tagPracticeId, async (id) => {
  tagProgress.value = { completed: 0, total: 0, current_index: 0 }
  if (!id) return
  try {
    const prog = await practiceModeApi.getProgress(`tag:${id}`)
    tagProgress.value = prog || tagProgress.value
  } catch {
    // 查询失败降级为无进度
  }
})

// 进度 key 语义（#390 与后端封闭校验对齐）：顺序练习 'sequential'；标签练习 'tag:<tagID>'；
// 专项（free:<type>）与随机不在后端封闭集（sequential/tag:<id>/paper:<id>，未知 400）——一律不落进度
function getPracticeModeKey(currentMode: PracticeMode): PracticeModeKey | '' {
  if (currentMode === 'sequential') return 'sequential'
  if (currentMode === 'tag' && tagPracticeId.value) return `tag:${tagPracticeId.value}`
  return ''
}

// 查询断点续练起始位置和持久化答题状态（answers_state 三态这里归一为 null）
async function resolveProgress(modeKey: PracticeModeKey | '', total: number): Promise<{ startIndex: number; answersState: Record<string, unknown> | null }> {
  if (!modeKey) return { startIndex: 0, answersState: null }
  try {
    const progRes = await practiceModeApi.getProgress(modeKey)
    const data = progRes || {}
    const idx = data.current_index || 0
    const startIndex = idx > 0 && idx < total ? idx : 0
    return { startIndex, answersState: data.answers_state || null }
  } catch (e) {}
  return { startIndex: 0, answersState: null }
}

// 练习会话状态机：三个 start 模式 + 进度保存作为 adapter 注入，页面不触碰 API 层
const {
  mode, questions, currentIdx, answers, toggleOption,
  correctCount, wrongCount,
  currentQuestion, textAnswer, submitted, lastResult, currentOptions,
  selectedOptionKeys, canSubmit, loading,
  start, submitAnswer, nextQuestion, prevQuestion, quit
} = usePracticeSession({
  start: async (startMode: PracticeMode): Promise<PracticeStartData | null> => {
    if (startMode === 'sequential') {
      const res = await practiceModeApi.startSequential()
      const data = res || {}
      const qs = data.questions || []
      if (!qs.length) return null
      return { questions: qs, ...(await resolveProgress('sequential', qs.length)) }
    }
    if (startMode === 'tag') {
      if (!tagPracticeId.value) return null
      const res = await practiceModeApi.startTagPractice({ tag_id: tagPracticeId.value, count: 0 })
      const data = res || {}
      const qs = data.questions || []
      if (!qs.length) return null
      return { questions: qs, ...(await resolveProgress(`tag:${tagPracticeId.value}`, qs.length)) }
    }
    // free：专项（选了题型）或随机 —— 两者均不在后端进度封闭集（#390），不查询/不落续练进度
    const params: Record<string, unknown> = {}
    if (specialType.value) {
      params.type = specialType.value
      params.count = 0
    } else {
      params.count = randomCount.value
    }
    const qs = (await practiceModeApi.getFreeQuestions(params)) || []
    if (!qs.length) return null
    return { questions: qs, ...(await resolveProgress('', qs.length)) }
  },
  submit: async (payload) => {
    try {
      return await practiceModeApi.submitAnswer({
        question_id: payload.question_id,
        user_answer: payload.user_answer as string,
        practice_type: payload.practice_type || 'free'
      })
    } catch (e) {
      return null
    }
  },
  saveProgress: async (payload) => {
    // 随机练习 modeKey 为空：不保存进度
    const modeKey = getPracticeModeKey(payload.mode)
    if (!modeKey) return
    try {
      await practiceModeApi.saveProgress(payload.index, modeKey, payload.total, payload.answersState)
      if (payload.mode === 'sequential') seqProgress.value.completed = payload.index
    } catch (e) {
      // 保存失败不阻断练习
    }
  }
})

// ===== 开始各模式（薄 wrapper：启动会话 + 空题数提示）=====
async function startSequential() {
  const ok = await start('sequential')
  if (!ok) ElMessage.warning('题库暂无题目')
}

async function startFree() {
  const ok = await start('free')
  if (!ok) ElMessage.warning('暂无符合条件的题目')
}

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
  if (!q) return
  try {
    const res = await favoriteApi.check({ target_type: 'question', target_id: q.id })
    favorited.value = !!res?.favorited
    favoriteId.value = res?.favorite_id || 0
  } catch {
    // 查询失败降级为未收藏
  }
})

async function toggleFavorite() {
  const q = currentQuestion.value
  if (!q) return
  try {
    if (favorited.value) {
      await favoriteApi.remove(favoriteId.value)
      favorited.value = false
      favoriteId.value = 0
    } else {
      const res = await favoriteApi.add({ target_type: 'question', target_id: q.id })
      favorited.value = true
      favoriteId.value = res?.favorite_id || 0
    }
  } catch {
    /* 错误已由拦截器提示 */
  }
}

watch(() => (lastResult.value as any)?.question_id, async (qid: number) => {
  if (!qid) { knowledgeTags.value = []; return }
  try {
    const tags = await questionInteractionApi.listKnowledge(qid)
    knowledgeTags.value = (tags as any) || []
  } catch { knowledgeTags.value = [] }
})

async function handleSubmit() {
  lastDuration.value = (Date.now() - questionStartTime.value) / 1000
  await submitAnswer()
  loadPracticeStats()
}

async function startTagPractice() {
  if (!tagPracticeId.value) return
  const ok = await start('tag')
  if (!ok) ElMessage.warning('该标签下暂无已发布题目')
}

async function confirmQuit() {
  try {
    await ElMessageBox.confirm('确定要退出本次练习吗？', '提示', { type: 'warning' })
    // 退出时保存当前游标和答题状态并返回入口，随后刷新卡片进度展示
    await quit()
    loadCardData()
    loadPracticeStats()
  } catch (e) {}
}

onMounted(() => {
  loadCardData()
  loadTags()
})

// 证件切换即重拉（单点：watch store.current.id，见 useCredentialRefetch；
// loadCardData 已含 getPracticeStats，覆盖原事件通知里的 loadPracticeStats）
useCredentialRefetch(() => {
  loadCardData()
})

async function loadTags() {
  tagsLoading.value = true
  try {
    // 拦截器已解包信封
    const data = await trainingApi.getTags()
    tags.value = data.tags || []
  } catch (e) {
    // 静默失败，标签入口降级为不可用
  } finally {
    tagsLoading.value = false
  }
}

async function loadCardData() {
  try {
    const [statsRes, progRes, practiceRes] = await Promise.all([
      questionBankApi.getStats().catch(() => null as any),
      practiceModeApi.getSequentialProgress().catch(() => null as any),
      practiceModeApi.getPracticeStats().catch(() => null as any)
    ])
    if (statsRes) totalQuestions.value = (statsRes.total as number) || 0
    if (progRes) seqProgress.value = progRes
    if (practiceRes) {
      practiceStats.value = {
        today_count: Number((practiceRes as any)?.today_count ?? 0),
        total_count: Number((practiceRes as any)?.total_count ?? 0),
        total_days: Number((practiceRes as any)?.total_days ?? 0)
      }
      practiceStatsLoading.value = false
    }
  } catch (e) {
    // 静默失败，卡片展示降级为默认值
  }
  // 若第 3 并发失败时 fallback 仍走独立 loader（避免悬在 skeleton）
  if (practiceStatsLoading.value) {
    try { await loadPracticeStats() } catch {}
  }
}
</script>

<style scoped>
.question-bank { max-width: 1200px; margin: 0 auto; }
.practice-area { margin-top: 10px; }
.practice-toolbar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 15px; padding: 10px 15px; background: var(--color-bg-card); border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
.progress-text { font-size: 15px; color: var(--color-text-primary); }
.progress-stats { margin-left: 12px; color: var(--color-text-tertiary); font-size: 13px; }
.question-card { margin-bottom: 15px; }
.question-header { display: flex; gap: 8px; align-items: center; margin-bottom: 15px; }
.fav-star { cursor: pointer; font-size: 18px; color: var(--color-text-disabled); }
.fav-star:hover { color: var(--color-warning); }
.fav-star.active { color: var(--color-warning); }
.q-image { max-width: 100%; max-height: 250px; border-radius: 8px; margin-bottom: 10px; }
.q-content { font-size: 16px; line-height: 1.8; margin-bottom: 15px; white-space: pre-wrap; }
.q-feedback { margin-top: 15px; }
.feedback-explanation { margin-top: 6px; color: var(--color-text-secondary); }
.q-actions { display: flex; justify-content: center; margin-top: 20px; }
</style>
