<template>
  <div class="home-page">
    <section class="hero-section">
      <div class="hero-content">
        <p class="hero-greeting">{{ greeting }}，</p>
        <h1 class="hero-name">{{ authStore.userInfo.name || authStore.userInfo.username }}</h1>
        <p class="hero-desc">欢迎使用叉车维修一站式服务平台</p>
      </div>
      <div class="hero-decor"></div>
    </section>

    <section class="modules-section">
      <div
        v-for="item in modules"
        :key="item.label"
        class="module-card"
        :class="{ 'module-disabled': item.disabled }"
        @click="onModuleClick(item)"
      >
        <div class="module-header">
          <div class="module-icon-wrap" :style="{ background: item.gradient }">
            <el-icon :size="28" color="white"><component :is="item.icon" /></el-icon>
          </div>
          <span v-if="item.disabled" class="module-badge">敬请期待</span>
          <el-icon v-else class="module-arrow"><ArrowRight /></el-icon>
        </div>
        <h3 class="module-title">{{ item.label }}</h3>
        <p class="module-desc">{{ item.desc }}</p>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { ElMessage } from 'element-plus'
import { Notebook, DataAnalysis, SetUp, MagicStick, ArrowRight } from '@element-plus/icons-vue'

interface ModuleItem {
  label: string
  desc: string
  icon: any
  path?: string
  gradient: string
  disabled?: boolean
}

const router = useRouter()
const authStore = useAuthStore()

const currentHour = ref(new Date().getHours())

const greeting = computed(() => {
  const hour = currentHour.value
  if (hour >= 6 && hour < 12) return '早上好'
  if (hour >= 12 && hour < 14) return '中午好'
  if (hour >= 14 && hour < 18) return '下午好'
  return '晚上好'
})

let greetingTimer: ReturnType<typeof setInterval> | null = null

const modules: ModuleItem[] = [
  {
    label: '叉车维修培训',
    desc: '系统化课程、题库练习、虚拟实操与等级考核',
    icon: Notebook,
    path: '/courses',
    gradient: 'linear-gradient(135deg, #2563EB, #3B82F6)'
  },
  {
    label: '叉车残值评估',
    desc: '整车残值与电池健康度智能评估',
    icon: DataAnalysis,
    path: '/valuation',
    gradient: 'linear-gradient(135deg, #059669, #10B981)'
  },
  {
    label: '叉车派单系统',
    desc: '维修工单派发与管理',
    icon: SetUp,
    path: '/dispatch',
    gradient: 'linear-gradient(135deg, #EA580C, #F97316)',
    disabled: true
  },
  {
    label: 'AI 助手',
    desc: '智能问答与维修知识辅助',
    icon: MagicStick,
    path: '/ai-generate',
    gradient: 'linear-gradient(135deg, #8B5CF6, #A78BFA)'
  }
]

function onModuleClick(item: ModuleItem) {
  if (item.disabled) {
    ElMessage.info('「叉车派单系统」敬请期待')
    router.push('/dispatch')
    return
  }
  if (item.path) router.push(item.path)
}

onMounted(() => {
  greetingTimer = setInterval(() => {
    currentHour.value = new Date().getHours()
  }, 60000)
})

onUnmounted(() => {
  if (greetingTimer) {
    clearInterval(greetingTimer)
    greetingTimer = null
  }
})
</script>

<style scoped>
.home-page {
  display: flex;
  flex-direction: column;
  gap: var(--space-6);
}

.hero-section {
  background: var(--gradient-brand);
  border-radius: var(--radius-2xl);
  padding: var(--space-10) var(--space-8);
  position: relative;
  overflow: hidden;
}

.hero-content {
  position: relative;
  z-index: 2;
}

.hero-greeting {
  font-size: var(--text-lg);
  color: rgba(255, 255, 255, 0.8);
  margin-bottom: var(--space-1);
}

.hero-name {
  font-family: var(--font-display);
  font-size: var(--text-4xl);
  font-weight: var(--font-bold);
  color: white;
  margin-bottom: var(--space-2);
}

.hero-desc {
  font-size: var(--text-base);
  color: rgba(255, 255, 255, 0.7);
}

.hero-decor {
  position: absolute;
  width: 300px;
  height: 300px;
  border-radius: var(--radius-full);
  border: 1px solid rgba(255, 255, 255, 0.06);
  right: -60px;
  top: -60px;
  pointer-events: none;
}

.modules-section {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: var(--space-5);
}

.module-card {
  background: var(--color-bg-card);
  border-radius: var(--radius-2xl);
  padding: var(--space-8);
  cursor: pointer;
  box-shadow: var(--shadow-xs);
  border: 1px solid var(--color-border-light);
  transition: all var(--duration-normal) var(--ease-default);
}

.module-card:hover {
  box-shadow: var(--shadow-lg);
  transform: translateY(-4px);
}

.module-disabled {
  opacity: 0.75;
}

.module-disabled:hover {
  transform: translateY(-2px);
}

.module-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--space-5);
}

.module-icon-wrap {
  width: 56px;
  height: 56px;
  border-radius: var(--radius-xl);
  display: flex;
  align-items: center;
  justify-content: center;
}

.module-badge {
  padding: var(--space-1) var(--space-3);
  border-radius: var(--radius-full);
  background: var(--color-accent-50);
  color: var(--color-accent-600);
  font-size: var(--text-xs);
  font-weight: var(--font-semibold);
}

.module-arrow {
  color: var(--color-text-disabled);
  transition: all var(--duration-fast) var(--ease-default);
}

.module-card:hover .module-arrow {
  color: var(--color-primary-500);
  transform: translateX(4px);
}

.module-title {
  font-family: var(--font-display);
  font-size: var(--text-xl);
  font-weight: var(--font-bold);
  color: var(--color-text-primary);
  margin-bottom: var(--space-2);
}

.module-desc {
  font-size: var(--text-sm);
  color: var(--color-text-tertiary);
  line-height: var(--leading-relaxed);
}

@media screen and (max-width: 768px) {
  .hero-section {
    padding: var(--space-6) var(--space-5);
  }

  .hero-name {
    font-size: var(--text-2xl);
  }

  .modules-section {
    grid-template-columns: 1fr;
  }
}

@media screen and (max-width: 480px) {
  .hero-section {
    padding: var(--space-5) var(--space-4);
    border-radius: var(--radius-xl);
  }

  .hero-name {
    font-size: var(--text-xl);
  }

  .module-card {
    padding: var(--space-6);
  }
}
</style>
