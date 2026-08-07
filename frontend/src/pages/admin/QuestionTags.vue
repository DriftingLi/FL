<template>
  <div class="question-tags-page">
    <div class="page-header">
      <h2>题库标签管理</h2>
      <el-button type="primary" @click="openTagDialog()">
        <el-icon><Plus /></el-icon> 新增标签
      </el-button>
    </div>

    <el-row :gutter="16" class="tags-layout">
      <el-col :xs="24" :md="10" :lg="8">
        <el-card shadow="never" class="tags-card">
          <template #header>
            <span>考点标签</span>
            <el-tag size="small" type="info" class="tags-count">{{ tags.length }}</el-tag>
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
                  <el-button link size="small" @click="openTagDialog(tag)">编辑</el-button>
                  <el-popconfirm title="删除标签不会删除题目，仅解除关联，确定？" @confirm="handleDeleteTag(tag)">
                    <template #reference>
                      <el-button link size="small" type="danger">删除</el-button>
                    </template>
                  </el-popconfirm>
                </span>
              </div>
            </div>
            <el-empty v-if="!tagsLoading && tags.length === 0" description="暂无标签" :image-size="60" />
          </div>
        </el-card>
      </el-col>

      <el-col :xs="24" :md="14" :lg="16">
        <el-card shadow="never" class="questions-card">
          <template #header>
            <span>{{ currentTagId ? `打标「${currentTagName}」下的题目` : '全部题目 · 选择题目并指定标签' }}</span>
          </template>

          <div class="filter-bar">
            <el-input
              v-model="keyword"
              placeholder="搜索题干"
              clearable
              style="width: 220px"
              @keyup.enter="loadQuestions"
              @clear="loadQuestions"
            />
            <el-select v-model="filterType" placeholder="题型" clearable style="width: 130px" @change="loadQuestions">
              <el-option v-for="o in questionTypeOptions" :key="o.value" :label="o.label" :value="o.value" />
            </el-select>
            <el-button type="primary" @click="loadQuestions">查询</el-button>
            <el-button
              v-if="selectedIds.length > 0"
              type="success"
              @click="openTagAssign"
            >
              批量打标 ({{ selectedIds.length }})
            </el-button>
          </div>

          <el-table
            :data="questions"
            v-loading="questionsLoading"
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
                  <el-tag v-for="t in row.tags" :key="t.id" size="small" class="question-tag" type="primary" effect="plain">
                    {{ t.name }}
                  </el-tag>
                </template>
                <span v-else class="no-tag">未打标</span>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="80" fixed="right" align="center">
              <template #default="{ row }">
                <el-button link size="small" type="primary" @click="openTagAssign([row.id])">打标</el-button>
              </template>
            </el-table-column>
          </el-table>

          <div class="pagination-wrapper" v-if="total > pageSize">
            <el-pagination
              v-model:current-page="currentPage"
              v-model:page-size="pageSize"
              :total="total"
              :page-sizes="[10, 20, 50]"
              layout="total, sizes, prev, pager, next"
              @size-change="currentPage = 1; loadQuestions()"
              @current-change="loadQuestions"
            />
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 标签新建/编辑 -->
    <el-dialog v-model="tagDialogVisible" :title="tagForm.id ? '编辑标签' : '新增标签'" width="420px" destroy-on-close>
      <el-form ref="tagFormRef" :model="tagForm" :rules="tagRules" label-width="80px">
        <el-form-item label="标签名" prop="name">
          <el-input v-model="tagForm.name" placeholder="如：法规、结构、液压、电气、制动、故障诊断、应急" maxlength="30" show-word-limit @keyup.enter="submitTag" />
        </el-form-item>
        <el-form-item label="编码" prop="code">
          <el-input v-model="tagForm.code" placeholder="唯一编码，如 HYDRAULIC" maxlength="30" @keyup.enter="submitTag" />
        </el-form-item>
        <el-form-item label="考点模块">
          <el-input v-model="tagForm.category" placeholder="如：液压、电气、制动（可选）" maxlength="30" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="tagDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="tagSubmitting" @click="submitTag">保存</el-button>
      </template>
    </el-dialog>

    <!-- 打标对话框 -->
    <el-dialog v-model="tagAssignVisible" :title="`题目打标（${tagAssignQuestionIds.length} 题）`" width="460px" destroy-on-close>
      <el-form label-width="80px">
        <el-form-item label="选择标签">
          <el-select
            v-model="tagAssignTagIds"
            multiple
            filterable
            collapse-tags
            placeholder="选择要关联的标签"
            style="width: 100%"
          >
            <el-option v-for="t in tags" :key="t.id" :label="t.name" :value="t.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="tagAssignVisible = false">取消</el-button>
        <el-button type="primary" :loading="tagSubmitting" @click="submitTagAssign">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { Plus } from '@element-plus/icons-vue'
import { ElMessage, type FormInstance } from 'element-plus'
import { trainingApi, type QuestionTag } from '@/api/training'
import { questionBankApi } from '@/api/questionBank'
import { questionTypeOptions, typeMap } from '@/constants/question'
import type { Question } from '@/types/question'

const tags = ref<QuestionTag[]>([])
const tagsLoading = ref(false)
const currentTagId = ref<number | null>(null)

const questions = ref<Question[]>([])
const questionsLoading = ref(false)
const keyword = ref('')
const filterType = ref('')
const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(0)
const selectedIds = ref<number[]>([])

const currentTagName = computed(() => {
  const tag = tags.value.find(t => t.id === currentTagId.value)
  return tag?.name || ''
})

const tagDialogVisible = ref(false)
const tagSubmitting = ref(false)
const tagFormRef = ref<FormInstance | null>(null)
const tagForm = reactive<{ id: number | null; name: string; code: string; category: string }>({ id: null, name: '', code: '', category: '' })
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
    const res = await trainingApi.getQuestionTags()
    if (res.code === 200) {
      tags.value = res.data.tags || []
    }
  } catch (error) {
    console.error('加载标签失败:', error)
    ElMessage.error('加载标签失败')
  } finally {
    tagsLoading.value = false
  }
}

async function loadQuestions() {
  questionsLoading.value = true
  try {
    const params: Record<string, unknown> = {
      page: currentPage.value,
      page_size: pageSize.value
    }
    if (currentTagId.value) params.tag_id = currentTagId.value
    if (keyword.value) params.keyword = keyword.value
    if (filterType.value) params.type = filterType.value

    const res = await questionBankApi.getQuestions(params as never)
    if (res.code === 200) {
      questions.value = res.data.questions || []
      total.value = res.data.total || 0
    }
  } catch (error) {
    console.error('加载题目失败:', error)
    ElMessage.error('加载题目失败')
  } finally {
    questionsLoading.value = false
  }
}

function selectTag(tagId: number) {
  currentTagId.value = currentTagId.value === tagId ? null : tagId
  currentPage.value = 1
  loadQuestions()
}

function handleSelectionChange(rows: Question[]) {
  selectedIds.value = rows.map(r => r.id)
}

function openTagDialog(tag?: QuestionTag) {
  tagForm.id = tag?.id ?? null
  tagForm.name = tag?.name ?? ''
  tagForm.code = tag?.code ?? ''
  tagForm.category = tag?.category ?? ''
  tagDialogVisible.value = true
}

async function submitTag() {
  if (!tagFormRef.value) return
  await tagFormRef.value.validate()
  tagSubmitting.value = true
  try {
    const payload = {
      name: tagForm.name,
      code: tagForm.code,
      category: tagForm.category || undefined
    }
    if (tagForm.id) {
      const res = await trainingApi.updateQuestionTag(tagForm.id, payload)
      if (res.code === 200) ElMessage.success('标签已更新')
    } else {
      const res = await trainingApi.createQuestionTag(payload)
      if (res.code === 201) ElMessage.success('标签已创建')
    }
    tagDialogVisible.value = false
    await loadTags()
  } catch (error) {
    console.error('保存标签失败:', error)
    ElMessage.error('保存失败')
  } finally {
    tagSubmitting.value = false
  }
}

async function handleDeleteTag(tag: QuestionTag) {
  try {
    const res = await trainingApi.deleteQuestionTag(tag.id)
    if (res.code === 200) {
      ElMessage.success('标签已删除')
      if (currentTagId.value === tag.id) currentTagId.value = null
      await loadTags()
      loadQuestions()
    }
  } catch (error) {
    console.error('删除标签失败:', error)
    ElMessage.error('删除失败')
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
    let ok = true
    for (const qid of tagAssignQuestionIds.value) {
      const res = await trainingApi.setQuestionTags(qid, tagAssignTagIds.value)
      if (res.code !== 200) ok = false
    }
    if (ok) {
      ElMessage.success('打标完成')
      tagAssignVisible.value = false
      loadQuestions()
    }
  } catch (error) {
    console.error('打标失败:', error)
    ElMessage.error('打标失败')
  } finally {
    tagSubmitting.value = false
  }
}

onMounted(() => {
  loadTags()
  loadQuestions()
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
  font-size: 22px;
  color: #303133;
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
  border-bottom: 1px solid #f0f2f5;
  cursor: pointer;
  border-radius: 6px;
  transition: background 0.2s;
}

.tag-row:hover {
  background: #f5f7fa;
}

.tag-row.active {
  background: #ecf5ff;
}

.tag-name {
  flex: 1;
  font-size: 14px;
  color: #303133;
  font-weight: 500;
}

.tag-count {
  font-size: 12px;
  color: #909399;
}

.tag-actions {
  opacity: 0;
  transition: opacity 0.2s;
}

.tag-row:hover .tag-actions {
  opacity: 1;
}

.filter-bar {
  display: flex;
  gap: 8px;
  margin-bottom: 14px;
  flex-wrap: wrap;
}

.question-tag {
  margin-right: 6px;
  margin-bottom: 4px;
}

.no-tag {
  font-size: 12px;
  color: #c0c4cc;
}

.pagination-wrapper {
  display: flex;
  justify-content: center;
  margin-top: 16px;
}

@media screen and (max-width: 768px) {
  .question-tags-page {
    padding: 12px;
  }
  .tag-actions {
    opacity: 1;
  }
}
</style>
