<template>
  <ChatPageShell
    :logo-sub="`AI 叉车助手 · ${feature?.title || ''}`"
    :login-redirect="route.fullPath"
    back-link-to="/ai-assistant"
    back-link-text="返回 AI 助手"
    :welcome-icon="feature?.icon || ChatDotRound"
    :welcome-title="feature?.title || ''"
    :welcome-desc="feature?.welcome || ''"
    :suggestions="feature?.suggestions || []"
    :input-placeholder="inputPlaceholder"
    :can-send="canSend"
    v-model:input-text="inputText"
    @send="handleSend"
    @suggest="useSuggestion"
    @new-session="handleNewSession"
  >
    <!-- 欢迎区差异内容：快捷选项（位于预设提示词之前） -->
    <template #welcome-top>
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
    </template>

    <!-- 输入区差异内容：待发送图片队列 -->
    <template #input-above>
      <div v-if="pendingImages.length" class="pending-images">
        <div v-for="(p, i) in pendingImages" :key="p.url" class="pending-image-item">
          <img :src="p.previewUrl" class="pending-image-thumb" alt="待发送图片" />
          <button class="pending-image-remove" title="移除" @click="removePendingImage(i)">
            <el-icon :size="12"><Close /></el-icon>
          </button>
          <div v-if="p.uploading" class="pending-image-mask">上传中</div>
        </div>
      </div>
    </template>

    <!-- 输入区差异内容：图片上传按钮 -->
    <template #input-prefix>
      <el-upload
        v-if="supportsImage"
        :show-file-list="false"
        :auto-upload="false"
        accept="image/png,image/jpeg,image/gif,image/webp,image/bmp,image/svg+xml"
        :disabled="store.streaming"
        multiple
        @change="handleImageSelect"
      >
        <UiButton :icon="Picture" circle :disabled="store.streaming || pendingImages.length >= maxImages" title="上传图片"/>
      </el-upload>
    </template>
  </ChatPageShell>
</template>

<script setup lang="ts">
// 专项功能聊天页（#398）：壳（顶栏/侧栏/消息/输入/滚底）收敛进 ChatPageShell，
// 本页仅保留快捷选项、图片队列等功能差异；助手内容随壳统一 markstream escape 安全渲染。
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import type { UploadFile } from 'element-plus'
import { ChatDotRound, Picture, Close } from '@element-plus/icons-vue'
import ChatPageShell from '@/components/ai-assistant/ChatPageShell.vue'
import { useAIAssistantStore } from '@/stores/aiAssistant'
import { getAIFeatureByRoute } from '@/config/aiFeatures'
import UiButton from '@/components/ui/UiButton.vue'

const store = useAIAssistantStore()
const router = useRouter()
const route = useRoute()

// 按完整路由路径匹配配置（路由 slug 是连字符缩写，与 feature key 下划线全名不同，见 #332）
const feature = computed(() => getAIFeatureByRoute(route.path))
const supportsImage = computed(() => feature.value?.supportsImage === true)
const maxImages = computed(() => feature.value?.maxImages ?? 4)

const inputText = ref('')
const inputPlaceholder = computed(() =>
  supportsImage.value
    ? '输入问题或上传图纸/习题图片...（Enter 发送，Shift+Enter 换行）'
    : '输入您的问题...（Enter 发送，Shift+Enter 换行）'
)

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

// 只切本地草稿态：会话在首次发消息时才创建（避免没说话就产生空历史）
function handleNewSession() {
  store.startDraft()
}

onMounted(() => {
  if (feature.value) {
    store.initFeature(feature.value.key)
  } else {
    router.replace('/ai-assistant')
  }
})
</script>

<style scoped>
/* ===== 快捷选项（本页差异样式） ===== */
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
  color: var(--color-text-secondary);
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
  border: 1px solid var(--color-border-light);
  background: var(--color-bg-card);
  color: var(--color-text-secondary);
  font-size: 13px;
  cursor: pointer;
  transition: all var(--duration-fast) var(--ease-default);
}

.quick-option-chip:hover {
  border-color: var(--color-primary-400);
  color: var(--color-primary-600);
}

.quick-option-chip.active {
  border-color: var(--color-primary-600);
  background: var(--color-primary-50);
  color: var(--color-primary-600);
  font-weight: 600;
}

/* ===== 待发送图片（本页差异样式） ===== */
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
  border: 1px solid var(--color-border-light);
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
  background: var(--color-danger);
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
</style>
