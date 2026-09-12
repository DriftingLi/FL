<template>
  <ChatPageShell
    logo-sub="AI 叉车助手 · HRWAI"
    login-redirect="/ai-assistant"
    :welcome-icon="ChatDotRound"
    welcome-title="选购维保故障操作，叉车问题一问即答"
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
        <UiCapsule
          v-for="f in aiFeatures"
          :key="f.key"
          :icon="f.icon"
          :label="f.title"
          :badge="f.freePreview ? '限免' : ''"
          @click="router.push(f.routePath)"
        />
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
import UiCapsule from '@/components/ai-assistant/UiCapsule.vue'
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
/* 模式未绑定提示（限免角标已收敛进 UiCapsule 组件） */
.model-warning {
  text-align: center;
  font-size: 12px;
  color: var(--color-warning-strong);
  margin-top: 8px;
}
</style>
