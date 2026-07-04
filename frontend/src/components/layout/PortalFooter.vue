<template>
  <footer id="footer" class="portal-footer">
    <div class="footer-container">
      <div class="footer-grid">
        <!-- Column 1: Brand -->
        <div class="footer-brand">
          <div class="brand-logo">
            <img src="/images/HRWAIlogo.jpg" alt="和润天下" class="brand-logo-img" />
            <span class="brand-name">和润天下</span>
          </div>
          <p class="brand-desc">
            和润天下人工智能科技有限公司，致力于用AI赋能叉车行业，推动残值评估与智能运维的数字化转型。
          </p>
        </div>

        <!-- Column 2: Contact -->
        <div class="footer-col">
          <h4 class="footer-title">联系我们</h4>
          <ul class="footer-list">
            <li class="contact-item">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M22 16.92v3a2 2 0 0 1-2.18 2 19.79 19.79 0 0 1-8.63-3.07 19.5 19.5 0 0 1-6-6 19.79 19.79 0 0 1-3.07-8.67A2 2 0 0 1 4.11 2h3a2 2 0 0 1 2 1.72 12.84 12.84 0 0 0 .7 2.81 2 2 0 0 1-.45 2.11L8.09 9.91a16 16 0 0 0 6 6l1.27-1.27a2 2 0 0 1 2.11-.45 12.84 12.84 0 0 0 2.81.7A2 2 0 0 1 22 16.92z"/>
              </svg>
              <span>400-XXX-XXXX</span>
            </li>
            <li class="contact-item">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"/>
                <polyline points="22,6 12,13 2,6"/>
              </svg>
              <span>contact@heruntianxia.com</span>
            </li>
            <li class="contact-item">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M21 10c0 7-9 13-9 13s-9-6-9-13a9 9 0 0 1 18 0z"/>
                <circle cx="12" cy="10" r="3"/>
              </svg>
              <span>中国 · 广州</span>
            </li>
          </ul>
        </div>

        <!-- Column 3: QR Code -->
        <div class="footer-col">
          <h4 class="footer-title">关注公众号</h4>
          <div class="qr-box">
            <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
              <rect x="3" y="3" width="7" height="7"/>
              <rect x="14" y="3" width="7" height="7"/>
              <rect x="3" y="14" width="7" height="7"/>
              <rect x="14" y="14" width="3" height="3"/>
              <rect x="18" y="14" width="3" height="3"/>
              <rect x="14" y="18" width="3" height="3"/>
              <rect x="18" y="18" width="3" height="3"/>
            </svg>
          </div>
        </div>

        <!-- Column 4: Quick Links -->
        <div class="footer-col">
          <h4 class="footer-title">快速链接</h4>
          <div class="quick-links">
            <a href="/" class="quick-link">首页</a>
            <a href="/#about" class="quick-link">公司介绍</a>
            <a href="/#products" class="quick-link">核心服务</a>
            <a href="/#cooperation" class="quick-link">合作模式</a>
            <a href="/#service" class="quick-link">服务保障</a>
            <a href="/#footer" class="quick-link">加入我们</a>
          </div>
        </div>
      </div>

      <div class="footer-bottom">
        <p>© 2026 和润天下人工智能科技有限公司 版权所有 | 粤ICP备XXXXXXXX号</p>
      </div>
    </div>
  </footer>
</template>

<script setup lang="ts">
</script>

<style scoped>
.portal-footer {
  background: var(--surface-dark);
  padding: var(--space-16) var(--space-6) 0;
  color: var(--color-text-on-dark);
}
.footer-container {
  max-width: var(--container-page);
  margin: 0 auto;
}
.footer-grid {
  display: grid;
  grid-template-columns: 2fr 1fr 1fr 1fr;
  gap: var(--space-12);
  padding-bottom: var(--space-12);
  border-bottom: 1px solid var(--color-border-darker);
}

.footer-brand {
  display: flex;
  flex-direction: column;
  gap: var(--space-6);
}
.brand-logo {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}
.brand-logo-img {
  width: 36px;
  height: 36px;
  border-radius: var(--radius-md);
  object-fit: cover;
}
.brand-name {
  font-family: var(--font-display);
  font-size: var(--text-lg);
  font-weight: var(--font-bold);
  color: var(--color-text-on-dark);
}
.brand-desc {
  font-size: var(--text-sm);
  line-height: var(--leading-relaxed);
  color: var(--color-text-muted);
  max-width: 320px;
  margin: 0;
}

.footer-title {
  font-family: var(--font-display);
  font-size: var(--text-base);
  font-weight: var(--font-semibold);
  color: var(--color-text-on-dark);
  margin: 0 0 var(--space-6);
}
.footer-list {
  list-style: none;
  padding: 0;
  margin: 0;
}
.contact-item {
  display: flex;
  gap: var(--space-3);
  margin-bottom: var(--space-4);
  align-items: flex-start;
  font-size: var(--text-sm);
  color: var(--color-text-muted);
  line-height: var(--leading-normal);
}
.contact-item:last-child {
  margin-bottom: 0;
}
.contact-item svg {
  flex-shrink: 0;
  margin-top: 2px;
  color: var(--color-text-muted);
}

.qr-box {
  width: 120px;
  height: 120px;
  border-radius: var(--radius-lg);
  background: var(--surface-dark-alt);
  border: 1px solid var(--color-border-darker);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--color-text-muted);
  opacity: 0.7;
}

.quick-links {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--space-3);
}
.quick-link {
  font-size: var(--text-sm);
  color: var(--color-text-muted);
  text-decoration: none;
  line-height: var(--leading-relaxed);
  transition: color var(--duration-fast);
}
.quick-link:hover {
  color: var(--color-text-on-dark);
}

.footer-bottom {
  padding: var(--space-6) 0;
  text-align: center;
}
.footer-bottom p {
  font-size: var(--text-xs);
  color: var(--color-text-muted);
  margin: 0;
}

@media (max-width: 768px) {
  .footer-grid {
    grid-template-columns: 1fr 1fr;
    gap: var(--space-6);
  }
  .footer-brand {
    grid-column: 1 / -1;
  }
}

@media (max-width: 480px) {
  .footer-grid {
    grid-template-columns: 1fr;
  }
  .quick-links {
    grid-template-columns: 1fr;
  }
}
</style>
