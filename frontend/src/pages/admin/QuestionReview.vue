<template>
  <div class="question-review-page">
    <div class="page-header">
      <h2>题库审核</h2>
      <div class="header-tips" v-if="pendingCount > 0">
        <el-tag type="warning">待审核 {{ pendingCount }} 题</el-tag>
      </div>
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
        <el-option label="待审核" value="pending" />
        <el-option label="已发布" value="published" />
        <el-option label="草稿" value="draft" />
      </el-select>
      <el-input v-model="filters.keyword" placeholder="搜索题目" clearable style="width: 200px" @keyup.enter="loadData" />
      <el-button type="primary" @click="loadData">查询</el-button>
      <el-button
        v-if="selectedIds.length > 0"
        type="success"
        @click="batchPublish"
      >
        批量发布 ({{ selectedIds.length }})
      </el-button>
      <el-button
        v-if="selectedIds.length > 0"
        type="danger"
        @click="batchReject"
      >
        批量驳回 ({{ selectedIds.length }})
      </el-button>
    </div>

    <el-table :data="questions" stripe v-loading="loading" @selection-change="handleSelection">
      <el-table-column type="selection" width="50" />
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column prop="type" label="题型" width="100">
        <template #default="{ row }">{{ (typeMap as Record<string, string>)[row.type] }}</template>
      </el-table-column>
      <el-table-column prop="content" label="题干" show-overflow-tooltip />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="statusType[row.status]" size="small">{{ statusMap[row.status] }}</el-tag>
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
                <el-dropdown-item v-if="row.status === 'pending'" command="publish">发布</el-dropdown-item>
                <el-dropdown-item v-if="row.status === 'pending'" command="reject" divided>驳回</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </template>
      </el-table-column>
    </el-table>

    <el-pagination
      v-model:current-page="page"
      :page-size="pageSize"
      :total="total"
      layout="prev, pager, next"
      @current-change="loadData"
      style="margin-top: 15px"
    />

    <!-- 题目详情弹窗 -->
    <el-dialog v-model="detailVisible" title="题目详情" width="640px">
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
          title="该题目已被驳回"
          type="error"
          :description="currentQuestion.reject_reason"
          show-icon
          :closable="false"
          style="margin-top: 15px"
        />
      </div>
      <template #footer v-if="currentQuestion && currentQuestion.status === 'pending'">
        <el-button @click="detailVisible = false">关闭</el-button>
        <el-button type="danger" @click="rejectFromDetail">驳回</el-button>
        <el-button type="success" @click="publishFromDetail">发布</el-button>
      </template>
    </el-dialog>

    <!-- 驳回理由弹窗 -->
    <el-dialog v-model="rejectDialogVisible" title="填写驳回理由" width="500px">
      <el-form>
        <el-form-item label="驳回理由" required>
          <el-input
            v-model="rejectReason"
            type="textarea"
            :rows="4"
            placeholder="请填写驳回理由，导师将看到此说明"
            maxlength="500"
            show-word-limit
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="cancelReject">取消</el-button>
        <el-button type="danger" :loading="rejecting" @click="confirmReject">确认驳回</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowDown } from '@element-plus/icons-vue'
import { questionBankApi } from '@/api/questionBank'
import type { Question } from '@/types/question'
import { typeMap } from '@/constants/question'

const statusMap: Record<string, string> = { draft: '草稿', pending: '待审核', published: '已发布' }
const statusType: Record<string, string> = { draft: 'info', pending: 'warning', published: 'success' }

const loading = ref(false)
const questions = ref<Question[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const filters = ref({ type: '', status: 'pending', keyword: '' })
const selectedIds = ref<number[]>([])
const pendingCount = ref(0)

const detailVisible = ref(false)
const currentQuestion = ref<Question | null>(null)

// 驳回相关
const rejectDialogVisible = ref(false)
const rejectReason = ref('')
const rejecting = ref(false)
// 驳回模式：single(单题) / batch(批量) / detail(从详情弹窗)
let rejectMode = 'single'
let rejectTargetId = 0

onMounted(() => loadData())

// 操作下拉菜单统一入口
function handleAction(cmd: string, row: any) {
  switch (cmd) {
    case 'view':
      viewDetail(row)
      break
    case 'publish':
      publishSingle(row)
      break
    case 'reject':
      rejectSingle(row)
      break
  }
}

async function loadData() {
  loading.value = true
  try {
    const res = await questionBankApi.getQuestions({ page: page.value, page_size: pageSize.value, ...filters.value })
    questions.value = res?.questions || []
    total.value = res?.total || 0
    // 加载待审核总数（仅当不是 pending 筛选时单独查询）
    await loadPendingCount()
  } catch {
    /* 错误已由拦截器提示 */
  } finally {
    loading.value = false
  }
}

async function loadPendingCount() {
  try {
    const res = await questionBankApi.getQuestions({ page: 1, page_size: 1, status: 'pending' })
    pendingCount.value = res?.total || 0
  } catch (e) {}
}

function handleSelection(rows: { id: number }[]) {
  selectedIds.value = rows.map((r: { id: number }) => r.id)
}

function viewDetail(row: any) {
  currentQuestion.value = row
  detailVisible.value = true
}

// 单题发布
async function publishSingle(row: { id: number }) {
  try {
    await ElMessageBox.confirm(`确定发布题目 #${row.id}？发布后学员可见。`, '确认发布', { type: 'success' })
    await questionBankApi.publishQuestion(row.id)
    ElMessage.success('发布成功')
    await loadData()
  } catch {
    /* 错误已由拦截器提示 */
  }
}

// 单题驳回
function rejectSingle(row: { id: number }) {
  rejectMode = 'single'
  rejectTargetId = row.id
  rejectReason.value = ''
  rejectDialogVisible.value = true
}

// 批量发布
async function batchPublish() {
  try {
    await ElMessageBox.confirm(`确定批量发布选中的 ${selectedIds.value.length} 道题目？`, '确认批量发布', { type: 'success' })
    await questionBankApi.batchPublish(selectedIds.value)
    ElMessage.success('批量发布成功')
    await loadData()
  } catch {
    /* 错误已由拦截器提示 */
  }
}

// 批量驳回
function batchReject() {
  if (selectedIds.value.length === 0) return
  rejectMode = 'batch'
  rejectReason.value = ''
  rejectDialogVisible.value = true
}

// 从详情弹窗发布
async function publishFromDetail() {
  if (!currentQuestion.value) return
  await publishSingle(currentQuestion.value)
  detailVisible.value = false
}

// 从详情弹窗驳回
function rejectFromDetail() {
  if (!currentQuestion.value) return
  rejectMode = 'detail'
  rejectTargetId = currentQuestion.value.id
  rejectReason.value = ''
  rejectDialogVisible.value = true
}

// 确认驳回
async function confirmReject() {
  if (!rejectReason.value.trim()) {
    ElMessage.warning('请填写驳回理由')
    return
  }
  rejecting.value = true
  try {
    if (rejectMode === 'batch') {
      await questionBankApi.batchReject(selectedIds.value, rejectReason.value)
      ElMessage.success('批量驳回成功')
    } else {
      await questionBankApi.rejectQuestion(rejectTargetId, rejectReason.value)
      ElMessage.success('已驳回')
      if (rejectMode === 'detail') detailVisible.value = false
    }
    rejectDialogVisible.value = false
    await loadData()
  } catch {
    /* 错误已由拦截器提示 */
  } finally {
    rejecting.value = false
  }
}

function cancelReject() {
  rejectDialogVisible.value = false
  rejectReason.value = ''
}
</script>

<style scoped>
.question-review-page { padding: 0; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
.page-header h2 { margin: 0; }
.filter-bar { display: flex; gap: 10px; margin-bottom: 15px; flex-wrap: wrap; align-items: center; }
</style>
