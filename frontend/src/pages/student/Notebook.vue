<template>
  <div class="mx-auto max-w-[900px]">
    <div class="mb-3 flex items-center justify-between">
      <h2>我的笔记</h2>
      <UiButton variant="primary" size="small" @click="openCreate">新建笔记</UiButton>
    </div>

    <UiSegmentTabs v-model="scope" :options="scopeOptions" class="mb-4" @change="handleScopeChange" />

    <UiAsyncSection
      :error="loadError"
      :loading="loading"
      :empty="notes.length === 0"
      :retrying="retrying"
      error-title="笔记加载失败"
      error-description="网络或服务端异常，可重试"
      @retry="retryLoad"
    >
      <template #skeleton>
        <UiSkeleton variant="card" :count="4" />
      </template>

      <div>
        <UiCard
          v-for="(n, i) in notes"
          :key="n.id"
          padding="base"
          class="stagger-in mb-3"
          :style="staggerStyle(i)"
        >
          <div class="mb-1.5 flex items-center gap-2">
            <RouterLink
              v-if="n.question_id"
              :to="`/training/questions/${n.question_id}`"
              class="inline-flex items-center"
            >
              <UiTag size="small">{{ questionLabel(n) }}</UiTag>
            </RouterLink>
            <UiTag v-else size="small" tone="info">独立笔记</UiTag>
            <span class="ml-auto text-xs text-ink-3">{{ formatDateTime(n.updated_at) }}</span>
          </div>
          <p class="mb-1 text-[15px] font-semibold text-ink">{{ noteTitle(n.content) }}</p>
          <p v-if="noteExcerpt(n.content)" class="mb-2.5 text-[13px] leading-[1.6] text-ink-2">
            {{ noteExcerpt(n.content) }}
          </p>
          <div class="flex gap-2">
            <UiButton size="small" @click="openEdit(n)">编辑</UiButton>
            <UiButton variant="danger" size="small" @click="handleDelete(n)">删除</UiButton>
          </div>
        </UiCard>
        <UiPagination
          v-if="total > pageSize"
          v-model:current-page="page"
          :page-size="pageSize"
          :total="total"
          :show-total="false"
          @current-change="handlePageChange"
        />
      </div>

      <template #empty>
        <UiEmptyState :description="emptyText" />
      </template>
    </UiAsyncSection>

    <UiDialog
      v-model="dialogVisible"
      :title="editingId ? '编辑笔记' : '新建笔记'"
      width="520px"
      confirm-text="保存"
      :confirm-loading="saving"
      @confirm="handleSave"
    >
      <el-input v-model="draft" type="textarea" :rows="6" placeholder="记点什么…" maxlength="2000" show-word-limit />
    </UiDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { noteApi, type NoteItem, type NoteScope } from '@/api/note'
import { useAsyncPage } from '@/composables/useAsyncPage'
import { useConfirm } from '@/composables/useConfirm'
import { useStagger } from '@/composables/useStagger'
import { formatDateTime } from '@/utils/format'
import UiAsyncSection from '@/components/ui/UiAsyncSection.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiPagination from '@/components/ui/UiPagination.vue'
import UiSegmentTabs from '@/components/ui/UiSegmentTabs.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiTag from '@/components/ui/UiTag.vue'

// 分段控件的三态口径与后端 scope 参数同名（ADR-0055），不另造端上词表。
const scopeOptions = [
  { label: '全部', value: 'all' },
  { label: '题目笔记', value: 'question' },
  { label: '独立笔记', value: 'standalone' }
]

const scope = ref<NoteScope>('all')
const notes = ref<NoteItem[]>([])
const staggerStyle = useStagger()

const {
  loading,
  loadError,
  retrying,
  retry: retryLoad,
  page,
  pageSize,
  total,
  run: loadData,
  handlePageChange
} = useAsyncPage(async () => {
  const res = await noteApi.list({ scope: scope.value, page: page.value, page_size: pageSize.value })
  notes.value = res?.items || []
  total.value = res?.total || 0
})

const emptyText = computed(() => {
  if (scope.value === 'question') return '还没有题目笔记'
  if (scope.value === 'standalone') return '还没有独立笔记'
  return '还没有笔记'
})

/** 正文首行即标题（ADR-0055：不给笔记加 title 列） */
function noteTitle(content: string): string {
  const first = (content || '').split('\n')[0].trim()
  return first.length > 40 ? first.slice(0, 40) + '…' : first
}

/** 首行之后的摘要（单行化，超长截断） */
function noteExcerpt(content: string): string {
  const rest = (content || '').split('\n').slice(1).join(' ').trim()
  return rest.length > 80 ? rest.slice(0, 80) + '…' : rest
}

/** 题目徽标：带回题干摘要时显示摘要，否则退回通用文案 */
function questionLabel(n: NoteItem): string {
  const t = (n.question_content || '').trim()
  if (!t) return '题目笔记'
  return t.length > 18 ? '题目：' + t.slice(0, 18) + '…' : '题目：' + t
}

function handleScopeChange() {
  page.value = 1
  void loadData()
}

// ===== 新建 / 编辑共用同一对话框：editingId 为空即新建独立笔记 =====
const dialogVisible = ref(false)
const saving = ref(false)
const draft = ref('')
const editingId = ref<number | null>(null)

function openCreate() {
  editingId.value = null
  draft.value = ''
  dialogVisible.value = true
}

function openEdit(n: NoteItem) {
  editingId.value = n.id
  draft.value = n.content
  dialogVisible.value = true
}

async function handleSave() {
  const content = draft.value.trim()
  if (!content) {
    ElMessage.warning('笔记内容不能为空')
    return
  }
  saving.value = true
  try {
    if (editingId.value) {
      await noteApi.update(editingId.value, { content })
      ElMessage.success('已保存')
    } else {
      await noteApi.create({ content })
      ElMessage.success('已新建')
    }
    dialogVisible.value = false
    await loadData()
  } catch (e) {
    // 失败保持对话框打开（输入不丢），错误由请求拦截器统一提示
  } finally {
    saving.value = false
  }
}

async function handleDelete(n: NoteItem) {
  try {
    await useConfirm().confirmDanger('确定删除这条笔记？', '提示')
  } catch (e) {
    return // 取消
  }
  try {
    await noteApi.remove(n.id)
    ElMessage.success('已删除')
    await loadData()
  } catch (e) {}
}

onMounted(() => {
  void loadData()
})
</script>
