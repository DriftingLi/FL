<template>
  <div class="ai-assistant-page">
    <!-- 顶部栏 -->
    <header class="topbar">
      <div class="topbar-container">
        <a :href="homeUrl" class="logo-link">
          <img src="/images/HRWAIlogo.jpg" alt="和润天下" class="logo-img" />
          <div class="logo-text-wrap">
            <span class="logo-text">和润天下</span>
            <span class="logo-sub">AI 叉车助手 · HRWAI</span>
          </div>
        </a>

        <div class="topbar-actions">
          <ModelSelector
            @manage="userModelDialogVisible = true"
            @custom="customModelDialogVisible = true"
          />

          <!-- 未登录：显示登录按钮 -->
          <el-button v-if="!isLoggedIn" type="primary" size="default" @click="loginDialogVisible = true">
            登录 HRWAI 账号
          </el-button>

          <!-- 已登录：显示用户名 + 退出 -->
          <template v-else>
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
      <aside v-if="isLoggedIn" class="session-sidebar">
        <div class="sidebar-header">
          <span class="sidebar-title">会话历史</span>
          <el-button type="primary" size="small" :icon="Plus" @click="handleNewSession">新建</el-button>
        </div>
        <div v-loading="store.sessionsLoading" class="session-list">
          <div v-if="store.sessions.length === 0 && !store.sessionsLoading" class="empty-sessions">
            暂无会话，点击"新建"开始对话
          </div>
          <div
            v-for="s in store.sessions"
            :key="s.id"
            class="session-item"
            :class="{ active: store.currentSessionId === s.id, editing: editingSessionId === s.id }"
            @click="handleSelectSession(s.id)"
          >
            <div class="session-info">
              <!-- 编辑模式 -->
              <el-input
                v-if="editingSessionId === s.id"
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
                <span class="session-time">{{ formatTime(s.updated_at) }}</span>
              </template>
            </div>
            <div v-if="editingSessionId !== s.id" class="session-actions">
              <button class="session-rename" title="重命名" @click.stop="startRename(s)">
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
          <!-- 空状态 -->
          <div v-if="store.messages.length === 0 && !store.streaming" class="welcome-area">
            <div class="welcome-icon">
              <el-icon :size="36"><ChatDotRound /></el-icon>
            </div>
            <h2 class="welcome-title">叉车维修 AI 助手</h2>
            <p class="welcome-desc">
              我是您的叉车维修专家助手，可以帮您解答叉车选购、维保周期、故障诊断、操作规范等问题。
            </p>
            <div class="suggestion-grid">
              <div class="suggestion-card" @click="useSuggestion('叉车日常检查项目有哪些？')">
                叉车日常检查项目有哪些？
              </div>
              <div class="suggestion-card" @click="useSuggestion('电动叉车电池续航下降怎么排查？')">
                电动叉车电池续航下降怎么排查？
              </div>
              <div class="suggestion-card" @click="useSuggestion('液压系统压力不足的常见原因？')">
                液压系统压力不足的常见原因？
              </div>
              <div class="suggestion-card" @click="useSuggestion('叉车季度保养项目有哪些？')">
                叉车季度保养项目有哪些？
              </div>
            </div>
            <div v-if="!isLoggedIn" class="guest-hint">
              您当前以游客身份使用，<a href="javascript:void(0)" @click="loginDialogVisible = true">登录</a> 后可保存对话历史
            </div>
          </div>

          <!-- 消息列表 -->
          <div
            v-for="msg in store.messages"
            :key="msg.id"
            class="message-item"
            :class="msg.role"
          >
            <div class="message-avatar">
              <el-icon v-if="msg.role === 'user'" :size="18"><User /></el-icon>
              <el-icon v-else :size="18"><ChatDotRound /></el-icon>
            </div>
            <div class="message-content">
              <div v-if="msg.role === 'user'" class="message-text">{{ msg.content }}</div>
              <div v-else class="message-text markdown-body" v-html="renderMarkdown(msg.content)"></div>
            </div>
          </div>

          <!-- 流式输出中 -->
          <div v-if="store.streaming" class="message-item assistant">
            <div class="message-avatar">
              <el-icon :size="18"><ChatDotRound /></el-icon>
            </div>
            <div class="message-content">
              <div v-if="store.streamingContent" class="message-text markdown-body" v-html="renderMarkdown(store.streamingContent)"></div>
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
          <div class="input-wrap">
            <el-input
              v-model="inputText"
              type="textarea"
              :rows="2"
              :autosize="{ minRows: 1, maxRows: 6 }"
              placeholder="输入您的问题...（Enter 发送，Shift+Enter 换行）"
              resize="none"
              @keydown.enter="handleEnter"
              :disabled="store.streaming"
            />
            <div class="input-actions">
              <el-button
                v-if="!store.streaming"
                type="primary"
                :icon="Promotion"
                :disabled="!inputText.trim() || !store.selectedModel"
                @click="handleSend"
              >
                发送
              </el-button>
              <el-button v-else type="danger" :icon="VideoPause" @click="store.stopStreaming">
                停止
              </el-button>
            </div>
          </div>
          <div v-if="!store.selectedModel" class="model-warning">
            请先选择模型
          </div>
        </div>
      </main>
    </div>

    <!-- 对话框 -->
    <LoginDialog v-model="loginDialogVisible" @success="handleLoginSuccess" />
    <UserModelDialog v-model="userModelDialogVisible" />
    <CustomModelDialog v-model="customModelDialogVisible" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
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
import { marked } from 'marked'
import { useAIAssistantStore } from '@/stores/aiAssistant'
import { useAuthStore } from '@/stores/auth'
import { authApi } from '@/api/auth'
import { buildSubdomainUrl } from '@/utils/subdomain'
import ModelSelector from '@/components/ai-assistant/ModelSelector.vue'
import UserModelDialog from '@/components/ai-assistant/UserModelDialog.vue'
import CustomModelDialog from '@/components/ai-assistant/CustomModelDialog.vue'
import LoginDialog from '@/components/ai-assistant/LoginDialog.vue'
import '@/assets/styles/markdown.css'

const store = useAIAssistantStore()
const authStore = useAuthStore()

const homeUrl = buildSubdomainUrl('main', '/')
const isLoggedIn = computed(() => store.isLoggedIn)
const displayName = computed(() => {
  const info = authStore.userInfo
  return info?.name || info?.username || 'HRWAI 用户'
})

const inputText = ref('')
const messageListRef = ref<HTMLElement>()

// 对话框状态
const loginDialogVisible = ref(false)
const userModelDialogVisible = ref(false)
const customModelDialogVisible = ref(false)

// 会话重命名状态
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
    } catch (e: any) {
      ElMessage.error(e.message || '重命名失败')
    } finally {
      cancelRename()
    }
  } finally {
    renamingLock = false
  }
}

// 配置 marked
marked.setOptions({
  breaks: true,
  gfm: true
})

function renderMarkdown(content: string): string {
  if (!content) return ''
  try {
    return marked.parse(content) as string
  } catch {
    return content
  }
}

function formatTime(t: string): string {
  if (!t) return ''
  const d = new Date(t)
  return `${d.getMonth() + 1}/${d.getDate()} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

async function scrollToBottom() {
  await nextTick()
  if (messageListRef.value) {
    messageListRef.value.scrollTop = messageListRef.value.scrollHeight
  }
}

function handleEnter(e: KeyboardEvent) {
  if (e.shiftKey) return
  e.preventDefault()
  handleSend()
}

async function handleSend() {
  const text = inputText.value.trim()
  if (!text) return
  if (!store.selectedModel) {
    ElMessage.warning('请先选择模型')
    return
  }
  if (store.streaming) return

  inputText.value = ''
  try {
    await store.sendMessage(text)
    await scrollToBottom()
  } catch (e: any) {
    // 错误已由 store 处理
  }
}

function useSuggestion(text: string) {
  inputText.value = text
  handleSend()
}

async function handleNewSession() {
  await store.createSession('新会话', store.selectedModel?.label)
}

async function handleDeleteSession(id: number) {
  try {
    await ElMessageBox.confirm('确定删除该会话？所有消息将一并删除。', '确认', { type: 'warning' })
  } catch {
    return
  }
  await store.deleteSession(id)
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
    store.userModels = []
    ElMessage.success('已退出登录')
  }
}

function handleLoginSuccess() {
  // 登录成功后加载会话和用户模型
  store.loadSessions()
  store.loadUserModels()
}

// 监听消息变化自动滚动
watch(() => store.messages.length, () => scrollToBottom())
watch(() => store.streamingContent, () => scrollToBottom(), { flush: 'post' })

onMounted(() => {
  store.init()
})
</script>

<style scoped>
.ai-assistant-page {
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
  transition: background 0.15s ease;
}

.user-trigger:hover {
  background: var(--color-bg-page, #f8fafc);
}

.back-link {
  font-size: 13px;
  color: var(--color-text-secondary, #475569);
  text-decoration: none;
  padding: 6px 12px;
  border: 1px solid var(--color-border-dark, #cbd5e1);
  border-radius: 8px;
  transition: all 0.15s ease;
}

.back-link:hover {
  border-color: var(--color-brand-500, #0ea5e9);
  color: var(--color-brand-600, #0284c7);
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
  transition: background 0.15s ease;
  margin-bottom: 4px;
}

.session-item:hover {
  background: var(--color-bg-page, #f8fafc);
}

.session-item.active {
  background: var(--color-brand-50, #f0f9ff);
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
  transition: all 0.15s ease;
}

.session-rename {
  border: none;
  background: transparent;
  color: var(--color-text-tertiary, #94a3b8);
  cursor: pointer;
  padding: 4px;
  border-radius: 4px;
  opacity: 0;
  transition: all 0.15s ease;
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
  color: var(--color-brand-600, #0284c7);
  background: var(--color-brand-50, #f0f9ff);
}

.session-delete:hover {
  color: #ef4444;
  background: #fef2f2;
}

.session-item.editing {
  background: var(--color-brand-50, #f0f9ff);
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
  max-width: 500px;
  margin: 0 auto 32px;
}

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
  transition: all 0.15s ease;
}

.suggestion-card:hover {
  border-color: var(--color-brand-400, #38bdf8);
  background: var(--color-brand-50, #f0f9ff);
  color: var(--color-brand-600, #0284c7);
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(14, 165, 233, 0.1);
}

.guest-hint {
  margin-top: 32px;
  font-size: 13px;
  color: var(--color-text-tertiary, #94a3b8);
}

.guest-hint a {
  color: var(--color-brand-600, #0284c7);
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
  background: var(--color-brand-100, #e0f2fe);
  color: var(--color-brand-600, #0284c7);
}

.message-item.user .message-avatar {
  background: var(--color-brand-600, #0284c7);
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
  background: var(--color-brand-600, #0284c7);
  color: white;
}

/* markdown 样式 */
.message-item.assistant .markdown-body :deep(h1),
.message-item.assistant .markdown-body :deep(h2),
.message-item.assistant .markdown-body :deep(h3) {
  margin: 16px 0 8px;
  font-weight: 600;
}

.message-item.assistant .markdown-body :deep(h1) { font-size: 18px; }
.message-item.assistant .markdown-body :deep(h2) { font-size: 16px; }
.message-item.assistant .markdown-body :deep(h3) { font-size: 15px; }

.message-item.assistant .markdown-body :deep(p) {
  margin: 8px 0;
}

.message-item.assistant .markdown-body :deep(ul),
.message-item.assistant .markdown-body :deep(ol) {
  margin: 8px 0;
  padding-left: 20px;
}

.message-item.assistant .markdown-body :deep(li) {
  margin: 4px 0;
}

.message-item.assistant .markdown-body :deep(code) {
  background: var(--color-bg-page, #f8fafc);
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 13px;
  font-family: var(--font-mono, monospace);
}

.message-item.assistant .markdown-body :deep(pre) {
  background: var(--color-bg-page, #f8fafc);
  padding: 12px;
  border-radius: 8px;
  overflow-x: auto;
  margin: 8px 0;
}

.message-item.assistant .markdown-body :deep(pre code) {
  background: transparent;
  padding: 0;
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
  background: var(--color-brand-400, #38bdf8);
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
  transition: border-color 0.15s ease;
}

.input-wrap:focus-within {
  border-color: var(--color-brand-400, #38bdf8);
}

.input-wrap :deep(.el-textarea__inner) {
  border: none;
  background: transparent;
  padding: 0;
  box-shadow: none !important;
  font-size: 14px;
  line-height: 1.6;
}

.input-actions {
  flex-shrink: 0;
}

.model-warning {
  text-align: center;
  font-size: 12px;
  color: var(--color-text-tertiary, #f59e0b);
  margin-top: 8px;
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
