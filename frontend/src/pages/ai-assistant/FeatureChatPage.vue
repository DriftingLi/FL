<template>
  <div class="feature-chat-page">
    <!-- 顶部栏 -->
    <header class="topbar">
      <div class="topbar-container">
        <a :href="homeUrl" class="logo-link">
          <img src="/images/HRWAIlogo.jpg" alt="和润天下" class="logo-img" />
          <div class="logo-text-wrap">
            <span class="logo-text">和润天下</span>
            <span class="logo-sub">AI 叉车助手 · {{ feature?.title }}</span>
          </div>
        </a>

        <div class="topbar-actions">
          <router-link to="/ai-assistant" class="back-link">返回 AI 助手</router-link>

          <!-- 未登录：显示登录按钮 -->
          <el-button v-if="!isLoggedIn" type="primary" size="default" @click="goLogin">
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
            :class="{ active: store.currentSessionId === s.id }"
            @click="store.selectSession(s.id)"
          >
            <div class="session-info">
              <span class="session-title">{{ s.title || '新会话' }}</span>
              <span class="session-time">{{ formatShortDateTime(s.updated_at) }}</span>
            </div>
            <button class="session-delete" title="删除" @click.stop="handleDeleteSession(s.id)">
              <el-icon :size="14"><Delete /></el-icon>
            </button>
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
              <el-icon :size="36"><component :is="feature?.icon" /></el-icon>
            </div>
            <h2 class="welcome-title">{{ feature?.title }}</h2>
            <p class="welcome-desc">{{ feature?.welcome }}</p>

            <!-- 预设选项 -->
            <div v-if="feature?.quickOptions?.length" class="quick-options-area">
              <div v-for="group in feature.quickOptions" :key="group.label" class="quick-option-group">
                <span class="quick-option-label">{{ group.label }}</span>
                <div class="quick-option-chips">
                  <button
                    v-for="opt in group.options"
                    :key="opt"
                    class="quick-option-chip"
                    :class="{ active: selectedOptions[group.label] === opt }"
                    @click="toggleOption(group.label, opt)"
                  >
                    {{ opt }}
                  </button>
                </div>
              </div>
            </div>

            <!-- 预设提示词 -->
            <div class="suggestion-grid">
              <div
                v-for="s in feature?.suggestions || []"
                :key="s"
                class="suggestion-card"
                @click="useSuggestion(s)"
              >
                {{ s }}
              </div>
            </div>
            <div v-if="!isLoggedIn" class="guest-hint">
              您当前以游客身份使用，<a href="javascript:void(0)" @click="goLogin">登录</a> 后可保存对话历史
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
              <!-- 用户消息：图片缩略图 + 文本 -->
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
          <!-- 待发送图片 -->
          <div v-if="pendingImages.length" class="pending-images">
            <div v-for="(p, i) in pendingImages" :key="p.url" class="pending-image-item">
              <img :src="p.previewUrl" class="pending-image-thumb" alt="待发送图片" />
              <button class="pending-image-remove" title="移除" @click="removePendingImage(i)">
                <el-icon :size="12"><Close /></el-icon>
              </button>
              <div v-if="p.uploading" class="pending-image-mask">上传中</div>
            </div>
          </div>
          <div class="input-wrap" :class="{ 'has-image': supportsImage }">
            <el-upload
              v-if="supportsImage"
              :show-file-list="false"
              :auto-upload="false"
              accept="image/png,image/jpeg,image/gif,image/webp,image/bmp,image/svg+xml"
              :disabled="store.streaming"
              multiple
              @change="handleImageSelect"
            >
              <el-button :icon="Picture" circle :disabled="store.streaming || pendingImages.length >= maxImages" title="上传图片" />
            </el-upload>
            <el-input
              v-model="inputText"
              type="textarea"
              :rows="2"
              :autosize="{ minRows: 1, maxRows: 6 }"
              :placeholder="supportsImage ? '输入问题或上传图纸/习题图片...（Enter 发送，Shift+Enter 换行）' : '输入您的问题...（Enter 发送，Shift+Enter 换行）'"
              resize="none"
              @keydown.enter="handleEnter"
              :disabled="store.streaming"
            />
            <div class="input-actions">
              <el-button
                v-if="!store.streaming"
                type="primary"
                :icon="Promotion"
                :disabled="!canSend"
                @click="handleSend"
              >
                发送
              </el-button>
              <el-button v-else type="danger" :icon="VideoPause" @click="store.stopStreaming">
                停止
              </el-button>
            </div>
          </div>
        </div>
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { UploadFile } from 'element-plus'
import {
  Plus,
  Delete,
  ArrowDown,
  User,
  ChatDotRound,
  Promotion,
  VideoPause,
  Picture,
  Close
} from '@element-plus/icons-vue'
import { marked } from 'marked'
import { useAIAssistantStore } from '@/stores/aiAssistant'
import { useAuthStore } from '@/stores/auth'
import { authApi } from '@/api/auth'
import { buildSubdomainUrl } from '@/utils/subdomain'
import { formatShortDateTime } from '@/utils/format'
import { getAIFeature } from '@/config/aiFeatures'
import '@/assets/styles/markdown.css'

const store = useAIAssistantStore()
const authStore = useAuthStore()
const router = useRouter()
const route = useRoute()

const homeUrl = buildSubdomainUrl('main', '/')
const isLoggedIn = computed(() => store.isLoggedIn)
const displayName = computed(() => {
  const info = authStore.userInfo
  return info?.username || 'HRWAI 用户'
})

const feature = computed(() => getAIFeature(route.params.featureKey as string))
const supportsImage = computed(() => feature.value?.supportsImage === true)
const maxImages = computed(() => feature.value?.maxImages ?? 4)

const inputText = ref('')
const messageListRef = ref<HTMLElement>()

interface PendingImage {
  url: string          // 上传成功后的服务器 URL
  previewUrl: string   // 本地 blob 预览
  uploading: boolean
}
const pendingImages = ref<PendingImage[]>([])

const canSend = computed(() =>
  (!inputText.value.trim() && pendingImages.value.length === 0)
    ? false
    : !pendingImages.value.some(p => p.uploading)
)

const selectedOptions = ref<Record<string, string>>({})

function goLogin() {
  router.push({ path: '/login', query: { redirect: route.fullPath } })
}

function toggleOption(label: string, opt: string) {
  if (selectedOptions.value[label] === opt) {
    delete selectedOptions.value[label]
  } else {
    selectedOptions.value[label] = opt
  }
}

// 组装消息内容：预设选项作为前缀注入
function buildContent(text: string): string {
  const groups = feature.value?.quickOptions || []
  const tags = groups
    .map(g => (selectedOptions.value[g.label] ? `[${g.label}：${selectedOptions.value[g.label]}]` : ''))
    .filter(Boolean)
  if (tags.length === 0) return text
  return tags.join(' ') + '\n\n' + text
}

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
  const images = pendingImages.value.map(p => p.url)
  if (!text && images.length === 0) return
  if (pendingImages.value.some(p => p.uploading)) {
    ElMessage.warning('图片上传中，请稍候')
    return
  }
  if (store.streaming) return

  inputText.value = ''
  pendingImages.value = []
  try {
    await store.sendMessage(buildContent(text), images.length > 0 ? images : undefined)
    await scrollToBottom()
  } catch (e: any) {
    // 错误已由 store 处理
  }
}

function useSuggestion(text: string) {
  inputText.value = text
  handleSend()
}

// 图片选择：立即上传，本地 blob 先行预览
async function handleImageSelect(file: UploadFile) {
  const raw = file.raw
  if (!raw) return
  if (pendingImages.value.length >= maxImages.value) {
    ElMessage.warning(`最多上传 ${maxImages.value} 张图片`)
    return
  }
  const previewUrl = URL.createObjectURL(raw)
  const pending: PendingImage = { url: '', previewUrl, uploading: true }
  pendingImages.value.push(pending)
  try {
    pending.url = await store.uploadImage(raw)
  } catch (e: any) {
    ElMessage.error(e?.message || '图片上传失败')
    pendingImages.value = pendingImages.value.filter(p => p !== pending)
    URL.revokeObjectURL(previewUrl)
  } finally {
    pending.uploading = false
  }
}

function removePendingImage(index: number) {
  const removed = pendingImages.value[index]
  if (removed) {
    URL.revokeObjectURL(removed.previewUrl)
  }
  pendingImages.value.splice(index, 1)
}

async function handleNewSession() {
  await store.createSession('新会话')
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
    ElMessage.success('已退出登录')
  }
}

// 监听消息变化自动滚动
watch(() => store.messages.length, () => scrollToBottom())
watch(() => store.streamingContent, () => scrollToBottom(), { flush: 'post' })

onMounted(() => {
  if (feature.value) {
    store.initFeature(feature.value.key)
  } else {
    router.replace('/ai-assistant')
  }
})
</script>

<style scoped>
.feature-chat-page {
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

.profile-link {
  font-size: 14px;
  font-weight: 500;
  color: var(--color-primary-600, #2563eb);
  text-decoration: none;
  transition: opacity 0.15s ease;
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
  flex-shrink: 0;
}

.session-item:hover .session-delete {
  opacity: 1;
}

.session-delete:hover {
  color: #ef4444;
  background: #fef2f2;
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
  padding: 40px 24px;
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

/* ===== 预设选项 ===== */
.quick-options-area {
  display: flex;
  flex-direction: column;
  gap: 10px;
  align-items: center;
  margin-bottom: 24px;
}

.quick-option-group {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  justify-content: center;
}

.quick-option-label {
  font-size: 13px;
  color: var(--color-text-secondary, #475569);
  font-weight: 600;
}

.quick-option-chips {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: center;
}

.quick-option-chip {
  padding: 4px 14px;
  border-radius: 999px;
  border: 1px solid var(--color-border-light, #e2e8f0);
  background: var(--color-surface, #fff);
  color: var(--color-text-secondary, #475569);
  font-size: 13px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.quick-option-chip:hover {
  border-color: var(--color-brand-400, #38bdf8);
  color: var(--color-brand-600, #0284c7);
}

.quick-option-chip.active {
  border-color: var(--color-brand-600, #0284c7);
  background: var(--color-brand-50, #f0f9ff);
  color: var(--color-brand-600, #0284c7);
  font-weight: 600;
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

.pending-images {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 8px;
}

.pending-image-item {
  position: relative;
  width: 64px;
  height: 64px;
}

.pending-image-thumb {
  width: 64px;
  height: 64px;
  object-fit: cover;
  border-radius: 8px;
  border: 1px solid var(--color-border-light, #e2e8f0);
}

.pending-image-remove {
  position: absolute;
  top: -6px;
  right: -6px;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  border: none;
  background: rgba(15, 23, 42, 0.7);
  color: #fff;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
}

.pending-image-remove:hover {
  background: #ef4444;
}

.pending-image-mask {
  position: absolute;
  inset: 0;
  border-radius: 8px;
  background: rgba(15, 23, 42, 0.5);
  color: #fff;
  font-size: 11px;
  display: flex;
  align-items: center;
  justify-content: center;
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

.input-wrap.has-image {
  align-items: flex-end;
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
