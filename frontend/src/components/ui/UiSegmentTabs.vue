<script setup lang="ts">
/**
 * 分段选项卡（segment tabs）：滑动指示条 + 激活项浮起，替代散落各页的 el-tabs/手搓胶囊。
 *
 * 交互：原生 button（键盘可达）；激活项白底浮起 + 滑动指示过渡；
 * prefers-reduced-motion 下过渡直跳（动效克制，交给全局媒体查询）。
 * 等分模式（equal）：窄容器内各选项等宽铺满。
 */
import { ref, watch, onMounted, nextTick } from 'vue'

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

// 激活变化/容器尺寸变化时重定位
watch(
  () => props.modelValue,
  async () => {
    await nextTick()
    placeIndicator()
  }
)

onMounted(() => {
  void nextTick().then(placeIndicator)
  if (typeof ResizeObserver !== 'undefined') {
    const ro = new ResizeObserver(() => placeIndicator())
    ro.observe(barRef.value as Element)
    // 无 unmount 钩子需清理的场景：组件卸载时 RO 自动断
  }
})
</script>

<template>
  <div
    ref="barRef"
    class="relative inline-flex max-w-full items-center gap-1 rounded-ctl bg-canvas p-1"
    :class="disabled ? 'opacity-60' : ''"
    role="tablist"
  >
    <!-- 滑动指示条：left/top 归零，translate 跟随激活项几何，避免 Tailwind 原子类计算偏差 -->
    <div
      aria-hidden="true"
      class="absolute left-0 top-0 rounded-[7px] bg-panel shadow-sm transition-[transform,width,height] duration-200 ease-out motion-reduce:transition-none"
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
      class="relative z-10 cursor-pointer rounded-[7px] px-3.5 py-1.5 text-xs font-medium whitespace-nowrap transition-colors duration-150"
      :class="modelValue === opt.value ? 'text-ink' : 'text-ink-3 hover:text-ink-2'"
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
