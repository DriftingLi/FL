<script setup lang="ts">
import { computed } from 'vue'

/**
 * 骨架屏：包 `el-skeleton`，按场景给出 5 种形状预设，避免每处手写 el-skeleton-item。
 *
 * 本组件恒为占位态（:loading="true"），真实内容的切换由调用方用 v-if 控制。
 */
const props = withDefaults(
  defineProps<{
    variant?: 'text' | 'list' | 'card' | 'chart' | 'table'
    /** list / card 的条目数；table 的数据行数 */
    count?: number
    /** text / list 每个条目的行数 */
    rows?: number
    animated?: boolean
  }>(),
  { variant: 'text', count: 3, rows: 2, animated: true }
)

/** 占位块基础样式：animated 时叠加脉冲，避免静态底色看起来像渲染完成 */
const block = computed(() => `rounded-ctl bg-canvas${props.animated ? ' animate-pulse' : ''}`)
</script>

<template>
  <el-skeleton :animated="props.animated" :loading="true" :count="1">
    <template #template>
      <!-- 大块图表占位 -->
      <div v-if="props.variant === 'chart'" :class="[block, 'h-52 w-full rounded-card']" />

      <!-- 表格：表头 + N 行 -->
      <div v-else-if="props.variant === 'table'" class="w-full">
        <div :class="[block, 'mb-3 h-8 w-full']" />
        <div
          v-for="i in props.count"
          :key="i"
          :class="[block, 'mb-2 h-10 w-full']"
        />
      </div>

      <!-- 卡片网格 -->
      <div v-else-if="props.variant === 'card'" class="grid gap-4 sm:grid-cols-2">
        <div
          v-for="i in props.count"
          :key="i"
          class="rounded-card border border-line p-4"
        >
          <div :class="[block, 'h-24 w-full']" />
          <div :class="[block, 'mt-3 h-4 w-2/3']" />
          <div :class="[block, 'mt-2 h-3 w-1/2']" />
        </div>
      </div>

      <!-- 列表：头像 + 多行文字 -->
      <div v-else-if="props.variant === 'list'" class="w-full">
        <div
          v-for="i in props.count"
          :key="i"
          class="mb-4 flex items-start gap-3"
        >
          <div :class="[block, 'h-9 w-9 shrink-0']" />
          <div class="flex-1">
            <div
              v-for="r in props.rows"
              :key="r"
              :class="[block, 'mb-2 h-3', r === props.rows ? 'w-1/2' : 'w-full']"
            />
          </div>
        </div>
      </div>

      <!-- 纯文本行 -->
      <div v-else class="w-full">
        <div
          v-for="r in props.rows"
          :key="r"
          :class="[block, 'mb-2 h-3', r === props.rows ? 'w-2/3' : 'w-full']"
        />
      </div>
    </template>
  </el-skeleton>
</template>
