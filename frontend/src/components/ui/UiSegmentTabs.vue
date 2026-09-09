<script setup lang="ts">
/**
 * 分段选项卡（segment tabs）：滑动指示条 + 激活项浮起，替代散落各页的 el-tabs/手搓胶囊。
 *
 * 交互：原生 button（键盘可达）；激活项浮起 + 滑动指示过渡；
 * prefers-reduced-motion 下过渡由全局媒体查询压平（见 global.css）。
 * 等分模式（equal）：窄容器内各选项等宽铺满。
 *
 * ⚠️ 配色约束（改动前必读）：
 * 1. 激活指示条**不能**用 bg-panel —— 它与最常见的父容器（卡片）同值，放进卡片里彻底隐身
 *    （浅色 #FFFFFF on #FFFFFF、深色 #1E293B on #1E293B）。
 * 2. 也**不能**用浅底档（bg-ui-50/100 = #F0FDFA/#CCFBF1）—— 压在 bg-canvas 灰胶囊底上
 *    亮度差太小，肉眼就是一块灰白，等于没改（2026-09-08 首版踩过，用户反馈「还是灰色」）。
 * 用实心品牌色 bg-ui-500 + 反白字：两套主题下都与背景有强对比。深色模式里它与
 * 品牌按钮同色系，属既有视觉语言，不突兀。
 */
import { ref, watch, onMounted, onUnmounted, nextTick } from 'vue'

export interface UiSegmentOption {
  label: string
  value: string
}

const props = withDefaults(
  defineProps<{
    modelValue: string
    options: UiSegmentOption[]
    /** 等分模式：选项均分容器宽 */
    equal?: boolean
    disabled?: boolean
  }>(),
  {
    equal: false,
    disabled: false
  }
)

const emit = defineEmits<{
  (e: 'update:modelValue', v: string): void
  (e: 'change', v: string): void
}>()

const barRef = ref<HTMLElement | null>(null)
interface Indicator { width: number; height: number; x: number; y: number }
const indicator = ref<Indicator>({ width: 0, height: 0, x: 0, y: 0 })

/**
 * 首帧定位完成前不加过渡。
 * indicator 初值为 0，若一开始就带 transition，滑块会从左上角以「尺寸 0 生长」的方式
 * 滑到激活项 —— 那不是设计中的动效。定位落定后再放开过渡。
 */
const ready = ref(false)

/** 定位指示条贴齐激活项几何（offsetLeft/Top/Width/Height 直接给绝对定位元素用，免疫 token 漂移） */
function placeIndicator() {
  const bar = barRef.value
  if (!bar) return
  const active = bar.querySelector('[aria-selected="true"]') as HTMLElement | null
  if (!active) return
  indicator.value = {
    width: active.offsetWidth,
    height: active.offsetHeight,
    x: active.offsetLeft,
    y: active.offsetTop
  }
}

function select(opt: UiSegmentOption) {
  if (props.disabled) return
  emit('update:modelValue', opt.value)
  emit('change', opt.value)
}

// 激活变化时重定位
watch(
  () => props.modelValue,
  async () => {
    await nextTick()
    placeIndicator()
  }
)

// 选项集合变化后几何也会变（ResizeObserver 只响应尺寸，不响应选项增删）
watch(
  () => props.options,
  async () => {
    await nextTick()
    placeIndicator()
  },
  { deep: true }
)

let ro: ResizeObserver | null = null

onMounted(() => {
  void nextTick().then(() => {
    placeIndicator()
    // 再等一帧才放开过渡：确保首帧的定位已作为无动画的初值生效
    requestAnimationFrame(() => {
      ready.value = true
    })
  })
  if (typeof ResizeObserver !== 'undefined' && barRef.value) {
    ro = new ResizeObserver(() => placeIndicator())
    ro.observe(barRef.value)
  }
})

onUnmounted(() => {
  // Vue 卸载不会自动断开 RO，必须显式 disconnect
  ro?.disconnect()
  ro = null
})
</script>

<template>
  <div
    ref="barRef"
    class="seg-bar relative inline-flex max-w-full items-center gap-1 overflow-x-auto rounded-ctl bg-canvas p-1 [scrollbar-width:none]"
    :class="disabled ? 'opacity-60' : ''"
    role="tablist"
  >
    <!--
      滑动指示条：left/top 归零，translate 跟随激活项几何，避免 Tailwind 原子类计算偏差。
      圆角 4px = 容器 rounded-ctl(8px) - p-1(4px) 的内圈，写死 7px 会顶出容器圆角。
    -->
    <div
      aria-hidden="true"
      class="absolute left-0 top-0 rounded-[4px] bg-ui-500 shadow-sm"
      :class="
        ready
          ? 'transition-[transform,width,height] duration-200 ease-out motion-reduce:transition-none'
          : ''
      "
      :style="{
        width: indicator.width + 'px',
        height: indicator.height + 'px',
        transform: 'translate(' + indicator.x + 'px,' + indicator.y + 'px)'
      }"
    />
    <button
      v-for="opt in options"
      :key="opt.value"
      type="button"
      role="tab"
      :aria-selected="modelValue === opt.value"
      class="relative z-10 cursor-pointer rounded-[4px] px-3.5 py-1.5 text-xs font-medium whitespace-nowrap transition-colors duration-150"
      :class="modelValue === opt.value ? 'text-white' : 'text-ink-3 hover:text-ink-2'"
      :style="equal ? { flex: '1 1 0' } : undefined"
      @click="select(opt)"
    >
      <!-- 自定义富 label（如带图标/计数）：覆盖默认 opt.label；不传则按 opt.label 渲染，向后兼容 -->
      <slot name="option" :option="opt" :is-active="modelValue === opt.value">
        {{ opt.label }}
      </slot>
    </button>
  </div>
</template>

<style scoped>
/* 选项过多时容器横向滚动（如搜索页「全部分类」），隐藏滚动条避免视觉噪音 */
.seg-bar::-webkit-scrollbar {
  display: none;
}
</style>
