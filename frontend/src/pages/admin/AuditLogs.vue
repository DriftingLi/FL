<template>
  <div class="audit-page">
    <div class="page-header">
      <h2>审计日志</h2>
    </div>

    <el-card>
      <div class="filter-bar">
        <el-input
          v-model="query.actor_id"
          placeholder="操作人ID"
          clearable
          style="width: 140px"
          @keyup.enter="load(1)"
        />
        <el-select v-model="query.role" placeholder="角色" clearable style="width: 130px" @change="load(1)">
          <el-option label="管理员" value="admin" />
          <el-option label="讲师" value="tutor" />
        </el-select>
        <el-input
          v-model="query.keyword"
          placeholder="路径 / 动作 / 操作人"
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
        <el-table-column prop="created_at" label="时间" width="170" />
        <el-table-column prop="actor_name" label="操作人" width="110" />
        <el-table-column label="角色" width="80" align="center">
          <template #default="{ row }">
            {{ row.actor_role === 'admin' ? '管理员' : '讲师' }}
          </template>
        </el-table-column>
        <el-table-column prop="action" label="动作" min-width="220" show-overflow-tooltip />
        <el-table-column prop="method" label="方法" width="80" align="center" />
        <el-table-column prop="path" label="路径" min-width="220" show-overflow-tooltip />
        <el-table-column label="状态" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status < 400 ? 'success' : 'danger'" size="small">
              {{ row.status }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="ip" label="IP" width="130" />
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

const items = ref<AuditLogItem[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const query = reactive<{ actor_id: string; role: string; keyword: string }>({
  actor_id: '',
  role: '',
  keyword: ''
})

async function load(p: number) {
  page.value = p
  try {
    const res = await adminApi.listAuditLogs({
      page: p,
      page_size: pageSize,
      actor_id: query.actor_id ? Number(query.actor_id) : undefined,
      role: query.role || undefined,
      keyword: query.keyword || undefined
    })
    if (res.code === 200 && res.data) {
      items.value = res.data.items || []
      total.value = res.data.total || 0
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
  color: #303133;
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
  background: #f8fafc;
  border-radius: 6px;
  font-size: 12px;
  color: #475569;
}

.pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
</style>
