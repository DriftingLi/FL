<template>
  <div class="valuation-layout">
    <header class="valuation-navbar">
      <div class="navbar-container">
        <router-link to="/" class="logo-link">
          <img src="/images/HRWAIlogo.jpg" alt="和润天下" class="logo-img" />
          <div class="logo-text-wrap">
            <span class="logo-text">和润天下</span>
            <span class="logo-sub">残值评估</span>
          </div>
        </router-link>

        <nav class="nav-links">
          <router-link to="/valuation" class="nav-link" :class="{ active: isActive('/valuation') }">评估首页</router-link>
          <router-link to="/valuation/input" class="nav-link" :class="{ active: isActive('/valuation/input') }">整车评估</router-link>
          <router-link to="/valuation/battery" class="nav-link" :class="{ active: isActive('/valuation/battery') }">电池评估</router-link>
          <router-link v-if="isLoggedIn" to="/valuation/history" class="nav-link" :class="{ active: isActive('/valuation/history') }">评估历史</router-link>
        </nav>

        <div class="nav-cta">
          <template v-if="!isLoggedIn">
            <router-link :to="{ path: '/login', query: { redirect: '/valuation/history' } }" class="btn-login">登录</router-link>
          </template>
          <template v-else>
            <router-link to="/" class="btn-back">返回官网</router-link>
          </template>
        </div>

        <button
          class="hamburger"
          :class="{ open: mobileOpen }"
          @click="mobileOpen = !mobileOpen"
          aria-label="菜单"
        >
          <span></span>
          <span></span>
          <span></span>
        </button>
      </div>

      <transition name="mobile-slide">
        <div v-if="mobileOpen" class="mobile-menu">
          <router-link to="/valuation" class="mobile-link" @click="mobileOpen = false">评估首页</router-link>
          <router-link to="/valuation/input" class="mobile-link" @click="mobileOpen = false">整车评估</router-link>
          <router-link to="/valuation/battery" class="mobile-link" @click="mobileOpen = false">电池评估</router-link>
          <router-link v-if="isLoggedIn" to="/valuation/history" class="mobile-link" @click="mobileOpen = false">评估历史</router-link>
          <div class="mobile-cta">
            <router-link v-if="!isLoggedIn" :to="{ path: '/login', query: { redirect: '/valuation/history' } }" class="btn-login" @click="mobileOpen = false">登录</router-link>
            <router-link v-else to="/" class="btn-back" @click="mobileOpen = false">返回官网</router-link>
          </div>
        </div>
      </transition>
    </header>

    <main class="valuation-main">
      <div class="valuation-content">
        <router-view />
      </div>
    </main>

    <footer class="valuation-footer">
      <div class="footer-container">
        <p class="footer-text">© {{ year }} 和润天下人工智能科技有限公司</p>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const authStore = useAuthStore()
const mobileOpen = ref(false)

const isLoggedIn = computed(() => !!(authStore.token && authStore.isLoggedIn && authStore.userInfo?.role))
const year = new Date().getFullYear()

function isActive(path: string): boolean {
  return route.path === path
}
</script>

<style scoped>
.valuation-layout {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--color-bg-page);
}

.valuation-navbar {
  position: sticky;
  top: 0;
  z-index: var(--z-sticky);
  background: var(--color-bg-card);
  border-bottom: 1px solid var(--color-border);
  box-shadow: var(--shadow-xs);
}

.navbar-container {
  max-width: var(--container-page);
  margin: 0 auto;
  padding: 0 var(--space-6);
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 64px;
  gap: var(--space-6);
}

.logo-link {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  text-decoration: none;
  flex-shrink: 0;
}

.logo-img {
  width: 36px;
  height: 36px;
  border-radius: var(--radius-md);
  object-fit: cover;
}

.logo-text-wrap {
  display: flex;
  flex-direction: column;
  line-height: 1;
}

.logo-text {
  font-family: var(--font-display);
  font-size: var(--text-lg);
  font-weight: var(--font-bold);
  color: var(--color-text-primary);
  letter-spacing: -0.025em;
}

.logo-sub {
  font-size: 10px;
  color: var(--color-text-tertiary);
  letter-spacing: 0.15em;
  text-transform: uppercase;
  margin-top: 2px;
}

.nav-links {
  display: none;
  list-style: none;
  margin: 0;
  padding: 0;
  gap: var(--space-6);
  align-items: center;
  flex: 1;
  justify-content: center;
}

.nav-link {
  font-family: var(--font-body);
  font-size: var(--text-sm);
  font-weight: var(--font-medium);
  color: var(--color-text-secondary);
  text-decoration: none;
  transition: color var(--duration-fast);
  cursor: pointer;
  position: relative;
  padding: var(--space-2) 0;
}

.nav-link:hover,
.nav-link.active {
  color: var(--color-primary-600);
}

.nav-link.active::after {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  bottom: -4px;
  height: 2px;
  background: var(--gradient-brand);
  border-radius: 2px;
}

.nav-cta {
  display: none;
  gap: var(--space-3);
  flex-shrink: 0;
}

.btn-login {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 40px;
  padding: 8px 20px;
  background: var(--gradient-brand);
  color: #fff;
  border: none;
  border-radius: var(--radius-md);
  font-size: var(--text-sm);
  font-weight: var(--font-medium);
  text-decoration: none;
  transition: opacity var(--duration-fast);
}

.btn-login:hover {
  opacity: 0.92;
}

.btn-back {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 40px;
  padding: 8px 20px;
  background: transparent;
  color: var(--color-text-secondary);
  border: 1px solid var(--color-border-dark);
  border-radius: var(--radius-md);
  font-size: var(--text-sm);
  font-weight: var(--font-medium);
  text-decoration: none;
  transition: all var(--duration-fast);
}

.btn-back:hover {
  border-color: var(--color-primary-500);
  color: var(--color-primary-600);
}

.hamburger {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 5px;
  width: 36px;
  height: 36px;
  background: none;
  border: none;
  cursor: pointer;
  padding: 0;
}

.hamburger span {
  display: block;
  width: 22px;
  height: 2px;
  background: var(--color-text-primary);
  border-radius: 1px;
  transition: all var(--duration-normal);
}

.hamburger.open span:nth-child(1) {
  transform: rotate(45deg) translate(5px, 5px);
}

.hamburger.open span:nth-child(2) {
  opacity: 0;
}

.hamburger.open span:nth-child(3) {
  transform: rotate(-45deg) translate(5px, -5px);
}

.mobile-menu {
  display: flex;
  flex-direction: column;
  background: var(--color-bg-card);
  padding: var(--space-4) var(--space-6);
  border-top: 1px solid var(--color-border);
}

.mobile-link {
  display: block;
  font-size: var(--text-base);
  font-weight: var(--font-medium);
  color: var(--color-text-primary);
  text-decoration: none;
  padding: var(--space-4) 0;
  border-bottom: 1px solid var(--color-border-light);
}

.mobile-link:last-of-type {
  border-bottom: none;
}

.mobile-cta {
  display: flex;
  gap: var(--space-3);
  margin-top: var(--space-5);
}

.mobile-cta .btn-login,
.mobile-cta .btn-back {
  flex: 1;
  min-height: 44px;
  text-align: center;
}

.mobile-slide-enter-active,
.mobile-slide-leave-active {
  transition: opacity var(--duration-normal), transform var(--duration-normal);
}

.mobile-slide-enter-from,
.mobile-slide-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}

.valuation-main {
  flex: 1;
  padding: var(--space-6) var(--space-4);
}

.valuation-content {
  max-width: var(--container-page);
  margin: 0 auto;
  width: 100%;
}

.valuation-footer {
  background: var(--surface-dark);
  padding: var(--space-6) var(--space-4);
  border-top: 1px solid var(--color-border-darker);
}

.footer-container {
  max-width: var(--container-page);
  margin: 0 auto;
  text-align: center;
}

.footer-text {
  font-size: var(--text-xs);
  color: var(--color-text-muted);
  margin: 0;
}

@media (min-width: 768px) {
  .nav-links {
    display: flex;
  }
  .nav-cta {
    display: flex;
  }
  .hamburger {
    display: none;
  }
  .mobile-menu {
    display: none !important;
  }
}

@media (max-width: 767px) {
  .hamburger {
    display: flex;
  }
  .navbar-container {
    padding: 0 var(--space-4);
    height: 56px;
  }
  .logo-text {
    font-size: var(--text-base);
  }
  .valuation-main {
    padding: var(--space-4) var(--space-3);
  }
}
</style>
