<template>
  <div class="portal-home-layout">
    <PortalNavbar :menu-items="menuItems" />
    <main class="portal-main">
      <router-view />
    </main>
    <PortalFooter />
  </div>
</template>

<script setup lang="ts">
import { onMounted, watch, nextTick } from 'vue'
import { useRoute } from 'vue-router'
import PortalNavbar from '@/components/layout/PortalNavbar.vue'
import PortalFooter from '@/components/layout/PortalFooter.vue'
import { roleNavigation } from '@/config/navigation'

const menuItems = roleNavigation.portal
const route = useRoute()

async function scrollToHash(hash: string) {
  await nextTick()
  // 等待页面渲染完毕
  setTimeout(() => {
    const el = document.getElementById(hash)
    if (el) {
      el.scrollIntoView({ behavior: 'smooth' })
    }
  }, 50)
}

onMounted(() => {
  if (route.hash) {
    scrollToHash(route.hash.slice(1))
  }
})

watch(() => route.hash, (hash) => {
  if (hash) {
    scrollToHash(hash.slice(1))
  } else {
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }
})
</script>

<style scoped>
.portal-home-layout {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--color-bg-page);
}
.portal-main {
  flex: 1;
}
</style>
