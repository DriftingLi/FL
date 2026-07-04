<template>
  <div class="home-page">
    <section class="hero-section">
      <div class="hero-content">
        <div class="hero-text">
          <p class="hero-greeting">{{ greeting }}，</p>
          <h1 class="hero-name">{{ authStore.userInfo?.name || authStore.userInfo?.username }}</h1>
          <p class="hero-desc">欢迎使用叉车维修一站式服务平台</p>
          <div class="hero-actions">
            <el-button type="primary" size="large" @click="$router.push('/courses')">
              开始学习
            </el-button>
            <el-button size="large" @click="$router.push('/valuation')">
              残值评估
            </el-button>
          </div>
        </div>
        <div class="hero-stats">
          <div v-for="stat in heroStats" :key="stat.label" class="hero-stat-card">
            <span class="hero-stat-value">{{ stat.value }}</span>
            <span class="hero-stat-label">{{ stat.label }}</span>
          </div>
        </div>
      </div>
      <div class="hero-decor"></div>
    </section>

    <StatsTicker />

    <section class="modules-section">
      <div
        v-for="item in modules"
        :key="item.label"
        class="module-card"
        :class="{ 'module-disabled': item.disabled }"
        @click="onModuleClick(item)"
      >
        <div class="module-main">
          <div class="module-icon-wrap" :style="{ background: item.gradient }">
            <el-icon :size="28" color="white"><component :is="item.icon" /></el-icon>
          </div>
          <div class="module-text">
            <h3 class="module-title">{{ item.label }}</h3>
            <p class="module-desc">{{ item.desc }}</p>
          </div>
        </div>
        <div class="module-side">
          <span v-if="item.disabled" class="module-badge">敬请期待</span>
          <el-icon v-else class="module-arrow"><ArrowRight /></el-icon>
        </div>
      </div>
    </section>

    <section class="quick-section">
      <div class="quick-card quick-recent">
        <h3 class="quick-title">最近动态</h3>
        <div class="quick-placeholder">
          <el-icon><Timer /></el-icon>
          <span>暂无最近学习记录</span>
        </div>
      </div>
      <div class="quick-card quick-links">
        <h3 class="quick-title">快捷入口</h3>
        <div class="quick-link-grid">
          <router-link v-for="link in quickLinks" :key="link.path" :to="link.path" class="quick-link">
            <el-icon><component :is="link.icon" /></el-icon>
            <span>{{ link.label }}</span>
          </router-link>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { ElMessage } from 'element-plus'
import {
  Notebook, DataAnalysis, SetUp, MagicStick, ArrowRight, Timer,
  EditPen, Document, User
} from '@element-plus/icons-vue'
import StatsTicker from '@/components/layout/StatsTicker.vue'

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

const heroStats = [
  { value: '12', label: '在学课程' },
  { value: '86%', label: '平均进度' },
  { value: '128', label: '练习题目' },
  { value: '5', label: '评估记录' }
]

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

const quickLinks = [
  { path: '/question-bank', label: '题库练习', icon: EditPen },
  { path: '/level-exam', label: '考试中心', icon: Document },
  { path: '/practice', label: '虚拟实操', icon: SetUp },
  { path: '/profile', label: '个人中心', icon: User }
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
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-8);
}

.hero-text {
  flex: 1;
  min-width: 0;
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
  margin-bottom: var(--space-3);
}

.hero-desc {
  font-size: var(--text-base);
  color: rgba(255, 255, 255, 0.7);
  margin-bottom: var(--space-6);
}

.hero-actions {
  display: flex;
  gap: var(--space-3);
  flex-wrap: wrap;
}

.hero-stats {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: var(--space-4);
  flex-shrink: 0;
}

.hero-stat-card {
  background: rgba(255, 255, 255, 0.12);
  backdrop-filter: blur(8px);
  border: 1px solid rgba(255, 255, 255, 0.16);
  border-radius: var(--radius-xl);
  padding: var(--space-5);
  min-width: 140px;
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}

.hero-stat-value {
  font-family: var(--font-display);
  font-size: var(--text-2xl);
  font-weight: var(--font-bold);
  color: white;
}

.hero-stat-label {
  font-size: var(--text-sm);
  color: rgba(255, 255, 255, 0.75);
}

.hero-decor {
  position: absolute;
  width: 360px;
  height: 360px;
  border-radius: var(--radius-full);
  border: 1px solid rgba(255, 255, 255, 0.08);
  right: -80px;
  top: -80px;
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
  padding: var(--space-6);
  cursor: pointer;
  box-shadow: var(--shadow-xs);
  border: 1px solid var(--color-border-light);
  transition: all var(--duration-normal) var(--ease-default);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-4);
}

.module-card:hover {
  box-shadow: var(--shadow-lg);
  transform: translateY(-4px);
  border-color: var(--color-primary-200);
}

.module-disabled {
  opacity: 0.75;
}

.module-disabled:hover {
  transform: translateY(-2px);
}

.module-main {
  display: flex;
  align-items: center;
  gap: var(--space-4);
  min-width: 0;
}

.module-icon-wrap {
  width: 56px;
  height: 56px;
  border-radius: var(--radius-xl);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.module-text {
  min-width: 0;
}

.module-title {
  font-family: var(--font-display);
  font-size: var(--text-xl);
  font-weight: var(--font-bold);
  color: var(--color-text-primary);
  margin-bottom: var(--space-1);
}

.module-desc {
  font-size: var(--text-sm);
  color: var(--color-text-tertiary);
  line-height: var(--leading-relaxed);
}

.module-side {
  flex-shrink: 0;
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

.quick-section {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--space-5);
}

.quick-card {
  background: var(--color-bg-card);
  border-radius: var(--radius-2xl);
  padding: var(--space-6);
  box-shadow: var(--shadow-xs);
  border: 1px solid var(--color-border-light);
}

.quick-title {
  font-family: var(--font-display);
  font-size: var(--text-lg);
  font-weight: var(--font-bold);
  color: var(--color-text-primary);
  margin-bottom: var(--space-4);
}

.quick-placeholder {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-6);
  background: var(--color-bg-page);
  border-radius: var(--radius-xl);
  color: var(--color-text-tertiary);
  font-size: var(--text-sm);
}

.quick-link-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: var(--space-3);
}

.quick-link {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-3) var(--space-4);
  border-radius: var(--radius-lg);
  background: var(--color-bg-page);
  color: var(--color-text-secondary);
  text-decoration: none;
  font-size: var(--text-sm);
  font-weight: var(--font-medium);
  transition: all var(--duration-fast);
}

.quick-link:hover {
  background: var(--color-primary-50);
  color: var(--color-primary-600);
}

@media screen and (max-width: 1024px) {
  .hero-content {
    flex-direction: column;
    align-items: flex-start;
  }

  .hero-stats {
    width: 100%;
  }
}

@media screen and (max-width: 768px) {
  .hero-section {
    padding: var(--space-8) var(--space-5);
  }

  .hero-name {
    font-size: var(--text-3xl);
  }

  .hero-stats {
    grid-template-columns: repeat(2, 1fr);
  }

  .modules-section,
  .quick-section {
    grid-template-columns: 1fr;
  }
}

@media screen and (max-width: 480px) {
  .hero-section {
    padding: var(--space-6) var(--space-4);
    border-radius: var(--radius-xl);
  }

  .hero-name {
    font-size: var(--text-2xl);
  }

  .hero-actions {
    flex-direction: column;
  }

  .module-card {
    padding: var(--space-5);
  }

  .module-title {
    font-size: var(--text-lg);
  }
}
</style>
