<template>
  <div class="mx-auto max-w-[900px] px-4 pb-10">
    <div class="back-bar mb-3">
      <UiButton variant="text" :icon="ArrowLeft" @click="goBack">返回列表</UiButton>
    </div>

    <UiErrorState
      v-if="loadError"
      title="帖子加载失败"
      description="网络或服务端异常，可重试"
      :retrying="retrying"
      @retry="retryLoad"
    />

    <template v-else-if="loading">
      <UiSkeleton variant="card" :count="1" />
      <UiSkeleton variant="list" :count="4" />
    </template>

    <template v-else-if="topic">
      <div class="topic-card mb-4 rounded-card bg-panel p-5 shadow-card">
        <div class="topic-header flex items-center gap-3">
          <el-avatar :size="46" :src="topic.author.avatar_url || undefined">
            {{ authorLetter(topic.author) }}
          </el-avatar>
          <div class="topic-author-info flex flex-col gap-0.5">
            <span class="author-name text-sm font-semibold text-ink">{{ displayName(topic.author) }}</span>
            <span class="topic-time text-xs text-ink-3">{{ formatRelativeTime(topic.created_at) }}</span>
          </div>
          <!-- 治理动作（举报 / 删除）收进 ⋯；互动动作（点赞 / 收藏）下沉到正文下方的操作行 -->
          <UiMoreMenu class="ml-auto" :items="topicMoreItems" @select="onTopicMoreSelect" />
        </div>
        <div class="topic-body mt-4">
          <div class="topic-title-row mb-3 flex flex-wrap items-center gap-2">
            <UiTag v-if="topic.category === 'question'" tone="success">问答</UiTag>
            <UiTag v-else-if="topic.chapter_id" tone="warning">
              {{ topic.chapter_title || '章节讨论' }}
            </UiTag>
            <UiTag v-else tone="neutral">综合</UiTag>
            <UiTag v-if="topic.category === 'question' && (topic.accepted_reply_id || topic.solved_at)" tone="success" effect="dark">✓ 已解决</UiTag>
            <UiTag v-else-if="topic.category === 'question'" tone="neutral" effect="plain">求助</UiTag>
            <h1 class="topic-title m-0 text-xl font-semibold text-ink">{{ topic.title }}</h1>
          </div>
          <div v-if="topic.category === 'question' && isTopicOwner && topic.accepted_reply_id" class="accept-actions mb-3">
            <UiButton size="small" @click="handleCancelAccept">取消采纳</UiButton>
          </div>
          <div class="topic-content whitespace-pre-wrap break-words text-[15px] leading-[1.8] text-ink">{{ topic.content }}</div>
          <ForumImageGallery :images="topic.images" />
          <!-- 帖子操作行：左统计、右互动。浏览/回复数取自列（与详情分页 total 同源），点赞数由操作 chip 承载，不重复渲染。 -->
          <div class="topic-stats mt-4 flex flex-wrap items-center gap-3 text-[13px] text-ink-3">
            <span class="inline-flex items-center gap-1">
              <el-icon><View /></el-icon>{{ topic.view_count }} 次浏览
            </span>
            <span class="inline-flex items-center gap-1">
              <el-icon><ChatDotRound /></el-icon>{{ topic.reply_count }} 条回复
            </span>
            <div class="topic-actions ml-auto flex items-center gap-3">
              <UiActionChip
                icon="like"
                :label="topic.liked_by_me ? '已赞' : '点赞'"
                :count="topic.likes_count"
                tone="like"
                borderless
                :active="!!topic.liked_by_me"
                @click="toggleTopicLike"
              />
              <UiActionChip
                icon="fav"
                :label="topicFavorited ? '已收藏' : '收藏'"
                tone="fav"
                borderless
                :active="topicFavorited"
                @click="toggleFavorite"
              />
            </div>
          </div>
        </div>
      </div>

      <div class="replies-card mb-4 rounded-card bg-panel p-5 shadow-card">
        <div class="replies-header mb-2 flex items-center justify-between">
          <h3 class="replies-title m-0 text-base font-semibold text-ink">全部回复（{{ replies.length }}）</h3>
          <div class="flex items-center gap-2">
            <UiSegmentTabs
              :model-value="replySort"
              :options="[
                { label: '最新', value: 'latest' },
                { label: '热门', value: 'hot' }
              ]"
              @update:model-value="(v: string) => { replySort = v as 'latest' | 'hot'; handleReplySortChange() }"
            />
            <UiButton size="small" :icon="replyOrder==='asc'? ArrowUp : ArrowDown" @click="toggleReplyOrder">{{ replyOrder==='asc' ? '正序' : '逆序' }}</UiButton>
          </div>
        </div>
        <template v-if="replies.length > 0">
          <!-- 渲染顺序即后端顺序：被采纳回复由后端保证占首页第一条（ADR-0042），
               前端不再派生置顶（旧 sortedReplies 已删）——分页后前端只拿得到一页，
               派生置顶必然失效。 -->
          <ForumReplyCard
            v-for="(reply, i) in replies"
            :key="reply.id"
            :id="`reply-${reply.id}`"
            class="stagger-in border-b border-line last:border-b-0"
            :style="staggerStyle(i)"
            :reply="reply"
            :topic-author-id="topic.author.user_id"
            :is-own="isOwnReply(reply)"
            :can-accept="canAcceptReply(reply)"
            :can-cancel-accept="canCancelAcceptReply(reply)"
            @reply-to="startReplyTo"
            @like="toggleReplyLike"
            @report="(r) => openReport('reply', r.id)"
            @delete="removeReply"
            @accept="handleAccept"
            @cancel-accept="handleCancelAccept"
          />
          <!-- 加载更多（ADR-0042）：追加下一页而非替换，保持阅读连续；到底显示结束态。
               深链 #reply-N 只由采纳通知生成（指向被采纳回复），而后端恒把被采纳回复放首页第一条，
               所以这里不需要「循环加载直到命中」的兜底。 -->
          <div v-if="hasMore" class="mt-3 flex justify-center">
            <UiButton :loading="loadingMore" @click="loadMore">
              加载更多回复（剩余 {{ remainingReplies }} 条）
            </UiButton>
          </div>
          <p v-else class="mt-3 mb-0 text-center text-xs text-ink-3">没有更多回复了</p>
        </template>
        <UiEmptyState v-else description="暂无回复，来说两句吧" />
      </div>

      <!-- 举报对话框（帖子/回复共用，ADR-0018） -->
      <UiDialog
        v-model="reportVisible"
        title="举报"
        width="440px"
        confirm-text="提交"
        :confirm-loading="reportSubmitting"
        @confirm="submitReport"
      >
        <UiInput
          v-model="reportReason"
          type="textarea"
          :rows="4"
          :maxlength="500"
          show-word-limit
          placeholder="请填写举报理由（1-500 字）"
        />
      </UiDialog>

      <div class="reply-editor mb-4">
        <ForumComposer
          v-model="replyContent"
          v-model:images="replyImages"
          v-model:replying-to="replyingTo"
          :submitting="submitting"
          :max-images="3"
          placeholder="写下你的回复…"
          @submit="submitReply"
        />
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, watch, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ArrowLeft, View, ChatDotRound, ArrowUp, ArrowDown } from '@element-plus/icons-vue'
import { forumApi, type ForumTopicItem, type ForumReplyItem } from '@/api/forum'
import { favoriteApi } from '@/api/favorite'
import ForumImageGallery from '@/components/student/ForumImageGallery.vue'
import ForumComposer from '@/components/student/ForumComposer.vue'
import ForumReplyCard from '@/components/student/ForumReplyCard.vue'
import { formatRelativeTime } from '@/utils/format'
import { displayName, authorLetter } from '@/utils/forumDisplay'
import { useAuthStore } from '@/stores/auth'
import { useAsyncPage } from '@/composables/useAsyncPage'
import { useForumSort } from '@/composables/useForumSort'
import { useLike } from '@/composables/useLike'
import { useStagger } from '@/composables/useStagger'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiSegmentTabs from '@/components/ui/UiSegmentTabs.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiTag from '@/components/ui/UiTag.vue'
import UiActionChip from '@/components/ui/UiActionChip.vue'
import UiMoreMenu, { type UiMoreMenuItem } from '@/components/ui/UiMoreMenu.vue'
import { useConfirm } from '@/composables/useConfirm'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

const submitting = ref(false)

const staggerStyle = useStagger()
const topic = ref<ForumTopicItem | null>(null)
const replies = ref<ForumReplyItem[]>([])
const replyContent = ref('')
const replyingTo = ref<{ id: number; username: string } | null>(null)
// 已上传图片 URL 由 ForumComposer 内部经 useForumImageUpload 维护，这里只持有结果
const replyImages = ref<string[]>([])

// 排序双轴收编（#389）：详情回复口径为「热门逆序、最新正序」，切维度时按此映射
const { sort: replySort, order: replyOrder, flipOrder: flipReplyOrder } = useForumSort('asc')

// 三态收编（#388，详情页无分页）：loader 抛错即错误态
// 论坛不受证件过滤，不随切换重装（#604 opt-out）
const { loading, loadError, retrying, retry: retryLoad, run: loadDetail } = useAsyncPage(loadDetailOnce, { credentialScoped: false })

function handleReplySortChange() {
  // 热门默认逆序，最新默认正序
  replyOrder.value = replySort.value === 'hot' ? 'desc' : 'asc'
  loadDetail()
}

function toggleReplyOrder(){
  flipReplyOrder()
  loadDetail()
}

const isTopicOwner = computed(() => !!topic.value && topic.value.author.user_id === authStore.userInfo?.user_id)

/** 回复是否为当前登录用户（楼主自己）所发 —— 自己的回答不可采纳（ADR-0028） */
function isOwnReply(reply: ForumReplyItem) {
  return reply.author.user_id === authStore.userInfo?.user_id
}

// ===== 回复分页（ADR-0042）=====
// 回复列表的唯一读取形态是分页；被采纳回复由**后端**保证占首页第一条，
// 前端不再派生置顶（旧 sortedReplies 已删）——分页后前端只拿得到一页，派生必然失效。
const REPLY_PAGE_SIZE = 20
const replyPage = ref(1)
const hasMore = ref(false)
const loadingMore = ref(false)

/** 楼主视角：这条可被采纳（问答帖 + 非本人作答 + 尚未采纳） */
function canAcceptReply(reply: ForumReplyItem) {
  return (
    !!topic.value &&
    topic.value.category === 'question' &&
    isTopicOwner.value &&
    !reply.is_accepted &&
    !isOwnReply(reply)
  )
}

/** 楼主视角：这条已被采纳，可取消（取消采纳不改类别，逃生口在楼主手上） */
function canCancelAcceptReply(reply: ForumReplyItem) {
  return !!topic.value && topic.value.category === 'question' && isTopicOwner.value && !!reply.is_accepted
}

/** 尚未加载的回复条数（加载更多按钮上的剩余量） */
const remainingReplies = computed(() => {
  const total = topic.value?.reply_count ?? replies.value.length
  return Math.max(total - replies.value.length, 0)
})

function scrollToHash() {
  const hash = route.hash || window.location.hash
  if (!hash || !hash.startsWith('#reply-')) return
  const id = hash.slice(1)
  nextTick(() => {
    const el = document.getElementById(id)
    if (el) el.scrollIntoView({ behavior: 'smooth', block: 'center' })
  })
}

async function loadDetailOnce() {
  const topicId = Number(route.params.topicId)
  // 首次加载与「切排序 / 重试」都回到第 1 页并**替换**列表（分页语义：不是追加）
  replyPage.value = 1
  const res = await forumApi.getTopic(topicId, replySort.value, replyOrder.value, 1, REPLY_PAGE_SIZE)
  topic.value = res.topic
  replies.value = res.replies || []
  hasMore.value = (res.page ?? 1) < (res.pages ?? 1)
  // 浏览记录走服务端（#701：详情访问即由后端 GetTopic 落浏览去重行），不再写本地 localStorage
  await nextTick()
  scrollToHash()
}

/** 加载更多：**追加**下一页（不替换），保持阅读连续；到底后入口消失。 */
async function loadMore() {
  if (loadingMore.value || !hasMore.value) return
  loadingMore.value = true
  try {
    const next = replyPage.value + 1
    const res = await forumApi.getTopic(
      Number(route.params.topicId), replySort.value, replyOrder.value, next, REPLY_PAGE_SIZE
    )
    replies.value = [...replies.value, ...(res.replies || [])]
    replyPage.value = res.page ?? next
    hasMore.value = replyPage.value < (res.pages ?? replyPage.value)
  } catch (e) {
    console.error('加载更多回复失败:', e)
    /* 错误已由拦截器提示 */
  } finally {
    loadingMore.value = false
  }
}

async function handleAccept(replyId: number) {
  if (!topic.value) return
  const isReplace = !!topic.value.accepted_reply_id
  const alreadyIssued = !!topic.value.reward_issued || isReplace
  const msg = alreadyIssued
    ? '该帖采纳奖励已发放，更换只会改变显示，不再产生积分。确认更换采纳？'
    : '确认采纳？+40 分将发放给该答主'
  const title = alreadyIssued ? '更换采纳' : '采纳回答'
  try {
    await useConfirm().confirm(msg, title, { type: alreadyIssued ? 'warning' : 'info', confirmButtonText: '确认', cancelButtonText: '取消' })
  } catch {
    return
  }
  try {
    const updated = await forumApi.acceptReply(topic.value.id, replyId)
    ElMessage.success('已采纳')
    if (updated) {
      topic.value = { ...topic.value, ...updated } as ForumTopicItem
      const newAccepted = (updated as ForumTopicItem).accepted_reply_id
      replies.value = replies.value.map((r) => ({ ...r, is_accepted: newAccepted != null && r.id === newAccepted }))
    } else {
      await loadDetail()
    }
  } catch (e) {
    console.error('采纳失败:', e)
  }
}

async function handleCancelAccept() {
  if (!topic.value) return
  try {
    await useConfirm().confirm('确认取消采纳？已发放积分不会收回。', '取消采纳', { type: 'warning' })
  } catch {
    return
  }
  try {
    const updated = await forumApi.cancelAccept(topic.value.id)
    ElMessage.success('已取消采纳')
    if (updated) {
      topic.value = { ...topic.value, ...updated } as ForumTopicItem
      replies.value = replies.value.map((r) => ({ ...r, is_accepted: false }))
    } else {
      await loadDetail()
    }
  } catch (e) {
    console.error('取消采纳失败:', e)
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

function startReplyTo(reply: ForumReplyItem) {
  replyingTo.value = { id: reply.id, username: displayName(reply.author) }
}

async function removeTopic() {
  const isSolved = topic.value?.accepted_reply_id != null
  const msg = isSolved
    ? '该帖已解决且已被采纳，删除后已采纳的答案将一并被删除，且计数将计入巡检，是否确认删除？'
    : '确定删除这个帖子吗？删除后无法恢复。'
  try {
    await useConfirm().confirmDanger(msg, '删除帖子', { type: 'warning' })
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
    await useConfirm().confirmDanger('确定删除这条回复吗？', '删除回复', { type: 'warning' })
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

// ===== 帖子卡的 ⋯ 菜单（互动下沉后，治理动作的唯一入口）=====
const topicMoreItems = computed<UiMoreMenuItem[]>(() => {
  const items: UiMoreMenuItem[] = [{ key: 'report', label: '举报' }]
  if (topic.value?.can_delete) items.push({ key: 'delete', label: '删除', tone: 'danger' })
  return items
})

function onTopicMoreSelect(key: string) {
  if (key === 'report') openReport('topic')
  else if (key === 'delete') removeTopic()
}

function goBack() {
  if (window.history.length > 1) {
    router.back()
  } else {
    router.push({ name: 'ForumPage' })
  }
}

// ===== 互动（ADR-0018）：收藏 / 点赞 / 举报 =====

// 点赞乐观更新 + 失败回滚（#389 单点）：帖与回复各注入一组端点，时序同源
const { toggle: toggleTopicLikeOnce } = useLike(forumApi.likeTopic, forumApi.unlikeTopic)
const { toggle: toggleReplyLikeOnce } = useLike(forumApi.likeReply, forumApi.unlikeReply)

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
  await toggleTopicLikeOnce(topic.value)
}

async function toggleReplyLike(reply: ForumReplyItem) {
  await toggleReplyLikeOnce(reply)
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

watch(
  () => route.hash,
  () => scrollToHash()
)

onMounted(() => {
  loadDetail()
  loadFavoriteState()
})
</script>
