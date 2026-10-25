<script setup lang="ts">
/**
 * 图标唯一入口（ADR-0079）。
 *
 * - **静态**是默认路径：直接把注册表里的 `[tag, attrs]` 渲染成 SVG，不加载任何动画模块；
 * - `morph` 打开时改走 morphicons，`name` 变化即从屏幕上的形状弹簧过渡到目标形状；
 * - 尺寸默认 `1em`，与 Element Plus 图标同口径（继承字号），迁移时零视觉位移；
 * - 没有 `label` 即装饰性图形（`aria-hidden`），有 `label` 则 `role="img"` + `<title>`。
 */
import { computed, onMounted, shallowRef, watch, watchEffect, type Component } from 'vue'
import type { IconInput } from 'morphicons'
import { resolveIcon, type IconName } from '@/icons/registry'
import {
  UI_REDUCED_MOTION_DEFAULT,
  UI_SPRING_DEFAULT,
  type UiReducedMotion,
  type UiSpring
} from '@/config/motion'

const props = withDefaults(
  defineProps<{
    /** 注册表里的语义名 */
    name: IconName
    /** 边长：数字为 px，字符串原样传入（默认 `1em`，跟随字号） */
    size?: number | string
    strokeWidth?: number
    /** 可访问名；缺省表示装饰性图标 */
    label?: string
    /** 打开后 `name` 的变化走弹簧形变；默认关闭（静态路径不加载动画模块） */
    morph?: boolean
    spring?: UiSpring
    reducedMotion?: UiReducedMotion
  }>(),
  {
    size: '1em',
    strokeWidth: 2,
    label: undefined,
    morph: false,
    spring: undefined,
    reducedMotion: UI_REDUCED_MOTION_DEFAULT
  }
)

const resolved = computed(() => resolveIcon(props.name))
const dimension = computed(() => (typeof props.size === 'number' ? `${props.size}px` : props.size))
const morphNode = computed<IconInput>(() => resolved.value.node as unknown as IconInput)

if (import.meta.env.DEV) {
  watchEffect(() => {
    if (resolved.value.fallback) {
      // 名字是契约：类型面已拦下绝大多数，这里兜运行期数据（后端下发、动态拼接）的漏网
      console.warn(`[UiIcon] 未登记的图标名 "${String(props.name)}"，已降级为 circle-alert`)
    }
  })
}

/**
 * 形变模块按需加载：只有声明了 `morph` 的调用点才会拉这一块（≈14 KB gzip），
 * 静态用法（绝大多数）不付这份流量。挂载后立刻预加载，用户点下去时已经就绪。
 */
const MorphImpl = shallowRef<Component | null>(null)
let started = false

function loadMorphImpl(): void {
  if (started) return
  started = true
  void import('./UiIconMorph.vue').then((mod) => {
    MorphImpl.value = mod.default
  })
}

onMounted(() => {
  if (props.morph) void loadMorphImpl()
})

// morph 可能在挂载之后才打开（父级按条件切换）：不补这一条，false → true 会永远静默走静态分支
watch(
  () => props.morph,
  (on) => {
    if (on) void loadMorphImpl()
  }
)
</script>

<template>
  <component
    :is="MorphImpl"
    v-if="morph && MorphImpl"
    :icon="morphNode"
    :size="dimension"
    :stroke-width="strokeWidth"
    :label="label"
    :spring="spring ?? UI_SPRING_DEFAULT"
    :reduced-motion="reducedMotion"
  />
  <svg
    v-else
    :width="dimension"
    :height="dimension"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    :stroke-width="strokeWidth"
    stroke-linecap="round"
    stroke-linejoin="round"
    :role="label ? 'img' : undefined"
    :aria-hidden="label ? undefined : 'true'"
  >
    <title v-if="label">{{ label }}</title>
    <component :is="child[0]" v-for="(child, index) in resolved.node" :key="index" v-bind="child[1]" />
  </svg>
</template>
