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
            <span class="topic-time text-xs text-ink-3">{{ formatLocaleDateTime(topic.created_at, '') }}</span>
          </div>
          <div class="topic-actions ml-auto flex items-center gap-1">
            <!-- #511：UiActionChip 统一互动/治理操作（图标内置、激活态填充） -->
            <UiActionChip icon="like" :label="topic?.liked_by_me ? '已赞' : '点赞'" :count="topic.likes_count" tone="like" :active="!!topic?.liked_by_me" @click="toggleTopicLike" />
            <UiActionChip icon="fav" :label="topicFavorited ? '已收藏' : '收藏'" tone="fav" :active="topicFavorited" @click="toggleFavorite" />
            <UiActionChip icon="report" label="举报" tone="neutral" @click="openReport('topic')" />
            <UiActionChip v-if="topic.can_delete" icon="delete" label="删除" tone="danger" @click="removeTopic" />
          </div>
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
          <div class="topic-stats mt-4 flex items-center gap-1.5 text-[13px] text-ink-3">
            <el-icon><View /></el-icon>
            {{ topic.view_count }} 次浏览
            <span class="like-stat ml-3 inline-flex items-center gap-0.5 text-bad">
              <svg class="size-3.5" viewBox="0 0 24 24" :fill="topic.liked_by_me ? 'currentColor' : 'none'" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z"/></svg>
              {{ topic.likes_count || 0 }} 点赞
            </span>
            <el-icon class="reply-icon ml-3"><ChatDotRound /></el-icon>
            {{ topic.reply_count }} 条回复
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
        <template v-if="sortedReplies.length > 0">
          <div
            v-for="(reply, i) in sortedReplies"
            :key="reply.id"
            :id="`reply-${reply.id}`"
            class="reply-item stagger-in flex gap-3 border-b border-line py-4 last:border-b-0"
            :class="
              reply.is_accepted
                ? 'is-accepted relative my-1.5 -mx-2 rounded-[8px] bg-ok-soft p-3 pl-4 before:absolute before:inset-y-0 before:left-0 before:w-[3px] before:rounded-full before:bg-ok'
                : ''
            "
            :style="staggerStyle(i)"
          >
            <el-avatar :size="38" :src="reply.author.avatar_url || undefined">
              {{ authorLetter(reply.author) }}
            </el-avatar>
            <div class="reply-main min-w-0 flex-1">
              <div class="reply-meta mb-1.5 flex flex-wrap items-center gap-2">
                <span class="author-name text-sm font-semibold text-ink">{{ displayName(reply.author) }}</span>
                <UiTag v-if="topic && reply.author.user_id === topic.author.user_id" tone="neutral" effect="plain" class="ml-1.5">楼主</UiTag>
                <UiTag v-if="reply.is_accepted" tone="success" effect="dark" class="ml-1.5">✓ 已采纳</UiTag>
                <span class="reply-time text-xs text-ink-3">{{ formatLocaleDateTime(reply.created_at, '') }}</span>
                <UiButton variant="primary" class="reply-btn" size="small" @click="startReplyTo(reply)">
                  回复
                </UiButton>
                <!-- #511：回复互动/治理统一药丸形态 -->
                <UiActionChip icon="like" :label="reply.liked_by_me ? '已赞' : '点赞'" :count="reply.likes_count" tone="like" compact :active="!!reply.liked_by_me" @click="toggleReplyLike(reply)" />
                <UiActionChip icon="report" label="举报" tone="neutral" compact @click="openReport('reply', reply.id)" />
                <UiActionChip v-if="reply.can_delete" icon="delete" label="删除" tone="danger" compact class="ml-auto" @click="removeReply(reply.id)" />
              </div>
              <div v-if="reply.parent_id && reply.parent_name" class="reply-quote mb-1 inline-block rounded-[6px] bg-canvas px-2 py-0.5 text-xs text-ink-3">
                回复 @{{ reply.parent_name }}
              </div>
              <div class="reply-content whitespace-pre-wrap break-words text-sm leading-[1.7] text-ink">{{ reply.content }}</div>
              <ForumImageGallery :images="reply.images" />
              <div v-if="topic && topic.category === 'question' && isTopicOwner && !reply.is_accepted && !isOwnReply(reply)" class="reply-accept-row mt-2">
                <UiButton variant="success" plain size="small" @click="handleAccept(reply.id)">采纳此回答</UiButton>
              </div>
              <div v-else-if="topic && topic.category === 'question' && isTopicOwner && reply.is_accepted" class="reply-accept-row mt-2">
                <UiButton size="small" @click="handleCancelAccept">取消采纳</UiButton>
              </div>
            </div>
          </div>
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
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowLeft, View, ChatDotRound, ArrowUp, ArrowDown } from '@element-plus/icons-vue'
import { forumApi, type ForumTopicItem, type ForumReplyItem } from '@/api/forum'
import { favoriteApi } from '@/api/favorite'
import ForumImageGallery from '@/components/student/ForumImageGallery.vue'
import ForumComposer from '@/components/student/ForumComposer.vue'
import { formatLocaleDateTime } from '@/utils/format'
import { displayName, authorLetter } from '@/utils/forumDisplay'
import { pushHistory, toHistoryItem } from '@/utils/forumHistory'
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
const { loading, loadError, retrying, retry: retryLoad, run: loadDetail } = useAsyncPage(loadDetailOnce)

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

const sortedReplies = computed(() => {
  if (!replies.value.length) return []
  const idx = replies.value.findIndex((r) => r.is_accepted)
  if (idx <= 0) return replies.value
  const copy = [...replies.value]
  const [acc] = copy.splice(idx, 1)
  copy.unshift(acc)
  return copy
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
  await nextTick()
  scrollToHash()
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
    await ElMessageBox.confirm(msg, title, { type: alreadyIssued ? 'warning' : 'info', confirmButtonText: '确认', cancelButtonText: '取消' })
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
    await ElMessageBox.confirm('确认取消采纳？已发放积分不会收回。', '取消采纳', { type: 'warning' })
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
    await ElMessageBox.confirm(msg, '删除帖子', { type: 'warning' })
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
