<template>
  <footer class="app-footer">
    <div class="footer-container">
      <div class="footer-grid">
        <div class="footer-brand">
          <div class="footer-logo">
            <img src="/images/HRWAIlogo.jpg" alt="和润天下" class="footer-logo-img" />
            <span class="footer-logo-text">和润天下</span>
          </div>
          <p class="footer-desc">一站式叉车维修培训、考核与残值评估服务平台。</p>
          <div class="footer-contact">
            <p class="contact-line">
              <span class="contact-label">服务热线（9:00-18:00）</span>
              <a href="tel:18024010891" class="contact-phone">18024010891</a>
              <span class="contact-sep">/</span>
              <a href="tel:13352891859" class="contact-phone">13352891859</a>
            </p>
            <p class="contact-line">
              <a href="mailto:HRWAI001@aidataexp.com" class="contact-phone">HRWAI001@aidataexp.com</a>
            </p>
          </div>
        </div>

        <div class="footer-column">
          <h4 class="footer-title">产品</h4>
          <ul class="footer-links">
            <li><router-link to="/training">课程中心</router-link></li>
            <li><router-link to="/training/question-bank">题库练习</router-link></li>
            <li><router-link to="/valuation">残值评估</router-link></li>
            <li><a href="https://mall.gccsmile.com/" target="_blank" rel="noopener noreferrer">二手叉车交易 ↗</a></li>
          </ul>
        </div>

        <div class="footer-column">
          <h4 class="footer-title">支持</h4>
          <ul class="footer-links">
            <li><router-link to="/ai-assistant">AI 助手</router-link></li>
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
            <li><a href="tel:18024010891">联系我们</a></li>
          </ul>
        </div>
      </div>

      <div class="footer-bottom">
        <p>© {{ currentYear }} 和润天下人工智能科技有限公司 版权所有 | <a href="https://beian.miit.gov.cn/#/Integrated/index" target="_blank" rel="noopener noreferrer" class="beian-link">粤ICP备2026005936号-1</a></p>
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

.footer-logo-img {
  width: 32px;
  height: 32px;
  border-radius: var(--radius-md);
  object-fit: cover;
}

.footer-desc {
  font-size: var(--text-sm);
  color: var(--color-text-secondary);
  line-height: var(--leading-relaxed);
  max-width: 280px;
}

.footer-contact {
  margin-top: var(--space-2);
}
.contact-line {
  margin: 0;
  font-size: var(--text-sm);
  color: var(--color-text-secondary);
  line-height: var(--leading-relaxed);
}
.contact-label {
  display: block;
  color: var(--color-text-tertiary);
  font-size: var(--text-xs);
  margin-bottom: 2px;
}
.contact-phone {
  color: var(--color-text-primary);
  text-decoration: none;
  font-weight: var(--font-medium);
  transition: color var(--duration-fast);
}
.contact-phone:hover {
  color: var(--color-primary-600);
}
.contact-sep {
  color: var(--color-text-tertiary);
  margin: 0 2px;
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

.beian-link {
  color: inherit;
  text-decoration: none;
  transition: color var(--duration-fast);
}

.beian-link:hover {
  color: var(--color-primary-500);
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
