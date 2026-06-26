import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import './assets/styles/design-tokens.css'
import 'element-plus/dist/index.css'
import './assets/styles/element-overrides.css'
import './assets/styles/global.css'
import './assets/styles/valuation-tokens.css'

import App from './App.vue'
import router from './router'
import icons from './icons'

const app = createApp(App)
const pinia = createPinia()

for (const [key, component] of Object.entries(icons)) {
  app.component(key, component)
}

app.use(pinia)
app.use(router)
app.use(ElementPlus)

app.mount('#app')
