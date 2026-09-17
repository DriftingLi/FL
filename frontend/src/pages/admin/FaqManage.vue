<template>
  <div>
    <div class="mb-3 flex items-center justify-between">
      <h2 class="m-0">帮助中心管理</h2>
      <UiButton variant="primary" size="small" @click="openCreate">
        {{ tab === 'categories' ? '新建分类' : '新建条目' }}
      </UiButton>
    </div>

    <UiSegmentTabs v-model="tab" :options="tabOptions" class="mb-4" @change="handleTabChange" />

    <UiAsyncSection
      :error="loadError"
      :loading="loading"
      :empty="false"
      :retrying="retrying"
      error-title="帮助中心管理数据加载失败"
      error-description="网络或服务端异常，可重试"
      @retry="retryLoad"
    >
      <template #skeleton>
        <UiSkeleton variant="card" :count="3" />
      </template>

      <!-- 分类 -->
      <el-table v-if="tab === 'categories'" :data="categories" empty-text="暂无分类">
        <el-table-column prop="code" label="标识" width="160" />
        <el-table-column prop="title" label="名称" min-width="160" />
        <el-table-column prop="sort_order" label="排序" width="80" />
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <UiTag size="small" :tone="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '停用' }}</UiTag>
          </template>
        </el-table-column>
        <el-table-column prop="entry_count" label="条目数" width="90" />
        <el-table-column label="操作" width="150">
          <template #default="{ row }">
            <UiButton size="small" @click="openEditCategory(row)">编辑</UiButton>
            <UiButton class="ml-2" variant="danger" size="small" @click="handleDeleteCategory(row)">删除</UiButton>
          </template>
        </el-table-column>
      </el-table>

      <!-- 条目 -->
      <template v-else>
        <div class="mb-3 flex items-center gap-2">
          <span class="text-[13px] text-ink-3">按分类筛选</span>
          <el-select v-model="entryFilterCategoryId" placeholder="全部分类" clearable style="width: 200px" @change="handleEntryFilterChange">
            <el-option v-for="c in categories" :key="c.id" :label="c.title" :value="c.id" />
          </el-select>
        </div>
        <el-table :data="entries" empty-text="暂无条目">
          <el-table-column prop="category_code" label="分类" width="140" />
          <el-table-column prop="question" label="问题" min-width="240" show-overflow-tooltip />
          <el-table-column prop="sort_order" label="排序" width="80" />
          <el-table-column label="状态" width="90">
            <template #default="{ row }">
              <UiTag size="small" :tone="row.published ? 'success' : 'info'">{{ row.published ? '已发布' : '停用' }}</UiTag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="150">
            <template #default="{ row }">
              <UiButton size="small" @click="openEditEntry(row)">编辑</UiButton>
              <UiButton class="ml-2" variant="danger" size="small" @click="handleDeleteEntry(row)">删除</UiButton>
            </template>
          </el-table-column>
        </el-table>
      </template>
    </UiAsyncSection>

    <!-- 分类对话框 -->
    <UiDialog
      v-model="categoryDialog"
      :title="editingCategoryId ? '编辑分类' : '新建分类'"
      width="480px"
      confirm-text="保存"
      :confirm-loading="saving"
      @confirm="saveCategory"
    >
      <div class="mb-3">
        <div class="mb-1 text-[13px] text-ink-3">标识（小写字母 / 数字 / 下划线，2-64 位）</div>
        <el-input v-model="categoryForm.code" placeholder="如 account" />
      </div>
      <div class="mb-3">
        <div class="mb-1 text-[13px] text-ink-3">名称</div>
        <el-input v-model="categoryForm.title" placeholder="如 账号与登录" />
      </div>
      <div class="mb-3">
        <div class="mb-1 text-[13px] text-ink-3">排序（小的在前）</div>
        <el-input-number v-model="categoryForm.sort_order" :min="0" />
      </div>
      <UiCheckbox v-model="categoryForm.enabled">启用（停用后学员端不再展示该分类）</UiCheckbox>
    </UiDialog>

    <!-- 条目的对话框 -->
    <UiDialog
      v-model="entryDialog"
      :title="editingEntryId ? '编辑条目' : '新建条目'"
      width="640px"
      confirm-text="保存"
      :confirm-loading="saving"
      @confirm="saveEntry"
    >
      <div class="mb-3">
        <div class="mb-1 text-[13px] text-ink-3">所属分类</div>
        <el-select v-model="entryForm.category_id" placeholder="请选择分类" style="width: 100%">
          <el-option v-for="c in categories" :key="c.id" :label="c.title" :value="c.id" />
        </el-select>
      </div>
      <div class="mb-3">
        <div class="mb-1 text-[13px] text-ink-3">问题</div>
        <el-input v-model="entryForm.question" maxlength="200" show-word-limit placeholder="学员会怎么问？" />
      </div>
      <div class="mb-3">
        <div class="mb-1 text-[13px] text-ink-3">答案（纯文本，换行原样展示）</div>
        <el-input v-model="entryForm.answer" type="textarea" :rows="6" maxlength="4000" show-word-limit />
      </div>
      <div class="mb-3">
        <div class="mb-1 text-[13px] text-ink-3">排序（小的在前）</div>
        <el-input-number v-model="entryForm.sort_order" :min="0" />
      </div>
      <UiCheckbox v-model="entryForm.published">发布（停用后学员端不可见）</UiCheckbox>
    </UiDialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import {
  faqApi,
  type AdminFaqCategory,
  type AdminFaqCategoryPayload,
  type AdminFaqEntry,
  type AdminFaqEntryPayload
} from '@/api/faq'
import { useAsyncPage } from '@/composables/useAsyncPage'
import { useConfirm } from '@/composables/useConfirm'
import UiAsyncSection from '@/components/ui/UiAsyncSection.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiCheckbox from '@/components/ui/UiCheckbox.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiSegmentTabs from '@/components/ui/UiSegmentTabs.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiTag from '@/components/ui/UiTag.vue'

const tabOptions = [
  { label: '分类', value: 'categories' },
  { label: '条目', value: 'entries' }
]

const tab = ref('categories')
const categories = ref<AdminFaqCategory[]>([])
const entries = ref<AdminFaqEntry[]>([])
const entryFilterCategoryId = ref<number | undefined>(undefined)

const { loading, loadError, retrying, retry: retryLoad, run: loadData } = useAsyncPage(async () => {
  const [cats, list] = await Promise.all([
    faqApi.adminListCategories(),
    faqApi.adminListEntries(entryFilterCategoryId.value ? { category_id: entryFilterCategoryId.value } : undefined)
  ])
  categories.value = cats?.categories || []
  entries.value = list?.entries || []
})

function handleTabChange() {
  // 切页不重新拉数据：两个清单一次取齐（条目量小），避免来回切换反复请求
}

function handleEntryFilterChange() {
  void loadData()
}

// ===== 分类 =====
const categoryDialog = ref(false)
const editingCategoryId = ref<number | null>(null)
const categoryForm = ref<AdminFaqCategoryPayload>({ code: '', title: '', sort_order: 0, enabled: true })

function openEditCategory(row: AdminFaqCategory) {
  editingCategoryId.value = row.id
  categoryForm.value = { code: row.code, title: row.title, sort_order: row.sort_order, enabled: row.enabled }
  categoryDialog.value = true
}

function openCreate() {
  if (tab.value === 'categories') {
    editingCategoryId.value = null
    categoryForm.value = { code: '', title: '', sort_order: 0, enabled: true }
    categoryDialog.value = true
    return
  }
  openCreateEntry()
}

const saving = ref(false)

async function saveCategory() {
  saving.value = true
  try {
    if (editingCategoryId.value) {
      await faqApi.adminUpdateCategory(editingCategoryId.value, categoryForm.value)
    } else {
      await faqApi.adminCreateCategory(categoryForm.value)
    }
    ElMessage.success('已保存')
    categoryDialog.value = false
    await loadData()
  } catch (e) {
    // 失败保持对话框打开（输入不丢）；错误文案由拦截器统一提示
  } finally {
    saving.value = false
  }
}

async function handleDeleteCategory(row: AdminFaqCategory) {
  try {
    await useConfirm().confirmDanger(
      `确定删除分类「${row.title}」？其下 ${row.entry_count} 条条目会一并删除，且不可恢复。`,
      '删除分类'
    )
  } catch (e) {
    return
  }
  try {
    await faqApi.adminDeleteCategory(row.id)
    ElMessage.success('已删除')
    await loadData()
  } catch (e) {}
}

// ===== 条目 =====
const entryDialog = ref(false)
const editingEntryId = ref<number | null>(null)
const entryForm = ref<AdminFaqEntryPayload>({ category_id: 0, question: '', answer: '', sort_order: 0, published: true })

function openCreateEntry() {
  editingEntryId.value = null
  entryForm.value = {
    category_id: entryFilterCategoryId.value || categories.value[0]?.id || 0,
    question: '',
    answer: '',
    sort_order: 0,
    published: true
  }
  entryDialog.value = true
}

function openEditEntry(row: AdminFaqEntry) {
  editingEntryId.value = row.id
  entryForm.value = {
    category_id: row.category_id,
    question: row.question,
    answer: row.answer,
    sort_order: row.sort_order,
    published: row.published
  }
  entryDialog.value = true
}

async function saveEntry() {
  if (!entryForm.value.category_id) {
    ElMessage.warning('请先选择所属分类')
    return
  }
  saving.value = true
  try {
    if (editingEntryId.value) {
      await faqApi.adminUpdateEntry(editingEntryId.value, entryForm.value)
    } else {
      await faqApi.adminCreateEntry(entryForm.value)
    }
    ElMessage.success('已保存')
    entryDialog.value = false
    await loadData()
  } catch (e) {
  } finally {
    saving.value = false
  }
}

async function handleDeleteEntry(row: AdminFaqEntry) {
  try {
    await useConfirm().confirmDanger(`确定删除条目「${row.question}」？`, '删除条目')
  } catch (e) {
    return
  }
  try {
    await faqApi.adminDeleteEntry(row.id)
    ElMessage.success('已删除')
    await loadData()
  } catch (e) {}
}

onMounted(() => {
  void loadData()
})
</script>
