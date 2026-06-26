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
          <el-tag size="small">{{ typeMap[item.question?.type] }}</el-tag>
          <span class="wrong-count">错误 {{ item.wrong_count }} 次</span>
        </div>
        <p class="wrong-content">{{ item.question?.content }}</p>
        <div v-if="redoingId === item.id" class="redo-area">
          <div v-if="item.question?.type !== 'short_answer'" class="redo-options">
            <template v-if="item.question?.type === 'true_false'">
              <div v-for="opt in [{ key: '对', label: '正确' }, { key: '错', label: '错误' }]" :key="opt.key"
                   class="redo-option" :class="{ selected: redoAnswer.includes(opt.key) }"
                   @click="toggleRedoOption(opt.key, item.question.type)">
                <span class="opt-label">{{ opt.key }}</span>
                <span>{{ opt.label }}</span>
              </div>
            </template>
            <template v-else>
              <div v-for="(label, key) in item.question?.options" :key="key"
                   class="redo-option" :class="{ selected: redoAnswer.includes(key) }"
                   @click="toggleRedoOption(key, item.question.type)">
                <span class="opt-label">{{ key }}</span>
                <span>{{ label }}</span>
              </div>
            </template>
          </div>
          <el-input v-else v-model="redoTextAnswer" type="textarea" :rows="3" placeholder="请输入答案" />
          <div class="redo-actions">
            <el-button type="primary" size="small" @click="submitRedo(item)">提交</el-button>
            <el-button size="small" @click="redoingId = null">取消</el-button>
          </div>
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

const typeMap = { single_choice: '单选题', multi_choice: '多选题', true_false: '判断题', fault_image: '故障识图', short_answer: '简答题' }

const wrongList = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const filterType = ref('')
const redoingId = ref(null)
const redoAnswer = ref([])
const redoTextAnswer = ref('')

onMounted(() => loadData())
watch(filterType, () => { page.value = 1; loadData() })

async function loadData() {
  try {
    const res = await wrongQuestionApi.getWrongQuestions({ page: page.value, page_size: pageSize.value, type: filterType.value || undefined })
    wrongList.value = res.data?.items || []
    total.value = res.data?.total || 0
  } catch (e) {}
}

function startRedo(item) {
  redoingId.value = item.id
  redoAnswer.value = []
  redoTextAnswer.value = ''
}

function toggleRedoOption(key, type) {
  if (type === 'multi_choice') {
    const idx = redoAnswer.value.indexOf(key)
    if (idx > -1) redoAnswer.value.splice(idx, 1)
    else redoAnswer.value.push(key)
  } else {
    redoAnswer.value = [key]
  }
}

async function submitRedo(item) {
  try {
    const answer = item.question?.type === 'short_answer' ? redoTextAnswer.value : redoAnswer.value
    const res = await wrongQuestionApi.redoWrongQuestion(item.question_id, answer)
    if (res.data?.is_correct === true) {
      ElMessage.success('回答正确！已移出错题本')
    } else if (res.data?.is_correct === false) {
      ElMessage.warning('回答错误，继续加油')
    } else {
      ElMessage.info('简答题需要教师批改，已提交')
    }
    redoingId.value = null
    await loadData()
  } catch (e) {
    ElMessage.error('提交失败')
  }
}

async function removeWrong(questionId) {
  try {
    await ElMessageBox.confirm('确定移除此错题？', '提示', { type: 'warning' })
    await wrongQuestionApi.removeWrongQuestion(questionId)
    ElMessage.success('已移除')
    await loadData()
  } catch (e) {}
}

async function exportWrong() {
  try {
    const res = await wrongQuestionApi.exportWrongQuestions()
    const blob = new Blob([res], { type: 'text/plain; charset=utf-8' })
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.setAttribute('download', 'wrong_questions.txt')
    document.body.appendChild(link)
    link.click()
    link.remove()
    window.URL.revokeObjectURL(url)
  } catch (e) {
    ElMessage.error('导出失败')
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
.redo-options { display: flex; flex-direction: column; gap: 6px; margin-bottom: 10px; }
.redo-option { display: flex; align-items: center; padding: 8px 12px; border: 1px solid #dcdfe6; border-radius: 6px; cursor: pointer; }
.redo-option:hover { border-color: #409eff; }
.redo-option.selected { border-color: #409eff; background: #ecf5ff; }
.opt-label { width: 24px; height: 24px; line-height: 24px; text-align: center; border-radius: 50%; background: #f5f7fa; margin-right: 8px; font-size: 12px; }
.redo-actions { margin-top: 8px; }
.wrong-actions { display: flex; gap: 8px; }
</style>
