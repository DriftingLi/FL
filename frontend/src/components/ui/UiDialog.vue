<script setup lang="ts">
import UiButton from './UiButton.vue'

/**
 * 对话框：包 `el-dialog`，补 Element Plus 缺的视觉层次。
 * EP 原厂弹窗只有「标题 + 内容 + 页脚」三段，头部没有图标位、没有副标题、
 * 没有分隔线，页脚按钮也没有收敛 —— 全项目 36 处裸用 el-dialog 时各自补样式，
 * 观感不统一。这里把结构收成一层，业务只传 title / footer。
 *
 * 约定（R1）：模板只用 Tailwind 原子类；改 EP 内部结构一律走 scoped 里的
 * `:deep()`，不在模板上堆原子类去压 EP 类名。
 *
 * 约定（R2 纯增量）：所有新增 prop 的默认值都等于改造前裸用 el-dialog 的行为
 * —— 不传 title 就不渲染标题区（EP 的 header 容器仍在，关闭按钮要挂在里面）、
 * hideFooter 默认 false、center 默认 false，
 * 让把裸 el-dialog 换成 UiDialog 的调用方零视觉 diff。
 *
 * ⚠️ global.css 的媒体查询已处理移动端 `--el-dialog-width: 92%/96%`，
 * 这里不要重复声明宽度，否则会与 !important 抢优先级。
 */
const visible = defineModel<boolean>({ default: false })

const props = withDefaults(
  defineProps<{
    /** 标题。不传则不渲染头部（等同 EP 默认行为） */
    title?: string
    /** 标题下的浅色说明行 */
    subtitle?: string
    /** 头部图标，接受全局注册名或组件引用（与 UiButton.icon 同款放宽） */
    icon?: string | object
    /** 宽度，直接透传给 el-dialog */
    width?: string | number
    /** 垂直居中。EP 默认贴顶 15vh，故默认 false 保持现状 */
    center?: boolean
    /** 隐藏整个页脚（业务要自定义按钮语义时用 #footer 插槽整体覆盖） */
    hideFooter?: boolean
    /** 隐藏「取消」按钮（只有「确定」的确认框） */
    showCancel?: boolean
    confirmText?: string
    cancelText?: string
    confirmLoading?: boolean
    confirmDisabled?: boolean
    /** 关闭按钮（右上角 ×）是否显示 */
    showClose?: boolean
    /** 点击遮罩是否关闭 */
    closeOnClickModal?: boolean
  }>(),
  {
    title: '',
    width: '50%',
    center: false,
    hideFooter: false,
    showCancel: true,
    confirmText: '确定',
    cancelText: '取消',
    confirmLoading: false,
    confirmDisabled: false,
    showClose: true,
    closeOnClickModal: true
  }
)

const emit = defineEmits<{
  /** 点「确定」 */
  confirm: []
  /** 点「取消」按钮 */
  cancel: []
  /** 对话框已关闭（含 × / 遮罩 / ESC，与 cancel 不互斥） */
  close: []
  /** 对话框打开时触发（透传 el-dialog 的 open）—— 内容需要按打开时机拉数据时用 */
  open: []
}>()

function onCancel() {
  emit('cancel')
  visible.value = false
}

function onConfirm() {
  emit('confirm')
}

function onClosed() {
  emit('close')
}
</script>

<template>
  <el-dialog
    v-model="visible"
    class="ui-dialog"
    :width="props.width"
    :align-center="props.center"
    :show-close="props.showClose"
    :close-on-click-modal="props.closeOnClickModal"
    @open="emit('open')"
    @closed="onClosed"
  >
    <template v-if="props.title || props.subtitle || props.icon || $slots.header" #header>
      <div class="flex items-start gap-3">
        <div
          v-if="props.icon"
          class="flex size-9 shrink-0 items-center justify-center rounded-ctl bg-ui-50 text-ui-600"
        >
          <el-icon class="text-lg"><component :is="props.icon" /></el-icon>
        </div>
        <div class="min-w-0 flex-1">
          <slot name="header">
            <h3 class="m-0 text-base font-semibold text-ink">{{ props.title }}</h3>
            <p v-if="props.subtitle" class="mt-1 mb-0 text-xs leading-normal text-ink-3">
              {{ props.subtitle }}
            </p>
          </slot>
        </div>
      </div>
    </template>

    <slot />

    <template v-if="!props.hideFooter" #footer>
      <slot name="footer">
        <div class="flex items-center justify-end gap-2 max-md:flex-col-reverse">
          <UiButton v-if="props.showCancel" class="max-md:w-full" @click="onCancel">
            {{ props.cancelText }}
          </UiButton>
          <UiButton
            variant="primary"
            class="max-md:w-full"
            :loading="props.confirmLoading"
            :disabled="props.confirmDisabled"
            @click="onConfirm"
          >
            {{ props.confirmText }}
          </UiButton>
        </div>
      </slot>
    </template>
  </el-dialog>
</template>

<style scoped>
/*
 * 仅改 EP 内部结构（R1 允许）：头部/页脚加分隔线并统一内边距，
 * body 超长时内部滚动而不是把整个弹窗顶出视口。
 *
 * 头部右侧留 44px 给 EP 绝对定位的关闭按钮，避免长标题被 × 压住。
 */
.ui-dialog :deep(.el-dialog__header) {
  padding: 18px 44px 14px 20px;
  margin-right: 0;
  border-bottom: 1px solid var(--color-line);
}

.ui-dialog :deep(.el-dialog__body) {
  padding: 20px;
  max-height: 60vh;
  overflow-y: auto;
}

.ui-dialog :deep(.el-dialog__footer) {
  padding: 14px 20px 18px;
  border-top: 1px solid var(--color-line);
}
</style>
