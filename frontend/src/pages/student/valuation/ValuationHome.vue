// 残值评估首页（设计稿风格：白底 + 居中 Hero + 卡片入口）
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
    meta: '基于品牌、型号、使用年限、工作环境等多维度数据，智能评估叉车当前市场残值',
    icon: '🚜',
    onClick: goValuation
  },
  {
    key: 'battery',
    title: '电池健康度评估',
    subtitle: 'Battery RUL',
    meta: '通过电池充放电数据分析，预测电池剩余使用寿命（RUL），帮助您制定科学的维护与更换计划',
    icon: '🔋',
    onClick: goBattery
  },
  {
    key: 'history',
    title: '评估历史记录',
    subtitle: 'Evaluation History',
    meta: '查看所有评估记录与报告，追踪叉车资产价值变化趋势，支持数据导出与对比分析',
    icon: '📋',
    onClick: goHistory
  }
] as const
</script>

<template>
  <div class="home valuation-root">
    <!-- Hero 区域：居中标题 + 副标题 -->
    <section class="hero">
      <div class="hero-inner">
        <p class="hero-eyebrow">forklift residual value</p>
        <h1 class="hero-title">叉车残值与电池健康度<br />一站式评估</h1>
        <p class="hero-sub">
          专业的叉车残值评估与电池健康度检测平台，为您提供精准的资产估值与设备状态分析服务。
        </p>
      </div>
    </section>

    <!-- 残值评估主入口：单卡居中突出 -->
    <section class="entry-section entry-section-primary">
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
          <span class="entry-card-cta">
            开始评估 <span class="entry-card-arrow">→</span>
          </span>
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
          <span class="entry-card-cta">
            进入 <span class="entry-card-arrow">→</span>
          </span>
        </article>
      </div>
    </section>
  </div>
</template>

<style scoped>
.home {
  background: var(--color-surface);
  min-height: calc(100vh - var(--header-h) - 40px);
  display: flex;
  flex-direction: column;
}

/* ===== Hero ===== */
.hero {
  padding: var(--sp-20) var(--sp-6) var(--sp-12);
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
  color: var(--color-text-muted);
  margin: 0 0 var(--sp-4);
}
.hero-title {
  font-family: var(--font-text);
  font-size: var(--fs-4xl);
  font-weight: var(--fw-medium);
  line-height: 1.25;
  color: var(--color-text);
  margin: 0 0 var(--sp-5);
  letter-spacing: normal;
}
.hero-sub {
  font-size: var(--fs-base);
  font-weight: var(--fw-regular);
  color: var(--color-text-secondary);
  margin: 0 auto;
  line-height: 1.75;
  max-width: 520px;
}

/* ===== 入口区域 ===== */
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
.entry-grid-primary {
  max-width: 720px;
  grid-template-columns: 1fr;
}

/* ===== 更多功能区域 ===== */
.entry-section-secondary {
  padding-top: 0;
  padding-bottom: var(--sp-24);
}
.section-label {
  max-width: 960px;
  margin: 0 auto var(--sp-6);
  font-size: var(--fs-sm);
  font-weight: var(--fw-medium);
  color: var(--color-text-muted);
  letter-spacing: 0.18em;
  text-transform: uppercase;
  text-align: center;
}

/* ===== 卡片 ===== */
.entry-card {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  padding: var(--sp-10) var(--sp-8);
  text-align: center;
  cursor: pointer;
  transition:
    border-color var(--t-base) var(--ease),
    box-shadow var(--t-base) var(--ease);
  display: flex;
  flex-direction: column;
  align-items: center;
  min-height: 320px;
  justify-content: center;
}
.entry-card:hover {
  border-color: var(--color-accent);
}
.entry-grid-primary .entry-card {
  min-height: 360px;
  padding: var(--sp-12) var(--sp-10);
}
.entry-card-secondary {
  min-height: 240px;
  padding: var(--sp-8) var(--sp-6);
  text-align: left;
  align-items: flex-start;
}

.entry-card-icon {
  font-size: 40px;
  color: var(--color-text);
  margin-bottom: var(--sp-5);
  line-height: 1;
  transition: color var(--t-base) var(--ease);
}
.entry-grid-primary .entry-card-icon {
  font-size: 48px;
  margin-bottom: var(--sp-6);
}
.entry-card-secondary .entry-card-icon {
  font-size: 32px;
  margin-bottom: var(--sp-4);
}
.entry-card:hover .entry-card-icon {
  color: var(--color-accent);
}

.entry-card-title {
  font-size: var(--fs-xl);
  font-weight: var(--fw-medium);
  color: var(--color-text);
  margin: 0 0 var(--sp-1);
}
.entry-card-secondary .entry-card-title {
  font-size: var(--fs-lg);
}
.entry-card-subtitle {
  font-size: var(--fs-xs);
  color: var(--color-text-muted);
  margin: 0 0 var(--sp-3);
  letter-spacing: 0.04em;
  text-transform: uppercase;
  font-weight: var(--fw-medium);
}
.entry-card-meta {
  font-size: var(--fs-sm);
  color: var(--color-text-secondary);
  margin: 0 0 var(--sp-6);
  line-height: 1.75;
  max-width: 460px;
}
.entry-card-secondary .entry-card-meta {
  margin-bottom: var(--sp-5);
}
.entry-card-cta {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: var(--fs-base);
  font-weight: var(--fw-medium);
  color: var(--color-accent);
  transition: gap var(--t-base) var(--ease);
}
.entry-card-secondary .entry-card-cta {
  font-size: var(--fs-sm);
}
.entry-card:hover .entry-card-cta {
  gap: 10px;
}
.entry-card-arrow {
  font-size: 12px;
  transition: transform var(--t-base) var(--ease);
}
.entry-card:hover .entry-card-arrow {
  transform: translateX(4px);
}

/* ===== 响应式 ===== */
@media (max-width: 768px) {
  .hero {
    padding: var(--sp-12) var(--sp-4) var(--sp-8);
  }
  .hero-title {
    font-size: var(--fs-2xl);
  }
  .hero-sub {
    font-size: var(--fs-sm);
  }
  .entry-grid {
    grid-template-columns: 1fr;
  }
  .entry-section {
    padding: 0 var(--sp-4) var(--sp-10);
  }
  .entry-section-secondary {
    padding-bottom: var(--sp-16);
  }
  .entry-card,
  .entry-grid-primary .entry-card {
    min-height: 240px;
    padding: var(--sp-8) var(--sp-6);
  }
  .entry-card-icon,
  .entry-grid-primary .entry-card-icon {
    font-size: 32px;
    margin-bottom: var(--sp-4);
  }
  .entry-card-title,
  .entry-grid-primary .entry-card-title {
    font-size: var(--fs-lg);
  }
}
</style>
