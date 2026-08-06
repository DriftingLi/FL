<template>
  <div class="forum-manage-page">
    <el-card>
      <template #header>
        <div class="card-header">
          <span class="card-title">论坛管理</span>
          <el-button :icon="Refresh" circle @click="loadList" />
        </div>
      </template>

      <div class="filter-bar">
        <el-tabs v-model="activeScope" @tab-change="handleScopeChange">
          <el-tab-pane label="全部帖子" name="all" />
          <el-tab-pane label="综合讨论区" name="general" />
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
                      <span class="reply-time">{{ formatTime(reply.created_at) }}</span>
                      <el-button
                        class="reply-delete"
                        type="danger"
                        size="small"
                        @click="deleteReply(reply)"
                      >
                        删除回复
                      </el-button>
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
              <el-tag v-if="row.chapter_id" size="small" type="warning">
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
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="90" fixed="right" align="center">
          <template #default="{ row }">
            <el-button type="danger" size="small" @click="deleteTopic(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-wrapper" v-if="total > pageSize">
        <el-pagination
          v-model:current-page="currentPage"
          :page-size="pageSize"
          :total="total"
          layout="total, prev, pager, next"
          @current-change="loadList"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Search } from '@element-plus/icons-vue'
import {
  adminApi,
  type AdminForumTopic,
  type AdminForumReply
} from '@/api/admin'
import ForumImageGallery from '@/components/student/ForumImageGallery.vue'

const loading = ref(false)
const topics = ref<AdminForumTopic[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(10)
const activeScope = ref<'all' | 'general'>('all')
const keyword = ref('')
const expandedRows = ref<number[]>([])
const replyMap = ref<Record<number, AdminForumReply[]>>({})
const detailLoadingId = ref<number | null>(null)

function displayName(author: AdminForumTopic['author']) {
  return author.nickname || author.name || author.username
}

function formatTime(iso: string) {
  if (!iso) return '-'
  const d = new Date(iso)
  if (isNaN(d.getTime())) return iso
  return d.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  })
}

async function loadList() {
  loading.value = true
  try {
    const res = await adminApi.listAdminForumTopics({
      scope: activeScope.value,
      page: currentPage.value,
      page_size: pageSize.value,
      keyword: keyword.value || undefined
    })
    if (res.code === 200 && res.data) {
      topics.value = res.data.topics || []
      total.value = res.data.total || 0
    }
  } catch (e) {
    console.error('加载论坛列表失败:', e)
    ElMessage.error('加载失败')
  } finally {
    loading.value = false
  }
}

function handleScopeChange() {
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
    const res = await adminApi.getAdminForumTopic(topicId)
    if (res.code === 200 && res.data) {
      replyMap.value = { ...replyMap.value, [topicId]: res.data.replies || [] }
    }
  } catch (e) {
    console.error('加载回复失败:', e)
    ElMessage.error('加载回复失败')
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
    await adminApi.deleteAdminForumTopic(row.id)
    ElMessage.success('已删除')
    delete replyMap.value[row.id]
    loadList()
  } catch (e) {
    console.error('删除失败:', e)
    ElMessage.error('删除失败')
  }
}

async function deleteReply(reply: AdminForumReply) {
  try {
    await ElMessageBox.confirm('确定删除这条回复？删除后不可恢复。', '删除回复', { type: 'warning' })
  } catch {
    return
  }
  try {
    await adminApi.deleteAdminForumReply(reply.id)
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
    ElMessage.error('删除失败')
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
  border-bottom: 1px solid #ebeef5;
}

.topic-content-text {
  font-size: 14px;
  color: #303133;
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
  border-bottom: 1px solid #f0f0f0;
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
  color: #303133;
}

.reply-quote {
  font-size: 12px;
  color: #909399;
  background: #f5f7fa;
  border-radius: 6px;
  padding: 1px 6px;
}

.reply-time {
  font-size: 12px;
  color: #909399;
}

.reply-delete {
  margin-left: auto;
}

.reply-content {
  margin-top: 4px;
  color: #303133;
  font-size: 13px;
  white-space: pre-wrap;
  word-break: break-word;
}

.reply-loading {
  color: #909399;
  font-size: 13px;
}

.pagination-wrapper {
  display: flex;
  justify-content: center;
  margin-top: 16px;
}
</style>
