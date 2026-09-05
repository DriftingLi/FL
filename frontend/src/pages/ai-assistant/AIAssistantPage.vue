<template>
  <ChatPageShell
    logo-sub="AI 叉车助手 · HRWAI"
    login-redirect="/ai-assistant"
    :welcome-icon="ChatDotRound"
    welcome-title="叉车维修 AI 助手"
    welcome-desc="我是您的叉车维修专家助手，可以帮您解答叉车选购、维保周期、故障诊断、操作规范等问题。"
    :suggestions="suggestions"
    enable-rename
    raised-input
    :input-placeholder="'输入您的问题...（Enter 发送，Shift+Enter 换行）'"
    :can-send="!!inputText.trim() && !isModeUnavailable"
    v-model:input-text="inputText"
    @send="handleSend"
    @suggest="useSuggestion"
    @new-session="handleNewSession"
  >
    <!-- 欢迎区差异内容：专项功能入口（位于预设提示词之后） -->
    <template #welcome-bottom>
      <div class="feature-entry-grid">
        <div
          v-for="f in aiFeatures"
          :key="f.key"
          class="feature-entry-card"
          @click="router.push(f.routePath)"
        >
          <el-icon :size="20" class="feature-entry-icon"><component :is="f.icon" /></el-icon>
          <span class="feature-entry-title">{{ f.title }}</span>
          <span class="feature-entry-desc">{{ f.entryDesc }}</span>
        </div>
      </div>
    </template>

    <!-- 空状态：模式选择 pills（居中，对齐 DeepSeek；会话进行中在输入框 footer 左侧） -->
    <template #welcome-modes>
      <el-radio-group v-model="selectedMode" size="small" :disabled="store.streaming">
        <el-radio-button value="normal" :disabled="!store.modeModels.normal">普通模式</el-radio-button>
        <el-radio-button value="expert" :disabled="!store.modeModels.expert">专家模式</el-radio-button>
      </el-radio-group>
    </template>

    <!-- 输入区差异内容：双模式选择 -->
    <template #input-footer-left>
      <el-radio-group v-model="selectedMode" size="small" :disabled="store.streaming">
        <el-radio-button value="normal" :disabled="!store.modeModels.normal">普通模式</el-radio-button>
        <el-radio-button value="expert" :disabled="!store.modeModels.expert">专家模式</el-radio-button>
      </el-radio-group>
    </template>

    <!-- 输入区差异内容：模式未绑定提示 -->
    <template #input-extra>
      <div v-if="isModeUnavailable" class="model-warning">
        当前模式未配置，请联系管理员在“AI 配置”中绑定
      </div>
    </template>
  </ChatPageShell>
</template>

<script setup lang="ts">
// 通用 AI 助手主页（#398）：壳（顶栏/侧栏/消息/输入/滚底）收敛进 ChatPageShell，
// 本页仅保留双模式选择与会话重命名启用等页面差异。
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ChatDotRound } from '@element-plus/icons-vue'
import ChatPageShell from '@/components/ai-assistant/ChatPageShell.vue'
import { useAIAssistantStore } from '@/stores/aiAssistant'
import { AI_FEATURES } from '@/config/aiFeatures'

const store = useAIAssistantStore()
const router = useRouter()

const inputText = ref('')

const selectedMode = computed({
  get: () => store.selectedMode,
  set: (v: 'normal' | 'expert') => store.selectMode(v)
})
const isModeUnavailable = computed(() => {
  const m = store.selectedMode
  if (m === 'normal') return !store.modeModels.normal
  if (m === 'expert') return !store.modeModels.expert
  return !store.modeModels.normal && !store.modeModels.expert
})

const aiFeatures = AI_FEATURES
const suggestions = [
  '叉车日常检查项目有哪些？',
  '电动叉车电池续航下降怎么排查？',
  '液压系统压力不足的常见原因？',
  '叉车季度保养项目有哪些？'
]

async function handleSend() {
  const text = inputText.value.trim()
  if (!text) return
  if (isModeUnavailable.value) {
    ElMessage.warning('当前模式未配置，请联系管理员')
    return
  }
  if (store.streaming) return

  inputText.value = ''
  try {
    await store.sendMessage(text)
  } catch (e: any) {
    // 错误已由 store 处理
  }
}

function useSuggestion(text: string) {
  inputText.value = text
  handleSend()
}

async function handleNewSession() {
  const label = store.selectedMode === 'expert' ? '专家模式' : '普通模式'
  await store.createSession('新会话', label)
}

onMounted(() => {
  store.init()
})
</script>

<style scoped>
/* ===== 专项功能入口（本页差异样式；#554 原则：色值走 token，深浅主题跟随） ===== */
.feature-entry-grid {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 12px;
  max-width: 760px;
  margin: 20px auto 0;
}

.feature-entry-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 16px 10px;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: 10px;
  cursor: pointer;
  transition: all var(--duration-fast) var(--ease-default);
}

.feature-entry-card:hover {
  border-color: var(--color-primary-400);
  background: var(--color-primary-50);
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(13, 148, 136, 0.1);
}

.feature-entry-icon {
  color: var(--color-primary-600);
}

.feature-entry-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.feature-entry-desc {
  font-size: 11px;
  color: var(--color-text-tertiary);
  text-align: center;
  line-height: 1.4;
}

.model-warning {
  text-align: center;
  font-size: 12px;
  color: var(--color-warning-strong);
  margin-top: 8px;
}

@media (max-width: 768px) {
  .feature-entry-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}
</style>
