<template>
  <div class="ai-chat-shell">
    <!-- 顶部栏 -->
    <header class="topbar">
      <div class="topbar-container">
        <a :href="homeUrl" class="logo-link">
          <img :src="logoSrc" alt="和润天下" class="logo-img" />
          <div class="logo-text-wrap">
            <span class="logo-text">和润天下</span>
            <span class="logo-sub">{{ logoSub }}</span>
          </div>
        </a>

        <div class="topbar-actions">
          <router-link v-if="backLinkTo" :to="backLinkTo" class="back-link">{{ backLinkText }}</router-link>

          <!-- 未登录：显示登录按钮 -->
          <el-button v-if="!store.isLoggedIn" type="primary" size="default" @click="goLogin">
            登录 HRWAI 账号
          </el-button>

          <!-- 已登录：显示用户名 + 退出 -->
          <template v-else>
            <router-link to="/training/profile" class="profile-link">个人资料</router-link>
            <el-dropdown trigger="click" @command="handleUserCommand">
              <span class="user-trigger">
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

          <a :href="homeUrl" class="back-link">返回官网</a>
        </div>
      </div>
    </header>

    <!-- 主体：侧栏 + 对话区 -->
    <div class="main-body">
      <!-- 左侧会话栏（仅登录后显示） -->
      <aside v-if="store.isLoggedIn" class="session-sidebar">
        <div class="sidebar-header">
          <span class="sidebar-title">会话历史</span>
          <el-button type="primary" size="small" :icon="Plus" @click="emit('new-session')">新建</el-button>
        </div>
        <div v-loading="store.sessionsLoading" class="session-list">
          <div v-if="store.sessions.length === 0 && !store.sessionsLoading" class="empty-sessions">
            暂无会话，点击"新建"开始对话
          </div>
          <div
            v-for="s in store.sessions"
            :key="s.id"
            class="session-item"
            :class="{ active: store.currentSessionId === s.id, editing: enableRename && editingSessionId === s.id }"
            @click="handleSelectSession(s.id)"
          >
            <div class="session-info">
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
                <span class="session-title" @dblclick.stop="startRename(s)">{{ s.title || '新会话' }}</span>
                <span class="session-time">{{ formatShortDateTime(s.updated_at) }}</span>
              </template>
            </div>
            <div v-if="editingSessionId !== s.id" class="session-actions">
              <button v-if="enableRename" class="session-rename" title="重命名" @click.stop="startRename(s)">
                <el-icon :size="14"><Edit /></el-icon>
              </button>
              <button class="session-delete" title="删除" @click.stop="handleDeleteSession(s.id)">
                <el-icon :size="14"><Delete /></el-icon>
              </button>
            </div>
          </div>
        </div>
      </aside>

      <!-- 右侧对话区 -->
      <main class="chat-main">
        <!-- 消息列表 -->
        <div ref="messageListRef" class="message-list">
          <!-- 空状态：欢迎区（差异内容走 welcome 槽位） -->
          <div v-if="store.messages.length === 0 && !store.streaming" class="welcome-area">
            <div class="welcome-icon">
              <el-icon :size="36"><component :is="welcomeIcon" /></el-icon>
            </div>
            <h2 class="welcome-title">{{ welcomeTitle }}</h2>
            <p class="welcome-desc">{{ welcomeDesc }}</p>

            <slot name="welcome-top" />

            <!-- 预设提示词（两页共用实现） -->
            <div v-if="suggestions.length" class="suggestion-grid">
              <div v-for="s in suggestions" :key="s" class="suggestion-card" @click="emit('suggest', s)">
                {{ s }}
              </div>
            </div>

            <slot name="welcome-bottom" />

            <div v-if="!store.isLoggedIn" class="guest-hint">
              您当前以游客身份使用，<a href="javascript:void(0)" @click="goLogin">登录</a> 后可保存对话历史
            </div>
          </div>

          <!-- 消息列表（安全渲染单点：助手内容统一 markstream escape） -->
          <div v-for="msg in store.messages" :key="msg.id" class="message-item" :class="msg.role">
            <div class="message-avatar">
              <el-icon v-if="msg.role === 'user'" :size="18"><User /></el-icon>
              <el-icon v-else :size="18"><ChatDotRound /></el-icon>
            </div>
            <div class="message-content">
              <template v-if="msg.role === 'user'">
                <div v-if="msg.images?.length" class="message-images">
                  <el-image
                    v-for="(img, i) in msg.images"
                    :key="i"
                    :src="img"
                    :preview-src-list="msg.images"
                    :initial-index="i"
                    fit="cover"
                    class="message-image-thumb"
                    preview-teleported
                  />
                </div>
                <div v-if="msg.content" class="message-text">{{ msg.content }}</div>
              </template>
              <div v-else class="message-text markstream-vue">
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
          <div v-if="store.streaming" class="message-item assistant">
            <div class="message-avatar">
              <el-icon :size="18"><ChatDotRound /></el-icon>
            </div>
            <div class="message-content">
              <div v-if="store.streamingContent" class="message-text markstream-vue">
                <MarkdownRender
                  mode="chat"
                  :content="store.streamingContent"
                  :final="!store.streaming"
                  html-policy="escape"
                  :fade="false"
                />
              </div>
              <div v-else class="message-loading">
                <span class="loading-dot"></span>
                <span class="loading-dot"></span>
                <span class="loading-dot"></span>
              </div>
            </div>
          </div>
        </div>

        <!-- 输入区 -->
        <div class="chat-input-area">
          <slot name="input-above" />
          <div class="input-wrap" :class="{ 'input-wrap--raised': raisedInput, 'has-image': !raisedInput && !!slots['input-prefix'] }">
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
            <div v-if="raisedInput" class="input-footer">
              <div class="mode-selector">
                <slot name="input-footer-left" />
              </div>
              <div class="input-actions">
                <el-button
                  v-if="!store.streaming"
                  type="primary"
                  :icon="Promotion"
                  :disabled="!canSend"
                  @click="emit('send')"
                >
                  发送
                </el-button>
                <el-button v-else type="danger" :icon="VideoPause" @click="store.stopStreaming">
                  停止
                </el-button>
              </div>
            </div>
            <div v-else class="input-actions">
              <el-button
                v-if="!store.streaming"
                type="primary"
                :icon="Promotion"
                :disabled="!canSend"
                @click="emit('send')"
              >
                发送
              </el-button>
              <el-button v-else type="danger" :icon="VideoPause" @click="store.stopStreaming">
                停止
              </el-button>
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
.ai-chat-shell {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background: var(--color-bg-page, #f8fafc);
}

/* ===== 顶部栏 ===== */
.topbar {
  background: var(--color-surface, #fff);
  border-bottom: 1px solid var(--color-border, #e2e8f0);
  flex-shrink: 0;
}

.topbar-container {
  max-width: 1600px;
  margin: 0 auto;
  padding: 0 var(--space-6, 24px);
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 64px;
  gap: var(--space-4, 16px);
}

.logo-link {
  display: flex;
  align-items: center;
  gap: var(--space-3, 12px);
  text-decoration: none;
  flex-shrink: 0;
}

.logo-img {
  width: 36px;
  height: 36px;
  border-radius: 8px;
  object-fit: cover;
}

.logo-text-wrap {
  display: flex;
  flex-direction: column;
  line-height: 1.1;
}

.logo-text {
  font-size: 16px;
  font-weight: 700;
  color: var(--color-text-primary, #0f172a);
}

.logo-sub {
  font-size: 11px;
  color: var(--color-text-tertiary, #94a3b8);
  letter-spacing: 0.1em;
  margin-top: 2px;
}

.topbar-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}

.user-trigger {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  cursor: pointer;
  font-size: 14px;
  font-weight: 500;
  color: var(--color-text-primary, #0f172a);
  padding: 6px 12px;
  border-radius: 8px;
  transition: background var(--duration-fast) var(--ease-default);
}

.user-trigger:hover {
  background: var(--color-bg-page, #f8fafc);
}

.profile-link {
  font-size: 14px;
  font-weight: 500;
  color: var(--color-primary-600, #2563eb);
  text-decoration: none;
  transition: opacity var(--duration-fast) var(--ease-default);
}

.profile-link:hover {
  opacity: 0.8;
  text-decoration: underline;
}

.back-link {
  font-size: 13px;
  color: var(--color-text-secondary, #475569);
  text-decoration: none;
  padding: 6px 12px;
  border: 1px solid var(--color-border-dark, #cbd5e1);
  border-radius: 8px;
  transition: all var(--duration-fast) var(--ease-default);
}

.back-link:hover {
  border-color: var(--color-primary-500);
  color: var(--color-primary-600);
}

/* ===== 主体布局 ===== */
.main-body {
  flex: 1;
  display: flex;
  overflow: hidden;
}

/* ===== 会话侧栏 ===== */
.session-sidebar {
  width: 260px;
  background: var(--color-surface, #fff);
  border-right: 1px solid var(--color-border-light, #e2e8f0);
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
}

.sidebar-header {
  padding: 12px 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--color-border-light, #e2e8f0);
}

.sidebar-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-secondary, #475569);
}

.session-list {
  flex: 1;
  overflow-y: auto;
  padding: 8px;
}

.empty-sessions {
  text-align: center;
  color: var(--color-text-tertiary, #94a3b8);
  font-size: 13px;
  padding: 32px 12px;
}

.session-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  border-radius: 8px;
  cursor: pointer;
  transition: background var(--duration-fast) var(--ease-default);
  margin-bottom: 4px;
}

.session-item:hover {
  background: var(--color-bg-page, #f8fafc);
}

.session-item.active {
  background: var(--color-primary-50);
}

.session-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.session-title {
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text-primary, #0f172a);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.session-time {
  font-size: 11px;
  color: var(--color-text-tertiary, #94a3b8);
}

.session-delete {
  border: none;
  background: transparent;
  color: var(--color-text-tertiary, #94a3b8);
  cursor: pointer;
  padding: 4px;
  border-radius: 4px;
  opacity: 0;
  transition: all var(--duration-fast) var(--ease-default);
  flex-shrink: 0;
}

.session-rename {
  border: none;
  background: transparent;
  color: var(--color-text-tertiary, #94a3b8);
  cursor: pointer;
  padding: 4px;
  border-radius: 4px;
  opacity: 0;
  transition: all var(--duration-fast) var(--ease-default);
}

.session-actions {
  display: flex;
  gap: 2px;
  opacity: 1;
}

.session-item:hover .session-rename,
.session-item:hover .session-delete {
  opacity: 1;
}

.session-rename:hover {
  color: var(--color-primary-600);
  background: var(--color-primary-50);
}

.session-delete:hover {
  color: #ef4444;
  background: #fef2f2;
}

.session-item.editing {
  background: var(--color-primary-50);
  cursor: default;
}

.session-item.editing .session-info {
  gap: 4px;
}

/* ===== 对话主区 ===== */
.chat-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--color-bg-page, #f8fafc);
}

.message-list {
  flex: 1;
  overflow-y: auto;
  padding: 24px;
  max-width: 900px;
  margin: 0 auto;
  width: 100%;
}

/* ===== 欢迎区 ===== */
.welcome-area {
  text-align: center;
  padding: 48px 24px;
}

.welcome-icon {
  width: 72px;
  height: 72px;
  border-radius: 20px;
  background: linear-gradient(135deg, #6366f1, #8b5cf6);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: white;
  margin-bottom: 20px;
}

.welcome-title {
  font-size: 24px;
  font-weight: 700;
  color: var(--color-text-primary, #0f172a);
  margin: 0 0 8px;
}

.welcome-desc {
  font-size: 14px;
  color: var(--color-text-tertiary, #94a3b8);
  line-height: 1.6;
  max-width: 560px;
  margin: 0 auto 24px;
}

/* ===== 预设提示词 ===== */
.suggestion-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
  max-width: 600px;
  margin: 0 auto;
}

.suggestion-card {
  padding: 14px 16px;
  background: var(--color-surface, #fff);
  border: 1px solid var(--color-border-light, #e2e8f0);
  border-radius: 10px;
  font-size: 13px;
  color: var(--color-text-secondary, #475569);
  cursor: pointer;
  text-align: left;
  transition: all var(--duration-fast) var(--ease-default);
}

.suggestion-card:hover {
  border-color: var(--color-primary-400);
  background: var(--color-primary-50);
  color: var(--color-primary-600);
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(13, 148, 136, 0.1);
}

.guest-hint {
  margin-top: 32px;
  font-size: 13px;
  color: var(--color-text-tertiary, #94a3b8);
}

.guest-hint a {
  color: var(--color-primary-600);
  font-weight: 600;
  text-decoration: none;
}

.guest-hint a:hover {
  text-decoration: underline;
}

/* ===== 消息项 ===== */
.message-item {
  display: flex;
  gap: 12px;
  margin-bottom: 24px;
}

.message-item.user {
  flex-direction: row-reverse;
}

.message-avatar {
  width: 36px;
  height: 36px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  background: var(--color-primary-100);
  color: var(--color-primary-600);
}

.message-item.user .message-avatar {
  background: var(--color-primary-600);
  color: white;
}

.message-content {
  max-width: 75%;
}

.message-text {
  padding: 12px 16px;
  border-radius: 12px;
  font-size: 14px;
  line-height: 1.7;
  word-break: break-word;
}

.message-item.assistant .message-text {
  background: var(--color-surface, #fff);
  border: 1px solid var(--color-border-light, #e2e8f0);
  color: var(--color-text-primary, #0f172a);
}

.message-item.user .message-text {
  background: var(--color-primary-600);
  color: white;
}

/* 用户消息图片 */
.message-images {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
  margin-bottom: 8px;
}

.message-image-thumb {
  width: 120px;
  height: 120px;
  border-radius: 8px;
  border: 1px solid var(--color-border-light, #e2e8f0);
  cursor: pointer;
}

/* ===== 加载动画 ===== */
.message-loading {
  display: flex;
  gap: 4px;
  padding: 4px 0;
}

.loading-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--color-primary-400);
  animation: dot-bounce 1.4s infinite ease-in-out;
}

.loading-dot:nth-child(2) { animation-delay: 0.2s; }
.loading-dot:nth-child(3) { animation-delay: 0.4s; }

@keyframes dot-bounce {
  0%, 80%, 100% { transform: scale(0.6); opacity: 0.4; }
  40% { transform: scale(1); opacity: 1; }
}

/* ===== 输入区 ===== */
.chat-input-area {
  padding: 12px 24px 20px;
  background: var(--color-bg-page, #f8fafc);
  max-width: 900px;
  margin: 0 auto;
  width: 100%;
}

.input-wrap {
  background: var(--color-surface, #fff);
  border: 1px solid var(--color-border-light, #e2e8f0);
  border-radius: 12px;
  padding: 8px 12px;
  display: flex;
  gap: 8px;
  align-items: center;
  transition: border-color var(--duration-fast) var(--ease-default);
}

.input-wrap.has-image {
  align-items: flex-end;
}

.input-wrap--raised {
  flex-direction: column;
  align-items: stretch;
  min-height: 132px;
  padding: 12px;
  gap: 12px;
}

.input-wrap--raised :deep(.el-textarea) {
  flex: 1;
}

.input-wrap:focus-within {
  border-color: var(--color-primary-400);
}

.input-wrap :deep(.el-textarea__inner) {
  border: none;
  background: transparent;
  padding: 0;
  box-shadow: none !important;
  font-size: 14px;
  line-height: 1.6;
}

.input-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.mode-selector {
  display: flex;
  align-items: center;
  gap: 8px;
}

.input-actions {
  flex-shrink: 0;
}

/* ===== 响应式 ===== */
@media (max-width: 768px) {
  .topbar-container {
    padding: 0 12px;
    height: 56px;
  }

  .logo-text {
    font-size: 14px;
  }

  .logo-sub {
    display: none;
  }

  .topbar-actions {
    gap: 8px;
  }

  .back-link {
    display: none;
  }

  .session-sidebar {
    display: none;
  }

  .message-list {
    padding: 16px;
  }

  .message-content {
    max-width: 85%;
  }

  .suggestion-grid {
    grid-template-columns: 1fr;
  }

  .chat-input-area {
    padding: 8px 12px 12px;
  }
}
</style>
