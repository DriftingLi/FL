<template>
  <div class="mx-auto max-w-[900px] px-4 pb-10">
    <div class="back-bar mb-3">
      <UiButton variant="text" :icon="ArrowLeft" @click="goBack">返回列表</UiButton>
    </div>

    <UiAsyncSection
      :error="loadError"
      :loading="loading"
      :empty="isEmpty"
      :retrying="retrying"
      error-title="帖子加载失败"
      error-description="网络或服务端异常，可重试"
      @retry="retryLoad"
    >
      <template #skeleton>
        <UiSkeleton variant="card" :count="1" />
        <UiSkeleton variant="list" :count="4" />
      </template>

      <template #empty>
        <UiEmptyState
          title="主题不存在"
          description="该帖子可能已被删除、驳回或作者撤回。"
          action-text="返回论坛"
          @action="goBack"
        />
      </template>

    <template v-if="topic">
      <div class="topic-card mb-4 rounded-card bg-panel p-5 shadow-card">
        <div class="topic-header flex items-center gap-3">
          <el-avatar :size="46" :src="topic.author.avatar_url || undefined">
            {{ authorLetter(topic.author) }}
          </el-avatar>
          <div class="topic-author-info flex flex-col gap-0.5">
            <span class="author-name text-sm font-semibold text-ink">{{ displayName(topic.author) }}</span>
            <!-- 属地（ADR-0045）：与作者名 / 时间同处一行；为空时整段不渲染 -->
            <span class="topic-time text-xs text-ink-3">
              {{ formatRelativeTime(topic.created_at)
              }}<template v-if="topicRegion"> · {{ topicRegion }}</template>
            </span>
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
          <!-- 正文按作者声明的格式渲染（ADR-0044）：纯文本分支的 whitespace-pre-wrap
               由 ForumContent 内部按需加，这里不再重复；字号/行高仍由本页决定。 -->
          <ForumContent
            :content="topic.content"
            :format="toForumContentFormat(topic.content_format)"
            class="topic-content text-[15px] leading-[1.8] text-ink"
          />
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
          <!-- 加载更多（ADR-0042 / ADR-0060 §4）：追加下一页而非替换，保持阅读连续；到底显示结束态。
               「还有没有下一批」由 useAsyncPage 的 append 档读服务端分页信封决定，页面不参与判断。
               深链 #reply-N 只由采纳通知生成（指向被采纳回复），而后端恒把被采纳回复放首页第一条，
               所以这里不需要「循环加载直到命中」的兜底。 -->
          <div v-if="hasMore" class="mt-3 flex justify-center">
            <UiButton :loading="loadingMore" @click="loadMore">
              加载更多回复<template v-if="remainingReplies !== null">（剩余 {{ remainingReplies }} 条）</template>
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
          @submit="(p) => submitReply(p)"
        />
      </div>
    </template>
    </UiAsyncSection>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, watch, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ArrowLeft, View, ChatDotRound, ArrowUp, ArrowDown } from '@element-plus/icons-vue'
import { forumApi, toForumContentFormat, type ForumTopicItem, type ForumReplyItem, type ForumContentFormat, type ForumTopicDetailData } from '@/api/forum'
import { useFavorite } from '@/composables/useFavorite'
import ForumImageGallery from '@/components/student/ForumImageGallery.vue'
import ForumComposer from '@/components/student/ForumComposer.vue'
import ForumReplyCard from '@/components/student/ForumReplyCard.vue'
import ForumContent from '@/components/student/ForumContent.vue'
import { formatRelativeTime } from '@/utils/format'
import { displayName, authorLetter, regionLabel } from '@/utils/forumDisplay'
// 空态判据的单点实现（#1101）：本页判据形状特殊（以 topic 为准，不以回复为准），
// 具名派生时复用这一份而不是抄表达式
import { isEmptyList } from '@/utils/listState'
import { useAuthStore } from '@/stores/auth'
import { useAsyncPage } from '@/composables/useAsyncPage'
import { useForumSort } from '@/composables/useForumSort'
import { useLike } from '@/composables/useLike'
import { useStagger } from '@/composables/useStagger'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiAsyncSection from '@/components/ui/UiAsyncSection.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiSegmentTabs from '@/components/ui/UiSegmentTabs.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiTag from '@/components/ui/UiTag.vue'
import UiActionChip from '@/components/ui/UiActionChip.vue'
import UiMoreMenu, { type UiMoreMenuItem } from '@/components/ui/UiMoreMenu.vue'
import { useConfirm } from '@/composables/useConfirm'
import { useForumReport } from '@/composables/useForumReport'

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

// ===== 回复分页（ADR-0042 / ADR-0060 §4 票 4）=====
// 回复列表的唯一读取形态是分页；被采纳回复由**后端**保证占首页第一条，前端不派生置顶
// （旧 sortedReplies 已删——分页后前端只拿得到已累积的几批，派生必然失效）。
// 本页唯一装载的就是这个回复窗口，故它整体住在 useAsyncPage 的 append 档里：
// 批大小就是发出去的 page_size，「还有没有下一批」由服务端的分页信封回答
// （getTopic 恒带 page/pages/total），页面不再自己按「本批满不满」猜。
// 手写的 replyPage / hasMore / loadingMore / replyTotal 与逐页 for 循环重载一并退役。
const REPLY_BATCH = 20

// 三态收编（#388）：loader 抛错即错误态。
// 票 5 配套（#1168）：判据区分「业务 404（主题不存在/已驳回）= 空态」与「其余错误 = 错误态 + retry」，
// 与 QuestionDetail 的 #1101 形态同源（复用 utils/listState 的 isEmptyList + ApiErrorKind）。
// 论坛不受证件过滤，不随切换重装（#604 opt-out）
const {
  loading,
  loadError,
  loadErrorKind,
  retrying,
  retry: retryLoad,
  run: loadFirstBatch,
  reset: reloadReplies,
  hasMore,
  loadingMore,
  loadMore,
  total
} = useAsyncPage(loadReplyBatch, {
  credentialScoped: false,
  mode: 'append',
  batchSize: REPLY_BATCH,
  itemsRef: replies,
  // 列表键是 replies（同响应里还有非列表的 topic），默认的 res.items 读不到
  pickItems: (res) => (res as ForumTopicDetailData).replies,
  // 排序双轴（维度 / 方向）即筛选轴：任一变化 → 清空累积 + 回第 1 批重装，
  // 页面不再手写「切排序要回第一页」
  filterDeps: [replySort, replyOrder]
})

/**
 * 空态判据：**主题对象没落地**才算空（业务 404 = 主题不存在/已驳回，或成功但无 topic）。
 * append 档的 `itemsRef` 是 replies，直接拿 composable 的 `isEmpty` 会把「零回复的帖子」
 * 渲染成「主题不存在」，故按 #1101 的口径具名派生、复用同一份 `isEmptyList` 实现。
 */
const isEmpty = computed(() =>
  isEmptyList(topic.value, { error: loadError.value, kind: loadErrorKind.value })
)

function handleReplySortChange() {
  // 热门默认逆序，最新默认正序；写完这条轴后重装由 filterDeps 单点负责
  replyOrder.value = replySort.value === 'hot' ? 'desc' : 'asc'
}

function toggleReplyOrder(){
  // 方向轴同在 filterDeps 里，翻转即重装（清空累积 + 回第 1 批）
  flipReplyOrder()
}

const isTopicOwner = computed(() => !!topic.value && topic.value.author.user_id === authStore.userInfo?.user_id)

/** 帖子作者的属地快照（ADR-0045）：空串 = 无属地，作者行那段整段不渲染 */
const topicRegion = computed(() => (topic.value ? regionLabel(topic.value) : ''))

/** 回复是否为当前登录用户（楼主自己）所发 —— 自己的回答不可采纳（ADR-0028） */
function isOwnReply(reply: ForumReplyItem) {
  return reply.author.user_id === authStore.userInfo?.user_id
}

// ===== 回复窗口的装载（append 档，见上方 REPLY_BATCH 处的说明）=====

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

/**
 * 尚未加载的回复条数（加载更多按钮上的剩余量）：以响应的 `total` 为准（与 pages 同源），
 * 不读 `topic.reply_count` —— 后者是列表页消费的反范式列，两者漂移会让文案与翻页自相矛盾。
 * `total` 尚未落地（响应没给）时返回 null：宁可不显示数字，也不显示一个凭累积数出来的 0。
 */
const remainingReplies = computed(() =>
  total.value > 0 ? Math.max(total.value - replies.value.length, 0) : null
)

function scrollToHash() {
  const hash = route.hash || window.location.hash
  if (!hash || !hash.startsWith('#reply-')) return
  const id = hash.slice(1)
  nextTick(() => {
    const el = document.getElementById(id)
    if (el) el.scrollIntoView({ behavior: 'smooth', block: 'center' })
  })
}

/**
 * 装载一批回复（append 档的唯一 loader）：页码推进与累积由 composable 负责，
 * 这里只取「第 page 批」，并把同响应里的**非列表部分**（topic）落地。
 * 浏览记录走服务端（#701：详情访问即由后端 GetTopic 落浏览去重行），不再写本地 localStorage。
 */
async function loadReplyBatch(page = 1) {
  const res = await forumApi.getTopic(
    Number(route.params.topicId), replySort.value, replyOrder.value, page, REPLY_BATCH
  )
  topic.value = res.topic
  // 深链 #reply-N 只由采纳通知生成（指向被采纳回复），而后端恒把它放首页第一条，
  // 故只在首页装载后滚动一次——加载更多/切排序不该把用户拽回置顶条。
  if (page === 1) {
    await nextTick()
    scrollToHash()
  }
  return res
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
    if (updated) topic.value = { ...topic.value, ...updated } as ForumTopicItem
    // 必须重载而不是本地翻 is_accepted：置顶是**后端事实**（ADR-0042），
    // 本地翻标记会让「已采纳」停在原位，与「恒占首页第一条」自相矛盾。
    // append 档的「刷新」入口是 reset：清空累积 + 回第 1 批（置顶换了人，累积窗口整体作废）。
    await reloadReplies()
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
    if (updated) topic.value = { ...topic.value, ...updated } as ForumTopicItem
    // 同 handleAccept：取消后该条回到自然排序位置，只能由后端重排
    await reloadReplies()
  } catch (e) {
    console.error('取消采纳失败:', e)
  }
}

async function submitReply(payload: { contentFormat: ForumContentFormat }) {
  const content = replyContent.value.trim()
  if (!content && replyImages.value.length === 0) {
    ElMessage.warning('请输入回复内容')
    return
  }
  submitting.value = true
  try {
    const topicId = Number(route.params.topicId)
    await forumApi.replyTopic(topicId, content, replyingTo.value?.id, replyImages.value, payload.contentFormat)
    ElMessage.success('回复成功')
    replyContent.value = ''
    replyImages.value = []
    replyingTo.value = null
    // 新回复要落进窗口：清空累积 + 回第 1 批（append 档的刷新入口）
    await reloadReplies()
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
  } catch (e) {
    console.error('删除失败:', e)
    /* 错误已由拦截器提示 */
    return
  }
  // 删除已成功，随后的刷新**不能**报成「删除失败」——append 档自己把刷新失败收敛成
  // 可重试的错误态（run 的 catch → loadError），页面这里不再手写 try/catch 兜底。
  // 删父回复会**级联删掉整棵楼中楼**（后端按子树大小减计数），本地 filter 一条是错的，
  // 必须回到服务端重取：清空累积 + 回第 1 批。
  // ⚠️ 语义变化（ADR-0060 §4）：旧实现有个逐页 for 循环，按「已加载的页数」重载以保住阅读
  // 位置；append 档不表达「保住 N 批」（为其加参数只服务一个调用方不值当），故删完回到
  // 第 1 批，长帖需要再点「加载更多」走回原位。
  await reloadReplies()
}

// ===== 帖子卡的 ⋯ 菜单（互动下沉后，治理动作的唯一入口）=====
const topicMoreItems = computed<UiMoreMenuItem[]>(() => {
  const items: UiMoreMenuItem[] = []
  // 与回复卡同规则：自己的内容不显示「举报」（后端无自举报拦截，这是前端入口收敛）
  if (!isTopicOwner.value) items.push({ key: 'report', label: '举报' })
  if (topic.value?.can_delete) items.push({ key: 'delete', label: '删除', tone: 'danger' })
  return items
})

function onTopicMoreSelect(key: string) {
  if (key === 'report') openReport('topic', Number(route.params.topicId))
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

// 收藏帖子：「查询—切换—提示—失败保持原态」的状态机在 useFavorite（ADR-0060 决策 3），
// 页面只留「何时查」这一本地事实（onMounted 一次）。种类判据（target_type='topic'）
// 由内容对象表给出，不再在本文件硬写。
const {
  favorited: topicFavorited,
  load: loadFavoriteState,
  toggle: toggleFavorite
} = useFavorite('topic', () => Number(route.params.topicId))

async function toggleTopicLike() {
  if (!topic.value) return
  await toggleTopicLikeOnce(topic.value)
}

async function toggleReplyLike(reply: ForumReplyItem) {
  await toggleReplyLikeOnce(reply)
}

// 举报（帖子/回复共用对话框）：状态机与提交口径收在 composable 一处，与章节讨论共用。
const {
  visible: reportVisible,
  reason: reportReason,
  submitting: reportSubmitting,
  open: openReport,
  submit: submitReport
} = useForumReport()

watch(
  () => route.hash,
  () => scrollToHash()
)

onMounted(() => {
  loadFirstBatch()
  loadFavoriteState()
})
</script>
