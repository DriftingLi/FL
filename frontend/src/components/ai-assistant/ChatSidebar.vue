<template>
  <!-- 移动端抽屉遮罩 -->
  <div
    v-if="isMobile && mobileDrawerOpen"
    class="fixed inset-0 z-30 bg-black/40"
    @click="emit('update:mobileDrawerOpen', false)"
  ></div>

  <!-- 左上角悬浮图标组（侧栏不可见时显示：桌面收起态 / 移动端） -->
  <!-- 常驻 DOM、靠透明度显隐：收起侧栏时不至于整块瞬间出现（v-if 会让它和侧栏宽度
       过渡同时「啪」地冒出）。隐藏态 inert，键盘不会 Tab 进不可见的按钮。 -->
  <div
    class="fixed left-3 top-3 z-20 flex items-center gap-2 transition-opacity duration-[var(--duration-fast)] ease-[var(--ease-default)]"
    :class="isMobile || collapsed ? 'opacity-100' : 'pointer-events-none opacity-0'"
    :inert="!isMobile && !collapsed"
    :aria-hidden="!isMobile && !collapsed ? 'true' : undefined"
  >
    <!-- 胶囊容器：logo + 侧栏开关 + 新建 + 主题（对齐 DeepSeek 收起态） -->
    <div class="flex items-center gap-1 rounded-pill border border-line bg-panel p-1 shadow-card">
      <a :href="homeUrl" class="ml-0.5 mr-0.5 flex h-8 w-8 shrink-0 items-center justify-center" title="和润天下">
        <img :src="logoSrc" alt="和润天下" class="h-7 w-7 rounded-ctl object-cover" />
      </a>
      <button
        class="floating-icon-btn flex h-8 w-8 max-md:h-10 max-md:w-10 cursor-pointer items-center justify-center rounded-pill border-0 bg-transparent text-ink-3 transition-colors duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:bg-canvas hover:text-ink"
        :title="isMobile ? '打开会话栏' : '展开侧栏'"
        :aria-label="isMobile ? '打开会话栏' : '展开侧栏'"
        @click="isMobile ? emit('update:mobileDrawerOpen', true) : emit('update:collapsed', false)"
      >
        <UiIcon :name="isMobile ? 'menu' : 'panel-left-open'" :size="16" />
      </button>
      <button
        v-if="store.isLoggedIn"
        class="floating-icon-btn flex h-8 w-8 max-md:h-10 max-md:w-10 cursor-pointer items-center justify-center rounded-pill border-0 bg-transparent text-ink-3 transition-colors duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:bg-canvas hover:text-ink disabled:cursor-not-allowed disabled:opacity-40"
        :disabled="store.streaming"
        :title="newSessionTitle"
        :aria-label="newSessionTitle"
        @click="emit('new-session')"
      >
        <UiIcon name="plus" :size="16" />
      </button>
      <!-- 主题切换（三态下拉菜单） -->
      <ThemeToggle variant="ghost" />
    </div>
  </div>

  <!-- 会话侧栏（登录/未登录都渲染；移动端为抽屉） -->
  <!-- 桌面收起不再 `hidden`（瞬时抹掉一整列）：宽度恒定 260、用负 margin 把它滑出
       根容器的 overflow-hidden 边界，主对话区的宽度随之平滑增长；内容宽度不变，
       所以收起过程中会话标题不会先被挤成一行省略号。 -->
  <aside
    class="session-sidebar flex shrink-0 flex-col bg-canvas"
    :class="[
      isMobile
        ? ['fixed inset-y-0 left-0 z-40 w-[260px] border-r border-line shadow-lg transition-transform duration-[var(--duration-normal)] ease-[var(--ease-default)]', mobileDrawerOpen ? 'translate-x-0' : '-translate-x-full']
        : ['relative h-full w-[260px] border-r border-line transition-[margin-left] duration-[var(--duration-normal)] ease-[var(--ease-default)]', collapsed ? '-ml-[260px] pointer-events-none' : 'ml-0']
    ]"
    :inert="isMobile ? !mobileDrawerOpen : collapsed"
    :aria-hidden="(isMobile ? !mobileDrawerOpen : collapsed) ? 'true' : undefined"
  >
    <!-- 侧栏顶部：logo + 侧栏开关 + 主题 -->
    <div class="sidebar-header flex items-center justify-between px-3 py-3">
      <a :href="homeUrl" class="flex min-w-0 items-center gap-2 no-underline">
        <img :src="logoSrc" alt="和润天下" class="logo-img h-8 w-8 shrink-0 rounded-ctl object-cover" />
        <span class="logo-sub truncate text-xs tracking-[0.1em] text-ink-3">{{ logoSub }}</span>
      </a>
      <div class="flex shrink-0 items-center gap-1" v-if="!isMobile">
        <ThemeToggle variant="ghost" />
        <button
          class="flex h-8 w-8 cursor-pointer items-center justify-center rounded-pill border-0 bg-transparent text-ink-3 transition-colors duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:bg-panel hover:text-ink"
          title="收起侧栏"
          aria-label="收起侧栏"
          @click="emit('update:collapsed', true)"
        >
          <UiIcon name="panel-left-close" :size="16" />
        </button>
      </div>
      <button
        v-else
        class="flex h-8 w-8 cursor-pointer items-center justify-center rounded-pill border-0 bg-transparent text-ink-3"
        title="关闭"
        aria-label="关闭会话栏"
        @click="emit('update:mobileDrawerOpen', false)"
      >
        <UiIcon name="x" :size="16" />
      </button>
    </div>

    <!-- 功能页：返回链接 -->
    <div v-if="backLinkTo" class="px-3 pb-1">
      <router-link :to="backLinkTo" class="back-link flex items-center gap-1.5 rounded-ctl px-2 py-1.5 text-sm text-ink-2 no-underline transition-colors duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:bg-panel hover:underline">
        <UiIcon name="arrow-left" :size="14" />{{ backLinkText }}
      </router-link>
    </div>

    <!-- 新建会话（登录可见，DeepSeek 胶囊样式） -->
    <div v-if="store.isLoggedIn" class="px-3 pt-1">
      <button
        class="new-chat-btn flex w-full cursor-pointer items-center justify-center gap-2 rounded-pill border border-line bg-panel px-4 py-2.5 text-sm font-medium text-ink shadow-card transition-all duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:border-ui-400 hover:bg-panel disabled:cursor-not-allowed disabled:opacity-40"
        :disabled="store.streaming"
        :title="newSessionTitle"
        @click="emit('new-session')"
      >
        <UiIcon name="plus" :size="15" />开启新对话
      </button>
    </div>

    <!-- 会话列表（五档时间分组；加载态只描述列表本身，消息加载态在消息区） -->
    <div class="sidebar-title px-4 pb-1 pt-4 text-xs font-medium text-ink-3">会话历史</div>
    <div v-loading="store.sessionsLoading" class="session-list flex-1 overflow-y-auto px-2 pb-2">
      <div v-if="store.sessions.length === 0 && !store.sessionsLoading" class="empty-sessions px-3 py-8 text-center text-sm text-ink-3">
        {{ store.isLoggedIn ? '暂无会话，点击"开启新对话"开始' : guestLoginHint }}
      </div>
      <template v-for="g in groupedSessions" :key="g.label">
        <div class="px-3 pb-1 pt-3 text-xs text-ink-3">{{ g.label }}</div>
        <div
          v-for="s in g.sessions"
          :key="s.id"
          class="session-item group mb-1 flex cursor-pointer items-center gap-2 rounded-ctl px-3 py-2.5 transition-colors duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:bg-panel"
          :class="[store.currentSessionId === s.id ? 'bg-panel' : '', enableRename && editingSessionId === s.id ? 'editing bg-panel cursor-default' : '']"
          @click="handleSelectSession(s.id)"
        >
          <div class="session-info flex min-w-0 flex-1 flex-col gap-0.5">
            <UiInput
              v-if="enableRename && editingSessionId === s.id"
              v-model="editingTitle"
              size="small"
              :ref="(el: any) => { if (el) editInputRef = el }"
              placeholder="输入新标题"
              :maxlength="100"
              @click.stop
              @keydown.enter.prevent="commitRename(s.id)"
              @keydown.esc.prevent="cancelRename"
              @blur="commitRename(s.id)"
            />
            <template v-else>
              <span class="session-title truncate text-sm font-medium text-ink" @dblclick.stop="startRename(s)">{{ s.title || '新会话' }}</span>
              <span class="session-time text-xs text-ink-3">{{ formatShortDateTime(s.updated_at) }}</span>
            </template>
          </div>
          <div v-if="editingSessionId !== s.id" class="session-actions flex gap-0.5">
            <button v-if="enableRename" class="session-rename shrink-0 cursor-pointer rounded-ctl border-0 bg-transparent p-1 opacity-0 transition-all duration-[var(--duration-fast)] ease-[var(--ease-default)] group-hover:opacity-100 hover:bg-ui-soft hover:text-ui-soft-ink" title="重命名" aria-label="重命名会话" @click.stop="startRename(s)">
              <UiIcon name="pencil" :size="14" />
            </button>
            <button class="session-delete shrink-0 cursor-pointer rounded-ctl border-0 bg-transparent p-1 opacity-0 transition-all duration-[var(--duration-fast)] ease-[var(--ease-default)] group-hover:opacity-100 hover:bg-bad-soft hover:text-bad" title="删除" aria-label="删除会话" @click.stop="handleDeleteSession(s.id)">
              <UiIcon name="trash-2" :size="14" />
            </button>
          </div>
        </div>
      </template>
    </div>

    <!-- 侧栏底部：登录态 -->
    <div class="sidebar-footer border-t border-line p-3">
      <template v-if="store.isLoggedIn">
        <div class="flex items-center gap-2">
          <!-- 头像（有 avatar_url 显图，否则首字母） -->
          <img
            v-if="authStore.userInfo?.avatar_url"
            :src="String(authStore.userInfo.avatar_url)"
            alt="头像"
            class="h-8 w-8 shrink-0 rounded-pill object-cover"
          />
          <div
            v-else
            class="flex h-8 w-8 shrink-0 items-center justify-center rounded-pill bg-ui-soft-strong text-sm font-semibold text-ui-soft-ink"
          >
            {{ (authStore.userInfo?.username || '?').charAt(0) }}
          </div>
          <router-link :to="href('StudentProfile')" class="profile-link min-w-0 truncate text-sm font-medium text-ink-2 no-underline transition-colors duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:underline">{{ displayName }}</router-link>
          <UiMoreMenu :items="userMenuItems" @select="handleUserCommand" />
        </div>
      </template>
      <template v-else>
        <UiButton variant="primary" class="w-full" @click="goLogin">登录 HRWAI 账号</UiButton>
      </template>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { useRouter, type RouteLocationRaw } from 'vue-router'
import { ElMessage } from 'element-plus'
import UiIcon from '@/components/ui/UiIcon.vue'
import UiMoreMenu, { type UiMoreMenuItem } from '@/components/ui/UiMoreMenu.vue'
import ThemeToggle from '@/components/ui/ThemeToggle.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiInput from '@/components/ui/UiInput.vue'
import { href } from '@/config/pages'
import { useAIAssistantStore } from '@/stores/aiAssistant'
import { useAuthStore } from '@/stores/auth'
import { buildSubdomainUrl } from '@/utils/subdomain'
import { formatShortDateTime } from '@/utils/format'
import { useConfirm } from '@/composables/useConfirm'

const props = withDefaults(
  defineProps<{
    logoSub: string
    loginRedirect: string
    backLinkTo?: RouteLocationRaw
    backLinkText?: string
    enableRename?: boolean
    /** 跨区状态由壳持有，子件只发同步事件 */
    collapsed: boolean
    mobileDrawerOpen: boolean
    isMobile: boolean
    /** 游客提示与欢迎区共用同一文案 */
    guestLoginHint: string
  }>(),
  {
    backLinkText: '',
    enableRename: false
  }
)

const emit = defineEmits<{
  (e: 'update:collapsed', value: boolean): void
  (e: 'update:mobileDrawerOpen', value: boolean): void
  (e: 'new-session'): void
}>()

const store = useAIAssistantStore()
const authStore = useAuthStore()
const router = useRouter()

const homeUrl = buildSubdomainUrl('main', '/')
// 运行时绑定（避免测试环境对静态资源 URL 的编译期解析）
const logoSrc = '/images/HRWAIlogo.jpg'
const displayName = computed(() => authStore.userInfo?.username || 'HRWAI 用户')

// 流式进行中的「新对话」显式反馈（#1104）：入口禁用 + 标题说明为什么不可点（不再静默吞掉点击）
const newSessionTitle = computed(() => (store.streaming ? '生成中，请先停止再开启新对话' : '开启新对话'))

// 会话时间分组五档（今天/昨天/7天内/30天内/更早），组内 updated_at 倒序
const groupedSessions = computed(() => {
  if (!store.isLoggedIn || store.sessions.length === 0) return []
  const now = new Date()
  const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime()
  const DAY = 86_400_000
  const groups: Array<{ label: string; sessions: typeof store.sessions }> = [
    { label: '今天', sessions: [] },
    { label: '昨天', sessions: [] },
    { label: '7 天内', sessions: [] },
    { label: '30 天内', sessions: [] },
    { label: '更早', sessions: [] }
  ]
  const sorted = [...store.sessions].sort(
    (a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()
  )
  for (const s of sorted) {
    const t = new Date(s.updated_at).getTime()
    if (Number.isNaN(t)) {
      groups[4].sessions.push(s)
      continue
    }
    const dayDiff = Math.floor((startOfToday - new Date(t).setHours(0, 0, 0, 0)) / DAY)
    if (dayDiff <= 0) groups[0].sessions.push(s)
    else if (dayDiff === 1) groups[1].sessions.push(s)
    else if (dayDiff < 7) groups[2].sessions.push(s)
    else if (dayDiff < 30) groups[3].sessions.push(s)
    else groups[4].sessions.push(s)
  }
  return groups.filter(g => g.sessions.length > 0)
})

// ===== 会话重命名（主页 enableRename 时启用） =====
const editingSessionId = ref<number | null>(null)
const editingTitle = ref('')
const editInputRef = ref<any>(null)
let renamingLock = false // 防止 blur + enter 重复触发

// 选中会话：若正在编辑当前会话则不切换；失败弹提示且不收抽屉（留在列表重试）
async function handleSelectSession(id: number) {
  if (editingSessionId.value === id) return
  try {
    await store.selectSession(id)
    if (props.isMobile) emit('update:mobileDrawerOpen', false)
  } catch (e: any) {
    ElMessage.error(e?.message || '加载会话消息失败，请重试')
  }
}

// 进入重命名模式
async function startRename(s: { id: number; title: string }) {
  editingSessionId.value = s.id
  editingTitle.value = s.title || ''
  renamingLock = false
  await nextTick()
  // 自动聚焦 + 选中文字
  const input = editInputRef.value?.input || editInputRef.value?.textarea
  if (input) {
    input.focus()
    setTimeout(() => input.select?.(), 0)
  }
}

function cancelRename() {
  editingSessionId.value = null
  editingTitle.value = ''
  renamingLock = false
}

// 提交重命名
async function commitRename(sessionId: number) {
  if (renamingLock) return
  renamingLock = true
  try {
    if (editingSessionId.value !== sessionId) {
      // 已被取消或切换到其他会话
      return
    }
    const newTitle = editingTitle.value.trim()
    if (!newTitle) {
      cancelRename()
      return
    }
    const session = store.sessions.find(s => s.id === sessionId)
    if (session && session.title === newTitle) {
      // 未变更，直接退出编辑
      cancelRename()
      return
    }
    try {
      await store.renameSession(sessionId, newTitle)
      ElMessage.success('已更新会话标题')
    } catch {
      /* 错误已由拦截器提示 */
    } finally {
      cancelRename()
    }
  } finally {
    renamingLock = false
  }
}

async function handleDeleteSession(id: number) {
  try {
    await useConfirm().confirmDanger('确定删除该会话？所有消息将一并删除。', '确认', { type: 'warning' })
  } catch {
    return
  }
  try {
    await store.deleteSession(id)
  } catch (e: any) {
    ElMessage.error(e?.message || '删除会话失败，请重试')
  }
}

// ===== 登录/退出 =====
function goLogin() {
  router.push({ ...href('Login'), query: { redirect: props.loginRedirect } })
}

/** 侧栏用户行的「⋯」：个人资料 / 退出登录（低频动作走菜单收纳） */
const userMenuItems: UiMoreMenuItem[] = [
  { key: 'profile', label: '个人资料' },
  { key: 'logout', label: '退出登录', divided: true }
]

async function handleUserCommand(cmd: string) {
  if (cmd === 'profile') {
    router.push(href('StudentProfile'))
    return
  }
  if (cmd === 'logout') {
    // 登出单点（票1）：revoke + 清本地走 store.signOut；本页专属清理（会话上下文）留在壳里
    await authStore.signOut()
    // 状态变更走 store action（#620）：清消息/会话上下文，未登录 loadSessions 清空侧栏列表
    store.clearMessages()
    await store.loadSessions()
    ElMessage.success('已退出登录')
  }
}
</script>
