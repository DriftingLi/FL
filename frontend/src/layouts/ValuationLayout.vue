<template>
  <div class="valuation-layout">
    <!-- 简洁顶部栏：logo + 估值用户登录状态 + 返回官网 -->
    <header class="topbar">
      <div class="topbar-container">
        <router-link to="/" class="logo-link">
          <img src="/images/HRWAIlogo.jpg" alt="和润天下" class="logo-img" />
          <div class="logo-text-wrap">
            <span class="logo-text">和润天下</span>
            <span class="logo-sub">残值评估 · HRWAI</span>
          </div>
        </router-link>

        <div class="user-zone">
          <!-- 未登录：显示登录/注册入口 -->
          <template v-if="!valuationAuth.isLoggedIn">
            <router-link to="/valuation/register" class="btn-entry btn-register">注册</router-link>
            <router-link to="/valuation/login" class="btn-entry btn-login">登录</router-link>
          </template>

          <!-- 已登录：显示用户名 + 退出 -->
          <template v-else>
            <span class="user-name" :title="displayName">{{ displayName }}</span>
            <button type="button" class="btn-entry btn-logout" @click="handleLogout">退出</button>
          </template>

          <a :href="mainSiteUrl" class="btn-back">返回官网</a>
        </div>
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
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import ValuationFooter from '@/components/valuation/ValuationFooter.vue'
import { buildSubdomainUrl } from '@/utils/subdomain'
import { useValuationAuthStore } from '@/stores/valuationAuth'
import { valuationAuthApi } from '@/api/valuation/auth'

const router = useRouter()
const valuationAuth = useValuationAuthStore()

// 跨子域名跳回主域名（router-link to="/" 在当前子域名下会被路由守卫重定向）
const mainSiteUrl = computed(() => buildSubdomainUrl('main', '/'))

// 估值用户显示名：优先 name，回退到 username
const displayName = computed(() => {
  const info = valuationAuth.userInfo
  return info?.name || info?.username || '评估用户'
})

// 退出登录：调用后端写黑名单 → 清除本地登录态 → 跳回估值首页
async function handleLogout() {
  try {
    await valuationAuthApi.logout()
  } catch (e) {
    // 即使后端调用失败也清除本地登录态，避免用户卡在已登录状态
  } finally {
    valuationAuth.clearAuthData()
    ElMessage.success('已退出登录')
    router.push('/valuation')
  }
}
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

/* ===== 顶部栏右侧用户区 ===== */
.user-zone {
  display: flex;
  align-items: center;
  gap: var(--space-3, 12px);
  flex-shrink: 0;
}

.btn-entry {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 40px;
  padding: 8px 18px;
  border-radius: var(--radius-md, 8px);
  font-size: var(--text-sm, 14px);
  font-weight: var(--fw-medium, 500);
  text-decoration: none;
  cursor: pointer;
  border: 1px solid transparent;
  transition: all var(--t-fast, 150ms) var(--ease, ease);
}

/* 注册按钮：次按钮样式（透明背景 + 边框） */
.btn-register {
  background: transparent;
  color: var(--color-text-secondary, #475569);
  border-color: var(--color-border-dark, #CBD5E1);
}

.btn-register:hover {
  border-color: var(--color-brand-500, #0EA5E9);
  color: var(--color-brand-600, #0284C7);
  background: var(--color-brand-50, #F0F9FF);
}

/* 登录按钮：主按钮样式（品牌色渐变） */
.btn-login {
  background: linear-gradient(135deg, #0EA5E9 0%, #0284C7 100%);
  color: #FFFFFF;
  border-color: transparent;
  box-shadow: 0 2px 6px rgba(14, 165, 233, 0.2);
}

.btn-login:hover {
  transform: translateY(-1px);
  box-shadow: 0 6px 14px rgba(14, 165, 233, 0.3);
  opacity: 0.95;
}

/* 已登录用户名 */
.user-name {
  font-size: var(--text-sm, 14px);
  font-weight: var(--fw-medium, 500);
  color: var(--color-text-primary, #0F172A);
  max-width: 140px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 退出按钮：次按钮样式 */
.btn-logout {
  background: transparent;
  color: var(--color-text-secondary, #475569);
  border-color: var(--color-border-dark, #CBD5E1);
}

.btn-logout:hover {
  border-color: #F87171;
  color: #DC2626;
  background: #FEF2F2;
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
    gap: var(--space-3, 12px);
  }
  .logo-text {
    font-size: var(--text-base, 16px);
  }
  .valuation-main {
    padding: 0 var(--space-3, 12px);
  }
  .user-zone {
    gap: var(--space-2, 8px);
  }
  /* 移动端隐藏用户名，仅显示退出按钮 */
  .user-name {
    display: none;
  }
  .btn-entry,
  .btn-back {
    min-height: 36px;
    padding: 6px 12px;
    font-size: 13px;
  }
}
</style>
