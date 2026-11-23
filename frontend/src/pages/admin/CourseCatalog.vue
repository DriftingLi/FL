<!--
  列表档位：useAsyncPage（只读计数）—— 课程目录档：目录树 + facet 计数 + 课程汇总卡
  （非服务端分页列表：筛选/分页在客户端，故不套 useAdminTable；依据 ADR-0056 §9）
  判定口径见 docs/agents/ui-conventions.md「管理端列表两档归属」。
-->
<template>
  <div class="cc-layout">
    <!-- 左侧目录导航 -->
    <aside class="cc-sidebar">
      <div class="cc-card cc-actions-card">
        <UiButton variant="primary" plain size="small" @click="openCategoryDialog()">新增分类</UiButton>
        <UiButton variant="ghost" size="small" @click="openCertificateDialog()">证书模板</UiButton>
      </div>
      <FacetCard title="课程分类">
        <FacetItem :active="categoryId === null" name="全部课程" :count="allCourses.length" @select="selectCategory(null)" />
        <div v-for="category in categories" :key="category.id" class="flex items-center gap-1">
          <FacetItem
            class="min-w-0 flex-1"
            :active="categoryId === category.id"
            :name="category.status === 0 ? category.name + '（停用）' : category.name"
            :count="categoryCounts.get(category.id) ?? 0"
            @select="selectCategory(category.id)"
          />
          <UiButton variant="text" size="small" :aria-label="'编辑分类 ' + category.name" @click="openCategoryDialog(category)">编辑</UiButton>
        </div>
        <FacetItem
          v-if="unclassifiedCount > 0"
          warn
          :active="categoryId === UNCLASSIFIED"
          name="待补分类"
          :count="unclassifiedCount"
          @select="selectCategory(UNCLASSIFIED)"
        />
      </FacetCard>
    </aside>

    <!-- 右侧课程表格 -->
    <main class="cc-main">
      <UiFilterBar>
        <template #filters>

        <UiInput v-model="keyword" placeholder="搜索课程名称…" clearable class="cc-search" @input="currentPage = 1">
          <template #prefix>
            <UiIcon name="search" />
          </template>
        </UiInput>
        <UiSelect v-model="filterStatus" placeholder="状态" clearable style="width: 120px" @change="currentPage = 1" :options="[{ label: '已上架', value: 1 }, { label: '未上架', value: 0 }]" />
        <UiSelect v-model="filterHotFeatured" placeholder="热门/精品" clearable style="width: 140px" @change="currentPage = 1" :options="[{ label: '热门', value: 'hot' }, { label: '精品', value: 'featured' }, { label: '全部', value: 'all' }]" />
        <UiButton variant="primary" @click="openDrawer()">新增课程</UiButton>
        </template>
      </UiFilterBar>

      <el-table :data="pagedCourses" v-loading="loading" style="width: 100%">
        <el-table-column label="分类" width="140">
          <template #default="{ row }">
            <UiTag v-if="row.category_id" size="small" effect="plain">{{ categoryNameOf(row.category_id) }}</UiTag>
            <UiTag v-else tone="danger" size="small">待补分类</UiTag>
          </template>
        </el-table-column>
        <el-table-column label="课程名称" min-width="220" show-overflow-tooltip>
          <template #default="{ row }">
            <UiTag v-if="row.status === 0" tone="info" size="small">草稿</UiTag>
            <span class="cc-cell-name">{{ row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="chapter_count" label="章节" width="70" align="center" />
        <el-table-column label="学时" width="130">
          <template #default="{ row }">
            <span class="cc-cell-dim">理论{{ row.theory_hours || 0 }} / 实操{{ row.practice_hours || 0 }}</span>
          </template>
        </el-table-column>
        <el-table-column label="证书" min-width="150" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="cc-cell-dim">{{ certificateNameOf(row.certificate_template_id) || '—' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="热门" width="90" align="center">
          <template #default="{ row }">
            <UiSwitch :model-value="!!row.is_hot" @change="(v: boolean) => toggleHot(row, v)" />
          </template>
        </el-table-column>
        <el-table-column label="精品" width="90" align="center">
          <template #default="{ row }">
            <UiSwitch :model-value="!!row.is_featured" @change="(v: boolean) => toggleFeatured(row, v)" />
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <UiTag :tone="row.status === 1 ? 'success' : 'info'" size="small">
              {{ row.status === 1 ? '已上架' : '未上架' }}
            </UiTag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="90" fixed="right" align="center">
          <template #default="{ row }">
            <UiMoreMenu trigger="text" :items="actionItems(row)" @select="(k: string) => handleAction(k, row)" />
          </template>
        </el-table-column>
        <template #empty>
          <UiEmptyState description="暂无课程" size="sm" />
        </template>
      </el-table>

      <div class="cc-pagination" v-if="filteredCourses.length > pageSize">
        <UiPagination
      v-model:current-page="currentPage"
      v-model:page-size="pageSize"
      :total="filteredCourses.length"
      show-sizes
      :page-sizes="[10, 20, 50]"
    />
      </div>
    </main>

    <!-- 课程编辑抽屉（全字段 + 章节管理）与弹窗表单已拆分为子组件 -->
    <CourseCatalogCourseDrawer
      ref="courseDrawerRef"
      :categories="categories"
      :certificate-templates="certificateTemplates"
      :course-options="courseOptions"
      :default-category-id="canSortCourses ? categoryId : null"
      :submitting="submitting"
      @saved="refreshCatalog"
    />
    <CourseCatalogDialogs
      ref="catalogDialogsRef"
      :certificate-templates="certificateTemplates"
      :submitting="submitting"
      @certificates-changed="loadCertificateTemplates"
    />
    <UiDialog
      v-model="categoryDialog"
      :title="editingCategoryId ? '编辑分类' : '新增分类'"
      width="420px"
      confirm-text="保存"
      :confirm-disabled="!categoryForm.name.trim()"
      :confirm-loading="savingCategory"
      @confirm="saveCategory"
    >
      <el-form :model="categoryForm" label-width="80px">
        <el-form-item label="分类名称" required><UiInput v-model="categoryForm.name" :maxlength="100" /></el-form-item>
        <el-form-item label="排序"><el-input-number v-model="categoryForm.sort_order" :min="0" :precision="0" /></el-form-item>
        <el-form-item label="状态">
          <UiSelect v-model="categoryForm.status" :options="[{ label: '启用', value: 1 }, { label: '停用', value: 0 }]" />
        </el-form-item>
      </el-form>
    </UiDialog>
  </div>
</template>

<script setup lang="ts">
import UiIcon from '@/components/ui/UiIcon.vue'
// keep-alive 的 include 按**组件名**匹配：名必须与路由名（页面描述符的 name）一致，
// 否则该页不会被缓存（#1620 的锁测试会拦下这种漂移）。
defineOptions({ name: 'CourseCatalog' })

import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { trainingApi, type TrainingCategoryDTO, type CertificateTemplate } from '@/api/training'
import { adminApi, type AdminCourseItem } from '@/api/admin'
import { useAsyncPage } from '@/composables/useAsyncPage'
import FacetCard from '@/components/catalog/FacetCard.vue'
import FacetItem from '@/components/catalog/FacetItem.vue'
import CourseCatalogCourseDrawer from '@/components/admin/CourseCatalogCourseDrawer.vue'
import CourseCatalogDialogs from '@/components/admin/CourseCatalogDialogs.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiPagination from '@/components/ui/UiPagination.vue'
import UiFilterBar from '@/components/ui/UiFilterBar.vue'
import { useConfirm } from '@/composables/useConfirm'
import UiTag from '@/components/ui/UiTag.vue'
import UiSwitch from '@/components/ui/UiSwitch.vue'
import UiMoreMenu, { type UiMoreMenuItem } from '@/components/ui/UiMoreMenu.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiSelect from '@/components/ui/UiSelect.vue'

const submitting = ref(false)

const UNCLASSIFIED = -1
const categoryId = ref<number | null>(null)
const categories = ref<TrainingCategoryDTO[]>([])
const allCourses = ref<AdminCourseItem[]>([])
const certificateTemplates = ref<CertificateTemplate[]>([])
const categoryDialog = ref(false)
const editingCategoryId = ref<number>()
const savingCategory = ref(false)
const categoryForm = ref<{ name: string; status: number; sort_order?: number }>({ name: '', status: 1 })

function openCategoryDialog(category?: TrainingCategoryDTO) {
  editingCategoryId.value = category?.id
  categoryForm.value = { name: category?.name || '', status: category?.status ?? 1, sort_order: category?.sort_order }
  categoryDialog.value = true
}

async function saveCategory() {
  if (!categoryForm.value.name.trim() || savingCategory.value) return
  savingCategory.value = true
  try {
    const id = editingCategoryId.value
    if (id) await trainingApi.updateTrainingCategory(id, categoryForm.value)
    else await trainingApi.createTrainingCategory(categoryForm.value)
    categoryDialog.value = false
    await refreshCatalog()
  } catch {
    /* 错误已由拦截器提示 */
  } finally {
    savingCategory.value = false
  }
}

const { loading, run: refreshCatalog } = useAsyncPage(async () => {
  const pageSize = 500
  const [categoryData, firstPage] = await Promise.all([
    trainingApi.getAdminCategories(),
    adminApi.getCourses({ page: 1, page_size: pageSize })
  ])
  const courses = [...firstPage.courses]
  for (let page = 2; page <= firstPage.pages; page++) {
    const nextPage = await adminApi.getCourses({ page, page_size: pageSize })
    courses.push(...nextPage.courses)
  }
  return { categories: categoryData.categories, courses }
}, {
  apply: (bundle) => {
    categories.value = bundle.categories
    allCourses.value = bundle.courses
  }
})

const categoryCounts = computed(() => {
  const counts = new Map<number, number>()
  for (const course of allCourses.value) {
    const key = course.category_id ?? UNCLASSIFIED
    counts.set(key, (counts.get(key) ?? 0) + 1)
  }
  return counts
})
const unclassifiedCount = computed(() => categoryCounts.value.get(UNCLASSIFIED) ?? 0)

function categoryNameOf(id?: number) {
  return categories.value.find(category => category.id === id)?.name ?? '—'
}

function selectCategory(id: number | null) {
  categoryId.value = id
  currentPage.value = 1
}

const filterStatus = ref<number | null>(null)
const filterHotFeatured = ref<string | null>(null)
const keyword = ref('')
const currentPage = ref(1)
const pageSize = ref(10)

// el-select clearable 清除后值为 undefined（valueOnClear 默认）/'' 而非 null，
// 须归一化判断，否则「先筛选再清除」会把全表过滤为空
function filterActive(v: unknown): boolean {
  return v !== null && v !== undefined && v !== ''
}

const filteredCourses = computed(() => {
  const k = keyword.value.trim().toLowerCase()
  const status = filterStatus.value
  return allCourses.value.filter(c => {
    if (categoryId.value !== null && (c.category_id ?? UNCLASSIFIED) !== categoryId.value) return false
    if (filterActive(status) && c.status !== status) return false
    if (filterHotFeatured.value === 'hot' && !c.is_hot) return false
    if (filterHotFeatured.value === 'featured' && !c.is_featured) return false
    if (k && !c.name.toLowerCase().includes(k)) return false
    return true
  })
})

const pagedCourses = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filteredCourses.value.slice(start, start + pageSize.value)
})

function certificateNameOf(id?: number | null) {
  if (!id) return ''
  return certificateTemplates.value.find(t => t.id === id)?.name || ''
}

const canSortCourses = computed(() => categoryId.value !== null && categoryId.value !== UNCLASSIFIED)

// ===== 子组件（抽屉/弹窗）入口 =====
const courseDrawerRef = ref<InstanceType<typeof CourseCatalogCourseDrawer> | null>(null)
const catalogDialogsRef = ref<InstanceType<typeof CourseCatalogDialogs> | null>(null)

function openDrawer(course?: AdminCourseItem | null) {
  courseDrawerRef.value?.open(course ?? null)
}

function openCertificateDialog() {
  catalogDialogsRef.value?.openCertificateDialog()
}

// ===== 排序（后端 swap 端点，同值默认也生效） =====
async function moveCourse(row: AdminCourseItem, delta: -1 | 1) {
  if (row.category_id == null) return
  const sibs = filteredCourses.value.filter(c => c.category_id === row.category_id)
  const idx = sibs.findIndex(c => c.course_id === row.course_id)
  const target = sibs[idx + delta]
  if (!target) return
  submitting.value = true
  try {
    await adminApi.swapCourse(row.course_id, target.course_id)
    ElMessage.success('排序已更新')
    await refreshCatalog()
  } catch (error) {
    console.error('排序失败:', error)
    /* 错误已由拦截器提示 */
  } finally {
    submitting.value = false
  }
}

// ===== 加载 =====
async function loadCertificateTemplates() {
  try {
    // trainingApi 已解包信封：成功直接返回业务负载
    const data = await trainingApi.getCertificateTemplates()
    certificateTemplates.value = data.certificate_templates || []
  } catch (error) {
    console.error('加载证书模板失败:', error)
  }
}

const courseOptions = computed(() =>
  allCourses.value.filter(c => c.category_id != null).map(c => ({
    course_id: c.course_id,
    name: categoryNameOf(c.category_id) + ' · ' + c.name
  }))
)

async function toggleHot(row: AdminCourseItem, val: boolean) {
  try {
    await adminApi.updateCourse(row.course_id, { is_hot: val })
    row.is_hot = val
    ElMessage.success(val ? '已设为热门' : '已取消热门')
  } catch (error) {
    console.error('切换热门失败:', error)
  }
}

async function toggleFeatured(row: AdminCourseItem, val: boolean) {
  try {
    await adminApi.updateCourse(row.course_id, { is_featured: val })
    row.is_featured = val
    ElMessage.success(val ? '已设为精品' : '已取消精品')
  } catch (error) {
    console.error('切换精品失败:', error)
  }
}

async function toggleStatus(row: AdminCourseItem) {
  if (row.status !== 1 && row.category_id == null) {
    ElMessage.warning('请先补全课程分类再上架')
    return
  }
  submitting.value = true
  try {
    await adminApi.updateCourse(row.course_id, { status: row.status === 1 ? 0 : 1 })
    ElMessage.success(row.status === 1 ? '已下架' : '已上架')
    await refreshCatalog()
  } catch (error) {
    console.error('切换状态失败:', error)
    /* 错误已由拦截器提示 */
  } finally {
    submitting.value = false
  }
}

function handleAction(cmd: string, row: AdminCourseItem) {
  switch (cmd) {
    case 'edit':
      openDrawer(row)
      break
    case 'toggle':
      toggleStatus(row)
      break
    case 'moveUp':
      moveCourse(row, -1)
      break
    case 'moveDown':
      moveCourse(row, 1)
      break
    case 'delete':
      useConfirm().confirmDanger('确定删除该课程？', '提示', {
        type: 'warning',
        confirmButtonText: '确定',
        cancelButtonText: '取消'
      }).then(() => handleDeleteCourse(row)).catch(() => {})
      break
  }
}

async function handleDeleteCourse(row: AdminCourseItem) {
  try {
    await adminApi.deleteCourse(row.course_id)
    ElMessage.success('已删除')
    await refreshCatalog()
  } catch (error) {
    console.error('删除失败:', error)
    /* 错误已由拦截器提示 */
  }
}

onMounted(() => {
  refreshCatalog()
  loadCertificateTemplates()
})

/** 行内「操作」菜单：编辑 / 上下架 / 排序 / 删除走菜单收纳（登记表「菜单收纳」行） */
function actionItems(row: AdminCourseItem): UiMoreMenuItem[] {
  const items: UiMoreMenuItem[] = [
    { key: 'edit', label: '编辑' },
    { key: 'toggle', label: row.status === 1 ? '下架' : '上架' }
  ]
  // 排序项只在有排序能力时出现（调用方判可见性，组件不认识语义）
  if (canSortCourses.value) {
    items.push({ key: 'moveUp', label: '上移' }, { key: 'moveDown', label: '下移' })
  }
  items.push({ key: 'delete', label: '删除', divided: true })
  return items
}
</script>

<style scoped>
.cc-layout {
  display: flex;
  gap: var(--space-5);
  align-items: flex-start;
}

/* ===== 左侧导航：三卡片 ===== */
.cc-sidebar {
  width: 210px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.cc-card {
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-lg);
  padding: var(--space-3) var(--space-2);
}

.cc-actions-card {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: var(--space-2);
}

.cc-actions-card .el-button {
  width: 100%;
  margin-left: 0;
}

.cc-nav-row {
  display: flex;
  align-items: center;
}

.cc-nav-row .cc-nav-item,
.cc-level-row .cc-nav-item {
  flex: 1;
  min-width: 0;
}

.cc-nav-move {
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  line-height: 1.1;
}

.cc-move-icon {
  cursor: pointer;
  font-size: var(--text-sm);
  color: var(--color-text-tertiary);
  transition: color var(--duration-fast) var(--ease-default);
}

.cc-move-icon:hover {
  color: var(--color-primary-soft-ink);
}

.cc-level-row {
  display: flex;
  align-items: center;
}

/* ===== 右侧表格 ===== */
.cc-main {
  flex: 1;
  min-width: 0;
}


.cc-search {
  max-width: 280px;
}

.cc-cell-name {
  margin-left: 4px;
}

.cc-cell-dim {
  color: var(--color-text-secondary);
  font-size: var(--text-sm);
  margin-left: 6px;
}

.cc-cell-warn {
  color: var(--color-danger);
  font-size: var(--text-sm);
}

.cc-pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: var(--space-4);
}

@media (max-width: 900px) {
  .cc-layout {
    flex-direction: column;
    /* 纵向后子元素需横向拉伸，否则 .cc-main 宽度=内容宽度（宽表格）导致整页横向溢出 */
    align-items: stretch;
  }

  .cc-sidebar {
    width: 100%;
  }
}

@media screen and (max-width: 768px) {

  .cc-search {
    flex: 1 1 100%;
    max-width: none;
  }
}
</style>
