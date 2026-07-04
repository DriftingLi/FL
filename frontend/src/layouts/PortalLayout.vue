<template>
  <div class="portal-layout">
    <AppNavbar :menu-items="menuItems" />
    <main class="portal-main">
      <div class="portal-content">
        <router-view />
      </div>
    </main>
    <AppFooter v-if="!hideFooter" />
  </div>
</template>

<script setup lang="ts">
import AppNavbar from '@/components/layout/AppNavbar.vue'
import AppFooter from '@/components/layout/AppFooter.vue'
import type { NavItem } from '@/config/navigation'

withDefaults(defineProps<{
  menuItems: NavItem[]
  hideFooter?: boolean
}>(), {
  hideFooter: false
})
</script>

<style scoped>
.portal-layout {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--color-bg-page);
}

.portal-main {
  flex: 1;
  padding: var(--space-6) var(--space-4);
}

.portal-content {
  max-width: var(--container-page);
  margin: 0 auto;
  width: 100%;
}

@media screen and (max-width: 768px) {
  .portal-main {
    padding: var(--space-4) var(--space-3);
  }
}

@media screen and (max-width: 480px) {
  .portal-main {
    padding: var(--space-3) var(--space-2);
  }
}
</style>
