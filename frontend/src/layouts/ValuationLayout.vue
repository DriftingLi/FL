<template>
  <div class="valuation-layout">
    <!-- 简洁顶部栏：仅 logo + 返回官网（无导航） -->
    <header class="topbar">
      <div class="topbar-container">
        <router-link to="/" class="logo-link">
          <img src="/images/HRWAIlogo.jpg" alt="和润天下" class="logo-img" />
          <div class="logo-text-wrap">
            <span class="logo-text">和润天下</span>
            <span class="logo-sub">残值评估 · HRWAI</span>
          </div>
        </router-link>
        <a :href="mainSiteUrl" class="btn-back">返回官网</a>
      </div>
    </header>

    <main class="valuation-main">
      <div class="valuation-content">
        <router-view />
      </div>
    </main>

    <!-- 深色页脚：吸收原导航功能 + 累计评估 + 公众号 + 版权 -->
    <ValuationFooter />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import ValuationFooter from '@/components/valuation/ValuationFooter.vue'
import { buildSubdomainUrl } from '@/utils/subdomain'

// 跨子域名跳回主域名（router-link to="/" 在当前子域名下会被路由守卫重定向）
const mainSiteUrl = computed(() => buildSubdomainUrl('main', '/'))
</script>

<style scoped>
.valuation-layout {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--color-bg-page, #F8FAFC);
}

/* ===== 简洁顶部栏 ===== */
.topbar {
  background: var(--color-surface, #FFFFFF);
  border-bottom: 1px solid var(--color-border, #E2E8F0);
}

.topbar-container {
  max-width: var(--container-max, 1280px);
  margin: 0 auto;
  padding: 0 var(--space-6, 24px);
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: var(--header-h, 72px);
  gap: var(--space-6, 24px);
}

.logo-link {
  display: flex;
  align-items: center;
  gap: var(--space-3, 12px);
  text-decoration: none;
  flex-shrink: 0;
}

.logo-img {
  width: 40px;
  height: 40px;
  border-radius: var(--radius-md, 8px);
  object-fit: cover;
}

.logo-text-wrap {
  display: flex;
  flex-direction: column;
  line-height: 1.1;
}

.logo-text {
  font-family: var(--font-display, 'DM Sans', sans-serif);
  font-size: var(--text-lg, 18px);
  font-weight: var(--fw-bold, 700);
  color: var(--color-text-primary, #0F172A);
  letter-spacing: -0.025em;
}

.logo-sub {
  font-size: 11px;
  color: var(--color-text-tertiary, #64748B);
  letter-spacing: 0.15em;
  text-transform: uppercase;
  margin-top: 2px;
}

.btn-back {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 40px;
  padding: 8px 20px;
  background: transparent;
  color: var(--color-text-secondary, #475569);
  border: 1px solid var(--color-border-dark, #CBD5E1);
  border-radius: var(--radius-md, 8px);
  font-size: var(--text-sm, 14px);
  font-weight: var(--fw-medium, 500);
  text-decoration: none;
  transition: all var(--t-fast, 150ms) var(--ease, ease);
}

.btn-back:hover {
  border-color: var(--color-brand-500, #0EA5E9);
  color: var(--color-brand-600, #0284C7);
  background: var(--color-brand-50, #F0F9FF);
}

/* ===== 主内容区 ===== */
.valuation-main {
  flex: 1;
  padding: 0 var(--space-4, 16px);
}

.valuation-content {
  max-width: var(--container-max, 1280px);
  margin: 0 auto;
  width: 100%;
}

@media (max-width: 767px) {
  .topbar-container {
    padding: 0 var(--space-4, 16px);
    height: 60px;
  }
  .logo-text {
    font-size: var(--text-base, 16px);
  }
  .valuation-main {
    padding: 0 var(--space-3, 12px);
  }
}
</style>
