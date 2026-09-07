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
    <!-- 输入框上方：专项功能胶囊工具栏（方案 B；空态随输入框居中，有消息沉底跟随） -->
    <template #input-toolbar>
      <div class="feature-toolbar flex gap-2 overflow-x-auto pb-2">
        <button
          v-for="f in aiFeatures"
          :key="f.key"
          class="feature-capsule flex shrink-0 cursor-pointer items-center gap-1.5 whitespace-nowrap rounded-pill border border-line bg-panel px-3.5 py-1.5 text-[13px] text-ink-2 transition-all duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:border-ui-400 hover:bg-ui-50 hover:text-ui-600"
          @click="router.push(f.routePath)"
        >
          <el-icon :size="14"><component :is="f.icon" /></el-icon>
          {{ f.title }}
          <i v-if="f.freePreview" class="free-preview-badge">限免</i>
        </button>
      </div>
    </template>

    <!-- 输入区差异内容：双模式下拉框 -->
    <template #input-footer-left>
      <el-select v-model="selectedMode" size="small" :disabled="store.streaming" class="mode-select" style="width: 112px">
        <el-option value="normal" label="普通模式" :disabled="!store.modeModels.normal" />
        <el-option value="expert" label="专家模式" :disabled="!store.modeModels.expert" />
      </el-select>
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

// 只切本地草稿态：会话在首次发消息时才创建（避免没说话就产生空历史）
function handleNewSession() {
  store.startDraft()
}

onMounted(() => {
  store.init()
})
</script>

<style scoped>
/* ===== 方案 B 胶囊工具栏（本页差异样式；#554 原则：色值走 token，深浅主题跟随） ===== */
.free-preview-badge {
  font-style: normal;
  font-size: 10px;
  font-weight: 600;
  color: #fff;
  background: var(--color-success);
  border-radius: 999px;
  padding: 1px 6px;
  line-height: 1.4;
}

.model-warning {
  text-align: center;
  font-size: 12px;
  color: var(--color-warning-strong);
  margin-top: 8px;
}
</style>
