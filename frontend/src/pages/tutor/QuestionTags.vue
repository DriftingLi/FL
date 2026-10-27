<template>
  <div class="question-tags-page">
    <div class="page-header">
      <h2>题库标签管理</h2>
      <UiButton variant="primary" @click="openTagDialog()">
        <UiIcon name="plus" /> 新增标签
      </UiButton>
    </div>

    <el-row :gutter="16" class="tags-layout">
      <el-col :xs="24" :md="10" :lg="8">
        <UiCard divider class="tags-card">
          <template #header>
            <span>考点标签</span>
          </template>
          <template #actions>
            <UiTag size="small" tone="info" class="tags-count">{{ tags.length }}</UiTag>
          </template>
          <div v-loading="tagsLoading">
            <div class="tag-list">
              <div
                v-for="tag in tags"
                :key="tag.id"
                class="tag-row"
                :class="{ active: currentTagId === tag.id }"
                @click="selectTag(tag.id)"
              >
                <span class="tag-name">{{ tag.name }}</span>
                <span class="tag-count">{{ tag.question_count ?? 0 }} 题</span>
                <span class="tag-actions" @click.stop>
                  <UiButton link size="small" @click="openTagDialog(tag)">编辑</UiButton>
                  <el-popconfirm title="删除标签不会删除题目，仅解除关联，确定？" @confirm="handleDeleteTag(tag)">
                    <template #reference>
                      <UiButton variant="danger" link size="small">删除</UiButton>
                    </template>
                  </el-popconfirm>
                </span>
              </div>
            </div>
            <UiEmptyState v-if="!tagsLoading && tags.length === 0" description="暂无标签" size="sm" />
          </div>
        </UiCard>
      </el-col>

      <el-col :xs="24" :md="14" :lg="16">
        <UiCard divider class="questions-card">
          <template #header>
            <span>{{ currentTagId ? `打标「${currentTagName}」下的题目` : '全部题目 · 选择题目并指定标签' }}</span>
          </template>

          <UiFilterBar>
        <template #filters>

            <UiInput
              v-model="searchKeyword"
              placeholder="搜索题干"
              clearable
              style="width: 220px"
              @keyup.enter="search"
              @clear="search"
            />
            <UiSelect
              v-model="filterType"
              :options="questionTypeOptions"
              placeholder="题型"
              clearable
              style="width: 130px"
              @change="applyQuestionFilters"
            />
            <UiButton variant="primary" @click="search">查询</UiButton>
            <UiButton variant="success" v-if="selectedIds.length > 0" @click="openTagAssign(selectedIds)">
              批量打标 ({{ selectedIds.length }})
            </UiButton>
        </template>
      </UiFilterBar>

          <el-table
            :data="list"
            v-loading="loading"
            stripe
            border
            size="small"
            @selection-change="handleSelectionChange"
          >
            <el-table-column type="selection" width="45" />
            <el-table-column prop="id" label="ID" width="60" align="center" />
            <el-table-column prop="type" label="题型" width="90">
              <template #default="{ row }">{{ (typeMap as Record<string, string>)[row.type] || row.type }}</template>
            </el-table-column>
            <el-table-column prop="content" label="题干" min-width="220" show-overflow-tooltip />
            <el-table-column label="标签" min-width="160">
              <template #default="{ row }">
                <template v-if="row.tags && row.tags.length > 0">
                  <UiTag v-for="t in row.tags" :key="t.id" size="small" class="question-tag" tone="primary" effect="plain">
                    {{ t.name }}
                  </UiTag>
                </template>
                <span v-else class="no-tag">未打标</span>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="80" fixed="right" align="center">
              <template #default="{ row }">
                <UiButton variant="primary" link size="small" @click="openTagAssign([row.id])">打标</UiButton>
              </template>
            </el-table-column>
          </el-table>

          <UiPagination
            v-if="total > pageSize"
            v-model:current-page="currentPage"
            v-model:page-size="pageSize"
            :total="total"
            show-sizes
            :page-sizes="[10, 20, 50]"
            align="center"
            @current-change="load"
            @size-change="search"
          />
        </UiCard>
      </el-col>
    </el-row>

    <!-- 标签新建/编辑 -->
    <UiDialog v-model="tagDialogVisible" :title="tagForm.id ? '编辑标签' : '新增标签'" width="420px" destroy-on-close confirm-text="保存" :confirm-loading="tagSubmitting" @confirm="submitTag">
      <el-form ref="tagFormRef" :model="tagForm" :rules="tagRules" label-width="80px">
        <el-form-item label="标签名" prop="name">
          <UiInput v-model="tagForm.name" placeholder="如：法规、结构、液压、电气、制动、故障诊断、应急" :maxlength="30" show-word-limit @keyup.enter="submitTag" />
        </el-form-item>
        <el-form-item label="编码" prop="code">
          <UiInput v-model="tagForm.code" placeholder="唯一编码，如 HYDRAULIC" :maxlength="30" @keyup.enter="submitTag" />
        </el-form-item>
      </el-form>
    </UiDialog>

    <!-- 打标对话框 -->
    <UiDialog v-model="tagAssignVisible" :title="`题目打标（${tagAssignQuestionIds.length} 题）`" width="460px" destroy-on-close confirm-text="保存" :confirm-loading="tagSubmitting" @confirm="submitTagAssign">
      <el-form label-width="80px">
        <el-form-item label="选择标签">
          <UiSelect
            v-model="tagAssignTagIds"
            :options="tagOptions"
            multiple
            filterable
            collapse-tags
            placeholder="选择要关联的标签"
            style="width: 100%"
          />
        </el-form-item>
      </el-form>
    </UiDialog>
  </div>
</template>

<script setup lang="ts">
import UiIcon from '@/components/ui/UiIcon.vue'
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, type FormInstance } from 'element-plus'
import { trainingApi, type QuestionTag } from '@/api/training'
import { questionBankApi, type QuestionsQuery } from '@/api/questionBank'
import { questionTypeOptions, typeMap } from '@/constants/question'
import { useAdminTable } from '@/composables/useAdminTable'
import type { Question } from '@/types/question'
import UiButton from '@/components/ui/UiButton.vue'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiPagination from '@/components/ui/UiPagination.vue'
import UiFilterBar from '@/components/ui/UiFilterBar.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiTag from '@/components/ui/UiTag.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiSelect, { type UiSelectOption } from '@/components/ui/UiSelect.vue'

const tags = ref<QuestionTag[]>([])
/** 标签 → UiSelect 的 {label,value}（打标对话框的多选） */
const tagOptions = computed<UiSelectOption[]>(() => tags.value.map((t) => ({ label: t.name, value: t.id })))
const tagsLoading = ref(false)
const currentTagId = ref<number | null>(null)

const filterType = ref('')
const selectedIds = ref<number[]>([])

const currentTagName = computed(() => {
  const tag = tags.value.find(t => t.id === currentTagId.value)
  return tag?.name || ''
})

const tagDialogVisible = ref(false)
const tagSubmitting = ref(false)
const tagFormRef = ref<FormInstance | null>(null)
const tagForm = reactive<{ id: number | null; name: string; code: string }>({ id: null, name: '', code: '' })
const tagRules = {
  name: [{ required: true, message: '请输入标签名', trigger: 'blur' }],
  code: [{ required: true, message: '请输入唯一编码', trigger: 'blur' }]
}

const tagAssignVisible = ref(false)
const tagAssignQuestionIds = ref<number[]>([])
const tagAssignTagIds = ref<number[]>([])

async function loadTags() {
  tagsLoading.value = true
  try {
    // 拦截器已解包信封
    const data = await trainingApi.getQuestionTags()
    tags.value = data.tags || []
  } catch (error) {
    console.error('加载标签失败:', error)
    /* 错误已由拦截器提示 */
  } finally {
    tagsLoading.value = false
  }
}

// 题目列表状态机：tags 侧栏仍为局部状态，questions 表格收编到 useAdminTable
const table = useAdminTable<Question>({
  fetch: (paging, filters) => {
    const params: QuestionsQuery = {
      page: paging.page,
      page_size: paging.pageSize
    }
    if (filters.tag_id) params.tag_id = Number(filters.tag_id)
    if (filters.keyword) params.keyword = String(filters.keyword)
    if (filters.type) params.type = String(filters.type)
    return questionBankApi.getQuestions(params)
  },
  searchable: true
})
const { loading, list, total, currentPage, pageSize, searchKeyword, load, search } = table

function applyQuestionFilters() {
  table.applyFilters({
    tag_id: currentTagId.value ?? '',
    type: filterType.value
  })
}

function selectTag(tagId: number) {
  currentTagId.value = currentTagId.value === tagId ? null : tagId
  applyQuestionFilters()
}

function handleSelectionChange(rows?: Question[] | undefined) {
  selectedIds.value = rows ? rows.map(r => r.id) : []
}

function openTagDialog(tag?: QuestionTag) {
  tagForm.id = tag?.id ?? null
  tagForm.name = tag?.name ?? ''
  tagForm.code = tag?.code ?? ''
  tagDialogVisible.value = true
}

async function submitTag() {
  if (!tagFormRef.value) return
  await tagFormRef.value.validate()
  tagSubmitting.value = true
  try {
    const payload = {
      name: tagForm.name,
      code: tagForm.code
    }
    if (tagForm.id) {
      await trainingApi.updateQuestionTag(tagForm.id, payload)
      ElMessage.success('标签已更新')
    } else {
      await trainingApi.createQuestionTag(payload)
      ElMessage.success('标签已创建')
    }
    tagDialogVisible.value = false
    await loadTags()
  } catch (error) {
    console.error('保存标签失败:', error)
    /* 错误已由拦截器提示 */
  } finally {
    tagSubmitting.value = false
  }
}

async function handleDeleteTag(tag: QuestionTag) {
  try {
    await trainingApi.deleteQuestionTag(tag.id)
    ElMessage.success('标签已删除')
    if (currentTagId.value === tag.id) currentTagId.value = null
    await loadTags()
    load()
  } catch (error) {
    console.error('删除标签失败:', error)
    /* 错误已由拦截器提示 */
  }
}

function openTagAssign(questionIds: number[]) {
  tagAssignQuestionIds.value = questionIds
  tagAssignTagIds.value = []
  tagAssignVisible.value = true
}

async function submitTagAssign() {
  tagSubmitting.value = true
  try {
    for (const qid of tagAssignQuestionIds.value) {
      await trainingApi.setQuestionTags(qid, tagAssignTagIds.value)
    }
    ElMessage.success('打标完成')
    tagAssignVisible.value = false
    load()
  } catch (error) {
    console.error('打标失败:', error)
    /* 错误已由拦截器提示 */
  } finally {
    tagSubmitting.value = false
  }
}

onMounted(() => {
  loadTags()
  load()
})
</script>

<style scoped>
.question-tags-page {
  padding: 20px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.page-header h2 {
  font-size: var(--text-2xl);
  color: var(--color-text-primary);
}

.tags-layout {
  align-items: flex-start;
}

.tags-count {
  margin-left: 8px;
}

.tag-list {
  max-height: 640px;
  overflow-y: auto;
}

.tag-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  border-bottom: 1px solid var(--color-bg-page);
  cursor: pointer;
  border-radius: var(--radius-md);
  transition: background var(--duration-base) var(--ease-default);
}

.tag-row:hover {
  background: var(--color-bg-page);
}

.tag-row.active {
  /* 固定色阶 primary-50 在深色下不翻转（#F0FDFA），而 .tag-name 用的是翻转的 text-primary
     ⇒ 深色下白字压浅青底读不清。改成品牌色浅混面板 token（同 #1750 的做法）。 */
  background: color-mix(in srgb, var(--color-primary-500) 12%, var(--color-bg-card));
}

.tag-name {
  flex: 1;
  font-size: var(--text-sm);
  color: var(--color-text-primary);
  font-weight: 500;
}

.tag-count {
  font-size: var(--text-xs);
  color: var(--color-text-tertiary);
}

.tag-actions {
  opacity: 0;
  transition: opacity var(--duration-base) var(--ease-default);
}

.tag-row:hover .tag-actions {
  opacity: 1;
}


.question-tag {
  margin-right: 6px;
  margin-bottom: 4px;
}

.no-tag {
  font-size: var(--text-xs);
  color: var(--color-text-disabled);
}

@media screen and (max-width: 768px) {
  .question-tags-page {
    padding: 12px;
  }
  .tag-actions {
    opacity: 1;
  }
  .tag-name {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
}
</style>
