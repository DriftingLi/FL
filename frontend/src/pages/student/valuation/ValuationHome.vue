// 首页：100vh 极简 hero + 卡片入口（Tesla 风格）
// 重构说明：入口从「电动/内燃」改为「叉车残值评估」与「电池 RUL 评估」
<script setup lang="ts">
import { useRouter } from 'vue-router'

const router = useRouter()

function goValuation() {
  router.push('/valuation/input')
}
function goBattery() {
  router.push('/valuation/battery')
}
function goHistory() {
  router.push('/valuation/history')
}

interface EntryCard {
  key: 'valuation' | 'battery' | 'history'
  title: string
  subtitle: string
  meta: string
  icon: string
  onClick: () => void
}

const cards: readonly EntryCard[] = [
  {
    key: 'valuation',
    title: '叉车残值评估',
    subtitle: 'Forklift Residual Value',
    meta: '基于品牌/车型/车况/区域的统一残值模型',
    icon: '🚜',
    onClick: goValuation
  },
  {
    key: 'battery',
    title: '电池健康度评估',
    subtitle: 'Battery RUL',
    meta: '基于循环数据的剩余寿命预测',
    icon: '🔋',
    onClick: goBattery
  },
  {
    key: 'history',
    title: '评估历史记录',
    subtitle: 'Evaluation History',
    meta: '查看历次残值评估记录',
    icon: '📋',
    onClick: goHistory
  }
] as const
</script>

<template>
  <div class="home valuation-root">
    <!-- Hero 区域：100vh，居中标题 + 副标题 + 极简分隔 -->
    <section class="hero">
      <div class="hero-inner">
        <p class="hero-eyebrow">forklift residual value</p>
        <h1 class="hero-title">叉车残值与电池健康度<br />一站式评估</h1>
        <p class="hero-sub">基于品牌 / 车型 / 车况 / 区域 / 使用强度的统一残值模型</p>
      </div>
    </section>

    <!-- 残值评估主入口：叉车残值评估（单卡居中） -->
    <section class="entry-section">
      <div class="entry-grid entry-grid-primary">
        <article
          v-for="card in cards.slice(0, 1)"
          :key="card.key"
          class="entry-card"
          @click="card.onClick"
        >
          <div class="entry-card-icon">
            <span>{{ card.icon }}</span>
          </div>
          <h3 class="entry-card-title">{{ card.title }}</h3>
          <p class="entry-card-subtitle">{{ card.subtitle }}</p>
          <p class="entry-card-meta">{{ card.meta }}</p>
          <a class="entry-card-cta">
            开始评估 <span class="entry-card-arrow">→</span>
          </a>
        </article>
      </div>
    </section>

    <!-- 更多功能：电池评估 + 历史记录 -->
    <section class="entry-section entry-section-secondary">
      <h2 class="section-label">更多功能</h2>
      <div class="entry-grid entry-grid-secondary">
        <article
          v-for="card in cards.slice(1)"
          :key="card.key"
          class="entry-card entry-card-secondary"
          @click="card.onClick"
        >
          <div class="entry-card-icon">
            <span>{{ card.icon }}</span>
          </div>
          <h3 class="entry-card-title">{{ card.title }}</h3>
          <p class="entry-card-subtitle">{{ card.subtitle }}</p>
          <p class="entry-card-meta">{{ card.meta }}</p>
          <a class="entry-card-cta">
            进入 <span class="entry-card-arrow">→</span>
          </a>
        </article>
      </div>
    </section>

  </div>
</template>

<style scoped>
.home {
  background: var(--color-bg);
  min-height: calc(100vh - var(--header-h) - 40px);
  display: flex;
  flex-direction: column;
}

/* ===== Hero ===== */
.hero {
  /* 不强制 100vh，按内容自然撑开，但留出大量顶部留白 */
  padding: var(--sp-20) var(--sp-6) var(--sp-16);
  text-align: center;
}
.hero-inner {
  max-width: 720px;
  margin: 0 auto;
}
.hero-eyebrow {
  font-size: var(--fs-sm);
  font-weight: var(--fw-medium);
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--color-text-tertiary);
  margin: 0 0 var(--sp-6);
}
.hero-title {
  font-family: var(--font-text);
  font-size: var(--fs-3xl);    /* 40px */
  font-weight: var(--fw-medium);
  line-height: 1.2;
  color: var(--color-text);
  margin: 0 0 var(--sp-5);
  letter-spacing: normal;
}
.hero-sub {
  font-size: var(--fs-base);
  font-weight: var(--fw-regular);
  color: var(--color-text-tertiary);
  margin: 0;
  line-height: 1.6;
}

/* ===== Category 入口 ===== */
.entry-section {
  padding: 0 var(--sp-6) var(--sp-16);
}
.entry-grid {
  max-width: 960px;
  margin: 0 auto;
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--sp-4);
}
/* 主入口（叉车残值评估）：单卡居中、更突出 */
.entry-grid-primary {
  max-width: 720px;
  grid-template-columns: 1fr;
}
.entry-grid-primary .entry-card {
  min-height: 360px;
  padding: var(--sp-12) var(--sp-10);
}
.entry-grid-primary .entry-card-icon {
  font-size: 48px;
  margin-bottom: var(--sp-5);
}

/* ===== 更多功能区域 ===== */
.entry-section-secondary {
  padding-top: 0;
  padding-bottom: var(--sp-12);
}
.section-label {
  max-width: 960px;
  margin: 0 auto var(--sp-5);
  font-size: var(--fs-sm);
  font-weight: var(--fw-medium);
  color: var(--color-text-tertiary);
  letter-spacing: 0.12em;
  text-transform: uppercase;
}
.entry-card-secondary {
  min-height: 240px;
  padding: var(--sp-8) var(--sp-6);
}
.entry-card-secondary .entry-card-icon {
  font-size: 32px;
  margin-bottom: var(--sp-4);
}
.entry-card {
  background: var(--color-bg);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  padding: var(--sp-10) var(--sp-8);
  text-align: center;
  cursor: pointer;
  transition: border-color var(--t-base) var(--ease);
  display: flex;
  flex-direction: column;
  align-items: center;
  min-height: 320px;
  justify-content: center;
}
.entry-card:hover {
  border-color: var(--color-primary);
}
.entry-card-icon {
  font-size: 40px;
  color: var(--color-text);
  margin-bottom: var(--sp-5);
  line-height: 1;
  transition: color var(--t-base) var(--ease);
}
.entry-card:hover .entry-card-icon {
  color: var(--color-primary);
}
.entry-card-title {
  font-size: var(--fs-xl);
  font-weight: var(--fw-medium);
  color: var(--color-text);
  margin: 0 0 var(--sp-2);
}
.entry-card-subtitle {
  font-size: var(--fs-sm);
  color: var(--color-text-tertiary);
  margin: 0 0 var(--sp-3);
  letter-spacing: 0.04em;
  text-transform: uppercase;
  font-weight: var(--fw-medium);
}
.entry-card-meta {
  font-size: var(--fs-sm);
  color: var(--color-text-muted);
  margin: 0 0 var(--sp-6);
}
.entry-card-cta {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: var(--fs-base);
  font-weight: var(--fw-medium);
  color: var(--color-primary);
  cursor: pointer;
  transition: color var(--t-base) var(--ease);
}
.entry-card-cta:hover {
  color: var(--color-primary-hover);
}
.entry-card-arrow {
  font-size: 12px;
  transition: transform var(--t-base) var(--ease);
}
.entry-card:hover .entry-card-arrow {
  transform: translateX(4px);
}

/* ===== 底部 ===== */
.home-footer {
  text-align: center;
  padding: var(--sp-12) var(--sp-6) var(--sp-8);
}
.home-footer-text {
  font-size: var(--fs-sm);
  color: var(--color-text-muted);
  margin: 0;
  letter-spacing: 0.04em;
}

/* ===== 响应式 ===== */
@media (max-width: 768px) {
  .hero {
    padding: var(--sp-12) var(--sp-4) var(--sp-10);
  }
  .hero-title {
    font-size: 28px;
  }
  .entry-grid {
    grid-template-columns: 1fr;
  }
  .entry-section {
    padding: 0 var(--sp-4) var(--sp-10);
  }
  .entry-card {
    min-height: 240px;
    padding: var(--sp-8) var(--sp-6);
  }
  .entry-card-icon {
    font-size: 32px;
  }
}
</style>
