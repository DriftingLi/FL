<template>
  <footer class="valuation-footer">
    <!-- 功能聚合区：浅色 → 深色渐变过渡，4 列：品牌简介 / 快速入口 / 累计数据 / 关注公众号 -->
    <div class="footer-container">
      <div class="footer-grid">
        <!-- Column 1: Brand -->
        <div class="footer-brand">
          <div class="brand-logo">
            <img src="/images/HRWAIlogo.jpg" alt="和润天下" class="brand-logo-img" />
            <span class="brand-name">和润天下 · 残值评估</span>
          </div>
          <p class="brand-desc">
            和润天下人工智能科技有限公司旗下叉车残值评估与电池健康度检测平台，
            用AI让每一台叉车的价值透明可见。
          </p>
          <div class="brand-meta">
            <a href="/" class="brand-link">访问官网 →</a>
          </div>
        </div>

        <!-- Column 2: 快速入口（原导航链接下移） -->
        <div class="footer-col">
          <h4 class="footer-title">快速入口</h4>
          <ul class="footer-list">
            <li><router-link to="/valuation/battery">电池健康度评估</router-link></li>
            <li v-if="isLoggedIn"><router-link to="/valuation/history">评估历史记录</router-link></li>
            <li v-else>
              <router-link :to="{ path: '/login', query: { redirect: '/valuation/history' } }">
                登录查看历史
              </router-link>
            </li>
          </ul>
        </div>

        <!-- Column 3: 累计数据（来自接口） -->
        <div class="footer-col">
          <h4 class="footer-title">累计评估数据</h4>
          <div class="footer-stats" v-loading="loadingStats">
            <div class="stat-item">
              <span class="stat-num">{{ statsTotal }}</span>
              <span class="stat-label">累计评估次数（次）</span>
            </div>
            <p class="footer-hint">
              已有用户提交残值评估，<br />
              数据每 10 分钟更新一次。
            </p>
          </div>
        </div>

        <!-- Column 4: 关注公众号 -->
        <div class="footer-col">
          <h4 class="footer-title">关注公众号</h4>
          <div class="qr-box" aria-label="公众号二维码占位">
            <svg width="56" height="56" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
              <rect x="3" y="3" width="7" height="7"/>
              <rect x="14" y="3" width="7" height="7"/>
              <rect x="3" y="14" width="7" height="7"/>
              <rect x="14" y="14" width="3" height="3"/>
              <rect x="18" y="14" width="3" height="3"/>
              <rect x="14" y="18" width="3" height="3"/>
              <rect x="18" y="18" width="3" height="3"/>
            </svg>
          </div>
          <p class="qr-hint">扫码关注获取最新功能与行业资讯</p>
          <p class="footer-hint">
            客服电话：400-XXX-XXXX<br />
            邮箱：contact@heruntianxia.com
          </p>
        </div>
      </div>

      <div class="footer-bottom">
        <p>© {{ year }} 和润天下人工智能科技有限公司 版权所有 | 粤ICP备XXXXXXXX号 </p>
      </div>
    </div>
  </footer>
</template>

<script setup lang="ts">
// 与 PortalFooter 视觉风格保持一致（深色多列 + 渐变点缀）
import { onMounted, ref, computed } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { getEvaluationStats } from '@/api/valuation/evaluation'

const authStore = useAuthStore()
const isLoggedIn = computed(() => {
  const u = authStore.userInfo as { role?: string } | null
  return !!(authStore.token && authStore.isLoggedIn && u?.role)
})
const year = new Date().getFullYear()

// 累计评估次数（与 ValuationInputView 同一接口，失败回退 0）
const statsTotal = ref(0)
const loadingStats = ref(false)
onMounted(async () => {
  loadingStats.value = true
  try {
    const s = await getEvaluationStats()
    statsTotal.value = s.total ?? 0
  } catch {
    statsTotal.value = 0
  } finally {
    loadingStats.value = false
  }
})
</script>

<style scoped>
.valuation-footer {
  background: var(--gradient-dark, linear-gradient(180deg, #0F172A 0%, #1E293B 100%));
  padding: var(--space-16, 64px) var(--space-6, 24px) 0;
  color: var(--color-text-on-dark, #F1F5F9);
  margin-top: var(--space-24, 96px);
}

.footer-container {
  max-width: var(--container-max, 1280px);
  margin: 0 auto;
}

.footer-grid {
  display: grid;
  grid-template-columns: 2fr 1fr 1fr 1fr;
  gap: var(--space-12, 48px);
  padding-bottom: var(--space-12, 48px);
  border-bottom: 1px solid var(--color-border-darker, #334155);
}

/* ===== Brand 列 ===== */
.footer-brand {
  display: flex;
  flex-direction: column;
  gap: var(--space-5, 20px);
}
.brand-logo {
  display: flex;
  align-items: center;
  gap: var(--space-3, 12px);
}
.brand-logo-img {
  width: 40px;
  height: 40px;
  border-radius: var(--radius-md, 8px);
  object-fit: cover;
}
.brand-name {
  font-family: var(--font-display, 'DM Sans', sans-serif);
  font-size: var(--text-lg, 18px);
  font-weight: var(--fw-bold, 700);
  color: var(--color-text-on-dark, #F1F5F9);
}
.brand-desc {
  font-size: var(--text-sm, 14px);
  line-height: 1.75;
  color: var(--color-text-muted, #94A3B8);
  max-width: 360px;
  margin: 0;
}
.brand-meta {
  margin-top: var(--space-2, 8px);
}
.brand-link {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: var(--text-sm, 14px);
  font-weight: var(--fw-medium, 500);
  color: var(--color-brand-200, #BAE6FD);
  text-decoration: none;
  padding: 8px 16px;
  border: 1px solid var(--color-border-darker, #334155);
  border-radius: var(--radius-full, 9999px);
  transition: all var(--t-fast, 150ms) var(--ease, ease);
}
.brand-link:hover {
  color: var(--color-text-on-dark, #F1F5F9);
  border-color: var(--color-brand-500, #0EA5E9);
  background: rgba(14, 165, 233, 0.1);
}

/* ===== 通用 footer 列 ===== */
.footer-col {
  display: flex;
  flex-direction: column;
}
.footer-title {
  font-family: var(--font-display, 'DM Sans', sans-serif);
  font-size: var(--text-base, 16px);
  font-weight: var(--fw-semibold, 600);
  color: var(--color-text-on-dark, #F1F5F9);
  margin: 0 0 var(--space-5, 20px);
  position: relative;
  padding-bottom: var(--space-2, 8px);
}
.footer-title::after {
  content: '';
  position: absolute;
  left: 0;
  bottom: 0;
  width: 24px;
  height: 2px;
  background: var(--gradient-brand, linear-gradient(135deg, #0EA5E9, #14B8A6));
  border-radius: 2px;
}
.footer-list {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-3, 12px);
}
.footer-list li a {
  font-size: var(--text-sm, 14px);
  color: var(--color-text-muted, #94A3B8);
  text-decoration: none;
  transition: color var(--t-fast, 150ms) var(--ease, ease);
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.footer-list li a::before {
  content: '›';
  color: var(--color-brand-500, #0EA5E9);
  font-weight: var(--fw-bold, 700);
  transition: transform var(--t-fast, 150ms) var(--ease, ease);
}
.footer-list li a:hover {
  color: var(--color-text-on-dark, #F1F5F9);
}
.footer-list li a:hover::before {
  transform: translateX(2px);
}

/* ===== 累计数据列 ===== */
.footer-stats {
  display: flex;
  flex-direction: column;
  gap: var(--space-3, 12px);
}
.stat-item {
  display: flex;
  align-items: baseline;
  gap: var(--space-2, 8px);
  flex-wrap: wrap;
}
.stat-num {
  font-family: var(--font-mono, 'JetBrains Mono', monospace);
  font-size: 32px;
  font-weight: var(--fw-semibold, 600);
  background: var(--gradient-brand, linear-gradient(135deg, #0EA5E9, #14B8A6));
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  color: transparent;
  letter-spacing: -0.02em;
  line-height: 1;
  font-feature-settings: 'tnum' 1;
}
.stat-label {
  font-size: var(--text-xs, 12px);
  color: var(--color-text-muted, #94A3B8);
  letter-spacing: 0.05em;
}
.footer-hint {
  font-size: var(--text-xs, 12px);
  line-height: 1.7;
  color: var(--color-text-muted, #94A3B8);
  margin: 0;
}

/* ===== 二维码列 ===== */
.qr-box {
  width: 120px;
  height: 120px;
  border-radius: var(--radius-lg, 12px);
  background: var(--color-bg-dark-alt, #1E293B);
  border: 1px solid var(--color-border-darker, #334155);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--color-text-muted, #94A3B8);
  opacity: 0.85;
  margin-bottom: var(--space-3, 12px);
}
.qr-hint {
  font-size: var(--text-xs, 12px);
  color: var(--color-text-on-dark, #F1F5F9);
  margin: 0 0 var(--space-4, 16px);
}

/* ===== 底部版权 ===== */
.footer-bottom {
  padding: var(--space-6, 24px) 0;
  text-align: center;
}
.footer-bottom p {
  font-size: var(--text-xs, 12px);
  color: var(--color-text-muted, #94A3B8);
  margin: 0;
  letter-spacing: 0.02em;
}

/* ===== 响应式 ===== */
@media (max-width: 1023px) {
  .footer-grid {
    grid-template-columns: 1fr 1fr;
    gap: var(--space-8, 32px);
  }
  .footer-brand {
    grid-column: 1 / -1;
  }
}
@media (max-width: 640px) {
  .valuation-footer {
    padding: var(--space-10, 40px) var(--space-4, 16px) 0;
    margin-top: var(--space-16, 64px);
  }
  .footer-grid {
    grid-template-columns: 1fr;
    gap: var(--space-8, 32px);
    padding-bottom: var(--space-8, 32px);
  }
  .footer-brand {
    grid-column: 1;
  }
  .stat-num {
    font-size: 28px;
  }
}
</style>
