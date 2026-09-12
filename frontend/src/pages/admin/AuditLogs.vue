<template>
  <div class="audit-page">
    <div class="page-header">
      <h2>审计日志</h2>
    </div>

    <el-card>
      <UiFilterBar>
        <template #filters>

        <el-select v-model="query.role" placeholder="角色" clearable style="width: 130px" @change="search()">
          <el-option label="管理员" value="admin" />
          <el-option label="讲师" value="tutor" />
        </el-select>
        <el-input
          v-model="query.keyword"
          placeholder="搜索操作内容或操作人"
          clearable
          style="width: 220px"
          @keyup.enter="search()"
        />
        <UiButton variant="primary" @click="search()">查询</UiButton>
        </template>
      </UiFilterBar>

      <UiErrorState
        v-if="loadError"
        title="日志加载失败"
        description="网络或服务端异常，可重试"
        :retrying="retrying"
        @retry="retryLoad"
      />
      <el-table v-else :data="items" stripe border style="width: 100%" v-loading="loading">
        <el-table-column type="expand">
          <template #default="{ row }">
            <pre class="audit-detail">{{ JSON.stringify(row.detail || {}, null, 2) }}</pre>
          </template>
        </el-table-column>
        <el-table-column label="时间" width="140">
          <template #default="{ row }">
            {{ formatTime(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column prop="actor_name" label="操作人" width="110" />
        <el-table-column label="角色" width="80" align="center">
          <template #default="{ row }">
            {{ row.actor_role === 'admin' ? '管理员' : '讲师' }}
          </template>
        </el-table-column>
        <el-table-column prop="action" label="操作内容" min-width="200" show-overflow-tooltip />
        <el-table-column label="结果" width="80" align="center">
          <template #default="{ row }">
            {{ row.status < 400 ? '成功' : '失败' }}
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination">
        <UiPagination
      v-model:current-page="page"
      :page-size="pageSize"
      :total="total"
      @current-change="load"
    />
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive } from 'vue'
import { adminApi, type AuditLogItem } from '@/api/admin'
import { formatTime } from '@/utils/format'
import UiButton from '@/components/ui/UiButton.vue'
import UiPagination from '@/components/ui/UiPagination.vue'
import UiFilterBar from '@/components/ui/UiFilterBar.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'
import { useAdminTable } from '@/composables/useAdminTable'

const query = reactive<{ role: string; keyword: string }>({
  role: '',
  keyword: ''
})

// 列表：admin 列表状态机 useAdminTable（#793，ADR-0039）——三态 + 分页 + 列表托管。
// 解构改名保持模板零改动；query 为页面自管筛选轴（由 fetch adapter 读取）。
const {
  loading,
  loadError,
  retrying,
  list: items,
  total,
  currentPage: page,
  pageSize,
  load,
  retry: retryLoad
} = useAdminTable<AuditLogItem>({
  pageSize: 20,
  fetch: async (paging) => {
    const data = await adminApi.listAuditLogs({
      page: paging.page,
      page_size: paging.pageSize,
      role: query.role || undefined,
      keyword: query.keyword || undefined
    })
    return { list: data?.items || [], total: data?.total || 0 }
  }
})

/** 查询 / 筛选变化：回第一页重装 */
function search(): void {
  page.value = 1
  void load()
}

onMounted(load)
</script>

<style scoped>
.audit-page {
  padding: 20px;
}

.page-header {
  margin-bottom: 20px;
}

.page-header h2 {
  font-size: 22px;
  color: var(--color-text-primary);
}


.audit-detail {
  margin: 0;
  padding: 12px;
  background: var(--color-bg-page);
  border-radius: 6px;
  font-size: 12px;
  color: var(--color-text-secondary);
}

.pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
</style>
