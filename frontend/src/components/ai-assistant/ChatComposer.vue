<template>
  <div
    class="chat-input-area mx-auto w-full bg-panel px-6 pb-5 pt-3 max-[768px]:px-3 max-[768px]:pb-3 max-[768px]:pt-2"
    :class="gridWelcome ? 'row-start-2 max-w-[760px]' : isWelcome ? 'max-w-[760px] shrink-0' : 'max-w-[1200px] shrink-0'"
  >
    <slot name="input-toolbar" />
    <slot name="input-above" />
    <!-- 当轮失败/中断（#1104）：原因走 store.lastTurnError 独立通道（正文不再拼「[生成失败：…]」），
         这里给可重发入口；重试原样重发该轮输入与差异参数（图片/结构化过滤） -->
    <div
      v-if="store.lastTurnError"
      class="turn-error mb-2 flex items-center justify-between gap-3 rounded-ctl border border-line bg-bad-soft px-3 py-2 text-sm text-bad"
    >
      <span class="turn-error-text min-w-0 flex-1 truncate">{{ turnErrorText }}</span>
      <UiButton variant="primary" @click="handleRetry">重试</UiButton>
    </div>
    <div class="input-wrap flex items-center gap-2 rounded-card border border-line bg-panel px-3 py-2 shadow-card transition-colors duration-[var(--duration-fast)] ease-[var(--ease-default)] focus-within:border-ui-400" :class="[raisedInput ? 'input-wrap--raised min-h-[132px] flex-col items-stretch p-3 gap-3' : '', !raisedInput && !!slots['input-prefix'] ? 'has-image items-end' : '']" @compositionstart="onCompositionStart" @compositionend="onCompositionEnd">
      <slot v-if="!raisedInput" name="input-prefix" />
      <UiInput :model-value="inputText" type="textarea" :rows="raisedInput ? 4 : 2" :autosize="raisedInput ? { minRows: 3, maxRows: 8 } : { minRows: 1, maxRows: 6 }" :placeholder="composerPlaceholder" resize="none" @keydown.enter="handleEnter" @update:model-value="emit('update:inputText', $event as string)" :disabled="store.streaming" />
      <div v-if="raisedInput" class="input-footer flex items-center justify-between gap-3">
        <div class="mode-selector flex items-center gap-2">
          <slot name="input-footer-left" />
        </div>
        <div class="input-actions shrink-0">
          <UiButton variant="primary" v-if="!store.streaming" :icon="'send'" :disabled="!canSend" @click="emit('send')">
            发送
          </UiButton>
          <UiButton variant="danger" v-else :icon="'pause'" @click="store.stopStreaming">
            停止
          </UiButton>
        </div>
      </div>
      <div v-else class="input-actions shrink-0">
        <UiButton variant="primary" v-if="!store.streaming" :icon="'send'" :disabled="!canSend" @click="emit('send')">
          发送
        </UiButton>
        <UiButton variant="danger" v-else :icon="'pause'" @click="store.stopStreaming">
          停止
        </UiButton>
      </div>
    </div>
    <slot name="input-extra" />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, useSlots } from 'vue'
import { ElMessage } from 'element-plus'
import UiButton from '@/components/ui/UiButton.vue'
import UiInput from '@/components/ui/UiInput.vue'
import { useAIAssistantStore } from '@/stores/aiAssistant'

const props = withDefaults(
  defineProps<{
    inputText: string
    inputPlaceholder?: string
    raisedInput?: boolean
    /** 壳合成的唯一发送判据：页面草稿就绪 × store 闸门 */
    canSend: boolean
    /** 跨区布局状态由壳持有 */
    gridWelcome: boolean
    isWelcome: boolean
  }>(),
  {
    inputPlaceholder: '输入您的问题...',
    raisedInput: false
  }
)

const emit = defineEmits<{
  (e: 'update:inputText', value: string): void
  (e: 'send'): void
}>()

const slots = useSlots()
const store = useAIAssistantStore()

/** 发送提示后缀（唯一一份）：页面只传任务片段，避免同一句话在四处手抄后各走各的 */
const SEND_HINT = '（Enter 发送，Shift+Enter 换行）'
const composerPlaceholder = computed(() => `${props.inputPlaceholder}${SEND_HINT}`)

// ===== 输入与发送：CJK 选词守卫 + 单一发送判据 =====
// 两件不同的事，别混成一句「一段时间内都拦」：
//   ① 组字中（Chrome keyCode 229 / isComposing）的 Enter 属于输入法，是选词确认，连 preventDefault 都不该抢；
//   ② 部分输入法在 compositionend 之后还会补一个 keyCode 13 的伴生 Enter —— 不拦就会把半截拼音发出去。
// ② 的窗口取 100ms 而不是常见的 500ms：伴生键与 compositionend 同一个任务里到达，
// 500ms 会把「鼠标点完候选词、0.3 秒后按下的真发送键」一起吃掉。拦截后立刻复位，不误吞下一次真击键。
const IME_ENTER_GRACE_MS = 100
const composing = ref(false)
// 哨兵不能是 0：否则页面加载后 100ms 内的第一次 Enter 会被算成「刚选完词」而误吞
let lastCompositionEndAt: number | null = null

function onCompositionStart() {
  composing.value = true
}

function onCompositionEnd() {
  composing.value = false
  lastCompositionEndAt = performance.now()
}

function handleEnter(e: KeyboardEvent) {
  if (e.shiftKey) return
  // ① 组字中：这一下 Enter 归输入法（选词确认），放行原生默认行为
  if (composing.value || e.isComposing || e.keyCode === 229) return
  // ② 选词确认后伴生到达的 Enter：吃掉它并立刻复位
  if (lastCompositionEndAt !== null && performance.now() - lastCompositionEndAt < IME_ENTER_GRACE_MS) {
    lastCompositionEndAt = null
    e.preventDefault()
    return
  }
  // #1104：Enter 与发送按钮读同一判据。闸门关着就保留原生换行
  //（此前无条件 preventDefault，连换行也一并吃掉）
  if (!props.canSend) return
  e.preventDefault()
  emit('send')
}

// 当轮失败/中断的提示文案（#1104）：error 带后端/网络原因；aborted 只描述中断事实
const turnErrorText = computed(() => {
  const err = store.lastTurnError
  if (!err) return ''
  return err.kind === 'error' ? `生成失败：${err.message}` : '已中断生成'
})

/** 重试上一轮（store.retryLastTurn）：被前置校验拒绝时把可执行原因告诉用户，不静默 */
async function handleRetry() {
  try {
    await store.retryLastTurn()
  } catch (e: any) {
    ElMessage.warning(e?.message || '重试失败，请稍后再试')
  }
}
</script>

<style scoped>
.input-wrap--raised :deep(.el-textarea) {
  flex: 1;
}

.input-wrap :deep(.el-textarea__inner) {
  border: none;
  background: transparent;
  padding: 0;
  /* !important 是必需的：EP 的 `.el-textarea__inner:focus` 用伪类提权到与本类同级，
     不带它时聚焦态会把 EP 自己的 1px inset 描边画回这个已置空的输入框。 */
  box-shadow: none !important;
  font-size: var(--text-sm);
  line-height: 1.6;
}
</style>
