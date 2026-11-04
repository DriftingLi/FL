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
    <!-- 欢迎区（诊断）：品牌卡 + 六张诉求卡 + FAQ。三件套超出首屏是必然的，
         壳让欢迎区自身滚动来兜（票 2241 的布局硬约束）。上游未配置时三块都不渲染。 -->
    <template #welcome-top>
      <DiagnosisWelcome
        v-if="isDiagnosis"
        :brands="brands"
        :faq="faq"
        @pick-brand="onPickBrand"
        @pick-scene="onPickScene"
        @ask="askDirectly"
      />
      <!-- 其余专项功能：静态快捷选项 -->
      <div v-if="!isDiagnosis && feature?.quickOptions?.length" class="quick-options-area">
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

    <!-- 欢迎区底部（诊断）：上游不可用时的唯一提示条。**不轮询**——只解决「打开就发现坏了」，
         运营期的抖动交给发送失败时的 turn-error 条（壳的既有通道，同信号不画两遍）。 -->
    <template #welcome-bottom>
      <UiAlert
        v-if="isDiagnosis && upstreamDown"
        class="mx-auto mt-4 w-full max-w-[600px]"
        type="warning"
        :closable="false"
        show-icon
        title="智能诊断服务暂未启用，可以先自由描述故障；管理员启用后这里会出现品牌与常见现象入口。"
      />
    </template>

    <!-- 输入区上方（诊断）：四步引导向导。形态是 UiCollapse（不是浮层）——
         林德 87 个型号在浮层里会盖住聊天区，且我方壳没有浮层槽位。 -->
    <template #input-toolbar>
      <div v-if="isDiagnosis" class="mb-2">
        <UiCapsule
          :label="guideCapsuleLabel"
          :icon="'list-checks'"
          :active="guideOpen"
          @click="guideOpen = !guideOpen"
        />
        <UiCollapse :open="guideOpen" lazy>
          <DiagnosisGuide
            :brands="brands"
            :preset-brand-id="presetBrandId"
            :preset-fault="presetFault"
            :disabled="store.streaming"
            @send="sendComposed"
          />
        </UiCollapse>
      </div>
    </template>

    <!-- 助手气泡上方（诊断）：能力徽章行（票 2245 的 badges） -->
    <template #assistant-caps="{ message }">
      <DiagnosisBadges v-if="isDiagnosis" :badges="message.meta?.badges || []" />
    </template>

    <!-- 助手气泡下方：逐轮来源回放 + 耗时（ADR-0033；默认折叠，历史每轮独立展开）。
         耗时并入来源折叠头右侧（票 2244 拍板）——只有耗时没有来源时退化为一行脚注。 -->
    <template #assistant-extra="{ message }">
      <DiagnosisSources
        v-if="isDiagnosis && (message.sources?.length || message.meta?.elapsed)"
        :sources="message.sources || []"
        :elapsed="message.meta?.elapsed || 0"
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
// 本页只保留快捷选项、图片队列等功能差异；助手内容随壳统一 markstream escape 安全渲染。
import { ref, computed, reactive, onMounted, onBeforeUnmount } from 'vue'
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
import DiagnosisBadges from '@/components/ai-assistant/DiagnosisBadges.vue'
import DiagnosisWelcome from '@/components/ai-assistant/DiagnosisWelcome.vue'
import DiagnosisGuide from '@/components/ai-assistant/DiagnosisGuide.vue'
import UiCapsule from '@/components/ai-assistant/UiCapsule.vue'
import UiCollapse from '@/components/ui/UiCollapse.vue'
import UiAlert from '@/components/ui/UiAlert.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiUpload from '@/components/ui/UiUpload.vue'
import { aiAssistantApi, type DiagnosisFAQ, type DiagnosisIntentBrand } from '@/api/aiAssistant'
import { compressImage } from '@/utils/imageCompress'

const store = useAIAssistantStore()
const router = useRouter()
const route = useRoute()

// 按完整路由路径匹配配置（路由 slug 是连字符缩写，与 feature key 下划线全名不同，见 #332）
const feature = computed(() => getAIFeatureByRoute(route.path))
const supportsImage = computed(() => feature.value?.supportsImage === true)
const maxImages = computed(() => feature.value?.maxImages ?? 4)
// 智能维修诊断（fault_diagnosis）：四级引导向导 + 欢迎区三件套 + 徽章/耗时回放
const isDiagnosis = computed(() => isDiagnosisFeature(feature.value?.key))

const inputText = ref('')
// 只给任务片段：统一的「（Enter 发送，Shift+Enter 换行）」由壳拼上（文案单点在 ChatPageShell）
const inputPlaceholder = computed(() => {
  if (isDiagnosis.value) return '描述故障现象或上传现场照片...'
  return supportsImage.value
    ? '输入问题或上传图纸/习题图片...'
    : '输入您的问题...'
})

// ===== 智能维修诊断：引导数据（品牌 / FAQ / 上游健康）=====
// 三个字典端点在上游未配置时回 502，health 恒 200 但 ok=false。
// 页面**不轮询**：进页拉一次，失败就记成 upstreamDown 出一条提示条。
const brands = ref<DiagnosisIntentBrand[]>([])
const faq = ref<DiagnosisFAQ[]>([])
const upstreamDown = ref(false)

const guideOpen = ref(false)
const presetBrandId = ref('')
const presetFault = ref('')

/** 胶囊文案：向导的当前状态一眼可见（未选 → 已选到哪一步）*/
const guideCapsuleLabel = computed(() => {
  if (presetFault.value) return `引导选诉求 · ${presetFault.value}`
  return guideOpen.value ? '收起引导' : '引导选诉求'
})

async function loadDiagnosisIntents() {
  if (!isDiagnosis.value) return
  try {
    const [health, intents] = await Promise.all([
      aiAssistantApi.getDiagnosisHealth().catch(() => null),
      aiAssistantApi.listDiagnosisIntents()
    ])
    // health 恒 200；ok=false 表示未配置或上游不可达 —— 两种都不该画点不动的卡片。
    // 因此 ok=false 时连已拿到的字典一起丢掉：留着品牌卡而 health 说不通，会出现
    // 「点得动、但点了就 502」的死入口（票 2244 拍板的降级是整块留白）。
    upstreamDown.value = health ? !health.ok : true
    brands.value = upstreamDown.value ? [] : intents?.brands ?? []
    faq.value = upstreamDown.value ? [] : intents?.faq ?? []
  } catch {
    // 字典端点 502（未配置）：品牌与 FAQ 留空、欢迎区三件套整块不渲染，只出提示条
    upstreamDown.value = true
    brands.value = []
    faq.value = []
  }
}

function onPickBrand(brandId: string) {
  presetBrandId.value = brandId
  presetFault.value = ''
  guideOpen.value = true
}

function onPickScene(faultName: string) {
  presetFault.value = faultName
  presetBrandId.value = ''
  guideOpen.value = true
}

/** 向导组好句：直接发出去（不经过输入框，句子本身就是用户要发的那句话） */
function sendComposed(sentence: string) {
  guideOpen.value = false
  presetFault.value = ''
  sendText(sentence)
}

/** FAQ 点击 / 预设提示词点击：一句现成的话直接发 */
function askDirectly(question: string) {
  if (question) sendText(question)
}

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

// 组装消息内容：预设选项作为前缀注入（智能维修诊断除外——诊断的品牌/型号走引导组句）
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

  inputText.value = ''
  pendingImages.value = []
  await sendText(buildContent(text), images.length > 0 ? { images } : {})
}

/** 发送单点：输入框发送 / 向导组句 / FAQ 点击都走这里 */
async function sendText(text: string, opts: SendOptions = {}) {
  // 品牌/车型不随请求发出：v2 的请求体只有 content 一个文本通道，后端已无消费方
  // （票 2237）。引导选出来的品牌/型号已由 diagnosisCompose 拼进正文。
  try {
    // 发送编排与终态都在 store（#1104）：失败/中断走 store.lastTurnError，由壳渲染重试入口
    await store.send(text, opts)
  } catch (e: any) {
    ElMessage.error(e?.message || '发送失败，请稍后再试')
  }
}

function useSuggestion(text: string) {
  inputText.value = text
  handleSend()
}

// 图片选择：先本地压缩（长边 1400 / q0.82），再上传压缩后的那一份
async function handleImageSelect(file: UploadFile) {
  const raw = file.raw
  if (!raw) return
  if (pendingImages.value.length >= maxImages.value) {
    ElMessage.warning(`最多上传 ${maxImages.value} 张图片`)
    return
  }
  // 压缩在选图时就做（票 2244）：传上去的已经是压好的那份，不浪费带宽也不让服务端重编码。
  // 压缩失败会原样返回（见 utils/imageCompress.ts 的降级），不阻断上传。
  const { file: toUpload } = await compressImage(raw)
  const previewUrl = URL.createObjectURL(toUpload)
  // 入队的是**代理本身**（票 #1358 / spec #1345 决策 #10）：push 裸对象时队列里读回的是
  // Vue 为它新建的代理，与局部变量身份不等 ⇒
  //  ① catch 分支「按身份摘掉失败项」的 `p !== pending` 恒成立，失败项永远摘不掉；
  //  ② 后续 pending.url / pending.uploading 写在裸对象上、绕过代理，不触发重渲染
  //     （蒙层不会自己消失、canSend 不会立刻翻真）。
  // reactive(p) 返回的代理与 arr[i] 读回的是同一个（reactive 幂等），两条都自动恢复正确。
  const pending = reactive<PendingImage>({ url: '', previewUrl, uploading: true })
  pendingImages.value.push(pending)
  try {
    pending.url = await store.uploadImage(toUpload)
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
    loadDiagnosisIntents()
  } else {
    router.replace(href('AIAssistant'))
  }
})

onBeforeUnmount(() => {
  // 离开页面时释放还没发出去的预览 blob（否则每个未发送的缩略图都留一份内存）
  pendingImages.value.forEach(p => URL.revokeObjectURL(p.previewUrl))
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
