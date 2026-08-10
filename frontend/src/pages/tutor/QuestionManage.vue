<template>
  <div class="question-manage">
    <div class="page-header">
      <h2>题库管理</h2>
      <el-button type="primary" @click="$router.push('/training/tutor/question-create')">新增题目</el-button>
    </div>

    <div class="filter-bar">
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
    </div>

    <el-table :data="questions" stripe v-loading="loading">
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column prop="type" label="题型" width="100">
        <template #default="{ row }">{{ (typeMap as Record<string, string>)[row.type] }}</template>
      </el-table-column>
      <el-table-column prop="content" label="题干" show-overflow-tooltip />
      <el-table-column label="状态" width="120">
        <template #default="{ row }">
          <el-tooltip
            v-if="row.status === 'draft' && row.reject_reason"
            :content="`驳回理由：${row.reject_reason}`"
            placement="top"
          >
            <el-tag type="danger" size="small">已驳回</el-tag>
          </el-tooltip>
          <el-tag v-else :type="statusType[row.status]" size="small">{{ statusMap[row.status] }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="90" fixed="right" align="center">
        <template #default="{ row }">
          <el-dropdown trigger="click" @command="(cmd: string) => handleAction(cmd, row)">
            <el-button type="primary" link size="small">
              操作<el-icon class="el-icon--right"><ArrowDown /></el-icon>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="view">查看</el-dropdown-item>
                <el-dropdown-item command="edit">编辑</el-dropdown-item>
                <el-dropdown-item v-if="row.status === 'draft'" command="review">提交审核</el-dropdown-item>
                <el-dropdown-item command="delete" divided>删除</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </template>
      </el-table-column>
    </el-table>

    <el-pagination v-model:current-page="page" :page-size="pageSize" :total="total" layout="prev, pager, next" @current-change="loadData" style="margin-top: 15px" />

    <el-dialog v-model="detailVisible" title="题目详情" width="600px">
      <div v-if="currentQuestion">
        <p><strong>题型：</strong>{{ typeMap[currentQuestion.type] }}</p>
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
        <el-alert
          v-if="currentQuestion.status === 'draft' && currentQuestion.reject_reason"
          title="该题目已被管理员驳回"
          type="error"
          :description="currentQuestion.reject_reason"
          show-icon
          :closable="false"
          style="margin-top: 15px"
        />
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowDown } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { questionBankApi } from '@/api/questionBank'
import type { Question } from '@/types/question'
import { typeMap } from '@/constants/question'

const router = useRouter()
const statusMap: Record<string, string> = { draft: '草稿', pending: '待审核', published: '已发布' }
const statusType: Record<string, string> = { draft: 'info', pending: 'warning', published: 'success' }

const loading = ref(false)
const questions = ref<Question[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const filters = ref({ type: '', status: '', keyword: '' })
const detailVisible = ref(false)
const currentQuestion = ref<(Question & { reject_reason?: string }) | null>(null)

onMounted(() => loadData())

async function loadData() {
  loading.value = true
  try {
    const res = await questionBankApi.getQuestions({ page: page.value, page_size: pageSize.value, ...filters.value })
    questions.value = res?.questions || []
    total.value = res?.total || 0
  } catch (e) {} finally { loading.value = false }
}

function viewDetail(row: Question) {
  currentQuestion.value = row
  detailVisible.value = true
}

function editQuestion(row: Question) {
  router.push({ path: '/training/tutor/question-create', query: { id: row.id } })
}

// 提交审核：将 draft 题目状态改为 pending（后端会清空驳回理由）
async function submitForReview(row: Question) {
  try {
    await ElMessageBox.confirm('确定提交该题目给管理员审核？', '提示', { type: 'info' })
    await questionBankApi.updateQuestion(row.id, { status: 'pending' })
    ElMessage.success('已提交审核')
    await loadData()
  } catch {
    /* 错误已由拦截器提示 */
  }
}

function handleAction(cmd: string, row: Question) {
  switch (cmd) {
    case 'view':
      viewDetail(row)
      break
    case 'edit':
      editQuestion(row)
      break
    case 'review':
      submitForReview(row)
      break
    case 'delete':
      handleDelete(row)
      break
  }
}

async function handleDelete(row: Question) {
  try {
    await ElMessageBox.confirm('确定删除此题目？', '提示', { type: 'warning' })
    await questionBankApi.deleteQuestion(row.id)
    ElMessage.success('删除成功')
    await loadData()
  } catch (e) {}
}
</script>

<style scoped>
.question-manage { padding: 0; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
.page-header h2 { margin: 0; }
.filter-bar { display: flex; gap: 10px; margin-bottom: 15px; flex-wrap: wrap; align-items: center; }
</style>
