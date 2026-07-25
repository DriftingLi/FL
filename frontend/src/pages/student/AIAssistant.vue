<template>
  <div class="coming-soon-page">
    <div class="coming-soon-card">
      <div class="icon-wrap">
        <el-icon :size="48" color="white"><MagicStick /></el-icon>
      </div>
      <h1 class="title">AI 叉车助手</h1>
      <p class="subtitle">敬请期待</p>
      <p class="desc">该模块正在紧锣密鼓地升级中，上线后将提供基于大语言模型的智能问答服务，随时解答叉车选购、维保、故障诊断等专业问题。</p>
      <el-button type="primary" size="large" class="back-btn" @click="goBack">返回</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'
import { MagicStick } from '@element-plus/icons-vue'

const router = useRouter()

function goBack() {
  // 优先返回上一页；若无历史记录则回到首页
  if (window.history.length > 1) {
    router.back()
  } else {
    router.push('/')
  }
}
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
  background: linear-gradient(135deg, #6366F1, #8B5CF6);
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
