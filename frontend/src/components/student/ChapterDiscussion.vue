<script setup lang="ts">
/**
 * 章节讨论（章节页内嵌）
 *
 * 改造记录（2026-09-02）：原来 495 行里约 200 行 scoped CSS 全部迁移为原子类，
 * 老变量（--color-text-primary 等）换成语义 token（ink/canvas/panel/line），
 * 发帖表单与回复输入改为复用 ForumPostForm / ForumComposer，
 * 列表补齐四段式（错误态+retry → 骨架 → 内容 → 空态）。
 *
 * 状态机：章节列表与展开详情是两级加载 —— 列表轻（只拉标题），详情按需拉取。
 */
import { ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { EditPen, ArrowDown, ArrowUp } from '@element-plus/icons-vue'
import { forumApi, type ForumTopicItem, type ForumReplyItem } from '@/api/forum'
import ForumImageGallery from '@/components/student/ForumImageGallery.vue'
import ForumPostForm from '@/components/student/ForumPostForm.vue'
import ForumComposer from '@/components/student/ForumComposer.vue'
import ForumReplyCard from '@/components/student/ForumReplyCard.vue'
import { formatRelativeTime } from '@/utils/format'
import { displayName, authorLetter } from '@/utils/forumDisplay'
import { useAuthStore } from '@/stores/auth'
import { useAsyncPage } from '@/composables/useAsyncPage'
import { useLike } from '@/composables/useLike'
import UiButton from '@/components/ui/UiButton.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import { useConfirm } from '@/composables/useConfirm'

const authStore = useAuthStore()

const props = defineProps<{
  chapterId: number
}>()

const detailLoading = ref(false)
const replying = ref(false)
const topics = ref<ForumTopicItem[]>([])
const expandedTopicId = ref<number | null>(null)
const expandedTopic = ref<ForumTopicItem | null>(null)
const detailContent = ref('')
const replies = ref<ForumReplyItem[]>([])
const replyContent = ref('')
const replyImages = ref<string[]>([])
const replyingTo = ref<{ id: number; username: string } | null>(null)

// ===== 发帖对话框（复用 ForumPostForm，与论坛页同一套表单）=====
const createVisible = ref(false)
const postForm = ref<InstanceType<typeof ForumPostForm> | null>(null)

// ===== 列表三态（四段式的骨架/错误/内容/空态由此驱动）=====
async function loadTopicsOnce() {
  if (!props.chapterId) return
  const res = await forumApi.listTopics({
    scope: 'chapter',
    chapter_id: props.chapterId,
    page: 1,
    page_size: 50
  })
  topics.value = res.topics || []
}

const {
  loading: listLoading,
  loadError,
  retrying,
  retry: retryLoad,
  run: loadTopics
} = useAsyncPage(loadTopicsOnce, { credentialScoped: false }) // 论坛不受证件过滤（#604 opt-out）

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
    // ADR-0042：详情回复一律分页读取。章节讨论是内嵌预览面板、没有翻页交互，
    // 故取页大小上限（100）以保持既有「展开即看全」的体验；超出 100 条的章节帖罕见，
    // 真出现也只影响该帖的尾部回复（论坛详情页仍是完整的分页读取）。
    const res = await forumApi.getTopic(topicId, undefined, undefined, 1, 100)
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
  postForm.value?.reset()
  createVisible.value = true
}

async function onTopicCreated() {
  createVisible.value = false
  await loadTopics()
}

function startReplyTo(reply: ForumReplyItem) {
  replyingTo.value = { id: reply.id, username: displayName(reply.author) }
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
    await useConfirm().confirmDanger('确定删除这个帖子吗？删除后无法恢复。', '删除帖子', { type: 'warning' })
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

// ===== 回复互动（#858：与帖子详情同口径——点赞乐观更新、举报走同一对话框形态）=====
const { toggle: toggleReplyLikeOnce } = useLike(forumApi.likeReply, forumApi.unlikeReply)

async function toggleReplyLike(reply: ForumReplyItem) {
  await toggleReplyLikeOnce(reply)
}

const reportVisible = ref(false)
const reportReason = ref('')
const reportSubmitting = ref(false)
const reportTarget = ref<{ kind: 'topic' | 'reply'; id: number } | null>(null)

function openReport(kind: 'topic' | 'reply', replyId?: number) {
  if (!expandedTopicId.value) return
  reportTarget.value = { kind, id: kind === 'topic' ? expandedTopicId.value : replyId! }
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
    if (kind === 'topic') await forumApi.reportTopic(id, reason)
    else await forumApi.reportReply(id, reason)
    ElMessage.success('举报已提交，等待处理')
    reportVisible.value = false
  } catch (e) {
    console.error('举报失败:', e)
    /* 错误已由拦截器提示 */
  } finally {
    reportSubmitting.value = false
  }
}

/** 是否为当前登录用户自己发的（决定 ⋯ 里是否出现「举报」） */
function isOwnReply(reply: ForumReplyItem) {
  return reply.author.user_id === authStore.userInfo?.user_id
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

<template>
  <div class="rounded-card border border-line bg-panel p-5 shadow-card max-md:p-3.5">
    <!-- 头部 -->
    <div class="mb-4 flex items-center justify-between gap-3">
      <h3 class="m-0 text-[17px] font-semibold text-ink">章节讨论</h3>
      <UiButton variant="primary" size="small" :icon="EditPen" @click="openCreate">发新帖</UiButton>
    </div>

    <!-- 列表四段式：错误态 → 骨架 → 内容 → 空态 -->
    <UiErrorState
      v-if="loadError"
      title="讨论加载失败"
      description="网络或服务端异常，可重试"
      :retrying="retrying"
      @retry="retryLoad"
    />

    <UiSkeleton v-else-if="listLoading" variant="list" :count="3" />

    <template v-else-if="topics.length > 0">
      <div
        v-for="topic in topics"
        :key="topic.id"
        class="mb-3 overflow-hidden rounded-[10px] border border-line last:mb-0"
      >
        <div
          class="cursor-pointer px-3.5 py-3 transition-colors duration-[var(--duration-tap)] ease-[var(--ease-default)] hover:bg-canvas"
          @click="toggleTopic(topic.id)"
        >
          <div class="flex items-center justify-between gap-2.5">
            <h4 class="m-0 truncate text-sm font-semibold text-ink">{{ topic.title }}</h4>
            <span class="shrink-0 text-xs text-ink-3">{{ topic.reply_count }} 回复</span>
          </div>
          <div class="mt-2 flex items-center gap-2 text-xs text-ink-3">
            <el-avatar :size="24" :src="topic.author.avatar_url || undefined">
              {{ authorLetter(topic.author) }}
            </el-avatar>
            <span class="text-ink-2">{{ displayName(topic.author) }}</span>
            <span>{{ formatRelativeTime(topic.created_at) }}</span>
            <el-icon class="ml-auto">
              <ArrowUp v-if="expandedTopicId === topic.id" />
              <ArrowDown v-else />
            </el-icon>
          </div>
        </div>

        <!-- 展开详情：两级加载，按需拉取 -->
        <div v-if="expandedTopicId === topic.id" class="border-t border-line bg-canvas p-3.5 max-md:p-2.5">
          <UiSkeleton v-if="detailLoading" variant="list" :count="2" />

          <template v-else>
            <div class="mb-3.5 whitespace-pre-wrap break-words text-sm leading-[1.7] text-ink">
              {{ detailContent }}
            </div>
            <ForumImageGallery :images="expandedTopic?.images" />

            <!-- 回复流 -->
            <div class="rounded-[8px] bg-panel px-3 py-1 max-md:px-2">
              <template v-if="replies.length > 0">
                <!-- 与帖子详情共用同一份回复渲染（含点赞 / 举报 / ⋯）；
                     密度取 compact：内嵌面板不套详情页尺寸，避免撑长课程页展开区。 -->
                <ForumReplyCard
                  v-for="reply in replies"
                  :key="reply.id"
                  class="border-b border-line last:border-b-0"
                  density="compact"
                  :reply="reply"
                  :topic-author-id="expandedTopic?.author.user_id"
                  :is-own="isOwnReply(reply)"
                  @reply-to="startReplyTo"
                  @like="toggleReplyLike"
                  @report="(r) => openReport('reply', r.id)"
                  @delete="removeReply"
                />
              </template>
              <UiEmptyState v-else description="还没有回复" />
            </div>

            <!-- 回复输入（与帖子详情页共用 ForumComposer） -->
            <div class="mt-3">
              <ForumComposer
                v-model="replyContent"
                v-model:images="replyImages"
                v-model:replying-to="replyingTo"
                :submitting="replying"
                :max-images="3"
                :rows="2"
                placeholder="写下你的回复…"
                @submit="submitReply(topic.id)"
              />
            </div>

            <div v-if="expandedTopic?.can_delete" class="mt-2 text-right">
              <UiButton variant="danger" text size="small" @click="removeTopic(topic.id)">
                删除本帖
              </UiButton>
            </div>

            <!-- 举报对话框（与帖子详情同形态） -->
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
          </template>
        </div>
      </div>
    </template>

    <UiEmptyState
      v-else
      description="本章还没有讨论，来发第一帖吧"
      action-text="发新帖"
      @action="openCreate"
    />

    <!-- 发帖对话框：表单与论坛页共用 ForumPostForm，只保留壳 -->
    <UiDialog
      v-model="createVisible"
      title="发布章节讨论"
      subtitle="发表后将在本章讨论列表中展示"
      width="620px"
      confirm-text="发布"
      :confirm-loading="postForm?.submitting"
      :confirm-disabled="!postForm?.canSubmit"
      @confirm="postForm?.submit()"
    >
      <ForumPostForm
        ref="postForm"
        category="discussion"
        :chapter-id="chapterId"
        success-message="发布成功"
        @success="onTopicCreated"
      />
    </UiDialog>
  </div>
</template>
