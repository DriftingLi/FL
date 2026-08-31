<template>
  <div>
    <UiPageHeader title="题库管理" :subtitle="total > 0 ? `共 ${total} 道题目` : undefined">
      <template #actions>
        <UiButton variant="primary" @click="router.push({ name: 'TutorQuestionCreate' })">
          新增题目
        </UiButton>
      </template>
    </UiPageHeader>

    <!-- 筛选栏 -->
    <UiCard variant="flat" padding="sm" class="mb-4">
      <div class="flex flex-wrap items-center gap-2">
        <el-select
          v-model="credentialId"
          placeholder="所属证件"
          clearable
          class="!w-[180px]"
          @change="loadData"
        >
          <el-option v-for="c in credentials" :key="c.id" :label="c.name" :value="c.id" />
        </el-select>
        <el-select v-model="filters.type" placeholder="题型" clearable class="!w-[130px]">
          <el-option label="单选题" value="single_choice" />
          <el-option label="多选题" value="multi_choice" />
          <el-option label="判断题" value="true_false" />
          <el-option label="故障识图" value="fault_image" />
          <el-option label="简答题" value="short_answer" />
        </el-select>
        <el-select v-model="filters.status" placeholder="状态" clearable class="!w-[120px]">
          <el-option label="草稿" value="draft" />
          <el-option label="待审核" value="pending" />
          <el-option label="已发布" value="published" />
        </el-select>
        <el-input
          v-model="filters.keyword"
          placeholder="搜索题目"
          clearable
          class="!w-[200px]"
          @keyup.enter="loadData"
        />
        <UiButton variant="primary" @click="loadData">查询</UiButton>
        <UiButton v-if="hasFilters" variant="ghost" @click="resetFilters">重置</UiButton>
      </div>
    </UiCard>

    <!-- 列表：错误 → 加载 → 内容 / 空态 -->
    <UiErrorState
      v-if="loadError"
      title="题目加载失败"
      description="网络或服务端异常，可重试"
      :retrying="retrying"
      @retry="handleRetry"
    />

    <UiSkeleton v-else-if="loading" variant="table" :count="8" />

    <template v-else-if="questions.length > 0">
      <el-table :data="questions" stripe>
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column prop="type" label="题型" width="100">
          <template #default="{ row }">{{ (typeMap as Record<string, string>)[row.type] }}</template>
        </el-table-column>
        <el-table-column label="证件" width="140">
          <template #default="{ row }">{{ credentials.find(c => c.id === (row as any).credential_id)?.name || '—' }}</template>
        </el-table-column>
        <el-table-column prop="content" label="题干" show-overflow-tooltip />
        <el-table-column label="状态" width="120">
          <template #default="{ row }">
            <el-tooltip
              v-if="row.status === 'draft' && row.reject_reason"
              :content="`驳回理由：${row.reject_reason}`"
              placement="top"
            >
              <UiTag tone="danger">已驳回</UiTag>
            </el-tooltip>
            <UiTag v-else :tone="statusTone[row.status]">{{ statusMap[row.status] }}</UiTag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="90" fixed="right" align="center">
          <template #default="{ row }">
            <el-dropdown trigger="click" @command="(cmd: string) => handleAction(cmd, row)">
              <UiButton variant="primary" link size="small">
                操作<el-icon class="el-icon--right"><ArrowDown /></el-icon>
              </UiButton>
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

      <div v-if="total > pageSize" class="mt-4 flex justify-center">
        <el-pagination
          v-model:current-page="page"
          :page-size="pageSize"
          :total="total"
          layout="prev, pager, next"
          @current-change="handlePageChange"
        />
      </div>
    </template>

    <UiEmptyState
      v-else
      title="暂无题目"
      :description="hasFilters ? '当前筛选条件下没有匹配的题目' : '还没有创建任何题目'"
      :action-text="hasFilters ? '清空筛选' : '新增题目'"
      @action="hasFilters ? resetFilters() : router.push({ name: 'TutorQuestionCreate' })"
    />

    <el-dialog v-model="detailVisible" title="题目详情" width="600px">
      <div v-if="currentQuestion" class="flex flex-col gap-2 text-sm">
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
        <div v-if="currentQuestion.image_url" class="mt-1">
          <p><strong>图片：</strong></p>
          <img
            :src="currentQuestion.image_url"
            class="mt-1 max-h-[300px] max-w-full rounded-card"
          />
        </div>
        <el-alert
          v-if="currentQuestion.status === 'draft' && currentQuestion.reject_reason"
          title="该题目已被管理员驳回"
          type="error"
          :description="currentQuestion.reject_reason"
          show-icon
          :closable="false"
          class="mt-2"
        />
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowDown } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { questionBankApi } from '@/api/questionBank'
import { credentialApi, type CredentialDict } from '@/api/credential'
import type { Question } from '@/types/question'
import { typeMap } from '@/constants/question'
import UiPageHeader from '@/components/ui/UiPageHeader.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiTag from '@/components/ui/UiTag.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import { useAsyncPage } from '@/composables/useAsyncPage'

const router = useRouter()
const statusMap: Record<string, string> = { draft: '草稿', pending: '待审核', published: '已发布' }
/** 状态徽标色调：draft 中性 / pending 警示 / published 成功 */
const statusTone: Record<string, 'neutral' | 'warning' | 'success'> = {
  draft: 'neutral',
  pending: 'warning',
  published: 'success'
}

const credentials = ref<CredentialDict[]>([])
const credentialId = ref<number | null>(null)
const questions = ref<Question[]>([])
const filters = ref({ type: '', status: '', keyword: '' })
const detailVisible = ref(false)
const currentQuestion = ref<(Question & { reject_reason?: string }) | null>(null)

// 三态 + 分页三件套收编 useAsyncPage（#401）：错误详情由拦截器统一 toast，loadError 降级 boolean
const {
  loading,
  loadError,
  retrying,
  retry: handleRetry,
  page,
  pageSize,
  total,
  run: loadData,
  handlePageChange
} = useAsyncPage(async () => {
  const params: any = { page: page.value, page_size: pageSize.value, ...filters.value }
  if (credentialId.value) params.credential_id = credentialId.value
  const res = await questionBankApi.getQuestions(params)
  questions.value = res?.questions || []
  total.value = res?.total || 0
})

const hasFilters = computed(
  () =>
    credentialId.value !== null ||
    filters.value.type !== '' ||
    filters.value.status !== '' ||
    filters.value.keyword.trim() !== ''
)

onMounted(() => {
  loadData()
  loadCredentials()
})

async function loadCredentials() {
  try {
    const data = await credentialApi.listCredentials()
    credentials.value = data.credentials || []
  } catch {
    // 证件字典加载失败不阻断列表：最多是「证件」列回显为 —
  }
}

function resetFilters() {
  credentialId.value = null
  filters.value = { type: '', status: '', keyword: '' }
  page.value = 1
  loadData()
}

function viewDetail(row: Question) {
  currentQuestion.value = row
  detailVisible.value = true
}

function editQuestion(row: Question) {
  router.push({ name: 'TutorQuestionCreate', query: { id: row.id } })
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
  } catch {
    /* 错误已由拦截器提示 */
  }
}
</script>
