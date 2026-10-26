<template>
  <div class="featured-list-page">
    <div class="page-header">
      <h2>内容精选管理</h2>
      <UiButton variant="primary" @click="goCreate">
        <UiIcon name="plus" /> 新建内容
      </UiButton>
    </div>

    <UiFilterBar>
        <template #filters>

      <el-select
        v-model="filterCategory"
        placeholder="全部分类"
        clearable
        style="width: 160px"
        @change="handleFilterChange"
      >
        <el-option
          v-for="opt in featuredCategoryOptions"
          :key="opt.value"
          :label="opt.label"
          :value="opt.value"
        />
      </el-select>
      <!-- 三态过滤（#1748）：「定时」不是一个 status 取值，故参数由 status(0/1) 改为 state -->
      <el-select
        v-model="filterState"
        placeholder="全部状态"
        clearable
        style="width: 140px"
        @change="handleFilterChange"
      >
        <el-option v-for="opt in stateOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
      </el-select>
      <UiButton variant="primary" @click="handleFilterChange">查询</UiButton>
      <UiButton @click="resetFilter">重置</UiButton>
        </template>
      </UiFilterBar>

    <el-table :data="list" v-loading="loading" stripe border style="width: 100%">
      <el-table-column prop="title" label="标题" min-width="240" show-overflow-tooltip />
      <el-table-column label="分类" width="120" align="center">
        <template #default="{ row }">
          <UiTag size="small">{{ categoryLabel(row.category) }}</UiTag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100" align="center">
        <template #default="{ row }">
          <UiTag :tone="rowMeta(row).tone" size="small">{{ rowMeta(row).label }}</UiTag>
        </template>
      </el-table-column>
      <el-table-column label="发布时间" width="180" align="center">
        <template #default="{ row }">
          <span v-if="rowTime(row)">
            <span v-if="rowState(row) === 'scheduled'" class="scheduled-hint">定时 </span>
            {{ formatDateTime(rowTime(row)) }}
          </span>
          <span v-else class="empty-text">—</span>
        </template>
      </el-table-column>
      <el-table-column prop="view_count" label="阅读量" width="100" align="center" />
      <el-table-column prop="sort_order" label="排序" width="80" align="center" />
      <el-table-column label="操作" width="90" fixed="right" align="center">
        <template #default="{ row }">
          <UiMoreMenu trigger="text" :items="actionItems(row)" @select="(k: string) => handleAction(k, row)" />
        </template>
      </el-table-column>
    </el-table>

      <UiPagination v-if="total > pageSize"
      v-model:current-page="currentPage"
      v-model:page-size="pageSize"
      :total="total"
      show-sizes
      :page-sizes="[10, 20, 50]"
      @current-change="load"
      @size-change="load"
    align="right" class="mt-4" />
    
  </div>
</template>

<script setup lang="ts">
import UiIcon from '@/components/ui/UiIcon.vue'
// keep-alive 的 include 按**组件名**匹配：名必须与路由名（页面描述符的 name）一致，
// 否则该页不会被缓存（#1620 的锁测试会拦下这种漂移）。
defineOptions({ name: 'AdminFeaturedContentList' })

import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { adminFeaturedApi, featuredCategoryOptions, categoryLabel, type FeaturedContentAdminListDTO } from '@/api/featured'
import { useConfirm } from '@/composables/useConfirm'
// #1748：三态徽标与时间列的判据是纯函数（单测见 __tests__/FeaturedSchedule.spec.ts）
import { FEATURED_STATE_META, featuredRowState, featuredRowTime } from './featuredSchedule'
import { useAdminTable } from '@/composables/useAdminTable'
import { formatDateTime } from '@/utils/format'
import UiButton from '@/components/ui/UiButton.vue'
import UiPagination from '@/components/ui/UiPagination.vue'
import UiFilterBar from '@/components/ui/UiFilterBar.vue'
import UiTag from '@/components/ui/UiTag.vue'
import UiMoreMenu, { type UiMoreMenuItem } from '@/components/ui/UiMoreMenu.vue'

const router = useRouter()

const filterCategory = ref('')
const filterState = ref<string>('')

// 模板里直接用纯函数（三态徽标 / 时间列）——判据只有一份，页面不重写规则
const rowState = featuredRowState
const rowTime = featuredRowTime
const rowMeta = (row: { status: number; scheduled_at?: string | null }) => FEATURED_STATE_META[featuredRowState(row)]
// 筛选选项也由 descriptor 派生：状态文案在全站只此一份（ui-conventions「状态词表」）
const stateOptions = Object.entries(FEATURED_STATE_META).map(([value, meta]) => ({ value, label: meta.label }))

// admin 列表状态机：本页只声明 fetch 与行操作 adapter
const table = useAdminTable<FeaturedContentAdminListDTO>({
  fetch: (paging, filters) => {
    const params: { page?: number; page_size?: number; category?: string; state?: string } = {
      page: paging.page,
      page_size: paging.pageSize
    }
    if (filters.category) params.category = String(filters.category)
    if (filters.state) params.state = String(filters.state)
    return adminFeaturedApi.getList(params)
  },
  actions: {
    edit: (row: FeaturedContentAdminListDTO) => goEdit(row.content_id),
    publish: (row: FeaturedContentAdminListDTO) => handlePublish(row.content_id),
    cancelSchedule: (row: FeaturedContentAdminListDTO) => cancelSchedule(row.content_id),
    delete: deleteRow
  }
})
const { loading, list, total, currentPage, pageSize, load, handleAction } = table

function deleteRow(row: FeaturedContentAdminListDTO): Promise<void> {
  return table.confirmDelete(row, r => handleDelete(r.content_id), '确定删除该内容？删除后不可恢复')
}

function handleFilterChange() {
  table.applyFilters({ category: filterCategory.value, state: filterState.value })
}

function resetFilter() {
  filterCategory.value = ''
  filterState.value = ''
  table.applyFilters({ category: '', state: '' })
}

function goCreate() {
  router.push({ name: 'AdminFeaturedContentEdit' })
}

function goEdit(id: number) {
  router.push({ name: 'AdminFeaturedContentEdit', params: { id } })
}

async function handlePublish(id: number) {
  try {
    // 不带 body = 立即发布（定时项也可提前上线，不必先改时间）
    await adminFeaturedApi.publish(id)
    ElMessage.success('已发布')
    load()
  } catch (e: any) {
    // 错误已由全局拦截器提示
  }
}

/** 转为草稿 = 取消排期（不带时间，后端语义就是「清空 scheduled_at」） */
async function cancelSchedule(id: number) {
  try {
    await useConfirm().confirmDanger('取消这条定时？内容会退回草稿，不再自动发布。', '取消定时', {
      confirmButtonText: '转为草稿'
    })
  } catch {
    return
  }
  try {
    await adminFeaturedApi.update(id, { status: 0 })
    ElMessage.success('已转为草稿')
    load()
  } catch (e: any) {
    // 错误已由全局拦截器提示
  }
}

async function handleDelete(id: number) {
  try {
    await adminFeaturedApi.remove(id)
    ElMessage.success('删除成功')
    load()
  } catch (e: any) {
    // 错误已由全局拦截器提示
  }
}

onMounted(() => {
  load()
})

/** 行内「操作」菜单：编辑 / 发布态动作 / 删除走菜单收纳（登记表「菜单收纳」行） */
function actionItems(row: FeaturedContentAdminListDTO): UiMoreMenuItem[] {
  const items: UiMoreMenuItem[] = [{ key: 'edit', label: '编辑' }]
  // 立即发布：草稿与定时中都能用（定时项提前上线，不必改时间；#1748 用户故事 17）
  if (rowState(row) !== 'published') items.push({ key: 'publish', label: '立即发布' })
  // 转为草稿：只对定时行出现 —— 它是「取消排期」这类内容的唯一出口
  if (rowState(row) === 'scheduled') items.push({ key: 'cancelSchedule', label: '转为草稿' })
  items.push({ key: 'delete', label: '删除', divided: true })
  return items
}
</script>

<style scoped>
.featured-list-page {
  padding: 20px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.page-header h2 {
  margin: 0;
  font-size: var(--text-2xl);
  font-weight: 600;
  color: var(--color-text-primary);
}



.empty-text {
  color: var(--color-text-disabled);
}

.scheduled-hint {
  color: var(--el-color-warning, var(--color-text-tertiary));
}

:deep(.el-table) {
  border-radius: 8px;
  overflow: hidden;
}
</style>
