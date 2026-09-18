<template>
  <div class="mx-auto max-w-[900px]">
    <h2>错题本</h2>
    <div class="mb-3 flex flex-wrap items-center gap-2.5">
      <el-select v-model="filterType" placeholder="题型筛选" clearable style="width: 150px">
        <el-option label="单选题" value="single_choice" />
        <el-option label="多选题" value="multi_choice" />
        <el-option label="判断题" value="true_false" />
        <el-option label="故障识图" value="fault_image" />
        <el-option label="简答题" value="short_answer" />
      </el-select>
      <UiButton @click="toggleSort">
        <el-icon class="mr-1"><SortDown v-if="sortOrder === 'desc'" /><SortUp v-else /></el-icon>
        {{ sortOrder === 'desc' ? '最新错误在前' : '最早错误在前' }}
      </UiButton>
      <UiCheckbox v-model="filterFavorited">收藏</UiCheckbox>
      <UiCheckbox v-model="filterMultiWrong">错多次</UiCheckbox>
      <UiButton @click="resetFilters">重置筛选</UiButton>
    </div>
    <div class="mb-5 flex flex-wrap items-center gap-2.5">
      <UiCheckbox :model-value="isAllSelected" :indeterminate="isIndeterminate" @change="toggleSelectAll" :disabled="wrongList.length===0">全选</UiCheckbox>
      <UiButton variant="danger" :disabled="selectedIds.size===0" @click="handleBatchRemove">批量移出</UiButton>
      <UiButton variant="success" :disabled="wrongList.length===0" @click="handleExport">导出错题</UiButton>
    </div>

    <UiAsyncSection
      :error="loadError"
      :loading="loading"
      :empty="isEmpty"
      :retrying="retrying"
      error-title="错题加载失败"
      error-description="网络或服务端异常，可重试"
      @retry="retryLoad"
    >
      <template #skeleton>
        <UiSkeleton variant="card" :count="4" />
      </template>

      <div>
      <el-card
        v-for="(item, i) in wrongList"
        :key="item.id"
        class="stagger-in mb-3"
        :style="staggerStyle(i)"
      >
        <div class="mb-2 flex items-center justify-between">
          <div class="flex items-center gap-2">
            <UiCheckbox :model-value="selectedIds.has(item.question_id)" @change="(val:boolean)=>toggleSelect(item.question_id, val)" />
            <UiTag size="small">{{ item.question?.type ? (typeMap as Record<string, string>)[item.question.type] : '' }}</UiTag>
            <UiTag v-if="item.is_redone" tone="success" size="small">已重做</UiTag>
            <el-icon class="fav-star cursor-pointer text-lg text-ink-muted hover:text-warn" :class="item.favorited ? 'text-warn' : ''" @click="toggleFavorite(item)">
              <StarFilled v-if="item.favorited" /><Star v-else />
            </el-icon>
          </div>
          <span class="text-[13px] text-bad">错误 {{ item.wrong_count }} 次 · 最近 {{ formatDateTime(item.last_wrong_at) }}</span>
        </div>
        <p class="mb-2.5 text-[15px] leading-[1.6]">{{ item.question?.content }}</p>
        <!-- 题干配图（#1077）：故障识图题的图此前在列表里完全不可见 -->
        <img
          v-if="item.question?.image_url"
          :src="item.question.image_url"
          alt="题目配图"
          class="mb-2.5 max-h-[280px] rounded-card border border-line object-contain"
        />
        <div v-if="redoItem?.id === item.id" class="mt-2.5">
          <template v-if="submitted && lastResult">
            <AnswerResultCard
              :correct-answer="lastResult.correct_answer || ''"
              :user-answer="lastResult.user_answer"
              :is-correct="!!lastResult.is_correct"
              :duration-seconds="lastDuration"
              :accuracy-rate="lastResult.accuracy_rate"
              :common-wrong="lastResult.common_wrong"
              :question-type="redoItem.question?.type"
            />
            <AIExplanationCard :ai-explanation="lastResult.ai_explanation" :fallback-explanation="lastResult.explanation" />
            <KnowledgeCard :tags="knowledgeTags" />
            <CommentCard :question-id="redoItem.question_id" />
            <NoteCard :question-id="redoItem.question_id" />
            <div class="mt-2 flex gap-2">
              <UiButton size="small" @click="closeRedo">关闭</UiButton>
              <UiButton variant="primary" size="small" @click="closeRedo">完成</UiButton>
            </div>
          </template>
          <template v-else>
            <div class="mb-2 flex items-center gap-2">
              <UiTag size="small">{{ redoItem.question?.type ? (typeMap as Record<string, string>)[redoItem.question.type] : '' }}</UiTag>
              <UiActionChip icon="fav" :label="favorited ? '已收藏' : '收藏'" tone="fav" :active="favorited" compact @click="toggleRedoFavorite" />
            </div>
            <QuestionOptionPicker
              v-if="redoItem.question?.type !== 'short_answer'"
              compact
              :options="currentOptions"
              :selected-keys="selectedOptionKeys"
              :multi-choice="redoItem.question?.type === 'multi_choice'"
              :disabled="submitted"
              @select="selectRedoOption"
            />
            <el-input v-else v-model="textAnswer" type="textarea" :rows="3" placeholder="请输入答案" :disabled="submitted" />
            <div class="mt-2 flex gap-2">
              <UiButton variant="primary" size="small" :disabled="!canSubmit" @click="submitRedo">提交</UiButton>
              <UiButton size="small" @click="closeRedo">取消</UiButton>
            </div>
          </template>
        </div>
        <template v-else>
          <!-- 折叠态「答案与解析」（#1077）：不重做也能复习。默认收起，同一时间只开一道 -->
          <div v-if="answerOpenId === item.question_id" class="mb-2.5 rounded-card border border-line bg-canvas px-3 py-2.5">
            <div class="mb-1 text-[13px] text-ink-3">
              我上次选的答案：<span class="font-semibold text-bad">{{ item.last_user_answer || '（无作答记录）' }}</span>
            </div>
            <div class="mb-1 text-[13px] text-ink-3">
              正确答案：<span class="font-semibold text-ok-strong">{{ item.question?.answer || '—' }}</span>
            </div>
            <div class="text-[13px] leading-[1.6] text-ink-2">
              解析：{{ item.question?.explanation || '暂无解析' }}
            </div>
          </div>
          <div class="flex gap-2">
            <UiButton size="small" @click="toggleAnswer(item.question_id)">
              {{ answerOpenId === item.question_id ? '收起答案' : '查看答案与解析' }}
            </UiButton>
            <UiButton variant="primary" size="small" @click="startRedo(item)">重做</UiButton>
            <UiButton variant="danger" size="small" @click="removeWrong(item.question_id)">移出</UiButton>
          </div>
        </template>
      </el-card>
      <UiPagination
      v-model:current-page="page"
      :page-size="pageSize"
      :total="total"
      :show-total="false"
      @current-change="handlePageChange"
    />
    </div>
      <template #empty>
        <UiEmptyState description="暂无错题" />
      </template>
    </UiAsyncSection>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { Star, StarFilled, SortDown, SortUp } from '@element-plus/icons-vue'
import { wrongQuestionApi, type WrongQuestionItem } from '@/api/wrongQuestion'
import { favoriteApi } from '@/api/favorite'
import { typeMap } from '@/constants/question'
import { downloadBlob } from '@/composables/useReportDownload'
import { formatDateTime } from '@/utils/format'
import type { Question } from '@/types/question'
import { usePracticeSession } from '@/composables/usePracticeSession'
import { useQuestionPeripherals, questionPeripheralAdapters } from '@/composables/useQuestionPeripherals'
import QuestionOptionPicker from '@/components/student/QuestionOptionPicker.vue'
import AnswerResultCard from '@/components/practice/AnswerResultCard.vue'
import AIExplanationCard from '@/components/practice/AIExplanationCard.vue'
import KnowledgeCard from '@/components/practice/KnowledgeCard.vue'
import CommentCard from '@/components/practice/CommentCard.vue'
import NoteCard from '@/components/practice/NoteCard.vue'
import { useAsyncPage } from '@/composables/useAsyncPage'
import { useStagger } from '@/composables/useStagger'
import UiAsyncSection from '@/components/ui/UiAsyncSection.vue'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiActionChip from '@/components/ui/UiActionChip.vue'
import UiPagination from '@/components/ui/UiPagination.vue'
import { useConfirm } from '@/composables/useConfirm'
import UiTag from '@/components/ui/UiTag.vue'
import UiCheckbox from '@/components/ui/UiCheckbox.vue'



const wrongList = ref<WrongQuestionItem[]>([])

// 三态 + 分页三件套收编（#388）
const {
  loading,
  loadError,
  retrying,
  retry: retryLoad,
  isEmpty,
  page,
  pageSize,
  total,
  run: loadData,
  handlePageChange
} = useAsyncPage(async () => {
  const res = await wrongQuestionApi.getWrongQuestions({
    page: page.value,
    page_size: pageSize.value,
    type: filterType.value || undefined,
    sort: sortOrder.value,
    favorited: filterFavorited.value || undefined,
    min_wrong_count: filterMultiWrong.value ? 2 : undefined
  })
  wrongList.value = res?.items || []
  total.value = res?.total || 0
  // 清理不在当前页的选择
  const ids = new Set(wrongList.value.map(i => i.question_id))
  const n = new Set<number>()
  selectedIds.value.forEach(id => { if (ids.has(id)) n.add(id) })
  selectedIds.value = n
}, { itemsRef: wrongList })

const staggerStyle = useStagger()
const filterType = ref('')
const sortOrder = ref<'desc' | 'asc'>('desc')
const filterFavorited = ref(false)
const filterMultiWrong = ref(false)
const selectedIds = ref<Set<number>>(new Set())

// 折叠态「答案与解析」的展开位（#1077）：同一时间只开一道；该卡进入重做态时本区不渲染
const answerOpenId = ref<number | null>(null)

function toggleAnswer(questionId: number) {
  answerOpenId.value = answerOpenId.value === questionId ? null : questionId
}

// ===== 错题重做 = 答题会话的单题变体（#617）=====
// 无推进节奏、单题即时提交：提交管线/判分装配与练习同源（usePracticeSession），
// 收藏/知识点/计时外围三件经 questionPeripheralAdapters 工厂接入（与练习页同一绑定点），
// 内联重做状态机（redoAnswer/redoResults/wrongKnowledge 等）删除。
const redoItem = ref<WrongQuestionItem | null>(null)

const session = usePracticeSession({
  // 单题变体 start：把当前重做项包装成单题会话（无断点进度）
  start: async (mode) => {
    const item = redoItem.value
    if (mode !== 'single' || !item?.question) return null
    return {
      // 生成物 QuestionDTO 自带 id（题库主键），单题会话的 id 用 question_id（与练习会话同口径）
      questions: [{ ...item.question, id: item.question_id } as Question],
      startIndex: 0,
      answersState: null
    }
  },
  // 单题提交走错题重做接口（判分口径由后端统一落 question_practice_record）；
  // 失败向上抛出 → 会话保持作答态（错误已由拦截器提示）
  submit: async (payload) => {
    const answer = Array.isArray(payload.user_answer) ? payload.user_answer.join(', ') : String(payload.user_answer ?? '')
    const res = await wrongQuestionApi.redoWrongQuestion(payload.question_id, answer)
    return {
      ...res,
      is_correct: res?.is_correct ?? null,
      correct_answer: res?.correct_answer ?? '',
      explanation: res?.explanation ?? '',
      question_id: payload.question_id,
      // 结果卡按数组渲染多选（「、」分隔），与 master 一致；接口提交用逗号串
      user_answer: Array.isArray(payload.user_answer) ? payload.user_answer : answer
    }
  },
  // 单题变体无断点进度，不落进度
  saveProgress: async () => {}
})

const {
  currentOptions,
  selectedOptionKeys,
  toggleOption,
  textAnswer,
  submitted,
  lastResult,
  canSubmit,
  start,
  submitAnswer,
  backToEntry
} = session

// 外围三件与练习同源：收藏（进题查态）、知识点（出结果查）、计时（提交前取用时）
// （toggleFavorite 重命名为 togglePanelFavorite，列表级 toggleFavorite 已占用该名）
const { favorited, toggleFavorite: togglePanelFavorite, knowledgeTags, lastDuration, recordDuration } = useQuestionPeripherals(
  session,
  questionPeripheralAdapters({ knowledgeTrigger: 'result' })
)

async function startRedo(item: WrongQuestionItem) {
  if (!item.question) return
  redoItem.value = item
  const ok = await start('single')
  if (!ok) redoItem.value = null
}

function closeRedo() {
  redoItem.value = null
  backToEntry()
}

function selectRedoOption(key: string | number) {
  const item = redoItem.value
  if (!item) return
  toggleOption(item.question_id, key, item.question?.type === 'multi_choice')
}

// 重做面板收藏与列表星标同态（同一 question_id 的收藏状态）
async function toggleRedoFavorite() {
  const r = await togglePanelFavorite()
  const item = redoItem.value
  if (r !== null && item) item.favorited = r === 'added'
}

async function submitRedo() {
  recordDuration()
  await submitAnswer()
  const r = lastResult.value
  if (!r) return // 提交失败：错误已由拦截器提示，留在作答态
  if (r.is_correct === true) {
    ElMessage.success('回答正确！已标记为已重做')
    // 更新列表中的标记
    const item = redoItem.value
    if (item) {
      const idx = wrongList.value.findIndex(w => w.id === item.id)
      if (idx >= 0) wrongList.value[idx].is_redone = true
    }
  } else if (r.is_correct === false) {
    ElMessage.warning('回答错误，继续加油')
  } else {
    ElMessage.info('简答题需要教师批改，已提交')
  }
}

const isAllSelected = computed(()=> wrongList.value.length>0 && wrongList.value.every(i=> selectedIds.value.has(i.question_id)))
const isIndeterminate = computed(()=> {
  const sel = selectedIds.value.size
  return sel>0 && sel < wrongList.value.length
})

function toggleSelect(qid:number, val:boolean){
  const n = new Set(selectedIds.value)
  if(val) n.add(qid); else n.delete(qid)
  selectedIds.value = n
}
function toggleSelectAll(val: boolean){
  if(val){
    selectedIds.value = new Set(wrongList.value.map(i=>i.question_id))
  } else {
    selectedIds.value = new Set()
  }
}

onMounted(() => loadData())
watch([filterType, sortOrder, filterFavorited, filterMultiWrong], () => { page.value = 1; loadData() })

// 证件切换即重拉（#605：错题本按当前证件分区，失效刷新已内聚进 useAsyncPage，
// 回第一页、筛选条件原样保留）

function toggleSort() {
  sortOrder.value = sortOrder.value === 'desc' ? 'asc' : 'desc'
}

function resetFilters() {
  filterType.value = ''
  sortOrder.value = 'desc'
  filterFavorited.value = false
  filterMultiWrong.value = false
  page.value = 1
  loadData()
}

async function toggleFavorite(item: WrongQuestionItem) {
  try {
    if (item.favorited) {
      await favoriteApi.remove(item.favorite_id!)
      item.favorited = false
      item.favorite_id = 0
      if (filterFavorited.value) await loadData()
    } else {
      const res = await favoriteApi.add({ target_type: 'question', target_id: item.question_id })
      item.favorited = true
      item.favorite_id = res?.favorite_id
    }
  } catch {
    /* 错误已由拦截器提示 */
  }
}

async function removeWrong(questionId: number) {
  try {
    await useConfirm().confirm('确定移出此错题？', '提示', { type: 'warning' })
    await wrongQuestionApi.removeWrongQuestion(questionId)
    ElMessage.success('已移出')
    selectedIds.value.delete(questionId)
    await loadData()
  } catch (e) {}
}

async function handleBatchRemove(){
  if(selectedIds.value.size===0){ ElMessage.warning('请选择要移出的题目'); return }
  try{
    await useConfirm().confirm(`确定移出选中的 ${selectedIds.value.size} 道错题？`, '提示', { type: 'warning' })
    await wrongQuestionApi.batchRemoveWrongQuestions(Array.from(selectedIds.value))
    ElMessage.success('已批量移出')
    selectedIds.value = new Set()
    await loadData()
  } catch {}
}

async function handleExport(){
  if(wrongList.value.length===0){ ElMessage.warning('暂无错题可导出'); return }
  if(selectedIds.value.size>0){
    const toExport = wrongList.value.filter(i=> selectedIds.value.has(i.question_id))
    if(toExport.length===0){ ElMessage.warning('请选择要导出的题目'); return }
    const lines: string[] = []
    lines.push('='.repeat(50))
    lines.push('错题本导出')
    lines.push(`导出时间: ${new Date().toLocaleString()}`)
    lines.push(`错题总数: ${toExport.length}`)
    lines.push('='.repeat(50))
    toExport.forEach((item, idx)=>{
      lines.push('')
      lines.push(`【第${idx+1}题】`)
      lines.push('-'.repeat(40))
      const q = item.question
      lines.push(`题型: ${(typeMap as any)[q?.type||''] || q?.type || ''}`)
      lines.push(`题目: ${q?.content||''}`)
      // options 在注解层是 any（生成物渲染 unknown，ADR-0048 片一已知限制）：
      // 导出为纯文本时按「选项键 → 选项文案」收窄，与题型/内容同源。
      const opts = q?.options as Record<string, string> | null | undefined
      if (opts) {
        lines.push('选项:')
        Object.keys(opts).sort().forEach(k=> lines.push(`  ${k}. ${opts[k]}`))
      }
      lines.push(`错误次数: ${item.wrong_count||0}`)
      lines.push('-'.repeat(40))
    })
    lines.push('')
    lines.push(`共 ${toExport.length} 道错题`)
    lines.push('='.repeat(50))
    const blob = new Blob([lines.join('\n')], { type: 'text/plain;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'wrong_questions.txt'
    a.click()
    URL.revokeObjectURL(url)
    return
  }
  try {
    const blob = await wrongQuestionApi.exportWrongQuestions()
    downloadBlob(blob as any, 'wrong_questions.txt')
  } catch {
    /* 错误已由拦截器提示 */
  }
}
</script>

