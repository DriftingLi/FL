<template>
  <div class="audit-page">
    <div class="page-header">
      <h2>审计日志</h2>
    </div>

    <el-card>
      <div class="filter-bar">
        <el-select v-model="query.role" placeholder="角色" clearable style="width: 130px" @change="load(1)">
          <el-option label="管理员" value="admin" />
          <el-option label="讲师" value="tutor" />
        </el-select>
        <el-input
          v-model="query.keyword"
          placeholder="搜索操作内容或操作人"
          clearable
          style="width: 220px"
          @keyup.enter="load(1)"
        />
        <el-button type="primary" @click="load(1)">查询</el-button>
      </div>

      <el-table :data="items" stripe border style="width: 100%">
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
        <el-pagination
          background
          layout="total, prev, pager, next"
          :total="total"
          :page-size="pageSize"
          :current-page="page"
          @current-change="load"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { adminApi, type AuditLogItem } from '@/api/admin'
import { formatTime } from '@/utils/format'

const items = ref<AuditLogItem[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const query = reactive<{ role: string; keyword: string }>({
  role: '',
  keyword: ''
})

async function load(p: number) {
  page.value = p
  try {
    const data = await adminApi.listAuditLogs({
      page: p,
      page_size: pageSize,
      role: query.role || undefined,
      keyword: query.keyword || undefined
    })
    if (data) {
      items.value = data.items || []
      total.value = data.total || 0
    }
  } catch (e) {
    // 拦截器已提示
  }
}

onMounted(() => {
  load(1)
})
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

.filter-bar {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
  flex-wrap: wrap;
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
