<template>
  <div class="featured-list-page">
    <div class="page-header">
      <h2>内容精选管理</h2>
      <el-button type="primary" @click="goCreate">
        <el-icon><Plus /></el-icon> 新建内容
      </el-button>
    </div>

    <div class="filter-bar">
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
      <el-select
        v-model="filterStatus"
        placeholder="全部状态"
        clearable
        style="width: 140px"
        @change="handleFilterChange"
      >
        <el-option label="草稿" :value="0" />
        <el-option label="已发布" :value="1" />
      </el-select>
      <el-button type="primary" @click="handleFilterChange">查询</el-button>
      <el-button @click="resetFilter">重置</el-button>
    </div>

    <el-table :data="list" v-loading="loading" stripe border style="width: 100%">
      <el-table-column prop="title" label="标题" min-width="240" show-overflow-tooltip />
      <el-table-column label="分类" width="120" align="center">
        <template #default="{ row }">
          <el-tag size="small">{{ categoryLabel(row.category) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100" align="center">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'" size="small">
            {{ row.status === 1 ? '已发布' : '草稿' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="发布时间" width="180" align="center">
        <template #default="{ row }">
          <span v-if="row.published_at">{{ formatDateTime(row.published_at) }}</span>
          <span v-else class="empty-text">—</span>
        </template>
      </el-table-column>
      <el-table-column prop="view_count" label="阅读量" width="100" align="center" />
      <el-table-column prop="sort_order" label="排序" width="80" align="center" />
      <el-table-column label="操作" width="90" fixed="right" align="center">
        <template #default="{ row }">
          <el-dropdown trigger="click" @command="(cmd: string) => handleAction(cmd, row)">
            <el-button type="primary" link size="small">
              操作<el-icon class="el-icon--right"><ArrowDown /></el-icon>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="edit">编辑</el-dropdown-item>
                <el-dropdown-item v-if="row.status === 0" command="publish">发布</el-dropdown-item>
                <el-dropdown-item command="delete" divided>删除</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
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
        @size-change="load"
        @current-change="load"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Plus, ArrowDown } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { adminFeaturedApi, featuredCategoryOptions, categoryLabel, type FeaturedContent } from '@/api/featured'
import { useAdminTable } from '@/composables/useAdminTable'
import { formatDateTime } from '@/utils/format'

const router = useRouter()

const filterCategory = ref('')
const filterStatus = ref<number | undefined>(undefined)

// admin 列表状态机：本页只声明 fetch 与行操作 adapter
const table = useAdminTable<FeaturedContent>({
  fetch: async (paging, filters) => {
    const params: { page?: number; page_size?: number; category?: string; status?: string } = {
      page: paging.page,
      page_size: paging.pageSize
    }
    if (filters.category) params.category = String(filters.category)
    if (filters.status !== undefined && filters.status !== null && filters.status !== '') {
      params.status = String(filters.status)
    }
    const res = await adminFeaturedApi.getList(params)
    return { list: res.items || [], total: res.total || 0 }
  },
  actions: {
    edit: (row: FeaturedContent) => goEdit(row.content_id),
    publish: (row: FeaturedContent) => handlePublish(row.content_id),
    delete: deleteRow
  }
})
const { loading, list, total, currentPage, pageSize, load, handleAction } = table

function deleteRow(row: FeaturedContent): Promise<void> {
  return table.confirmDelete(row, r => handleDelete(r.content_id), '确定删除该内容？删除后不可恢复')
}

function handleFilterChange() {
  table.applyFilters({ category: filterCategory.value, status: filterStatus.value ?? '' })
}

function resetFilter() {
  filterCategory.value = ''
  filterStatus.value = undefined
  table.applyFilters({ category: '', status: '' })
}

function goCreate() {
  router.push({ name: 'AdminFeaturedContentEdit' })
}

function goEdit(id: number) {
  router.push({ name: 'AdminFeaturedContentEdit', params: { id } })
}

async function handlePublish(id: number) {
  try {
    await adminFeaturedApi.publish(id)
    ElMessage.success('发布成功')
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
  font-size: 22px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.filter-bar {
  display: flex;
  gap: 12px;
  margin-bottom: 20px;
  flex-wrap: wrap;
}

.pagination-wrapper {
  margin-top: 20px;
  display: flex;
  justify-content: flex-end;
}

.empty-text {
  color: var(--color-text-disabled);
}

:deep(.el-table) {
  border-radius: 8px;
  overflow: hidden;
}
</style>
