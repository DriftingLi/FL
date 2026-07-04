<template>
  <footer class="app-footer">
    <div class="footer-container">
      <div class="footer-grid">
        <div class="footer-brand">
          <div class="footer-logo">
            <svg width="28" height="28" viewBox="0 0 28 28" fill="none">
              <rect width="28" height="28" rx="8" fill="url(#footer-logo-gradient)"/>
              <path d="M8 18L14 8L20 18H8Z" stroke="white" stroke-width="1.5" stroke-linejoin="round" fill="none"/>
              <circle cx="14" cy="15" r="2" fill="white"/>
              <defs>
                <linearGradient id="footer-logo-gradient" x1="0" y1="0" x2="28" y2="28">
                  <stop stop-color="#1E40AF"/>
                  <stop offset="1" stop-color="#2563EB"/>
                </linearGradient>
              </defs>
            </svg>
            <span class="footer-logo-text">ForkLift<span class="footer-logo-accent">Pro</span></span>
          </div>
          <p class="footer-desc">一站式叉车维修培训、考核与残值评估服务平台。</p>
        </div>

        <div class="footer-column">
          <h4 class="footer-title">产品</h4>
          <ul class="footer-links">
            <li><router-link to="/courses">课程中心</router-link></li>
            <li><router-link to="/question-bank">题库练习</router-link></li>
            <li><router-link to="/practice">虚拟实操</router-link></li>
            <li><router-link to="/valuation">残值评估</router-link></li>
          </ul>
        </div>

        <div class="footer-column">
          <h4 class="footer-title">支持</h4>
          <ul class="footer-links">
            <li><router-link to="/ai-generate">AI 助手</router-link></li>
            <li><router-link to="/profile">个人中心</router-link></li>
            <li><a href="#">帮助文档</a></li>
            <li><a href="#">意见反馈</a></li>
          </ul>
        </div>

        <div class="footer-column">
          <h4 class="footer-title">关于</h4>
          <ul class="footer-links">
            <li><a href="#">关于我们</a></li>
            <li><a href="#">服务条款</a></li>
            <li><a href="#">隐私政策</a></li>
            <li><a href="#">联系我们</a></li>
          </ul>
        </div>
      </div>

      <div class="footer-bottom">
        <p>© {{ currentYear }} ForkLift Pro. 保留所有权利。</p>
      </div>
    </div>
  </footer>
</template>

<script setup lang="ts">
const currentYear = new Date().getFullYear()
</script>

<style scoped>
.app-footer {
  background: var(--footer-bg, #F1F5F9);
  border-top: 1px solid var(--color-border, #E2E8F0);
  margin-top: auto;
}

.footer-container {
  max-width: var(--container-page);
  margin: 0 auto;
  padding: var(--space-12) var(--space-5) var(--space-6);
}

.footer-grid {
  display: grid;
  grid-template-columns: 2fr repeat(3, 1fr);
  gap: var(--space-8);
  padding-bottom: var(--space-8);
  border-bottom: 1px solid var(--color-border-light);
}

.footer-brand {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.footer-logo {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.footer-logo-text {
  font-family: var(--font-display);
  font-size: var(--text-xl);
  font-weight: var(--font-bold);
  color: var(--color-text-primary);
}

.footer-logo-accent {
  color: var(--color-primary-500);
}

.footer-desc {
  font-size: var(--text-sm);
  color: var(--color-text-secondary);
  line-height: var(--leading-relaxed);
  max-width: 280px;
}

.footer-title {
  font-size: var(--text-sm);
  font-weight: var(--font-semibold);
  color: var(--color-text-primary);
  margin-bottom: var(--space-4);
}

.footer-links {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.footer-links a {
  font-size: var(--text-sm);
  color: var(--color-text-secondary);
  text-decoration: none;
  transition: color var(--duration-fast);
}

.footer-links a:hover {
  color: var(--color-primary-600);
}

.footer-bottom {
  padding-top: var(--space-6);
  text-align: center;
  font-size: var(--text-sm);
  color: var(--color-text-tertiary);
}

@media screen and (max-width: 768px) {
  .footer-grid {
    grid-template-columns: 1fr 1fr;
    gap: var(--space-6);
  }

  .footer-brand {
    grid-column: 1 / -1;
  }
}

@media screen and (max-width: 480px) {
  .footer-grid {
    grid-template-columns: 1fr;
  }
}
</style>
