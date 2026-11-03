<template>
  <ChatPageShell
    :logo-sub="`AI 叉车助手 · ${feature?.title || ''}`"
    :login-redirect="route.fullPath"
    :back-link-to="href('AIAssistant')"
    back-link-text="返回 AI 助手"
    :welcome-icon="feature?.icon || 'message-circle'"
    :welcome-title="feature?.title || ''"
    :suggestions="feature?.suggestions || []"
    :input-placeholder="inputPlaceholder"
    :can-send="draftReady"
    v-model:input-text="inputText"
    @send="handleSend"
    @suggest="useSuggestion"
    @new-session="handleNewSession"
  >
    <!-- 诊断筛选面板已随 v1 周边端点退役（票 2237）：品牌/车型/故障码三条数据源不再存在，
         按 v2 契约重接归票 2242、页面重适配归票 2244。这里宁可暂时变素，也不留点了必 404 的 UI。 -->

    <!-- 欢迎区差异内容：其余专项功能静态快捷选项 -->
    <template #welcome-top>
      <!-- 其余专项功能：静态快捷选项 -->
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

    <!-- 回答下方：逐轮来源回放（ADR-0033；默认折叠，历史每轮独立展开） -->
    <template #assistant-extra="{ message }">
      <DiagnosisSources
        v-if="isDiagnosis && message.sources?.length"
        :sources="message.sources"
      />
    </template>

    <!-- 输入区差异内容：图片待发队列（诊断来源只在回答下方逐轮展示，不在输入框上方残留） -->
    <template #input-above>
      <!-- key 用 previewUrl（每个 blob 唯一）：上传中 url 恒为空串，多张图会撞 key，
           移除失败项时 Vue 的 keyed diff 可能摘掉另一张的 DOM（票 #1358 判据 2） -->
      <div v-if="pendingImages.length" class="pending-images">
        <div v-for="(p, i) in pendingImages" :key="p.previewUrl" class="pending-image-item">
          <img :src="p.previewUrl" class="pending-image-thumb" alt="待发送图片" />
          <button class="pending-image-remove" title="移除" aria-label="移除待发送图片" @click="removePendingImage(i)">
            <UiIcon name="x" :size="12" />
          </button>
          <div v-if="p.uploading" class="pending-image-mask">上传中</div>
        </div>
      </div>
    </template>

    <!-- 输入区差异内容：图片上传按钮 -->
    <template #input-prefix>
      <UiUpload
        v-if="supportsImage"
        :show-file-list="false"
        :auto-upload="false"
        accept="image/png,image/jpeg,image/gif,image/webp,image/bmp,image/svg+xml"
        :disabled="store.streaming"
        multiple
        @change="handleImageSelect"
      >
        <UiButton :icon="'image'" circle :disabled="store.streaming || pendingImages.length >= maxImages" title="上传图片" aria-label="上传图片"/>
      </UiUpload>
    </template>
  </ChatPageShell>
</template>

<script setup lang="ts">
import UiIcon from '@/components/ui/UiIcon.vue'
// 专项功能聊天页（#398）：壳（顶栏/侧栏/消息/输入/滚底）收敛进 ChatPageShell，
// 本页仅保留快捷选项、图片队列等功能差异；助手内容随壳统一 markstream escape 安全渲染。
import { ref, computed, reactive, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import type { UploadFile } from 'element-plus'
import { href } from '@/config/pages'
import ChatPageShell from '@/components/ai-assistant/ChatPageShell.vue'
import { useAIAssistantStore, type SendOptions } from '@/stores/aiAssistant'
import {
  getAIFeatureByRoute,
  isDiagnosisFeature
} from '@/config/aiFeatures'
import DiagnosisSources from '@/components/ai-assistant/DiagnosisSources.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiUpload from '@/components/ui/UiUpload.vue'

const store = useAIAssistantStore()
const router = useRouter()
const route = useRoute()

// 按完整路由路径匹配配置（路由 slug 是连字符缩写，与 feature key 下划线全名不同，见 #332）
const feature = computed(() => getAIFeatureByRoute(route.path))
const supportsImage = computed(() => feature.value?.supportsImage === true)
const maxImages = computed(() => feature.value?.maxImages ?? 4)
// 智能维修诊断（fault_diagnosis）：品牌/车型联动 + 故障码面板 + 来源面板
const isDiagnosis = computed(() => isDiagnosisFeature(feature.value?.key))

const inputText = ref('')
// 只给任务片段：统一的「（Enter 发送，Shift+Enter 换行）」由壳拼上（文案单点在 ChatPageShell）
const inputPlaceholder = computed(() => {
  if (isDiagnosis.value) return '描述故障现象或上传现场照片...'
  return supportsImage.value
    ? '输入问题或上传图纸/习题图片...'
    : '输入您的问题...'
})

// ===== 智能维修诊断：筛选面板已随 v1 周边端点退役（票 2237）=====
// 品牌/车型/故障码三条数据源（/diagnosis/{brands,models,fault-codes}）在 v2 不存在，
// 调用点已同步删除——宁可诊断页暂时只剩自由输入，也不留点了必 404 的中间态。
// 按 v2 契约（intents/systems/faults/code_hint）重接归票 2242，页面重适配归票 2244。

interface PendingImage {
  url: string          // 上传成功后的服务器 URL
  previewUrl: string   // 本地 blob 预览
  uploading: boolean
}
const pendingImages = ref<PendingImage[]>([])

// 草稿就绪（页面 UI 态，#1104）：有文本或有图片，且没有还在上传的图片；
// store 侧闸门（无在飞轮次）由壳合成后作为唯一发送判据（Enter 与发送按钮同一值）。
const draftReady = computed(() =>
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

// 组装消息内容：预设选项作为前缀注入（智能维修诊断除外——它的筛选走前端组句，不注标签）
function buildContent(text: string): string {
  if (isDiagnosis.value) return text
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

  // 品牌/车型不再随请求发出：v2 的请求体只有 content 一个文本通道，后端已无消费方
  // （票 2237）。组句改由前端拼进正文（票 2241 的 diagnosisCompose，待 2244 落地）。
  const opts: SendOptions = images.length > 0 ? { images } : {}

  inputText.value = ''
  pendingImages.value = []
  try {
    // 发送编排与终态都在 store（#1104）：失败/中断走 store.lastTurnError，由壳渲染重试入口
    await store.send(buildContent(text), opts)
  } catch (e: any) {
    ElMessage.error(e?.message || '发送失败，请稍后再试')
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
  // 入队的是**代理本身**（票 #1358 / spec #1345 决策 #10）：push 裸对象时队列里读回的是
  // Vue 为它新建的代理，与局部变量身份不等 ⇒
  //  ① catch 分支「按身份摘掉失败项」的 `p !== pending` 恒成立，失败项永远摘不掉；
  //  ② 后续 pending.url / pending.uploading 写在裸对象上、绕过代理，不触发重渲染
  //     （蒙层不会自己消失、canSend 不会立刻翻真）。
  // reactive(p) 返回的代理与 arr[i] 读回的是同一个（reactive 幂等），两条都自动恢复正确。
  const pending = reactive<PendingImage>({ url: '', previewUrl, uploading: true })
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
    router.replace(href('AIAssistant'))
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
  font-size: var(--text-sm);
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
  border-radius: var(--radius-full);
  border: 1px solid var(--color-border-light);
  background: var(--color-bg-card);
  color: var(--color-text-secondary);
  font-size: var(--text-sm);
  cursor: pointer;
  transition: all var(--duration-fast) var(--ease-default);
}

.quick-option-chip:hover {
  border-color: var(--color-primary-400);
  color: var(--color-primary-soft-ink);
}

.quick-option-chip.active {
  border-color: var(--color-primary-600);
  background: var(--color-primary-soft);
  color: var(--color-primary-soft-ink);
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
  border-radius: var(--radius-md);
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
  border-radius: var(--radius-md);
  background: rgba(15, 23, 42, 0.5);
  color: #fff;
  font-size: var(--text-xs);
  display: flex;
  align-items: center;
  justify-content: center;
}
</style>
