<template>
  <div class="ai-chat-shell flex h-screen flex-col bg-canvas">
    <!-- 顶部栏 -->
    <header class="topbar shrink-0 border-b border-line bg-panel">
      <div class="topbar-container mx-auto flex h-16 max-w-[1600px] items-center justify-between gap-4 px-6 max-[768px]:h-14 max-[768px]:px-3">
        <a :href="homeUrl" class="logo-link flex shrink-0 items-center gap-3 no-underline">
          <img :src="logoSrc" alt="和润天下" class="logo-img h-9 w-9 rounded-ctl object-cover" />
          <div class="logo-text-wrap flex flex-col leading-[1.1]">
            <span class="logo-text text-base font-bold text-ink max-[768px]:text-sm">和润天下</span>
            <span class="logo-sub mt-0.5 text-[11px] tracking-[0.1em] text-ink-3 max-[768px]:hidden">{{ logoSub }}</span>
          </div>
        </a>

        <div class="topbar-actions flex shrink-0 items-center gap-3 max-[768px]:gap-2">
          <router-link v-if="backLinkTo" :to="backLinkTo" class="back-link rounded-ctl border border-line-strong px-3 py-1.5 text-[13px] text-ink-2 no-underline transition-all duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:border-ui-500 hover:text-ui-600 max-[768px]:hidden">{{ backLinkText }}</router-link>

          <!-- 未登录：显示登录按钮 -->
          <UiButton variant="primary" v-if="!store.isLoggedIn" size="default" @click="goLogin">
            登录 HRWAI 账号
          </UiButton>

          <!-- 已登录：显示用户名 + 退出 -->
          <template v-else>
            <router-link to="/training/profile" class="profile-link text-sm font-medium text-ui-600 no-underline transition-opacity duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:opacity-80 hover:underline">个人资料</router-link>
            <el-dropdown trigger="click" @command="handleUserCommand">
              <span class="user-trigger inline-flex cursor-pointer items-center gap-1 rounded-ctl px-3 py-1.5 text-sm font-medium text-ink transition-colors duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:bg-canvas">
                {{ displayName }}
                <el-icon><ArrowDown /></el-icon>
              </span>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="logout">退出登录</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>

          <a :href="homeUrl" class="back-link rounded-ctl border border-line-strong px-3 py-1.5 text-[13px] text-ink-2 no-underline transition-all duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:border-ui-500 hover:text-ui-600 max-[768px]:hidden">返回官网</a>
        </div>
      </div>
    </header>

    <!-- 主体：侧栏 + 对话区 -->
    <div class="main-body flex flex-1 overflow-hidden">
      <!-- 左侧会话栏（仅登录后显示） -->
      <aside v-if="store.isLoggedIn" class="session-sidebar flex w-[260px] shrink-0 flex-col border-r border-line bg-panel max-[768px]:hidden">
        <div class="sidebar-header flex items-center justify-between border-b border-line px-4 py-3">
          <span class="sidebar-title text-[13px] font-semibold text-ink-2">会话历史</span>
          <UiButton variant="primary" size="small" :icon="Plus" @click="emit('new-session')">新建</UiButton>
        </div>
        <div v-loading="store.sessionsLoading" class="session-list flex-1 overflow-y-auto p-2">
          <div v-if="store.sessions.length === 0 && !store.sessionsLoading" class="empty-sessions px-3 py-8 text-center text-[13px] text-ink-3">
            暂无会话，点击"新建"开始对话
          </div>
          <div
            v-for="s in store.sessions"
            :key="s.id"
            class="session-item group mb-1 flex cursor-pointer items-center gap-2 rounded-ctl px-3 py-2.5 transition-colors duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:bg-canvas"
            :class="[store.currentSessionId === s.id ? 'bg-ui-50' : '', enableRename && editingSessionId === s.id ? 'editing bg-ui-50 cursor-default' : '']"
            @click="handleSelectSession(s.id)"
          >
            <div class="session-info flex min-w-0 flex-1 flex-col gap-0.5">
              <!-- 编辑模式（enableRename） -->
              <el-input
                v-if="enableRename && editingSessionId === s.id"
                v-model="editingTitle"
                size="small"
                :ref="(el: any) => { if (el) editInputRef = el }"
                placeholder="输入新标题"
                maxlength="100"
                @click.stop
                @keydown.enter.prevent="commitRename(s.id)"
                @keydown.esc.prevent="cancelRename"
                @blur="commitRename(s.id)"
              />
              <!-- 展示模式 -->
              <template v-else>
                <span class="session-title truncate text-[13px] font-medium text-ink" @dblclick.stop="startRename(s)">{{ s.title || '新会话' }}</span>
                <span class="session-time text-[11px] text-ink-3">{{ formatShortDateTime(s.updated_at) }}</span>
              </template>
            </div>
            <div v-if="editingSessionId !== s.id" class="session-actions flex gap-0.5">
              <button v-if="enableRename" class="session-rename shrink-0 cursor-pointer rounded border-0 bg-transparent p-1 opacity-0 transition-all duration-[var(--duration-fast)] ease-[var(--ease-default)] group-hover:opacity-100 hover:bg-ui-50 hover:text-ui-600" title="重命名" @click.stop="startRename(s)">
                <el-icon :size="14"><Edit /></el-icon>
              </button>
              <button class="session-delete shrink-0 cursor-pointer rounded border-0 bg-transparent p-1 opacity-0 transition-all duration-[var(--duration-fast)] ease-[var(--ease-default)] group-hover:opacity-100 hover:bg-bad-soft hover:text-bad" title="删除" @click.stop="handleDeleteSession(s.id)">
                <el-icon :size="14"><Delete /></el-icon>
              </button>
            </div>
          </div>
        </div>
      </aside>

      <!-- 右侧对话区 -->
      <main class="chat-main flex flex-1 flex-col overflow-hidden bg-canvas">
        <!-- 消息列表 -->
        <div ref="messageListRef" class="message-list mx-auto w-full max-w-[900px] flex-1 overflow-y-auto p-6 max-[768px]:p-4">
          <!-- 空状态：欢迎区（差异内容走 welcome 槽位） -->
          <div v-if="store.messages.length === 0 && !store.streaming" class="welcome-area px-6 py-12 text-center">
            <div class="welcome-icon mb-5 inline-flex h-[72px] w-[72px] items-center justify-center rounded-[20px] bg-[linear-gradient(135deg,var(--color-violet-500,#6366f1),#8b5cf6)] text-white">
              <el-icon :size="36"><component :is="welcomeIcon" /></el-icon>
            </div>
            <h2 class="welcome-title m-0 mb-2 text-2xl font-bold text-ink">{{ welcomeTitle }}</h2>
            <p class="welcome-desc mx-auto mb-6 max-w-[560px] text-sm leading-[1.6] text-ink-3">{{ welcomeDesc }}</p>

            <slot name="welcome-top" />

            <!-- 预设提示词（两页共用实现） -->
            <div v-if="suggestions.length" class="suggestion-grid mx-auto grid max-w-[600px] grid-cols-2 gap-3 max-[768px]:grid-cols-1">
              <div v-for="s in suggestions" :key="s" class="suggestion-card cursor-pointer rounded-[10px] border border-line bg-panel px-4 py-3.5 text-left text-[13px] text-ink-2 transition-all duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:-translate-y-px hover:border-ui-400 hover:bg-ui-50 hover:text-ui-600 hover:shadow-[0_4px_12px_rgba(13,148,136,0.1)]" @click="emit('suggest', s)">
                {{ s }}
              </div>
            </div>

            <slot name="welcome-bottom" />

            <div v-if="!store.isLoggedIn" class="guest-hint mt-8 text-[13px] text-ink-3">
              您当前以游客身份使用，<a href="javascript:void(0)" class="font-semibold text-ui-600 no-underline hover:underline" @click="goLogin">登录</a> 后可保存对话历史
            </div>
          </div>

          <!-- 消息列表（安全渲染单点：助手内容统一 markstream escape） -->
          <div v-for="msg in store.messages" :key="msg.id" class="message-item mb-6 flex gap-3" :class="[msg.role, msg.role === 'user' ? 'flex-row-reverse' : '']">
            <div class="message-avatar flex h-9 w-9 shrink-0 items-center justify-center rounded-ctl" :class="msg.role === 'user' ? 'bg-ui-600 text-white' : 'bg-ui-100 text-ui-600'">
              <el-icon v-if="msg.role === 'user'" :size="18"><User /></el-icon>
              <el-icon v-else :size="18"><ChatDotRound /></el-icon>
            </div>
            <div class="message-content max-w-[75%] max-[768px]:max-w-[85%]">
              <template v-if="msg.role === 'user'">
                <div v-if="msg.images?.length" class="message-images mb-2 flex flex-wrap justify-end gap-2">
                  <el-image
                    v-for="(img, i) in msg.images"
                    :key="i"
                    :src="img"
                    :preview-src-list="msg.images"
                    :initial-index="i"
                    fit="cover"
                    class="message-image-thumb h-[120px] w-[120px] cursor-pointer rounded-ctl border border-line"
                    preview-teleported
                  />
                </div>
                <div v-if="msg.content" class="message-text break-words rounded-card bg-ui-600 px-4 py-3 text-sm leading-[1.7] text-white">{{ msg.content }}</div>
              </template>
              <div v-else class="message-text markstream-vue break-words rounded-card border border-line bg-panel px-4 py-3 text-sm leading-[1.7] text-ink">
                <MarkdownRender
                  mode="chat"
                  :content="msg.content"
                  :final="true"
                  html-policy="escape"
                  :fade="false"
                />
              </div>
            </div>
          </div>

          <!-- 流式输出中 -->
          <div v-if="store.streaming" class="message-item assistant mb-6 flex gap-3">
            <div class="message-avatar flex h-9 w-9 shrink-0 items-center justify-center rounded-ctl bg-ui-100 text-ui-600">
              <el-icon :size="18"><ChatDotRound /></el-icon>
            </div>
            <div class="message-content max-w-[75%] max-[768px]:max-w-[85%]">
              <div v-if="store.streamingContent" class="message-text markstream-vue break-words rounded-card border border-line bg-panel px-4 py-3 text-sm leading-[1.7] text-ink">
                <MarkdownRender
                  mode="chat"
                  :content="store.streamingContent"
                  :final="!store.streaming"
                  html-policy="escape"
                  :fade="false"
                />
              </div>
              <div v-else class="message-loading flex gap-1 py-1">
                <span class="loading-dot h-2 w-2 rounded-full bg-ui-400"></span>
                <span class="loading-dot h-2 w-2 rounded-full bg-ui-400"></span>
                <span class="loading-dot h-2 w-2 rounded-full bg-ui-400"></span>
              </div>
            </div>
          </div>
        </div>

        <!-- 输入区 -->
        <div class="chat-input-area mx-auto w-full max-w-[900px] bg-canvas px-6 pb-5 pt-3 max-[768px]:px-3 max-[768px]:pb-3 max-[768px]:pt-2">
          <slot name="input-above" />
          <div class="input-wrap flex items-center gap-2 rounded-card border border-line bg-panel px-3 py-2 transition-colors duration-[var(--duration-fast)] ease-[var(--ease-default)] focus-within:border-ui-400" :class="[raisedInput ? 'input-wrap--raised min-h-[132px] flex-col items-stretch p-3 gap-3' : '', !raisedInput && !!slots['input-prefix'] ? 'has-image items-end' : '']">
            <slot v-if="!raisedInput" name="input-prefix" />
            <el-input
              :model-value="inputText"
              type="textarea"
              :rows="raisedInput ? 4 : 2"
              :autosize="raisedInput ? { minRows: 3, maxRows: 8 } : { minRows: 1, maxRows: 6 }"
              :placeholder="inputPlaceholder"
              resize="none"
              @keydown.enter="handleEnter"
              @update:model-value="emit('update:inputText', $event as string)"
              :disabled="store.streaming"
            />
            <div v-if="raisedInput" class="input-footer flex items-center justify-between gap-3">
              <div class="mode-selector flex items-center gap-2">
                <slot name="input-footer-left" />
              </div>
              <div class="input-actions shrink-0">
                <UiButton variant="primary" v-if="!store.streaming" :icon="Promotion" :disabled="!canSend" @click="emit('send')">
                  发送
                </UiButton>
                <UiButton variant="danger" v-else :icon="VideoPause" @click="store.stopStreaming">
                  停止
                </UiButton>
              </div>
            </div>
            <div v-else class="input-actions shrink-0">
              <UiButton variant="primary" v-if="!store.streaming" :icon="Promotion" :disabled="!canSend" @click="emit('send')">
                发送
              </UiButton>
              <UiButton variant="danger" v-else :icon="VideoPause" @click="store.stopStreaming">
                停止
              </UiButton>
            </div>
          </div>
          <slot name="input-extra" />
        </div>
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
// AI 聊天壳（#398）：顶部栏/会话侧栏/消息列表/输入区/自动滚底为壳实现，
// 差异槽位化——主页模式选择器（input-footer-left）与模型告警（input-extra）、
// 功能页图片队列（input-above/input-prefix）与快捷选项（welcome）。
// 安全渲染单点：助手内容统一 markstream-vue + html-policy="escape"，AI 域不再有裸 v-html。
import { ref, computed, useSlots, watch, nextTick, type Component } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Plus,
  Delete,
  Edit,
  ArrowDown,
  User,
  ChatDotRound,
  Promotion,
  VideoPause
} from '@element-plus/icons-vue'
import MarkdownRender from 'markstream-vue'
import 'markstream-vue/index.css'
import { useAIAssistantStore } from '@/stores/aiAssistant'
import { useAuthStore } from '@/stores/auth'
import { authApi } from '@/api/auth'
import { buildSubdomainUrl } from '@/utils/subdomain'
import { formatShortDateTime } from '@/utils/format'
import UiButton from '@/components/ui/UiButton.vue'

const props = withDefaults(
  defineProps<{
    /** 顶部栏副标题（主页：AI 叉车助手 · HRWAI；功能页：AI 叉车助手 · 功能名） */
    logoSub: string
    /** 输入框内容（v-model:input-text，状态由页面持有） */
    inputText: string
    /** 登录跳转目标（登录成功后回跳） */
    loginRedirect: string
    /** 顶部栏附加返回链接（功能页：返回 AI 助手） */
    backLinkTo?: string
    backLinkText?: string
    /** 欢迎区图标/标题/描述 */
    welcomeIcon: Component
    welcomeTitle: string
    welcomeDesc: string
    /** 预设提示词（点击后 emit('suggest', text)） */
    suggestions?: string[]
    /** 会话重命名（主页专用能力） */
    enableRename?: boolean
    /** 输入区拉高布局（主页：模式选择器 footer） */
    raisedInput?: boolean
    inputPlaceholder?: string
    /** 发送按钮可用性（流式更新时由壳切换为停止按钮） */
    canSend?: boolean
  }>(),
  {
    backLinkTo: '',
    backLinkText: '',
    suggestions: () => [],
    enableRename: false,
    raisedInput: false,
    inputPlaceholder: '输入您的问题...（Enter 发送，Shift+Enter 换行）',
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
const authStore = useAuthStore()
const router = useRouter()

const homeUrl = buildSubdomainUrl('main', '/')
// 运行时绑定（避免测试环境对静态资源 URL 的编译期解析）
const logoSrc = '/images/HRWAIlogo.jpg'
const displayName = computed(() => authStore.userInfo?.username || 'HRWAI 用户')
const messageListRef = ref<HTMLElement>()

// ===== 会话重命名（主页 enableRename 时启用） =====
const editingSessionId = ref<number | null>(null)
const editingTitle = ref('')
const editInputRef = ref<any>(null)
let renamingLock = false // 防止 blur + enter 重复触发

// 选中会话：若正在编辑当前会话则不切换
function handleSelectSession(id: number) {
  if (editingSessionId.value === id) return
  store.selectSession(id)
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
    await ElMessageBox.confirm('确定删除该会话？所有消息将一并删除。', '确认', { type: 'warning' })
  } catch {
    return
  }
  await store.deleteSession(id)
}

// ===== 登录/退出 =====
function goLogin() {
  router.push({ path: '/login', query: { redirect: props.loginRedirect } })
}

async function handleUserCommand(cmd: string) {
  if (cmd === 'logout') {
    try {
      await authApi.logout()
    } catch {
      // 忽略后端错误
    }
    authStore.clearAuthData()
    store.messages = []
    store.currentSessionId = null
    store.sessions = []
    ElMessage.success('已退出登录')
  }
}

// ===== 输入与发送 =====
function handleEnter(e: KeyboardEvent) {
  if (e.shiftKey) return
  e.preventDefault()
  emit('send')
}

// ===== 自动滚底（拖底） =====
async function scrollToBottom() {
  await nextTick()
  if (messageListRef.value) {
    messageListRef.value.scrollTop = messageListRef.value.scrollHeight
  }
}

watch(() => store.messages.length, () => scrollToBottom())
watch(() => store.streamingContent, () => scrollToBottom(), { flush: 'post' })
</script>

<style scoped>
/* R1 允许保留：:deep(EP 内部)、keyframes 与 nth-child 动画延迟。其余样式已全部原子化。
   语义类名（ai-chat-shell/topbar/session-item/welcome-area 等）全部保留在模板上，
   是 chat-page-shell.spec.ts 的 DOM 定位钩子，不得删除。 */

.input-wrap--raised :deep(.el-textarea) {
  flex: 1;
}

.input-wrap :deep(.el-textarea__inner) {
  border: none;
  background: transparent;
  padding: 0;
  box-shadow: none !important;
  font-size: 14px;
  line-height: 1.6;
}

.loading-dot {
  animation: dot-bounce 1.4s infinite ease-in-out;
}

.loading-dot:nth-child(2) { animation-delay: 0.2s; }
.loading-dot:nth-child(3) { animation-delay: 0.4s; }

@keyframes dot-bounce {
  0%, 80%, 100% { transform: scale(0.6); opacity: 0.4; }
  40% { transform: scale(1); opacity: 1; }
}
</style>
