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
            <el-tooltip :content="topic?.liked_by_me ? '取消点赞' : '点赞'" placement="top">
              <el-button
                :type="topic?.liked_by_me ? 'danger' : 'default'"
                circle
                size="small"
                @click="toggleTopicLike"
              >
                <span class="heart-btn">{{ topic?.liked_by_me ? '♥' : '♡' }}</span>
              </el-button>
            </el-tooltip>
            <span v-if="topic" class="like-count">{{ topic.likes_count || 0 }}</span>
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
            <span class="like-stat">
              <span class="heart" :class="{ liked: topic.liked_by_me }">{{ topic.liked_by_me ? '♥' : '♡' }}</span>
              {{ topic.likes_count || 0 }} 点赞
            </span>
            <el-icon class="reply-icon"><ChatDotRound /></el-icon>
            {{ topic.reply_count }} 条回复
          </div>
        </div>
      </div>

      <div class="replies-card">
        <div class="replies-header">
          <h3 class="replies-title">全部回复（{{ replies.length }}）</h3>
          <div style="display:flex; gap:8px; align-items:center;">
            <el-radio-group v-model="replySort" size="small" @change="handleReplySortChange">
              <el-radio-button value="latest">最新</el-radio-button>
              <el-radio-button value="hot">热门</el-radio-button>
            </el-radio-group>
            <el-button size="small" :icon="replyOrder==='asc'? ArrowUp : ArrowDown" @click="toggleReplyOrder">{{ replyOrder==='asc' ? '正序' : '逆序' }}</el-button>
          </div>
        </div>
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
                  class="like-btn"
                  :type="reply.liked_by_me ? 'danger' : 'default'"
                  text
                  size="small"
                  @click="toggleReplyLike(reply)"
                >
                  <span class="heart">{{ reply.liked_by_me ? '♥' : '♡' }}</span>
                  <span v-if="(reply.likes_count || 0) > 0" class="like-count-inline">{{ reply.likes_count }}</span>
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
        <div class="reply-box">
          <div v-if="replyImages.length > 0" class="reply-images-bar">
            <div v-for="(url, index) in replyImages" :key="url + index" class="reply-thumb">
              <el-image :src="resolveFileUrl(url)" fit="cover" class="reply-thumb-img" />
              <button type="button" class="reply-thumb-remove" @click="removeReplyImage(index)">
                <el-icon><Close /></el-icon>
              </button>
            </div>
          </div>
          <el-input
            v-model="replyContent"
            type="textarea"
            :rows="3"
            maxlength="5000"
            show-word-limit
            placeholder="写下你的回复…"
            class="reply-textarea"
          />
          <div class="reply-box-footer">
            <el-button :icon="Paperclip" text circle size="small" :disabled="replyImages.length >= 3" title="添加图片" @click="triggerReplyFile" />
            <el-button :icon="Promotion" type="primary" circle size="small" :loading="submitting" :disabled="!canSubmitReply" title="发表回复" @click="submitReply" />
          </div>
        </div>
        <input ref="replyFileInput" type="file" accept="image/*" multiple style="display: none" @change="handleReplyFileSelect" />
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowLeft, View, ChatDotRound, Star, StarFilled, Paperclip, Promotion, Close, ArrowUp, ArrowDown } from '@element-plus/icons-vue'
import { forumApi, type ForumTopicItem, type ForumReplyItem } from '@/api/forum'
import { favoriteApi } from '@/api/favorite'
import ForumImageGallery from '@/components/student/ForumImageGallery.vue'
import { formatLocaleDateTime } from '@/utils/format'
import { resolveFileUrl } from '@/utils/fileUrl'
import { pushHistory, toHistoryItem } from '@/utils/forumHistory'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

const loading = ref(false)
const submitting = ref(false)
const topic = ref<ForumTopicItem | null>(null)
const replies = ref<ForumReplyItem[]>([])
const replyContent = ref('')
const replyImages = ref<string[]>([])
const replyingTo = ref<{ id: number; username: string } | null>(null)
const replyFileInput = ref<HTMLInputElement | null>(null)
const canSubmitReply = computed(() => replyContent.value.trim().length > 0 || replyImages.value.length > 0)
const replySort = ref<'latest' | 'hot'>('latest')
const replyOrder = ref<'asc' | 'desc'>('asc')

function handleReplySortChange() {
  // 热门默认逆序，最新默认正序
  replyOrder.value = replySort.value === 'hot' ? 'desc' : 'asc'
  loadDetail()
}

function toggleReplyOrder(){
  replyOrder.value = replyOrder.value === 'asc' ? 'desc' : 'asc'
  loadDetail()
}

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
    const res = await forumApi.getTopic(topicId, replySort.value, replyOrder.value)
    topic.value = res.topic
    replies.value = res.replies || []
    if (res.topic) {
      try {
        pushHistory(toHistoryItem(res.topic), authStore.userInfo?.user_id)
      } catch {
        // ignore storage errors
      }
    }
  } catch (e) {
    console.error('加载帖子详情失败:', e)
    /* 错误已由拦截器提示 */
  } finally {
    loading.value = false
  }
}

async function submitReply() {
  const content = replyContent.value.trim()
  if (!content && replyImages.value.length === 0) {
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

function triggerReplyFile() {
  replyFileInput.value?.click()
}

function handleReplyFileSelect(event: Event) {
  const target = event.target as HTMLInputElement
  const files = Array.from(target.files ?? [])
  if (files.length > 0) uploadReplyFiles(files)
  target.value = ''
}

async function uploadReplyFiles(files: File[]) {
  const remaining = 3 - replyImages.value.length
  if (remaining <= 0) {
    ElMessage.warning('最多上传 3 张图片')
    return
  }
  const toUpload = files.filter(f => f.type.startsWith('image/')).slice(0, remaining)
  if (toUpload.length === 0) return
  for (const file of toUpload) {
    if (file.size > 20 * 1024 * 1024) {
      ElMessage.error(`"${file.name}" 超过 20MB，已跳过`)
      continue
    }
    const formData = new FormData()
    formData.append('file', file)
    try {
      const res = await forumApi.uploadImage(formData)
      if (res?.url) {
        if (replyImages.value.length >= 3) break
        replyImages.value = [...replyImages.value, res.url]
      } else {
        ElMessage.error(`"${file.name}" 上传失败`)
      }
    } catch {
      /* 错误已由拦截器提示 */
    }
  }
}

function removeReplyImage(index: number) {
  const next = [...replyImages.value]
  next.splice(index, 1)
  replyImages.value = next
}

function handlePaste(event: ClipboardEvent) {
  const items = event.clipboardData?.items
  if (!items) return
  if (replyImages.value.length >= 3) return
  const files: File[] = []
  for (const item of items) {
    if (item.kind === 'file' && item.type.startsWith('image/')) {
      const file = item.getAsFile()
      if (file) files.push(file)
    }
  }
  if (files.length > 0) {
    event.preventDefault()
    uploadReplyFiles(files)
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

// ===== 互动（ADR-0018）：收藏 / 点赞 / 举报 =====

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

async function toggleTopicLike() {
  if (!topic.value) return
  const topicId = Number(route.params.topicId)
  const prevLiked = !!topic.value.liked_by_me
  const prevCount = topic.value.likes_count || 0
  // 乐观更新
  topic.value.liked_by_me = !prevLiked
  topic.value.likes_count = prevLiked ? Math.max(0, prevCount - 1) : prevCount + 1
  try {
    const res = prevLiked ? await forumApi.unlikeTopic(topicId) : await forumApi.likeTopic(topicId)
    if (topic.value) {
      topic.value.likes_count = res.likes_count
      topic.value.liked_by_me = res.liked
    }
  } catch (e) {
    // 回滚
    if (topic.value) {
      topic.value.liked_by_me = prevLiked
      topic.value.likes_count = prevCount
    }
    console.error('点赞操作失败:', e)
  }
}

async function toggleReplyLike(reply: ForumReplyItem) {
  const prevLiked = !!reply.liked_by_me
  const prevCount = reply.likes_count || 0
  reply.liked_by_me = !prevLiked
  reply.likes_count = prevLiked ? Math.max(0, prevCount - 1) : prevCount + 1
  try {
    const res = prevLiked ? await forumApi.unlikeReply(reply.id) : await forumApi.likeReply(reply.id)
    reply.likes_count = res.likes_count
    reply.liked_by_me = res.liked
  } catch (e) {
    reply.liked_by_me = prevLiked
    reply.likes_count = prevCount
    console.error('评论点赞失败:', e)
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
  document.addEventListener('paste', handlePaste)
})

onBeforeUnmount(() => {
  document.removeEventListener('paste', handlePaste)
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

.like-count {
  font-size: 12px;
  color: #f56c6c;
  min-width: 16px;
  text-align: left;
}

.heart-btn {
  font-size: 14px;
  line-height: 1;
}

.like-stat {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  margin-left: 12px;
  color: #f56c6c;
}

.like-stat .heart.liked {
  color: #f56c6c;
}

.reply-icon {
  margin-left: 12px;
}

.replies-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.replies-title {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
  margin: 0;
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

.like-btn .heart {
  font-size: 13px;
  line-height: 1;
}

.like-count-inline {
  font-size: 12px;
  margin-left: 2px;
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

.reply-box {
  border: 1px solid #dcdfe6;
  border-radius: 8px;
  background: #fff;
  padding: 8px;
  transition: border-color 0.2s;
}

.reply-box:focus-within {
  border-color: #409eff;
}

.reply-images-bar {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 8px;
}

.reply-thumb {
  position: relative;
  width: 48px;
  height: 48px;
  border-radius: 6px;
  overflow: hidden;
  border: 1px solid #ebeef5;
  flex-shrink: 0;
}

.reply-thumb-img {
  width: 100%;
  height: 100%;
}

.reply-thumb-remove {
  position: absolute;
  top: 0;
  right: 0;
  width: 16px;
  height: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: none;
  border-radius: 0 0 0 6px;
  background: rgba(0, 0, 0, 0.55);
  color: #fff;
  cursor: pointer;
  padding: 0;
  font-size: 10px;
}

.reply-thumb-remove:hover {
  background: rgba(245, 108, 108, 0.9);
}

.reply-textarea :deep(.el-textarea__inner) {
  border: none !important;
  box-shadow: none !important;
  padding: 4px 0;
  resize: none;
}

.reply-box-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 6px;
}
</style>
