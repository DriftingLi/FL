<template>
  <el-dropdown trigger="click" @command="onCommand">
    <button
      type="button"
      class="theme-toggle-btn flex cursor-pointer items-center justify-center rounded-pill transition-all duration-[var(--duration-tap)] ease-[var(--ease-default)] active:scale-[0.94]"
      :class="[variant === 'ghost' ? ghostClass : cardClass, fixed ? fixedClass : '']"
      aria-label="切换主题"
      title="切换主题"
    >
      <!-- 主题三态是同一个位置上的三张图形：形状变了，所以走形变（ADR-0079） -->
      <UiIcon :name="themeIcon" :size="18" morph />
    </button>
    <template #dropdown>
      <el-dropdown-menu>
        <el-dropdown-item command="light">
          <span class="flex items-center gap-2">
            <UiIcon name="sun" :size="14" />浅色
            <UiIcon v-if="themeStore.mode === 'light'" name="check" class="ml-auto text-ui-soft-ink" :size="14" />
          </span>
        </el-dropdown-item>
        <el-dropdown-item command="dark">
          <span class="flex items-center gap-2">
            <UiIcon name="moon" :size="14" />深色
            <UiIcon v-if="themeStore.mode === 'dark'" name="check" class="ml-auto text-ui-soft-ink" :size="14" />
          </span>
        </el-dropdown-item>
        <el-dropdown-item command="system">
          <span class="flex items-center gap-2">
            <UiIcon name="theme-system" :size="14" />跟随系统
            <UiIcon v-if="themeStore.mode === 'system'" name="check" class="ml-auto text-ui-soft-ink" :size="14" />
          </span>
        </el-dropdown-item>
      </el-dropdown-menu>
    </template>
  </el-dropdown>
</template>

<script setup lang="ts">
/**
 * 主题切换按钮（全站统一）：三态下拉菜单（浅色/深色/跟随系统，当前项打勾），
 * 替代旧的 cycle() 循环按钮（交互对齐 DeepSeek）。
 *
 * - variant="card"（默认）：白底描边阴影圆按钮 —— SidebarLayout / AuthPageShell 原外观
 * - variant="ghost"：透明无边框、hover 反色 —— ChatPageShell 悬浮胶囊与侧栏 header 内嵌用
 * - fixed：附加右上角固定定位（SidebarLayout / AuthPageShell 需悬浮于内容区；移动端加大触控目标）
 */
import { computed } from 'vue'
import { useThemeStore, type ThemeMode } from '@/stores/theme'
import type { IconName } from '@/icons/registry'
import UiIcon from './UiIcon.vue'

withDefaults(
  defineProps<{
    variant?: 'card' | 'ghost'
    fixed?: boolean
  }>(),
  { variant: 'card', fixed: false }
)

const themeStore = useThemeStore()

/**
 * 触发器的图形 = 当前外观三态之一：位置固定、只有形状变，正是形变的用场。
 * 「跟随系统」优先于 resolved —— 否则系统处在深色时会被伪装成「深色」。
 */
const themeIcon = computed<IconName>(() => {
  if (themeStore.mode === 'system') return 'theme-system'
  return themeStore.resolved === 'dark' ? 'moon' : 'sun'
})

// 两种触发器外观（类串集中定义，色值全走 token）
const cardClass =
  'h-9 w-9 border border-line bg-panel text-ink shadow-card hover:bg-canvas'
const ghostClass =
  'h-8 w-8 border-0 bg-transparent text-ink-3 hover:bg-canvas hover:text-ink'
// fixed 附加：右上角固定 + 移动端加大触控目标（对齐旧 .theme-toggle 移动段）
const fixedClass =
  'fixed right-4 top-4 z-[var(--z-sticky)] max-[768px]:h-11 max-[768px]:w-11'

function onCommand(mode: ThemeMode) {
  themeStore.setMode(mode)
}
</script>

<style scoped>
/* 触发器尺寸（R1 允许：仅按钮本体尺寸，外观类由 script 常量绑定） */
.theme-toggle-btn {
  width: 36px;
  height: 36px;
}
</style>
