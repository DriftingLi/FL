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
            <span class="topic-time">{{ formatLocaleDateTime(topic.created_at, '') }}</span>
          </div>
          <div class="topic-actions">
            <el-tooltip :content="topicFavorited ? '取消收藏' : '收藏'" placement="top">
              <el-button
                :icon="topicFavorited ? StarFilled : Star"
                :type="topicFavorited ? 'warning' : 'default'"
                circle
                size="small"
                @click="toggleFavorite"
              />
            </el-tooltip>
            <el-button text size="small" @click="openReport('topic')">举报</el-button>
            <el-button
              v-if="topic.can_delete"
              class="delete-btn"
              type="danger"
              text
              size="small"
              @click="removeTopic"
            >
              删除
            </el-button>
          </div>
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
                <span class="reply-time">{{ formatLocaleDateTime(reply.created_at, '') }}</span>
                <el-button
                  class="reply-btn"
                  type="primary"
                  size="small"
                  @click="startReplyTo(reply)"
                >
                  回复
                </el-button>
                <el-button
                  class="reply-btn"
                  text
                  size="small"
                  @click="openReport('reply', reply.id)"
                >
                  举报
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

      <!-- 举报对话框（帖子/回复共用，ADR-0018） -->
      <el-dialog v-model="reportVisible" title="举报" width="440px">
        <el-input
          v-model="reportReason"
          type="textarea"
          :rows="4"
          maxlength="500"
          show-word-limit
          placeholder="请填写举报理由（1-500 字）"
        />
        <template #footer>
          <el-button @click="reportVisible = false">取消</el-button>
          <el-button type="primary" :loading="reportSubmitting" @click="submitReport">提交</el-button>
        </template>
      </el-dialog>

      <div class="reply-editor">
        <div v-if="replyingTo" class="replying-bar">
          <el-tag closable type="info" size="small" @close="replyingTo = null">
            回复 @{{ replyingTo.username }}
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
import { ArrowLeft, View, ChatDotRound, Star, StarFilled } from '@element-plus/icons-vue'
import { forumApi, type ForumTopicItem, type ForumReplyItem } from '@/api/forum'
import { favoriteApi } from '@/api/favorite'
import ForumImageGallery from '@/components/student/ForumImageGallery.vue'
import ForumImageUploader from '@/components/student/ForumImageUploader.vue'
import { formatLocaleDateTime } from '@/utils/format'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const submitting = ref(false)
const topic = ref<ForumTopicItem | null>(null)
const replies = ref<ForumReplyItem[]>([])
const replyContent = ref('')
const replyImages = ref<string[]>([])
const replyingTo = ref<{ id: number; username: string } | null>(null)

function displayName(author: ForumTopicItem['author']) {
  return author.username
}

function authorLetter(author: ForumTopicItem['author']) {
  return (displayName(author) || '?').charAt(0).toUpperCase()
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
    /* 错误已由拦截器提示 */
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
    /* 错误已由拦截器提示 */
  } finally {
    submitting.value = false
  }
}

function startReplyTo(reply: ForumReplyItem) {
  replyingTo.value = { id: reply.id, username: displayName(reply.author) }
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
    /* 错误已由拦截器提示 */
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
    /* 错误已由拦截器提示 */
  }
}

function goBack() {
  if (window.history.length > 1) {
    router.back()
  } else {
    router.push({ name: 'ForumPage' })
  }
}

// ===== 互动（ADR-0018）：收藏 / 举报 =====
// （点赞 web 端不展示：星形与收藏重复且无后续消费场景，API 保留供移动端使用）

// 收藏帖子
const topicFavorited = ref(false)
const topicFavoriteId = ref<number>(0)

async function loadFavoriteState() {
  topicFavorited.value = false
  topicFavoriteId.value = 0
  try {
    const res = await favoriteApi.check({ target_type: 'topic', target_id: Number(route.params.topicId) })
    topicFavorited.value = !!res?.favorited
    topicFavoriteId.value = res?.favorite_id || 0
  } catch (e) {
    console.error('查询收藏状态失败:', e)
  }
}

async function toggleFavorite() {
  const topicId = Number(route.params.topicId)
  try {
    if (topicFavorited.value) {
      await favoriteApi.remove(topicFavoriteId.value)
      topicFavorited.value = false
      topicFavoriteId.value = 0
      ElMessage.success('已取消收藏')
    } else {
      const res = await favoriteApi.add({ target_type: 'topic', target_id: topicId })
      topicFavorited.value = true
      topicFavoriteId.value = res?.favorite_id || 0
      ElMessage.success('已收藏')
    }
  } catch (e) {
    console.error('收藏操作失败:', e)
    /* 错误已由拦截器提示 */
  }
}

// 举报（帖子/回复共用对话框）
const reportVisible = ref(false)
const reportReason = ref('')
const reportSubmitting = ref(false)
const reportTarget = ref<{ kind: 'topic' | 'reply'; id: number } | null>(null)

function openReport(kind: 'topic' | 'reply', replyId?: number) {
  reportTarget.value = { kind, id: kind === 'topic' ? Number(route.params.topicId) : replyId! }
  reportReason.value = ''
  reportVisible.value = true
}

async function submitReport() {
  const reason = reportReason.value.trim()
  if (!reportTarget.value) return
  if (reason.length < 1 || reason.length > 500) {
    ElMessage.warning('举报理由需为 1-500 字')
    return
  }
  reportSubmitting.value = true
  try {
    const { kind, id } = reportTarget.value
    if (kind === 'topic') {
      await forumApi.reportTopic(id, reason)
    } else {
      await forumApi.reportReply(id, reason)
    }
    ElMessage.success('举报已提交，等待处理')
    reportVisible.value = false
  } catch (e) {
    console.error('举报失败:', e)
    /* 错误已由拦截器提示 */
  } finally {
    reportSubmitting.value = false
  }
}

onMounted(() => {
  loadDetail()
  loadFavoriteState()
})
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

.topic-actions {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-left: auto;
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
