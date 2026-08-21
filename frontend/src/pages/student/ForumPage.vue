<template>
  <div class="forum-page">
    <div class="forum-header">
      <h1 class="forum-title">学员论坛</h1>
      <el-button type="primary" size="large" :icon="EditPen" @click="openCreateDialog">
        发布新帖
      </el-button>
    </div>

    <!-- 每日打卡卡片（spec #268 A1） -->
    <div class="checkin-card" @click="openCheckIn('calendar')">
      <div class="checkin-left">
        <el-icon class="checkin-icon"><Calendar /></el-icon>
        <span class="checkin-text">
          <template v-if="checkInTodayChecked">今日已打卡</template>
          <template v-else>今日未打卡</template>
          <span v-if="checkInTotal > 0" class="checkin-sub">｜已连续 {{ checkInStreak }} 天｜累计 {{ checkInTotal }} 天</span>
        </span>
      </div>
      <div class="checkin-right" @click.stop>
        <el-button type="primary" size="small" :loading="checkInChecking" :disabled="checkInTodayChecked" @click="doCheckIn">
          {{ checkInTodayChecked ? '今日已打卡' : '立即打卡' }}
        </el-button>
        <el-button text size="small" @click="openCheckIn('rank')">排行榜</el-button>
      </div>
    </div>

    <el-radio-group v-model="mode" class="forum-mode" @change="handleModeChange">
      <el-radio-button value="all">全部</el-radio-button>
      <el-radio-button value="my-topics">我的帖子</el-radio-button>
      <el-radio-button value="my-replies">我的回复</el-radio-button>
    </el-radio-group>

    <CheckInDialog v-model="checkInDialogVisible" :initial-tab="checkInTab" @checked="onCheckInChecked" />

    <!-- 我的回复列表（条目带主题标题回填，点击跳对应帖子） -->
    <div v-if="mode === 'my-replies'" v-loading="loading" class="topic-list">
      <template v-if="myReplies.length > 0">
        <div v-for="reply in myReplies" :key="reply.id" class="topic-item" @click="goDetail(reply.topic_id)">
          <div class="topic-main">
            <div class="topic-title-row">
              <el-tag size="small" type="info">回复</el-tag>
              <h3 class="topic-title">{{ reply.topic_title || '原帖已删除' }}</h3>
            </div>
            <p class="topic-excerpt">{{ reply.content }}</p>
            <div class="topic-meta">
              <span>{{ formatRelativeTime(reply.created_at) }}</span>
            </div>
          </div>
        </div>
      </template>
      <el-empty v-else-if="!loading" description="暂无回复" />
    </div>

    <div v-else v-loading="loading" class="topic-list">
      <template v-if="topics.length > 0">
        <div
          v-for="topic in topics"
          :key="topic.id"
          class="topic-item"
          @click="goDetail(topic.id)"
        >
          <div class="topic-author">
            <el-avatar :size="42" :src="topic.author.avatar_url || undefined" class="author-avatar">
              {{ authorLetter(topic.author) }}
            </el-avatar>
          </div>
          <div class="topic-main">
            <div class="topic-title-row">
              <el-tag v-if="topic.chapter_id" size="small" type="warning" class="chapter-tag">
                {{ topic.chapter_title || '章节讨论' }}
              </el-tag>
              <h3 class="topic-title">{{ topic.title }}</h3>
            </div>
            <p class="topic-excerpt">{{ topic.content }}</p>
            <div class="topic-meta">
              <span class="author-name">{{ displayName(topic.author) }}</span>
              <span class="meta-divider">·</span>
              <span>{{ formatRelativeTime(topic.created_at) }}</span>
              <span class="meta-right">
                <span v-if="topic.images && topic.images.length > 0" class="img-mark">
                  <el-icon><Picture /></el-icon>
                  {{ topic.images.length }}
                </span>
                <el-icon><View /></el-icon>
                {{ topic.view_count }}
                <span class="like-mark">
                  <span class="heart" :class="{ liked: topic.liked_by_me }">{{ topic.liked_by_me ? '♥' : '♡' }}</span>
                  {{ topic.likes_count || 0 }}
                </span>
                <el-icon class="reply-icon"><ChatDotRound /></el-icon>
                {{ topic.reply_count }}
              </span>
            </div>
          </div>
        </div>
      </template>
      <el-empty v-else-if="!loading" description="还没有帖子，来发第一帖吧" />
    </div>

    <div class="pagination-wrapper" v-if="total > pageSize">
      <el-pagination
        v-model:current-page="currentPage"
        :page-size="pageSize"
        :total="total"
        layout="total, prev, pager, next"
        @current-change="loadTopics"
      />
    </div>

    <el-dialog v-model="createDialogVisible" title="发布新帖" width="640px">
      <el-form label-width="70px">
        <el-form-item label="标题" required>
          <el-input
            v-model="createForm.title"
            maxlength="100"
            show-word-limit
            placeholder="请输入标题（1-100 字）"
          />
        </el-form-item>
        <el-form-item label="内容" required>
          <el-input
            v-model="createForm.content"
            type="textarea"
            :rows="8"
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
        <el-button @click="createDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitTopic">发布</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { EditPen, View, ChatDotRound, Picture, Calendar } from '@element-plus/icons-vue'
import { forumApi, type ForumTopicItem, type MyReplyItem } from '@/api/forum'
import { formatRelativeTime } from '@/utils/format'
import ForumImageUploader from '@/components/student/ForumImageUploader.vue'
import CheckInDialog from '@/components/student/CheckInDialog.vue'

const router = useRouter()

const loading = ref(false)
const submitting = ref(false)
const topics = ref<ForumTopicItem[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(10)

// 列表模式：全部 / 我的帖子 / 我的回复（ADR-0018）
type ForumMode = 'all' | 'my-topics' | 'my-replies'
const mode = ref<ForumMode>('all')
const myReplies = ref<MyReplyItem[]>([])

function handleModeChange() {
  currentPage.value = 1
  loadTopics()
}
const createDialogVisible = ref(false)
const createForm = ref<{ title: string; content: string; images: string[] }>({ title: '', content: '', images: [] })

function displayName(author: ForumTopicItem['author']) {
  return author.username
}

function authorLetter(author: ForumTopicItem['author']) {
  return (displayName(author) || '?').charAt(0).toUpperCase()
}

async function loadTopics() {
  loading.value = true
  try {
    const params = { page: currentPage.value, page_size: pageSize.value }
    if (mode.value === 'my-replies') {
      const res = await forumApi.getMyReplies(params)
      myReplies.value = res.replies || []
      total.value = res.total || 0
    } else if (mode.value === 'my-topics') {
      const res = await forumApi.getMyTopics(params)
      topics.value = res.topics || []
      total.value = res.total || 0
    } else {
      const res = await forumApi.listTopics({ scope: 'general', ...params })
      topics.value = res.topics || []
      total.value = res.total || 0
    }
  } catch (e) {
    console.error('加载论坛列表失败:', e)
    /* 错误已由拦截器提示 */
  } finally {
    loading.value = false
  }
}

function openCreateDialog() {
  createForm.value = { title: '', content: '', images: [] }
  createDialogVisible.value = true
}

async function submitTopic() {
  const title = createForm.value.title.trim()
  const content = createForm.value.content.trim()
  if (!title || !content) {
    ElMessage.warning('请填写标题和内容')
    return
  }
  submitting.value = true
  try {
    await forumApi.createTopic({ chapter_id: null, title, content, images: createForm.value.images })
    ElMessage.success('发布成功')
    createDialogVisible.value = false
    currentPage.value = 1
    loadTopics()
  } catch (e) {
    console.error('发布失败:', e)
    /* 错误已由拦截器提示 */
  } finally {
    submitting.value = false
  }
}

function goDetail(id: number) {
  router.push({ name: 'ForumDetail', params: { topicId: String(id) } })
}

// ===== 打卡（spec #268 A1-A5）=====
const checkInStreak = ref(0)
const checkInTotal = ref(0)
const checkInTodayChecked = ref(false)
const checkInChecking = ref(false)
const checkInDialogVisible = ref(false)
const checkInTab = ref<'calendar' | 'rank'>('calendar')

async function loadCheckInStatus() {
  try {
    const now = new Date()
    const res = await forumApi.getCheckInCalendar({ year: now.getFullYear(), month: now.getMonth() + 1 })
    checkInStreak.value = res.streak
    checkInTotal.value = res.total
    checkInTodayChecked.value = res.today_checked
  } catch (e) {
    console.error('加载打卡状态失败:', e)
  }
}

function openCheckIn(tab: 'calendar' | 'rank') {
  checkInTab.value = tab
  checkInDialogVisible.value = true
}

async function doCheckIn() {
  if (checkInTodayChecked.value) return
  checkInChecking.value = true
  try {
    const res = await forumApi.checkIn()
    ElMessage.success(`打卡成功，已连续 ${res.streak} 天`)
    checkInStreak.value = res.streak
    checkInTotal.value = res.total
    checkInTodayChecked.value = res.today_checked
  } catch (e) {
    console.error('打卡失败:', e)
  } finally {
    checkInChecking.value = false
  }
}

function onCheckInChecked(data: { streak: number; total: number; today_checked: boolean }) {
  checkInStreak.value = data.streak
  checkInTotal.value = data.total
  checkInTodayChecked.value = data.today_checked
}

onMounted(() => {
  loadTopics()
  loadCheckInStatus()
})
</script>

<style scoped>
.forum-page {
  max-width: 960px;
  margin: 0 auto;
  padding: 0 16px 40px;
}

.forum-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 16px;
}

.forum-title {
  font-size: 24px;
  font-weight: 600;
  color: #303133;
  margin: 0 0 6px;
}

.forum-mode {
  margin-bottom: 12px;
}

.topic-list {
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
  min-height: 300px;
}

.topic-item {
  display: flex;
  gap: 14px;
  padding: 18px 20px;
  border-bottom: 1px solid #f0f0f0;
  cursor: pointer;
  transition: background 0.2s;
}

.topic-item:hover {
  background: #f7f9fc;
}

.topic-item:last-child {
  border-bottom: none;
}

.topic-main {
  flex: 1;
  min-width: 0;
}

.topic-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.topic-title {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.topic-excerpt {
  color: #606266;
  font-size: 13px;
  margin: 6px 0 8px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.topic-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: #909399;
}

.meta-divider {
  color: #dcdfe6;
}

.meta-right {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 4px;
}

.reply-icon {
  margin-left: 10px;
}

.img-mark {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  margin-right: 10px;
  color: #e6a23c;
}

.like-mark {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  margin-left: 10px;
  color: #f56c6c;
}

.like-mark .heart {
  font-size: 12px;
  line-height: 1;
}

.like-mark .heart.liked {
  color: #f56c6c;
}

.checkin-card {
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
  padding: 14px 16px;
  margin-bottom: 12px;
  cursor: pointer;
}

.checkin-left {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: #303133;
}

.checkin-icon {
  font-size: 16px;
  color: #409eff;
}

.checkin-sub {
  color: #909399;
  font-size: 12px;
  margin-left: 6px;
}

.checkin-right {
  display: flex;
  align-items: center;
  gap: 4px;
}

.pagination-wrapper {
  display: flex;
  justify-content: center;
  margin-top: 20px;
}

@media screen and (max-width: 768px) {
  .forum-header {
    flex-direction: column;
  }

  .topic-item {
    padding: 14px;
  }
}
</style>
