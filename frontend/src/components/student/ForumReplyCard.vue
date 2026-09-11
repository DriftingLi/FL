<script setup lang="ts">
/**
 * 论坛回复卡（ADR-0042）：帖子详情与章节讨论共用的一份回复渲染。
 *
 * 信息层级（评审定稿形态）：
 *   昵称行  头像 · 昵称 ›（被回复人小头像 + 名）·[楼主]·[✓已采纳] ······ ⋯
 *   正文    内容 + 图片画廊
 *   采纳行  楼主视角的「采纳此回答 / 取消采纳」（独立一行，不进 ⋯、不并入互动行）
 *   操作行  左：相对时间    右：回复、点赞
 *
 * 动作分层的硬边界（docs/agents/ui-conventions.md「溢出菜单」）：**互动动作（回复 / 点赞）不进 ⋯**，
 * 治理动作（举报 / 删除）只出现在 ⋯ 里。UiMoreMenu 只认 items；**「自己的内容不显示举报」这条规则**
 * 收在本组件与帖子卡各自的 items 计算里（调用方只提供事实 `isOwn`），不在模板层散判。
 *
 * density：详情页 comfortable（38px 头像 / text-sm）；章节讨论 compact（26px / 13px）——
 * 内嵌面板不套详情页尺寸，避免把课程页的展开区撑长。
 */
import { computed } from 'vue'
import { ChatDotRound } from '@element-plus/icons-vue'
import type { ForumReplyItem } from '@/api/forum'
import { displayName, authorLetter } from '@/utils/forumDisplay'
import { formatRelativeTime } from '@/utils/format'
import ForumImageGallery from './ForumImageGallery.vue'
import UiActionChip from '@/components/ui/UiActionChip.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiMoreMenu, { type UiMoreMenuItem } from '@/components/ui/UiMoreMenu.vue'
import UiTag from '@/components/ui/UiTag.vue'

const props = withDefaults(defineProps<{
  reply: ForumReplyItem
  /** 楼主 user_id：用于「楼主」标记；不传则不判楼主 */
  topicAuthorId?: number
  /** 是否为当前登录用户自己发的（决定 ⋯ 里是否出现「举报」） */
  isOwn?: boolean
  /** comfortable：帖子详情 / compact：章节讨论内嵌面板 */
  density?: 'comfortable' | 'compact'
  /** 楼主视角：这条可被采纳（问答帖 + 非本人作答 + 尚未采纳） */
  canAccept?: boolean
  /** 楼主视角：这条已被采纳，可取消 */
  canCancelAccept?: boolean
}>(), {
  density: 'comfortable',
  isOwn: false,
  canAccept: false,
  canCancelAccept: false
})

const emit = defineEmits<{
  'reply-to': [reply: ForumReplyItem]
  like: [reply: ForumReplyItem]
  report: [reply: ForumReplyItem]
  delete: [replyId: number]
  accept: [replyId: number]
  'cancel-accept': []
}>()

const compact = computed(() => props.density === 'compact')
const avatarSize = computed(() => (compact.value ? 26 : 38))
const contentClass = computed(() =>
  compact.value ? 'text-[13px] leading-[1.6]' : 'text-sm leading-[1.7]'
)
const isTopicAuthor = computed(
  () => props.topicAuthorId != null && props.reply.author.user_id === props.topicAuthorId
)

/**
 * ⋯ 菜单项：**自己的回复不出现「举报」**（后端目前无自举报拦截，这是前端入口收敛）；
 * 「删除」按后端下发的 can_delete 出现在自己的回复上。两者都为空时 UiMoreMenu 不渲染触发按钮。
 */
const moreItems = computed<UiMoreMenuItem[]>(() => {
  const items: UiMoreMenuItem[] = []
  if (!props.isOwn) items.push({ key: 'report', label: '举报' })
  if (props.reply.can_delete) items.push({ key: 'delete', label: '删除', tone: 'danger' })
  return items
})

function onMoreSelect(key: string) {
  if (key === 'report') emit('report', props.reply)
  else if (key === 'delete') emit('delete', props.reply.id)
}
</script>

<template>
  <div
    class="reply-item flex"
    :class="[
      compact ? 'gap-2 py-3' : 'gap-3 py-4',
      reply.is_accepted
        ? 'is-accepted relative -mx-2 my-1.5 rounded-[8px] bg-ok-soft p-3 pl-4 before:absolute before:inset-y-0 before:left-0 before:w-[3px] before:rounded-full before:bg-ok'
        : ''
    ]"
  >
    <el-avatar :size="avatarSize" :src="reply.author.avatar_url || undefined" class="shrink-0">
      {{ authorLetter(reply.author) }}
    </el-avatar>

    <div class="reply-main min-w-0 flex-1">
      <!-- 昵称行：昵称 ›（被回复人小头像 + 名）·标签·  ······ ⋯ -->
      <div class="reply-meta flex flex-wrap items-center" :class="compact ? 'gap-1.5' : 'gap-2'">
        <span class="author-name font-semibold text-ink" :class="compact ? 'text-[13px]' : 'text-sm'">
          {{ displayName(reply.author) }}
        </span>
        <span
          v-if="reply.parent_id && reply.parent_name"
          class="reply-parent inline-flex min-w-0 items-center gap-1 text-xs text-ink-3"
        >
          <span class="text-[var(--color-border-dark)]">›</span>
          <el-avatar :size="16" :src="reply.parent_avatar_url || undefined" class="shrink-0">
            {{ (reply.parent_name || '?').charAt(0).toUpperCase() }}
          </el-avatar>
          <span class="truncate">{{ reply.parent_name }}</span>
        </span>
        <UiTag v-if="isTopicAuthor" tone="neutral" effect="plain">楼主</UiTag>
        <UiTag v-if="reply.is_accepted" tone="success" effect="dark" class="font-semibold">✓ 已采纳</UiTag>
        <UiMoreMenu class="ml-auto" :items="moreItems" @select="onMoreSelect" />
      </div>

      <div class="reply-content mt-1.5 whitespace-pre-wrap break-words text-ink" :class="contentClass">
        {{ reply.content }}
      </div>
      <ForumImageGallery :images="reply.images" />

      <!-- 采纳行：楼主视角专属，独立于互动行（采纳是判定动作，不与点赞同级） -->
      <div v-if="canAccept || canCancelAccept" class="reply-accept-row mt-2">
        <UiButton v-if="canAccept" variant="success" plain size="small" @click="emit('accept', reply.id)">
          采纳此回答
        </UiButton>
        <UiButton v-else size="small" @click="emit('cancel-accept')">取消采纳</UiButton>
      </div>

      <!-- 操作行：左时间、右互动（回复 + 点赞）；治理动作在上方 ⋯ 里 -->
      <div class="reply-actions mt-2 flex items-center gap-1.5 text-xs text-ink-3">
        <span class="reply-time">{{ formatRelativeTime(reply.created_at) }}</span>
        <div class="reply-actions-right ml-auto flex items-center gap-3">
          <button
            type="button"
            class="reply-btn inline-flex cursor-pointer items-center gap-1 rounded-[6px] border-0 bg-transparent p-0 px-1.5 py-1 text-xs font-medium leading-none text-ink-3 transition-colors duration-[var(--duration-base)] ease-[var(--ease-default)] hover:text-ui-500"
            @click="emit('reply-to', reply)"
          >
            <el-icon><ChatDotRound /></el-icon>
            <span>回复</span>
          </button>
          <UiActionChip
            icon="like"
            :label="reply.liked_by_me ? '已赞' : '点赞'"
            :count="reply.likes_count"
            tone="like"
            compact
            borderless
            :active="!!reply.liked_by_me"
            @click="emit('like', reply)"
          />
        </div>
      </div>
    </div>
  </div>
</template>
