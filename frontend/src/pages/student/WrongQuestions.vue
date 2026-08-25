<template>
  <div class="wrong-questions">
    <h2>错题本</h2>
    <div class="filter-bar">
      <el-select v-model="filterType" placeholder="题型筛选" clearable style="width: 150px">
        <el-option label="单选题" value="single_choice" />
        <el-option label="多选题" value="multi_choice" />
        <el-option label="判断题" value="true_false" />
        <el-option label="故障识图" value="fault_image" />
        <el-option label="简答题" value="short_answer" />
      </el-select>
      <el-button @click="toggleSort">
        <el-icon class="sort-icon"><SortDown v-if="sortOrder === 'desc'" /><SortUp v-else /></el-icon>
        {{ sortOrder === 'desc' ? '最新错误在前' : '最早错误在前' }}
      </el-button>
      <el-checkbox v-model="filterFavorited">收藏</el-checkbox>
      <el-checkbox v-model="filterMultiWrong">错多次</el-checkbox>
      <el-button @click="resetFilters">重置筛选</el-button>
    </div>
    <div class="action-bar">
      <el-checkbox :model-value="isAllSelected" :indeterminate="isIndeterminate" @change="toggleSelectAll" :disabled="wrongList.length===0">全选</el-checkbox>
      <el-button type="danger" :disabled="selectedIds.size===0" @click="handleBatchRemove">批量移出</el-button>
      <el-button type="success" :disabled="wrongList.length===0" @click="handleExport">导出错题</el-button>
    </div>

    <div v-if="wrongList.length > 0">
      <el-card v-for="item in wrongList" :key="item.id" class="wrong-item">
        <div class="wrong-header">
          <div class="header-left">
            <el-checkbox :model-value="selectedIds.has(item.question_id)" @change="(val:boolean)=>toggleSelect(item.question_id, val)" />
            <el-tag size="small">{{ item.question?.type ? (typeMap as Record<string, string>)[item.question.type] : '' }}</el-tag>
            <el-tag v-if="item.is_redone" type="success" size="small">已重做</el-tag>
            <el-icon class="fav-star" :class="{ active: item.favorited }" @click="toggleFavorite(item)">
              <StarFilled v-if="item.favorited" /><Star v-else />
            </el-icon>
          </div>
          <span class="wrong-count">错误 {{ item.wrong_count }} 次</span>
        </div>
        <p class="wrong-content">{{ item.question?.content }}</p>
        <div v-if="redoingId === item.id" class="redo-area">
          <template v-if="redoResults[item.id]">
            <AnswerResultCard
              :correct-answer="redoResults[item.id].correct_answer || ''"
              :user-answer="redoResults[item.id].user_answer"
              :is-correct="!!redoResults[item.id].is_correct"
              :duration-seconds="redoDurations[item.id]"
              :accuracy-rate="redoResults[item.id].accuracy_rate"
              :common-wrong="redoResults[item.id].common_wrong"
              :question-type="item.question?.type"
            />
            <AIExplanationCard :ai-explanation="redoResults[item.id].ai_explanation" :fallback-explanation="redoResults[item.id].explanation" />
            <KnowledgeCard :tags="wrongKnowledge[item.id] || []" />
            <CommentCard :question-id="item.question_id" />
            <NoteCard :question-id="item.question_id" />
            <div class="redo-actions">
              <el-button size="small" @click="redoingId = null">关闭</el-button>
              <el-button type="primary" size="small" @click="redoingId = null">完成</el-button>
            </div>
          </template>
          <template v-else>
            <QuestionOptionPicker
              v-if="item.question?.type !== 'short_answer'"
              compact
              :options="buildQuestionOptions(item.question ?? {})"
              :selected-keys="redoAnswer"
              :multi-choice="item.question?.type === 'multi_choice'"
              @select="key => toggleRedoOption(key, item.question?.type ?? 'single_choice')"
            />
            <el-input v-else v-model="redoTextAnswer" type="textarea" :rows="3" placeholder="请输入答案" />
            <div class="redo-actions">
              <el-button type="primary" size="small" @click="submitRedo(item)">提交</el-button>
              <el-button size="small" @click="redoingId = null">取消</el-button>
            </div>
          </template>
        </div>
        <div v-else class="wrong-actions">
          <el-button type="primary" size="small" @click="startRedo(item)">重做</el-button>
          <el-button type="danger" size="small" @click="removeWrong(item.question_id)">移出</el-button>
        </div>
      </el-card>
      <el-pagination v-model:current-page="page" :page-size="pageSize" :total="total" layout="prev, pager, next" @current-change="loadData" />
    </div>
    <el-empty v-else description="暂无错题" />
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Star, StarFilled, SortDown, SortUp } from '@element-plus/icons-vue'
import { wrongQuestionApi, type RedoResult } from '@/api/wrongQuestion'
import { favoriteApi } from '@/api/favorite'
import { typeMap } from '@/constants/question'
import { toggleAnswer, buildQuestionOptions } from '@/composables/useQuestionAnswer'
import { downloadBlob } from '@/composables/useReportDownload'
import QuestionOptionPicker from '@/components/student/QuestionOptionPicker.vue'
import AnswerResultCard from '@/components/practice/AnswerResultCard.vue'
import AIExplanationCard from '@/components/practice/AIExplanationCard.vue'
import KnowledgeCard from '@/components/practice/KnowledgeCard.vue'
import CommentCard from '@/components/practice/CommentCard.vue'
import NoteCard from '@/components/practice/NoteCard.vue'
import { questionInteractionApi } from '@/api/questionInteraction'

interface WrongItem {
  id: number
  question_id: number
  wrong_count?: number
  is_redone?: boolean
  favorited?: boolean
  favorite_id?: number
  question?: {
    type?: string
    options?: Record<string, string>
    content?: string
  }
}

const wrongList = ref<WrongItem[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const filterType = ref('')
const sortOrder = ref<'desc' | 'asc'>('desc')
const filterFavorited = ref(false)
const filterMultiWrong = ref(false)
const redoingId = ref<number | null>(null)
const redoAnswer = ref<(string | number)[]>([])
const redoTextAnswer = ref('')
const redoStartTime = ref<number>(Date.now())
const redoResults = ref<Record<number, RedoResult>>({})
const redoDurations = ref<Record<number, number>>({})
const wrongKnowledge = ref<Record<number, any[]>>({})
const selectedIds = ref<Set<number>>(new Set())

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

async function toggleFavorite(item: WrongItem) {
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

async function loadData() {
  try {
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
    const ids = new Set(wrongList.value.map(i=>i.question_id))
    const n = new Set<number>()
    selectedIds.value.forEach(id=>{ if(ids.has(id)) n.add(id) })
    selectedIds.value = n
  } catch (e) {}
}

function startRedo(item: WrongItem) {
  redoingId.value = item.id
  redoAnswer.value = []
  redoTextAnswer.value = ''
  redoStartTime.value = Date.now()
  delete redoResults.value[item.id]
  delete redoDurations.value[item.id]
}

function toggleRedoOption(key: string | number, type: string) {
  const next = toggleAnswer(redoAnswer.value, key, type === 'multi_choice')
  redoAnswer.value = type === 'multi_choice' ? (next as (string | number)[]) : [next as string | number]
}

async function submitRedo(item: WrongItem) {
  try {
    const answer = item.question?.type === 'short_answer' ? redoTextAnswer.value : redoAnswer.value
    const duration = (Date.now() - redoStartTime.value) / 1000
    const res = await wrongQuestionApi.redoWrongQuestion(item.question_id, Array.isArray(answer) ? answer.join(', ') : answer)
    redoDurations.value[item.id] = duration
    redoResults.value[item.id] = { ...res, user_answer: answer }
    try {
      const tags = await questionInteractionApi.listKnowledge(item.question_id)
      wrongKnowledge.value[item.id] = tags || []
    } catch { wrongKnowledge.value[item.id] = [] }
    if (res?.is_correct === true) {
      ElMessage.success('回答正确！已标记为已重做')
      // 更新列表中的标记
      const idx = wrongList.value.findIndex(w=>w.id===item.id)
      if(idx>=0) wrongList.value[idx].is_redone = true
    } else if (res?.is_correct === false) {
      ElMessage.warning('回答错误，继续加油')
    } else {
      ElMessage.info('简答题需要教师批改，已提交')
    }
  } catch {
    /* 错误已由拦截器提示 */
  }
}

async function removeWrong(questionId: number) {
  try {
    await ElMessageBox.confirm('确定移出此错题？', '提示', { type: 'warning' })
    await wrongQuestionApi.removeWrongQuestion(questionId)
    ElMessage.success('已移出')
    selectedIds.value.delete(questionId)
    await loadData()
  } catch (e) {}
}

async function handleBatchRemove(){
  if(selectedIds.value.size===0){ ElMessage.warning('请选择要移出的题目'); return }
  try{
    await ElMessageBox.confirm(`确定移出选中的 ${selectedIds.value.size} 道错题？`, '提示', { type: 'warning' })
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
      if(q?.options){
        lines.push('选项:')
        Object.keys(q.options).sort().forEach(k=> lines.push(`  ${k}. ${q.options![k]}`))
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

<style scoped>
.wrong-questions { max-width: 900px; margin: 0 auto; }
.wrong-questions h2 { margin-bottom: 20px; }
.filter-bar { display: flex; gap: 10px; margin-bottom: 12px; align-items: center; flex-wrap: wrap; }
.action-bar { display: flex; gap: 10px; margin-bottom: 20px; align-items: center; flex-wrap: wrap; }
.sort-icon { margin-right: 4px; }
.fav-star { cursor: pointer; font-size: 18px; color: #c0c4cc; }
.fav-star:hover { color: #e6a23c; }
.fav-star.active { color: #e6a23c; }
.wrong-item { margin-bottom: 12px; }
.wrong-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; }
.header-left { display:flex; align-items:center; gap:8px; }
.wrong-count { color: #f56c6c; font-size: 13px; }
.wrong-content { font-size: 15px; line-height: 1.6; margin-bottom: 10px; }
.redo-area { margin-top: 10px; }
.redo-actions { margin-top: 8px; display:flex; gap:8px; }
.wrong-actions { display: flex; gap: 8px; }
</style>
