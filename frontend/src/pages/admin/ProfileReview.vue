<template>
  <div class="profile-review-page">
    <el-card>
      <template #header>
        <div class="card-header">
          <span class="card-title">资料审核（昵称 / 头像）</span>
          <UiButton :icon="Refresh" circle @click="load"/>
        </div>
      </template>

      <el-tabs v-model="activeStatus" @tab-change="handleTabChange">
        <el-tab-pane label="待审核" name="pending" />
        <el-tab-pane label="已通过" name="approved" />
        <el-tab-pane label="已拒绝" name="rejected" />
      </el-tabs>

      <el-table v-loading="loading" :data="list" stripe style="width: 100%">
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column label="用户" min-width="160">
          <template #default="{ row }">
            <div class="user-cell">
              <el-avatar :size="32" :src="row.avatar_url || undefined">
                {{ displayName(row).charAt(0).toUpperCase() }}
              </el-avatar>
              <div class="user-meta">
                <span class="user-name">{{ displayName(row) }}</span>
                <span class="user-username">@{{ row.username }}</span>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="修改项" width="100">
          <template #default="{ row }">
            <el-tag :type="row.field_type === 'nickname' ? 'primary' : 'warning'" size="small">
              {{ row.field_type === 'nickname' ? '昵称' : '头像' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="原值 → 新值" min-width="220">
          <template #default="{ row }">
            <div v-if="row.field_type === 'nickname'" class="value-cell">
              <span class="old-value">{{ row.old_value || '（空）' }}</span>
              <el-icon class="arrow"><ArrowRight /></el-icon>
              <span class="new-value">{{ row.new_value }}</span>
            </div>
            <div v-else class="avatar-value-cell">
              <el-avatar :size="36" :src="row.old_value || undefined">旧</el-avatar>
              <el-icon class="arrow"><ArrowRight /></el-icon>
              <el-avatar :size="36" :src="row.new_value || undefined">新</el-avatar>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="提交时间" width="170">
          <template #default="{ row }">{{ formatLocaleDateTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column v-if="activeStatus === 'rejected'" label="驳回原因" min-width="140">
          <template #default="{ row }">{{ row.reject_reason || '-' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="90" fixed="right" align="center">
          <template #default="{ row }">
            <template v-if="row.status === 'pending'">
              <el-dropdown trigger="click" @command="(cmd: string) => handleAction(cmd, row)">
                <UiButton variant="primary" link size="small">
                  操作<el-icon class="el-icon--right"><ArrowDown /></el-icon>
                </UiButton>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="approve">通过</el-dropdown-item>
                    <el-dropdown-item command="reject">驳回</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </template>
            <el-tag v-else :type="row.status === 'approved' ? 'success' : 'danger'" size="small">
              {{ row.status === 'approved' ? '已通过' : '已拒绝' }}
            </el-tag>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-wrapper" v-if="total > pageSize">
        <el-pagination
          v-model:current-page="currentPage"
          :page-size="pageSize"
          :total="total"
          layout="total, prev, pager, next"
          @current-change="load"
        />
      </div>
    </el-card>

    <el-dialog v-model="rejectDialogVisible" title="驳回修改" width="480px">
      <el-input
        v-model="rejectReason"
        type="textarea"
        :rows="3"
        maxlength="200"
        show-word-limit
        placeholder="请输入驳回原因（选填）"
      />
      <template #footer>
        <UiButton @click="rejectDialogVisible = false">取消</UiButton>
        <UiButton variant="danger" :loading="submitting" @click="reject">确认驳回</UiButton>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, ArrowRight, ArrowDown } from '@element-plus/icons-vue'
import { adminApi, type ProfileChangeRequest } from '@/api/admin'
import { useAdminTable } from '@/composables/useAdminTable'
import { formatLocaleDateTime } from '@/utils/format'
import UiButton from '@/components/ui/UiButton.vue'

const submitting = ref(false)
const activeStatus = ref<'pending' | 'approved' | 'rejected'>('pending')
const rejectDialogVisible = ref(false)
const rejectReason = ref('')
const currentRow = ref<ProfileChangeRequest | null>(null)

function displayName(row: ProfileChangeRequest) {
  return row.username
}

// admin 列表状态机：本页只声明 fetch 与行操作 adapter
const table = useAdminTable<ProfileChangeRequest>({
  fetch: async (paging, filters) => {
    const data = await adminApi.listProfileReviews({
      status: String(filters.status || 'pending'),
      page: paging.page,
      page_size: paging.pageSize
    })
    return { list: data?.requests || [], total: data?.total || 0 }
  },
  actions: {
    approve,
    reject: openReject
  }
})
const { loading, list, total, currentPage, pageSize, load, handleAction } = table

function handleTabChange() {
  table.applyFilters({ status: activeStatus.value })
}

async function approve(row: ProfileChangeRequest) {
  try {
    await ElMessageBox.confirm('确认通过该修改？通过后立即生效。', '通过审核', { type: 'info' })
  } catch {
    return
  }
  try {
    await adminApi.approveProfileReview(row.id)
    ElMessage.success('已通过审核')
    load()
  } catch (e) {
    console.error('审核失败:', e)
    /* 错误已由拦截器提示 */
  }
}

function openReject(row: ProfileChangeRequest) {
  currentRow.value = row
  rejectReason.value = ''
  rejectDialogVisible.value = true
}

async function reject() {
  if (!currentRow.value) return
  submitting.value = true
  try {
    await adminApi.rejectProfileReview(currentRow.value.id, rejectReason.value.trim())
    ElMessage.success('已驳回')
    rejectDialogVisible.value = false
    load()
  } catch (e) {
    console.error('驳回失败:', e)
    /* 错误已由拦截器提示 */
  } finally {
    submitting.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-title {
  font-size: 16px;
  font-weight: 600;
}

.user-cell {
  display: flex;
  align-items: center;
  gap: 10px;
}

.user-meta {
  display: flex;
  flex-direction: column;
  line-height: 1.3;
}

.user-name {
  font-size: 14px;
  color: var(--color-text-primary);
}

.user-username {
  font-size: 12px;
  color: var(--color-text-tertiary);
}

.value-cell,
.avatar-value-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.old-value {
  color: var(--color-text-tertiary);
  text-decoration: line-through;
}

.new-value {
  color: var(--color-text-primary);
  font-weight: 500;
}

.arrow {
  color: var(--color-text-disabled);
}

.pagination-wrapper {
  display: flex;
  justify-content: center;
  margin-top: 16px;
}
</style>
