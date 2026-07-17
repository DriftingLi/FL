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
              <span>
                服务热线（9:00-18:00）<br />
                <a href="tel:18024010891" class="contact-phone">18024010891</a> /
                <a href="tel:13352891859" class="contact-phone">13352891859</a>
              </span>
            </li>
            <li class="contact-item">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"/>
                <polyline points="22,6 12,13 2,6"/>
              </svg>
              <a href="mailto:HRWAI001@aidataexp.com" class="contact-link">HRWAI001@aidataexp.com</a>
            </li>
            <li class="contact-item">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="9" cy="21" r="1"/>
                <circle cx="20" cy="21" r="1"/>
                <path d="M1 1h4l2.68 13.39a2 2 0 0 0 2 1.61h9.72a2 2 0 0 0 2-1.61L23 6H6"/>
              </svg>
              <a href="https://mall.gccsmile.com/" target="_blank" rel="noopener noreferrer" class="contact-link">二手叉车交易平台 →</a>
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
          <h4 class="footer-title">扫码关注</h4>
          <div class="qr-group">
            <div class="qr-item">
              <div class="qr-box">
                <img src="/images/公众号.png" alt="公众号二维码" class="qr-img" />
              </div>
              <p class="qr-label">公众号</p>
            </div>
            <div class="qr-item">
              <div class="qr-box">
                <img src="/images/小程序.png" alt="小程序二维码" class="qr-img" />
              </div>
              <p class="qr-label">小程序</p>
            </div>
          </div>
        </div>

        <!-- Column 4: Quick Links -->
        <div class="footer-col">
          <h4 class="footer-title">快速链接</h4>
          <div class="quick-links">
            <a :href="homeUrl" class="quick-link">首页</a>
            <a :href="aboutUrl" class="quick-link">公司介绍</a>
            <a :href="productsUrl" class="quick-link">核心服务</a>
            <a :href="cooperationUrl" class="quick-link">合作模式</a>
            <a :href="serviceUrl" class="quick-link">服务保障</a>
            <a :href="joinUrl" class="quick-link">加入我们</a>
          </div>
        </div>
      </div>

      <div class="footer-bottom">
        <p>© 2026 和润天下人工智能科技有限公司 版权所有 | <a href="https://beian.miit.gov.cn/#/Integrated/index" target="_blank" rel="noopener noreferrer" class="beian-link">粤ICP备2026005936号-1</a></p>
      </div>
    </div>
  </footer>
</template>

<script setup lang="ts">
import { buildSubdomainUrl } from '@/utils/subdomain'

// 主域名为锚点首页（hash 链接）
const homeUrl = buildSubdomainUrl('main', '/')
const aboutUrl = buildSubdomainUrl('main', '/#about')
const productsUrl = buildSubdomainUrl('main', '/#products')
const cooperationUrl = buildSubdomainUrl('main', '/#cooperation')
const serviceUrl = buildSubdomainUrl('main', '/#service')
const joinUrl = buildSubdomainUrl('main', '/#footer')
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
.contact-phone,
.contact-link {
  color: var(--color-text-muted);
  text-decoration: none;
  transition: color var(--duration-fast);
}
.contact-phone:hover,
.contact-link:hover {
  color: var(--color-text-on-dark);
}

.qr-group {
  display: flex;
  gap: var(--space-4);
}
.qr-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-2);
}
.qr-box {
  width: 110px;
  height: 110px;
  border-radius: var(--radius-lg);
  background: #fff;
  border: 1px solid var(--color-border-darker);
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}
.qr-img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}
.qr-label {
  font-size: var(--text-xs);
  color: var(--color-text-muted);
  margin: 0;
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

.beian-link {
  color: inherit;
  text-decoration: none;
  transition: color var(--duration-fast);
}

.beian-link:hover {
  color: var(--color-primary-400);
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
