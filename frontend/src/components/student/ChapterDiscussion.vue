<template>
  <div class="chapter-discussion">
    <div class="discussion-header">
      <div>
        <h3 class="discussion-title">章节讨论</h3>
        <p class="discussion-subtitle">针对本章内容提问或交流</p>
      </div>
      <el-button type="primary" size="small" :icon="EditPen" @click="openCreate">发新帖</el-button>
    </div>

    <div v-loading="listLoading" class="discussion-list">
      <template v-if="topics.length > 0">
        <div v-for="topic in topics" :key="topic.id" class="discussion-item">
          <div class="discussion-item-main" @click="toggleTopic(topic.id)">
            <div class="discussion-title-row">
              <h4 class="discussion-item-title">{{ topic.title }}</h4>
              <span class="discussion-count">{{ topic.reply_count }} 回复</span>
            </div>
            <div class="discussion-meta">
              <el-avatar :size="24" :src="topic.author.avatar_url || undefined" class="meta-avatar">
                {{ authorLetter(topic.author) }}
              </el-avatar>
              <span class="discussion-author">{{ displayName(topic.author) }}</span>
              <span class="discussion-time">{{ formatTime(topic.created_at) }}</span>
              <el-icon class="expand-icon">
                <ArrowDown v-if="expandedTopicId !== topic.id" />
                <ArrowUp v-else />
              </el-icon>
            </div>
          </div>

          <div v-if="expandedTopicId === topic.id" v-loading="detailLoading" class="discussion-detail">
            <div class="detail-content">{{ detailContent }}</div>
            <ForumImageGallery :images="expandedTopic?.images" />

            <div class="detail-replies">
              <template v-if="replies.length > 0">
                <div v-for="reply in replies" :key="reply.id" class="detail-reply">
                  <div class="reply-head">
                    <el-avatar :size="26" :src="reply.author.avatar_url || undefined" class="meta-avatar">
                      {{ authorLetter(reply.author) }}
                    </el-avatar>
                    <span class="reply-author">{{ displayName(reply.author) }}</span>
                    <span class="reply-time">{{ formatTime(reply.created_at) }}</span>
                    <div class="reply-actions">
                      <el-button type="primary" size="small" @click="startReplyTo(reply)">回复</el-button>
                      <el-button v-if="reply.can_delete" text type="danger" size="small" @click="removeReply(reply.id)">
                        删除
                      </el-button>
                    </div>
                  </div>
                  <div v-if="reply.parent_id && reply.parent_name" class="reply-quote">
                    回复 @{{ reply.parent_name }}
                  </div>
                  <div class="reply-content">{{ reply.content }}</div>
                  <ForumImageGallery :images="reply.images" />
                </div>
              </template>
              <el-empty v-else description="还没有回复" :image-size="60" />
            </div>

            <div class="reply-input-bar">
              <el-tag v-if="replyingTo" closable size="small" type="info" @close="replyingTo = null">
                回复 @{{ replyingTo.name }}
              </el-tag>
              <div class="reply-input-row">
                <el-input
                  v-model="replyContent"
                  type="textarea"
                  :rows="2"
                  maxlength="5000"
                  placeholder="写下你的回复…"
                />
                <el-button type="primary" :loading="replying" @click="submitReply(topic.id)">发表</el-button>
              </div>
              <ForumImageUploader v-model="replyImages" :max="3" />
            </div>

            <div class="detail-actions">
              <el-button v-if="expandedTopic?.can_delete" text type="danger" size="small" @click="removeTopic(topic.id)">
                删除本帖
              </el-button>
            </div>
          </div>
        </div>
      </template>
      <el-empty v-else-if="!listLoading" description="本章还没有讨论，来发第一帖吧" :image-size="80" />
    </div>

    <el-dialog v-model="createVisible" title="发布章节讨论" width="620px">
      <el-form label-width="70px">
        <el-form-item label="标题" required>
          <el-input v-model="createForm.title" maxlength="100" show-word-limit placeholder="请输入标题（1-100 字）" />
        </el-form-item>
        <el-form-item label="内容" required>
          <el-input
            v-model="createForm.content"
            type="textarea"
            :rows="6"
            maxlength="10000"
            show-word-limit
            placeholder="请输入内容（1-10000 字）"
          />
        </el-form-item>
        <el-form-item label="图片">
          <ForumImageUploader v-model="createForm.images" :max="9" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="submitCreate">发布</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { EditPen, ArrowDown, ArrowUp } from '@element-plus/icons-vue'
import { forumApi, type ForumTopicItem, type ForumReplyItem } from '@/api/forum'
import ForumImageGallery from '@/components/student/ForumImageGallery.vue'
import ForumImageUploader from '@/components/student/ForumImageUploader.vue'

const props = defineProps<{
  chapterId: number
}>()

const listLoading = ref(false)
const detailLoading = ref(false)
const replying = ref(false)
const creating = ref(false)
const topics = ref<ForumTopicItem[]>([])
const expandedTopicId = ref<number | null>(null)
const expandedTopic = ref<ForumTopicItem | null>(null)
const detailContent = ref('')
const replies = ref<ForumReplyItem[]>([])
const replyContent = ref('')
const replyImages = ref<string[]>([])
const replyingTo = ref<{ id: number; name: string } | null>(null)
const createVisible = ref(false)
const createForm = ref<{ title: string; content: string; images: string[] }>({ title: '', content: '', images: [] })

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
  const now = new Date()
  const diff = now.getTime() - d.getTime()
  if (diff < 60 * 1000) return '刚刚'
  if (diff < 60 * 60 * 1000) return `${Math.floor(diff / 60000)} 分钟前`
  if (diff < 24 * 60 * 60 * 1000) return `${Math.floor(diff / 3600000)} 小时前`
  return d.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

async function loadTopics() {
  if (!props.chapterId) return
  listLoading.value = true
  try {
    const res = await forumApi.listTopics({
      scope: 'chapter',
      chapter_id: props.chapterId,
      page: 1,
      page_size: 50
    })
    topics.value = res.topics || []
  } catch (e) {
    console.error('加载章节讨论失败:', e)
  } finally {
    listLoading.value = false
  }
}

async function toggleTopic(topicId: number) {
  if (expandedTopicId.value === topicId) {
    expandedTopicId.value = null
    expandedTopic.value = null
    replies.value = []
    return
  }
  expandedTopicId.value = topicId
  expandedTopic.value = null
  await loadDetail(topicId)
}

async function loadDetail(topicId: number) {
  detailLoading.value = true
  detailContent.value = ''
  replies.value = []
  try {
    const res = await forumApi.getTopic(topicId)
    expandedTopic.value = res.topic || null
    detailContent.value = res.topic?.content || ''
    replies.value = res.replies || []
  } catch (e) {
    console.error('加载帖子详情失败:', e)
    /* 错误已由拦截器提示 */
  } finally {
    detailLoading.value = false
  }
}

function openCreate() {
  createForm.value = { title: '', content: '', images: [] }
  createVisible.value = true
}

async function submitCreate() {
  const title = createForm.value.title.trim()
  const content = createForm.value.content.trim()
  if (!title || !content) {
    ElMessage.warning('请填写标题和内容')
    return
  }
  creating.value = true
  try {
    await forumApi.createTopic({ chapter_id: props.chapterId, title, content, images: createForm.value.images })
    ElMessage.success('发布成功')
    createVisible.value = false
    await loadTopics()
  } catch (e) {
    console.error('发布失败:', e)
    /* 错误已由拦截器提示 */
  } finally {
    creating.value = false
  }
}

function startReplyTo(reply: ForumReplyItem) {
  replyingTo.value = { id: reply.id, name: displayName(reply.author) }
}

async function submitReply(topicId: number) {
  const content = replyContent.value.trim()
  if (!content) {
    ElMessage.warning('请输入回复内容')
    return
  }
  replying.value = true
  try {
    await forumApi.replyTopic(topicId, content, replyingTo.value?.id, replyImages.value)
    ElMessage.success('回复成功')
    replyContent.value = ''
    replyImages.value = []
    replyingTo.value = null
    await Promise.all([loadDetail(topicId), loadTopics()])
  } catch (e) {
    console.error('回复失败:', e)
    /* 错误已由拦截器提示 */
  } finally {
    replying.value = false
  }
}

async function removeTopic(topicId: number) {
  try {
    await ElMessageBox.confirm('确定删除这个帖子吗？删除后无法恢复。', '删除帖子', { type: 'warning' })
  } catch {
    return
  }
  try {
    await forumApi.deleteTopic(topicId)
    ElMessage.success('已删除')
  if (expandedTopicId.value === topicId) {
    expandedTopicId.value = null
    expandedTopic.value = null
    replies.value = []
    }
    await loadTopics()
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
    if (expandedTopicId.value) {
      await Promise.all([loadDetail(expandedTopicId.value), loadTopics()])
    }
  } catch (e) {
    console.error('删除失败:', e)
    /* 错误已由拦截器提示 */
  }
}

watch(() => props.chapterId, () => {
  expandedTopicId.value = null
  expandedTopic.value = null
  replies.value = []
  loadTopics()
}, { immediate: true })
</script>

<style scoped>
.chapter-discussion {
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
  padding: 20px;
  margin-bottom: 20px;
}

.discussion-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
  margin-bottom: 14px;
}

.discussion-title {
  font-size: 17px;
  font-weight: 600;
  color: #303133;
  margin: 0 0 4px;
}

.discussion-subtitle {
  font-size: 13px;
  color: #909399;
  margin: 0;
}

.discussion-item {
  border: 1px solid #f0f0f0;
  border-radius: 10px;
  margin-bottom: 12px;
  overflow: hidden;
}

.discussion-item:last-child {
  margin-bottom: 0;
}

.discussion-item-main {
  padding: 12px 14px;
  cursor: pointer;
  transition: background 0.2s;
}

.discussion-item-main:hover {
  background: #f7f9fc;
}

.discussion-title-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 10px;
}

.discussion-item-title {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.discussion-count {
  font-size: 12px;
  color: #909399;
  flex-shrink: 0;
}

.discussion-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
  font-size: 12px;
  color: #909399;
}

.discussion-author {
  color: #606266;
}

.expand-icon {
  margin-left: auto;
}

.discussion-detail {
  border-top: 1px solid #f0f0f0;
  background: #fafbfc;
  padding: 14px;
}

.detail-content {
  font-size: 14px;
  line-height: 1.7;
  color: #303133;
  white-space: pre-wrap;
  word-break: break-word;
  margin-bottom: 14px;
}

.detail-replies {
  background: #fff;
  border-radius: 8px;
  padding: 4px 12px;
}

.detail-reply {
  padding: 12px 0;
  border-bottom: 1px solid #f5f5f5;
}

.detail-reply:last-child {
  border-bottom: none;
}

.reply-head {
  display: flex;
  align-items: center;
  gap: 8px;
}

.reply-author {
  font-size: 13px;
  font-weight: 600;
  color: #303133;
}

.reply-time {
  font-size: 12px;
  color: #909399;
}

.reply-actions {
  margin-left: auto;
}

.reply-quote {
  display: inline-block;
  font-size: 12px;
  color: #909399;
  background: #f5f7fa;
  border-radius: 6px;
  padding: 2px 8px;
  margin: 6px 0 2px;
}

.reply-content {
  font-size: 13px;
  line-height: 1.6;
  color: #303133;
  white-space: pre-wrap;
  word-break: break-word;
}

.reply-input-bar {
  margin-top: 12px;
}

.reply-input-row {
  display: flex;
  gap: 10px;
  align-items: flex-end;
  margin-top: 8px;
}

.reply-input-row .el-input {
  flex: 1;
}

.detail-actions {
  margin-top: 8px;
  text-align: right;
}

@media screen and (max-width: 768px) {
  .chapter-discussion {
    padding: 14px;
  }

  .discussion-item-main {
    padding: 10px 12px;
  }

  .discussion-detail {
    padding: 10px;
  }

  .detail-replies {
    padding: 4px 8px;
  }

  .reply-input-row {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>
