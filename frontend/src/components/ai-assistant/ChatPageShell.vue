<template>
  <div class="ai-chat-shell relative flex h-screen overflow-hidden bg-canvas">
    <ChatSidebar
      :logo-sub="logoSub"
      :login-redirect="loginRedirect"
      :back-link-to="backLinkTo"
      :back-link-text="backLinkText"
      :enable-rename="enableRename"
      :collapsed="collapsed"
      :mobile-drawer-open="mobileDrawerOpen"
      :is-mobile="isMobile"
      :guest-login-hint="GUEST_LOGIN_HINT"
      @update:collapsed="collapsed = $event"
      @update:mobile-drawer-open="mobileDrawerOpen = $event"
      @new-session="emit('new-session')"
    />

    <!-- 右侧对话区。两种空态形态由 dockInput 决定：
         ① 居中态（默认，其余功能页与助手主页）：grid 三行 1fr/auto/1fr——输入框在正中行精确居中，
            欢迎区占第一行底对齐（依托输入框上沿向上生长，矮屏行保持内容高可滚动不裁切）；
         ② 贴底态（dockInput=true，智能维修诊断页）：空态与有消息态同一套 flex 列 ——
            内容区 flex-1 自滚、输入框始终贴底。对接方原设计（其 `.composer` 是 `.chat-area`
            flex 列的最后一项、`.chat`/`.welcome` 各自 flex:1 滚动）就是这样：选诉求时视线
            不用往下找输入框，向导面板展开也不会把整块内容顶上去。
         有消息：列表占满、输入框沉底（两态一致） -->
    <main
      class="chat-main min-w-0 flex-1 bg-panel"
      :class="gridWelcome ? 'grid grid-rows-[1fr_auto_1fr] overflow-hidden' : 'flex flex-col overflow-hidden'"
    >
      <ChatMessageList
        :ref="setMessageListRef"
        :is-welcome="isWelcome"
        :grid-welcome="gridWelcome"
        :at-bottom="atBottom"
        @resume-follow="resumeFollow"
      >
        <template #welcome>
        <!-- 空状态：欢迎区（图标左标题右横排；标题即一句话功能介绍，无副标题）。
             定位：grid 第一行 + self-end 贴住输入框上沿，不把输入框往下推。
             溢出：欢迎区可能比首屏高（诊断页会塞品牌卡 + 六张诉求卡 + FAQ，票 2241 已拍板），
             此时**只让欢迎区自己滚**（max-height + overflow-y-auto），不能让 chat-main 滚 ——
             否则矮屏下输入框被挤出视口，而输入框是这个页面的唯一出口。
             贴底态例外（dockInput）：列表本身已是 flex-1 的滚动容器，欢迎区只管按内容长高、
             由列表滚 —— 两层都 overflow-y-auto 会出现互相抢滚的双滚动条。 -->
        <div
          v-if="isWelcome"
          class="welcome-area w-full px-6 pb-3 pt-6 text-center"
          :class="gridWelcome ? 'overflow-y-auto' : ''"
          :style="gridWelcome ? 'max-height: 100%; scrollbar-width: thin' : 'scrollbar-width: thin'"
        >
          <div class="welcome-head mx-auto flex max-w-[760px] items-center justify-center gap-3 text-left">
            <div class="welcome-icon inline-flex h-[52px] w-[52px] shrink-0 items-center justify-center rounded-card bg-[linear-gradient(135deg,var(--color-violet-500,#6366f1),#8b5cf6)] text-white">
              <UiIcon :name="welcomeIcon" :size="28" />
            </div>
            <h2 class="welcome-title m-0 text-xl font-bold leading-snug text-ink">{{ welcomeTitle }}</h2>
          </div>

          <slot name="welcome-top" />

          <!-- 预设提示词：聊天气泡横向一字排开（方案 B 翻页箭头收纳；自动换行不省略） -->
          <div v-if="suggestions.length" class="suggestion-pager mx-auto mt-5 flex max-w-[600px] items-center gap-1.5">
            <SuggestionArrow dir="prev" :disabled="!canPagePrev" @page="pageSuggestions(-1)" />
            <div class="suggestion-view min-w-0 flex-1 overflow-hidden">
              <div
                class="suggestion-track flex gap-2 transition-transform duration-[var(--duration-normal)] ease-[var(--ease-default)]"
                :style="{ transform: `translateX(-${suggestionPage * 100}%)` }"
              >
                <div
                  v-for="(page, pi) in suggestionPages"
                  :key="pi"
                  class="suggestion-page flex w-full shrink-0 flex-nowrap gap-2 overflow-hidden"
                >
                  <div
                    v-for="s in page"
                    :key="s"
                    class="suggestion-bubble min-w-0 flex-1 cursor-pointer whitespace-normal break-words rounded-card border border-line bg-panel px-3.5 py-2.5 text-left text-sm leading-[1.5] text-ink-2 transition-all duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:border-ui-400 hover:bg-ui-soft hover:text-ui-soft-ink"
                    :title="s"
                    @click="emit('suggest', s)"
                  >
                    {{ s }}
                  </div>
                </div>
              </div>
            </div>
            <SuggestionArrow dir="next" :disabled="!canPageNext" @page="pageSuggestions(1)" />
          </div>

          <slot name="welcome-bottom" />

          <button
            v-if="!store.isLoggedIn"
            type="button"
            class="guest-hint mt-8 inline-block cursor-pointer border-0 bg-transparent p-0 text-sm text-ink-3 transition-colors duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:text-ui-soft-ink"
            @click="goLogin"
          >{{ GUEST_LOGIN_HINT }}</button>
        </div>

        </template>
        <template #assistant-caps="{ message }">
          <slot name="assistant-caps" :message="message" />
        </template>
        <template #assistant-extra="{ message }">
          <slot name="assistant-extra" :message="message" />
        </template>
      </ChatMessageList>

      <ChatComposer
        :input-text="props.inputText"
        :input-placeholder="props.inputPlaceholder"
        :raised-input="raisedInput"
        :can-send="canSend"
        :grid-welcome="gridWelcome"
        :is-welcome="isWelcome"
        @update:input-text="emit('update:inputText', $event)"
        @send="emit('send')"
      >
        <template #input-toolbar><slot name="input-toolbar" /></template>
        <template #input-above><slot name="input-above" /></template>
        <template v-if="slots['input-prefix']" #input-prefix><slot name="input-prefix" /></template>
        <template #input-footer-left><slot name="input-footer-left" /></template>
        <template #input-extra><slot name="input-extra" /></template>
      </ChatComposer>
    </main>
  </div>
</template>

<script setup lang="ts">
import type { IconName } from '@/icons/registry'
import UiIcon from '@/components/ui/UiIcon.vue'
// AI 聊天壳（#398）：共享 store、跨区布局/侧栏/滚动状态、欢迎区与建议词留在壳内；
// 会话侧栏、消息列表、输入区按 DOM 职责拆成子件，页面差异只经槽位与窄 props/emits 接入。
import { ref, computed, useSlots, watch, onMounted, onBeforeUnmount } from 'vue'
import type { ComponentPublicInstance } from 'vue'
import { useRouter, type RouteLocationRaw } from 'vue-router'
import { href } from '@/config/pages'
import { useAIAssistantStore } from '@/stores/aiAssistant'
import SuggestionArrow from '@/components/ai-assistant/SuggestionArrow.vue'
import ChatComposer from '@/components/ai-assistant/ChatComposer.vue'
import ChatSidebar from '@/components/ai-assistant/ChatSidebar.vue'
import ChatMessageList from '@/components/ai-assistant/ChatMessageList.vue'

const props = withDefaults(
  defineProps<{
    /** 顶部栏副标题（主页：AI 叉车助手 · HRWAI；功能页：AI 叉车助手 · 功能名） */
    logoSub: string
    /** 输入框内容（v-model:input-text，状态由页面持有） */
    inputText: string
    /** 登录跳转目标（登录成功后回跳） */
    loginRedirect: string
    /** 顶部栏附加返回链接（功能页：返回 AI 助手） */
    backLinkTo?: RouteLocationRaw
    backLinkText?: string
    /** 欢迎区图标/一句话标题（标题即功能介绍，无副标题） */
    welcomeIcon: IconName
    welcomeTitle: string
    /** 预设提示词（点击后 emit('suggest', text)） */
    suggestions?: string[]
    /**
     * 输入框贴底（默认 false = 空态垂直居中）。
     * 诊断页打开它：对接方原设计的 `.composer` 就钉在对话列底部，
     * 选诉求时视线不用往下找输入框，向导面板展开也不会把内容顶上去。
     */
    dockInput?: boolean
    /** 会话重命名（主页专用能力） */
    enableRename?: boolean
    /** 输入区拉高布局（主页：模式选择器 footer） */
    raisedInput?: boolean
    /** 输入框占位文案的任务片段 —— 统一发送提示（Enter / Shift+Enter）由输入区在其后拼上 */
    inputPlaceholder?: string
    /** 页面侧草稿就绪（文本非空 / 图片队列就绪）；壳再与 store.canSend 合成唯一发送判据（#1104） */
    canSend?: boolean
  }>(),
  {
    backLinkText: '',
    suggestions: () => [],
    dockInput: false,
    enableRename: false,
    raisedInput: false,
    inputPlaceholder: '输入您的问题...',
    canSend: false
  }
)

const emit = defineEmits<{
  (e: 'update:inputText', value: string): void
  (e: 'send'): void
  (e: 'suggest', text: string): void
  (e: 'new-session'): void
}>()

const slots = useSlots()
const store = useAIAssistantStore()
const router = useRouter()

function goLogin() {
  router.push({ ...href('Login'), query: { redirect: props.loginRedirect } })
}

/** 游客提示唯一文案：侧栏空列表与欢迎区共用（侧栏底不再重复一遍同样的句子） */
const GUEST_LOGIN_HINT = '登录后可保存对话历史'
// 空状态判定（欢迎区渲染条件 + 消息列表垂直居中开关）：**加载中也算非空态** ——
// 否则点会话那一瞬欢迎区与「会话加载中…」同屏，建议词还能点进一个尚未装载完的会话。
const isWelcome = computed(
  () => store.messages.length === 0 && !store.streaming && !store.messagesLoading
)
// 空态是否走 grid 三行居中：只有「空态 且 没开贴底」时才是。贴底态下空态与有消息态
// 共用同一套 flex 列（见模板 chat-main 注释），这样两态的输入框位置不会在首问后跳变。
const gridWelcome = computed(() => isWelcome.value && !props.dockInput)

// 发送判据单点（#1104）：页面草稿就绪（prop）× store 闸门（无在飞轮次）——
// Enter 与发送按钮读同一个值，页面不再有「Enter 直通」的旁路。
const canSend = computed(() => Boolean(props.canSend && store.canSend))

// ref 属性挂在 MessageList 上，壳把组件公开实例的 $el 收为根节点引用。
const messageListRef = ref<HTMLElement>()

function setMessageListRef(instance: Element | ComponentPublicInstance | null) {
  messageListRef.value = (instance && '$el' in instance ? instance.$el : instance) as HTMLElement | undefined
}
// 是否贴底（跟随开关）：滚动时更新，既决定是否自动跟随，也决定「回到最新」入口出不出
const atBottom = ref(true)

// ===== 侧栏状态：桌面收起（localStorage 持久化）+ 移动端抽屉 =====
const COLLAPSE_KEY = 'ai-sidebar-collapsed'
const collapsed = ref(localStorage.getItem(COLLAPSE_KEY) === '1')
const mobileDrawerOpen = ref(false)
// 视口判定（jsdom/SSR 安全默认桌面）
const isMobile = ref(false)
let mql: MediaQueryList | null = null

function syncViewport(e: MediaQueryListEvent | MediaQueryList) {
  isMobile.value = e.matches
  if (!e.matches) mobileDrawerOpen.value = false
}

onMounted(() => {
  mql = window.matchMedia('(max-width: 768px)')
  syncViewport(mql)
  mql.addEventListener('change', syncViewport)
  // 滚动才更新跟随开关（passive：只读几何，不阻塞合成器）
  messageListRef.value?.addEventListener('scroll', syncFollowState, { passive: true })
})

onBeforeUnmount(() => {
  mql?.removeEventListener('change', syncViewport)
  messageListRef.value?.removeEventListener('scroll', syncFollowState)
})

watch(collapsed, v => {
  localStorage.setItem(COLLAPSE_KEY, v ? '1' : '0')
})

// ===== 预设提示词翻页（方案 B：横向一字排开、箭头收纳；suggestions 变化回第一页）=====
// 每页条数内联常量：规格即 3 条一页横向单行，无调用方需要配置。
const SUGGESTION_PAGE_SIZE = 3
const suggestionPage = ref(0)
const suggestionPages = computed(() => {
  const pages: string[][] = []
  for (let i = 0; i < props.suggestions.length; i += SUGGESTION_PAGE_SIZE) {
    pages.push(props.suggestions.slice(i, i + SUGGESTION_PAGE_SIZE))
  }
  return pages.length ? pages : [[]]
})
const canPagePrev = computed(() => suggestionPage.value > 0)
const canPageNext = computed(() => suggestionPage.value < suggestionPages.value.length - 1)

function pageSuggestions(dir: number) {
  const next = suggestionPage.value + dir
  if (next < 0 || next >= suggestionPages.value.length) return
  suggestionPage.value = next
}

watch(() => props.suggestions, () => {
  suggestionPage.value = 0
})

// ===== 视口跟随（拖底）=====
// 无条件滚底是桌面端的经典刺点：用户趁流式吐字向上翻历史，每来一个 chunk 就被拽回底部。
// 判据只此一处（距底 ≤ 40px），跟随与「回到最新」入口共用同一份判定。
const BOTTOM_FOLLOW_THRESHOLD_PX = 40

function distanceFromBottom(el: HTMLElement): number {
  return el.scrollHeight - el.scrollTop - el.clientHeight
}

function syncFollowState() {
  const el = messageListRef.value
  if (!el) return
  atBottom.value = distanceFromBottom(el) <= BOTTOM_FOLLOW_THRESHOLD_PX
}

/**
 * 滚到底。smooth 只给用户主动点击用（并尊重 prefers-reduced-motion）；
 * 内容增长触发的跟随一律瞬时，避免动画追不上流式吐字。
 */
function scrollToBottom(smooth = false) {
  const el = messageListRef.value
  if (!el) return
  const reduced = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches === true
  if (smooth && !reduced && typeof el.scrollTo === 'function') {
    el.scrollTo({ top: el.scrollHeight, behavior: 'smooth' })
  } else {
    el.scrollTop = el.scrollHeight
  }
  atBottom.value = true
}

/** 「回到最新」入口：复位跟随再滚，避免下一帧又被贴底判定弹回 */
function resumeFollow() {
  scrollToBottom(true)
}

watch(() => store.messages.length, (next, prev) => {
  if (next <= prev) return
  // 用户刚发出的那条一定贴底；assistant 落库不抢正在回翻的视口
  if (store.messages[next - 1]?.role === 'user' || atBottom.value) scrollToBottom()
}, { flush: 'post' })
watch(() => store.streamingContent, () => {
  if (atBottom.value) scrollToBottom()
}, { flush: 'post' })
// 会话切换：等消息装载完再落底（加载期间列表里还是上一会话的内容，抢滚会滚错位置）
watch(() => store.messagesLoading, (_loading, wasLoading) => {
  if (wasLoading && !_loading) scrollToBottom()
}, { flush: 'post' })
</script>
