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
      <el-button type="success" @click="exportWrong">导出错题</el-button>
    </div>

    <div v-if="wrongList.length > 0">
      <el-card v-for="item in wrongList" :key="item.id" class="wrong-item">
        <div class="wrong-header">
          <el-tag size="small">{{ item.question?.type ? (typeMap as Record<string, string>)[item.question.type] : '' }}</el-tag>
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
              <el-button size="small" @click="redoingId = null; delete redoResults[item.id]">关闭</el-button>
              <el-button v-if="redoResults[item.id].is_correct" type="primary" size="small" @click="redoingId = null; loadData()">完成</el-button>
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
          <el-button type="danger" size="small" @click="removeWrong(item.question_id)">移除</el-button>
        </div>
      </el-card>
      <el-pagination v-model:current-page="page" :page-size="pageSize" :total="total" layout="prev, pager, next" @current-change="loadData" />
    </div>
    <el-empty v-else description="暂无错题" />
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { wrongQuestionApi } from '@/api/wrongQuestion'
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
const redoingId = ref<number | null>(null)
const redoAnswer = ref<(string | number)[]>([])
const redoTextAnswer = ref('')
const redoStartTime = ref<number>(Date.now())
const redoResults = ref<Record<number, any>>({})
const redoDurations = ref<Record<number, number>>({})
const wrongKnowledge = ref<Record<number, any[]>>({})

onMounted(() => loadData())
watch(filterType, () => { page.value = 1; loadData() })

async function loadData() {
  try {
    const res = await wrongQuestionApi.getWrongQuestions({ page: page.value, page_size: pageSize.value, type: filterType.value || undefined })
    wrongList.value = res?.items || []
    total.value = res?.total || 0
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
    const res = await wrongQuestionApi.redoWrongQuestion(item.question_id, Array.isArray(answer) ? answer.join(', ') : answer) as any
    redoDurations.value[item.id] = duration
    redoResults.value[item.id] = { ...res, user_answer: answer }
    try {
      const tags = await questionInteractionApi.listKnowledge(item.question_id)
      wrongKnowledge.value[item.id] = (tags as any) || []
    } catch { wrongKnowledge.value[item.id] = [] }
    if (res?.is_correct === true) {
      ElMessage.success('回答正确！')
    } else if (res?.is_correct === false) {
      ElMessage.warning('回答错误，继续加油')
    } else {
      ElMessage.info('简答题需要教师批改，已提交')
    }
    // 延迟刷新以便展示解析
    setTimeout(async () => {
      if (res?.is_correct === true) {
        redoingId.value = null
        await loadData()
      }
    }, 1500)
  } catch {
    /* 错误已由拦截器提示 */
  }
}

async function removeWrong(questionId: number) {
  try {
    await ElMessageBox.confirm('确定移除此错题？', '提示', { type: 'warning' })
    await wrongQuestionApi.removeWrongQuestion(questionId)
    ElMessage.success('已移除')
    await loadData()
  } catch (e) {}
}

async function exportWrong() {
  try {
    downloadBlob(await wrongQuestionApi.exportWrongQuestions(), 'wrong_questions.txt')
  } catch {
    /* 错误已由拦截器提示 */
  }
}
</script>

<style scoped>
.wrong-questions { max-width: 900px; margin: 0 auto; }
.wrong-questions h2 { margin-bottom: 20px; }
.filter-bar { display: flex; gap: 10px; margin-bottom: 20px; align-items: center; }
.wrong-item { margin-bottom: 12px; }
.wrong-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; }
.wrong-count { color: #f56c6c; font-size: 13px; }
.wrong-content { font-size: 15px; line-height: 1.6; margin-bottom: 10px; }
.redo-area { margin-top: 10px; }
.redo-actions { margin-top: 8px; }
.wrong-actions { display: flex; gap: 8px; }
</style>
