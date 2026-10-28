<template>
  <div class="feedback-page">
    <div class="page-header">
      <h2>学员反馈</h2>
    </div>

    <UiCard>
      <UiFilterBar>
        <template #filters>
          <UiSelect
            v-model="query.status"
            placeholder="状态"
            clearable
            style="width: 140px"
            :options="statusOptions"
            @change="search()"
          />
        </template>
      </UiFilterBar>

      <UiAsyncSection
        :error="loadError"
        :loading="loading"
        :empty="isEmpty"
        :retrying="retrying"
        error-title="反馈加载失败"
        error-description="网络或服务端异常，可重试"
        @retry="retryLoad"
      >
        <template #skeleton>
          <UiSkeleton variant="list" :count="4" />
        </template>

        <el-table :data="items" stripe border style="width: 100%">
          <el-table-column label="时间" width="150">
            <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
          </el-table-column>
          <el-table-column label="提交人" width="120">
            <template #default="{ row }">{{ row.username || `学员 #${row.user_id}` }}</template>
          </el-table-column>
          <el-table-column label="反馈内容" min-width="260">
            <template #default="{ row }">
              <div class="feedback-content">{{ row.content }}</div>
              <div v-if="row.contact" class="feedback-contact">联系方式：{{ row.contact }}</div>
              <div v-if="row.images && row.images.length" class="feedback-images">
                <el-image
                  v-for="(url, i) in row.images"
                  :key="url"
                  :src="url"
                  :preview-src-list="row.images"
                  :initial-index="i"
                  fit="cover"
                  class="feedback-thumb"
                  preview-teleported
                />
              </div>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="100" align="center">
            <template #default="{ row }">
              <UiTag :tone="describeFeedbackStatus(row.status).tone" size="small">
                {{ describeFeedbackStatus(row.status).label }}
              </UiTag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="90" fixed="right">
            <template #default="{ row }">
              <UiButton
                v-if="row.status !== 'handled'"
                link
                :loading="handlingId === row.id"
                @click="markHandled(row)"
              >标记已处理</UiButton>
              <span v-else class="feedback-done">—</span>
            </template>
          </el-table-column>
        </el-table>

        <template #empty>
          <UiEmptyState description="暂无反馈" />
        </template>
      </UiAsyncSection>

      <div class="pagination">
        <UiPagination
          v-model:current-page="page"
          :page-size="pageSize"
          :total="total"
          @current-change="load"
        />
      </div>
    </UiCard>
  </div>
</template>

<script setup lang="ts">
// keep-alive 的 include 按**组件名**匹配：名必须与路由名（页面描述符的 name）一致。
defineOptions({ name: 'AdminFeedbackManage' })

import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { feedbackApi, type FeedbackDTO } from '@/api/feedback'
import { describeFeedbackStatus } from '@/utils/feedbackStatus'
import { formatTime } from '@/utils/format'
import { useAdminTable } from '@/composables/useAdminTable'
import UiAsyncSection from '@/components/ui/UiAsyncSection.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiFilterBar from '@/components/ui/UiFilterBar.vue'
import UiPagination from '@/components/ui/UiPagination.vue'
import UiSelect, { type UiSelectOption } from '@/components/ui/UiSelect.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiTag from '@/components/ui/UiTag.vue'

const statusOptions: UiSelectOption[] = [
  { label: '待处理', value: 'pending' },
  { label: '已处理', value: 'handled' }
]

/** 状态筛选是页面自管的筛选轴（由 fetch adapter 读取，不塞进 composable）。 */
const query = reactive<{ status: string }>({ status: '' })
/** 正在标记的那一行（行内 loading，避免整表转圈）。 */
const handlingId = ref<number | null>(null)

const {
  loading,
  loadError,
  retrying,
  isEmpty,
  list: items,
  total,
  currentPage: page,
  pageSize,
  load,
  retry: retryLoad
} = useAdminTable<FeedbackDTO>({
  pageSize: 20,
  fetch: (paging) =>
    // 分页容器由 api 层出口给（ADR-0060 决策 6），页面不手抄 { items, total }
    feedbackApi.list({
      page: paging.page,
      page_size: paging.pageSize,
      status: query.status || undefined
    })
})

/** 筛选变化：回第一页重装 */
function search(): void {
  page.value = 1
  void load()
}

/** 标记已处理：后端幂等，成功后重载当前页（状态筛选会随之变化） */
async function markHandled(row: FeedbackDTO): Promise<void> {
  handlingId.value = row.id
  try {
    await feedbackApi.markHandled(row.id)
    ElMessage.success('已标记为已处理')
    await load()
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e)
    ElMessage.error(msg || '操作失败')
  } finally {
    handlingId.value = null
  }
}

onMounted(load)
</script>

<style scoped>
.feedback-page {
  padding: 20px;
}

.page-header {
  margin-bottom: 20px;
}

.page-header h2 {
  font-size: var(--text-2xl);
  color: var(--color-text-primary);
}

.feedback-content {
  white-space: pre-wrap;
  word-break: break-word;
}

.feedback-contact {
  margin-top: 4px;
  font-size: var(--text-xs);
  color: var(--color-text-secondary);
}

.feedback-images {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 6px;
}

.feedback-thumb {
  width: 56px;
  height: 56px;
  border-radius: var(--radius-md);
}

.feedback-done {
  color: var(--color-text-tertiary);
}

.pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
</style>
