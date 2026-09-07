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
    <!-- 诊断筛选移入输入框上方工具栏胶囊（方案 B；空态随输入框居中，不再撑欢迎区） -->
    <template #input-toolbar>
      <div v-if="isDiagnosis" class="flex items-center gap-2 overflow-x-auto pb-2">
        <UiCapsule
          :label="filterSummary ? `⚙ 筛选：${filterSummary}` : '⚙ 筛选'"
          :active="filterOpen"
          @click="filterOpen = !filterOpen"
        />
        <UiCapsule
          v-for="question in diagnosisQuickAsks"
          :key="question"
          :label="question"
          @click="quickAsk(question)"
        />
      </div>
      <!-- 折叠面板本体（挂输入区上方，随输入框居中/沉底；欢迎区不再渲染，避免撑爆空态） -->
      <div v-if="isDiagnosis && filterOpen" class="diagnosis-panel">
        <div class="diagnosis-catalog">
          <div class="catalog-row">
            <span class="catalog-label">品牌</span>
            <div class="catalog-chips">
              <button
                v-for="b in brands"
                :key="b.value"
                class="quick-option-chip"
                :class="{ active: selectedBrand === b.value }"
                @click="onBrandChange(b.value)"
              >{{ b.label }}</button>
              <span v-if="catalogLoading" class="catalog-loading">加载中...</span>
            </div>
          </div>
          <div v-if="selectedBrand && selectedBrand !== 'all'" class="catalog-row">
            <span class="catalog-label">车型</span>
            <div class="catalog-chips">
              <button
                v-for="m in models"
                :key="m"
                class="quick-option-chip"
                :class="{ active: selectedModel === m }"
                @click="toggleModel(m)"
              >{{ m }}</button>
            </div>
          </div>
          <div class="catalog-row" v-if="selectedBrand || faultTotal">
            <span class="catalog-label">故障码</span>
            <div class="catalog-fault-search">
              <input
                v-model="faultKeyword"
                class="catalog-fault-input"
                placeholder="搜索故障码或故障名"
                @keyup.enter="loadFaultCodes(1)"
              />
              <button class="quick-option-chip" @click="loadFaultCodes(1)">查询</button>
              <span v-if="faultTotal" class="catalog-fault-total">共 {{ faultTotal }} 条</span>
            </div>
            <div v-if="faultCodes.length" class="catalog-fault-list">
              <button
                v-for="fc in faultCodes"
                :key="fc.id"
                class="catalog-fault-item"
                :title="fc.symptom"
                @click="useFaultCode(fc)"
              >
                <b>{{ fc.fault_code }}</b>
                <span>{{ fc.fault_name }}</span>
              </button>
              <button
                v-if="faultTotal > faultCodes.length"
                class="quick-option-chip catalog-fault-more"
                @click="loadFaultCodes(faultPage + 1)"
              >加载更多</button>
            </div>
          </div>
        </div>
      </div>
    </template>

    <!-- 欢迎区差异内容：其余专项功能静态快捷选项（诊断筛选已搬入工具栏，此处不再渲染） -->
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

    <!-- 输入区差异内容：当轮来源面板与图片队列（叠放非互斥，见内层注释） -->
    <template #input-above>
      <!-- 当轮来源面板（SSE sources 事件内存态；落库后由逐轮回放接管） -->
      <!-- 与图片队列叠放（非互斥）：诊断页上传图后不断流也能看到待发送队列 -->
      <div v-if="isDiagnosis && store.lastSources.length" class="diagnosis-sources flex flex-col gap-2.5">
        <SourcesFoldHeader
          title="资料来源"
          :count="store.lastSources.length"
          suffix="，可从资料链接跳转原文"
          :open="sourcesOpen"
          @toggle="sourcesOpen = !sourcesOpen"
        />
        <template v-if="sourcesOpen">
          <DiagnosisSources :sources="store.lastSources" embedded />
        </template>
      </div>
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
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import type { UploadFile } from 'element-plus'
import { ChatDotRound, Picture, Close } from '@element-plus/icons-vue'
import ChatPageShell from '@/components/ai-assistant/ChatPageShell.vue'
import { useAIAssistantStore } from '@/stores/aiAssistant'
import { getAIFeatureByRoute } from '@/config/aiFeatures'
import { aiAssistantApi, type DiagnosisBrandOption, type DiagnosisFaultCodeItem } from '@/api/aiAssistant'
import DiagnosisSources from '@/components/ai-assistant/DiagnosisSources.vue'
import SourcesFoldHeader from '@/components/ai-assistant/SourcesFoldHeader.vue'
import UiCapsule from '@/components/ai-assistant/UiCapsule.vue'
import UiButton from '@/components/ui/UiButton.vue'

const store = useAIAssistantStore()
const router = useRouter()
const route = useRoute()

// 按完整路由路径匹配配置（路由 slug 是连字符缩写，与 feature key 下划线全名不同，见 #332）
const feature = computed(() => getAIFeatureByRoute(route.path))
const supportsImage = computed(() => feature.value?.supportsImage === true)
const maxImages = computed(() => feature.value?.maxImages ?? 4)
// 智能维修诊断（fault_diagnosis）：品牌/车型联动 + 故障码面板 + 来源面板
const isDiagnosis = computed(() => feature.value?.key === 'fault_diagnosis')

const inputText = ref('')
const inputPlaceholder = computed(() => {
  if (isDiagnosis.value) {
    return '描述故障现象或上传现场照片...（Enter 发送，Shift+Enter 换行）'
  }
  return supportsImage.value
    ? '输入问题或上传图纸/习题图片...（Enter 发送，Shift+Enter 换行）'
    : '输入您的问题...（Enter 发送，Shift+Enter 换行）'
})

// ===== 智能维修诊断：筛选折叠（T2 默认折叠，不再撑爆空态居中）=====
const filterOpen = ref(false)
const filterSummary = computed(() => {
  const parts: string[] = []
  if (selectedBrand.value && selectedBrand.value !== 'all') {
    parts.push(brands.value.find(b => b.value === selectedBrand.value)?.label || selectedBrand.value)
  }
  if (selectedModel.value) parts.push(selectedModel.value)
  return parts.join(' / ')
})
const diagnosisQuickAsks = [
  '叉车无法行驶且仪表报警，怎么排查？',
  '故障码 E102 是什么意思？',
  '货叉提升缓慢且伴随异响，可能是什么原因？'
]

function quickAsk(question: string) {
  inputText.value = question
  handleSend()
}

// ===== 智能维修诊断：品牌/车型联动（/diagnosis/brands|models 动态数据源）=====
const brands = ref<DiagnosisBrandOption[]>([])
const models = ref<string[]>([])
const selectedBrand = ref('')
const selectedModel = ref('')
const catalogLoading = ref(false)

async function loadBrands() {
  catalogLoading.value = true
  try {
    brands.value = await aiAssistantApi.listDiagnosisBrands()
  } catch {
    // 数据源暂不可用时保持空目录（无级联依赖，页面其余功能不受影响）
  } finally {
    catalogLoading.value = false
  }
}

async function onBrandChange(brand: string) {
  selectedBrand.value = brand
  selectedModel.value = ''
  models.value = []
  if (!brand || brand === 'all') return
  try {
    models.value = await aiAssistantApi.listDiagnosisModels(brand)
  } catch {
    models.value = []
  }
}

function toggleModel(model: string) {
  selectedModel.value = selectedModel.value === model ? '' : model
}

// ===== 智能维修诊断：故障码快捷查询（直连 fault-codes 代理，精确命中秒回）=====
const faultCodes = ref<DiagnosisFaultCodeItem[]>([])
const faultTotal = ref(0)
const faultPage = ref(1)
const faultKeyword = ref('')
const faultLoading = ref(false)

async function loadFaultCodes(page = 1) {
  if (faultLoading.value) return
  faultLoading.value = true
  try {
    const data = await aiAssistantApi.listDiagnosisFaultCodes({
      brand: selectedBrand.value && selectedBrand.value !== 'all' ? selectedBrand.value : undefined,
      keyword: faultKeyword.value.trim() || undefined,
      page,
      page_size: 5
    })
    faultCodes.value = data.items
    faultTotal.value = data.total
    faultPage.value = page
  } catch {
    // 查询失败静默（输入框仍可发起自由诊断）
  } finally {
    faultLoading.value = false
  }
}

function useFaultCode(item: DiagnosisFaultCodeItem) {
  inputText.value = `故障码 ${item.fault_code}（${item.fault_name}），怎么处理？`
  handleSend()
}

// ===== 当轮来源展开态（默认折叠；新一轮开始自动收起）=====
// 解析函数已收敛进 DiagnosisSources 组件；本页只保留展开态。
const sourcesOpen = ref(false)
// 新一轮开始自动收起（sourcesOpen 只描述当轮展开态，不跟随历史）
watch(() => store.lastSources, () => {
  sourcesOpen.value = false
})

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

// 组装消息内容：预设选项作为前缀注入（智能维修诊断除外——品牌/车型走结构化参数）
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
  if (store.streaming) return

  inputText.value = ''
  pendingImages.value = []
  try {
    await store.sendMessage(
      buildContent(text),
      images.length > 0 ? images : undefined,
      // 智能维修诊断：品牌/车型结构化过滤（不进正文）
      isDiagnosis.value
        ? {
            brand: selectedBrand.value && selectedBrand.value !== 'all' ? selectedBrand.value : undefined,
            model: selectedModel.value || undefined
          }
        : undefined
    )
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
    if (isDiagnosis.value) {
      loadBrands()
    }
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

/* ===== 智能维修诊断：品牌/车型联动 + 故障码面板（本页差异样式） ===== */
.diagnosis-panel {
  width: 100%;
  max-width: 640px;
  margin: 0 auto 24px;
}

.diagnosis-catalog {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 14px 16px;
  border: 1px solid var(--color-border-light);
  border-radius: 12px;
  background: var(--color-bg-card);
}

.catalog-row {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  flex-wrap: wrap;
}

.catalog-label {
  font-size: 13px;
  color: var(--color-text-secondary);
  font-weight: 600;
  line-height: 28px;
  min-width: 44px;
}

.catalog-chips {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  align-items: center;
}

.catalog-loading {
  font-size: 12px;
  color: var(--color-text-tertiary);
}

.catalog-fault-search {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}

.catalog-fault-input {
  width: 180px;
  height: 28px;
  padding: 0 10px;
  border: 1px solid var(--color-border-light);
  border-radius: 8px;
  background: var(--color-bg-field, var(--color-bg-card));
  color: var(--color-text-primary);
  font-size: 13px;
  outline: none;
}

.catalog-fault-input:focus {
  border-color: var(--color-primary-400);
}

.catalog-fault-total {
  font-size: 12px;
  color: var(--color-text-tertiary);
}

.catalog-fault-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  width: 100%;
}

.catalog-fault-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  border: 1px solid var(--color-border-light);
  border-radius: 8px;
  background: var(--color-bg-card);
  text-align: left;
  cursor: pointer;
  transition: border-color var(--duration-fast) var(--ease-default);
}

.catalog-fault-item:hover {
  border-color: var(--color-primary-400);
}

.catalog-fault-item b {
  color: var(--color-primary-600);
  font-size: 13px;
  min-width: 34px;
}

.catalog-fault-item span {
  color: var(--color-text-secondary);
  font-size: 13px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.catalog-fault-more {
  align-self: flex-start;
}

/* ===== 智能维修诊断：当轮来源面板容器（R1 原子类区：flex 结构走模板原子类） ===== */
.diagnosis-sources {
  margin-bottom: 8px;
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
