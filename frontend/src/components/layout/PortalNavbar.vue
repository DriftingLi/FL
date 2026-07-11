<template>
  <nav class="portal-navbar" :class="{ scrolled }">
    <div class="navbar-container">
      <!-- Logo -->
      <a :href="homeUrl" class="logo-link" @click.prevent="goHome">
        <img src="/images/HRWAIlogo.jpg" alt="和润天下" class="logo-img" />
        <div class="logo-text-wrap">
          <span class="logo-text">和润天下</span>
          <span class="logo-sub">HRWAI</span>
        </div>
      </a>

      <!-- Desktop Nav -->
      <ul class="desktop-nav">
        <li v-for="item in menuItems" :key="item.key">
          <a
            :href="item.path"
            class="nav-link"
            :class="{ active: activeAnchor === item.key }"
            @click.prevent="handleNavClick(item)"
          >{{ item.label }}</a>
        </li>
      </ul>

      <!-- Mobile Hamburger -->
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

    <!-- Mobile Menu -->
    <transition name="mobile-slide">
      <div v-if="mobileOpen" class="mobile-menu" :class="{ scrolled }">
        <a
          v-for="item in menuItems"
          :key="item.key"
          :href="item.path"
          class="mobile-link"
          @click.prevent="handleNavClick(item, true)"
        >{{ item.label }}</a>
      </div>
    </transition>
  </nav>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import type { NavItem } from '@/config/navigation'
import { buildSubdomainUrl } from '@/utils/subdomain'

defineProps<{ menuItems: NavItem[] }>()

const router = useRouter()
const scrolled = ref(false)
const mobileOpen = ref(false)
const activeAnchor = ref('home')
// 主域名为锚点首页（hash 链接），主域名访问时链接为 example.top/#
const homeUrl = buildSubdomainUrl('main', '/')

function onScroll() {
  scrolled.value = window.scrollY > 80
  // 更新当前活动锚点
  const sections = ['hero', 'about', 'products', 'cooperation', 'service', 'footer']
  for (const id of sections) {
    const el = document.getElementById(id)
    if (el) {
      const rect = el.getBoundingClientRect()
      if (rect.top <= 120 && rect.bottom >= 120) {
        activeAnchor.value = id === 'footer' ? 'contact' : id
        break
      }
    }
  }
}

function goHome() {
  if (router.currentRoute.value.path !== '/') {
    router.push('/')
  } else {
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }
  activeAnchor.value = 'home'
}

async function handleNavClick(item: NavItem, fromMobile = false) {
  mobileOpen.value = false
  if (!item.path) return

  // 解析路径与锚点
  const [path, hash] = item.path.split('#')
  const targetPath = path || '/'
  const currentPath = router.currentRoute.value.path

  if (targetPath !== currentPath) {
    await router.push(targetPath)
  }
  if (hash) {
    await nextTick()
    const el = document.getElementById(hash)
    if (el) {
      el.scrollIntoView({ behavior: 'smooth' })
      activeAnchor.value = hash === 'footer' ? 'contact' : hash
    }
  } else {
    window.scrollTo({ top: 0, behavior: 'smooth' })
    activeAnchor.value = 'home'
  }
}

onMounted(() => {
  window.addEventListener('scroll', onScroll, { passive: true })
  onScroll()
})
onUnmounted(() => {
  window.removeEventListener('scroll', onScroll)
})
</script>

<style scoped>
.portal-navbar {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  z-index: var(--z-sticky);
  background: transparent;
  transition: background var(--duration-normal) var(--ease-default),
              box-shadow var(--duration-normal) var(--ease-default);
}
.portal-navbar.scrolled {
  background: var(--surface-dark);
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.15);
}

.navbar-container {
  max-width: var(--container-page);
  margin: 0 auto;
  padding: 0 var(--space-6);
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 72px;
}

.logo-link {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  text-decoration: none;
  cursor: pointer;
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
  font-size: var(--text-xl);
  font-weight: var(--font-bold);
  color: #fff;
  letter-spacing: -0.025em;
}
.logo-sub {
  font-size: 10px;
  color: rgba(255, 255, 255, 0.7);
  letter-spacing: 0.15em;
  text-transform: uppercase;
  margin-top: 2px;
}

.desktop-nav {
  display: none;
  list-style: none;
  margin: 0;
  padding: 0;
  gap: 32px;
  align-items: center;
}
.nav-link {
  font-family: var(--font-body);
  font-size: var(--text-base);
  font-weight: var(--font-medium);
  color: rgba(255, 255, 255, 0.85);
  text-decoration: none;
  transition: color var(--duration-fast);
  cursor: pointer;
  position: relative;
}
.nav-link:hover,
.nav-link.active {
  color: #fff;
}
.nav-link.active::after {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  bottom: -6px;
  height: 2px;
  background: var(--gradient-brand);
  border-radius: 2px;
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
  width: 24px;
  height: 2px;
  background: #fff;
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
  display: none;
  flex-direction: column;
  background: var(--surface-dark);
  padding: var(--space-4) var(--space-6);
  border-top: 1px solid var(--color-border-darker);
}
.mobile-link {
  display: block;
  font-size: var(--text-lg);
  font-weight: var(--font-medium);
  color: var(--color-text-on-dark);
  text-decoration: none;
  padding: var(--space-4) 0;
  border-bottom: 1px solid var(--color-border-darker);
}
.mobile-link:last-of-type {
  border-bottom: none;
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

@media (min-width: 768px) {
  .desktop-nav {
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
  .mobile-menu {
    display: flex;
  }
  .navbar-container {
    height: 60px;
    padding: 0 var(--space-4);
  }
  .logo-text {
    font-size: var(--text-lg);
  }
}
</style>
