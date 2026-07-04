<template>
  <div class="question-manage">
    <div class="page-header">
      <h2>题库管理</h2>
      <el-button type="primary" @click="$router.push('/training/tutor/question-create')">新增题目</el-button>
    </div>

    <div class="filter-bar">
      <el-select v-model="filters.level" placeholder="等级" clearable style="width: 120px">
        <el-option label="初级" value="beginner" />
        <el-option label="中级" value="intermediate" />
        <el-option label="高级" value="advanced" />
      </el-select>
      <el-select v-model="filters.type" placeholder="题型" clearable style="width: 130px">
        <el-option label="单选题" value="single_choice" />
        <el-option label="多选题" value="multi_choice" />
        <el-option label="判断题" value="true_false" />
        <el-option label="故障识图" value="fault_image" />
        <el-option label="简答题" value="short_answer" />
      </el-select>
      <el-select v-model="filters.status" placeholder="状态" clearable style="width: 120px">
        <el-option label="草稿" value="draft" />
        <el-option label="待审核" value="pending" />
        <el-option label="已发布" value="published" />
      </el-select>
      <el-input v-model="filters.keyword" placeholder="搜索题目" clearable style="width: 200px" @keyup.enter="loadData" />
      <el-button type="primary" @click="loadData">查询</el-button>
      <el-button @click="batchPublishSelected" :disabled="selectedIds.length === 0">批量发布</el-button>
    </div>

    <el-table :data="questions" stripe v-loading="loading" @selection-change="handleSelection">
      <el-table-column type="selection" width="50" />
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column prop="type" label="题型" width="100">
        <template #default="{ row }">{{ typeMap[row.type] }}</template>
      </el-table-column>
      <el-table-column prop="level" label="等级" width="80">
        <template #default="{ row }">{{ levelMap[row.level] }}</template>
      </el-table-column>
      <el-table-column prop="content" label="题干" show-overflow-tooltip />
      <el-table-column prop="status" label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="statusType[row.status]" size="small">{{ statusMap[row.status] }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="200">
        <template #default="{ row }">
          <el-button size="small" @click="viewDetail(row)">查看</el-button>
          <el-button size="small" type="primary" @click="editQuestion(row)">编辑</el-button>
          <el-button size="small" type="danger" @click="handleDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-pagination v-model:current-page="page" :page-size="pageSize" :total="total" layout="prev, pager, next" @current-change="loadData" style="margin-top: 15px" />

    <el-dialog v-model="detailVisible" title="题目详情" width="600px">
      <div v-if="currentQuestion">
        <p><strong>题型：</strong>{{ typeMap[currentQuestion.type] }}</p>
        <p><strong>等级：</strong>{{ levelMap[currentQuestion.level] }}</p>
        <p><strong>题干：</strong>{{ currentQuestion.content }}</p>
        <div v-if="currentQuestion.options">
          <p><strong>选项：</strong></p>
          <p v-for="(val, key) in currentQuestion.options" :key="key">{{ key }}. {{ val }}</p>
        </div>
        <p><strong>答案：</strong>{{ currentQuestion.answer }}</p>
        <p v-if="currentQuestion.explanation"><strong>解析：</strong>{{ currentQuestion.explanation }}</p>
        <p v-if="currentQuestion.reference_answer"><strong>参考答案：</strong>{{ currentQuestion.reference_answer }}</p>
        <p v-if="currentQuestion.scoring_criteria"><strong>评分标准：</strong>{{ currentQuestion.scoring_criteria }}</p>
        <div v-if="currentQuestion.image_url" style="margin-top: 10px">
          <p><strong>图片：</strong></p>
          <img :src="currentQuestion.image_url" style="max-width: 100%; max-height: 300px; border-radius: 8px" />
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { questionBankApi } from '@/api/questionBank'

const router = useRouter()
const typeMap = { single_choice: '单选题', multi_choice: '多选题', true_false: '判断题', fault_image: '故障识图', short_answer: '简答题' }
const levelMap = { beginner: '初级', intermediate: '中级', advanced: '高级', expert: '顶级' }
const statusMap = { draft: '草稿', pending: '待审核', published: '已发布' }
const statusType = { draft: 'info', pending: 'warning', published: 'success' }

const loading = ref(false)
const questions = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const filters = ref({ level: '', type: '', status: '', keyword: '' })
const selectedIds = ref([])
const detailVisible = ref(false)
const currentQuestion = ref(null)

onMounted(() => loadData())

async function loadData() {
  loading.value = true
  try {
    const res = await questionBankApi.getQuestions({ page: page.value, page_size: pageSize.value, ...filters.value })
    questions.value = res.data?.questions || []
    total.value = res.data?.total || 0
  } catch (e) {} finally { loading.value = false }
}

function handleSelection(rows) {
  selectedIds.value = rows.map(r => r.id)
}

function viewDetail(row) {
  currentQuestion.value = row
  detailVisible.value = true
}

function editQuestion(row) {
  router.push({ path: '/training/tutor/question-create', query: { id: row.id } })
}

async function handleDelete(row) {
  try {
    await ElMessageBox.confirm('确定删除此题目？', '提示', { type: 'warning' })
    await questionBankApi.deleteQuestion(row.id)
    ElMessage.success('删除成功')
    await loadData()
  } catch (e) {}
}

async function batchPublishSelected() {
  try {
    await questionBankApi.batchPublish(selectedIds.value)
    ElMessage.success('发布成功')
    await loadData()
  } catch (e) {
    ElMessage.error('发布失败')
  }
}
</script>

<style scoped>
.question-manage { padding: 0; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
.page-header h2 { margin: 0; }
.filter-bar { display: flex; gap: 10px; margin-bottom: 15px; flex-wrap: wrap; align-items: center; }
</style>
