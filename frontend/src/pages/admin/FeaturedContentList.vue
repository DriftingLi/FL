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
      <el-button type="primary" @click="loadList">查询</el-button>
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
        @size-change="loadList"
        @current-change="loadList"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Plus, ArrowDown } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { adminFeaturedApi, featuredCategoryOptions, categoryLabel, type FeaturedContent } from '@/api/featured'
import { formatDateTime } from '@/utils/format'

const router = useRouter()

const loading = ref(false)
const list = ref<FeaturedContent[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(10)

const filterCategory = ref('')
const filterStatus = ref<number | undefined>(undefined)

async function loadList() {
  loading.value = true
  try {
    const params: { page?: number; page_size?: number; category?: string; status?: string } = {
      page: currentPage.value,
      page_size: pageSize.value
    }
    if (filterCategory.value) params.category = filterCategory.value
    if (filterStatus.value !== undefined && filterStatus.value !== null) {
      params.status = String(filterStatus.value)
    }
    const res = await adminFeaturedApi.getList(params)
    list.value = res.items || []
    total.value = res.total || 0
  } catch (e: any) {
    // 错误已由全局拦截器提示
  } finally {
    loading.value = false
  }
}

function handleFilterChange() {
  currentPage.value = 1
  loadList()
}

function resetFilter() {
  filterCategory.value = ''
  filterStatus.value = undefined
  currentPage.value = 1
  loadList()
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
    loadList()
  } catch (e: any) {
    // 错误已由全局拦截器提示
  }
}

async function handleDelete(id: number) {
  try {
    await adminFeaturedApi.remove(id)
    ElMessage.success('删除成功')
    loadList()
  } catch (e: any) {
    // 错误已由全局拦截器提示
  }
}

// 操作下拉菜单统一入口
async function handleAction(cmd: string, row: any) {
  switch (cmd) {
    case 'edit':
      goEdit(row.content_id)
      break
    case 'publish':
      handlePublish(row.content_id)
      break
    case 'delete':
      try {
        await ElMessageBox.confirm('确定删除该内容？删除后不可恢复', '提示', {
          type: 'warning',
          confirmButtonText: '确定',
          cancelButtonText: '取消'
        })
        await handleDelete(row.content_id)
      } catch {
        // 用户取消
      }
      break
  }
}

onMounted(() => {
  loadList()
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
  color: #1e293b;
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
  color: #cbd5e1;
}

:deep(.el-table) {
  border-radius: 8px;
  overflow: hidden;
}
</style>
