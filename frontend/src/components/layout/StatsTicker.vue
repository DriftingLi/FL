<template>
  <div class="stats-ticker">
    <div class="ticker-container">
      <div v-for="item in stats" :key="item.label" class="ticker-item">
        <span class="ticker-value">{{ item.value }}</span>
        <span class="ticker-label">{{ item.label }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
interface StatItem {
  value: string
  label: string
}

const stats: StatItem[] = [
  { value: '10,000+', label: '注册学员' },
  { value: '120+', label: '精品课程' },
  { value: '8,500+', label: '题库题目' },
  { value: '3,200+', label: '残值评估' }
]
</script>

<style scoped>
.stats-ticker {
  background: var(--color-bg-card);
  border-top: 1px solid var(--ticker-border, var(--color-border-light));
  border-bottom: 1px solid var(--ticker-border, var(--color-border-light));
}

.ticker-container {
  max-width: var(--container-page);
  margin: 0 auto;
  padding: var(--space-6) var(--space-5);
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: var(--space-8);
}

.ticker-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  gap: var(--space-2);
}

.ticker-value {
  font-family: var(--font-display);
  font-size: var(--text-3xl);
  font-weight: var(--font-bold);
  color: var(--color-text-primary);
  background: var(--gradient-brand);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.ticker-label {
  font-size: var(--text-sm);
  color: var(--color-text-secondary);
  font-weight: var(--font-medium);
}

@media screen and (max-width: 768px) {
  .ticker-container {
    grid-template-columns: repeat(2, 1fr);
    gap: var(--space-6);
  }
}

@media screen and (max-width: 480px) {
  .ticker-container {
    grid-template-columns: 1fr;
    gap: var(--space-5);
  }
}
</style>
