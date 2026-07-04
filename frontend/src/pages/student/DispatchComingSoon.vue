<template>
  <div class="coming-soon-page">
    <div class="coming-soon-card">
      <div class="icon-wrap">
        <el-icon :size="48" color="white"><SetUp /></el-icon>
      </div>
      <h1 class="title">叉车派单系统</h1>
      <p class="subtitle">敬请期待</p>
      <p class="desc">该模块正在紧锣密鼓地开发中，上线后将提供维修工单派发与管理功能。</p>
      <el-button type="primary" size="large" class="back-btn" @click="router.push('/dashboard')">返回首页</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'
import { SetUp } from '@element-plus/icons-vue'

const router = useRouter()
</script>

<style scoped>
.coming-soon-page {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 70vh;
  padding: var(--space-6);
}

.coming-soon-card {
  background: var(--color-bg-card);
  border-radius: var(--radius-2xl);
  box-shadow: var(--shadow-md);
  border: 1px solid var(--color-border-light);
  padding: var(--space-12) var(--space-10);
  text-align: center;
  max-width: 480px;
  width: 100%;
}

.icon-wrap {
  width: 88px;
  height: 88px;
  border-radius: var(--radius-2xl);
  background: linear-gradient(135deg, #EA580C, #F97316);
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 0 auto var(--space-6);
}

.title {
  font-family: var(--font-display);
  font-size: var(--text-3xl);
  font-weight: var(--font-bold);
  color: var(--color-text-primary);
  margin-bottom: var(--space-2);
}

.subtitle {
  font-size: var(--text-xl);
  font-weight: var(--font-semibold);
  color: var(--color-accent-600);
  margin-bottom: var(--space-4);
}

.desc {
  font-size: var(--text-base);
  color: var(--color-text-tertiary);
  line-height: var(--leading-relaxed);
  margin-bottom: var(--space-8);
}

.back-btn {
  min-width: 160px;
  border-radius: var(--radius-lg);
  font-weight: var(--font-semibold);
}
</style>
