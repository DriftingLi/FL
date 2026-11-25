<template>
  <div
    class="message-list mx-auto w-full p-6 max-[768px]:p-4"
    :class="gridWelcome
      ? 'row-start-1 h-full max-w-[760px] self-end justify-self-center min-h-0 overflow-hidden'
      : isWelcome
        ? 'max-w-[760px] flex-1 overflow-y-auto [scrollbar-gutter:stable]'
        : 'max-w-[1200px] flex-1 overflow-y-auto [scrollbar-gutter:stable]'"
  >
    <slot name="welcome" />

    <!-- 消息加载中（选中会话拉取正文；侧栏不锁，失败由选中逻辑弹提示） -->
    <div v-if="store.messagesLoading" class="message-loading-more flex items-center justify-center gap-2 py-8 text-sm text-ink-3">
      <span class="loading-dot h-2 w-2 rounded-pill bg-ui-400"></span>
      <span class="loading-dot h-2 w-2 rounded-pill bg-ui-400"></span>
      <span class="loading-dot h-2 w-2 rounded-pill bg-ui-400"></span>
      会话加载中…
    </div>

    <!-- 消息列表（安全渲染单点：助手内容统一 markstream escape） -->
    <div v-for="msg in store.messages" :key="msg.id" class="message-item mb-6 flex gap-3" :class="[msg.role, msg.role === 'user' ? 'flex-row-reverse' : '']">
      <div class="message-avatar flex h-9 w-9 shrink-0 items-center justify-center rounded-ctl" :class="msg.role === 'user' ? 'bg-ui-600 text-white' : 'bg-ui-soft-strong text-ui-soft-ink'">
        <UiIcon name="user" v-if="msg.role === 'user'" :size="18" />
        <UiIcon name="message-circle" v-else :size="18" />
      </div>
      <div class="message-content max-w-[75%] max-[768px]:max-w-[85%]">
        <template v-if="msg.role === 'user'">
          <div v-if="msg.images?.length" class="message-images mb-2 flex flex-wrap justify-end gap-2">
            <el-image
              v-for="(img, i) in msg.images"
              :key="i"
              :src="img"
              :preview-src-list="msg.images"
              :initial-index="i"
              fit="cover"
              class="message-image-thumb h-[120px] w-[120px] cursor-pointer rounded-ctl border border-line"
              preview-teleported
            />
          </div>
          <div v-if="msg.content" class="message-text break-words rounded-card bg-ui-600 px-4 py-3 text-sm leading-[1.7] text-white">{{ msg.content }}</div>
        </template>
        <div v-else class="message-text markstream-vue break-words rounded-card border border-line bg-panel px-4 py-3 text-sm leading-[1.7] text-ink" @click="openImagePreview">
          <!-- 能力徽章行：刻意在正文**之前**（对方 app.js:265 的 .caps 同样在 body 之前）——
               先把「这轮用了哪些能力」交代清楚，正文再展开。槽位独立于 assistant-extra：
               后者在正文之后、语义是「补充材料」（来源、耗时），两者不共用。 -->
          <slot name="assistant-caps" :message="msg" />
          <MarkdownRender
            mode="chat"
            :content="msg.content"
            :final="true"
            html-policy="escape"
            :mermaid-props="MARKSTREAM_MERMAID_PROPS"
            :fade="false"
          />
          <slot name="assistant-extra" :message="msg" />
        </div>
      </div>
    </div>

    <!-- 流式输出中 -->
    <div v-if="store.streaming" class="message-item assistant mb-6 flex gap-3">
      <div class="message-avatar flex h-9 w-9 shrink-0 items-center justify-center rounded-ctl bg-ui-soft-strong text-ui-soft-ink">
        <UiIcon name="message-circle" :size="18" />
      </div>
      <div class="message-content max-w-[75%] max-[768px]:max-w-[85%]">
        <div v-if="store.streamingContent" class="message-text markstream-vue break-words rounded-card border border-line bg-panel px-4 py-3 text-sm leading-[1.7] text-ink" @click="openImagePreview">
          <MarkdownRender
            mode="chat"
            :content="store.streamingContent"
            :final="!store.streaming"
            html-policy="escape"
            :mermaid-props="MARKSTREAM_MERMAID_PROPS"
            :fade="false"
          />
        </div>
        <div v-else class="message-loading flex items-center gap-2 py-1 text-sm text-ink-3">
          <span class="flex gap-1">
            <span class="loading-dot h-2 w-2 rounded-pill bg-ui-400"></span>
            <span class="loading-dot h-2 w-2 rounded-pill bg-ui-400"></span>
            <span class="loading-dot h-2 w-2 rounded-pill bg-ui-400"></span>
          </span>
          <!-- 上游过程文案（票 2280 / 2245）：诊断服务的检索阶段会推「正在调取该系统的
               维修资料…」这类人话，把 8~15 秒只有三个点的空档填上。
               其它功能不推 status，这里就只剩三个点，与改造前一致。 -->
          <span v-if="store.streamingStatus" class="loading-status">{{ store.streamingStatus }}</span>
        </div>
      </div>
    </div>

    <!-- 当轮计费脚注（#620：usage 走 store 独立 lastUsage 通道，由消息列表渲染；消息正文不含计费文本） -->
    <div v-if="store.lastUsage" class="usage-footnote pb-1 pr-1 text-right text-xs text-ink-3">
      本轮消耗 {{ store.lastUsage.points_cost }} 分 · {{ (store.lastUsage.total_tokens / 1000).toFixed(1) }}k tokens · 余额 {{ store.lastUsage.balance }}
    </div>

    <!-- 回到最新：粘在滚动视口底部的零高容器（不占布局、不推内容），只在用户回翻时出现 -->
    <div v-if="!isWelcome" class="sticky bottom-0 z-10 h-0">
      <div class="absolute inset-x-0 bottom-3 flex justify-center">
        <button
          v-if="!atBottom"
          type="button"
          class="scroll-to-bottom cursor-pointer rounded-pill border border-line bg-panel px-3 py-1.5 text-xs text-ink-2 shadow-card transition-colors duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:border-ui-400 hover:text-ink"
          @click="emit('resume-follow')"
        >
          {{ store.streaming ? '有新回复 ↓' : '回到底部 ↓' }}
        </button>
      </div>
    </div>

    <el-image-viewer
      v-if="previewUrls.length"
      class="assistant-image-viewer"
      :url-list="previewUrls"
      :initial-index="previewIndex"
      hide-on-click-modal
      teleported
      @close="closeImagePreview"
    />
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import MarkdownRender from 'markstream-vue'
import 'markstream-vue/index.css'
import UiIcon from '@/components/ui/UiIcon.vue'
import { MARKSTREAM_MERMAID_PROPS } from '@/utils/markstreamRuntime'
import { useAIAssistantStore } from '@/stores/aiAssistant'

defineProps<{
  /** 跨区状态由壳持有：决定欢迎区/列表宽度与回底入口 */
  isWelcome: boolean
  gridWelcome: boolean
  atBottom: boolean
}>()

const emit = defineEmits<{
  (e: 'resume-follow'): void
}>()

const store = useAIAssistantStore()

// ===== 助手气泡图片点击放大（正文 markdown 图 + 来源面板缩略图共用一个预览器）=====
// markstream 的图片节点只给 cursor-pointer、不管预览，来源面板此前也只是普通 <img>
// ⇒ Web 两处都点不开，而移动端 ai-chat-sources 已用 uni.previewImage，这是跨端不对称。
// 事件委托挂在气泡容器上（库渲染的 <img> 我们插不进属性）；预览列表取当前气泡内全部
// 图片，语义与移动端「打开一张、整列表可翻」一致。
const previewUrls = ref<string[]>([])
const previewIndex = ref(0)

function openImagePreview(event: MouseEvent) {
  const bubble = event.currentTarget as HTMLElement | null
  const img = (event.target as HTMLElement | null)?.closest('img') as HTMLImageElement | null
  if (!bubble || !img) return
  const clicked = img.currentSrc || img.src
  if (!clicked) return
  const urls = Array.from(
    new Set(Array.from(bubble.querySelectorAll('img')).map(node => node.currentSrc || node.src).filter(Boolean))
  )
  if (urls.length === 0) return
  previewUrls.value = urls
  previewIndex.value = Math.max(0, urls.indexOf(clicked))
}

function closeImagePreview() {
  previewUrls.value = []
  previewIndex.value = 0
}
</script>

<style scoped>
.loading-dot {
  animation: dot-bounce 1.4s infinite ease-in-out;
}

.loading-dot:nth-child(2) { animation-delay: 0.2s; }
.loading-dot:nth-child(3) { animation-delay: 0.4s; }

@keyframes dot-bounce {
  0%, 80%, 100% { transform: scale(0.6); opacity: 0.4; }
  40% { transform: scale(1); opacity: 1; }
}
</style>
