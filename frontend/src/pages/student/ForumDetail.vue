<template>
  <div class="forum-detail-page" v-loading="loading">
    <div class="back-bar">
      <el-button text :icon="ArrowLeft" @click="goBack">返回列表</el-button>
    </div>

    <template v-if="topic">
      <div class="topic-card">
        <div class="topic-header">
          <el-avatar :size="46" :src="topic.author.avatar_url || undefined" class="author-avatar">
            {{ authorLetter(topic.author) }}
          </el-avatar>
          <div class="topic-author-info">
            <span class="author-name">{{ displayName(topic.author) }}</span>
            <span class="topic-time">{{ formatTime(topic.created_at) }}</span>
          </div>
          <el-button
            v-if="topic.can_delete"
            class="delete-btn"
            type="danger"
            text
            @click="removeTopic"
          >
            删除
          </el-button>
        </div>
        <div class="topic-body">
          <div class="topic-title-row">
            <el-tag v-if="topic.chapter_id" size="small" type="warning">
              {{ topic.chapter_title || '章节讨论' }}
            </el-tag>
            <el-tag v-else size="small" type="info">综合</el-tag>
            <h1 class="topic-title">{{ topic.title }}</h1>
          </div>
          <div class="topic-content">{{ topic.content }}</div>
          <ForumImageGallery :images="topic.images" />
          <div class="topic-stats">
            <el-icon><View /></el-icon>
            {{ topic.view_count }} 次浏览
            <el-icon class="reply-icon"><ChatDotRound /></el-icon>
            {{ topic.reply_count }} 条回复
          </div>
        </div>
      </div>

      <div class="replies-card">
        <h3 class="replies-title">全部回复（{{ replies.length }}）</h3>
        <template v-if="replies.length > 0">
          <div v-for="reply in replies" :key="reply.id" class="reply-item">
            <el-avatar :size="38" :src="reply.author.avatar_url || undefined" class="author-avatar">
              {{ authorLetter(reply.author) }}
            </el-avatar>
            <div class="reply-main">
              <div class="reply-meta">
                <span class="author-name">{{ displayName(reply.author) }}</span>
                <span class="reply-time">{{ formatTime(reply.created_at) }}</span>
                <el-button
                  class="reply-btn"
                  type="primary"
                  size="small"
                  @click="startReplyTo(reply)"
                >
                  回复
                </el-button>
                <el-button
                  v-if="reply.can_delete"
                  class="delete-btn"
                  type="danger"
                  text
                  size="small"
                  @click="removeReply(reply.id)"
                >
                  删除
                </el-button>
              </div>
              <div v-if="reply.parent_id && reply.parent_name" class="reply-quote">
                回复 @{{ reply.parent_name }}
              </div>
              <div class="reply-content">{{ reply.content }}</div>
              <ForumImageGallery :images="reply.images" />
            </div>
          </div>
        </template>
        <el-empty v-else description="暂无回复，来说两句吧" :image-size="80" />
      </div>

      <div class="reply-editor">
        <div v-if="replyingTo" class="replying-bar">
          <el-tag closable type="info" size="small" @close="replyingTo = null">
            回复 @{{ replyingTo.name }}
          </el-tag>
        </div>
        <el-input
          v-model="replyContent"
          type="textarea"
          :rows="4"
          maxlength="5000"
          show-word-limit
          placeholder="写下你的回复…"
        />
        <ForumImageUploader v-model="replyImages" :max="3" />
        <div class="editor-actions">
          <el-button type="primary" :loading="submitting" @click="submitReply">发表回复</el-button>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowLeft, View, ChatDotRound } from '@element-plus/icons-vue'
import { forumApi, type ForumTopicItem, type ForumReplyItem } from '@/api/forum'
import ForumImageGallery from '@/components/student/ForumImageGallery.vue'
import ForumImageUploader from '@/components/student/ForumImageUploader.vue'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const submitting = ref(false)
const topic = ref<ForumTopicItem | null>(null)
const replies = ref<ForumReplyItem[]>([])
const replyContent = ref('')
const replyImages = ref<string[]>([])
const replyingTo = ref<{ id: number; name: string } | null>(null)

function displayName(author: ForumTopicItem['author']) {
  return author.nickname || author.name || author.username
}

function authorLetter(author: ForumTopicItem['author']) {
  return (displayName(author) || '?').charAt(0).toUpperCase()
}

function formatTime(iso: string) {
  if (!iso) return ''
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

async function loadDetail() {
  loading.value = true
  try {
    const topicId = Number(route.params.topicId)
    const res = await forumApi.getTopic(topicId)
    topic.value = res.topic
    replies.value = res.replies || []
  } catch (e) {
    console.error('加载帖子详情失败:', e)
    ElMessage.error('加载帖子详情失败')
  } finally {
    loading.value = false
  }
}

async function submitReply() {
  const content = replyContent.value.trim()
  if (!content) {
    ElMessage.warning('请输入回复内容')
    return
  }
  submitting.value = true
  try {
    const topicId = Number(route.params.topicId)
    await forumApi.replyTopic(topicId, content, replyingTo.value?.id, replyImages.value)
    ElMessage.success('回复成功')
    replyContent.value = ''
    replyImages.value = []
    replyingTo.value = null
    loadDetail()
  } catch (e) {
    console.error('回复失败:', e)
    ElMessage.error('回复失败，请稍后重试')
  } finally {
    submitting.value = false
  }
}

function startReplyTo(reply: ForumReplyItem) {
  replyingTo.value = { id: reply.id, name: displayName(reply.author) }
}

async function removeTopic() {
  try {
    await ElMessageBox.confirm('确定删除这个帖子吗？删除后无法恢复。', '删除帖子', { type: 'warning' })
  } catch {
    return
  }
  try {
    await forumApi.deleteTopic(Number(route.params.topicId))
    ElMessage.success('已删除')
    goBack()
  } catch (e) {
    console.error('删除失败:', e)
    ElMessage.error('删除失败')
  }
}

async function removeReply(replyId: number) {
  try {
    await ElMessageBox.confirm('确定删除这条回复吗？', '删除回复', { type: 'warning' })
  } catch {
    return
  }
  try {
    await forumApi.deleteReply(replyId)
    ElMessage.success('已删除')
    loadDetail()
  } catch (e) {
    console.error('删除失败:', e)
    ElMessage.error('删除失败')
  }
}

function goBack() {
  if (window.history.length > 1) {
    router.back()
  } else {
    router.push({ name: 'ForumPage' })
  }
}

onMounted(loadDetail)
</script>

<style scoped>
.forum-detail-page {
  max-width: 900px;
  margin: 0 auto;
  padding: 0 16px 40px;
}

.back-bar {
  margin-bottom: 12px;
}

.topic-card,
.replies-card,
.reply-editor {
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
  padding: 20px;
  margin-bottom: 16px;
}

.topic-header {
  display: flex;
  align-items: center;
  gap: 12px;
}

.topic-author-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.author-name {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
}

.topic-time,
.reply-time {
  font-size: 12px;
  color: #909399;
}

.delete-btn {
  margin-left: auto;
}

.topic-body {
  margin-top: 16px;
}

.topic-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}

.topic-title {
  font-size: 20px;
  font-weight: 600;
  color: #303133;
  margin: 0;
}

.topic-content {
  color: #303133;
  font-size: 15px;
  line-height: 1.8;
  white-space: pre-wrap;
  word-break: break-word;
}

.topic-stats {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 16px;
  font-size: 13px;
  color: #909399;
}

.reply-icon {
  margin-left: 12px;
}

.replies-title {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
  margin: 0 0 8px;
}

.reply-item {
  display: flex;
  gap: 12px;
  padding: 16px 0;
  border-bottom: 1px solid #f5f5f5;
}

.reply-item:last-child {
  border-bottom: none;
}

.reply-main {
  flex: 1;
  min-width: 0;
}

.reply-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}

.reply-content {
  color: #303133;
  font-size: 14px;
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-word;
}

.reply-quote {
  font-size: 12px;
  color: #909399;
  background: #f5f7fa;
  border-radius: 6px;
  padding: 2px 8px;
  margin-bottom: 4px;
  display: inline-block;
}

.replying-bar {
  margin-bottom: 10px;
}

.editor-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 12px;
}
</style>
