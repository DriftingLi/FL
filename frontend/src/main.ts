import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
// 自托管字体（@fontsource，替代 Google Fonts 渲染阻塞外链 —— #748 P0-2）
import '@fontsource/dm-sans/400.css'
import '@fontsource/dm-sans/400-italic.css'
import '@fontsource/dm-sans/500.css'
import '@fontsource/dm-sans/600.css'
import '@fontsource/dm-sans/700.css'
import '@fontsource/jetbrains-mono/400.css'
import '@fontsource/jetbrains-mono/500.css'
import '@fontsource/noto-sans-sc/400.css'
import '@fontsource/noto-sans-sc/500.css'
import '@fontsource/noto-sans-sc/600.css'
import '@fontsource/noto-sans-sc/700.css'
// 样式统一由 tailwind.css 按 @layer 顺序引入（vendor → theme → base → utilities）
import './assets/styles/tailwind.css'

import App from './App.vue'
import router from './router'
import icons from './icons'
import { useAuthStore } from './stores/auth'
import { useThemeStore } from './stores/theme'

const app = createApp(App)
const pinia = createPinia()

for (const [key, component] of Object.entries(icons)) {
  app.component(key, component)
}

app.use(pinia)

// 认证初始化：localStorage 恢复 + /auth/me 校验 + URL auth_token 交接（幂等，路由守卫 await 同一 Promise）
useAuthStore().initialize()

// 主题初始化：首屏状态已由 index.html 内联脚本设好，这里补上系统主题联动监听与手动切换入口
useThemeStore()

app.use(router)
app.use(ElementPlus)

app.mount('#app')
