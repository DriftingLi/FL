import { ref, computed } from 'vue'
import { defineStore } from 'pinia'

export type ThemeMode = 'system' | 'light' | 'dark'

const STORAGE_KEY = 'hrwai-theme-mode'

/**
 * 三态主题：system（跟随系统，默认）/ light / dark（手动覆盖）。
 *
 * 生效方式（两路，缺一不可）：
 * 1. `documentElement.dataset.theme` —— design-tokens.css / element-overrides.css
 *    的 [data-theme="dark"] 覆盖块 + Tailwind v4 的 dark: 变体都靠它。
 * 2. `documentElement.classList.toggle('dark')` —— EP 官方暗色表
 *    （element-plus/theme-chalk/dark/css-vars.css）挂在 .dark 选择器下。
 *
 * 防闪：index.html 的 <head> 内联脚本在 CSS/JS 加载前就设好两路状态，
 * 本 store 只负责「跟系统联动 + 手动切换 + 持久化」，不重复做首屏初始化。
 */
export const useThemeStore = defineStore('theme', () => {
  const mql = window.matchMedia('(prefers-color-scheme: dark)')

  const mode = ref<ThemeMode>(readStoredMode())
  const resolved = computed<'light' | 'dark'>(() =>
    mode.value === 'system' ? (mql.matches ? 'dark' : 'light') : mode.value
  )

  function apply(resolvedTheme: 'light' | 'dark') {
    const root = document.documentElement
    root.dataset.theme = resolvedTheme
    root.classList.toggle('dark', resolvedTheme === 'dark')
  }

  /** 手动切换：system → light → dark 循环 */
  function cycle() {
    const order: ThemeMode[] = ['system', 'light', 'dark']
    setMode(order[(order.indexOf(mode.value) + 1) % order.length])
  }

  function setMode(next: ThemeMode) {
    mode.value = next
    localStorage.setItem(STORAGE_KEY, next)
    apply(resolved.value)
  }

  function onSystemChange(e: MediaQueryListEvent) {
    if (mode.value === 'system') apply(e.matches ? 'dark' : 'light')
  }

  // 同步一次（应对 index.html 内联脚本之后用户手动改过系统主题的窗口期）
  apply(resolved.value)
  mql.addEventListener('change', onSystemChange)

  return { mode, resolved, cycle, setMode }
})

function readStoredMode(): ThemeMode {
  try {
    const v = localStorage.getItem(STORAGE_KEY)
    return v === 'light' || v === 'dark' ? v : 'system'
  } catch {
    return 'system'
  }
}
