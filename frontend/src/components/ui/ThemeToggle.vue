<template>
  <el-dropdown trigger="click" @command="onCommand">
    <button
      type="button"
      class="theme-toggle-btn flex cursor-pointer items-center justify-center rounded-pill transition-all duration-[var(--duration-tap)] ease-[var(--ease-default)] active:scale-[0.94]"
      :class="[variant === 'ghost' ? ghostClass : cardClass, fixed ? fixedClass : '']"
      aria-label="切换主题"
      title="切换主题"
    >
      <el-icon v-if="themeStore.resolved === 'dark'" :size="18"><Moon /></el-icon>
      <svg v-else-if="themeStore.mode === 'system'" class="h-[18px] w-[18px]" viewBox="0 0 24 24" aria-hidden="true">
        <path d="M12 3a9 9 0 0 0 0 18V3Z" fill="currentColor" opacity="0.9"/>
        <path d="M12 6a6 6 0 0 0 0 12 8 8 0 0 1 0-12Z" fill="var(--color-bg-card)"/>
        <path d="M12 3a9 9 0 0 1 9 9 9 9 0 0 1-9 9V3Z" fill="none" stroke="currentColor" stroke-width="1.5"/>
        <path d="M16.8 6.2l1.4-1.4M21 12h2M16.8 17.8l1.4 1.4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
      </svg>
      <el-icon v-else :size="18"><Sunny /></el-icon>
    </button>
    <template #dropdown>
      <el-dropdown-menu>
        <el-dropdown-item command="light">
          <span class="flex items-center gap-2">
            <el-icon class="w-4" :size="14"><Sunny /></el-icon>浅色
            <el-icon v-if="themeStore.mode === 'light'" class="ml-auto text-ui-600" :size="14"><Check /></el-icon>
          </span>
        </el-dropdown-item>
        <el-dropdown-item command="dark">
          <span class="flex items-center gap-2">
            <el-icon class="w-4" :size="14"><Moon /></el-icon>深色
            <el-icon v-if="themeStore.mode === 'dark'" class="ml-auto text-ui-600" :size="14"><Check /></el-icon>
          </span>
        </el-dropdown-item>
        <el-dropdown-item command="system">
          <span class="flex items-center gap-2">
            <el-icon class="w-4" :size="14"><Monitor /></el-icon>跟随系统
            <el-icon v-if="themeStore.mode === 'system'" class="ml-auto text-ui-600" :size="14"><Check /></el-icon>
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
 * - variant="card"（默认）：白底描边阴影圆按钮 —— SidebarLayout / ValuationLayout / AuthPageShell 原外观
 * - variant="ghost"：透明无边框、hover 反色 —— ChatPageShell 悬浮胶囊与侧栏 header 内嵌用
 * - fixed：附加右上角固定定位（SidebarLayout / AuthPageShell 需悬浮于内容区；移动端加大触控目标）
 */
import { Moon, Sunny, Check, Monitor } from '@element-plus/icons-vue'
import { useThemeStore, type ThemeMode } from '@/stores/theme'

withDefaults(
  defineProps<{
    variant?: 'card' | 'ghost'
    fixed?: boolean
  }>(),
  { variant: 'card', fixed: false }
)

const themeStore = useThemeStore()

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
