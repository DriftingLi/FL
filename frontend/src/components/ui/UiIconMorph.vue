<script setup lang="ts">
/**
 * 形变适配层：本仓唯一**在运行时** import morphicons 的文件（`UiIcon` 只有一句 `import type`，
 * 编译后擦除）。
 *
 * 由 `UiIcon` 在 `morph` 开启时动态 import —— 静态图标用法不下载这一块。
 * props 只是转交，判定（名字解析、尺寸口径、降级）都在 `UiIcon` 里。
 */
import { MorphIcon } from 'morphicons/vue'
import type { IconInput } from 'morphicons'
import type { UiReducedMotion, UiSpring } from '@/config/motion'

defineProps<{
  icon: IconInput
  size: string | number
  strokeWidth: number
  label?: string
  spring: UiSpring
  reducedMotion: UiReducedMotion
}>()
</script>

<template>
  <MorphIcon
    :icon="icon"
    :size="size"
    :stroke-width="strokeWidth"
    :label="label"
    :spring="spring"
    :reduced-motion="reducedMotion"
  />
</template>
