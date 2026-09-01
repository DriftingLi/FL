<template>
  <div class="forum-manage-page">
    <el-card>
      <template #header>
        <div class="card-header">
          <span class="card-title">论坛管理</span>
          <UiButton :icon="Refresh" circle @click="activeMainTab === 'reports' ? loadReports() : loadList()"/>
        </div>
      </template>

      <el-tabs v-model="activeMainTab" @tab-change="handleMainTabChange">
        <el-tab-pane label="帖子管理" name="topics" />
        <el-tab-pane label="举报管理" name="reports" />
      </el-tabs>

      <!-- ===== 举报管理（ADR-0018）===== -->
      <template v-if="activeMainTab === 'reports'">
        <div class="filter-bar">
          <el-radio-group v-model="reportStatus" @change="handleReportStatusChange">
            <el-radio-button :value="-1">全部</el-radio-button>
            <el-radio-button :value="0">待处理</el-radio-button>
            <el-radio-button :value="1">已处理</el-radio-button>
          </el-radio-group>
        </div>

        <el-table v-loading="reportLoading" :data="reports" border>
          <el-table-column prop="id" label="ID" width="60" align="center" />
          <el-table-column label="举报人" width="110">
            <template #default="{ row }">{{ row.reporter || '-' }}</template>
          </el-table-column>
          <el-table-column label="对象" min-width="200">
            <template #default="{ row }">
              <el-tag size="small" :type="row.reply_id ? 'info' : 'warning'">
                {{ row.reply_id ? '回复' : '帖子' }}
              </el-tag>
              <span class="report-target">{{ row.topic_title || `#${row.topic_id}` }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="reason" label="举报理由" min-width="180" show-overflow-tooltip />
          <el-table-column label="状态" width="90" align="center">
            <template #default="{ row }">
              <el-tag size="small" :type="row.status === 1 ? 'success' : 'danger'">
                {{ row.status === 1 ? '已处理' : '待处理' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="时间" width="160" align="center">
            <template #default="{ row }">{{ formatLocaleDateTime(row.created_at) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="100" fixed="right" align="center">
            <template #default="{ row }">
              <UiButton variant="primary" v-if="row.status === 0" size="small" link @click="handleReport(row)">
                标记已处理
              </UiButton>
              <span v-else class="report-done">—</span>
            </template>
          </el-table-column>
        </el-table>

        <div class="pagination-wrapper" v-if="reportTotal > reportPageSize">
          <el-pagination
            v-model:current-page="reportCurrentPage"
            :page-size="reportPageSize"
            :total="reportTotal"
            layout="total, prev, pager, next"
            @current-change="loadReports"
          />
        </div>
      </template>

      <!-- ===== 帖子管理（原有内容）===== -->
      <template v-else>
      <div class="filter-bar">
        <el-tabs v-model="activeTab" @tab-change="handleTabChange">
          <el-tab-pane label="全部帖子" name="all" />
          <el-tab-pane label="综合讨论区" name="discussion" />
          <el-tab-pane label="问答区" name="question" />
        </el-tabs>
        <el-input
          v-model="keyword"
          placeholder="搜索标题 / 内容"
          clearable
          style="width: 260px"
          @clear="handleSearch"
          @keyup.enter="handleSearch"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
      </div>

      <el-table
        v-loading="loading"
        :data="topics"
        stripe
        style="width: 100%"
        row-key="id"
        :expand-row-keys="expandedRows"
        @expand-change="handleExpand"
      >
        <el-table-column type="expand" width="40">
          <template #default="{ row }">
            <div v-loading="detailLoadingId === row.id" class="expand-replies">
              <template v-if="replyMap[row.id]">
                <div class="topic-content">
                  <div class="topic-content-text">{{ row.content }}</div>
                  <ForumImageGallery :images="row.images" />
                </div>
                <div v-if="replyMap[row.id].length > 0" class="reply-list">
                  <div v-for="reply in replyMap[row.id]" :key="reply.id" class="reply-item">
                    <div class="reply-meta">
                      <el-avatar :size="24" :src="reply.author.avatar_url || undefined">
                        {{ displayName(reply.author).charAt(0).toUpperCase() }}
                      </el-avatar>
                      <span class="reply-author">{{ displayName(reply.author) }}</span>
                      <span v-if="reply.parent_id && reply.parent_name" class="reply-quote">
                        回复 @{{ reply.parent_name }}
                      </span>
                      <span class="reply-time">{{ formatLocaleDateTime(reply.created_at) }}</span>
                      <UiButton variant="danger" class="reply-delete" size="small" @click="deleteReply(reply)">
                        删除回复
                      </UiButton>
                    </div>
                    <div class="reply-content">{{ reply.content }}</div>
                    <ForumImageGallery :images="reply.images" />
                  </div>
                </div>
                <el-empty v-else description="暂无回复" :image-size="60" />
              </template>
              <div v-else class="reply-loading">加载中…</div>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="id" label="ID" width="70" align="center" />
        <el-table-column label="标题" min-width="240">
          <template #default="{ row }">
            <div class="title-cell">
              <el-tag v-if="row.category === 'question'" size="small" type="success">问答</el-tag>
              <el-tag v-else-if="row.chapter_id" size="small" type="warning">
                {{ row.chapter_title || '章节讨论' }}
              </el-tag>
              <el-tag v-else size="small" type="info">综合</el-tag>
              <span class="title-text">{{ row.title }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="作者" min-width="120">
          <template #default="{ row }">{{ displayName(row.author) }}</template>
        </el-table-column>
        <el-table-column prop="reply_count" label="回复数" width="80" align="center" />
        <el-table-column prop="view_count" label="浏览" width="70" align="center" />
        <el-table-column label="创建时间" width="160" align="center">
          <template #default="{ row }">{{ formatLocaleDateTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="90" fixed="right" align="center">
          <template #default="{ row }">
            <UiButton variant="danger" size="small" @click="deleteTopic(row)">删除</UiButton>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-wrapper" v-if="total > pageSize">
        <el-pagination
          v-model:current-page="currentPage"
          :page-size="pageSize"
          :total="total"
          layout="total, prev, pager, next"
          @current-change="handlePageChange"
        />
      </div>
      </template>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Search } from '@element-plus/icons-vue'
import {
  adminForumApi,
  forumTabQuery,
  type ForumTab,
  type AdminForumTopic,
  type AdminForumReply,
  type AdminForumReportItem
} from '@/api/forum'
import ForumImageGallery from '@/components/student/ForumImageGallery.vue'
import { formatLocaleDateTime } from '@/utils/format'
import { useAsyncPage } from '@/composables/useAsyncPage'
import UiButton from '@/components/ui/UiButton.vue'

const topics = ref<AdminForumTopic[]>([])

// 帖子列表三态 + 分页收编 useAsyncPage（#439）
const {
  loading,
  total,
  page: currentPage,
  pageSize,
  run: loadList,
  handlePageChange
} = useAsyncPage(async () => {
  const res = await adminForumApi.listTopics({
    ...forumTabQuery(activeTab.value),
    page: currentPage.value,
    page_size: pageSize.value,
    keyword: keyword.value || undefined
  })
  topics.value = res.topics || []
  total.value = res.total || 0
})
// 管理端筛选轴：all=全部帖子、discussion=综合讨论区、question=问答区。
// 三个值都交给 forumTabQuery 翻译成查询参数——"综合讨论区必须带 category=discussion"
// 这条规则只在 api 层写一遍，学员端与管理端共用同一份映射。
const activeTab = ref<ForumTab>('all')
const keyword = ref('')
const expandedRows = ref<number[]>([])
const replyMap = ref<Record<number, AdminForumReply[]>>({})
const detailLoadingId = ref<number | null>(null)

// ===== 举报管理（ADR-0018）=====
const activeMainTab = ref<'topics' | 'reports'>('topics')
const reportLoading = ref(false)
const reports = ref<AdminForumReportItem[]>([])
const reportTotal = ref(0)
const reportCurrentPage = ref(1)
const reportPageSize = ref(20)
const reportStatus = ref(-1)

function handleMainTabChange(tab: string | number) {
  if (tab === 'reports' && reports.value.length === 0) {
    loadReports()
  }
}

function handleReportStatusChange() {
  reportCurrentPage.value = 1
  loadReports()
}

async function loadReports() {
  reportLoading.value = true
  try {
    const res = await adminForumApi.listReports({
      status: reportStatus.value >= 0 ? reportStatus.value : undefined,
      page: reportCurrentPage.value,
      page_size: reportPageSize.value
    })
    reports.value = res.reports || []
    reportTotal.value = res.total || 0
  } catch (e) {
    console.error('加载举报列表失败:', e)
    /* 错误已由拦截器提示 */
  } finally {
    reportLoading.value = false
  }
}

async function handleReport(row: AdminForumReportItem) {
  try {
    await ElMessageBox.confirm('确认将该举报标记为已处理？', '处理举报', { type: 'warning' })
  } catch {
    return
  }
  try {
    await adminForumApi.handleReport(row.id, 1)
    ElMessage.success('已标记处理')
    loadReports()
  } catch (e) {
    console.error('处理举报失败:', e)
    /* 错误已由拦截器提示 */
  }
}

function displayName(author: AdminForumTopic['author']) {
  return author.username
}

function handleTabChange() {
  currentPage.value = 1
  loadList()
}

function handleSearch() {
  currentPage.value = 1
  loadList()
}

async function handleExpand(row: AdminForumTopic, expandedRowsNow: AdminForumTopic[]) {
  const expanded = expandedRowsNow.map(r => r.id)
  expandedRows.value = expanded
  if (expanded.includes(row.id) && !replyMap.value[row.id]) {
    await loadReplies(row.id)
  }
}

async function loadReplies(topicId: number) {
  detailLoadingId.value = topicId
  try {
    const res = await adminForumApi.getTopic(topicId)
    replyMap.value = { ...replyMap.value, [topicId]: res.replies || [] }
  } catch (e) {
    console.error('加载回复失败:', e)
    /* 错误已由拦截器提示 */
  } finally {
    detailLoadingId.value = null
  }
}

async function deleteTopic(row: AdminForumTopic) {
  try {
    await ElMessageBox.confirm(`确定删除帖子「${row.title}」？删除后不可恢复。`, '删除帖子', { type: 'warning' })
  } catch {
    return
  }
  try {
    await adminForumApi.deleteTopic(row.id)
    ElMessage.success('已删除')
    delete replyMap.value[row.id]
    loadList()
  } catch (e) {
    console.error('删除失败:', e)
    /* 错误已由拦截器提示 */
  }
}

async function deleteReply(reply: AdminForumReply) {
  try {
    await ElMessageBox.confirm('确定删除这条回复？删除后不可恢复。', '删除回复', { type: 'warning' })
  } catch {
    return
  }
  try {
    await adminForumApi.deleteReply(reply.id)
    ElMessage.success('已删除')
    if (replyMap.value[reply.topic_id]) {
      replyMap.value = {
        ...replyMap.value,
        [reply.topic_id]: replyMap.value[reply.topic_id].filter(r => r.id !== reply.id)
      }
    }
    loadList()
  } catch (e) {
    console.error('删除失败:', e)
    /* 错误已由拦截器提示 */
  }
}

onMounted(loadList)
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

.report-target {
  margin-left: 8px;
  font-size: 13px;
}

.report-done {
  color: var(--color-text-disabled);
}

.filter-bar {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}

.title-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.title-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.expand-replies {
  padding: 8px 20px 8px 60px;
}

.topic-content {
  margin-bottom: 14px;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--color-border-light);
}

.topic-content-text {
  font-size: 14px;
  color: var(--color-text-primary);
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-word;
}

.reply-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.reply-item {
  border-bottom: 1px solid var(--color-bg-page);
  padding-bottom: 10px;
}

.reply-item:last-child {
  border-bottom: none;
  padding-bottom: 0;
}

.reply-meta {
  display: flex;
  align-items: center;
  gap: 8px;
}

.reply-author {
  font-weight: 600;
  color: var(--color-text-primary);
}

.reply-quote {
  font-size: 12px;
  color: var(--color-text-tertiary);
  background: var(--color-bg-page);
  border-radius: 6px;
  padding: 1px 6px;
}

.reply-time {
  font-size: 12px;
  color: var(--color-text-tertiary);
}

.reply-delete {
  margin-left: auto;
}

.reply-content {
  margin-top: 4px;
  color: var(--color-text-primary);
  font-size: 13px;
  white-space: pre-wrap;
  word-break: break-word;
}

.reply-loading {
  color: var(--color-text-tertiary);
  font-size: 13px;
}

.pagination-wrapper {
  display: flex;
  justify-content: center;
  margin-top: 16px;
}
</style>
