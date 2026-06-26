// 首页：100vh 极简 hero + 两张白卡入口（Tesla 风格）
<script setup lang="ts">
import { useRouter } from 'vue-router'

const router = useRouter()

function goElectric() {
  router.push('/valuation/input/electric')
}
function goCombustion() {
  router.push('/valuation/input/combustion')
}

interface EntryCard {
  key: 'electric' | 'combustion'
  title: string
  subtitle: string
  meta: string
  icon: string
  onClick: () => void
}

const cards: readonly EntryCard[] = [
  {
    key: 'electric',
    title: '电动叉车',
    subtitle: 'Electric Forklift',
    meta: '10 部件大类 · 68 评估项',
    icon: '⚡',
    onClick: goElectric
  },
  {
    key: 'combustion',
    title: '内燃叉车',
    subtitle: 'Internal Combustion',
    meta: '12 部件大类 · 75 评估项',
    icon: '🔥',
    onClick: goCombustion
  }
] as const
</script>

<template>
  <div class="home valuation-root">
    <!-- Hero 区域：100vh，居中标题 + 副标题 + 极简分隔 -->
    <section class="hero">
      <div class="hero-inner">
        <p class="hero-eyebrow">forklift residual value</p>
        <h1 class="hero-title">选择叉车类型<br />开始残值评估</h1>
        <p class="hero-sub">基于车况 / 品牌 / 使用强度 / 工况 / 时间的五维残值模型</p>
      </div>
    </section>

    <!-- Category 入口：两张白卡 -->
    <section class="entry-section">
      <div class="entry-grid">
        <article
          v-for="card in cards"
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
